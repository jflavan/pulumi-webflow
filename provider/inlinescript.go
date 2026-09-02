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
	"unicode/utf8"
)

// InlineScriptResponse represents the Webflow API response for an inline script.
// This struct matches the Webflow API v2 response format for inline registered scripts.
type InlineScriptResponse struct {
	ID             string `json:"id,omitempty"`             // Human-readable ID derived from display name (read-only)
	DisplayName    string `json:"displayName"`              // User-facing name for the script (1-50 alphanumeric chars)
	SourceCode     string `json:"sourceCode"`               // The inline script source code
	HostedLocation string `json:"hostedLocation,omitempty"` // URI for the hosted version (read-only, set by Webflow)
	IntegrityHash  string `json:"integrityHash"`            // Sub-Resource Integrity Hash (SRI)
	CanCopy        bool   `json:"canCopy"`                  // Whether script can be copied on site duplication
	Version        string `json:"version"`                  // Semantic Version (SemVer) string
	CreatedOn      string `json:"createdOn,omitempty"`      // Timestamp when created (read-only)
	LastUpdated    string `json:"lastUpdated,omitempty"`    // Timestamp when last updated (read-only)
}

// InlineScriptRequest represents the request body for POST /registered_scripts/inline.
type InlineScriptRequest struct {
	SourceCode    string `json:"sourceCode"`
	Version       string `json:"version"`
	DisplayName   string `json:"displayName"`
	CanCopy       bool   `json:"canCopy,omitempty"`
	IntegrityHash string `json:"integrityHash,omitempty"`
}

// maxSourceCodeLength is the maximum number of characters allowed for inline script source code.
const maxSourceCodeLength = 2000

// ValidateSourceCode validates that a sourceCode value is valid for an inline script.
// Must be non-empty and at most 2000 characters (counted as Unicode code points, not bytes,
// so multi-byte characters are not penalised).
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateSourceCode(code string) error {
	if code == "" {
		return errors.New("sourceCode is required but was not provided. " +
			"Please provide the inline JavaScript code to register. " +
			"The code is limited to 2000 characters. " +
			"Example: 'console.log(\"Hello from Webflow\");'")
	}
	if n := utf8.RuneCountInString(code); n > maxSourceCodeLength {
		return fmt.Errorf("sourceCode is too long: got %d characters, maximum is %d. "+
			"Please shorten your inline script code. "+
			"If your script is too large for inline registration, consider hosting it externally "+
			"and using the RegisteredScript resource with a hostedLocation instead",
			n, maxSourceCodeLength)
	}
	return nil
}

// validateOptionalIntegrityHash accepts an empty hash (the field is optional for inline
// scripts) and otherwise applies ValidateIntegrityHash.
func validateOptionalIntegrityHash(hash string) error {
	if hash == "" {
		return nil
	}
	return ValidateIntegrityHash(hash)
}

// GenerateInlineScriptResourceID generates a Pulumi resource ID for an InlineScript resource.
// Format: {siteID}/inline_scripts/{scriptID}
func GenerateInlineScriptResourceID(siteID, scriptID string) string {
	return fmt.Sprintf("%s/inline_scripts/%s", siteID, scriptID)
}

// ExtractIDsFromInlineScriptResourceID extracts the siteID and scriptID from an InlineScript resource ID.
// Expected format: {siteID}/inline_scripts/{scriptID}. Both IDs must be non-empty.
func ExtractIDsFromInlineScriptResourceID(resourceID string) (siteID, scriptID string, err error) {
	return splitScriptResourceID(resourceID, "inline_scripts")
}

// PostInlineScript registers a new inline script on a Webflow site.
// It calls POST /v2/sites/{site_id}/registered_scripts/inline.
func PostInlineScript(
	ctx context.Context, client *http.Client, siteID string, request InlineScriptRequest,
) (*InlineScriptResponse, error) {
	var out InlineScriptResponse
	if _, err := doRequest(ctx, client, http.MethodPost,
		apiURL("/v2/sites/%s/registered_scripts/inline", siteID), request, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
