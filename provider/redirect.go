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

// RedirectRule represents a redirect configuration in Webflow.
// This struct matches the Webflow API v2 response format for redirect rules.
//
// The documented API object carries only id, fromUrl and toUrl. statusCode and createdOn are
// decoded when present so that a future API revision is picked up, but callers must not rely
// on them being set.
type RedirectRule struct {
	ID              string `json:"id,omitempty"`         // Webflow-assigned redirect ID
	SourcePath      string `json:"fromUrl"`              // Path to redirect from (e.g., "/old-page")
	DestinationPath string `json:"toUrl"`                // Path to redirect to (e.g., "/new-page")
	StatusCode      int    `json:"statusCode,omitempty"` // 301 or 302 when the API reports it
	CreatedOn       string `json:"createdOn,omitempty"`  // Creation timestamp when the API reports it
}

// RedirectPagination is the pagination block of a redirect list response.
type RedirectPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// RedirectResponse represents the Webflow API response for redirects.
type RedirectResponse struct {
	Redirects  []RedirectRule     `json:"redirects"`            // List of redirect rules
	Pagination RedirectPagination `json:"pagination,omitempty"` // Pagination info for the list
}

// RedirectRequest represents the request body for POST/PATCH redirects.
type RedirectRequest struct {
	SourcePath      string `json:"fromUrl,omitempty"`    // Path to redirect from
	DestinationPath string `json:"toUrl,omitempty"`      // Path to redirect to
	StatusCode      int    `json:"statusCode,omitempty"` // 301 or 302 (not part of the documented API; sent when set)
}

// redirectPageSize is the page size requested when listing redirects.
const redirectPageSize = 100

// pathPattern is the regex pattern for validating URL paths.
// Valid paths: start with "/" followed by alphanumeric, hyphens, underscores, slashes, dots
var pathPattern = regexp.MustCompile(`^/[a-zA-Z0-9\-_/.]*$`)

// redirectIDPattern matches Webflow redirect IDs (URL-safe identifiers, typically 24-char hex).
var redirectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ValidateSourcePath validates that a sourcePath is a valid URL path.
// Webflow redirects expect paths to start with "/" and contain only valid URL characters.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateSourcePath(path string) error {
	if path == "" {
		return errors.New("sourcePath is required but was not provided. " +
			"Please provide a valid URL path starting with '/' (e.g., '/old-page', '/blog/2023'). " +
			"Example valid paths: '/about-us', '/products/item-1', '/news/2024'")
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("sourcePath must start with '/': got '%s'. "+
			"Example valid paths: '/old-page', '/blog/2023', '/products/item-1'. "+
			"Please ensure the path begins with a forward slash", path)
	}
	if !pathPattern.MatchString(path) {
		return fmt.Errorf("sourcePath contains invalid characters: got '%s'. "+
			"Allowed characters: A-Z, a-z, 0-9, hyphens (-), underscores (_), forward slashes (/), and dots (.). "+
			"Example valid paths: '/old-page', '/blog/2023', '/products/item-1'. "+
			"Please remove any invalid characters", path)
	}
	return nil
}

// ValidateDestinationPath validates that a destinationPath is a valid URL path.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateDestinationPath(path string) error {
	if path == "" {
		return errors.New("destinationPath is required but was not provided. " +
			"Please provide a valid URL path starting with '/' (e.g., '/new-page', '/home'). " +
			"Example valid paths: '/about-us', '/products/item-1', '/news/2024'")
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("destinationPath must start with '/': got '%s'. "+
			"Example valid paths: '/new-page', '/home', '/products/item-1'. "+
			"Please ensure the path begins with a forward slash", path)
	}
	if !pathPattern.MatchString(path) {
		return fmt.Errorf("destinationPath contains invalid characters: got '%s'. "+
			"Allowed characters: A-Z, a-z, 0-9, hyphens (-), underscores (_), forward slashes (/), and dots (.). "+
			"Example valid paths: '/new-page', '/home', '/products/item-1'. "+
			"Please remove any invalid characters", path)
	}
	return nil
}

// ValidateStatusCode validates that a statusCode is either 301 or 302.
// 301 = permanent redirect, 302 = temporary redirect
// Returns actionable error messages explaining redirect types and accepted values.
func ValidateStatusCode(statusCode int) error {
	if statusCode != 301 && statusCode != 302 {
		return fmt.Errorf("statusCode must be either 301 or 302: got %d. "+
			"301 = permanent redirect (use for pages moved permanently). "+
			"302 = temporary redirect (use for temporary page moves or maintenance). "+
			"Example: statusCode=301 for permanent moves, statusCode=302 for temporary redirects", statusCode)
	}
	return nil
}

// ValidateRedirectID validates a Webflow redirect ID parsed from a resource ID before it is
// interpolated into an API URL.
func ValidateRedirectID(redirectID string) error {
	if redirectID == "" {
		return errors.New("redirectId is required but was not provided. " +
			"Expected a Webflow redirect ID (typically a 24-character hexadecimal string)")
	}
	if !redirectIDPattern.MatchString(redirectID) {
		return fmt.Errorf("redirectId has invalid format: got '%s'. "+
			"Expected a Webflow redirect ID containing only letters, digits, hyphens and underscores", redirectID)
	}
	return nil
}

// GenerateRedirectResourceID generates a Pulumi resource ID for a Redirect resource.
// Format: {siteID}/redirects/{redirectID}
func GenerateRedirectResourceID(siteID, redirectID string) string {
	return fmt.Sprintf("%s/redirects/%s", siteID, redirectID)
}

// ExtractIDsFromRedirectResourceID extracts the siteID and redirectID from a Redirect resource ID.
// Expected format: {siteID}/redirects/{redirectID}
func ExtractIDsFromRedirectResourceID(resourceID string) (siteID, redirectID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.Split(resourceID, "/")
	if len(parts) < 3 || parts[1] != "redirects" {
		return "", "", fmt.Errorf("invalid resource ID format: expected {siteId}/redirects/{redirectId}, got: %s", resourceID)
	}

	siteID = parts[0]
	redirectID = strings.Join(parts[2:], "/") // Handle redirectID that might contain slashes

	return siteID, redirectID, nil
}

// ListRedirectsPage retrieves one page of redirects for a Webflow site.
// It calls GET /v2/sites/{site_id}/redirects?limit=N&offset=M.
func ListRedirectsPage(
	ctx context.Context, client *http.Client, siteID string, limit, offset int,
) (*RedirectResponse, error) {
	var response RedirectResponse
	_, err := doRequest(ctx, client, http.MethodGet,
		apiURL("/v2/sites/%s/redirects?limit=%d&offset=%d", siteID, limit, offset), nil, &response, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// GetRedirects retrieves all redirects for a Webflow site, following pagination until the
// list is exhausted. The returned Pagination reflects the total reported by the API.
func GetRedirects(ctx context.Context, client *http.Client, siteID string) (*RedirectResponse, error) {
	all := &RedirectResponse{Redirects: []RedirectRule{}}
	err := forEachRedirectPage(ctx, client, siteID, func(page *RedirectResponse) bool {
		all.Redirects = append(all.Redirects, page.Redirects...)
		all.Pagination = page.Pagination
		return true
	})
	if err != nil {
		return nil, err
	}
	all.Pagination.Offset = 0
	all.Pagination.Limit = len(all.Redirects)
	if all.Pagination.Total < len(all.Redirects) {
		all.Pagination.Total = len(all.Redirects)
	}
	return all, nil
}

// FindRedirect looks up a single redirect by ID, paging through the site's redirect list
// only as far as needed. Returns nil, nil when the list is exhausted without a match.
func FindRedirect(ctx context.Context, client *http.Client, siteID, redirectID string) (*RedirectRule, error) {
	var found *RedirectRule
	err := forEachRedirectPage(ctx, client, siteID, func(page *RedirectResponse) bool {
		for i := range page.Redirects {
			if page.Redirects[i].ID == redirectID {
				rule := page.Redirects[i]
				found = &rule
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// forEachRedirectPage calls visit for every page of redirects until visit returns false or the
// pagination reports no more results. It stops on an empty page so an API that ignores the
// offset parameter cannot cause an infinite loop.
func forEachRedirectPage(
	ctx context.Context, client *http.Client, siteID string, visit func(*RedirectResponse) bool,
) error {
	offset := 0
	for {
		page, err := ListRedirectsPage(ctx, client, siteID, redirectPageSize, offset)
		if err != nil {
			return err
		}
		if !visit(page) {
			return nil
		}
		if len(page.Redirects) == 0 {
			return nil
		}
		offset += len(page.Redirects)
		if offset >= page.Pagination.Total {
			return nil
		}
	}
}

// PostRedirect creates a new redirect for a Webflow site.
// It calls POST /v2/sites/{site_id}/redirects endpoint.
// Returns the created redirect or an error if the request fails.
func PostRedirect(
	ctx context.Context, client *http.Client,
	siteID, sourcePath, destinationPath string, statusCode int,
) (*RedirectRule, error) {
	requestBody := RedirectRequest{
		SourcePath:      sourcePath,
		DestinationPath: destinationPath,
		StatusCode:      statusCode,
	}

	var redirect RedirectRule
	_, err := doRequest(ctx, client, http.MethodPost, apiURL("/v2/sites/%s/redirects", siteID),
		requestBody, &redirect, http.StatusOK, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	return &redirect, nil
}

// PatchRedirect updates an existing redirect for a Webflow site.
// It calls PATCH /v2/sites/{site_id}/redirects/{redirect_id}, which accepts toUrl (and fromUrl).
// The source path is treated as the redirect's identity by this provider and is not sent.
// Returns the updated redirect or an error if the request fails.
func PatchRedirect(
	ctx context.Context, client *http.Client,
	siteID, redirectID, destinationPath string, statusCode int,
) (*RedirectRule, error) {
	requestBody := RedirectRequest{
		DestinationPath: destinationPath,
		StatusCode:      statusCode,
	}

	var redirect RedirectRule
	_, err := doRequest(ctx, client, http.MethodPatch, apiURL("/v2/sites/%s/redirects/%s", siteID, redirectID),
		requestBody, &redirect, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &redirect, nil
}

// DeleteRedirect removes a redirect from a Webflow site.
// It calls DELETE /v2/sites/{site_id}/redirects/{redirect_id} endpoint.
// Returns nil on success (including 404 for idempotency) or an error if the request fails.
func DeleteRedirect(ctx context.Context, client *http.Client, siteID, redirectID string) error {
	return doDelete(ctx, client, apiURL("/v2/sites/%s/redirects/%s", siteID, redirectID), nil)
}
