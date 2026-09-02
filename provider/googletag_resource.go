// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"fmt"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// GoogleTag is the resource controller for a single Google Tag ID configured on a Webflow site.
// It implements the infer.CustomResource interface for full CRUD operations.
//
// Webflow exposes the site's tags as a list with an upsert endpoint; this resource manages exactly
// one entry of that list (identified by tagId) so that several GoogleTag resources can coexist on
// one site without clobbering each other.
type GoogleTag struct{}

// GoogleTagArgs defines the input properties for the GoogleTag resource.
type GoogleTagArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	SiteID string `pulumi:"siteId"`
	// TagID is the Google Tag ID (G-, GT-, AW- or DC- prefix). Changing it replaces the resource.
	TagID string `pulumi:"tagId"`
	// DisplayName is the human-readable label for the tag.
	DisplayName string `pulumi:"displayName"`
	// Order is the optional display position of the tag. When omitted Webflow assigns it.
	Order *int `pulumi:"order,optional"`
}

// GoogleTagState defines the output properties for the GoogleTag resource.
// It embeds GoogleTagArgs so inputs are also available as outputs.
type GoogleTagState struct {
	GoogleTagArgs
	// EffectiveOrder is the position Webflow reports for this tag after (re)normalization.
	EffectiveOrder *int `pulumi:"effectiveOrder,optional"`
}

// Annotate adds descriptions and constraints to the GoogleTag resource.
func (r *GoogleTag) Annotate(a infer.Annotator) {
	a.SetToken("index", "GoogleTag")
	a.Describe(r, "Manages a single Google Tag ID (Google Analytics 4, Google Tag, Google Ads or "+
		"Campaign Manager) configured on a Webflow site via the Google Tag Manager integration API. "+
		"Each resource manages one entry in the site's tag list, so multiple GoogleTag resources may "+
		"target the same site. Tags on the site that are not managed by Pulumi are left untouched. "+
		"Requires the 'sites:read' and 'sites:write' scopes. Webflow allows up to 25 tags per site and "+
		"rejects legacy Universal Analytics 'UA-' IDs.")
}

// Annotate adds descriptions to the GoogleTagArgs fields.
func (args *GoogleTagArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"Changing this value replaces the resource.")
	a.Describe(&args.TagID,
		"The Google Tag ID to configure, e.g., 'G-1A2B3C4D5E' (GA4 measurement ID), 'GT-ABC123' "+
			"(Google Tag), 'AW-123456789' (Google Ads) or 'DC-1234567' (Campaign Manager). "+
			"Webflow rejects Universal Analytics 'UA-' IDs. Changing this value replaces the resource.")
	a.Describe(&args.DisplayName,
		"A human-readable label for the tag shown in the Webflow dashboard (e.g., 'Primary Google Analytics').")
	a.Describe(&args.Order,
		"Optional display position of the tag within the site's tag list. When omitted, Webflow assigns "+
			"the position automatically (see 'effectiveOrder'). Webflow renormalizes positions after a "+
			"tag is deleted, so only set this when you need to control ordering explicitly.")
}

// Annotate adds descriptions to the GoogleTagState fields.
func (state *GoogleTagState) Annotate(a infer.Annotator) {
	a.Describe(&state.EffectiveOrder,
		"The display position Webflow reports for this tag. This reflects any automatic assignment or "+
			"renormalization performed by Webflow and may differ from 'order'. Read-only.")
}

// validateGoogleTagArgs validates all inputs before any API call is made.
func validateGoogleTagArgs(args GoogleTagArgs) error {
	if err := ValidateSiteID(args.SiteID); err != nil {
		return fmt.Errorf("validation failed for GoogleTag resource: %w", err)
	}
	if err := ValidateGoogleTagID(args.TagID); err != nil {
		return fmt.Errorf("validation failed for GoogleTag resource: %w", err)
	}
	if err := ValidateGoogleTagDisplayName(args.DisplayName); err != nil {
		return fmt.Errorf("validation failed for GoogleTag resource: %w", err)
	}
	if err := ValidateGoogleTagOrder(args.Order); err != nil {
		return fmt.Errorf("validation failed for GoogleTag resource: %w", err)
	}
	return nil
}

// intPtrEqual compares two optional ints.
func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// Diff determines what changes need to be made to the GoogleTag resource.
// siteId and tagId changes trigger replacement; displayName and order update in place.
func (r *GoogleTag) Diff(
	ctx context.Context, req infer.DiffRequest[GoogleTagArgs, GoogleTagState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{DetailedDiff: map[string]p.PropertyDiff{}}

	if req.State.SiteID != req.Inputs.SiteID {
		diff.DetailedDiff["siteId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if !strings.EqualFold(req.State.TagID, req.Inputs.TagID) {
		diff.DetailedDiff["tagId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if len(diff.DetailedDiff) > 0 {
		diff.HasChanges = true
		diff.DeleteBeforeReplace = true
		return diff, nil
	}

	if req.State.DisplayName != req.Inputs.DisplayName {
		diff.DetailedDiff["displayName"] = p.PropertyDiff{Kind: p.Update}
	}
	// An unset order means "let Webflow decide", so only a change between explicit values,
	// or setting/unsetting an explicit value, is a diff.
	if !intPtrEqual(req.State.Order, req.Inputs.Order) {
		diff.DetailedDiff["order"] = p.PropertyDiff{Kind: p.Update}
	}
	diff.HasChanges = len(diff.DetailedDiff) > 0
	return diff, nil
}

// upsertGoogleTag sends the tag to Webflow and returns the resulting state.
func upsertGoogleTag(ctx context.Context, args GoogleTagArgs) (GoogleTagState, error) {
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return GoogleTagState{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := UpsertGoogleTags(ctx, client, args.SiteID, []GoogleTagEntry{{
		DisplayName: args.DisplayName,
		TagID:       args.TagID,
		Order:       args.Order,
	}})
	if err != nil {
		return GoogleTagState{}, err
	}

	entry := findGoogleTag(response.GoogleTagIDs, args.TagID)
	if entry == nil {
		return GoogleTagState{}, fmt.Errorf("tag '%s' was accepted by Webflow but is missing from the "+
			"returned tag list for site '%s'; the tag may have been rejected silently. "+
			"Check the tag ID and the site's Google Tag configuration in the Webflow dashboard",
			args.TagID, args.SiteID)
	}

	state := GoogleTagState{GoogleTagArgs: args, EffectiveOrder: entry.Order}
	state.DisplayName = entry.DisplayName
	return state, nil
}

// Create adds the Google Tag to the site via the upsert endpoint.
func (r *GoogleTag) Create(
	ctx context.Context, req infer.CreateRequest[GoogleTagArgs],
) (infer.CreateResponse[GoogleTagState], error) {
	id := GenerateGoogleTagResourceID(req.Inputs.SiteID, req.Inputs.TagID)

	// During preview, return the expected state without making API calls.
	// Validation is deferred to apply-time because inputs may contain Pulumi unknowns.
	if req.DryRun {
		return infer.CreateResponse[GoogleTagState]{
			ID:     id,
			Output: GoogleTagState{GoogleTagArgs: req.Inputs},
		}, nil
	}

	if err := validateGoogleTagArgs(req.Inputs); err != nil {
		return infer.CreateResponse[GoogleTagState]{}, err
	}

	state, err := upsertGoogleTag(ctx, req.Inputs)
	if err != nil {
		return infer.CreateResponse[GoogleTagState]{}, fmt.Errorf("failed to create Google Tag: %w", err)
	}

	return infer.CreateResponse[GoogleTagState]{ID: id, Output: state}, nil
}

// Read retrieves the current state of the Google Tag from the site's tag list.
// Used for drift detection and import operations.
func (r *GoogleTag) Read(
	ctx context.Context, req infer.ReadRequest[GoogleTagArgs, GoogleTagState],
) (infer.ReadResponse[GoogleTagArgs, GoogleTagState], error) {
	siteID, tagID, err := ExtractIDsFromGoogleTagResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[GoogleTagArgs, GoogleTagState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[GoogleTagArgs, GoogleTagState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := ListGoogleTags(ctx, client, siteID)
	if err != nil {
		if IsNotFound(err) {
			// The site itself is gone.
			return infer.ReadResponse[GoogleTagArgs, GoogleTagState]{ID: ""}, nil
		}
		return infer.ReadResponse[GoogleTagArgs, GoogleTagState]{}, fmt.Errorf("failed to read Google Tags: %w", err)
	}

	entry := findGoogleTag(response.GoogleTagIDs, tagID)
	if entry == nil {
		// Tag was removed outside of Pulumi.
		return infer.ReadResponse[GoogleTagArgs, GoogleTagState]{ID: ""}, nil
	}

	inputs := GoogleTagArgs{
		SiteID:      siteID,
		TagID:       entry.TagID,
		DisplayName: entry.DisplayName,
		// Preserve the user's intent: only report an explicit order when one was configured.
		Order: req.Inputs.Order,
	}
	if inputs.Order != nil && !intPtrEqual(inputs.Order, entry.Order) {
		inputs.Order = entry.Order
	}

	return infer.ReadResponse[GoogleTagArgs, GoogleTagState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  GoogleTagState{GoogleTagArgs: inputs, EffectiveOrder: entry.Order},
	}, nil
}

// Update modifies the display name or order of the Google Tag via the upsert endpoint.
func (r *GoogleTag) Update(
	ctx context.Context, req infer.UpdateRequest[GoogleTagArgs, GoogleTagState],
) (infer.UpdateResponse[GoogleTagState], error) {
	if req.DryRun {
		return infer.UpdateResponse[GoogleTagState]{
			Output: GoogleTagState{GoogleTagArgs: req.Inputs, EffectiveOrder: req.State.EffectiveOrder},
		}, nil
	}

	if err := validateGoogleTagArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[GoogleTagState]{}, err
	}

	state, err := upsertGoogleTag(ctx, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[GoogleTagState]{}, fmt.Errorf("failed to update Google Tag: %w", err)
	}

	return infer.UpdateResponse[GoogleTagState]{Output: state}, nil
}

// Delete removes the Google Tag from the site. A 404 is treated as success.
func (r *GoogleTag) Delete(
	ctx context.Context, req infer.DeleteRequest[GoogleTagState],
) (infer.DeleteResponse, error) {
	siteID, tagID, err := ExtractIDsFromGoogleTagResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	if err := DeleteGoogleTag(ctx, client, siteID, tagID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete Google Tag: %w", err)
	}
	return infer.DeleteResponse{}, nil
}
