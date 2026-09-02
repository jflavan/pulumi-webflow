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

// ValidateScriptID validates that a scriptID is a non-empty string.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateScriptID(scriptID string) error {
	if scriptID == "" {
		return errors.New("script id is required but was not provided. " +
			"Please provide the ID of a registered custom code script. " +
			"Scripts must be registered first using the RegisteredScript resource " +
			"before they can be applied to a page")
	}
	return nil
}

// ValidateScriptVersion validates that a scriptVersion is a valid semantic version.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateScriptVersion(version string) error {
	if version == "" {
		return errors.New("script version is required but was not provided. " +
			"Please provide a semantic version string (e.g., '1.0.0'). " +
			"Version must match a registered version of the script")
	}
	return nil
}

// ValidateScriptLocation validates that a scriptLocation is either "header" or "footer".
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateScriptLocation(location string) error {
	if location != "header" && location != "footer" {
		return fmt.Errorf("script location must be either 'header' or 'footer': got '%s'. "+
			"'header' = script loads in the page header (recommended for performance). "+
			"'footer' = script loads in the page footer (use for DOM-dependent scripts). "+
			"Please specify one of these two values", location)
	}
	return nil
}

// validateCustomCodeScripts validates every script in a SiteCustomCode or PageCustomCode input list.
// The resource name is used in error messages.
func validateCustomCodeScripts[T customCodeScriptInput](resource string, scripts []T) error {
	for i, s := range scripts {
		script := CustomCodeScript(s)
		if err := ValidateScriptID(script.ID); err != nil {
			return fmt.Errorf("validation failed for %s resource at scripts[%d]: %w", resource, i, err)
		}
		if err := ValidateScriptVersion(script.Version); err != nil {
			return fmt.Errorf("validation failed for %s resource at scripts[%d]: %w", resource, i, err)
		}
		if err := ValidateScriptLocation(script.Location); err != nil {
			return fmt.Errorf("validation failed for %s resource at scripts[%d]: %w", resource, i, err)
		}
	}
	return nil
}

// pageCustomCodeIDSuffix is appended to the page ID to form a PageCustomCode resource ID.
const pageCustomCodeIDSuffix = "/custom-code"

// GeneratePageCustomCodeResourceID generates a Pulumi resource ID for a PageCustomCode resource.
// Format: {pageID}/custom-code
// Note: PageCustomCode is a 1:1 relationship with a page, so we use a simple suffix.
func GeneratePageCustomCodeResourceID(pageID string) string {
	return pageID + pageCustomCodeIDSuffix
}

// ExtractPageIDFromPageCustomCodeResourceID extracts the pageID from a PageCustomCode resource ID.
// Expected format: {pageID}/custom-code. The page ID must be non-empty.
func ExtractPageIDFromPageCustomCodeResourceID(resourceID string) (string, error) {
	if resourceID == "" {
		return "", errors.New("resourceId cannot be empty")
	}
	pageID, ok := strings.CutSuffix(resourceID, pageCustomCodeIDSuffix)
	if !ok || pageID == "" {
		return "", fmt.Errorf("invalid resource ID format: expected {pageId}/custom-code, got: %s", resourceID)
	}
	return pageID, nil
}

// GetPageCustomCode retrieves all custom code scripts applied to a page.
// It calls GET /v2/pages/{page_id}/custom_code.
// A 404 is returned as an error satisfying IsNotFound.
func GetPageCustomCode(ctx context.Context, client *http.Client, pageID string) (*CustomCodeResponse, error) {
	var out CustomCodeResponse
	if _, err := doRequest(ctx, client, http.MethodGet,
		apiURL("/v2/pages/%s/custom_code", pageID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutPageCustomCode replaces the custom code scripts applied to a page.
// It calls PUT /v2/pages/{page_id}/custom_code with a JSON body. The full list is sent
// every time; scripts omitted from the list are removed from the page.
func PutPageCustomCode(
	ctx context.Context, client *http.Client, pageID string, scripts []CustomCodeScript,
) (*CustomCodeResponse, error) {
	if scripts == nil {
		scripts = []CustomCodeScript{}
	}
	var out CustomCodeResponse
	if _, err := doRequest(ctx, client, http.MethodPut,
		apiURL("/v2/pages/%s/custom_code", pageID), CustomCodeRequest{Scripts: scripts}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeletePageCustomCode removes all custom code scripts from a page.
// It calls DELETE /v2/pages/{page_id}/custom_code; 404 is treated as success (idempotent).
func DeletePageCustomCode(ctx context.Context, client *http.Client, pageID string) error {
	return doDelete(ctx, client, apiURL("/v2/pages/%s/custom_code", pageID), nil)
}
