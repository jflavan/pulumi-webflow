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
	"strconv"
	"strings"
)

// PageSEO holds the SEO title and description of a page.
type PageSEO struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// PageOpenGraph holds the Open Graph settings of a page.
type PageOpenGraph struct {
	Title             string `json:"title,omitempty"`
	TitleCopied       bool   `json:"titleCopied"`
	Description       string `json:"description,omitempty"`
	DescriptionCopied bool   `json:"descriptionCopied"`
}

// Page represents a single page in a Webflow site.
// This struct matches the Webflow API v2 response format for pages.
type Page struct {
	// ID is the unique identifier for the page (24-character hex string).
	ID string `json:"id"`
	// SiteID is the Webflow site ID this page belongs to.
	SiteID string `json:"siteId"`
	// Title is the page title (appears in browser tabs and search results).
	Title string `json:"title"`
	// Slug is the URL slug for the page (e.g., "about" for "/about").
	Slug string `json:"slug"`
	// ParentID is the ID of the parent folder (optional).
	ParentID string `json:"parentId,omitempty"`
	// CollectionID is the ID of the CMS collection (for collection template pages, optional).
	CollectionID string `json:"collectionId,omitempty"`
	// CreatedOn is the timestamp when the page was created.
	CreatedOn string `json:"createdOn"`
	// LastUpdated is the timestamp when the page was last updated.
	LastUpdated string `json:"lastUpdated"`
	// Archived indicates if the page is archived.
	Archived bool `json:"archived"`
	// Draft indicates if the page is in draft mode.
	Draft bool `json:"draft"`
	// CanBranch indicates if the page can be branched (read-only).
	CanBranch bool `json:"canBranch,omitempty"`
	// IsBranch indicates whether the page is a branch of another page.
	IsBranch bool `json:"isBranch,omitempty"`
	// BranchID is the ID of the parent branch, if any.
	BranchID string `json:"branchId,omitempty"`
	// SEO holds the SEO title and description.
	SEO *PageSEO `json:"seo,omitempty"`
	// OpenGraph holds the Open Graph settings.
	OpenGraph *PageOpenGraph `json:"openGraph,omitempty"`
	// LocaleID is the locale of the returned page data (nullable).
	LocaleID string `json:"localeId,omitempty"`
	// PublishedPath is the relative URL path of the published page.
	PublishedPath string `json:"publishedPath,omitempty"`
}

// PagesResponse represents the Webflow API response for GET /sites/{site_id}/pages.
type PagesResponse struct {
	// Pages is the list of pages in the site.
	Pages []Page `json:"pages"`
	// Pagination describes the page of results returned.
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Total  int `json:"total"`
	} `json:"pagination,omitempty"`
}

// PageSEOUpdate is the seo object of a PUT /v2/pages/{page_id} body; nil fields are omitted.
type PageSEOUpdate struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

// PageOpenGraphUpdate is the openGraph object of a PUT /v2/pages/{page_id} body; nil fields are omitted.
type PageOpenGraphUpdate struct {
	Title             *string `json:"title,omitempty"`
	TitleCopied       *bool   `json:"titleCopied,omitempty"`
	Description       *string `json:"description,omitempty"`
	DescriptionCopied *bool   `json:"descriptionCopied,omitempty"`
}

// PageMetadataUpdateRequest is the body of PUT /v2/pages/{page_id}. Only non-nil fields are sent.
type PageMetadataUpdateRequest struct {
	Title     *string              `json:"title,omitempty"`
	Slug      *string              `json:"slug,omitempty"`
	SEO       *PageSEOUpdate       `json:"seo,omitempty"`
	OpenGraph *PageOpenGraphUpdate `json:"openGraph,omitempty"`
}

// pageIDPattern is the regex pattern for validating Webflow page IDs.
// Page IDs are 24-character lowercase hexadecimal strings (same format as site IDs).
var pageIDPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

// ValidatePageID validates that a pageID matches the Webflow page ID format.
// Webflow page IDs are 24-character lowercase hexadecimal strings.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidatePageID(pageID string) error {
	if pageID == "" {
		return errors.New("pageId is required but was not provided. " +
			"Please provide a valid Webflow page ID " +
			"(24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c4'). " +
			"You can find page IDs using the getPages function or in the Webflow designer")
	}
	if !pageIDPattern.MatchString(pageID) {
		return fmt.Errorf("pageId has invalid format: got '%s'. "+
			"Expected a 24-character lowercase hexadecimal string "+
			"(e.g., '5f0c8c9e1c9d440000e8d8c4'). "+
			"Please check your page ID and ensure it contains only "+
			"lowercase letters (a-f) and digits (0-9)", pageID)
	}
	return nil
}

// ValidateLocaleID validates an optional Webflow locale ID (24-character lowercase hex).
// An empty value is allowed and means "primary locale".
func ValidateLocaleID(localeID string) error {
	if localeID == "" {
		return nil
	}
	if !pageIDPattern.MatchString(localeID) {
		return fmt.Errorf("localeId has invalid format: got '%s'. "+
			"Expected a 24-character lowercase hexadecimal string (e.g., '653fd9af6a07fc9cfd7a5e57'). "+
			"Locale IDs are listed under Site Settings > Localization or via the Get Site endpoint", localeID)
	}
	return nil
}

// GeneratePageResourceID generates a Pulumi resource ID in the form {siteID}/pages/{pageID}.
func GeneratePageResourceID(siteID, pageID string) string {
	return fmt.Sprintf("%s/pages/%s", siteID, pageID)
}

// ExtractIDsFromPageResourceID extracts the siteID and pageID from a {siteID}/pages/{pageID} ID.
func ExtractIDsFromPageResourceID(resourceID string) (siteID, pageID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	idx := strings.Index(resourceID, "/pages/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid resource ID format: expected {siteId}/pages/{pageId}, got: %s", resourceID)
	}

	siteID = resourceID[:idx]
	pageID = resourceID[idx+len("/pages/"):]

	if siteID == "" || pageID == "" {
		return "", "", fmt.Errorf("invalid resource ID format: expected {siteId}/pages/{pageId}, got: %s", resourceID)
	}

	return siteID, pageID, nil
}

// listPagesPageSize is the maximum page size accepted by GET /v2/sites/{site_id}/pages.
const listPagesPageSize = 100

// ListPages retrieves every page of a Webflow site, following pagination.
// It calls GET /v2/sites/{site_id}/pages with ?localeId= when localeID is set.
func ListPages(ctx context.Context, client *http.Client, siteID, localeID string) ([]Page, error) {
	var all []Page
	offset := 0
	for {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(listPagesPageSize))
		query.Set("offset", strconv.Itoa(offset))
		if localeID != "" {
			query.Set("localeId", localeID)
		}

		var response PagesResponse
		u := apiURL("/v2/sites/%s/pages", siteID) + "?" + query.Encode()
		if _, err := doRequest(ctx, client, http.MethodGet, u, nil, &response); err != nil {
			return nil, err
		}
		all = append(all, response.Pages...)

		// Stop when the server returned fewer than a full page, or we have reached the total.
		if len(response.Pages) == 0 || len(response.Pages) < listPagesPageSize {
			break
		}
		offset += len(response.Pages)
		if response.Pagination.Total > 0 && offset >= response.Pagination.Total {
			break
		}
	}
	if all == nil {
		all = []Page{}
	}
	return all, nil
}

// GetPageMetadata retrieves a single page by ID (GET /v2/pages/{page_id}).
// localeID selects a locale. translatable is the ID of the secondary locale being translated
// into and is sent verbatim as ?translatable=<localeId>: Webflow answers 400 when it is the
// primary locale ID or any other value, and 403 when translation exclusions are disabled for
// the site (August 2026 API addition).
func GetPageMetadata(
	ctx context.Context, client *http.Client, pageID, localeID, translatable string,
) (*Page, error) {
	query := url.Values{}
	if localeID != "" {
		query.Set("localeId", localeID)
	}
	if translatable != "" {
		query.Set("translatable", translatable)
	}
	u := apiURL("/v2/pages/%s", pageID)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var page Page
	if _, err := doRequest(ctx, client, http.MethodGet, u, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// PutPageMetadata updates page settings (PUT /v2/pages/{page_id}?localeId=...).
// Only the non-nil fields of body are sent. Note that Webflow silently ignores slug for the
// home page, collection template pages, utility pages, and for secondary locales without an
// Advanced/Enterprise localization plan; compare the returned slug with the requested one.
func PutPageMetadata(
	ctx context.Context, client *http.Client, pageID, localeID string, body PageMetadataUpdateRequest,
) (*Page, error) {
	u := apiURL("/v2/pages/%s", pageID)
	if localeID != "" {
		u += "?localeId=" + url.QueryEscape(localeID)
	}
	var page Page
	if _, err := doRequest(ctx, client, http.MethodPut, u, body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}
