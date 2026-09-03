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
	"strconv"
	"strings"
)

// maxPageContentNodes is the documented maximum number of nodes one
// POST /v2/pages/{page_id}/dom request may update.
const maxPageContentNodes = 1000

// pageDOMPageSize is the page size used when reading the DOM (GET /v2/pages/{page_id}/dom
// accepts limit up to 100).
const pageDOMPageSize = 100

// domNodeTypeText is the DOM node type whose content the Update Page Content endpoint edits.
const domNodeTypeText = "text"

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

// PageContentResponse represents the Webflow API response for GET /v2/pages/{page_id}/dom.
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

// PageContentRequest represents the request body for POST /v2/pages/{page_id}/dom.
type PageContentRequest struct {
	// Nodes is the array of node updates to apply.
	Nodes []DOMNodeUpdate `json:"nodes"`
}

// DOMNodeUpdate represents a single text-node update in the page content request.
type DOMNodeUpdate struct {
	// NodeID is the unique identifier for the node to update (required).
	NodeID string `json:"nodeId"`
	// Text is the new HTML content for text nodes (required by the API; its tags must match the
	// node's current content as returned by GET /v2/pages/{page_id}/dom).
	Text *string `json:"text,omitempty"`
}

// PageContentUpdateResponse is the response of POST /v2/pages/{page_id}/dom.
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
			"GET /v2/pages/{page_id}/dom endpoint")
	}
	return nil
}

// ValidateNodeText validates the replacement text of a node. Webflow requires the text (HTML)
// to be present; an empty string does not clear a node, it is rejected.
func ValidateNodeText(text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("text is required but was empty. " +
			"Webflow does not clear a node when text is empty; provide the node's new HTML content, " +
			"using the same tags the node currently has (see GET /v2/pages/{page_id}/dom)")
	}
	return nil
}

// ValidatePageContentLocaleID validates the locale ID of a PageContent resource, which is
// required: the Update Page Content endpoint only edits secondary locales.
func ValidatePageContentLocaleID(localeID string) error {
	if localeID == "" {
		return errors.New("localeId is required but was not provided. " +
			"Webflow's Update Page Content endpoint only edits static content in a secondary locale; " +
			"the primary locale's content cannot be changed via the API. " +
			"Provide the ID of a secondary locale (24-character lowercase hexadecimal string, listed under " +
			"Site Settings > Localization or via the Get Site endpoint)")
	}
	return ValidateLocaleID(localeID)
}

// GeneratePageContentResourceID generates a Pulumi resource ID for a PageContent resource.
// Format: {pageID}/content/{localeID}
func GeneratePageContentResourceID(pageID, localeID string) string {
	return pageID + "/content/" + localeID
}

// ExtractIDsFromPageContentResourceID extracts the pageID and localeID from a PageContent
// resource ID. The current format is {pageID}/content/{localeID}; the legacy {pageID}/content
// form (from before localeId was required) is accepted and returns an empty localeID, which
// callers fill from state.
func ExtractIDsFromPageContentResourceID(resourceID string) (pageID, localeID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}
	parts := strings.Split(resourceID, "/")
	if (len(parts) != 2 && len(parts) != 3) || parts[1] != "content" || parts[0] == "" {
		return "", "", fmt.Errorf(
			"invalid resource ID format: expected {pageId}/content/{localeId}, got: %s", resourceID)
	}
	if len(parts) == 3 {
		if parts[2] == "" {
			return "", "", fmt.Errorf(
				"invalid resource ID format: expected {pageId}/content/{localeId}, got: %s", resourceID)
		}
		localeID = parts[2]
	}
	return parts[0], localeID, nil
}

// pageDOMQuery builds the query string for the DOM endpoints. localeID is added when set;
// limit and offset are added when limit is positive.
func pageDOMQuery(localeID string, limit, offset int) string {
	q := url.Values{}
	if localeID != "" {
		q.Set("localeId", localeID)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// GetPageContent retrieves one page of the DOM structure of a page (GET /v2/pages/{page_id}/dom).
// localeID is optional; when empty Webflow returns the primary locale. No pagination
// parameters are sent, so Webflow applies its defaults; use ListPageTextNodes to read every node.
func GetPageContent(ctx context.Context, client *http.Client, pageID, localeID string) (*PageContentResponse, error) {
	return getPageContentPage(ctx, client, pageID, localeID, 0, 0)
}

// getPageContentPage retrieves one page of DOM nodes with explicit pagination.
func getPageContentPage(
	ctx context.Context, client *http.Client, pageID, localeID string, limit, offset int,
) (*PageContentResponse, error) {
	var response PageContentResponse
	u := apiURL("/v2/pages/%s/dom", pageID) + pageDOMQuery(localeID, limit, offset)
	if _, err := doRequest(ctx, client, http.MethodGet, u, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ListPageTextNodes retrieves every text node of a page's DOM, following pagination with
// limit 100. Text nodes are the only nodes the Update Page Content endpoint can edit.
func ListPageTextNodes(ctx context.Context, client *http.Client, pageID, localeID string) ([]DOMNode, error) {
	nodes := []DOMNode{}
	offset := 0
	for {
		page, err := getPageContentPage(ctx, client, pageID, localeID, pageDOMPageSize, offset)
		if err != nil {
			return nil, err
		}
		for _, node := range page.Nodes {
			if node.Type == domNodeTypeText {
				nodes = append(nodes, node)
			}
		}
		// Stop when the server returned fewer than a full page, or we have reached the total.
		if len(page.Nodes) < pageDOMPageSize {
			break
		}
		offset += len(page.Nodes)
		if page.Pagination.Total > 0 && offset >= page.Pagination.Total {
			break
		}
	}
	return nodes, nil
}

// PostPageContent updates static text content of a page in a secondary locale
// (POST /v2/pages/{page_id}/dom?localeId=...). localeId is required by Webflow and must be a
// secondary locale of the site; the primary locale cannot be edited via the API. The operation
// fails when the response lists any errors.
func PostPageContent(
	ctx context.Context, client *http.Client, pageID, localeID string, nodes []DOMNodeUpdate,
) (*PageContentUpdateResponse, error) {
	if err := ValidatePageContentLocaleID(localeID); err != nil {
		return nil, err
	}
	if len(nodes) > maxPageContentNodes {
		return nil, fmt.Errorf("cannot update %d nodes in one request: Webflow accepts at most %d nodes "+
			"per Update Page Content request. Split the nodes across several PageContent resources",
			len(nodes), maxPageContentNodes)
	}
	u := apiURL("/v2/pages/%s/dom", pageID) + pageDOMQuery(localeID, 0, 0)
	var response PageContentUpdateResponse
	if _, err := doRequest(ctx, client, http.MethodPost, u, PageContentRequest{Nodes: nodes}, &response); err != nil {
		return nil, err
	}
	if len(response.Errors) > 0 {
		return &response, fmt.Errorf("webflow rejected %d node update(s): %s. "+
			"Verify the node IDs exist on the page (GET /v2/pages/{page_id}/dom), are text nodes, and that "+
			"the new text uses the same HTML tags as the current content",
			len(response.Errors), strings.Join(response.Errors, "; "))
	}
	return &response, nil
}
