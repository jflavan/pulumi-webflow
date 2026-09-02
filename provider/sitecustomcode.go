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
	"strings"
)

// siteCustomCodeIDSuffix is appended to the site ID to form a SiteCustomCode resource ID.
const siteCustomCodeIDSuffix = "/custom_code"

// GenerateSiteCustomCodeResourceID generates a Pulumi resource ID for a SiteCustomCode resource.
// Format: {siteID}/custom_code
// Note: SiteCustomCode is a 1:1 relationship with a site, so we use a simple suffix.
func GenerateSiteCustomCodeResourceID(siteID string) string {
	return siteID + siteCustomCodeIDSuffix
}

// ExtractSiteIDFromSiteCustomCodeResourceID extracts the siteID from a SiteCustomCode resource ID.
// Expected format: {siteID}/custom_code. The site ID must be non-empty.
func ExtractSiteIDFromSiteCustomCodeResourceID(resourceID string) (string, error) {
	if resourceID == "" {
		return "", errors.New("resourceId cannot be empty")
	}
	siteID, ok := strings.CutSuffix(resourceID, siteCustomCodeIDSuffix)
	if !ok || siteID == "" {
		return "", fmt.Errorf("invalid resource ID format: expected {siteId}/custom_code, got: %s", resourceID)
	}
	return siteID, nil
}

// GetSiteCustomCode retrieves all custom code scripts applied to a Webflow site.
// It calls GET /v2/sites/{site_id}/custom_code.
// A 404 is returned as an error satisfying IsNotFound.
func GetSiteCustomCode(ctx context.Context, client *http.Client, siteID string) (*CustomCodeResponse, error) {
	var out CustomCodeResponse
	if _, err := doRequest(ctx, client, http.MethodGet,
		apiURL("/v2/sites/%s/custom_code", siteID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutSiteCustomCode creates or replaces the custom code scripts applied to a Webflow site.
// It calls PUT /v2/sites/{site_id}/custom_code. The full list is sent every time; scripts
// omitted from the list are removed from the site.
func PutSiteCustomCode(
	ctx context.Context, client *http.Client, siteID string, scripts []CustomCodeScript,
) (*CustomCodeResponse, error) {
	if scripts == nil {
		scripts = []CustomCodeScript{}
	}
	var out CustomCodeResponse
	if _, err := doRequest(ctx, client, http.MethodPut,
		apiURL("/v2/sites/%s/custom_code", siteID), CustomCodeRequest{Scripts: scripts}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSiteCustomCode removes all custom code scripts from a Webflow site.
// It calls DELETE /v2/sites/{site_id}/custom_code; 404 is treated as success (idempotent).
func DeleteSiteCustomCode(ctx context.Context, client *http.Client, siteID string) error {
	return doDelete(ctx, client, apiURL("/v2/sites/%s/custom_code", siteID), nil)
}
