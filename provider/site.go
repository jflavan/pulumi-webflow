// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

// Site represents a Webflow site configuration.
// This struct maps to the Webflow API v2 Site object.
type Site struct {
	// ID is the unique identifier for the site (read-only).
	ID string `json:"id,omitempty"`
	// WorkspaceID is the workspace that contains this site (read-only).
	WorkspaceID string `json:"workspaceId,omitempty"`
	// DisplayName is the human-readable name of the site.
	DisplayName string `json:"displayName"`
	// ShortName is the slugified version of the site name (lowercase alphanumeric with hyphens).
	ShortName string `json:"shortName,omitempty"`
	// TimeZone is the IANA timezone identifier for the site.
	TimeZone string `json:"timeZone,omitempty"`
	// LastPublished is the timestamp of the last site publish (read-only).
	LastPublished string `json:"lastPublished,omitempty"`
	// LastUpdated is the timestamp of the last site update (read-only).
	LastUpdated string `json:"lastUpdated,omitempty"`
	// PreviewURL is the URL to a preview image of the site (read-only).
	PreviewURL string `json:"previewUrl,omitempty"`
	// ParentFolderID is the folder where the site is organized (optional).
	ParentFolderID string `json:"parentFolderId,omitempty"`
	// CustomDomains is the list of custom domains attached to the site (read-only for now).
	// The API returns domain objects ({id, url}); older mocks return plain strings. Both decode.
	CustomDomains SiteCustomDomains `json:"customDomains,omitempty"`
	// DataCollectionEnabled indicates if data collection is enabled for the site (read-only).
	DataCollectionEnabled bool `json:"dataCollectionEnabled,omitempty"`
	// DataCollectionType is the type of data collection enabled (read-only).
	DataCollectionType string `json:"dataCollectionType,omitempty"`
}

// SiteCustomDomains is the list of custom domain URLs attached to a site.
// The Webflow API returns custom domains as objects ({"id": "...", "url": "example.com"}); this
// type also accepts a plain string array so that either representation decodes to the URL list.
type SiteCustomDomains []string

// UnmarshalJSON accepts either ["example.com"] or [{"id": "...", "url": "example.com"}].
func (d *SiteCustomDomains) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			out = append(out, s)
			continue
		}
		var obj struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		}
		if err := json.Unmarshal(item, &obj); err != nil {
			return fmt.Errorf("customDomains: unexpected element %s", string(item))
		}
		if obj.URL != "" {
			out = append(out, obj.URL)
		} else {
			out = append(out, obj.ID)
		}
	}
	*d = out
	return nil
}

// SiteResponse represents the API response structure for site list/get operations.
type SiteResponse struct {
	Sites []Site `json:"sites,omitempty"`
}

// SiteCreateRequest represents the request body for creating a new site.
// Note: The Webflow API uses "name" in the request, but returns "displayName" in the response.
type SiteCreateRequest struct {
	// Name is the name of the site (maps to displayName in response).
	Name string `json:"name"`
	// TemplateName is the optional template to use for site creation.
	TemplateName string `json:"templateName,omitempty"`
	// ParentFolderID is the optional folder where the site will be organized.
	ParentFolderID string `json:"parentFolderId,omitempty"`
}

// SiteUpdateRequest represents the request body for PATCH /v2/sites/{site_id}.
// The API accepts "name" (not "displayName") and a nullable "parentFolderId".
//
// ParentFolderID has three states:
//   - nil: the field is omitted from the request (leave the folder unchanged)
//   - pointer to "": the field is sent as JSON null (move the site back to the workspace root)
//   - pointer to a value: the field is sent as that folder ID
//
// Note: shortName and timeZone are read-only and cannot be set via the API.
type SiteUpdateRequest struct {
	Name           string
	ParentFolderID *string
}

// MarshalJSON encodes the request, emitting an explicit null for a cleared parentFolderId.
func (r SiteUpdateRequest) MarshalJSON() ([]byte, error) {
	body := make(map[string]any, 2)
	if r.Name != "" {
		body["name"] = r.Name
	}
	if r.ParentFolderID != nil {
		if *r.ParentFolderID == "" {
			body["parentFolderId"] = nil
		} else {
			body["parentFolderId"] = *r.ParentFolderID
		}
	}
	return json.Marshal(body)
}

// SitePublishRequest represents the request body for POST /v2/sites/{site_id}/publish.
// See https://developers.webflow.com/data/reference/sites/publish
type SitePublishRequest struct {
	// CustomDomains is the list of custom domain IDs to publish to.
	CustomDomains []string `json:"customDomains,omitempty"`
	// PublishToWebflowSubdomain publishes to the site's default webflow.io subdomain.
	PublishToWebflowSubdomain bool `json:"publishToWebflowSubdomain"`
	// PageID limits the publish to a single page (added to the API in April 2026).
	PageID string `json:"pageId,omitempty"`
}

// SitePublishedDomain is one custom domain entry in a publish response.
type SitePublishedDomain struct {
	ID            string `json:"id,omitempty"`
	URL           string `json:"url,omitempty"`
	LastPublished string `json:"lastPublished,omitempty"`
}

// SitePublishResponse represents the API response from publishing a site (202 Accepted).
type SitePublishResponse struct {
	CustomDomains             []SitePublishedDomain `json:"customDomains,omitempty"`
	PublishToWebflowSubdomain bool                  `json:"publishToWebflowSubdomain,omitempty"`
	// PublishScope is "site" or "page" depending on whether a single page was published.
	PublishScope string `json:"publishScope,omitempty"`
}

// ValidateDisplayName validates that displayName meets Webflow requirements.
// Actionable error messages explain: what's wrong, expected format, and how to fix it.
func ValidateDisplayName(displayName string) error {
	if displayName == "" {
		return errors.New("displayName is required but was not provided. " +
			"Expected format: A non-empty string representing your site's name. " +
			"Fix: Provide a name for your site (e.g., 'My Marketing Site', 'Company Blog', 'Product Landing Page')")
	}

	// Webflow site names typically have a practical length limit
	if len(displayName) > 255 {
		return fmt.Errorf("displayName is too long: '%s' exceeds maximum length of 255 characters. "+
			"Expected format: A string with 1-255 characters. "+
			"Fix: Use a shorter, more concise site name", displayName)
	}

	return nil
}

// ValidateShortName validates that shortName meets Webflow's slug requirements.
// Webflow's shortName must be lowercase alphanumeric with hyphens, no leading/trailing hyphens.
// If shortName is empty, that's OK - Webflow will generate one from displayName.
// Actionable error messages explain: what's wrong, expected format, and how to fix it.
func ValidateShortName(shortName string) error {
	// shortName is optional - if empty, Webflow will auto-generate from displayName
	if shortName == "" {
		return nil
	}

	// Webflow shortName must be lowercase alphanumeric with hyphens only
	// Pattern: start with letter/number, can have hyphens in middle, end with letter/number
	shortNameRegex := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	if !shortNameRegex.MatchString(shortName) {
		return fmt.Errorf("invalid shortName format: '%s' contains invalid characters. "+
			"Expected format: lowercase letters (a-z), numbers (0-9), and hyphens (-) only. "+
			"Must start and end with a letter or number (e.g., 'my-site', 'company-blog-2024', 'product-1'). "+
			"Fix: Use only lowercase letters, numbers, and hyphens. No spaces, underscores, or special characters. "+
			"No leading/trailing hyphens", shortName)
	}

	return nil
}

// ValidateWorkspaceID validates that workspaceID is a non-empty string.
// Workspace IDs are required for site creation via the Webflow API.
// Actionable error messages explain: what's wrong, expected format, and how to fix it.
func ValidateWorkspaceID(workspaceID string) error {
	if workspaceID == "" {
		return errors.New("workspaceId is required but was not provided. " +
			"Expected format: Your Webflow workspace ID (a 24-character hexadecimal string). " +
			"Fix: Provide your workspace ID. You can find it in your Webflow dashboard under Account Settings > Workspace. " +
			"Note: Creating sites via API requires an Enterprise workspace")
	}

	return nil
}

// PostSite creates a new site in the specified Webflow workspace.
// Enterprise workspace is required for site creation via API.
// Note: API request uses "name" but response returns "displayName".
// Returns the created Site or an error if the request fails.
func PostSite(
	ctx context.Context, client *http.Client,
	workspaceID, displayName, parentFolderID, templateName string,
) (*Site, error) {
	requestBody := SiteCreateRequest{
		Name:           displayName,
		TemplateName:   templateName,   // Optional, empty string OK
		ParentFolderID: parentFolderID, // Optional, empty string OK
	}

	var site Site
	_, err := doRequest(ctx, client, http.MethodPost, apiURL("/v2/workspaces/%s/sites", workspaceID),
		requestBody, &site, http.StatusOK, http.StatusCreated, http.StatusAccepted)
	if err != nil {
		return nil, err
	}
	return &site, nil
}

// PatchSite updates an existing site's configuration via PATCH /v2/sites/{site_id}.
//
// parentFolderID is nullable: nil leaves the folder untouched, a pointer to "" sends JSON null
// to move the site back to the workspace root, and a pointer to a value moves it to that folder.
// Note: shortName and timeZone are read-only and cannot be updated via the API.
// Returns the updated Site or an error if the request fails.
func PatchSite(
	ctx context.Context, client *http.Client,
	siteID, displayName string, parentFolderID *string,
) (*Site, error) {
	requestBody := SiteUpdateRequest{
		Name:           displayName,
		ParentFolderID: parentFolderID,
	}

	var site Site
	_, err := doRequest(ctx, client, http.MethodPatch, apiURL("/v2/sites/%s", siteID),
		requestBody, &site, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &site, nil
}

// PublishSite publishes a site (or a single page) via POST /v2/sites/{site_id}/publish.
// The operation is asynchronous: the API answers 202 Accepted and completes the publish later.
// Progress can be observed through the lastPublished timestamp on subsequent reads.
func PublishSite(
	ctx context.Context, client *http.Client, siteID string, request SitePublishRequest,
) (*SitePublishResponse, error) {
	var publishResp SitePublishResponse
	_, err := doRequest(ctx, client, http.MethodPost, apiURL("/v2/sites/%s/publish", siteID),
		request, &publishResp, http.StatusOK, http.StatusAccepted)
	if err != nil {
		return nil, err
	}
	return &publishResp, nil
}

// DeleteSite permanently deletes a site from Webflow.
// This operation cannot be undone - the site and all its content will be permanently removed.
// Returns nil on success (204 No Content), or an error if the request fails.
// Note: 404 responses are treated as success (idempotent - site already deleted).
func DeleteSite(ctx context.Context, client *http.Client, siteID string) error {
	return doDelete(ctx, client, apiURL("/v2/sites/%s", siteID), nil)
}

// GetSite retrieves the current state of a site from Webflow.
// Returns the site data if successful, or an error if the request fails.
// Note: Returns nil, nil (not an error) when the site is not found (404) - the caller handles that.
func GetSite(ctx context.Context, client *http.Client, siteID string) (*Site, error) {
	var site Site
	_, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/sites/%s", siteID), nil, &site, http.StatusOK)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}
