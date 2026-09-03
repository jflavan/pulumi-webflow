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

// InlineScript is the resource controller for managing Webflow inline registered scripts.
// It implements the infer.CustomResource interface for full CRUD operations.
type InlineScript struct{}

// InlineScriptArgs defines the input properties for the InlineScript resource.
type InlineScriptArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	SiteID string `pulumi:"siteId"`
	// SourceCode is the inline JavaScript code to register, limited to 2000 characters.
	SourceCode string `pulumi:"sourceCode"`
	// Version is the Semantic Version (SemVer) string for the script.
	// Format: "major.minor.patch" (e.g., "1.0.0", "2.3.1")
	Version string `pulumi:"scriptVersion"`
	// DisplayName is the user-facing name for the script (1-50 letters, digits or spaces).
	// Example: "CMS Slider", "AnalyticsScript", "MyCustomScript123"
	DisplayName string `pulumi:"displayName"`
	// CanCopy indicates whether the script can be copied on site duplication.
	// Default: false
	CanCopy bool `pulumi:"canCopy,optional"`
	// IntegrityHash is the Sub-Resource Integrity Hash (SRI) for the script.
	// Format: "sha384-<hash>", "sha256-<hash>", or "sha512-<hash>"
	// This field is optional for inline scripts.
	IntegrityHash string `pulumi:"integrityHash,optional"`
}

// InlineScriptState defines the output properties for the InlineScript resource.
// It embeds InlineScriptArgs to include input properties in the output.
type InlineScriptState struct {
	InlineScriptArgs
	// ScriptID is the Webflow-assigned script ID (read-only).
	// This is typically the lowercase version of displayName and is used
	// when applying scripts via SiteCustomCode or PageCustomCode.
	ScriptID string `pulumi:"scriptId"`
	// HostedLocation is the URI for the hosted version of the inline script (read-only).
	// This is set by Webflow after the inline script is registered.
	HostedLocation string `pulumi:"hostedLocation,optional"`
	// CreatedOn is the timestamp when the script was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
	// LastUpdated is the timestamp when the script was last updated (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
}

// Annotate adds descriptions and constraints to the InlineScript resource.
func (r *InlineScript) Annotate(a infer.Annotator) {
	a.SetToken("index", "InlineScript")
	a.Describe(r, "Registers inline JavaScript in a Webflow site's script registry "+
		"(POST /v2/sites/{site_id}/registered_scripts/inline). Registered scripts are applied to a site "+
		"or page with the SiteCustomCode and PageCustomCode resources.\n\n"+
		customCodeScopesNote+"\n\n"+
		"**IMPORTANT LIMITATION:** Webflow has no endpoint to update or unregister a registered script. "+
		"Registrations are versioned and permanent (a site can hold up to 800). "+
		"Changing sourceCode, displayName, scriptVersion, canCopy or integrityHash therefore registers "+
		"a new script (a new version when only scriptVersion changes) and the previous registration "+
		"remains in the registry. Destroying the resource is a logged no-op: the script stays registered "+
		"and Pulumi simply stops managing it. Applied code is removed by SiteCustomCode and PageCustomCode. "+
		"The list endpoint does not return sourceCode, so after an import the source is unknown until "+
		"it is set in the program; Diff does not report a change for it in that case.")
}

// Annotate adds descriptions to the InlineScriptArgs fields.
func (args *InlineScriptArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.SourceCode,
		"The inline JavaScript code to register, limited to 2000 characters. "+
			"This code will be directly embedded in your Webflow site. "+
			"If your script exceeds 2000 characters, consider hosting it externally "+
			"and using the RegisteredScript resource with a hostedLocation instead. "+
			"Webflow does not return the source code when listing scripts, so it cannot be read back after import.")

	a.Describe(&args.Version,
		"The Semantic Version (SemVer) string for the script "+
			"(e.g., '1.0.0', '2.3.1'). Required by the Webflow register endpoint. "+
			"Registered scripts are versioned: changing this value registers a new version of the script "+
			"and the previous version remains registered. "+
			"See https://semver.org/ for more information on semantic versioning.")

	a.Describe(&args.DisplayName,
		"The user-facing name for the script (1-50 characters: letters, digits and spaces). "+
			"This name is used to identify the script in the Webflow interface and derives the scriptId. "+
			"Changing it registers a new script; the previous registration remains. "+
			"Example valid names: 'CMS Slider', 'AnalyticsScript', 'MyCustomScript123'.")

	a.Describe(&args.CanCopy,
		"Indicates whether the script can be copied when the site is duplicated. "+
			"Default: false. "+
			"When true, the script will be included when creating a copy of the site.")

	a.Describe(&args.IntegrityHash,
		"The Sub-Resource Integrity (SRI) hash for the script (optional). "+
			"Format: 'sha384-<hash>', 'sha256-<hash>', or 'sha512-<hash>'. "+
			"SRI hashes help ensure that the script hasn't been modified in transit. "+
			"You can generate an SRI hash using https://www.srihash.org/")
}

// Annotate adds descriptions to the InlineScriptState fields.
func (state *InlineScriptState) Annotate(a infer.Annotator) {
	a.Describe(&state.ScriptID,
		"The Webflow-assigned script ID (read-only). "+
			"This is typically the lowercase version of displayName. "+
			"Use this value when referencing the script in SiteCustomCode or PageCustomCode resources.")

	a.Describe(&state.HostedLocation,
		"The URI for the hosted version of the inline script (read-only). "+
			"This is automatically set by Webflow after the inline script is registered.")

	a.Describe(&state.CreatedOn,
		"The timestamp when the script was created (RFC3339 format). "+
			"This is automatically set by Webflow when the script is created and is read-only.")

	a.Describe(&state.LastUpdated,
		"The timestamp when the script was last updated (RFC3339 format). "+
			"This is automatically updated by Webflow when the script is modified and is read-only.")
}

// Check validates the known inputs at preview time. Values that are still unknown
// (computed from other resources) are skipped and validated again in Create.
func (r *InlineScript) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[InlineScriptArgs], error) {
	inputs, failures, err := checkStrings[InlineScriptArgs](ctx, req.NewInputs,
		stringValidator{"siteId", ValidateSiteID},
		stringValidator{"sourceCode", ValidateSourceCode},
		stringValidator{"scriptVersion", ValidateVersion},
		stringValidator{"displayName", ValidateScriptDisplayName},
		stringValidator{"integrityHash", validateOptionalIntegrityHash},
	)
	return infer.CheckResponse[InlineScriptArgs]{Inputs: inputs, Failures: failures}, err
}

// Diff determines what changes need to be made to the inline script resource.
// Webflow has neither an update nor a delete endpoint for registered scripts, so every
// change is a replacement that registers a new script (or a new version of it); the
// previous registration stays in the registry because Delete cannot remove it.
func (r *InlineScript) Diff(
	ctx context.Context, req infer.DiffRequest[InlineScriptArgs, InlineScriptState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}
	detailedDiff := map[string]p.PropertyDiff{}

	if req.State.SiteID != req.Inputs.SiteID {
		detailedDiff["siteId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// The list endpoint never returns sourceCode, so after an import the state value is
	// empty: that means "unknown", not "different", and must not force a replacement.
	if req.State.SourceCode != "" && req.State.SourceCode != req.Inputs.SourceCode {
		detailedDiff["sourceCode"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if req.State.DisplayName != req.Inputs.DisplayName {
		detailedDiff["displayName"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// integrityHash is optional: an omitted input means "don't care", so only an
	// explicitly configured hash that differs from the registered one forces a replace.
	if req.Inputs.IntegrityHash != "" && req.State.IntegrityHash != req.Inputs.IntegrityHash {
		detailedDiff["integrityHash"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// An empty state version means Read could not recover it (the list endpoint may omit
	// version, e.g. right after import) or the state predates scriptVersion: unknown, not
	// different.
	if req.State.Version != "" && req.Inputs.Version != "" && req.State.Version != req.Inputs.Version {
		detailedDiff["scriptVersion"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// canCopy defaults to false in Webflow, so an omitted input (false) matches a freshly
	// registered script; a difference means it was configured explicitly or changed out of band.
	if req.State.CanCopy != req.Inputs.CanCopy {
		detailedDiff["canCopy"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// The new registration is created before the old resource is dropped from state
	// (Pulumi's default order): Delete is a no-op, so nothing is gained by deleting first.
	if len(detailedDiff) > 0 {
		diff.HasChanges = true
		diff.DetailedDiff = detailedDiff
	}

	return diff, nil
}

// validateInlineScriptArgs validates fully-resolved inputs at apply time.
func validateInlineScriptArgs(args InlineScriptArgs) error {
	checks := []error{
		ValidateSiteID(args.SiteID),
		ValidateSourceCode(args.SourceCode),
		ValidateVersion(args.Version),
		ValidateScriptDisplayName(args.DisplayName),
		validateOptionalIntegrityHash(args.IntegrityHash),
	}
	for _, err := range checks {
		if err != nil {
			return fmt.Errorf("validation failed for InlineScript resource: %w", err)
		}
	}
	return nil
}

// Create registers a new inline script on the Webflow site.
func (r *InlineScript) Create(
	ctx context.Context, req infer.CreateRequest[InlineScriptArgs],
) (infer.CreateResponse[InlineScriptState], error) {
	state := InlineScriptState{InlineScriptArgs: req.Inputs}

	// During preview, return the inputs without calling the API. The ID and the
	// Webflow-assigned outputs (scriptId, hostedLocation, timestamps) are unknown until
	// apply, so an empty ID is returned and dependents see unknown values. Inputs may be
	// unknown, so full validation is deferred to apply time (Check validated known values).
	if req.DryRun {
		return infer.CreateResponse[InlineScriptState]{Output: state}, nil
	}

	if err := validateInlineScriptArgs(req.Inputs); err != nil {
		return infer.CreateResponse[InlineScriptState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[InlineScriptState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := PostInlineScript(ctx, client, req.Inputs.SiteID, InlineScriptRequest{
		SourceCode:    req.Inputs.SourceCode,
		Version:       req.Inputs.Version,
		DisplayName:   req.Inputs.DisplayName,
		CanCopy:       req.Inputs.CanCopy,
		IntegrityHash: req.Inputs.IntegrityHash,
	})
	if err != nil {
		return infer.CreateResponse[InlineScriptState]{},
			fmt.Errorf("failed to create inline script: %w", err)
	}

	// Defensive check: Ensure Webflow API returned a valid script ID
	if response.ID == "" {
		return infer.CreateResponse[InlineScriptState]{}, errors.New(
			"webflow API returned empty inline script ID - " +
				"this is unexpected and may indicate an API issue")
	}

	// Outputs come from the API response only. The integrity hash Webflow reports is
	// recorded in state (Diff ignores it unless the user configured one explicitly).
	state.ScriptID = response.ID
	state.HostedLocation = response.HostedLocation
	if response.IntegrityHash != "" {
		state.IntegrityHash = response.IntegrityHash
	}
	state.CreatedOn = response.CreatedOn
	state.LastUpdated = response.LastUpdated

	return infer.CreateResponse[InlineScriptState]{
		ID:     GenerateInlineScriptResourceID(req.Inputs.SiteID, response.ID),
		Output: state,
	}, nil
}

// Read retrieves the current state of an inline script from Webflow.
// Used for drift detection and import operations (empty inputs and state).
// Note: The list endpoint (GET /registered_scripts) is shared between hosted and inline scripts.
func (r *InlineScript) Read(
	ctx context.Context, req infer.ReadRequest[InlineScriptArgs, InlineScriptState],
) (infer.ReadResponse[InlineScriptArgs, InlineScriptState], error) {
	siteID, scriptID, err := ExtractIDsFromInlineScriptResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := validateScriptResourceIDs(siteID, scriptID); err != nil {
		return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{},
			fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Locate the script in the site's registered scripts (the list endpoint is shared
	// between hosted and inline scripts), following pagination.
	foundScript, err := FindRegisteredScript(ctx, client, siteID, scriptID)
	if err != nil {
		// Only "not found" (site or script gone) signals deletion; every other failure
		// (network, auth, rate limiting, 5xx) is propagated.
		if IsNotFound(err) {
			return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{ID: ""}, nil
		}
		return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{},
			fmt.Errorf("failed to read inline script: %w", err)
	}

	// The list endpoint may omit version. Fall back to the configured or recorded value;
	// when neither exists (import) it stays empty, which Diff treats as unknown rather
	// than inventing a version that was never registered.
	version := foundScript.Version
	if version == "" {
		version = req.Inputs.Version
	}
	if version == "" {
		version = req.State.Version
	}

	// The list endpoint never returns sourceCode: keep the configured or recorded value.
	// After an import both are empty and Diff treats the empty state value as unknown.
	sourceCode := req.Inputs.SourceCode
	if sourceCode == "" {
		sourceCode = req.State.SourceCode
	}

	// The registered hash goes into state so drift is visible; the list endpoint may omit
	// it, in which case the previously recorded value is kept. Inputs only carry the hash
	// when the user configured one — an omitted integrityHash stays omitted so that Read
	// never turns a "don't care" input into a managed value.
	stateHash := foundScript.IntegrityHash
	if stateHash == "" {
		stateHash = req.State.IntegrityHash
	}
	inputHash := req.Inputs.IntegrityHash
	if inputHash != "" {
		inputHash = stateHash
	}

	currentInputs := InlineScriptArgs{
		SiteID:        siteID,
		SourceCode:    sourceCode,
		Version:       version,
		DisplayName:   foundScript.DisplayName,
		CanCopy:       foundScript.CanCopy,
		IntegrityHash: inputHash,
	}
	currentState := InlineScriptState{
		InlineScriptArgs: currentInputs,
		ScriptID:         foundScript.ID,
		HostedLocation:   foundScript.HostedLocation,
		CreatedOn:        foundScript.CreatedOn,
		LastUpdated:      foundScript.LastUpdated,
	}
	currentState.IntegrityHash = stateHash

	return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update is not supported by Webflow API for inline scripts.
// All changes trigger replacement via Diff, so this method should never be called.
// This is a safety net that returns an error if somehow invoked.
func (r *InlineScript) Update(
	_ context.Context, _ infer.UpdateRequest[InlineScriptArgs, InlineScriptState],
) (infer.UpdateResponse[InlineScriptState], error) {
	return infer.UpdateResponse[InlineScriptState]{},
		errors.New("inline scripts cannot be updated in-place: " +
			"Webflow API does not support PATCH for inline scripts. " +
			"All changes require replacement (a new registration). " +
			"If you see this error, please report it as a provider bug")
}

// Delete is a logged no-op: the Webflow API has no endpoint to unregister a script, so the
// registration stays in the site's registry and Pulumi simply stops managing it. No HTTP
// request is made.
func (r *InlineScript) Delete(
	ctx context.Context, req infer.DeleteRequest[InlineScriptState],
) (infer.DeleteResponse, error) {
	siteID, scriptID, err := ExtractIDsFromInlineScriptResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	scriptDeleteNoOpWarning(ctx, "InlineScript", siteID, scriptID)
	return infer.DeleteResponse{}, nil
}
