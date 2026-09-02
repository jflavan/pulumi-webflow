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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// PageContent is the resource controller for managing the static text content of a Webflow
// page in a secondary locale. It updates text within existing page DOM nodes.
// Note: This does NOT manage page structure/layout, only content within existing text nodes.
type PageContent struct{}

// NodeContentUpdate represents a single node content update.
type NodeContentUpdate struct {
	// NodeID is the unique identifier for the DOM node to update (required).
	NodeID string `pulumi:"nodeId"`
	// Text is the new HTML content for the node (required, non-empty).
	Text string `pulumi:"text"`
}

// PageContentArgs defines the input properties for the PageContent resource.
type PageContentArgs struct {
	// PageID is the Webflow page ID to update (24-character lowercase hexadecimal string).
	PageID string `pulumi:"pageId"`
	// LocaleID is the secondary locale whose content is updated (required by the API).
	LocaleID string `pulumi:"localeId"`
	// Nodes is the list of node content updates to apply (1 to 1000 entries).
	Nodes []NodeContentUpdate `pulumi:"nodes"`
}

// PageContentState defines the output properties for the PageContent resource.
type PageContentState struct {
	PageContentArgs
}

// Annotate adds descriptions and constraints to the PageContent resource.
func (r *PageContent) Annotate(a infer.Annotator) {
	a.SetToken("index", "PageContent")
	a.Describe(r, "Manages the static text content of a Webflow page in a secondary locale "+
		"(POST /v2/pages/{page_id}/dom?localeId=...). Webflow's Update Page Content endpoint only edits "+
		"secondary locales: localeId is required and must be a valid secondary locale of the site, and the "+
		"primary locale's content cannot be changed via the API. "+
		"This resource updates the HTML of existing text nodes; it does NOT manage page structure or layout. "+
		"Find node IDs by fetching the page DOM (GET /v2/pages/{page_id}/dom?localeId=...). "+
		"Each node's text is required, and its HTML tags must match the node's current content; "+
		"an empty text does not clear a node and is rejected. At most 1000 nodes may be updated per resource. "+
		"Webflow reports per-node failures in the response; the update fails if any node was rejected. "+
		"Refresh reads the current text of the managed nodes from the page DOM, so content changed outside "+
		"of Pulumi shows up as drift. Import with the ID {pageId}/content/{localeId} to adopt every text node "+
		"of the page. Destroying the resource leaves the content in place.")
}

// Annotate adds descriptions to the PageContentArgs fields.
func (args *PageContentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.PageID,
		"The Webflow page ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c4'). Use the getPages function to find page IDs. "+
			"Changing it replaces the resource.")

	a.Describe(&args.LocaleID,
		"The ID of the secondary locale to update (24-character lowercase hexadecimal string). Required: "+
			"the Update Page Content endpoint only edits secondary locales, and Webflow rejects the request when "+
			"the locale is the primary locale or not a locale of the site. Locale IDs are listed under "+
			"Site Settings > Localization or via the Get Site endpoint. Changing it replaces the resource.")

	a.Describe(&args.Nodes,
		"List of node content updates to apply (1 to 1000 entries). Each entry names a nodeId from the "+
			"page's DOM and the node's new HTML text. Node IDs must be unique within the list.")
}

// Annotate adds descriptions to NodeContentUpdate fields.
func (ncu *NodeContentUpdate) Annotate(a infer.Annotator) {
	a.Describe(&ncu.NodeID,
		"The unique identifier for the DOM node to update. "+
			"Retrieve node IDs using GET /v2/pages/{page_id}/dom.")

	a.Describe(&ncu.Text,
		"The new HTML content for the node (required, non-empty). The HTML tags must match the node's "+
			"current content as returned by GET /v2/pages/{page_id}/dom (e.g., '<h1>Hello</h1>' for a heading). "+
			"An empty string does not clear the node; Webflow rejects it.")
}

// findDuplicateNodeID returns the first nodeId that appears more than once, or "".
func findDuplicateNodeID(nodes []NodeContentUpdate) string {
	seen := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if node.NodeID == "" {
			continue
		}
		if _, dup := seen[node.NodeID]; dup {
			return node.NodeID
		}
		seen[node.NodeID] = struct{}{}
	}
	return ""
}

// validatePageContentNodeCount checks the 1..1000 node count limits.
func validatePageContentNodeCount(count int) error {
	if count == 0 {
		return errors.New("at least one node update is required. " +
			"Please provide a list of nodes with nodeId and text fields. " +
			"Node IDs can be retrieved using the GET /v2/pages/{page_id}/dom endpoint")
	}
	if count > maxPageContentNodes {
		return fmt.Errorf("%d nodes were provided but Webflow accepts at most %d nodes per "+
			"Update Page Content request. Split the nodes across several PageContent resources",
			count, maxPageContentNodes)
	}
	return nil
}

// validatePageContentArgs validates fully-resolved inputs at apply time.
func validatePageContentArgs(args PageContentArgs) error {
	if err := ValidatePageID(args.PageID); err != nil {
		return fmt.Errorf("validation failed for PageContent resource: %w", err)
	}
	if err := ValidatePageContentLocaleID(args.LocaleID); err != nil {
		return fmt.Errorf("validation failed for PageContent resource: %w", err)
	}
	if err := validatePageContentNodeCount(len(args.Nodes)); err != nil {
		return fmt.Errorf("validation failed for PageContent resource: %w", err)
	}
	for i, node := range args.Nodes {
		if err := ValidateNodeID(node.NodeID); err != nil {
			return fmt.Errorf("validation failed for PageContent resource, node[%d]: %w", i, err)
		}
		if err := ValidateNodeText(node.Text); err != nil {
			return fmt.Errorf("validation failed for PageContent resource, node[%d] (%s): %w", i, node.NodeID, err)
		}
	}
	if dup := findDuplicateNodeID(args.Nodes); dup != "" {
		return fmt.Errorf("validation failed for PageContent resource: nodeId '%s' appears more than once. "+
			"Each node may only be listed once", dup)
	}
	return nil
}

// checkPageContentNodes validates the known parts of the 'nodes' input at preview time:
// the node count, empty or duplicate node IDs, and empty text. Unknown values are skipped.
func checkPageContentNodes(inputs property.Map) []p.CheckFailure {
	v, ok := inputs.GetOk("nodes")
	if !ok || v.IsNull() || v.IsComputed() || !v.IsArray() {
		return nil
	}
	nodes := v.AsArray()
	if err := validatePageContentNodeCount(nodes.Len()); err != nil {
		return []p.CheckFailure{checkFailure("nodes", err)}
	}

	var failures []p.CheckFailure
	seen := make(map[string]struct{}, nodes.Len())
	for i := 0; i < nodes.Len(); i++ {
		elem := nodes.Get(i)
		if elem.IsComputed() || !elem.IsMap() {
			continue
		}
		node := elem.AsMap()
		if nodeID, known := knownString(node, "nodeId"); known {
			if err := ValidateNodeID(nodeID); err != nil {
				failures = append(failures, checkFailure(fmt.Sprintf("nodes[%d].nodeId", i), err))
			} else if _, dup := seen[nodeID]; dup {
				failures = append(failures, p.CheckFailure{
					Property: "nodes",
					Reason:   fmt.Sprintf("nodeId '%s' appears more than once; each node may only be listed once", nodeID),
				})
			} else {
				seen[nodeID] = struct{}{}
			}
		}
		if text, known := knownString(node, "text"); known {
			if err := ValidateNodeText(text); err != nil {
				failures = append(failures, checkFailure(fmt.Sprintf("nodes[%d].text", i), err))
			}
		}
	}
	return failures
}

// Check validates the known inputs at preview time: pageId and localeId formats (localeId is
// required), the node count, empty or duplicate node IDs and empty text. Unknown values are
// validated again at apply time.
func (r *PageContent) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[PageContentArgs], error) {
	inputs, failures, err := checkStrings[PageContentArgs](ctx, req.NewInputs,
		stringValidator{property: "pageId", validate: ValidatePageID},
		stringValidator{property: "localeId", validate: ValidatePageContentLocaleID},
	)
	if err != nil {
		return infer.CheckResponse[PageContentArgs]{Inputs: inputs, Failures: failures}, err
	}
	failures = append(failures, checkPageContentNodes(req.NewInputs)...)
	return infer.CheckResponse[PageContentArgs]{Inputs: inputs, Failures: failures}, nil
}

// Diff determines what changes need to be made to the page content resource.
// pageId and localeId changes trigger replacement (a different target); nodes changes update in place.
func (r *PageContent) Diff(
	ctx context.Context, req infer.DiffRequest[PageContentArgs, PageContentState],
) (infer.DiffResponse, error) {
	if req.State.PageID != req.Inputs.PageID {
		return infer.DiffResponse{
			HasChanges:   true,
			DetailedDiff: map[string]p.PropertyDiff{"pageId": {Kind: p.UpdateReplace}},
		}, nil
	}
	if req.State.LocaleID != req.Inputs.LocaleID {
		return infer.DiffResponse{
			HasChanges:   true,
			DetailedDiff: map[string]p.PropertyDiff{"localeId": {Kind: p.UpdateReplace}},
		}, nil
	}

	if !pageContentNodesEqual(req.State.Nodes, req.Inputs.Nodes) {
		return infer.DiffResponse{
			HasChanges:   true,
			DetailedDiff: map[string]p.PropertyDiff{"nodes": {Kind: p.Update}},
		}, nil
	}

	return infer.DiffResponse{}, nil
}

// pageContentNodesEqual compares node lists ignoring order.
func pageContentNodesEqual(a, b []NodeContentUpdate) bool {
	if len(a) != len(b) {
		return false
	}
	am := make(map[string]string, len(a))
	for _, n := range a {
		am[n.NodeID] = n.Text
	}
	for _, n := range b {
		text, ok := am[n.NodeID]
		if !ok || text != n.Text {
			return false
		}
	}
	return true
}

// toDOMNodeUpdates converts inputs to the API request shape.
func toDOMNodeUpdates(nodes []NodeContentUpdate) []DOMNodeUpdate {
	updates := make([]DOMNodeUpdate, len(nodes))
	for i := range nodes {
		text := nodes[i].Text
		updates[i] = DOMNodeUpdate{NodeID: nodes[i].NodeID, Text: &text}
	}
	return updates
}

// applyPageContent validates and posts the node updates.
func applyPageContent(ctx context.Context, args PageContentArgs) error {
	if err := validatePageContentArgs(args); err != nil {
		return err
	}
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	if _, err := PostPageContent(ctx, client, args.PageID, args.LocaleID, toDOMNodeUpdates(args.Nodes)); err != nil {
		return fmt.Errorf("failed to update page content: %w", err)
	}
	return nil
}

// Create applies the configured node updates to the page.
func (r *PageContent) Create(
	ctx context.Context, req infer.CreateRequest[PageContentArgs],
) (infer.CreateResponse[PageContentState], error) {
	state := PageContentState{PageContentArgs: req.Inputs}

	// During preview, return the inputs without calling the API. Inputs may be unknown, so
	// validation is deferred to apply time and no ID is reported unless both parts are known.
	if req.DryRun {
		id := ""
		if req.Inputs.PageID != "" && req.Inputs.LocaleID != "" {
			id = GeneratePageContentResourceID(req.Inputs.PageID, req.Inputs.LocaleID)
		}
		return infer.CreateResponse[PageContentState]{ID: id, Output: state}, nil
	}

	if err := applyPageContent(ctx, req.Inputs); err != nil {
		return infer.CreateResponse[PageContentState]{}, err
	}
	return infer.CreateResponse[PageContentState]{
		ID:     GeneratePageContentResourceID(req.Inputs.PageID, req.Inputs.LocaleID),
		Output: state,
	}, nil
}

// Read fetches the page DOM (GET /v2/pages/{page_id}/dom?localeId=..., paginated) and reports
// the current HTML of the managed text nodes so drift is detected. When importing (no nodes in
// state) every text node of the page is captured.
func (r *PageContent) Read(
	ctx context.Context, req infer.ReadRequest[PageContentArgs, PageContentState],
) (infer.ReadResponse[PageContentArgs, PageContentState], error) {
	pageID, localeID, err := ExtractIDsFromPageContentResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidatePageID(pageID); err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if localeID == "" {
		// Legacy {pageId}/content ID: the locale lives in state.
		localeID = req.State.LocaleID
	}
	if err := ValidatePageContentLocaleID(localeID); err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf(
			"invalid resource ID '%s': %w. Import PageContent resources with the ID {pageId}/content/{localeId}",
			req.ID, err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	textNodes, err := ListPageTextNodes(ctx, client, pageID, localeID)
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[PageContentArgs, PageContentState]{ID: ""}, nil
		}
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("failed to read page content: %w", err)
	}

	currentInputs := PageContentArgs{
		PageID:   pageID,
		LocaleID: localeID,
		Nodes:    pageContentNodesFromDOM(ctx, pageID, req.State.Nodes, textNodes),
	}
	return infer.ReadResponse[PageContentArgs, PageContentState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  PageContentState{PageContentArgs: currentInputs},
	}, nil
}

// pageContentNodesFromDOM builds the node list reported by Read. With managed nodes in state,
// each one's text is refreshed from the DOM (text nodes carry the HTML in text.html); a managed
// node missing from the DOM keeps its state value so the next update surfaces the API error.
// Without managed nodes (import), every text node of the page is captured.
func pageContentNodesFromDOM(
	ctx context.Context, pageID string, managed []NodeContentUpdate, textNodes []DOMNode,
) []NodeContentUpdate {
	htmlByID := make(map[string]string, len(textNodes))
	for _, node := range textNodes {
		html := ""
		if node.Text != nil {
			html = node.Text.HTML
		}
		htmlByID[node.ID] = html
	}

	if len(managed) == 0 {
		nodes := make([]NodeContentUpdate, 0, len(textNodes))
		for _, node := range textNodes {
			nodes = append(nodes, NodeContentUpdate{NodeID: node.ID, Text: htmlByID[node.ID]})
		}
		return nodes
	}

	nodes := make([]NodeContentUpdate, 0, len(managed))
	for _, node := range managed {
		if html, ok := htmlByID[node.NodeID]; ok {
			node.Text = html
		} else {
			NewLogContext(ctx).
				WithField("pageId", pageID).
				WithField("nodeId", node.NodeID).
				Warn("Managed node is no longer a text node of the page DOM; keeping the last known text")
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// Update re-applies the configured node updates.
func (r *PageContent) Update(
	ctx context.Context, req infer.UpdateRequest[PageContentArgs, PageContentState],
) (infer.UpdateResponse[PageContentState], error) {
	state := PageContentState{PageContentArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[PageContentState]{Output: state}, nil
	}
	if err := applyPageContent(ctx, req.Inputs); err != nil {
		return infer.UpdateResponse[PageContentState]{}, err
	}
	return infer.UpdateResponse[PageContentState]{Output: state}, nil
}

// Delete is a no-op: the content stays on the page; Pulumi simply stops managing it.
func (r *PageContent) Delete(
	ctx context.Context, req infer.DeleteRequest[PageContentState],
) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}
