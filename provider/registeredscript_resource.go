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

// RegisteredScriptResource is the resource controller for managing Webflow registered scripts.
// It implements the infer.CustomResource interface for full CRUD operations.
type RegisteredScriptResource struct{}

// RegisteredScriptResourceArgs defines the input properties for the RegisteredScript resource.
type RegisteredScriptResourceArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	SiteID string `pulumi:"siteId"`
	// DisplayName is the user-facing name for the script (1-50 letters, digits or spaces).
	// Example: "CMS Slider", "AnalyticsScript", "MyCustomScript123"
	DisplayName string `pulumi:"displayName"`
	// HostedLocation is the URI for the externally hosted script.
	// Example: "https://cdn.example.com/my-script.js"
	HostedLocation string `pulumi:"hostedLocation"`
	// IntegrityHash is the Sub-Resource Integrity Hash (SRI) for the script.
	// Format: "sha384-<hash>", "sha256-<hash>", or "sha512-<hash>"
	// You can generate an SRI hash using https://www.srihash.org/
	IntegrityHash string `pulumi:"integrityHash"`
	// Version is the Semantic Version (SemVer) string for the script.
	// Format: "major.minor.patch" (e.g., "1.0.0", "2.3.1")
	// See https://semver.org/ for more information.
	// Required: the Webflow register endpoint rejects requests without a version, so the
	// schema requires it too and a missing value surfaces at preview time.
	Version string `pulumi:"scriptVersion"`
	// CanCopy indicates whether the script can be copied on site duplication.
	// Default: false
	CanCopy bool `pulumi:"canCopy,optional"`
}

// RegisteredScriptResourceState defines the output properties for the RegisteredScript resource.
// It embeds RegisteredScriptResourceArgs to include input properties in the output.
type RegisteredScriptResourceState struct {
	RegisteredScriptResourceArgs
	// ScriptID is the Webflow-assigned script ID (read-only).
	// This is typically the lowercase version of displayName and is used
	// when applying scripts via SiteCustomCode or PageCustomCode.
	ScriptID string `pulumi:"scriptId"`
	// CreatedOn is the timestamp when the script was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
	// LastUpdated is the timestamp when the script was last updated (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
}

// Annotate adds descriptions and constraints to the RegisteredScript resource.
func (r *RegisteredScriptResource) Annotate(a infer.Annotator) {
	a.SetToken("index", "RegisteredScript")
	a.Describe(r, "Registers an externally hosted script in a Webflow site's script registry "+
		"(POST /v2/sites/{site_id}/registered_scripts/hosted). Registered scripts are applied to a site "+
		"or page with the SiteCustomCode and PageCustomCode resources. "+
		"scriptVersion is required: the register endpoint rejects requests without one.\n\n"+
		customCodeScopesNote+"\n\n"+
		"**IMPORTANT LIMITATION:** Webflow has no endpoint to update or unregister a registered script. "+
		"Registrations are versioned and permanent (a site can hold up to 800). "+
		"Changing displayName, hostedLocation, integrityHash, scriptVersion or canCopy therefore registers "+
		"a new script (a new version when only scriptVersion changes) and the previous registration "+
		"remains in the registry. Destroying the resource is a logged no-op: the script stays registered "+
		"and Pulumi simply stops managing it. Applied code is removed by SiteCustomCode and PageCustomCode.")
}

// Annotate adds descriptions to the RegisteredScriptResourceArgs fields.
func (args *RegisteredScriptResourceArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.DisplayName,
		"The user-facing name for the script (1-50 characters: letters, digits and spaces). "+
			"This name is used to identify the script in the Webflow interface and derives the scriptId. "+
			"Changing it registers a new script; the previous registration remains. "+
			"Example valid names: 'CMS Slider', 'AnalyticsScript', 'MyCustomScript123'.")

	a.Describe(&args.HostedLocation,
		"The URI for the externally hosted script (e.g., 'https://cdn.example.com/my-script.js'). "+
			"Must be a valid HTTP or HTTPS URL. "+
			"The script should be publicly accessible and properly configured for cross-origin requests.")

	a.Describe(&args.IntegrityHash,
		"The Sub-Resource Integrity (SRI) hash for the script. "+
			"Format: 'sha384-<hash>', 'sha256-<hash>', or 'sha512-<hash>'. "+
			"SRI hashes help ensure that the script hasn't been modified in transit. "+
			"You can generate an SRI hash using https://www.srihash.org/")

	a.Describe(&args.Version,
		"The Semantic Version (SemVer) string for the script "+
			"(e.g., '1.0.0', '2.3.1'). Required by the Webflow register endpoint. "+
			"Registered scripts are versioned: changing this value registers a new version of the script "+
			"and the previous version remains registered. "+
			"See https://semver.org/ for more information on semantic versioning.")

	a.Describe(&args.CanCopy,
		"Indicates whether the script can be copied when the site is duplicated. "+
			"Default: false. "+
			"When true, the script will be included when creating a copy of the site.")
}

// Annotate adds descriptions to the RegisteredScriptResourceState fields.
func (state *RegisteredScriptResourceState) Annotate(a infer.Annotator) {
	a.Describe(&state.ScriptID,
		"The Webflow-assigned script ID (read-only). "+
			"This is typically the lowercase version of displayName. "+
			"Use this value when referencing the script in SiteCustomCode or PageCustomCode resources.")

	a.Describe(&state.CreatedOn,
		"The timestamp when the script was created (RFC3339 format). "+
			"This is automatically set by Webflow when the script is created and is read-only.")

	a.Describe(&state.LastUpdated,
		"The timestamp when the script was last updated (RFC3339 format). "+
			"This is automatically updated by Webflow when the script is modified and is read-only.")
}

// Check validates the known inputs at preview time. Values that are still unknown
// (computed from other resources) are skipped and validated again in Create.
func (r *RegisteredScriptResource) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[RegisteredScriptResourceArgs], error) {
	inputs, failures, err := checkStrings[RegisteredScriptResourceArgs](ctx, req.NewInputs,
		stringValidator{"siteId", ValidateSiteID},
		stringValidator{"displayName", ValidateScriptDisplayName},
		stringValidator{"hostedLocation", ValidateHostedLocation},
		stringValidator{"integrityHash", ValidateIntegrityHash},
		stringValidator{"scriptVersion", ValidateVersion},
	)
	return infer.CheckResponse[RegisteredScriptResourceArgs]{Inputs: inputs, Failures: failures}, err
}

// Diff determines what changes need to be made to the registered script resource.
// Webflow has neither an update nor a delete endpoint for registered scripts, so every
// change is a replacement that registers a new script (or a new version of it); the
// previous registration stays in the registry because Delete cannot remove it.
func (r *RegisteredScriptResource) Diff(
	ctx context.Context, req infer.DiffRequest[RegisteredScriptResourceArgs, RegisteredScriptResourceState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}
	detailedDiff := map[string]p.PropertyDiff{}

	if req.State.SiteID != req.Inputs.SiteID {
		detailedDiff["siteId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if req.State.DisplayName != req.Inputs.DisplayName {
		detailedDiff["displayName"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if req.State.HostedLocation != req.Inputs.HostedLocation {
		detailedDiff["hostedLocation"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if req.State.IntegrityHash != req.Inputs.IntegrityHash {
		detailedDiff["integrityHash"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// An empty state version means Read could not recover it (the list endpoint may omit
	// version, e.g. right after import) or the state predates scriptVersion. It is unknown
	// rather than different, so it must not force a replacement.
	if req.State.Version != "" && req.Inputs.Version != "" && req.State.Version != req.Inputs.Version {
		detailedDiff["scriptVersion"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if req.State.CanCopy != req.Inputs.CanCopy {
		detailedDiff["canCopy"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// The new registration is created before the old resource is dropped from state
	// (Pulumi's default order): Delete is a no-op, so nothing is gained by deleting first
	// and a failed registration would otherwise leave the resource missing from state.
	if len(detailedDiff) > 0 {
		diff.HasChanges = true
		diff.DetailedDiff = detailedDiff
	}

	return diff, nil
}

// validateRegisteredScriptArgs validates fully-resolved inputs at apply time.
func validateRegisteredScriptArgs(args RegisteredScriptResourceArgs) error {
	checks := []error{
		ValidateSiteID(args.SiteID),
		ValidateScriptDisplayName(args.DisplayName),
		ValidateHostedLocation(args.HostedLocation),
		ValidateIntegrityHash(args.IntegrityHash),
		ValidateVersion(args.Version),
	}
	for _, err := range checks {
		if err != nil {
			return fmt.Errorf("validation failed for RegisteredScript resource: %w", err)
		}
	}
	return nil
}

// Create registers a new hosted script on the Webflow site.
func (r *RegisteredScriptResource) Create(
	ctx context.Context, req infer.CreateRequest[RegisteredScriptResourceArgs],
) (infer.CreateResponse[RegisteredScriptResourceState], error) {
	state := RegisteredScriptResourceState{RegisteredScriptResourceArgs: req.Inputs}

	// During preview, return the inputs without calling the API. The ID and the
	// Webflow-assigned outputs (scriptId, timestamps) are unknown until apply, so an
	// empty ID is returned and dependents see unknown values. Inputs may be unknown, so
	// full validation is deferred to apply time (Check already validated known values).
	if req.DryRun {
		return infer.CreateResponse[RegisteredScriptResourceState]{Output: state}, nil
	}

	if err := validateRegisteredScriptArgs(req.Inputs); err != nil {
		return infer.CreateResponse[RegisteredScriptResourceState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[RegisteredScriptResourceState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := PostRegisteredScript(ctx, client, req.Inputs.SiteID, RegisteredScriptRequest{
		DisplayName:    req.Inputs.DisplayName,
		HostedLocation: req.Inputs.HostedLocation,
		IntegrityHash:  req.Inputs.IntegrityHash,
		CanCopy:        req.Inputs.CanCopy,
		Version:        req.Inputs.Version,
	})
	if err != nil {
		return infer.CreateResponse[RegisteredScriptResourceState]{},
			fmt.Errorf("failed to create registered script: %w", err)
	}

	// Defensive check: Ensure Webflow API returned a valid script ID
	if response.ID == "" {
		return infer.CreateResponse[RegisteredScriptResourceState]{}, errors.New(
			"webflow API returned empty registered script ID - " +
				"this is unexpected and may indicate an API issue")
	}

	// Outputs come from the API response only; nothing is fabricated.
	state.ScriptID = response.ID
	state.CreatedOn = response.CreatedOn
	state.LastUpdated = response.LastUpdated

	return infer.CreateResponse[RegisteredScriptResourceState]{
		ID:     GenerateRegisteredScriptResourceID(req.Inputs.SiteID, response.ID),
		Output: state,
	}, nil
}

// Read retrieves the current state of a registered script from Webflow.
// Used for drift detection and import operations (empty inputs and state).
func (r *RegisteredScriptResource) Read(
	ctx context.Context, req infer.ReadRequest[RegisteredScriptResourceArgs, RegisteredScriptResourceState],
) (infer.ReadResponse[RegisteredScriptResourceArgs, RegisteredScriptResourceState], error) {
	siteID, scriptID, err := ExtractIDsFromRegisteredScriptResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[RegisteredScriptResourceArgs, RegisteredScriptResourceState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := validateScriptResourceIDs(siteID, scriptID); err != nil {
		return infer.ReadResponse[RegisteredScriptResourceArgs, RegisteredScriptResourceState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[RegisteredScriptResourceArgs, RegisteredScriptResourceState]{},
			fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Locate the script in the site's registered scripts, following pagination.
	foundScript, err := FindRegisteredScript(ctx, client, siteID, scriptID)
	if err != nil {
		// Only "not found" (site or script gone) signals deletion; every other failure
		// (network, auth, rate limiting, 5xx) is propagated.
		if IsNotFound(err) {
			return infer.ReadResponse[RegisteredScriptResourceArgs, RegisteredScriptResourceState]{ID: ""}, nil
		}
		return infer.ReadResponse[RegisteredScriptResourceArgs, RegisteredScriptResourceState]{},
			fmt.Errorf("failed to read registered script: %w", err)
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

	currentInputs := RegisteredScriptResourceArgs{
		SiteID:         siteID,
		DisplayName:    foundScript.DisplayName,
		HostedLocation: foundScript.HostedLocation,
		IntegrityHash:  foundScript.IntegrityHash,
		Version:        version,
		CanCopy:        foundScript.CanCopy,
	}
	currentState := RegisteredScriptResourceState{
		RegisteredScriptResourceArgs: currentInputs,
		ScriptID:                     foundScript.ID,
		CreatedOn:                    foundScript.CreatedOn,
		LastUpdated:                  foundScript.LastUpdated,
	}

	return infer.ReadResponse[RegisteredScriptResourceArgs, RegisteredScriptResourceState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update is not supported by Webflow API for registered scripts.
// All changes trigger replacement via Diff, so this method should never be called.
// This is a safety net that returns an error if somehow invoked.
func (r *RegisteredScriptResource) Update(
	_ context.Context, _ infer.UpdateRequest[RegisteredScriptResourceArgs, RegisteredScriptResourceState],
) (infer.UpdateResponse[RegisteredScriptResourceState], error) {
	return infer.UpdateResponse[RegisteredScriptResourceState]{},
		errors.New("registered scripts cannot be updated in-place: " +
			"Webflow API does not support PATCH for registered scripts. " +
			"All changes require replacement (a new registration). " +
			"If you see this error, please report it as a provider bug")
}

// Delete is a logged no-op: the Webflow API has no endpoint to unregister a script, so the
// registration stays in the site's registry and Pulumi simply stops managing it. No HTTP
// request is made.
func (r *RegisteredScriptResource) Delete(
	ctx context.Context, req infer.DeleteRequest[RegisteredScriptResourceState],
) (infer.DeleteResponse, error) {
	siteID, scriptID, err := ExtractIDsFromRegisteredScriptResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	scriptDeleteNoOpWarning(ctx, "RegisteredScript", siteID, scriptID)
	return infer.DeleteResponse{}, nil
}
