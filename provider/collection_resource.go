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

// CollectionResource is the resource controller for managing Webflow CMS collections.
// It implements the infer.CustomResource interface for full CRUD operations.
type CollectionResource struct{}

// CollectionArgs defines the input properties for the Collection resource.
type CollectionArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	SiteID string `pulumi:"siteId"`
	// DisplayName is the human-readable name of the collection.
	// Example: "Blog Posts", "Products", "Team Members"
	DisplayName string `pulumi:"displayName"`
	// SingularName is the singular form of the collection name.
	// Example: "Blog Post" for "Blog Posts", "Product" for "Products"
	SingularName string `pulumi:"singularName"`
	// Slug is the URL-friendly slug for the collection (optional).
	// If not provided, Webflow will auto-generate from displayName.
	// Example: "blog-posts", "products"
	Slug string `pulumi:"slug,optional"`
}

// CollectionState defines the output properties for the Collection resource.
// It embeds CollectionArgs to include input properties in the output.
type CollectionState struct {
	CollectionArgs
	// CollectionID is the Webflow-assigned collection ID (read-only).
	// This is the raw 24-character ID that can be used with CollectionField and CollectionItem resources.
	CollectionID string `pulumi:"collectionId,optional"`
	// CreatedOn is the timestamp when the collection was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
	// LastUpdated is the timestamp when the collection was last updated (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
}

// Annotate adds descriptions and constraints to the Collection resource.
func (c *CollectionResource) Annotate(a infer.Annotator) {
	a.SetToken("index", "Collection")
	a.Describe(c, "Manages CMS collections for a Webflow site. "+
		"Collections are containers for structured content items (blog posts, products, etc.). "+
		"displayName, singularName and slug are updated in place (PATCH /v2/collections/{collection_id}); "+
		"changing siteId requires replacement (delete + recreate).")
}

// Annotate adds descriptions to the CollectionArgs fields.
func (args *CollectionArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.DisplayName,
		"The human-readable name of the collection (e.g., 'Blog Posts', 'Products', 'Team Members'). "+
			"This name appears in the Webflow CMS interface. "+
			"Maximum length: 255 characters.")

	a.Describe(&args.SingularName,
		"The singular form of the collection name (e.g., 'Blog Post' for 'Blog Posts', 'Product' for 'Products'). "+
			"Used in the CMS UI when referring to individual items. "+
			"Maximum length: 255 characters.")

	a.Describe(&args.Slug,
		"The URL-friendly slug for the collection (optional, e.g., 'blog-posts', 'products'). "+
			"If not provided, Webflow will auto-generate a slug from the displayName and the generated "+
			"value is recorded in the resource outputs without causing a diff. "+
			"The slug determines the URL path for collection items; changing an explicit slug updates "+
			"the collection in place, which changes the URLs of its items.")
}

// Annotate adds descriptions to the CollectionState fields.
func (state *CollectionState) Annotate(a infer.Annotator) {
	a.Describe(&state.CollectionID,
		"The Webflow-assigned collection ID (24-character lowercase hexadecimal string). "+
			"Use this ID when creating CollectionField or CollectionItem resources. "+
			"This is automatically assigned when the collection is created and is read-only.")

	a.Describe(&state.CreatedOn,
		"The timestamp when the collection was created (RFC3339 format). "+
			"This is automatically set by Webflow and is read-only.")

	a.Describe(&state.LastUpdated,
		"The timestamp when the collection was last updated (RFC3339 format). "+
			"This is automatically updated by Webflow and is read-only.")
}

// collectionValidators are the per-property checks shared by Check and the apply-time validation.
var collectionValidators = []stringValidator{
	{property: "siteId", validate: ValidateSiteID},
	{property: "displayName", validate: ValidateCollectionDisplayName},
	{property: "singularName", validate: ValidateSingularName},
	{property: "slug", validate: ValidateCollectionSlug},
}

// Check validates the known inputs at preview time. Unknown (computed) values, such as a
// siteId that comes from a Site resource, are skipped and validated again in Create/Update.
func (c *CollectionResource) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[CollectionArgs], error) {
	inputs, failures, err := checkStrings[CollectionArgs](ctx, req.NewInputs, collectionValidators...)
	return infer.CheckResponse[CollectionArgs]{Inputs: inputs, Failures: failures}, err
}

// validateCollectionArgs validates fully-resolved inputs at apply time.
func validateCollectionArgs(args CollectionArgs) error {
	if err := ValidateSiteID(args.SiteID); err != nil {
		return fmt.Errorf("validation failed for Collection resource: %w", err)
	}
	if err := ValidateCollectionDisplayName(args.DisplayName); err != nil {
		return fmt.Errorf("validation failed for Collection resource: %w", err)
	}
	if err := ValidateSingularName(args.SingularName); err != nil {
		return fmt.Errorf("validation failed for Collection resource: %w", err)
	}
	if err := ValidateCollectionSlug(args.Slug); err != nil {
		return fmt.Errorf("validation failed for Collection resource: %w", err)
	}
	return nil
}

// Diff determines what changes need to be made to the collection resource.
// displayName, singularName and slug are updated in place through
// PATCH /v2/collections/{collection_id}; only a siteId change requires replacement.
//
// An omitted slug input means "let Webflow choose": it never diffs against the
// Webflow-generated slug that Create or Read recorded in state.
func (c *CollectionResource) Diff(
	ctx context.Context, req infer.DiffRequest[CollectionArgs, CollectionState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}
	detailedDiff := map[string]p.PropertyDiff{}

	update := func(property string) {
		detailedDiff[property] = p.PropertyDiff{Kind: p.Update}
		diff.HasChanges = true
	}

	if req.State.SiteID != req.Inputs.SiteID {
		detailedDiff["siteId"] = p.PropertyDiff{Kind: p.UpdateReplace}
		diff.DeleteBeforeReplace = true
		diff.HasChanges = true
	}
	if req.State.DisplayName != req.Inputs.DisplayName {
		update("displayName")
	}
	if req.State.SingularName != req.Inputs.SingularName {
		update("singularName")
	}
	if req.Inputs.Slug != "" && req.State.Slug != req.Inputs.Slug {
		update("slug")
	}

	if len(detailedDiff) > 0 {
		diff.DetailedDiff = detailedDiff
	}
	return diff, nil
}

// Create creates a new collection on the Webflow site.
func (c *CollectionResource) Create(
	ctx context.Context, req infer.CreateRequest[CollectionArgs],
) (infer.CreateResponse[CollectionState], error) {
	log := NewLogContext(ctx).
		WithField("siteId", req.Inputs.SiteID).
		WithField("displayName", req.Inputs.DisplayName).
		WithField("singularName", req.Inputs.SingularName)

	state := CollectionState{CollectionArgs: req.Inputs}

	// During preview, return the expected state without making API calls or validating:
	// inputs that come from other resources are unknown (empty) at this point.
	// The ID is left empty so dependents see it as unknown, and server-assigned outputs
	// (collectionId, timestamps) are left empty rather than fabricated.
	if req.DryRun {
		log.Debug("Dry run mode - skipping API call")
		return infer.CreateResponse[CollectionState]{Output: state}, nil
	}

	log.Info("Creating Webflow collection")

	if err := validateCollectionArgs(req.Inputs); err != nil {
		return infer.CreateResponse[CollectionState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[CollectionState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := PostCollection(
		ctx, client, req.Inputs.SiteID,
		req.Inputs.DisplayName, req.Inputs.SingularName, req.Inputs.Slug,
	)
	if err != nil {
		log.Errorf("Failed to create collection via API: %v", err)
		return infer.CreateResponse[CollectionState]{}, fmt.Errorf("failed to create collection: %w", err)
	}

	if response.ID == "" {
		return infer.CreateResponse[CollectionState]{}, errors.New(
			"webflow API returned empty collection ID - " +
				"this is unexpected and may indicate an API issue")
	}

	log.WithField("collectionId", response.ID).Info("Collection created successfully")

	state.CollectionID = response.ID
	state.CreatedOn = response.CreatedOn
	state.LastUpdated = response.LastUpdated
	// Record the slug Webflow actually assigned (it generates one when the input is omitted).
	// The input value is untouched, so an omitted slug stays omitted for Diff purposes.
	if response.Slug != "" {
		state.Slug = response.Slug
	}

	return infer.CreateResponse[CollectionState]{
		ID:     GenerateCollectionResourceID(req.Inputs.SiteID, response.ID),
		Output: state,
	}, nil
}

// Read retrieves the current state of a collection from Webflow.
// Used for drift detection and import operations.
func (c *CollectionResource) Read(
	ctx context.Context, req infer.ReadRequest[CollectionArgs, CollectionState],
) (infer.ReadResponse[CollectionArgs, CollectionState], error) {
	siteID, collectionID, err := ExtractIDsFromCollectionResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[CollectionArgs, CollectionState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return infer.ReadResponse[CollectionArgs, CollectionState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.ReadResponse[CollectionArgs, CollectionState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[CollectionArgs, CollectionState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := GetCollection(ctx, client, collectionID)
	if err != nil {
		if IsNotFound(err) {
			// Resource no longer exists - return empty ID to signal deletion
			return infer.ReadResponse[CollectionArgs, CollectionState]{}, nil
		}
		return infer.ReadResponse[CollectionArgs, CollectionState]{}, fmt.Errorf("failed to read collection: %w", err)
	}

	currentInputs := CollectionArgs{
		SiteID:       siteID,
		DisplayName:  response.DisplayName,
		SingularName: response.SingularName,
		// An omitted slug input stays omitted; only an explicit slug is refreshed from the API.
		Slug: req.Inputs.Slug,
	}
	if req.Inputs.Slug != "" {
		currentInputs.Slug = response.Slug
	}

	currentState := CollectionState{
		CollectionArgs: currentInputs,
		CollectionID:   collectionID,
		CreatedOn:      response.CreatedOn,
		LastUpdated:    response.LastUpdated,
	}
	currentState.Slug = response.Slug

	return infer.ReadResponse[CollectionArgs, CollectionState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update changes the display name, singular name and/or slug of the collection in place
// through PATCH /v2/collections/{collection_id}. An omitted slug is not sent, so the
// Webflow-assigned slug is kept.
func (c *CollectionResource) Update(
	ctx context.Context, req infer.UpdateRequest[CollectionArgs, CollectionState],
) (infer.UpdateResponse[CollectionState], error) {
	state := CollectionState{
		CollectionArgs: req.Inputs,
		CollectionID:   req.State.CollectionID,
		CreatedOn:      req.State.CreatedOn,
		LastUpdated:    req.State.LastUpdated,
	}
	// Keep the Webflow-assigned slug when the user did not specify one.
	if state.Slug == "" {
		state.Slug = req.State.Slug
	}

	// During preview, return the expected state without making API calls.
	if req.DryRun {
		return infer.UpdateResponse[CollectionState]{Output: state}, nil
	}

	if err := validateCollectionArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[CollectionState]{}, err
	}

	_, collectionID, err := ExtractIDsFromCollectionResourceID(req.ID)
	if err != nil {
		return infer.UpdateResponse[CollectionState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.UpdateResponse[CollectionState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.UpdateResponse[CollectionState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	response, err := PatchCollection(ctx, client, collectionID, CollectionUpdateRequest{
		DisplayName:  req.Inputs.DisplayName,
		SingularName: req.Inputs.SingularName,
		Slug:         req.Inputs.Slug,
	})
	if err != nil {
		return infer.UpdateResponse[CollectionState]{}, fmt.Errorf("failed to update collection: %w", err)
	}

	NewLogContext(ctx).WithField("collectionId", collectionID).Info("Collection updated successfully")

	if state.CollectionID == "" {
		state.CollectionID = collectionID
	}
	if response.Slug != "" {
		state.Slug = response.Slug
	}
	if response.CreatedOn != "" {
		state.CreatedOn = response.CreatedOn
	}
	if response.LastUpdated != "" {
		state.LastUpdated = response.LastUpdated
	}

	return infer.UpdateResponse[CollectionState]{Output: state}, nil
}

// Delete removes a collection from the Webflow site.
func (c *CollectionResource) Delete(
	ctx context.Context, req infer.DeleteRequest[CollectionState],
) (infer.DeleteResponse, error) {
	_, collectionID, err := ExtractIDsFromCollectionResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 404 is treated as success so deletes are idempotent
	if err := DeleteCollection(ctx, client, collectionID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete collection: %w", err)
	}

	return infer.DeleteResponse{}, nil
}
