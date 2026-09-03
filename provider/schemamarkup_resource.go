// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// PageSchemaMarkup is the resource controller for the JSON-LD schema markup of a Webflow page.
// It implements the infer.CustomResource interface for full CRUD operations.
//
// This resource "adopts" a page-scoped setting: Create and Update both PUT the markup, and
// Delete clears it by PUTting null, because Webflow has no DELETE endpoint for schema markup.
type PageSchemaMarkup struct{}

// PageSchemaMarkupArgs defines the input properties for the PageSchemaMarkup resource.
type PageSchemaMarkupArgs struct {
	// PageID is the Webflow page ID (24-character lowercase hexadecimal string).
	PageID string `pulumi:"pageId"`
	// LocaleID optionally targets a secondary locale. Omit for the primary locale.
	LocaleID string `pulumi:"localeId,optional"`
	// SchemaMarkup is the JSON-LD document as a JSON string.
	SchemaMarkup string `pulumi:"schemaMarkup"`
}

// PageSchemaMarkupState defines the output properties for the PageSchemaMarkup resource.
// It embeds PageSchemaMarkupArgs so inputs are also available as outputs.
type PageSchemaMarkupState struct {
	PageSchemaMarkupArgs
	// SiteID is the identifier of the site containing the page.
	SiteID string `pulumi:"siteId,optional"`
	// EffectiveLocaleID is the locale whose markup Webflow reports.
	EffectiveLocaleID string `pulumi:"effectiveLocaleId,optional"`
	// PublishedPath is the relative published URL path of the page.
	PublishedPath string `pulumi:"publishedPath,optional"`
	// LastUpdated is the most recent update timestamp reported by Webflow.
	LastUpdated string `pulumi:"lastUpdated,optional"`
	// IsInherited is true when a secondary locale has no override of its own.
	IsInherited bool `pulumi:"isInherited"`
}

// Annotate adds descriptions and constraints to the PageSchemaMarkup resource.
func (r *PageSchemaMarkup) Annotate(a infer.Annotator) {
	a.SetToken("index", "PageSchemaMarkup")
	a.Describe(r, "Manages the JSON-LD schema markup (structured data) of a Webflow page, optionally "+
		"for a specific secondary locale, using the Pages schema markup API (beta). "+
		"Creating or updating the resource PUTs the markup; deleting it clears the markup by writing "+
		"null, since Webflow has no delete endpoint for schema markup. "+
		"The markup is supplied as a JSON string and compared semantically, so key order and "+
		"whitespace differences never cause a diff. "+
		"Requires the 'pages:read' and 'pages:write' scopes. Webflow limits each entry to 60KB, "+
		"32 levels of nesting and 5,000 nodes, and silently strips the keys '__proto__', 'constructor' "+
		"and 'prototype'.")
}

// Annotate adds descriptions to the PageSchemaMarkupArgs fields.
func (args *PageSchemaMarkupArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.PageID,
		"The Webflow page ID (24-character lowercase hexadecimal string, e.g., '6596da6045e56dee495bcbba'). "+
			"Changing this value replaces the resource.")
	a.Describe(&args.LocaleID,
		"Optional secondary locale ID (24-character lowercase hexadecimal string) to manage the markup "+
			"of a localized version of the page. Omit to manage the primary locale. "+
			"Changing this value replaces the resource.")
	a.Describe(&args.SchemaMarkup,
		"The JSON-LD document as a JSON string, e.g. "+
			`'{"@context":"https://schema.org","@type":"FAQPage","mainEntity":[...]}'. `+
			"Must be a single JSON object (use '@graph' to publish several entities). "+
			"Use JSON.stringify / json.dumps / JsonSerializer to serialize an object from your program. "+
			"Compared semantically, so formatting and key order do not cause diffs.")
}

// Annotate adds descriptions to the PageSchemaMarkupState fields.
func (state *PageSchemaMarkupState) Annotate(a infer.Annotator) {
	a.Describe(&state.SiteID, "The ID of the Webflow site containing the page. Read-only.")
	a.Describe(&state.EffectiveLocaleID,
		"The locale whose markup Webflow reports for this page. Differs from 'localeId' when a secondary "+
			"locale falls back to the primary locale. Read-only.")
	a.Describe(&state.PublishedPath, "The relative published URL path of the page (e.g., '/about'). Read-only.")
	a.Describe(&state.LastUpdated, "The timestamp of the most recent update to the markup (RFC3339). Read-only.")
	a.Describe(&state.IsInherited,
		"True when the targeted secondary locale has no schema markup of its own and Webflow serves "+
			"the primary locale's markup instead. Always false for the primary locale. Read-only.")
}

var _ infer.CustomCheck[PageSchemaMarkupArgs] = (*PageSchemaMarkup)(nil)

// validateSchemaMarkupInput checks that a known schemaMarkup parses as a single JSON object.
func validateSchemaMarkupInput(markup string) error {
	_, err := NormalizeSchemaMarkup(markup)
	return err
}

// Check validates the known inputs at preview time: pageId and localeId formats and that
// schemaMarkup parses as a JSON object. Unknown values are validated again at apply time.
func (r *PageSchemaMarkup) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[PageSchemaMarkupArgs], error) {
	inputs, failures, err := checkStrings[PageSchemaMarkupArgs](ctx, req.NewInputs,
		stringValidator{property: "pageId", validate: ValidatePageID},
		stringValidator{property: "localeId", validate: ValidateLocaleID},
		stringValidator{property: "schemaMarkup", validate: validateSchemaMarkupInput},
	)
	return infer.CheckResponse[PageSchemaMarkupArgs]{Inputs: inputs, Failures: failures}, err
}

// validatePageSchemaMarkupArgs validates all inputs and returns the canonical markup.
func validatePageSchemaMarkupArgs(args PageSchemaMarkupArgs) (string, error) {
	if err := ValidatePageID(args.PageID); err != nil {
		return "", fmt.Errorf("validation failed for PageSchemaMarkup resource: %w", err)
	}
	if err := ValidateLocaleID(args.LocaleID); err != nil {
		return "", fmt.Errorf("validation failed for PageSchemaMarkup resource: %w", err)
	}
	canonical, err := NormalizeSchemaMarkup(args.SchemaMarkup)
	if err != nil {
		return "", fmt.Errorf("validation failed for PageSchemaMarkup resource: %w", err)
	}
	return canonical, nil
}

// schemaMarkupEqual reports whether two markup strings are semantically equal JSON.
// Falls back to a plain string comparison when either side is not valid JSON.
func schemaMarkupEqual(a, b string) bool {
	ca, errA := NormalizeSchemaMarkup(a)
	cb, errB := NormalizeSchemaMarkup(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return ca == cb
}

// Diff determines what changes need to be made to the PageSchemaMarkup resource.
// pageId and localeId changes trigger replacement; schemaMarkup changes update in place.
func (r *PageSchemaMarkup) Diff(
	ctx context.Context, req infer.DiffRequest[PageSchemaMarkupArgs, PageSchemaMarkupState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{DetailedDiff: map[string]p.PropertyDiff{}}

	if req.State.PageID != req.Inputs.PageID {
		diff.DetailedDiff["pageId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.State.LocaleID != req.Inputs.LocaleID {
		diff.DetailedDiff["localeId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if len(diff.DetailedDiff) > 0 {
		diff.HasChanges = true
		diff.DeleteBeforeReplace = true
		return diff, nil
	}

	if !schemaMarkupEqual(req.State.SchemaMarkup, req.Inputs.SchemaMarkup) {
		diff.DetailedDiff["schemaMarkup"] = p.PropertyDiff{Kind: p.Update}
		diff.HasChanges = true
	}
	return diff, nil
}

// parsePageSchemaMarkupResourceID extracts and validates the page and locale IDs of a resource
// ID before any URL is built from them.
func parsePageSchemaMarkupResourceID(resourceID string) (pageID, localeID string, err error) {
	pageID, localeID, err = ExtractIDsFromPageSchemaMarkupResourceID(resourceID)
	if err != nil {
		return "", "", fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidatePageID(pageID); err != nil {
		return "", "", fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateLocaleID(localeID); err != nil {
		return "", "", fmt.Errorf("invalid resource ID: %w", err)
	}
	return pageID, localeID, nil
}

// stateFromSchemaMarkupResponse merges an API response into the resource state.
func stateFromSchemaMarkupResponse(args PageSchemaMarkupArgs, resp *PageSchemaMarkupResponse) PageSchemaMarkupState {
	return PageSchemaMarkupState{
		PageSchemaMarkupArgs: args,
		SiteID:               resp.SiteID,
		EffectiveLocaleID:    derefString(resp.EffectiveLocaleID),
		PublishedPath:        derefString(resp.PublishedPath),
		LastUpdated:          derefString(resp.LastUpdated),
		IsInherited:          resp.IsInherited,
	}
}

// putPageSchemaMarkup validates inputs, PUTs the markup and returns the resulting state.
func putPageSchemaMarkup(ctx context.Context, args PageSchemaMarkupArgs) (PageSchemaMarkupState, error) {
	canonical, err := validatePageSchemaMarkupArgs(args)
	if err != nil {
		return PageSchemaMarkupState{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return PageSchemaMarkupState{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	resp, err := PutPageSchemaMarkup(ctx, client, args.PageID, args.LocaleID, json.RawMessage(canonical))
	if err != nil {
		return PageSchemaMarkupState{}, err
	}
	return stateFromSchemaMarkupResponse(args, resp), nil
}

// Create sets the schema markup on the page.
func (r *PageSchemaMarkup) Create(
	ctx context.Context, req infer.CreateRequest[PageSchemaMarkupArgs],
) (infer.CreateResponse[PageSchemaMarkupState], error) {
	id := GeneratePageSchemaMarkupResourceID(req.Inputs.PageID, req.Inputs.LocaleID)

	// During preview, return the expected state without making API calls.
	// Validation is deferred to apply-time because inputs may contain Pulumi unknowns, and no
	// ID is reported while pageId is unknown.
	if req.DryRun {
		if req.Inputs.PageID == "" {
			id = ""
		}
		return infer.CreateResponse[PageSchemaMarkupState]{
			ID:     id,
			Output: PageSchemaMarkupState{PageSchemaMarkupArgs: req.Inputs},
		}, nil
	}

	state, err := putPageSchemaMarkup(ctx, req.Inputs)
	if err != nil {
		return infer.CreateResponse[PageSchemaMarkupState]{}, fmt.Errorf("failed to create page schema markup: %w", err)
	}
	return infer.CreateResponse[PageSchemaMarkupState]{ID: id, Output: state}, nil
}

// Read retrieves the current schema markup of the page for drift detection and import.
// A page without markup of its own (cleared, or a secondary locale inheriting the primary
// locale's markup) is reported as deleted.
func (r *PageSchemaMarkup) Read(
	ctx context.Context, req infer.ReadRequest[PageSchemaMarkupArgs, PageSchemaMarkupState],
) (infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState], error) {
	pageID, localeID, err := parsePageSchemaMarkupResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState]{},
			fmt.Errorf("failed to create HTTP client: %w", err)
	}

	resp, err := GetPageSchemaMarkupAPI(ctx, client, pageID, localeID)
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState]{ID: ""}, nil
		}
		return infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState]{},
			fmt.Errorf("failed to read page schema markup: %w", err)
	}

	markup, err := canonicalizeRawJSON(resp.JSONLDSchema)
	if err != nil {
		return infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState]{}, err
	}
	if markup == "" {
		// Legacy multi-block markup is only exposed raw; surface it so the diff shows the drift.
		markup = derefString(resp.RawJSONLDSchema)
	}
	if markup == "" || (localeID != "" && resp.IsInherited) {
		// No markup of its own: treat the resource as gone so Pulumi recreates it.
		return infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState]{ID: ""}, nil
	}

	inputs := PageSchemaMarkupArgs{PageID: pageID, LocaleID: localeID, SchemaMarkup: markup}
	// Keep the user's formatting when it is semantically identical, to avoid noisy refresh diffs.
	if schemaMarkupEqual(req.Inputs.SchemaMarkup, markup) {
		inputs.SchemaMarkup = req.Inputs.SchemaMarkup
	}

	return infer.ReadResponse[PageSchemaMarkupArgs, PageSchemaMarkupState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  stateFromSchemaMarkupResponse(inputs, resp),
	}, nil
}

// Update replaces the schema markup on the page.
func (r *PageSchemaMarkup) Update(
	ctx context.Context, req infer.UpdateRequest[PageSchemaMarkupArgs, PageSchemaMarkupState],
) (infer.UpdateResponse[PageSchemaMarkupState], error) {
	if req.DryRun {
		state := req.State
		state.PageSchemaMarkupArgs = req.Inputs
		return infer.UpdateResponse[PageSchemaMarkupState]{Output: state}, nil
	}

	state, err := putPageSchemaMarkup(ctx, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[PageSchemaMarkupState]{}, fmt.Errorf("failed to update page schema markup: %w", err)
	}
	return infer.UpdateResponse[PageSchemaMarkupState]{Output: state}, nil
}

// Delete clears the schema markup by writing null. A 404 (page gone) is treated as success.
func (r *PageSchemaMarkup) Delete(
	ctx context.Context, req infer.DeleteRequest[PageSchemaMarkupState],
) (infer.DeleteResponse, error) {
	pageID, localeID, err := parsePageSchemaMarkupResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	if err := ClearPageSchemaMarkup(ctx, client, pageID, localeID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to clear page schema markup: %w", err)
	}
	return infer.DeleteResponse{}, nil
}
