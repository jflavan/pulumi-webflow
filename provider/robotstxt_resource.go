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
	"sync/atomic"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// RobotsTxt is the resource controller for managing robots.txt configuration.
// It implements the infer.CustomResource interface for full CRUD operations.
type RobotsTxt struct{}

// RobotsTxtArgs defines the input properties for the RobotsTxt resource.
type RobotsTxtArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	SiteID string `pulumi:"siteId"`
	// Content is the robots.txt content in traditional format.
	Content string `pulumi:"content"`
}

// RobotsTxtState defines the output properties for the RobotsTxt resource.
// It embeds RobotsTxtArgs to include input properties in the output.
type RobotsTxtState struct {
	RobotsTxtArgs
	// LastModified is the RFC3339 timestamp of the last modification.
	LastModified string `pulumi:"lastModified"`
}

// Annotate adds descriptions and constraints to the RobotsTxt resource.
func (r *RobotsTxt) Annotate(a infer.Annotator) {
	a.SetToken("index", "RobotsTxt")
	a.Describe(r, "Manages robots.txt configuration for a Webflow site. "+
		"This resource allows you to define crawler access rules and sitemap references.")
}

// Annotate adds descriptions to the RobotsTxtArgs fields.
func (args *RobotsTxtArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3').")
	a.Describe(&args.Content, "The robots.txt content in traditional format. "+
		"Supports User-agent, Allow, Disallow, and Sitemap directives. "+
		"Comments and other directives are not stored by Webflow and are dropped with a warning. "+
		"Formatting differences (blank lines, spacing, directive casing) do not cause a diff.")
}

// Annotate adds descriptions to the RobotsTxtState fields.
func (state *RobotsTxtState) Annotate(a infer.Annotator) {
	a.Describe(&state.LastModified, "RFC3339 timestamp of the last modification made through this provider. "+
		"The Webflow API does not report one, so the provider records the time of its last successful write.")
}

// errRobotsTxtContentRequiredShort is the Check-time reason for an empty content value.
var errRobotsTxtContentRequiredShort = errors.New("content is required: provide robots.txt content with at " +
	"least one directive (e.g., 'User-agent: *\\nAllow: /')")

// validateRobotsTxtContent reports an error for empty content.
func validateRobotsTxtContent(content string) error {
	if content == "" {
		return errRobotsTxtContentRequiredShort
	}
	return nil
}

// robotsTxtCheckValidators lists the known-value validators applied by Check.
var robotsTxtCheckValidators = []stringValidator{
	{property: "siteId", validate: ValidateSiteID},
	{property: "content", validate: validateRobotsTxtContent},
}

// Check validates the inputs that are already known at preview time. Values that still depend
// on other resources' outputs are skipped here and validated again in Create or Update.
func (r *RobotsTxt) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[RobotsTxtArgs], error) {
	inputs, failures, err := checkStrings[RobotsTxtArgs](ctx, req.NewInputs, robotsTxtCheckValidators...)
	return infer.CheckResponse[RobotsTxtArgs]{Inputs: inputs, Failures: failures}, err
}

// Diff determines what changes need to be made to the resource.
// siteId changes trigger replacement; content changes trigger update. Content is compared
// after parsing, so formatting-only differences between the program and the normalized
// content returned by Webflow do not produce a permanent diff.
func (r *RobotsTxt) Diff(
	ctx context.Context, req infer.DiffRequest[RobotsTxtArgs, RobotsTxtState],
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

	// Check for a semantic content change (in-place update)
	if !RobotsTxtContentEqual(req.State.Content, req.Inputs.Content) {
		diff.HasChanges = true
		diff.DetailedDiff = map[string]p.PropertyDiff{
			"content": {Kind: p.Update},
		}
	}

	return diff, nil
}

// errRobotsTxtContentRequired is returned when content is empty.
var errRobotsTxtContentRequired = errors.New(
	"validation failed for RobotsTxt resource: " +
		"content is required but was not provided. " +
		"Please provide robots.txt content with at least one directive " +
		"(e.g., 'User-agent: *\\nAllow: /'). " +
		"The content should follow the traditional robots.txt format " +
		"with User-agent, Allow, Disallow, and Sitemap directives")

// validateRobotsTxtArgs validates the RobotsTxt inputs.
func validateRobotsTxtArgs(args RobotsTxtArgs) error {
	if err := ValidateSiteID(args.SiteID); err != nil {
		return fmt.Errorf("validation failed for RobotsTxt resource: %w", err)
	}
	if args.Content == "" {
		return errRobotsTxtContentRequired
	}
	return nil
}

// parseRobotsTxtInput parses the user's content and logs one warning per dropped line.
func parseRobotsTxtInput(ctx context.Context, siteID, content string) (rules []RobotsTxtRule, sitemap string) {
	rules, sitemap, warnings := ParseRobotsTxtContentWithWarnings(content)
	if len(warnings) > 0 {
		log := NewLogContext(ctx).WithField("siteId", siteID)
		for _, w := range warnings {
			log.Warnf("robots.txt content: %s", w)
		}
	}
	return rules, sitemap
}

// parseRobotsTxtResourceID extracts and validates the site ID from a resource ID.
func parseRobotsTxtResourceID(resourceID string) (string, error) {
	siteID, err := ExtractSiteIDFromResourceID(resourceID)
	if err != nil {
		return "", fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return "", fmt.Errorf("invalid resource ID %q: %w", resourceID, err)
	}
	return siteID, nil
}

// Create creates a new robots.txt configuration on the Webflow site.
func (r *RobotsTxt) Create(
	ctx context.Context, req infer.CreateRequest[RobotsTxtArgs],
) (infer.CreateResponse[RobotsTxtState], error) {
	state := RobotsTxtState{
		RobotsTxtArgs: req.Inputs,
	}

	// Preview: return the inputs without an ID, without a fabricated lastModified and without
	// calling the API. An empty ID tells the framework to present the ID and every output as
	// unknown to dependent resources. siteId may still be unknown (zeroed) during preview.
	if req.DryRun {
		return infer.CreateResponse[RobotsTxtState]{Output: state}, nil
	}

	if err := validateRobotsTxtArgs(req.Inputs); err != nil {
		return infer.CreateResponse[RobotsTxtState]{}, err
	}
	resourceID := GenerateRobotsTxtResourceID(req.Inputs.SiteID)

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[RobotsTxtState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	rules, sitemap := parseRobotsTxtInput(ctx, req.Inputs.SiteID, req.Inputs.Content)

	if _, err := PutRobotsTxt(ctx, client, req.Inputs.SiteID, rules, sitemap); err != nil {
		return infer.CreateResponse[RobotsTxtState]{}, fmt.Errorf("failed to create robots.txt: %w", err)
	}

	// Keep the user's raw content in state so Diff compares like with like.
	state.Content = req.Inputs.Content
	state.LastModified = time.Now().UTC().Format(time.RFC3339)

	return infer.CreateResponse[RobotsTxtState]{
		ID:     resourceID,
		Output: state,
	}, nil
}

// Read retrieves the current state of the robots.txt from Webflow.
// Used for drift detection and import operations. A 404 yields an empty ID; any other error
// is returned so transient failures are never mistaken for a deleted resource.
func (r *RobotsTxt) Read(
	ctx context.Context, req infer.ReadRequest[RobotsTxtArgs, RobotsTxtState],
) (infer.ReadResponse[RobotsTxtArgs, RobotsTxtState], error) {
	siteID, err := parseRobotsTxtResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[RobotsTxtArgs, RobotsTxtState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[RobotsTxtArgs, RobotsTxtState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := GetRobotsTxt(ctx, client, siteID)
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[RobotsTxtArgs, RobotsTxtState]{ID: ""}, nil
		}
		return infer.ReadResponse[RobotsTxtArgs, RobotsTxtState]{}, fmt.Errorf("failed to read robots.txt: %w", err)
	}

	// Webflow returns a normalized rule set. When it still describes what the user wrote,
	// keep the user's raw text so refresh does not rewrite inputs and cause a spurious diff.
	content := FormatRobotsTxtContent(response.Rules, response.Sitemap)
	switch {
	case req.Inputs.Content != "" && RobotsTxtContentEqual(req.Inputs.Content, content):
		content = req.Inputs.Content
	case req.State.Content != "" && RobotsTxtContentEqual(req.State.Content, content):
		content = req.State.Content
	}

	currentInputs := RobotsTxtArgs{
		SiteID:  siteID,
		Content: content,
	}
	currentState := RobotsTxtState{
		RobotsTxtArgs: currentInputs,
		// The Webflow API doesn't return a last-modified timestamp; regenerating it on every
		// Read would cause false drift, so preserve the stored value.
		LastModified: req.State.LastModified,
	}

	return infer.ReadResponse[RobotsTxtArgs, RobotsTxtState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update modifies an existing robots.txt configuration.
func (r *RobotsTxt) Update(
	ctx context.Context, req infer.UpdateRequest[RobotsTxtArgs, RobotsTxtState],
) (infer.UpdateResponse[RobotsTxtState], error) {
	state := RobotsTxtState{
		RobotsTxtArgs: req.Inputs,
	}

	// Preview: return the expected state without validating (inputs may be unknown), without
	// fabricating lastModified, and without calling the API.
	if req.DryRun {
		return infer.UpdateResponse[RobotsTxtState]{
			Output: state,
		}, nil
	}

	if err := validateRobotsTxtArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[RobotsTxtState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.UpdateResponse[RobotsTxtState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	rules, sitemap := parseRobotsTxtInput(ctx, req.Inputs.SiteID, req.Inputs.Content)

	if _, err := PutRobotsTxt(ctx, client, req.Inputs.SiteID, rules, sitemap); err != nil {
		return infer.UpdateResponse[RobotsTxtState]{}, fmt.Errorf("failed to update robots.txt: %w", err)
	}

	// Keep the user's raw content in state so Diff compares like with like.
	state.Content = req.Inputs.Content
	state.LastModified = time.Now().UTC().Format(time.RFC3339)

	return infer.UpdateResponse[RobotsTxtState]{
		Output: state,
	}, nil
}

// Delete removes the managed rules from the site's robots.txt configuration.
// The Webflow DELETE endpoint expects the rules to remove in the request body, so the rules
// currently held in state (and the sitemap) are sent.
func (r *RobotsTxt) Delete(ctx context.Context, req infer.DeleteRequest[RobotsTxtState]) (infer.DeleteResponse, error) {
	siteID, err := parseRobotsTxtResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	rules, sitemap := ParseRobotsTxtContent(req.State.Content)

	// 200/204 are success; 404 is treated as success for idempotency
	if err := DeleteRobotsTxt(ctx, client, siteID, rules, sitemap); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete robots.txt: %w", err)
	}

	return infer.DeleteResponse{}, nil
}

// providerVersionValue holds the provider version reported in the User-Agent header.
// It is set once by Provider() and read concurrently by every resource operation.
var providerVersionValue atomic.Value

// SetProviderVersion sets the provider version for use in API calls.
func SetProviderVersion(version string) {
	providerVersionValue.Store(version)
}

// currentProviderVersion returns the version set by SetProviderVersion, or "0.0.0".
func currentProviderVersion() string {
	if v, ok := providerVersionValue.Load().(string); ok && v != "" {
		return v
	}
	return "0.0.0"
}
