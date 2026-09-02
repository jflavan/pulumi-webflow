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

// CollectionField is the resource controller for managing Webflow CMS collection fields.
// It implements the infer.CustomResource interface for full CRUD operations.
type CollectionField struct{}

// CollectionFieldArgs defines the input properties for the CollectionField resource.
type CollectionFieldArgs struct {
	// CollectionID is the Webflow collection ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	CollectionID string `pulumi:"collectionId"`
	// Type is the field type (PlainText, RichText, Image, etc.).
	// Cannot be changed after creation - requires replacement.
	Type string `pulumi:"type"`
	// DisplayName is the human-readable name of the field.
	// Example: "Title", "Description", "Author"
	DisplayName string `pulumi:"displayName"`
	// Slug is the URL-friendly slug for the field (optional, create-only).
	// If not provided, Webflow will auto-generate from displayName.
	// Example: "title", "description"
	Slug string `pulumi:"slug,optional"`
	// IsRequired indicates whether the field is required (optional, defaults to false).
	IsRequired bool `pulumi:"isRequired,optional"`
	// HelpText is optional help text shown in the CMS interface.
	HelpText string `pulumi:"helpText,optional"`
	// Validations contains type-specific validation rules (optional, create-only).
	// Example for Number: {"min": 0, "max": 100}
	Validations map[string]interface{} `pulumi:"validations,optional"`
	// Metadata carries the type-specific configuration required by Option fields
	// ({"options": [{"name": "..."}]}) and Reference/MultiReference fields ({"collectionId": "..."}).
	// Create-only.
	Metadata map[string]interface{} `pulumi:"metadata,optional"`
}

// CollectionFieldState defines the output properties for the CollectionField resource.
// It embeds CollectionFieldArgs to include input properties in the output.
type CollectionFieldState struct {
	CollectionFieldArgs
	// FieldID is the Webflow-assigned field ID (read-only).
	FieldID string `pulumi:"fieldId,optional"`
	// IsEditable indicates whether the field can be edited (read-only).
	IsEditable bool `pulumi:"isEditable,optional"`
}

// Annotate adds descriptions and constraints to the CollectionField resource.
func (f *CollectionField) Annotate(a infer.Annotator) {
	a.SetToken("index", "CollectionField")
	a.Describe(f, "Manages fields for a Webflow CMS collection. "+
		"Collection fields define the structure of content items in a collection. "+
		"Only displayName, helpText and isRequired can be updated in place; "+
		"type, slug, validations and metadata cannot be changed after creation and "+
		"changing them requires replacement (delete + recreate).")
}

// Annotate adds descriptions to the CollectionFieldArgs fields.
func (args *CollectionFieldArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.CollectionID,
		"The Webflow collection ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find collection IDs via the Webflow API or dashboard. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.Type,
		"The field type (e.g., 'PlainText', 'RichText', 'Image', 'Number'). "+
			"Supported types: "+supportedFieldTypeList+". "+
			"IMPORTANT: Cannot be changed after creation - changing this requires replacement.")

	a.Describe(&args.DisplayName,
		"The human-readable name of the field (e.g., 'Title', 'Description', 'Author'). "+
			"This name appears in the Webflow CMS interface. "+
			"Maximum length: 255 characters.")

	a.Describe(&args.Slug,
		"The URL-friendly slug for the field (optional, e.g., 'title', 'description'). "+
			"If not provided, Webflow will auto-generate a slug from the displayName and the generated "+
			"value is recorded in the outputs without causing a diff. "+
			"The slug is used in API requests and exports and cannot be changed after creation - "+
			"changing an explicit slug requires replacement.")

	a.Describe(&args.IsRequired,
		"Whether the field is required (optional, defaults to false). "+
			"When true, content items must provide a value for this field.")

	a.Describe(&args.HelpText,
		"Optional help text shown in the CMS interface (e.g., 'Enter the article title'). "+
			"Helps content editors understand what to enter in this field.")

	a.Describe(&args.Validations,
		"Type-specific validation rules (optional, create-only). "+
			"Different field types support different validations. "+
			"Example for Number type: {\"min\": 0, \"max\": 100}. "+
			"Example for PlainText type: {\"maxLength\": 500}. "+
			"Changing validations requires replacement. "+
			"Refer to Webflow API documentation for validation options for each field type.")

	a.Describe(&args.Metadata,
		"Type-specific configuration (create-only). "+
			"Required for Option fields: {\"options\": [{\"name\": \"Draft\"}, {\"name\": \"Published\"}]}. "+
			"Required for Reference and MultiReference fields: {\"collectionId\": \"<referenced collection ID>\"}. "+
			"Not accepted for other field types. Changing metadata requires replacement.")
}

// Annotate adds descriptions to the CollectionFieldState fields.
func (state *CollectionFieldState) Annotate(a infer.Annotator) {
	a.Describe(&state.FieldID,
		"The Webflow-assigned field ID (read-only). "+
			"This ID is automatically generated when the field is created.")

	a.Describe(&state.IsEditable,
		"Whether the field can be edited (read-only). "+
			"System fields may not be editable.")
}

// Diff determines what changes need to be made to the collection field resource.
//
// collectionId, type, slug, validations and metadata are create-only in the Webflow API
// and trigger replacement. displayName, isRequired and helpText are updated in place.
// Omitted slug/validations/metadata inputs mean "don't care" and never diff against the
// values Read recorded from the API; explicit values are compared as a subset of what the
// API reports, since Webflow decorates those objects with server-generated keys.
func (f *CollectionField) Diff(
	ctx context.Context, req infer.DiffRequest[CollectionFieldArgs, CollectionFieldState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}
	detailedDiff := map[string]p.PropertyDiff{}

	replace := func(property string) {
		detailedDiff[property] = p.PropertyDiff{Kind: p.UpdateReplace}
		diff.DeleteBeforeReplace = true
		diff.HasChanges = true
	}
	update := func(property string) {
		detailedDiff[property] = p.PropertyDiff{Kind: p.Update}
		diff.HasChanges = true
	}

	if req.State.CollectionID != req.Inputs.CollectionID {
		replace("collectionId")
	}
	if req.State.Type != req.Inputs.Type {
		replace("type")
	}
	if req.Inputs.Slug != "" && req.State.Slug != req.Inputs.Slug {
		replace("slug")
	}
	if len(req.Inputs.Validations) > 0 && !subsetEqual(req.Inputs.Validations, req.State.Validations) {
		replace("validations")
	}
	if len(req.Inputs.Metadata) > 0 && !subsetEqual(req.Inputs.Metadata, req.State.Metadata) {
		replace("metadata")
	}

	if req.State.DisplayName != req.Inputs.DisplayName {
		update("displayName")
	}
	if req.State.IsRequired != req.Inputs.IsRequired {
		update("isRequired")
	}
	if req.State.HelpText != req.Inputs.HelpText {
		update("helpText")
	}

	if len(detailedDiff) > 0 {
		diff.DetailedDiff = detailedDiff
	}
	return diff, nil
}

// validateCollectionFieldArgs validates the inputs shared by Create and Update.
func validateCollectionFieldArgs(args CollectionFieldArgs) error {
	if err := ValidateCollectionID(args.CollectionID); err != nil {
		return fmt.Errorf("validation failed for CollectionField resource: %w", err)
	}
	if err := ValidateFieldType(args.Type); err != nil {
		return fmt.Errorf("validation failed for CollectionField resource: %w", err)
	}
	if err := ValidateFieldDisplayName(args.DisplayName); err != nil {
		return fmt.Errorf("validation failed for CollectionField resource: %w", err)
	}
	if err := ValidateFieldMetadata(args.Type, args.Metadata); err != nil {
		return fmt.Errorf("validation failed for CollectionField resource: %w", err)
	}
	return nil
}

// Create creates a new field for a Webflow collection.
func (f *CollectionField) Create(
	ctx context.Context, req infer.CreateRequest[CollectionFieldArgs],
) (infer.CreateResponse[CollectionFieldState], error) {
	state := CollectionFieldState{
		CollectionFieldArgs: req.Inputs,
		IsEditable:          true,
	}

	// During preview, return the expected state without validating or calling the API:
	// inputs that come from other resources (e.g. collectionId) are unknown at this point.
	if req.DryRun {
		return infer.CreateResponse[CollectionFieldState]{
			ID:     GenerateCollectionFieldResourceID(req.Inputs.CollectionID, "preview"),
			Output: state,
		}, nil
	}

	if err := validateCollectionFieldArgs(req.Inputs); err != nil {
		return infer.CreateResponse[CollectionFieldState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[CollectionFieldState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	isRequired := req.Inputs.IsRequired
	response, err := PostCollectionField(ctx, client, req.Inputs.CollectionID, CollectionFieldCreateRequest{
		Type:        req.Inputs.Type,
		DisplayName: req.Inputs.DisplayName,
		Slug:        req.Inputs.Slug,
		IsRequired:  &isRequired,
		HelpText:    req.Inputs.HelpText,
		Validations: req.Inputs.Validations,
		Metadata:    req.Inputs.Metadata,
	})
	if err != nil {
		return infer.CreateResponse[CollectionFieldState]{}, fmt.Errorf("failed to create collection field: %w", err)
	}

	if response.ID == "" {
		return infer.CreateResponse[CollectionFieldState]{}, errors.New(
			"webflow API returned empty field ID - " +
				"this is unexpected and may indicate an API issue")
	}

	state.FieldID = response.ID
	state.IsEditable = response.IsEditable
	// Record the slug Webflow assigned; the input stays as the user gave it.
	if response.Slug != "" {
		state.Slug = response.Slug
	}

	return infer.CreateResponse[CollectionFieldState]{
		ID:     GenerateCollectionFieldResourceID(req.Inputs.CollectionID, response.ID),
		Output: state,
	}, nil
}

// Read retrieves the current state of a collection field from Webflow.
// Used for drift detection and import operations.
func (f *CollectionField) Read(
	ctx context.Context, req infer.ReadRequest[CollectionFieldArgs, CollectionFieldState],
) (infer.ReadResponse[CollectionFieldArgs, CollectionFieldState], error) {
	collectionID, fieldID, err := ExtractIDsFromCollectionFieldResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[CollectionFieldArgs, CollectionFieldState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.ReadResponse[CollectionFieldArgs, CollectionFieldState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateFieldID(fieldID); err != nil {
		return infer.ReadResponse[CollectionFieldArgs, CollectionFieldState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[CollectionFieldArgs, CollectionFieldState]{}, fmt.Errorf(
			"failed to create HTTP client: %w", err)
	}

	response, err := GetCollectionField(ctx, client, collectionID, fieldID)
	if err != nil {
		if IsNotFound(err) {
			// Collection or field no longer exists - return empty ID to signal deletion
			return infer.ReadResponse[CollectionFieldArgs, CollectionFieldState]{}, nil
		}
		return infer.ReadResponse[CollectionFieldArgs, CollectionFieldState]{}, fmt.Errorf(
			"failed to read collection field: %w", err)
	}

	// Inputs the user omitted stay omitted (slug, validations, metadata are "don't care"
	// when absent); explicitly provided ones are refreshed from the API so drift is visible.
	currentInputs := CollectionFieldArgs{
		CollectionID: collectionID,
		Type:         response.Type,
		DisplayName:  response.DisplayName,
		Slug:         req.Inputs.Slug,
		IsRequired:   response.IsRequired,
		HelpText:     response.HelpText,
		Validations:  req.Inputs.Validations,
		Metadata:     req.Inputs.Metadata,
	}
	if req.Inputs.Slug != "" {
		currentInputs.Slug = response.Slug
	}
	if len(req.Inputs.Validations) > 0 {
		currentInputs.Validations = response.Validations
	}
	if len(req.Inputs.Metadata) > 0 {
		currentInputs.Metadata = response.Metadata
	}

	currentState := CollectionFieldState{
		CollectionFieldArgs: currentInputs,
		FieldID:             response.ID,
		IsEditable:          response.IsEditable,
	}
	currentState.Slug = response.Slug
	currentState.Validations = response.Validations
	currentState.Metadata = response.Metadata

	return infer.ReadResponse[CollectionFieldArgs, CollectionFieldState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update modifies the mutable properties (displayName, isRequired, helpText) of a field.
func (f *CollectionField) Update(
	ctx context.Context, req infer.UpdateRequest[CollectionFieldArgs, CollectionFieldState],
) (infer.UpdateResponse[CollectionFieldState], error) {
	state := CollectionFieldState{
		CollectionFieldArgs: req.Inputs,
		FieldID:             req.State.FieldID,    // Preserve field ID
		IsEditable:          req.State.IsEditable, // Preserve editability flag
	}
	// Keep the Webflow-assigned slug when the user did not specify one.
	if state.Slug == "" {
		state.Slug = req.State.Slug
	}

	// During preview, return expected state without making API calls
	if req.DryRun {
		return infer.UpdateResponse[CollectionFieldState]{Output: state}, nil
	}

	if err := validateCollectionFieldArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[CollectionFieldState]{}, err
	}

	collectionID, fieldID, err := ExtractIDsFromCollectionFieldResourceID(req.ID)
	if err != nil {
		return infer.UpdateResponse[CollectionFieldState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.UpdateResponse[CollectionFieldState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateFieldID(fieldID); err != nil {
		return infer.UpdateResponse[CollectionFieldState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.UpdateResponse[CollectionFieldState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	isRequired := req.Inputs.IsRequired
	response, err := PatchCollectionField(ctx, client, collectionID, fieldID, CollectionFieldUpdateRequest{
		IsRequired:  &isRequired,
		DisplayName: req.Inputs.DisplayName,
		HelpText:    req.Inputs.HelpText,
	})
	if err != nil {
		return infer.UpdateResponse[CollectionFieldState]{}, fmt.Errorf("failed to update collection field: %w", err)
	}

	state.IsEditable = response.IsEditable
	if response.Slug != "" {
		state.Slug = response.Slug
	}

	return infer.UpdateResponse[CollectionFieldState]{Output: state}, nil
}

// Delete removes a field from a Webflow collection.
func (f *CollectionField) Delete(
	ctx context.Context, req infer.DeleteRequest[CollectionFieldState],
) (infer.DeleteResponse, error) {
	collectionID, fieldID, err := ExtractIDsFromCollectionFieldResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateFieldID(fieldID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// 404 is treated as success so deletes are idempotent
	if err := DeleteCollectionField(ctx, client, collectionID, fieldID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete collection field: %w", err)
	}

	return infer.DeleteResponse{}, nil
}
