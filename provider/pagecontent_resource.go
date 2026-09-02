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
)

// PageContent is the resource controller for managing Webflow page content.
// It allows updating static content (text) within existing page DOM nodes.
// Note: This does NOT manage page structure/layout, only content within existing nodes.
type PageContent struct{}

// NodeContentUpdate represents a single node content update.
type NodeContentUpdate struct {
	// NodeID is the unique identifier for the DOM node to update (required).
	NodeID string `pulumi:"nodeId"`
	// Text is the new text content for the node. An empty string clears the node.
	Text string `pulumi:"text"`
}

// PageContentArgs defines the input properties for the PageContent resource.
type PageContentArgs struct {
	// PageID is the Webflow page ID to update (24-character lowercase hexadecimal string).
	PageID string `pulumi:"pageId"`
	// LocaleID optionally targets a secondary locale. When omitted Webflow updates the primary locale.
	LocaleID string `pulumi:"localeId,optional"`
	// Nodes is the list of node content updates to apply.
	Nodes []NodeContentUpdate `pulumi:"nodes"`
}

// PageContentState defines the output properties for the PageContent resource.
type PageContentState struct {
	PageContentArgs
}

// Annotate adds descriptions and constraints to the PageContent resource.
func (r *PageContent) Annotate(a infer.Annotator) {
	a.SetToken("index", "PageContent")
	a.Describe(r, "Manages static text content of a Webflow page (POST /v2/pages/{page_id}/dom). "+
		"This resource updates text within existing DOM nodes; it does NOT manage page structure or layout. "+
		"Find node IDs by fetching the page DOM (GET /v2/pages/{page_id}/dom). "+
		"Set localeId to update a secondary locale; when omitted, Webflow targets the primary locale. "+
		"Webflow reports per-node failures in the response; the update fails if any node was rejected. "+
		"\n\n**IMPORTANT LIMITATION:** This resource does NOT detect drift for content changed outside of Pulumi; "+
		"refresh only verifies that the page still exists. Destroying the resource leaves the content in place.")
}

// Annotate adds descriptions to the PageContentArgs fields.
func (args *PageContentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.PageID,
		"The Webflow page ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c4'). Use the getPages function to find page IDs.")

	a.Describe(&args.LocaleID,
		"Optional locale ID to update a secondary locale. When omitted the localeId query parameter "+
			"is not sent and Webflow updates the primary locale.")

	a.Describe(&args.Nodes,
		"List of node content updates to apply. Each entry names a nodeId from the page's DOM "+
			"and the new text (HTML allowed). Node IDs must be unique within the list.")
}

// Annotate adds descriptions to NodeContentUpdate fields.
func (ncu *NodeContentUpdate) Annotate(a infer.Annotator) {
	a.Describe(&ncu.NodeID,
		"The unique identifier for the DOM node to update. "+
			"Retrieve node IDs using GET /pages/{page_id}/dom.")

	a.Describe(&ncu.Text,
		"The new text content for the node (HTML is allowed). "+
			"An empty string clears the node's text.")
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

// validatePageContentArgs validates fully-resolved inputs at apply time.
func validatePageContentArgs(args PageContentArgs) error {
	if err := ValidatePageID(args.PageID); err != nil {
		return fmt.Errorf("validation failed for PageContent resource: %w", err)
	}
	if err := ValidateLocaleID(args.LocaleID); err != nil {
		return fmt.Errorf("validation failed for PageContent resource: %w", err)
	}
	if len(args.Nodes) == 0 {
		return errors.New("validation failed for PageContent resource: " +
			"at least one node update is required. " +
			"Please provide a list of nodes with nodeId and text fields. " +
			"Node IDs can be retrieved using GET /pages/{page_id}/dom endpoint")
	}
	for i, node := range args.Nodes {
		if err := ValidateNodeID(node.NodeID); err != nil {
			return fmt.Errorf("validation failed for PageContent resource, node[%d]: %w", i, err)
		}
	}
	if dup := findDuplicateNodeID(args.Nodes); dup != "" {
		return fmt.Errorf("validation failed for PageContent resource: nodeId '%s' appears more than once. "+
			"Each node may only be listed once", dup)
	}
	return nil
}

// Check rejects duplicate node IDs early, at preview time, in addition to the default checks.
func (r *PageContent) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[PageContentArgs], error) {
	inputs, failures, err := infer.DefaultCheck[PageContentArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[PageContentArgs]{Inputs: inputs, Failures: failures}, err
	}
	if dup := findDuplicateNodeID(inputs.Nodes); dup != "" {
		failures = append(failures, p.CheckFailure{
			Property: "nodes",
			Reason:   fmt.Sprintf("nodeId '%s' appears more than once; each node may only be listed once", dup),
		})
	}
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

// toDOMNodeUpdates converts inputs to the API request shape. Text is always sent, so an
// empty string clears the node instead of being dropped.
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
	client, err := GetHTTPClient(ctx, providerVersion)
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
	id := GeneratePageContentResourceID(req.Inputs.PageID)

	// During preview, return the inputs without calling the API. Inputs may be unknown, so
	// validation is deferred to apply time.
	if req.DryRun {
		return infer.CreateResponse[PageContentState]{ID: id, Output: state}, nil
	}

	if err := applyPageContent(ctx, req.Inputs); err != nil {
		return infer.CreateResponse[PageContentState]{}, err
	}
	return infer.CreateResponse[PageContentState]{ID: id, Output: state}, nil
}

// Read verifies the page still exists. The configured nodes are preserved from state because
// mapping the full DOM back onto the managed text values is not attempted (see resource docs).
func (r *PageContent) Read(
	ctx context.Context, req infer.ReadRequest[PageContentArgs, PageContentState],
) (infer.ReadResponse[PageContentArgs, PageContentState], error) {
	pageID, err := ExtractPageIDFromPageContentResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidatePageID(pageID); err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateLocaleID(req.State.LocaleID); err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("invalid state: %w", err)
	}

	client, err := GetHTTPClient(ctx, providerVersion)
	if err != nil {
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	if _, err := GetPageContent(ctx, client, pageID, req.State.LocaleID); err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[PageContentArgs, PageContentState]{ID: ""}, nil
		}
		return infer.ReadResponse[PageContentArgs, PageContentState]{}, fmt.Errorf("failed to read page content: %w", err)
	}

	currentInputs := PageContentArgs{
		PageID:   pageID,
		LocaleID: req.State.LocaleID,
		Nodes:    req.State.Nodes, // Preserved from state (NOT read from the API)
	}
	return infer.ReadResponse[PageContentArgs, PageContentState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  PageContentState{PageContentArgs: currentInputs},
	}, nil
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
