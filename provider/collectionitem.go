// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// CollectionItem represents a single item in a Webflow CMS collection.
// This struct matches the Webflow API v2 response format for collection items.
type CollectionItem struct {
	ID            string                 `json:"id,omitempty"`            // Webflow-assigned item ID (read-only)
	CmsLocaleID   string                 `json:"cmsLocaleId,omitempty"`   // Locale ID for localized sites
	LastPublished string                 `json:"lastPublished,omitempty"` // Last publish timestamp (read-only)
	LastUpdated   string                 `json:"lastUpdated,omitempty"`   // Last update timestamp (read-only)
	CreatedOn     string                 `json:"createdOn,omitempty"`     // Creation timestamp (read-only)
	IsArchived    bool                   `json:"isArchived"`              // Whether the item is archived
	IsDraft       bool                   `json:"isDraft"`                 // Whether the item is a draft
	FieldData     map[string]interface{} `json:"fieldData"`               // Dynamic field data (name, slug, etc.)
}

// CollectionItemRequest represents the request body for POST/PATCH collection items.
type CollectionItemRequest struct {
	FieldData   map[string]interface{} `json:"fieldData"`             // Dynamic field data
	IsArchived  *bool                  `json:"isArchived,omitempty"`  // Whether the item is archived
	IsDraft     *bool                  `json:"isDraft,omitempty"`     // Whether the item is a draft
	CmsLocaleID string                 `json:"cmsLocaleId,omitempty"` // Locale ID for localized sites
}

// CollectionItemPublishTarget identifies one item (and optionally its locales) to publish.
type CollectionItemPublishTarget struct {
	ID           string   `json:"id"`
	CmsLocaleIDs []string `json:"cmsLocaleIds,omitempty"`
}

// CollectionItemPublishRequest is the body for POST /v2/collections/{id}/items/publish.
// Exactly one of ItemIDs or Items is sent.
type CollectionItemPublishRequest struct {
	ItemIDs []string                      `json:"itemIds,omitempty"`
	Items   []CollectionItemPublishTarget `json:"items,omitempty"`
}

// CollectionItemPublishResponse is the response of the publish endpoint.
type CollectionItemPublishResponse struct {
	PublishedItemIDs []string `json:"publishedItemIds"`
	Errors           []string `json:"errors"`
}

// ValidateFieldData validates that fieldData is non-empty and contains required fields.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateFieldData(fieldData map[string]interface{}) error {
	if len(fieldData) == 0 {
		return errors.New("fieldData is required but was not provided. " +
			"Please provide a map of field slugs to values (e.g., {\"name\": \"My Item\", \"slug\": \"my-item\"}). " +
			"The field slugs must match the fields defined in the collection schema")
	}
	// Note: Name and slug are typically required but may be auto-generated
	// We don't enforce this here as it depends on the collection schema
	return nil
}

// ValidateItemID validates a Webflow item ID before it is used in an API path.
func ValidateItemID(itemID string) error {
	return validatePathID("itemId", itemID)
}

// GenerateCollectionItemResourceID generates a Pulumi resource ID for a CollectionItem resource.
// Format: {collectionID}/items/{itemID}
func GenerateCollectionItemResourceID(collectionID, itemID string) string {
	return fmt.Sprintf("%s/items/%s", collectionID, itemID)
}

// ExtractIDsFromCollectionItemResourceID extracts the collectionID and itemID from a CollectionItem resource ID.
// Expected format: {collectionID}/items/{itemID}
func ExtractIDsFromCollectionItemResourceID(resourceID string) (collectionID, itemID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.Split(resourceID, "/")
	if len(parts) < 3 || parts[1] != "items" {
		return "", "", fmt.Errorf(
			"invalid resource ID format: expected {collectionId}/items/{itemId}, got: %s",
			resourceID,
		)
	}

	collectionID = parts[0]
	itemID = strings.Join(parts[2:], "/") // Handle itemID that might contain slashes

	return collectionID, itemID, nil
}

// collectionItemURL builds the URL of a single item. live selects the /live variant,
// which addresses the published copy of the item instead of the staged one.
func collectionItemURL(collectionID, itemID, cmsLocaleID string, live bool) string {
	u := apiURL("/v2/collections/%s/items/%s", collectionID, itemID)
	if live {
		u += "/live"
	}
	if cmsLocaleID != "" {
		u += "?" + url.Values{"cmsLocaleId": {cmsLocaleID}}.Encode()
	}
	return u
}

// GetCollectionItem retrieves a single collection item by ID.
// It calls GET /v2/collections/{collection_id}/items/{item_id} (or .../live when live is true),
// passing cmsLocaleId as a query parameter when set.
// A 404 is returned as an error satisfying IsNotFound.
func GetCollectionItem(
	ctx context.Context, client *http.Client, collectionID, itemID, cmsLocaleID string, live bool,
) (*CollectionItem, error) {
	var item CollectionItem
	if _, err := doRequest(ctx, client, http.MethodGet,
		collectionItemURL(collectionID, itemID, cmsLocaleID, live), nil, &item, http.StatusOK); err != nil {
		return nil, err
	}
	return &item, nil
}

// PostCollectionItem creates a new staged item in a Webflow collection.
// It calls POST /v2/collections/{collection_id}/items. The API answers 202 Accepted.
func PostCollectionItem(
	ctx context.Context, client *http.Client, collectionID string, body CollectionItemRequest,
) (*CollectionItem, error) {
	var item CollectionItem
	if _, err := doRequest(ctx, client, http.MethodPost,
		apiURL("/v2/collections/%s/items", collectionID), body, &item,
		http.StatusOK, http.StatusCreated, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &item, nil
}

// PatchCollectionItem updates an existing staged item in a Webflow collection.
// It calls PATCH /v2/collections/{collection_id}/items/{item_id}.
func PatchCollectionItem(
	ctx context.Context, client *http.Client, collectionID, itemID string, body CollectionItemRequest,
) (*CollectionItem, error) {
	var item CollectionItem
	if _, err := doRequest(ctx, client, http.MethodPatch,
		apiURL("/v2/collections/%s/items/%s", collectionID, itemID), body, &item,
		http.StatusOK, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &item, nil
}

// PublishCollectionItems publishes staged items to the live site.
// It calls POST /v2/collections/{collection_id}/items/publish with itemIds, or with
// items[{id, cmsLocaleIds}] when a locale is given. The API answers 202 Accepted with the
// IDs that were published and any per-item errors; an item that was not published is an error.
func PublishCollectionItems(
	ctx context.Context, client *http.Client, collectionID string, itemIDs []string, cmsLocaleID string,
) (*CollectionItemPublishResponse, error) {
	body := CollectionItemPublishRequest{}
	if cmsLocaleID == "" {
		body.ItemIDs = itemIDs
	} else {
		for _, id := range itemIDs {
			body.Items = append(body.Items, CollectionItemPublishTarget{ID: id, CmsLocaleIDs: []string{cmsLocaleID}})
		}
	}

	var response CollectionItemPublishResponse
	if _, err := doRequest(ctx, client, http.MethodPost,
		apiURL("/v2/collections/%s/items/publish", collectionID), body, &response,
		http.StatusOK, http.StatusAccepted); err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		return &response, fmt.Errorf("webflow reported errors while publishing items: %s. "+
			"Publishing requires the site to have been published at least once and the item to exist in staging",
			strings.Join(response.Errors, "; "))
	}
	published := make(map[string]bool, len(response.PublishedItemIDs))
	for _, id := range response.PublishedItemIDs {
		published[id] = true
	}
	for _, id := range itemIDs {
		if !published[id] {
			return &response, fmt.Errorf("item %s was not published: the Webflow API did not include it in publishedItemIds", id)
		}
	}
	return &response, nil
}

// DeleteCollectionItem removes an item from a Webflow collection.
// It calls DELETE /v2/collections/{collection_id}/items/{item_id}, or the /live variant
// when live is true (which unpublishes the item from the live site).
// A 404 is treated as success so deletes are idempotent.
func DeleteCollectionItem(ctx context.Context, client *http.Client, collectionID, itemID string, live bool) error {
	return doDelete(ctx, client, collectionItemURL(collectionID, itemID, "", live), nil)
}

// prepareFieldDataForPatch returns a copy of newFieldData suitable for a PATCH request.
//
// Webflow rejects updates that re-send an unchanged slug with a "Unique value is already in
// database" validation error, so the slug is dropped from the payload when it matches the
// slug already recorded in state. Non-string slugs are left untouched so the API can report them.
func prepareFieldDataForPatch(oldFieldData, newFieldData map[string]interface{}) map[string]interface{} {
	fieldData := make(map[string]interface{}, len(newFieldData))
	for k, v := range newFieldData {
		fieldData[k] = v
	}

	oldSlug, oldOk := oldFieldData["slug"].(string)
	newSlug, newOk := fieldData["slug"].(string)
	if oldOk && newOk && oldSlug == newSlug {
		delete(fieldData, "slug")
	}
	return fieldData
}
