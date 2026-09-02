// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// webflowBetaPathPrefix is the URL prefix of the Webflow Data API beta surface.
// The reference documentation and all generated code samples use "/beta/..." on api.webflow.com.
const webflowBetaPathPrefix = "/beta"

// maxSchemaMarkupBytes is the documented raw input limit for a single schema markup entry (60KB).
const maxSchemaMarkupBytes = 60 * 1024

// PageSchemaMarkupResponse is the response body of the page schema markup endpoints
// (GET and PUT /beta/pages/{page_id}/schema-markup).
type PageSchemaMarkupResponse struct {
	// ID is the page identifier.
	ID string `json:"id,omitempty"`
	// SiteID is the identifier of the site containing the page.
	SiteID string `json:"siteId,omitempty"`
	// LocaleID is the locale targeted by the request (primary locale when omitted).
	LocaleID *string `json:"localeId,omitempty"`
	// EffectiveLocaleID is the locale whose markup was returned; differs from LocaleID on fallback.
	EffectiveLocaleID *string `json:"effectiveLocaleId,omitempty"`
	// PublishedPath is the relative published URL path of the page.
	PublishedPath *string `json:"publishedPath,omitempty"`
	// LastUpdated is the most recent update timestamp.
	LastUpdated *string `json:"lastUpdated,omitempty"`
	// JSONLDSchema is the parsed JSON-LD object, or null when none exists.
	JSONLDSchema json.RawMessage `json:"jsonLdSchema,omitempty"`
	// RawJSONLDSchema is the raw stored markup including script tags (legacy multi-block formats only).
	RawJSONLDSchema *string `json:"rawJsonLdSchema,omitempty"`
	// IsInherited is true when a secondary locale has no override and the primary locale's markup is returned.
	IsInherited bool `json:"isInherited"`
}

// PageSchemaMarkupRequest is the request body of PUT /beta/pages/{page_id}/schema-markup.
// JSONLDSchema is sent verbatim; a JSON null clears the markup.
type PageSchemaMarkupRequest struct {
	// JSONLDSchema is the JSON-LD object to store, or null to clear.
	JSONLDSchema json.RawMessage `json:"jsonLdSchema"`
}

// NormalizeSchemaMarkup validates that markup is a JSON object and returns its canonical
// compact encoding (keys sorted, no insignificant whitespace, numbers preserved verbatim).
// Two documents that differ only in key order or whitespace normalize to the same string,
// which keeps Diff stable regardless of how the user formatted the input.
func NormalizeSchemaMarkup(markup string) (string, error) {
	trimmed := strings.TrimSpace(markup)
	if trimmed == "" {
		return "", errors.New("schemaMarkup is required but was not provided. " +
			"Provide the JSON-LD document as a JSON string, e.g. " +
			`'{"@context":"https://schema.org","@type":"Organization","name":"Acme"}'. ` +
			"Most SDKs offer JSON.stringify / json.dumps / JsonSerializer to produce this from an object")
	}
	if len(trimmed) > maxSchemaMarkupBytes {
		return "", fmt.Errorf("schemaMarkup is %d bytes, which exceeds Webflow's 60KB limit for a single "+
			"page's schema markup. Reduce the size of the JSON-LD document", len(trimmed))
	}

	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return "", fmt.Errorf("schemaMarkup is not valid JSON: %w. "+
			"Provide the JSON-LD document as a JSON string, e.g. "+
			`'{"@context":"https://schema.org","@type":"Organization","name":"Acme"}'`, err)
	}
	if dec.More() {
		return "", errors.New("schemaMarkup contains trailing data after the JSON document. " +
			"Provide exactly one JSON object")
	}
	if _, ok := value.(map[string]any); !ok {
		return "", errors.New("schemaMarkup must be a JSON object (e.g. starting with '{' and containing " +
			"'@context' and '@type'). Webflow stores a single JSON-LD object per page and locale; " +
			"to publish several entities, nest them under '@graph'")
	}

	return canonicalJSON(value)
}

// canonicalJSON encodes a decoded JSON value compactly with sorted keys and without HTML escaping.
func canonicalJSON(value any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return "", fmt.Errorf("failed to encode schema markup: %w", err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// canonicalizeRawJSON returns the canonical form of a raw JSON document, or "" for null/empty.
func canonicalizeRawJSON(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return "", fmt.Errorf("failed to parse jsonLdSchema returned by Webflow: %w", err)
	}
	return canonicalJSON(value)
}

// GeneratePageSchemaMarkupResourceID generates a Pulumi resource ID for a PageSchemaMarkup resource.
// Format: {pageID}/schema-markup or {pageID}/schema-markup/{localeID} for a secondary locale.
func GeneratePageSchemaMarkupResourceID(pageID, localeID string) string {
	if localeID == "" {
		return pageID + "/schema-markup"
	}
	return fmt.Sprintf("%s/schema-markup/%s", pageID, localeID)
}

// ExtractIDsFromPageSchemaMarkupResourceID extracts the pageID and optional localeID from a
// PageSchemaMarkup resource ID.
func ExtractIDsFromPageSchemaMarkupResourceID(resourceID string) (pageID, localeID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}
	parts := strings.Split(resourceID, "/")
	if (len(parts) != 2 && len(parts) != 3) || parts[1] != "schema-markup" || parts[0] == "" {
		return "", "", fmt.Errorf("invalid resource ID format: expected {pageId}/schema-markup "+
			"or {pageId}/schema-markup/{localeId}, got: %s", resourceID)
	}
	if len(parts) == 3 {
		if parts[2] == "" {
			return "", "", fmt.Errorf("invalid resource ID format: expected {pageId}/schema-markup "+
				"or {pageId}/schema-markup/{localeId}, got: %s", resourceID)
		}
		localeID = parts[2]
	}
	return parts[0], localeID, nil
}

// pageSchemaMarkupURL builds the schema markup endpoint URL for a page and optional locale.
func pageSchemaMarkupURL(pageID, localeID string) string {
	u := apiURL(webflowBetaPathPrefix+"/pages/%s/schema-markup", pageID)
	if localeID != "" {
		u += "?localeId=" + url.QueryEscape(localeID)
	}
	return u
}

// GetPageSchemaMarkupAPI retrieves the JSON-LD schema markup of a page.
// It calls GET /beta/pages/{page_id}/schema-markup (scope: pages:read).
func GetPageSchemaMarkupAPI(
	ctx context.Context, client *http.Client, pageID, localeID string,
) (*PageSchemaMarkupResponse, error) {
	var out PageSchemaMarkupResponse
	_, err := doRequest(ctx, client, http.MethodGet, pageSchemaMarkupURL(pageID, localeID), nil, &out, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// PutPageSchemaMarkup sets the JSON-LD schema markup of a page.
// It calls PUT /beta/pages/{page_id}/schema-markup (scope: pages:write).
// jsonLD is sent verbatim; pass nil to clear the markup (Webflow receives {"jsonLdSchema": null}).
func PutPageSchemaMarkup(
	ctx context.Context, client *http.Client, pageID, localeID string, jsonLD json.RawMessage,
) (*PageSchemaMarkupResponse, error) {
	body := PageSchemaMarkupRequest{JSONLDSchema: jsonLD}
	if len(bytes.TrimSpace(jsonLD)) == 0 {
		body.JSONLDSchema = json.RawMessage("null")
	}
	var out PageSchemaMarkupResponse
	_, err := doRequest(ctx, client, http.MethodPut, pageSchemaMarkupURL(pageID, localeID), body, &out,
		http.StatusOK, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearPageSchemaMarkup removes the JSON-LD schema markup from a page (or one of its locales)
// by sending PUT /beta/pages/{page_id}/schema-markup with {"jsonLdSchema": null}.
// Webflow has no DELETE endpoint for schema markup. A 404 is treated as success.
func ClearPageSchemaMarkup(ctx context.Context, client *http.Client, pageID, localeID string) error {
	_, err := PutPageSchemaMarkup(ctx, client, pageID, localeID, nil)
	if err != nil && IsNotFound(err) {
		return nil
	}
	return err
}

// derefString returns the string behind an optional pointer, or "".
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
