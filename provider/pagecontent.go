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
	"net/url"
	"strings"
)

// DOMText is the text of a text node as returned by GET /v2/pages/{page_id}/dom.
// The API returns an object {"html": ..., "text": ...}; older payloads used a plain string,
// which is accepted too and stored in both fields.
type DOMText struct {
	HTML string `json:"html,omitempty"`
	Text string `json:"text,omitempty"`
}

// UnmarshalJSON accepts either a JSON object or a bare string.
func (t *DOMText) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*t = DOMText{}
		return nil
	}
	if strings.HasPrefix(trimmed, "\"") {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*t = DOMText{HTML: s, Text: s}
		return nil
	}
	type raw DOMText
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	*t = DOMText(r)
	return nil
}

// DOMNode represents a node in the page DOM structure as returned by the Webflow API.
type DOMNode struct {
	// ID is the unique identifier for this DOM node (used as nodeId in updates).
	ID string `json:"id,omitempty"`
	// Type is the node type ("text", "image", "component-instance", "text-input", "select", ...).
	Type string `json:"type,omitempty"`
	// Text is the content of text nodes.
	Text *DOMText `json:"text,omitempty"`
	// Attributes contains the node's HTML attributes.
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	// ComponentID is set for component-instance nodes.
	ComponentID string `json:"componentId,omitempty"`
}

// PageContentResponse represents the Webflow API response for GET /pages/{page_id}/dom.
type PageContentResponse struct {
	// PageID is the unique identifier for the page.
	PageID string `json:"pageId,omitempty"`
	// BranchID is the branch the DOM belongs to, if any.
	BranchID string `json:"branchId,omitempty"`
	// Nodes is the array of DOM nodes for the page.
	Nodes []DOMNode `json:"nodes,omitempty"`
	// Pagination describes the page of nodes returned.
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Total  int `json:"total"`
	} `json:"pagination,omitempty"`
	// LastUpdated is when the DOM was last updated (nullable).
	LastUpdated string `json:"lastUpdated,omitempty"`
}

// PageContentRequest represents the request body for POST /pages/{page_id}/dom.
type PageContentRequest struct {
	// Nodes is the array of node updates to apply.
	Nodes []DOMNodeUpdate `json:"nodes"`
}

// DOMNodeUpdate represents a single text-node update in the page content request.
type DOMNodeUpdate struct {
	// NodeID is the unique identifier for the node to update (required).
	NodeID string `json:"nodeId"`
	// Text is the new content for text nodes (HTML allowed). Empty clears the node.
	Text *string `json:"text,omitempty"`
}

// PageContentUpdateResponse is the response of POST /pages/{page_id}/dom.
// A 200 with a non-empty errors list still means some nodes were not updated.
type PageContentUpdateResponse struct {
	Errors []string `json:"errors"`
}

// ValidateNodeID validates that a nodeID is non-empty.
func ValidateNodeID(nodeID string) error {
	if nodeID == "" {
		return errors.New("nodeId is required but was not provided. " +
			"Please provide a valid node ID from the page's DOM structure. " +
			"You can retrieve node IDs by fetching the page content first using the Webflow API " +
			"GET /pages/{page_id}/dom endpoint")
	}
	return nil
}

// GeneratePageContentResourceID generates a Pulumi resource ID for a PageContent resource.
// Format: {pageID}/content
func GeneratePageContentResourceID(pageID string) string {
	return pageID + "/content"
}

// ExtractPageIDFromPageContentResourceID extracts the pageID from a PageContent resource ID.
// Expected format: {pageID}/content
func ExtractPageIDFromPageContentResourceID(resourceID string) (string, error) {
	if resourceID == "" {
		return "", errors.New("resourceId cannot be empty")
	}

	suffix := "/content"
	if len(resourceID) <= len(suffix) || !strings.HasSuffix(resourceID, suffix) {
		return "", fmt.Errorf("invalid resource ID format: expected {pageId}/content, got: %s", resourceID)
	}

	pageID := strings.TrimSuffix(resourceID, suffix)
	if pageID == "" {
		return "", fmt.Errorf("invalid resource ID format: expected {pageId}/content, got: %s", resourceID)
	}

	return pageID, nil
}

// pageDOMQuery builds the optional ?localeId= query string.
func pageDOMQuery(localeID string) string {
	if localeID == "" {
		return ""
	}
	return "?localeId=" + url.QueryEscape(localeID)
}

// GetPageContent retrieves the DOM structure of a page (GET /v2/pages/{page_id}/dom).
// localeID is optional; when empty Webflow returns the primary locale.
func GetPageContent(ctx context.Context, client *http.Client, pageID, localeID string) (*PageContentResponse, error) {
	var response PageContentResponse
	u := apiURL("/v2/pages/%s/dom", pageID) + pageDOMQuery(localeID)
	if _, err := doRequest(ctx, client, http.MethodGet, u, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// PostPageContent updates static text content of a page (POST /v2/pages/{page_id}/dom).
// Webflow documents localeId as required; when localeID is empty the parameter is omitted and
// Webflow targets the primary locale. The operation fails when the response lists any errors.
func PostPageContent(
	ctx context.Context, client *http.Client, pageID, localeID string, nodes []DOMNodeUpdate,
) (*PageContentUpdateResponse, error) {
	u := apiURL("/v2/pages/%s/dom", pageID) + pageDOMQuery(localeID)
	var response PageContentUpdateResponse
	if _, err := doRequest(ctx, client, http.MethodPost, u, PageContentRequest{Nodes: nodes}, &response); err != nil {
		return nil, err
	}
	if len(response.Errors) > 0 {
		return &response, fmt.Errorf("webflow rejected %d node update(s): %s. "+
			"Verify the node IDs exist on the page (GET /v2/pages/{page_id}/dom) and are text nodes",
			len(response.Errors), strings.Join(response.Errors, "; "))
	}
	return &response, nil
}
