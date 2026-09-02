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
	"time"

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
	// DisplayName is the user-facing name for the script (1-50 alphanumeric characters).
	// Example: "CmsSlider", "AnalyticsScript", "MyCustomScript123"
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
	a.Describe(r, "Manages inline custom code scripts in the Webflow script registry. "+
		"This resource allows you to register and manage inline JavaScript code that can be "+
		"deployed across your Webflow site with version control.")
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
			"and using the RegisteredScript resource with a hostedLocation instead.")

	a.Describe(&args.Version,
		"The Semantic Version (SemVer) string for the script "+
			"(e.g., '1.0.0', '2.3.1'). "+
			"This helps track different versions of your script. "+
			"See https://semver.org/ for more information on semantic versioning.")

	a.Describe(&args.DisplayName,
		"The user-facing name for the script (1-50 alphanumeric characters). "+
			"This name is used to identify the script in the Webflow interface. "+
			"Only letters (A-Z, a-z) and numbers (0-9) are allowed. "+
			"Example valid names: 'CmsSlider', 'AnalyticsScript', 'MyCustomScript123'.")

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

// Diff determines what changes need to be made to the inline script resource.
// NOTE: Webflow API does not support updating inline scripts (no PATCH endpoint).
// All changes require replacement (delete + recreate).
func (r *InlineScript) Diff(
	ctx context.Context, req infer.DiffRequest[InlineScriptArgs, InlineScriptState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}
	detailedDiff := map[string]p.PropertyDiff{}

	// All field changes trigger replacement since Webflow API doesn't support PATCH
	if req.State.SiteID != req.Inputs.SiteID {
		detailedDiff["siteId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if req.State.SourceCode != req.Inputs.SourceCode {
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

	// Compare version only when both sides have one: state written before scriptVersion
	// existed has no version, and that must not force a replace.
	stateVersion := req.State.Version
	inputVersion := req.Inputs.Version
	if stateVersion != "" && inputVersion != "" && stateVersion != inputVersion {
		detailedDiff["scriptVersion"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// canCopy defaults to false in Webflow, so an omitted input (false) matches a freshly
	// registered script; a difference means it was configured explicitly or changed out of band.
	if req.State.CanCopy != req.Inputs.CanCopy {
		detailedDiff["canCopy"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// If any changes were detected, all require replacement
	if len(detailedDiff) > 0 {
		diff.HasChanges = true
		diff.DeleteBeforeReplace = true
		diff.DetailedDiff = detailedDiff
	}

	return diff, nil
}

// Create creates a new inline script on the Webflow site.
func (r *InlineScript) Create(
	ctx context.Context, req infer.CreateRequest[InlineScriptArgs],
) (infer.CreateResponse[InlineScriptState], error) {
	state := InlineScriptState{
		InlineScriptArgs: req.Inputs,
		CreatedOn:        "", // Will be populated after creation
		LastUpdated:      "", // Will be populated after creation
	}

	// During preview, return expected state without making API calls.
	// Validation is deferred to apply-time because inputs may contain Pulumi unknowns
	// (e.g., siteId from a Site output) which the infer framework deserializes as zero values.
	if req.DryRun {
		// Set a preview timestamp
		now := time.Now().Format(time.RFC3339)
		state.CreatedOn = now
		state.LastUpdated = now
		// Generate a predictable ID for dry-run
		previewID := fmt.Sprintf("preview-%d", time.Now().Unix())
		state.ScriptID = previewID
		return infer.CreateResponse[InlineScriptState]{
			ID:     GenerateInlineScriptResourceID(req.Inputs.SiteID, previewID),
			Output: state,
		}, nil
	}

	// Validate inputs BEFORE making API calls (all values are resolved at apply-time)
	if err := ValidateSiteID(req.Inputs.SiteID); err != nil {
		return infer.CreateResponse[InlineScriptState]{},
			fmt.Errorf("validation failed for InlineScript resource: %w", err)
	}
	if err := ValidateSourceCode(req.Inputs.SourceCode); err != nil {
		return infer.CreateResponse[InlineScriptState]{},
			fmt.Errorf("validation failed for InlineScript resource: %w", err)
	}
	if err := ValidateVersion(req.Inputs.Version); err != nil {
		return infer.CreateResponse[InlineScriptState]{},
			fmt.Errorf("validation failed for InlineScript resource: %w", err)
	}
	if err := ValidateScriptDisplayName(req.Inputs.DisplayName); err != nil {
		return infer.CreateResponse[InlineScriptState]{},
			fmt.Errorf("validation failed for InlineScript resource: %w", err)
	}
	// IntegrityHash is optional for inline scripts, but validate if provided
	if req.Inputs.IntegrityHash != "" {
		if err := ValidateIntegrityHash(req.Inputs.IntegrityHash); err != nil {
			return infer.CreateResponse[InlineScriptState]{},
				fmt.Errorf("validation failed for InlineScript resource: %w", err)
		}
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, providerVersion)
	if err != nil {
		return infer.CreateResponse[InlineScriptState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API
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

	// Update state with values from API response. The integrity hash Webflow reports
	// is recorded in state (Diff ignores it unless the user configured one explicitly).
	state.ScriptID = response.ID
	state.HostedLocation = response.HostedLocation
	if response.IntegrityHash != "" {
		state.IntegrityHash = response.IntegrityHash
	}
	state.CreatedOn = response.CreatedOn
	state.LastUpdated = response.LastUpdated

	resourceID := GenerateInlineScriptResourceID(req.Inputs.SiteID, response.ID)

	return infer.CreateResponse[InlineScriptState]{
		ID:     resourceID,
		Output: state,
	}, nil
}

// Read retrieves the current state of an inline script from Webflow.
// Used for drift detection and import operations.
// Note: The list endpoint (GET /registered_scripts) is shared between hosted and inline scripts.
func (r *InlineScript) Read(
	ctx context.Context, req infer.ReadRequest[InlineScriptArgs, InlineScriptState],
) (infer.ReadResponse[InlineScriptArgs, InlineScriptState], error) {
	// Extract siteID and scriptID from resource ID
	siteID, scriptID, err := ExtractIDsFromInlineScriptResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := validateScriptResourceIDs(siteID, scriptID); err != nil {
		return infer.ReadResponse[InlineScriptArgs, InlineScriptState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, providerVersion)
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

	// Build current state from API response
	// Note: Webflow's list scripts API may not return all fields,
	// so we preserve values from the existing inputs/state when API returns empty.
	version := foundScript.Version
	if version == "" {
		switch {
		case req.Inputs.Version != "":
			version = req.Inputs.Version
		case req.State.Version != "":
			version = req.State.Version
		default:
			version = "0.0.0"
			p.GetLogger(ctx).Warningf(
				"InlineScript '%s': Webflow API did not return version and no previous state available. "+
					"Using fallback version '0.0.0'. The actual registered script version may differ. "+
					"To set the correct version, update your Pulumi configuration with the actual version.",
				foundScript.DisplayName,
			)
		}
	}

	// Preserve sourceCode from inputs/state since the list endpoint may not return it
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
			"All changes require replacement (delete + recreate). " +
			"If you see this error, please report it as a provider bug")
}

// Delete removes an inline script from the Webflow site.
// Uses the same DELETE endpoint as hosted scripts: DELETE /v2/sites/{site_id}/registered_scripts/{script_id}
func (r *InlineScript) Delete(
	ctx context.Context, req infer.DeleteRequest[InlineScriptState],
) (infer.DeleteResponse, error) {
	// Extract siteID and scriptID from resource ID
	siteID, scriptID, err := ExtractIDsFromInlineScriptResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := validateScriptResourceIDs(siteID, scriptID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, providerVersion)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API (handles 404 gracefully for idempotency)
	// Uses the same delete endpoint as hosted scripts
	if err := DeleteRegisteredScript(ctx, client, siteID, scriptID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete inline script: %w", err)
	}

	return infer.DeleteResponse{}, nil
}
