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

// AssetFolder is the resource controller for managing Webflow asset folders.
// It implements the infer.CustomResource interface for CRUD operations.
// Note: The Webflow API does not support delete or update operations for asset folders.
// Deleting this resource will only remove it from Pulumi state, not from Webflow.
type AssetFolder struct{}

var _ infer.CustomCheck[AssetFolderArgs] = (*AssetFolder)(nil)

// AssetFolderArgs defines the input properties for the AssetFolder resource.
type AssetFolderArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	SiteID string `pulumi:"siteId"`
	// DisplayName is the human-readable name for the asset folder.
	DisplayName string `pulumi:"displayName"`
	// ParentFolder is the optional ID of the parent folder.
	ParentFolder string `pulumi:"parentFolder,optional"`
}

// AssetFolderState defines the output properties for the AssetFolder resource.
// It embeds AssetFolderArgs to include input properties in the output.
type AssetFolderState struct {
	AssetFolderArgs
	// FolderID is the Webflow-assigned folder ID (read-only).
	FolderID string `pulumi:"folderId,optional"`
	// Assets is the list of asset IDs contained in this folder (read-only).
	Assets []string `pulumi:"assets,optional"`
	// CreatedOn is the timestamp when the folder was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
	// LastUpdated is the timestamp when the folder was last modified (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
}

// assetFolderLeftBehindWarning is logged whenever a folder is replaced or removed from state,
// because Webflow has no API to delete asset folders.
const assetFolderLeftBehindWarning = "The Webflow API cannot delete asset folders: this folder will be " +
	"removed from Pulumi state but remains in Webflow. Delete it manually in the Webflow Assets panel " +
	"if it is no longer needed"

// Annotate adds descriptions and constraints to the AssetFolder resource.
func (r *AssetFolder) Annotate(a infer.Annotator) {
	a.SetToken("index", "AssetFolder")
	a.Describe(r, "Manages asset folders for organizing files in a Webflow site. "+
		"This resource allows you to create folders to organize your assets (images, documents, etc.) "+
		"in the Webflow Assets panel. "+
		"NOTE: The Webflow API does not support deleting or updating asset folders. "+
		"Deleting this resource only removes it from Pulumi state; the folder remains in Webflow. "+
		"Changing displayName, parentFolder or siteId creates a new folder and leaves the old one behind.")
}

// Annotate adds descriptions to the AssetFolderArgs fields.
func (args *AssetFolderArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.DisplayName,
		"The human-readable name for the asset folder. "+
			"This name appears in the Webflow Assets panel and helps organize your files. "+
			"Examples: 'Images', 'Documents', 'Icons', 'Hero Backgrounds'. "+
			"Maximum length: 255 characters.")

	a.Describe(&args.ParentFolder,
		"Optional ID of the parent folder for creating nested folder structures. "+
			"If not specified, the folder will be created at the root level of the Assets panel. "+
			"Example: '5f0c8c9e1c9d440000e8d8c4'.")
}

// Annotate adds descriptions to the AssetFolderState fields.
func (state *AssetFolderState) Annotate(a infer.Annotator) {
	a.Describe(&state.FolderID,
		"The Webflow-assigned folder ID (read-only). "+
			"This unique identifier can be used to reference the folder in other resources, "+
			"such as when uploading assets to this folder.")

	a.Describe(&state.Assets,
		"List of asset IDs currently contained in this folder (read-only). "+
			"This is automatically populated by Webflow when assets are added to the folder.")

	a.Describe(&state.CreatedOn,
		"The timestamp when the folder was created (RFC3339 format, read-only). "+
			"This is automatically set when the folder is created.")

	a.Describe(&state.LastUpdated,
		"The timestamp when the folder was last modified (RFC3339 format, read-only). "+
			"This is updated when assets are added or removed from the folder.")
}

// Check validates the known inputs at preview time: siteId and parentFolder formats and a
// non-empty displayName. Unknown values are validated again at apply time.
func (r *AssetFolder) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[AssetFolderArgs], error) {
	inputs, failures, err := checkStrings[AssetFolderArgs](ctx, req.NewInputs,
		stringValidator{property: "siteId", validate: ValidateSiteID},
		stringValidator{property: "displayName", validate: ValidateDisplayName},
		stringValidator{property: "parentFolder", validate: validateOptionalAssetFolderID},
	)
	return infer.CheckResponse[AssetFolderArgs]{Inputs: inputs, Failures: failures}, err
}

// Diff determines what changes need to be made to the asset folder resource.
// Since Webflow doesn't support updates, all changes require replacement. The old folder is
// created-before-deleted (the delete is a no-op anyway) and a warning tells the user it stays behind.
func (r *AssetFolder) Diff(
	ctx context.Context, req infer.DiffRequest[AssetFolderArgs, AssetFolderState],
) (infer.DiffResponse, error) {
	var changed string
	switch {
	case req.State.SiteID != req.Inputs.SiteID:
		changed = "siteId"
	case req.State.DisplayName != req.Inputs.DisplayName:
		changed = "displayName"
	case req.State.ParentFolder != req.Inputs.ParentFolder:
		changed = "parentFolder"
	default:
		return infer.DiffResponse{}, nil
	}

	NewLogContext(ctx).
		WithField("siteId", req.State.SiteID).
		WithField("folderId", req.State.FolderID).
		WithField("folderName", req.State.DisplayName).
		WithField("changedProperty", changed).
		Warn("Replacing asset folder. " + assetFolderLeftBehindWarning)

	return infer.DiffResponse{
		HasChanges:   true,
		DetailedDiff: map[string]p.PropertyDiff{changed: {Kind: p.UpdateReplace}},
	}, nil
}

// Create creates a new asset folder in the Webflow site.
func (r *AssetFolder) Create(
	ctx context.Context, req infer.CreateRequest[AssetFolderArgs],
) (infer.CreateResponse[AssetFolderState], error) {
	state := AssetFolderState{AssetFolderArgs: req.Inputs}

	// During preview, return the inputs without calling the API. Inputs may be unknown at this
	// point, so validation happens at apply time. The Webflow-assigned folder ID is unknown.
	if req.DryRun {
		return infer.CreateResponse[AssetFolderState]{Output: state}, nil
	}

	if err := ValidateSiteID(req.Inputs.SiteID); err != nil {
		return infer.CreateResponse[AssetFolderState]{}, fmt.Errorf("validation failed for AssetFolder resource: %w", err)
	}
	if err := ValidateDisplayName(req.Inputs.DisplayName); err != nil {
		return infer.CreateResponse[AssetFolderState]{}, fmt.Errorf("validation failed for AssetFolder resource: %w", err)
	}
	if err := validateOptionalAssetFolderID(req.Inputs.ParentFolder); err != nil {
		return infer.CreateResponse[AssetFolderState]{},
			fmt.Errorf("validation failed for AssetFolder resource (%w)", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[AssetFolderState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	folder, err := PostAssetFolder(ctx, client, req.Inputs.SiteID, req.Inputs.DisplayName, req.Inputs.ParentFolder)
	if err != nil {
		return infer.CreateResponse[AssetFolderState]{}, fmt.Errorf("failed to create asset folder: %w", err)
	}
	if folder.ID == "" {
		return infer.CreateResponse[AssetFolderState]{}, errors.New(
			"webflow API returned empty folder ID - this is unexpected and may indicate an API issue")
	}

	state.FolderID = folder.ID
	state.Assets = folder.Assets
	state.CreatedOn = folder.CreatedOn
	state.LastUpdated = folder.LastUpdated

	return infer.CreateResponse[AssetFolderState]{
		ID:     GenerateAssetFolderResourceID(req.Inputs.SiteID, folder.ID),
		Output: state,
	}, nil
}

// Read retrieves the current state of an asset folder from Webflow.
// Used for drift detection and import operations.
func (r *AssetFolder) Read(
	ctx context.Context, req infer.ReadRequest[AssetFolderArgs, AssetFolderState],
) (infer.ReadResponse[AssetFolderArgs, AssetFolderState], error) {
	siteID, folderID, err := ExtractIDsFromAssetFolderResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[AssetFolderArgs, AssetFolderState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return infer.ReadResponse[AssetFolderArgs, AssetFolderState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateAssetFolderID(folderID); err != nil {
		return infer.ReadResponse[AssetFolderArgs, AssetFolderState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[AssetFolderArgs, AssetFolderState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	folder, err := GetAssetFolder(ctx, client, folderID)
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[AssetFolderArgs, AssetFolderState]{ID: ""}, nil
		}
		return infer.ReadResponse[AssetFolderArgs, AssetFolderState]{}, fmt.Errorf("failed to read asset folder: %w", err)
	}

	currentInputs := AssetFolderArgs{
		SiteID:       siteID,
		DisplayName:  folder.DisplayName,
		ParentFolder: folder.ParentFolder,
	}
	currentState := AssetFolderState{
		AssetFolderArgs: currentInputs,
		FolderID:        folder.ID,
		Assets:          folder.Assets,
		CreatedOn:       folder.CreatedOn,
		LastUpdated:     folder.LastUpdated,
	}

	return infer.ReadResponse[AssetFolderArgs, AssetFolderState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update is not supported for asset folders - the Webflow API doesn't have an update endpoint.
// Any changes will trigger a replacement (create new folder, then drop the old one from state).
func (r *AssetFolder) Update(
	ctx context.Context, req infer.UpdateRequest[AssetFolderArgs, AssetFolderState],
) (infer.UpdateResponse[AssetFolderState], error) {
	return infer.UpdateResponse[AssetFolderState]{}, errors.New(
		"asset folders cannot be updated in place - the Webflow API does not support folder updates. " +
			"Any changes will trigger a replacement (create new folder, then remove old from state). " +
			"Note: The old folder will remain in Webflow as the API does not support deletion")
}

// Delete removes the asset folder from Pulumi state.
// NOTE: The Webflow API does not support deleting asset folders, so the folder
// will remain in Webflow even after this resource is destroyed.
func (r *AssetFolder) Delete(
	ctx context.Context, req infer.DeleteRequest[AssetFolderState],
) (infer.DeleteResponse, error) {
	NewLogContext(ctx).
		WithField("siteId", req.State.SiteID).
		WithField("folderId", req.State.FolderID).
		WithField("folderName", req.State.DisplayName).
		Warn(assetFolderLeftBehindWarning)

	return infer.DeleteResponse{}, nil
}
