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
	"reflect"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Webhook is the resource controller for managing Webflow webhooks.
// It implements the infer.CustomResource interface for full CRUD operations.
type Webhook struct{}

// WebhookArgs defines the input properties for the Webhook resource.
type WebhookArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	SiteID string `pulumi:"siteId"`
	// TriggerType is the Webflow event that triggers this webhook.
	// Valid values are the documented webhook events; see validTriggerTypeList in webhook.go.
	TriggerType string `pulumi:"triggerType"`
	// URL is the HTTPS endpoint where Webflow will send webhook events.
	// Must be a valid HTTPS URL (e.g., "https://example.com/webhooks/webflow")
	URL string `pulumi:"url"`
	// Filter is an optional event filter. The Webflow API only documents it for the
	// form_submission trigger, with the shape { name: string } (the form name).
	Filter map[string]interface{} `pulumi:"filter,optional"`
}

// WebhookState defines the output properties for the Webhook resource.
// It embeds WebhookArgs to include input properties in the output.
type WebhookState struct {
	WebhookArgs
	// CreatedOn is the timestamp when the webhook was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
	// LastTriggered is the timestamp when the webhook was last triggered (read-only).
	LastTriggered string `pulumi:"lastTriggered,optional"`
}

// Annotate adds descriptions and constraints to the Webhook resource.
func (w *Webhook) Annotate(a infer.Annotator) {
	a.SetToken("index", "Webhook")
	a.Describe(w, "Manages webhooks for a Webflow site. "+
		"Webhooks allow you to receive real-time notifications when events occur in your Webflow site, "+
		"such as form submissions, page updates, e-commerce orders, and more. "+
		"\n\n**Authentication:** the webhook endpoints require a Data Client (OAuth) token with the "+
		"`sites:write` scope plus the read scope of the event family being subscribed to "+
		"(`forms:read`, `cms:read`, `pages:read`, `ecommerce:read` or `comments:read`); reading a webhook "+
		"requires `sites:read`. "+
		"\n\nWebhooks cannot be updated in place: any change to `triggerType`, `url` or `filter` replaces the "+
		"webhook (the replacement is created before the old one is deleted, so there is no window without "+
		"a webhook; Webflow allows several webhooks per trigger).")
}

// Annotate adds descriptions to the WebhookArgs fields.
func (args *WebhookArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.TriggerType,
		"The Webflow event that triggers this webhook. "+
			"Valid values: "+strings.Join(validTriggerTypeList, ", ")+". "+
			"Webhooks require a Data Client (OAuth) token with `sites:write` plus the read scope of the "+
			"event family: `forms:read` (form_submission), `cms:read` (collection_item_*), "+
			"`pages:read` (page_*), `ecommerce:read` (ecomm_*), `comments:read` (comment_created); "+
			"site_publish needs `sites:read`. "+
			"Example: 'form_submission' to receive notifications when forms are submitted. "+
			"Changing this value replaces the webhook.")

	a.Describe(&args.URL,
		"The HTTPS endpoint where Webflow will send webhook events "+
			"(e.g., 'https://example.com/webhooks/webflow', 'https://api.example.com/events'). "+
			"Must be a valid HTTPS URL. Webflow requires HTTPS for security. "+
			"Your endpoint should accept POST requests with JSON payloads containing event data. "+
			"Changing this value replaces the webhook.")

	a.Describe(&args.Filter,
		"Optional filter for webhook events. "+
			"Only supported for the `form_submission` trigger type: it selects the form whose submissions "+
			"should be sent, with the shape `{ name: string }` where `name` is the form name "+
			"(e.g., `{ name: 'Contact Form' }`). "+
			"Setting a filter for any other trigger type, or any key other than `name`, is rejected. "+
			"When omitted the provider does not track the filter Webflow reports, so a filter set outside "+
			"Pulumi does not cause a diff. Changing this value replaces the webhook.")
}

// Annotate adds descriptions to the WebhookState fields.
func (state *WebhookState) Annotate(a infer.Annotator) {
	a.Describe(&state.CreatedOn,
		"The timestamp when the webhook was created (RFC3339 format). "+
			"This is automatically set by Webflow when the webhook is created and is read-only.")

	a.Describe(&state.LastTriggered,
		"The timestamp when the webhook was last triggered (RFC3339 format). "+
			"This is automatically updated by Webflow when the webhook fires and is read-only. "+
			"Will be empty if the webhook has never been triggered.")
}

// webhookCheckValidators lists the known-value validators applied by Check.
var webhookCheckValidators = []stringValidator{
	{property: "siteId", validate: ValidateSiteID},
	{property: "triggerType", validate: ValidateTriggerType},
	{property: "url", validate: ValidateWebhookURL},
}

// Check validates the inputs that are already known at preview time. Values that still depend
// on other resources' outputs are skipped here and validated again in Create.
func (w *Webhook) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[WebhookArgs], error) {
	inputs, failures, err := checkStrings[WebhookArgs](ctx, req.NewInputs, webhookCheckValidators...)
	if err != nil {
		return infer.CheckResponse[WebhookArgs]{Inputs: inputs, Failures: failures}, err
	}
	// The filter contract depends on the trigger type, so both must be known.
	if isKnown(req.NewInputs, "filter") {
		if triggerType, known := knownString(req.NewInputs, "triggerType"); known {
			if verr := ValidateWebhookFilter(triggerType, inputs.Filter); verr != nil {
				failures = append(failures, checkFailure("filter", verr))
			}
		}
	}
	return infer.CheckResponse[WebhookArgs]{Inputs: inputs, Failures: failures}, nil
}

// Diff determines what changes need to be made to the webhook resource.
// Webflow webhooks do not support updates - all changes require replacement. The replacement
// is created before the old webhook is deleted (no DeleteBeforeReplace) so that there is never
// a window without a webhook; the API allows several webhooks per trigger.
func (w *Webhook) Diff(
	ctx context.Context, req infer.DiffRequest[WebhookArgs, WebhookState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}
	detailedDiff := map[string]p.PropertyDiff{}

	// Check for siteId change (requires replacement)
	if req.State.SiteID != req.Inputs.SiteID {
		detailedDiff["siteId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// Check for triggerType change (requires replacement - webhooks cannot be updated)
	if req.State.TriggerType != req.Inputs.TriggerType {
		detailedDiff["triggerType"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// Check for URL change (requires replacement - webhooks cannot be updated)
	if req.State.URL != req.Inputs.URL {
		detailedDiff["url"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// Check for filter change (requires replacement - webhooks cannot be updated).
	// A nil filter and an empty filter map are the same thing: no filter.
	if !mapsEqual(req.State.Filter, req.Inputs.Filter) {
		detailedDiff["filter"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// If any changes were detected, populate the diff response
	if len(detailedDiff) > 0 {
		diff.HasChanges = true
		diff.DetailedDiff = detailedDiff
	}

	return diff, nil
}

// mapsEqual compares two filter maps structurally (nested values included).
// nil and empty maps are treated as equal because both mean "no filter".
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// validateWebhookArgs validates the fully-resolved inputs at apply time.
func validateWebhookArgs(args WebhookArgs) error {
	if err := ValidateSiteID(args.SiteID); err != nil {
		return fmt.Errorf("validation failed for Webhook resource: %w", err)
	}
	if err := ValidateTriggerType(args.TriggerType); err != nil {
		return fmt.Errorf("validation failed for Webhook resource: %w", err)
	}
	if err := ValidateWebhookURL(args.URL); err != nil {
		return fmt.Errorf("validation failed for Webhook resource: %w", err)
	}
	if err := ValidateWebhookFilter(args.TriggerType, args.Filter); err != nil {
		return fmt.Errorf("validation failed for Webhook resource: %w", err)
	}
	return nil
}

// Create creates a new webhook on the Webflow site.
func (w *Webhook) Create(
	ctx context.Context, req infer.CreateRequest[WebhookArgs],
) (infer.CreateResponse[WebhookState], error) {
	// Log the start of webhook creation. The URL is deliberately not logged: it may carry
	// secrets (signing tokens, credentials) in its query string.
	log := NewLogContext(ctx).
		WithField("siteId", req.Inputs.SiteID).
		WithField("triggerType", req.Inputs.TriggerType)
	log.Info("Creating Webflow webhook")

	state := WebhookState{
		WebhookArgs: req.Inputs,
	}

	// Preview: return the inputs without an ID, without fabricated timestamps and without
	// calling the API. An empty ID tells the framework to present the ID and every output as
	// unknown to dependent resources. Inputs may still be unknown (zeroed) at this point, so
	// validation of the resolved values happens at apply time (Check validated the known ones).
	if req.DryRun {
		log.Debug("Dry run mode - skipping API call")
		return infer.CreateResponse[WebhookState]{Output: state}, nil
	}

	// Validate inputs BEFORE making API calls (all values are resolved at apply-time)
	if err := validateWebhookArgs(req.Inputs); err != nil {
		log.Errorf("Validation failed: %v", err)
		return infer.CreateResponse[WebhookState]{}, err
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		log.Errorf("Failed to create HTTP client: %v", err)
		return infer.CreateResponse[WebhookState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API
	log.Debug("Calling Webflow API to create webhook")
	response, err := PostWebhook(ctx, client, req.Inputs.SiteID, WebhookRequest{
		TriggerType: req.Inputs.TriggerType,
		URL:         req.Inputs.URL,
		Filter:      req.Inputs.Filter,
	})
	if err != nil {
		log.Errorf("Failed to create webhook via API: %v", err)
		return infer.CreateResponse[WebhookState]{}, fmt.Errorf("failed to create webhook: %w", err)
	}

	// Defensive check: Ensure Webflow API returned a valid webhook ID
	if response.ID == "" {
		log.Error("API returned empty webhook ID")
		return infer.CreateResponse[WebhookState]{}, errors.New(
			"webflow API returned empty webhook ID - " +
				"this is unexpected and may indicate an API issue")
	}

	log.WithField("webhookId", response.ID).Info("Webhook created successfully")

	// Populate state with API response data
	state.CreatedOn = response.CreatedOn
	state.LastTriggered = response.LastTriggered

	resourceID := GenerateWebhookResourceID(req.Inputs.SiteID, response.ID)

	return infer.CreateResponse[WebhookState]{
		ID:     resourceID,
		Output: state,
	}, nil
}

// Read retrieves the current state of a webhook from Webflow via GET /v2/webhooks/{webhook_id}.
// Used for drift detection and import operations (empty inputs and state). A 404 yields an
// empty ID; any other error is returned so transient failures never look like a deleted webhook.
func (w *Webhook) Read(
	ctx context.Context, req infer.ReadRequest[WebhookArgs, WebhookState],
) (infer.ReadResponse[WebhookArgs, WebhookState], error) {
	// Extract siteID and webhookID from resource ID
	siteID, webhookID, err := ExtractIDsFromWebhookResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	webhook, err := GetWebhook(ctx, client, webhookID)
	if err != nil {
		// Only "not found" signals deletion; every other failure (network, auth, rate
		// limiting, 5xx) is propagated.
		if IsNotFound(err) {
			return infer.ReadResponse[WebhookArgs, WebhookState]{ID: ""}, nil
		}
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, fmt.Errorf("failed to read webhook: %w", err)
	}

	// The webhook object carries its own siteId; a mismatch with the resource ID means the
	// resource ID (e.g. an import argument) names the wrong site.
	if webhook.SiteID != "" && webhook.SiteID != siteID {
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, fmt.Errorf(
			"webhook %s belongs to site %s, not site %s named in resource ID %q; "+
				"use the resource ID {siteId}/webhooks/{webhookId} with the webhook's own site",
			webhookID, webhook.SiteID, siteID, req.ID)
	}

	// filter has don't-care semantics: only track what Webflow reports when the program (or
	// the previous state) set a filter. Otherwise a filter set outside Pulumi is left alone,
	// which also keeps imported webhooks free of an unrequested filter input.
	var filter map[string]interface{}
	if len(req.Inputs.Filter) > 0 || len(req.State.Filter) > 0 {
		filter = webhook.Filter
	}

	// Build current state from API response
	currentInputs := WebhookArgs{
		SiteID:      siteID,
		TriggerType: webhook.TriggerType,
		URL:         webhook.URL,
		Filter:      filter,
	}
	currentState := WebhookState{
		WebhookArgs:   currentInputs,
		CreatedOn:     webhook.CreatedOn,
		LastTriggered: webhook.LastTriggered,
	}

	return infer.ReadResponse[WebhookArgs, WebhookState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update is not supported for webhooks.
// Webflow does not provide an update endpoint for webhooks.
// All changes require replacement (delete + recreate).
func (w *Webhook) Update(
	ctx context.Context, req infer.UpdateRequest[WebhookArgs, WebhookState],
) (infer.UpdateResponse[WebhookState], error) {
	// This should never be called because Diff marks all changes as UpdateReplace
	// But we implement it defensively to return a clear error message
	return infer.UpdateResponse[WebhookState]{}, errors.New(
		"webhooks cannot be updated in-place. " +
			"Webflow does not support updating webhooks - all changes require replacement. " +
			"This is a provider bug if you're seeing this error. " +
			"Please report this issue at https://github.com/jdetmar/pulumi-webflow/issues")
}

// Delete removes a webhook from the Webflow site.
func (w *Webhook) Delete(ctx context.Context, req infer.DeleteRequest[WebhookState]) (infer.DeleteResponse, error) {
	// Extract siteID and webhookID from resource ID. The parser guarantees both are
	// non-empty; the webhook ID format is not validated because Create accepts whatever
	// ID Webflow assigns, and Delete must be able to remove exactly that.
	_, webhookID, err := ExtractIDsFromWebhookResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	// Get HTTP client
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Call Webflow API (handles 404 gracefully for idempotency)
	if err := DeleteWebhook(ctx, client, webhookID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete webhook: %w", err)
	}

	return infer.DeleteResponse{}, nil
}
