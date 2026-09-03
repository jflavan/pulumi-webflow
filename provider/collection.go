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
	"regexp"
	"strings"
)

// Collection represents a Webflow CMS collection.
// This struct matches the Webflow API v2 response format for collections.
type Collection struct {
	ID           string `json:"id,omitempty"`          // Webflow-assigned collection ID (read-only)
	DisplayName  string `json:"displayName"`           // Human-readable name of the collection
	SingularName string `json:"singularName"`          // Singular form of the collection name
	Slug         string `json:"slug,omitempty"`        // URL-friendly slug for the collection
	CreatedOn    string `json:"createdOn,omitempty"`   // Creation timestamp (read-only)
	LastUpdated  string `json:"lastUpdated,omitempty"` // Last update timestamp (read-only)
	// Field definitions (GET /v2/collections/{id} only)
	Fields []CollectionFieldResponse `json:"fields,omitempty"`
}

// CollectionRequest represents the request body for POST collection.
type CollectionRequest struct {
	DisplayName  string `json:"displayName"`    // Human-readable name
	SingularName string `json:"singularName"`   // Singular form
	Slug         string `json:"slug,omitempty"` // Optional URL slug
}

// CollectionUpdateRequest represents the request body for PATCH /v2/collections/{collection_id}.
// Every property is optional; only the ones set are changed.
type CollectionUpdateRequest struct {
	DisplayName  string `json:"displayName,omitempty"`
	SingularName string `json:"singularName,omitempty"`
	Slug         string `json:"slug,omitempty"`
}

// collectionSlugPattern matches URL-friendly collection slugs.
var collectionSlugPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ValidateCollectionSlug validates an explicit collection slug. An empty slug is valid and
// lets Webflow generate one from displayName.
func ValidateCollectionSlug(slug string) error {
	if slug == "" {
		return nil
	}
	if len(slug) > 255 {
		return fmt.Errorf("slug is too long: '%s' exceeds maximum length of 255 characters", slug)
	}
	if !collectionSlugPattern.MatchString(slug) {
		return fmt.Errorf("slug has invalid format: got '%s'. "+
			"Expected a URL-friendly slug containing only lowercase letters, digits, hyphens and underscores "+
			"(e.g., 'blog-posts'), or omit it to let Webflow generate one from displayName", slug)
	}
	return nil
}

// pathIDPattern matches identifiers that are safe to embed as a single URL path segment.
// Webflow field and item IDs are 24-character hex strings, but the pattern is deliberately
// a little looser so that imports of unusual IDs still work; it only guards against
// separators and other characters that would change the meaning of the request path.
var pathIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// validatePathID validates an identifier that will be interpolated into an API path.
func validatePathID(name, id string) error {
	if id == "" {
		return fmt.Errorf("%s is required but was not provided. "+
			"Please provide a valid Webflow %s (e.g., '5f0c8c9e1c9d440000e8d8c3')", name, name)
	}
	if !pathIDPattern.MatchString(id) {
		return fmt.Errorf("%s has invalid format: got '%s'. "+
			"Expected an identifier containing only letters, digits, hyphens and underscores "+
			"(Webflow IDs are 24-character lowercase hexadecimal strings, e.g., '5f0c8c9e1c9d440000e8d8c3')", name, id)
	}
	return nil
}

// ValidateCollectionID validates that a collectionID matches the Webflow collection ID format.
// Collection IDs are 24-character lowercase hexadecimal strings (same format as site IDs).
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateCollectionID(collectionID string) error {
	if collectionID == "" {
		return errors.New("collectionId is required but was not provided. " +
			"Please provide a valid Webflow collection ID " +
			"(24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c3'). " +
			"You can find collection IDs via the Webflow API or dashboard")
	}
	if !siteIDPattern.MatchString(collectionID) {
		return fmt.Errorf("collectionId has invalid format: got '%s'. "+
			"Expected a 24-character lowercase hexadecimal string "+
			"(e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"Please ensure the collection ID contains only lowercase letters (a-f) and digits (0-9)", collectionID)
	}
	return nil
}

// ValidateCollectionDisplayName validates that displayName is non-empty and reasonable length.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateCollectionDisplayName(displayName string) error {
	if displayName == "" {
		return errors.New("displayName is required but was not provided. " +
			"Please provide a name for your collection (e.g., 'Blog Posts', 'Products', 'Team Members'). " +
			"The display name is shown in the Webflow CMS interface")
	}
	if len(displayName) > 255 {
		return fmt.Errorf("displayName is too long: '%s' exceeds maximum length of 255 characters. "+
			"Please use a shorter, more concise name for your collection", displayName)
	}
	return nil
}

// ValidateSingularName validates that singularName is non-empty and reasonable length.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateSingularName(singularName string) error {
	if singularName == "" {
		return errors.New("singularName is required but was not provided. " +
			"Please provide the singular form of your collection name " +
			"(e.g., 'Blog Post' for 'Blog Posts', 'Product' for 'Products'). " +
			"The singular name is used in the CMS UI when referring to individual items")
	}
	if len(singularName) > 255 {
		return fmt.Errorf("singularName is too long: '%s' exceeds maximum length of 255 characters. "+
			"Please use a shorter name", singularName)
	}
	return nil
}

// GenerateCollectionResourceID generates a Pulumi resource ID for a Collection resource.
// Format: {siteID}/collections/{collectionID}
func GenerateCollectionResourceID(siteID, collectionID string) string {
	return fmt.Sprintf("%s/collections/%s", siteID, collectionID)
}

// ExtractIDsFromCollectionResourceID extracts the siteID and collectionID from a Collection resource ID.
// Expected format: {siteID}/collections/{collectionID}
func ExtractIDsFromCollectionResourceID(resourceID string) (siteID, collectionID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.Split(resourceID, "/")
	if len(parts) < 3 || parts[1] != "collections" {
		return "", "", fmt.Errorf(
			"invalid resource ID format: expected {siteId}/collections/{collectionId}, got: %s",
			resourceID,
		)
	}

	siteID = parts[0]
	collectionID = strings.Join(parts[2:], "/") // Handle collectionID that might contain slashes

	return siteID, collectionID, nil
}

// GetCollection retrieves a single collection by ID, including its field definitions.
// It calls GET /v2/collections/{collection_id}.
// A 404 is returned as an error satisfying IsNotFound.
func GetCollection(ctx context.Context, client *http.Client, collectionID string) (*Collection, error) {
	var collection Collection
	if _, err := doRequest(ctx, client, http.MethodGet,
		apiURL("/v2/collections/%s", collectionID), nil, &collection, http.StatusOK); err != nil {
		return nil, err
	}
	return &collection, nil
}

// PostCollection creates a new collection for a Webflow site.
// It calls POST /v2/sites/{site_id}/collections.
// An empty slug lets Webflow generate one from displayName.
func PostCollection(
	ctx context.Context, client *http.Client,
	siteID, displayName, singularName, slug string,
) (*Collection, error) {
	body := CollectionRequest{
		DisplayName:  displayName,
		SingularName: singularName,
		Slug:         slug,
	}
	var collection Collection
	if _, err := doRequest(ctx, client, http.MethodPost,
		apiURL("/v2/sites/%s/collections", siteID), body, &collection,
		http.StatusOK, http.StatusCreated, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &collection, nil
}

// PatchCollection updates the display name, singular name and/or slug of a collection.
// It calls PATCH /v2/collections/{collection_id} (scope cms:write); every body property is
// optional and only the ones set are changed. The API answers 200 with the updated collection.
func PatchCollection(
	ctx context.Context, client *http.Client, collectionID string, body CollectionUpdateRequest,
) (*Collection, error) {
	var collection Collection
	if _, err := doRequest(ctx, client, http.MethodPatch,
		apiURL("/v2/collections/%s", collectionID), body, &collection,
		http.StatusOK, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &collection, nil
}

// DeleteCollection removes a collection from a Webflow site.
// It calls DELETE /v2/collections/{collection_id}; a 404 is treated as success.
func DeleteCollection(ctx context.Context, client *http.Client, collectionID string) error {
	return doDelete(ctx, client, apiURL("/v2/collections/%s", collectionID), nil)
}
