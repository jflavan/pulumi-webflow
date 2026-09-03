// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// SiteCustomCode is the resource controller for managing Webflow site custom code.
// It implements the infer.CustomResource interface for full CRUD operations.
type SiteCustomCode struct{}

// CustomScriptArgs defines the input properties for a custom script to be applied to a site.
type CustomScriptArgs struct {
	// ID is the unique identifier of the registered custom code script.
	// The script must be registered to the site via the Register Script endpoint first.
	// Examples: "cms_slider", "analytics", "custom_widget"
	ID string `pulumi:"id"`
	// Version is the semantic version string for the registered script (e.g., "1.0.0").
	// This version must exist for the registered script.
	Version string `pulumi:"scriptVersion"`
	// Location is where the script is placed on the page.
	// Valid values: "header" (placed in <head>) or "footer" (placed before </body>).
	Location string `pulumi:"location"`
	// Attributes are optional developer-specified key/value pairs applied as HTML attributes to the script tag.
	// Example: {"data-config": "value"}
	Attributes map[string]interface{} `pulumi:"attributes,optional"`
}

// SiteCustomCodeArgs defines the input properties for the SiteCustomCode resource.
type SiteCustomCodeArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	SiteID string `pulumi:"siteId"`
	// Scripts is a list of custom scripts to apply to the site.
	// If you have multiple scripts, ensure they are all included in this list.
	// To remove individual scripts, call this endpoint without the script in the list.
	Scripts []CustomScriptArgs `pulumi:"scripts"`
}

// SiteCustomCodeState defines the output properties for the SiteCustomCode resource.
// It embeds SiteCustomCodeArgs to include input properties in the output.
type SiteCustomCodeState struct {
	SiteCustomCodeArgs
	// LastUpdated is the timestamp when the site's custom code was last updated (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
	// CreatedOn is the timestamp when the site's custom code was first created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
}

// Annotate adds descriptions and constraints to the SiteCustomCode resource.
func (r *SiteCustomCode) Annotate(a infer.Annotator) {
	a.SetToken("index", "SiteCustomCode")
	a.Describe(r, "Manages custom JavaScript code applied to a Webflow site "+
		"(PUT /v2/sites/{site_id}/custom_code). "+
		"This resource allows you to apply registered custom scripts to a site and control where they are placed "+
		"(header or footer). Custom scripts must be registered to the site first via the RegisteredScript "+
		"or InlineScript resource. The full list is sent on every update, so scripts omitted from the list "+
		"are removed from the site; destroying the resource removes all applied code "+
		"(DELETE /v2/sites/{site_id}/custom_code) but leaves the scripts registered.\n\n"+
		customCodeScopesNote+appliedCodeScopesNote)
}

// Check validates the known inputs at preview time. Unknown values (for example a script id
// taken from a RegisteredScript output) are skipped and validated again at apply time.
func (r *SiteCustomCode) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[SiteCustomCodeArgs], error) {
	inputs, failures, err := checkStrings[SiteCustomCodeArgs](ctx, req.NewInputs,
		stringValidator{"siteId", ValidateSiteID},
	)
	if err != nil {
		return infer.CheckResponse[SiteCustomCodeArgs]{Inputs: inputs, Failures: failures}, err
	}
	failures = append(failures, checkCustomCodeScripts(req.NewInputs)...)
	return infer.CheckResponse[SiteCustomCodeArgs]{Inputs: inputs, Failures: failures}, nil
}

// Annotate adds descriptions to the SiteCustomCodeArgs fields.
func (args *SiteCustomCodeArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.Scripts,
		"A list of custom scripts to apply to the site. "+
			"Each script must be registered to the site first. "+
			"To remove individual scripts, simply exclude them from this list on the next update. "+
			"If you have multiple scripts your app manages, ensure they are always included in this list.")
}

// Annotate adds descriptions to the CustomScriptArgs fields.
func (args *CustomScriptArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ID,
		"The unique identifier of the registered custom code script. "+
			"The script must first be registered to the site using the RegisterScript resource. "+
			"Examples: 'cms_slider', 'analytics', 'custom_widget'")

	a.Describe(&args.Version,
		"The semantic version string for the registered script (e.g., '1.0.0', '0.1.2'). "+
			"This version must exist for the registered script ID. "+
			"When you update the version, a different version of the script will be applied.")

	a.Describe(&args.Location,
		"The location where the script is placed on the page. "+
			"Valid values: 'header' (placed in the <head> section), 'footer' (placed before </body>). "+
			"Scripts in the header execute before page content loads, "+
			"while footer scripts execute after the page has loaded.")

	a.Describe(&args.Attributes,
		"Optional developer-specified key/value pairs applied as HTML attributes to the script tag. "+
			"Example: {'data-config': 'my-value'}. "+
			"These attributes are passed directly to the script tag.")
}

// Annotate adds descriptions to the SiteCustomCodeState fields.
func (state *SiteCustomCodeState) Annotate(a infer.Annotator) {
	a.Describe(&state.LastUpdated,
		"The timestamp when the site's custom code was last updated (RFC3339 format). "+
			"This is automatically set and is read-only.")

	a.Describe(&state.CreatedOn,
		"The timestamp when the site's custom code was first created (RFC3339 format). "+
			"This is automatically set when custom code is first applied and is read-only.")
}

// Diff determines what changes need to be made to the site custom code resource.
// siteId change triggers replacement (primary key).
// scripts change triggers in-place update.
func (r *SiteCustomCode) Diff(
	ctx context.Context, req infer.DiffRequest[SiteCustomCodeArgs, SiteCustomCodeState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}

	// Check for siteId change (requires replacement)
	if req.State.SiteID != req.Inputs.SiteID {
		diff.DeleteBeforeReplace = true
		diff.HasChanges = true
		diff.DetailedDiff = map[string]p.PropertyDiff{
			"siteId": {Kind: p.UpdateReplace},
		}
		return diff, nil
	}

	// Scripts changes trigger update (not replace). The comparison is order-insensitive
	// (scripts are matched by id + location) and compares attributes structurally.
	if !scriptListsEqual(req.State.Scripts, req.Inputs.Scripts) {
		diff.HasChanges = true
		diff.DetailedDiff = map[string]p.PropertyDiff{
			"scripts": {Kind: p.Update},
		}
		return diff, nil
	}

	return diff, nil
}

// Create creates new custom code on the Webflow site.
func (r *SiteCustomCode) Create(
	ctx context.Context, req infer.CreateRequest[SiteCustomCodeArgs],
) (infer.CreateResponse[SiteCustomCodeState], error) {
	state := SiteCustomCodeState{SiteCustomCodeArgs: req.Inputs}

	// During preview, return the inputs without calling the API. The ID and the timestamps
	// are unknown until apply, so an empty ID is returned and nothing is fabricated.
	// Full validation is deferred to apply time because inputs may contain Pulumi unknowns
	// (e.g., scripts[].id from a RegisteredScript output) which the infer framework
	// deserializes as zero values; Check already validated the known values.
	if req.DryRun {
		return infer.CreateResponse[SiteCustomCodeState]{Output: state}, nil
	}

	// Validate inputs BEFORE making API calls (all values are resolved at apply-time)
	if err := ValidateSiteID(req.Inputs.SiteID); err != nil {
		return infer.CreateResponse[SiteCustomCodeState]{},
			fmt.Errorf("validation failed for SiteCustomCode resource: %w", err)
	}

	if err := validateCustomCodeScripts("SiteCustomCode", req.Inputs.Scripts); err != nil {
		return infer.CreateResponse[SiteCustomCodeState]{}, err
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[SiteCustomCodeState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API
	response, err := PutSiteCustomCode(ctx, client, req.Inputs.SiteID, toAPIScripts(req.Inputs.Scripts))
	if err != nil {
		return infer.CreateResponse[SiteCustomCodeState]{}, fmt.Errorf("failed to create site custom code: %w", err)
	}

	// Timestamps come from the API response only.
	state.LastUpdated = response.LastUpdated
	state.CreatedOn = response.CreatedOn

	return infer.CreateResponse[SiteCustomCodeState]{
		ID:     GenerateSiteCustomCodeResourceID(req.Inputs.SiteID),
		Output: state,
	}, nil
}

// Read retrieves the current state of site custom code from Webflow.
// Used for drift detection and import operations.
func (r *SiteCustomCode) Read(
	ctx context.Context, req infer.ReadRequest[SiteCustomCodeArgs, SiteCustomCodeState],
) (infer.ReadResponse[SiteCustomCodeArgs, SiteCustomCodeState], error) {
	// Extract siteID from resource ID
	siteID, err := ExtractSiteIDFromSiteCustomCodeResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[SiteCustomCodeArgs, SiteCustomCodeState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return infer.ReadResponse[SiteCustomCodeArgs, SiteCustomCodeState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[SiteCustomCodeArgs, SiteCustomCodeState]{},
			fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API
	response, err := GetSiteCustomCode(ctx, client, siteID)
	if err != nil {
		// Only a 404 means the resource is gone; every other failure (network,
		// auth, rate limiting, 5xx) is propagated so it never looks like a deletion.
		if IsNotFound(err) {
			return infer.ReadResponse[SiteCustomCodeArgs, SiteCustomCodeState]{ID: ""}, nil
		}
		return infer.ReadResponse[SiteCustomCodeArgs, SiteCustomCodeState]{},
			fmt.Errorf("failed to read site custom code: %w", err)
	}

	// Build current state from API response
	currentInputs := SiteCustomCodeArgs{
		SiteID:  siteID,
		Scripts: fromAPIScripts[CustomScriptArgs](response.Scripts),
	}
	currentState := SiteCustomCodeState{
		SiteCustomCodeArgs: currentInputs,
		LastUpdated:        response.LastUpdated,
		CreatedOn:          response.CreatedOn,
	}

	return infer.ReadResponse[SiteCustomCodeArgs, SiteCustomCodeState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update modifies existing site custom code.
func (r *SiteCustomCode) Update(
	ctx context.Context, req infer.UpdateRequest[SiteCustomCodeArgs, SiteCustomCodeState],
) (infer.UpdateResponse[SiteCustomCodeState], error) {
	state := SiteCustomCodeState{
		SiteCustomCodeArgs: req.Inputs,
		LastUpdated:        req.State.LastUpdated,
		CreatedOn:          req.State.CreatedOn,
	}

	// During preview, return the expected state without calling the API. The timestamps
	// recorded in state are carried over rather than fabricated; the real values come
	// from the API at apply time. Validation is deferred because inputs may be unknown.
	if req.DryRun {
		return infer.UpdateResponse[SiteCustomCodeState]{Output: state}, nil
	}

	// Validate inputs BEFORE making API calls (all values are resolved at apply-time)
	if err := ValidateSiteID(req.Inputs.SiteID); err != nil {
		return infer.UpdateResponse[SiteCustomCodeState]{},
			fmt.Errorf("validation failed for SiteCustomCode resource: %w", err)
	}

	if err := validateCustomCodeScripts("SiteCustomCode", req.Inputs.Scripts); err != nil {
		return infer.UpdateResponse[SiteCustomCodeState]{}, err
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.UpdateResponse[SiteCustomCodeState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API
	response, err := PutSiteCustomCode(ctx, client, req.Inputs.SiteID, toAPIScripts(req.Inputs.Scripts))
	if err != nil {
		return infer.UpdateResponse[SiteCustomCodeState]{}, fmt.Errorf("failed to update site custom code: %w", err)
	}

	// Timestamps come from the API response; a value the response omits keeps the recorded one.
	if response.LastUpdated != "" {
		state.LastUpdated = response.LastUpdated
	}
	if response.CreatedOn != "" {
		state.CreatedOn = response.CreatedOn
	}

	return infer.UpdateResponse[SiteCustomCodeState]{
		Output: state,
	}, nil
}

// Delete removes all custom code from the Webflow site.
func (r *SiteCustomCode) Delete(
	ctx context.Context, req infer.DeleteRequest[SiteCustomCodeState],
) (infer.DeleteResponse, error) {
	// Extract siteID from resource ID
	siteID, err := ExtractSiteIDFromSiteCustomCodeResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API (handles 404 gracefully for idempotency)
	if err := DeleteSiteCustomCode(ctx, client, siteID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete site custom code: %w", err)
	}

	return infer.DeleteResponse{}, nil
}
