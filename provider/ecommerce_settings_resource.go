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

// EcommerceSettings is the resource controller for Webflow ecommerce settings.
//
// Note: This is a read-only resource. Ecommerce must be enabled through the Webflow dashboard.
// This resource allows you to import and track existing ecommerce settings as infrastructure state.
type EcommerceSettings struct{}

// EcommerceSettingsArgs defines the input properties for the EcommerceSettings resource.
type EcommerceSettingsArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	SiteID string `pulumi:"siteId"`
}

// EcommerceSettingsState defines the output properties for the EcommerceSettings resource.
type EcommerceSettingsState struct {
	EcommerceSettingsArgs
	// DefaultCurrency is the three-letter ISO 4217 currency code for the site (read-only).
	DefaultCurrency string `pulumi:"defaultCurrency,optional"`
	// CreatedOn is the timestamp when ecommerce was enabled on the site (read-only, ISO 8601 format).
	CreatedOn string `pulumi:"createdOn,optional"`
}

// Annotate adds descriptions and constraints to the EcommerceSettings resource.
func (r *EcommerceSettings) Annotate(a infer.Annotator) {
	a.SetToken("index", "EcommerceSettings")
	a.Describe(r, "Manages (imports) ecommerce settings for a Webflow site. "+
		"This is a read-only resource that allows you to track and reference existing ecommerce settings. "+
		"Ecommerce must be enabled through the Webflow dashboard before this resource can be used. "+
		"Use this resource to access the site's default currency and verify ecommerce is enabled.")
}

// Annotate adds descriptions to the EcommerceSettingsArgs fields.
func (args *EcommerceSettingsArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"The site must have ecommerce enabled through the Webflow dashboard. "+
			"You can find your site ID in the Webflow dashboard under Site Settings.")
}

// Annotate adds descriptions to the EcommerceSettingsState fields.
func (state *EcommerceSettingsState) Annotate(a infer.Annotator) {
	a.Describe(&state.DefaultCurrency,
		"The three-letter ISO 4217 currency code for the site (e.g., 'USD', 'EUR', 'GBP'). "+
			"This is the default currency used for ecommerce transactions on this site. "+
			"This value is set in the Webflow dashboard and is read-only.")

	a.Describe(&state.CreatedOn,
		"The timestamp when ecommerce was enabled on the site (ISO 8601 format). "+
			"This is automatically set when ecommerce is enabled and is read-only.")
}

// Diff determines what changes need to be made to the ecommerce settings resource.
// Only siteId changes trigger replacement since this is a read-only resource. Delete is a
// state-only no-op, so the default create-before-delete ordering is kept.
func (r *EcommerceSettings) Diff(
	ctx context.Context, req infer.DiffRequest[EcommerceSettingsArgs, EcommerceSettingsState],
) (infer.DiffResponse, error) {
	if req.State.SiteID != req.Inputs.SiteID {
		return infer.DiffResponse{
			HasChanges:   true,
			DetailedDiff: map[string]p.PropertyDiff{"siteId": {Kind: p.UpdateReplace}},
		}, nil
	}
	return infer.DiffResponse{}, nil
}

// Create "creates" an ecommerce settings resource by reading the existing settings from Webflow.
func (r *EcommerceSettings) Create(
	ctx context.Context, req infer.CreateRequest[EcommerceSettingsArgs],
) (infer.CreateResponse[EcommerceSettingsState], error) {
	state := EcommerceSettingsState{EcommerceSettingsArgs: req.Inputs}
	resourceID := GenerateEcommerceSettingsResourceID(req.Inputs.SiteID)

	// During preview the real values are unknown; return the inputs only, and no ID when the
	// site ID itself is still unknown.
	if req.DryRun {
		if req.Inputs.SiteID == "" {
			resourceID = ""
		}
		return infer.CreateResponse[EcommerceSettingsState]{ID: resourceID, Output: state}, nil
	}

	if err := ValidateSiteID(req.Inputs.SiteID); err != nil {
		return infer.CreateResponse[EcommerceSettingsState]{},
			fmt.Errorf("validation failed for EcommerceSettings resource: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[EcommerceSettingsState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := GetEcommerceSettings(ctx, client, req.Inputs.SiteID)
	if err != nil {
		return infer.CreateResponse[EcommerceSettingsState]{}, fmt.Errorf("failed to read ecommerce settings: %w", err)
	}

	state.DefaultCurrency = response.DefaultCurrency
	state.CreatedOn = response.CreatedOn

	return infer.CreateResponse[EcommerceSettingsState]{ID: resourceID, Output: state}, nil
}

// Read retrieves the current state of ecommerce settings from Webflow.
// Only a 404 clears the resource from state; "ecommerce not enabled" (409) is surfaced as an
// actionable error so the user can decide what to do.
func (r *EcommerceSettings) Read(
	ctx context.Context, req infer.ReadRequest[EcommerceSettingsArgs, EcommerceSettingsState],
) (infer.ReadResponse[EcommerceSettingsArgs, EcommerceSettingsState], error) {
	siteID, err := ExtractSiteIDFromEcommerceSettingsResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[EcommerceSettingsArgs, EcommerceSettingsState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return infer.ReadResponse[EcommerceSettingsArgs, EcommerceSettingsState]{},
			fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[EcommerceSettingsArgs, EcommerceSettingsState]{},
			fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := GetEcommerceSettings(ctx, client, siteID)
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[EcommerceSettingsArgs, EcommerceSettingsState]{ID: ""}, nil
		}
		return infer.ReadResponse[EcommerceSettingsArgs, EcommerceSettingsState]{},
			fmt.Errorf("failed to read ecommerce settings: %w", err)
	}

	currentInputs := EcommerceSettingsArgs{SiteID: siteID}
	currentState := EcommerceSettingsState{
		EcommerceSettingsArgs: currentInputs,
		DefaultCurrency:       response.DefaultCurrency,
		CreatedOn:             response.CreatedOn,
	}

	return infer.ReadResponse[EcommerceSettingsArgs, EcommerceSettingsState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update is a no-op for this read-only resource; it returns the current state.
func (r *EcommerceSettings) Update(
	ctx context.Context, req infer.UpdateRequest[EcommerceSettingsArgs, EcommerceSettingsState],
) (infer.UpdateResponse[EcommerceSettingsState], error) {
	state := EcommerceSettingsState{
		EcommerceSettingsArgs: req.Inputs,
		DefaultCurrency:       req.State.DefaultCurrency,
		CreatedOn:             req.State.CreatedOn,
	}
	return infer.UpdateResponse[EcommerceSettingsState]{Output: state}, nil
}

// Delete removes the ecommerce settings from Pulumi state.
// This does NOT disable ecommerce on the site; that must be done in the Webflow dashboard.
func (r *EcommerceSettings) Delete(
	ctx context.Context, req infer.DeleteRequest[EcommerceSettingsState],
) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}
