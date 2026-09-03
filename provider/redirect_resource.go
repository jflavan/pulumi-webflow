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

// Redirect is the resource controller for managing Webflow redirects.
// It implements the infer.CustomResource interface for full CRUD operations.
type Redirect struct{}

// RedirectArgs defines the input properties for the Redirect resource.
type RedirectArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	SiteID string `pulumi:"siteId"`
	// SourcePath is the URL path to redirect from (e.g., "/old-page").
	// Must start with "/" and contain only valid URL characters.
	// Examples: "/old-page", "/blog/2023", "/products/item-1"
	SourcePath string `pulumi:"sourcePath"`
	// DestinationPath is the URL path to redirect to (e.g., "/new-page").
	// Must start with "/" and contain only valid URL characters.
	// Examples: "/new-page", "/home", "/products/item-1"
	DestinationPath string `pulumi:"destinationPath"`
	// StatusCode is DEPRECATED and ignored. Webflow redirects are always 301 (permanent); the
	// API object is {id, fromUrl, toUrl} and carries no status code. The property is kept only
	// so existing programs keep compiling; it is never sent, diffed or validated.
	StatusCode int `pulumi:"statusCode,optional"`
}

// RedirectState defines the output properties for the Redirect resource.
// It embeds RedirectArgs to include input properties in the output.
type RedirectState struct {
	RedirectArgs
	// CreatedOn is the timestamp when the redirect was created (read-only). The Webflow API
	// does not document this field; it is only set if the API happens to return it.
	CreatedOn string `pulumi:"createdOn,optional"`
}

// Annotate adds descriptions and constraints to the Redirect resource.
func (r *Redirect) Annotate(a infer.Annotator) {
	a.SetToken("index", "Redirect")
	a.Describe(r, "Manages HTTP redirects for a Webflow site "+
		"(POST/PATCH/DELETE /v2/sites/{site_id}/redirects, scope sites:write; GET requires sites:read). "+
		"Webflow redirects are always permanent (301) redirects from one site path to another; "+
		"the API does not support other status codes. "+
		"Changing `sourcePath` replaces the redirect; changing `destinationPath` updates it in place.")
}

// Annotate adds descriptions to the RedirectArgs fields.
func (args *RedirectArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.SourcePath,
		"The URL path to redirect from (e.g., '/old-page', '/blog/2023'). "+
			"Must start with '/' and contain only valid URL characters "+
			"(letters, numbers, hyphens, underscores, slashes, dots). "+
			"Query strings and fragments are not allowed in the source path. "+
			"Changing this value replaces the redirect.")

	a.Describe(&args.DestinationPath,
		"The URL path to redirect to (e.g., '/new-page', '/home'). "+
			"Must start with '/' and contain only valid URL characters. "+
			"This is the location where users will be redirected when they visit the source path. "+
			"Changing this value updates the redirect in place.")

	a.Describe(&args.StatusCode,
		"Deprecated and ignored. Webflow redirects are always 301 (permanent) redirects: the "+
			"redirect API object is {id, fromUrl, toUrl} and has no status code, so this value is "+
			"never sent to Webflow, never validated and never produces a diff. "+
			"Remove it from your program; it only remains for backwards compatibility.")
	a.Deprecate(&args.StatusCode,
		"Webflow redirects are always 301; statusCode is ignored and will be removed in a future major version.")
}

// Annotate adds descriptions to the RedirectState fields.
func (state *RedirectState) Annotate(a infer.Annotator) {
	a.Describe(&state.CreatedOn,
		"The timestamp when the redirect was created (RFC3339 format), if the Webflow API reports it. "+
			"The redirects API does not document this field, so it is normally empty. Read-only.")
}

// redirectCheckValidators lists the known-value validators applied by Check.
var redirectCheckValidators = []stringValidator{
	{property: "siteId", validate: ValidateSiteID},
	{property: "sourcePath", validate: ValidateSourcePath},
	{property: "destinationPath", validate: ValidateDestinationPath},
}

// Check validates the inputs that are already known at preview time. Values that still depend
// on other resources' outputs are skipped here and validated again in Create or Update.
// statusCode is deprecated and deliberately not validated.
func (r *Redirect) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[RedirectArgs], error) {
	inputs, failures, err := checkStrings[RedirectArgs](ctx, req.NewInputs, redirectCheckValidators...)
	return infer.CheckResponse[RedirectArgs]{Inputs: inputs, Failures: failures}, err
}

// Diff determines what changes need to be made to the redirect resource.
// siteId and sourcePath changes trigger replacement (they identify the redirect).
// destinationPath changes trigger an in-place update via PATCH.
// statusCode is deprecated and ignored by the API, so it is never diffed.
func (r *Redirect) Diff(
	ctx context.Context, req infer.DiffRequest[RedirectArgs, RedirectState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{DetailedDiff: map[string]p.PropertyDiff{}}

	// siteId change requires replacement
	if req.State.SiteID != req.Inputs.SiteID {
		diff.DeleteBeforeReplace = true
		diff.HasChanges = true
		diff.DetailedDiff["siteId"] = p.PropertyDiff{Kind: p.UpdateReplace}
		return diff, nil
	}

	// sourcePath is the redirect's identity from the user's point of view - replace
	if req.State.SourcePath != req.Inputs.SourcePath {
		diff.DeleteBeforeReplace = true
		diff.HasChanges = true
		diff.DetailedDiff["sourcePath"] = p.PropertyDiff{Kind: p.UpdateReplace}
		return diff, nil
	}

	// destinationPath is updatable in place (PATCH /v2/sites/{id}/redirects/{rid} accepts toUrl)
	if req.State.DestinationPath != req.Inputs.DestinationPath {
		diff.HasChanges = true
		diff.DetailedDiff["destinationPath"] = p.PropertyDiff{Kind: p.Update}
	}

	return diff, nil
}

// Create creates a new redirect on the Webflow site.
func (r *Redirect) Create(
	ctx context.Context, req infer.CreateRequest[RedirectArgs],
) (infer.CreateResponse[RedirectState], error) {
	state := RedirectState{
		RedirectArgs: req.Inputs,
	}

	// Preview: return the inputs without an ID and without calling the API. An empty ID tells
	// the framework to present the ID and every output as unknown to dependent resources.
	// Inputs such as siteId may still be unknown (zeroed) during preview.
	if req.DryRun {
		return infer.CreateResponse[RedirectState]{Output: state}, nil
	}

	if err := validateRedirectArgs(req.Inputs); err != nil {
		return infer.CreateResponse[RedirectState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[RedirectState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := PostRedirect(ctx, client, req.Inputs.SiteID, req.Inputs.SourcePath, req.Inputs.DestinationPath)
	if err != nil {
		return infer.CreateResponse[RedirectState]{}, fmt.Errorf("failed to create redirect: %w", err)
	}

	if response.ID == "" {
		return infer.CreateResponse[RedirectState]{}, errors.New(
			"webflow API returned empty redirect ID - " +
				"this is unexpected and may indicate an API issue")
	}

	// createdOn comes from the API when it reports one; it is never fabricated locally.
	state.CreatedOn = response.CreatedOn

	return infer.CreateResponse[RedirectState]{
		ID:     GenerateRedirectResourceID(req.Inputs.SiteID, response.ID),
		Output: state,
	}, nil
}

// validateRedirectArgs validates every input of the Redirect resource.
// statusCode is deprecated and ignored, so it is not validated.
func validateRedirectArgs(args RedirectArgs) error {
	if err := ValidateSiteID(args.SiteID); err != nil {
		return fmt.Errorf("validation failed for Redirect resource: %w", err)
	}
	if err := ValidateSourcePath(args.SourcePath); err != nil {
		return fmt.Errorf("validation failed for Redirect resource: %w", err)
	}
	if err := ValidateDestinationPath(args.DestinationPath); err != nil {
		return fmt.Errorf("validation failed for Redirect resource: %w", err)
	}
	return nil
}

// parseRedirectResourceID extracts and validates the site and redirect IDs from a resource ID.
func parseRedirectResourceID(resourceID string) (siteID, redirectID string, err error) {
	siteID, redirectID, err = ExtractIDsFromRedirectResourceID(resourceID)
	if err != nil {
		return "", "", fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return "", "", fmt.Errorf("invalid resource ID %q: %w", resourceID, err)
	}
	if err := ValidateRedirectID(redirectID); err != nil {
		return "", "", fmt.Errorf("invalid resource ID %q: %w", resourceID, err)
	}
	return siteID, redirectID, nil
}

// Read retrieves the current state of a redirect from Webflow.
// The API has no single-redirect GET, so the site's redirect list is paged through until the
// redirect is found. A missing redirect (or a 404 for the site) yields an empty ID; any other
// error is returned so transient failures are never mistaken for a deleted resource.
// With empty inputs and state (pulumi import) the inputs are built from the API object alone.
func (r *Redirect) Read(
	ctx context.Context, req infer.ReadRequest[RedirectArgs, RedirectState],
) (infer.ReadResponse[RedirectArgs, RedirectState], error) {
	siteID, redirectID, err := parseRedirectResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[RedirectArgs, RedirectState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[RedirectArgs, RedirectState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	found, err := FindRedirect(ctx, client, siteID, redirectID)
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[RedirectArgs, RedirectState]{ID: ""}, nil
		}
		return infer.ReadResponse[RedirectArgs, RedirectState]{}, fmt.Errorf("failed to read redirects: %w", err)
	}
	if found == nil {
		return infer.ReadResponse[RedirectArgs, RedirectState]{ID: ""}, nil
	}

	// statusCode is deprecated and not part of the API object: keep whatever the user wrote
	// (program inputs first, then state) rather than writing 0 into the inputs.
	statusCode := req.Inputs.StatusCode
	if statusCode == 0 {
		statusCode = req.State.StatusCode
	}
	createdOn := found.CreatedOn
	if createdOn == "" {
		createdOn = req.State.CreatedOn
	}

	currentInputs := RedirectArgs{
		SiteID:          siteID,
		SourcePath:      found.SourcePath,
		DestinationPath: found.DestinationPath,
		StatusCode:      statusCode,
	}
	currentState := RedirectState{
		RedirectArgs: currentInputs,
		CreatedOn:    createdOn,
	}

	return infer.ReadResponse[RedirectArgs, RedirectState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update modifies an existing redirect in place via PATCH.
func (r *Redirect) Update(
	ctx context.Context, req infer.UpdateRequest[RedirectArgs, RedirectState],
) (infer.UpdateResponse[RedirectState], error) {
	state := RedirectState{
		RedirectArgs: req.Inputs,
		CreatedOn:    req.State.CreatedOn,
	}

	// Preview: return the expected state without validating (inputs may be unknown) or calling the API.
	if req.DryRun {
		return infer.UpdateResponse[RedirectState]{
			Output: state,
		}, nil
	}

	if err := validateRedirectArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[RedirectState]{}, err
	}

	siteID, redirectID, err := parseRedirectResourceID(req.ID)
	if err != nil {
		return infer.UpdateResponse[RedirectState]{}, err
	}
	if siteID != req.Inputs.SiteID {
		return infer.UpdateResponse[RedirectState]{}, fmt.Errorf(
			"validation failed for Redirect resource: siteId %q does not match the site in resource ID %q",
			req.Inputs.SiteID, req.ID)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.UpdateResponse[RedirectState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := PatchRedirect(ctx, client, siteID, redirectID, req.Inputs.DestinationPath)
	if err != nil {
		return infer.UpdateResponse[RedirectState]{}, fmt.Errorf("failed to update redirect: %w", err)
	}
	if response.DestinationPath != "" {
		state.DestinationPath = response.DestinationPath
	}
	if response.CreatedOn != "" {
		state.CreatedOn = response.CreatedOn
	}

	return infer.UpdateResponse[RedirectState]{
		Output: state,
	}, nil
}

// Delete removes a redirect from the Webflow site.
func (r *Redirect) Delete(ctx context.Context, req infer.DeleteRequest[RedirectState]) (infer.DeleteResponse, error) {
	siteID, redirectID, err := parseRedirectResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 404 is treated as success for idempotency
	if err := DeleteRedirect(ctx, client, siteID, redirectID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete redirect: %w", err)
	}

	return infer.DeleteResponse{}, nil
}
