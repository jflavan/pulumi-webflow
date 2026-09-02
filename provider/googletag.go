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
	"regexp"
	"strings"
)

// GoogleTagEntry is a single Google Tag configured on a site.
// This struct matches the Webflow API v2 shape used by the
// /v2/sites/{site_id}/integrations/google_tags endpoints.
type GoogleTagEntry struct {
	// DisplayName is the human-readable label shown for the tag in the Webflow dashboard.
	DisplayName string `json:"displayName"`
	// TagID is the Google Tag ID (G-, GT-, AW- or DC- prefix). UA- tags are rejected by Webflow.
	TagID string `json:"tagId"`
	// Order is the display position of the tag. Optional on input; always present on output.
	Order *int `json:"order,omitempty"`
}

// GoogleTagsResponse is the response body returned by the list, upsert and
// delete-single Google Tag endpoints. Tags are sorted by order.
type GoogleTagsResponse struct {
	// GoogleTagIDs is the list of tags configured on the site, sorted by order.
	GoogleTagIDs []GoogleTagEntry `json:"googleTagIds"`
}

// GoogleTagsRequest is the request body for PATCH /v2/sites/{site_id}/integrations/google_tags.
// Tags already on the site that are not referenced in the request are preserved.
type GoogleTagsRequest struct {
	// GoogleTagIDs is the list of tags to add or update.
	GoogleTagIDs []GoogleTagEntry `json:"googleTagIds"`
}

// googleTagIDPattern matches the tag ID prefixes Webflow accepts: G-, GT-, AW- and DC-.
// The UA- prefix (Universal Analytics) is explicitly rejected by the API.
var googleTagIDPattern = regexp.MustCompile(`^(G|GT|AW|DC)-[A-Za-z0-9]+$`)

// ValidateGoogleTagID validates a Google Tag ID.
// Webflow accepts G-, GT-, AW- and DC- prefixed IDs and rejects UA- IDs.
func ValidateGoogleTagID(tagID string) error {
	if tagID == "" {
		return errors.New("tagId is required but was not provided. " +
			"Please provide the Google Tag ID from Google Analytics, Google Ads or Google Tag Manager " +
			"(e.g., 'G-1A2B3C4D5E', 'GT-ABC123', 'AW-123456789', 'DC-1234567')")
	}
	if strings.HasPrefix(strings.ToUpper(tagID), "UA-") {
		return fmt.Errorf("tagId '%s' uses the Universal Analytics 'UA-' prefix, which Webflow does not accept. "+
			"Universal Analytics was retired by Google; use a Google Analytics 4 measurement ID (e.g., 'G-1A2B3C4D5E') "+
			"or a Google Tag ID (e.g., 'GT-ABC123') instead", tagID)
	}
	if !googleTagIDPattern.MatchString(tagID) {
		return fmt.Errorf("tagId has invalid format: got '%s'. "+
			"Expected a Google Tag ID with a G-, GT-, AW- or DC- prefix followed by letters and digits "+
			"(e.g., 'G-1A2B3C4D5E', 'GT-ABC123', 'AW-123456789', 'DC-1234567')", tagID)
	}
	return nil
}

// ValidateGoogleTagDisplayName validates the display name of a Google Tag.
func ValidateGoogleTagDisplayName(displayName string) error {
	if strings.TrimSpace(displayName) == "" {
		return errors.New("displayName is required but was not provided. " +
			"Please provide a human-readable label for the tag (e.g., 'Primary Google Analytics')")
	}
	return nil
}

// ValidateGoogleTagOrder validates the optional order of a Google Tag.
// A nil order lets Webflow assign the position automatically.
func ValidateGoogleTagOrder(order *int) error {
	if order != nil && *order < 0 {
		return fmt.Errorf("order must not be negative: got %d. "+
			"Omit order to let Webflow append the tag, or provide its position in the list", *order)
	}
	return nil
}

// GenerateGoogleTagResourceID generates a Pulumi resource ID for a GoogleTag resource.
// Format: {siteID}/google_tags/{tagID}
func GenerateGoogleTagResourceID(siteID, tagID string) string {
	return fmt.Sprintf("%s/google_tags/%s", siteID, tagID)
}

// ExtractIDsFromGoogleTagResourceID extracts the siteID and tagID from a GoogleTag resource ID.
// Expected format: {siteID}/google_tags/{tagID}
func ExtractIDsFromGoogleTagResourceID(resourceID string) (siteID, tagID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}
	parts := strings.Split(resourceID, "/")
	if len(parts) != 3 || parts[1] != "google_tags" || parts[0] == "" || parts[2] == "" {
		return "", "", fmt.Errorf("invalid resource ID format: expected {siteId}/google_tags/{tagId}, got: %s", resourceID)
	}
	return parts[0], parts[2], nil
}

// findGoogleTag returns the entry whose tagId matches, or nil when absent.
// Matching is case-insensitive because Webflow normalizes tag IDs.
func findGoogleTag(tags []GoogleTagEntry, tagID string) *GoogleTagEntry {
	for i := range tags {
		if strings.EqualFold(tags[i].TagID, tagID) {
			return &tags[i]
		}
	}
	return nil
}

// ListGoogleTags retrieves all Google Tags configured for a site.
// It calls GET /v2/sites/{site_id}/integrations/google_tags (scope: sites:read).
func ListGoogleTags(ctx context.Context, client *http.Client, siteID string) (*GoogleTagsResponse, error) {
	var out GoogleTagsResponse
	_, err := doRequest(ctx, client, http.MethodGet,
		apiURL("/v2/sites/%s/integrations/google_tags", siteID), nil, &out, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpsertGoogleTags adds or updates Google Tags on a site.
// It calls PATCH /v2/sites/{site_id}/integrations/google_tags (scope: sites:write).
// Existing tags not referenced in the request are preserved by Webflow.
func UpsertGoogleTags(
	ctx context.Context, client *http.Client, siteID string, tags []GoogleTagEntry,
) (*GoogleTagsResponse, error) {
	var out GoogleTagsResponse
	_, err := doRequest(ctx, client, http.MethodPatch,
		apiURL("/v2/sites/%s/integrations/google_tags", siteID),
		GoogleTagsRequest{GoogleTagIDs: tags}, &out, http.StatusOK, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteGoogleTag removes a single Google Tag from a site.
// It calls DELETE /v2/sites/{site_id}/integrations/google_tags/{tag_id} (scope: sites:write).
// A 404 is treated as success so deletes are idempotent.
func DeleteGoogleTag(ctx context.Context, client *http.Client, siteID, tagID string) error {
	return doDelete(ctx, client,
		apiURL("/v2/sites/%s/integrations/google_tags/%s", siteID, url.PathEscape(tagID)), nil)
}
