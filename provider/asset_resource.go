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
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Asset is the resource controller for managing Webflow assets.
// It implements the infer.CustomResource interface for CRUD operations.
// Note: Assets are immutable - updates require replacement.
type Asset struct{}

// AssetArgs defines the input properties for the Asset resource.
type AssetArgs struct {
	// SiteID is the Webflow site ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	SiteID string `pulumi:"siteId"`
	// FileName is the name of the file to upload.
	// Must include the file extension (e.g., "logo.png", "hero.jpg").
	FileName string `pulumi:"fileName"`
	// FileSource is where the file bytes come from: a local path (resolved relative to the
	// Pulumi program's working directory) or an http(s) URL.
	FileSource string `pulumi:"fileSource"`
	// FileHash is the MD5 hash of the file content. It is computed from fileSource; when
	// provided explicitly it must match the actual content or Create fails.
	FileHash string `pulumi:"fileHash,optional"`
	// ParentFolder is the optional folder ID where the asset will be placed.
	ParentFolder string `pulumi:"parentFolder,optional"`
}

// AssetState defines the output properties for the Asset resource.
// It embeds AssetArgs to include input properties in the output.
type AssetState struct {
	AssetArgs
	// AssetID is the Webflow-assigned asset ID (read-only).
	AssetID string `pulumi:"assetId,optional"`
	// UploadURL is the presigned S3 URL the file was uploaded to (read-only, secret).
	UploadURL string `pulumi:"uploadUrl,optional" provider:"secret"`
	// UploadDetails contains the signed AWS S3 POST form fields (read-only, secret).
	UploadDetails map[string]string `pulumi:"uploadDetails,optional" provider:"secret"`
	// AssetURL is the direct S3 URL for the asset (read-only).
	AssetURL string `pulumi:"assetUrl,optional"`
	// HostedURL is the Webflow CDN URL where the asset is hosted (read-only).
	HostedURL string `pulumi:"hostedUrl,optional"`
	// ContentType is the MIME type of the asset (read-only).
	ContentType string `pulumi:"contentType,optional"`
	// Size is the size of the asset in bytes (read-only).
	Size int `pulumi:"size,optional"`
	// FolderID is the folder the asset lives in, or empty at the site root (read-only).
	FolderID string `pulumi:"folderId,optional"`
	// CreatedOn is the timestamp when the asset was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
	// LastUpdated is the timestamp when the asset was last modified (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
}

// Annotate adds descriptions and constraints to the Asset resource.
func (r *Asset) Annotate(a infer.Annotator) {
	a.SetToken("index", "Asset")
	a.Describe(r, "Uploads and manages an asset (image, file, document) in a Webflow site. "+
		"Create registers the asset metadata with Webflow and then uploads the file bytes from "+
		"fileSource to Webflow's storage. Assets are immutable: changing any input, or changing the "+
		"content of a local fileSource, replaces the asset.")
}

// Annotate adds descriptions to the AssetArgs fields.
func (args *AssetArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SiteID,
		"The Webflow site ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find your site ID in the Webflow dashboard under Site Settings.")

	a.Describe(&args.FileName,
		"The name of the file as it will appear in Webflow, including the extension. "+
			"Examples: 'logo.png', 'hero-image.jpg', 'document.pdf'. "+
			"Must not exceed 255 characters or contain <, >, :, \", |, ?, *.")

	a.Describe(&args.FileSource,
		"Where the file bytes come from: a local file path (resolved relative to the Pulumi program's "+
			"working directory, e.g., './assets/logo.png') or an http(s) URL "+
			"(e.g., 'https://example.com/logo.png'). The content is read at apply time, "+
			"MD5-hashed and uploaded to Webflow.")

	a.Describe(&args.FileHash,
		"MD5 hash of the file content. Computed automatically from fileSource; "+
			"if you set it explicitly it must match the actual content. "+
			"For local files, a content change (different hash) replaces the asset.")

	a.Describe(&args.ParentFolder,
		"Optional asset folder ID where the asset will be organized in the Webflow Assets panel. "+
			"If not specified, the asset is placed at the root level. "+
			"Example: '5f0c8c9e1c9d440000e8d8c4'.")
}

// Annotate adds descriptions to the AssetState fields.
func (state *AssetState) Annotate(a infer.Annotator) {
	a.Describe(&state.AssetID, "The Webflow-assigned asset ID (read-only).")
	a.Describe(&state.UploadURL,
		"The presigned S3 URL the file was uploaded to (read-only, secret). "+
			"The provider performs the upload; this is recorded for reference only.")
	a.Describe(&state.UploadDetails,
		"The signed AWS S3 POST form fields used for the upload (read-only, secret).")
	a.Describe(&state.AssetURL, "The direct S3 URL for the asset (read-only).")
	a.Describe(&state.HostedURL,
		"The Webflow CDN URL where the asset is hosted (read-only). "+
			"Example: 'https://cdn.prod.website-files.com/.../logo.png'.")
	a.Describe(&state.ContentType,
		"The MIME type of the asset (read-only), e.g., 'image/png', 'application/pdf'.")
	a.Describe(&state.Size, "The size of the uploaded file in bytes (read-only).")
	a.Describe(&state.FolderID,
		"The ID of the asset folder the asset belongs to, or empty when it is at the site root (read-only).")
	a.Describe(&state.CreatedOn,
		"The timestamp when the asset metadata was created (RFC3339 format, read-only).")
	a.Describe(&state.LastUpdated,
		"The timestamp when the asset was last modified (RFC3339 format, read-only).")
}

// Diff determines what changes need to be made to the asset resource.
// Assets are immutable - any change requires replacement (delete + recreate).
// A local fileSource whose content hash differs from the recorded fileHash is also a replacement.
func (r *Asset) Diff(
	ctx context.Context, req infer.DiffRequest[AssetArgs, AssetState],
) (infer.DiffResponse, error) {
	replace := func(field string) infer.DiffResponse {
		return infer.DiffResponse{
			DeleteBeforeReplace: true,
			HasChanges:          true,
			DetailedDiff:        map[string]p.PropertyDiff{field: {Kind: p.UpdateReplace}},
		}
	}

	switch {
	case req.State.SiteID != req.Inputs.SiteID:
		return replace("siteId"), nil
	case req.State.FileName != req.Inputs.FileName:
		return replace("fileName"), nil
	case req.State.ParentFolder != req.Inputs.ParentFolder:
		return replace("parentFolder"), nil
	case req.State.FileSource != req.Inputs.FileSource:
		return replace("fileSource"), nil
	}

	// Determine the hash the new inputs imply. An explicit fileHash wins; otherwise a local
	// file is hashed now so content changes are detected. Remote URLs are not fetched during
	// Diff (that would be a network call on every preview); set fileHash to track them.
	expectedHash := strings.ToLower(req.Inputs.FileHash)
	if expectedHash == "" && req.Inputs.FileSource != "" && !IsRemoteAssetSource(req.Inputs.FileSource) {
		data, err := ReadAssetSource(ctx, req.Inputs.FileSource)
		if err != nil {
			return infer.DiffResponse{}, fmt.Errorf("cannot determine whether asset content changed: %w", err)
		}
		expectedHash = ComputeFileHash(data)
	}
	if expectedHash != "" && req.State.FileHash != "" && !strings.EqualFold(expectedHash, req.State.FileHash) {
		return replace("fileHash"), nil
	}

	return infer.DiffResponse{}, nil
}

// Create registers the asset with Webflow and uploads the file content.
func (r *Asset) Create(
	ctx context.Context, req infer.CreateRequest[AssetArgs],
) (infer.CreateResponse[AssetState], error) {
	state := AssetState{AssetArgs: req.Inputs}

	// During preview, return the inputs without touching the file or the API. Inputs may be
	// unknown (zero values) at this point, so validation is deferred to apply time. The
	// resource ID depends on the Webflow-assigned asset ID and is therefore unknown too.
	if req.DryRun {
		return infer.CreateResponse[AssetState]{Output: state}, nil
	}

	log := NewLogContext(ctx).
		WithField("siteId", req.Inputs.SiteID).
		WithField("fileName", req.Inputs.FileName)

	if err := ValidateSiteID(req.Inputs.SiteID); err != nil {
		return infer.CreateResponse[AssetState]{}, fmt.Errorf("validation failed for Asset resource: %w", err)
	}
	if err := ValidateFileName(req.Inputs.FileName); err != nil {
		return infer.CreateResponse[AssetState]{}, fmt.Errorf("validation failed for Asset resource: %w", err)
	}
	if req.Inputs.ParentFolder != "" {
		if err := ValidateAssetFolderID(req.Inputs.ParentFolder); err != nil {
			return infer.CreateResponse[AssetState]{},
				fmt.Errorf("validation failed for Asset resource (parentFolder): %w", err)
		}
	}
	if req.Inputs.FileHash != "" {
		if err := ValidateFileHash(req.Inputs.FileHash); err != nil {
			return infer.CreateResponse[AssetState]{}, fmt.Errorf("validation failed for Asset resource: %w", err)
		}
	}

	data, err := ReadAssetSource(ctx, req.Inputs.FileSource)
	if err != nil {
		return infer.CreateResponse[AssetState]{}, fmt.Errorf("validation failed for Asset resource: %w", err)
	}
	hash := ComputeFileHash(data)
	if req.Inputs.FileHash != "" && !strings.EqualFold(req.Inputs.FileHash, hash) {
		return infer.CreateResponse[AssetState]{}, fmt.Errorf(
			"validation failed for Asset resource: fileHash '%s' does not match the MD5 of fileSource '%s' (%s). "+
				"Remove fileHash to have it computed, or update it to match the file content",
			req.Inputs.FileHash, req.Inputs.FileSource, hash)
	}
	state.FileHash = hash

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[AssetState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	log.Debug("Registering asset metadata with Webflow")
	uploadResp, err := PostAssetUploadURL(
		ctx,
		client,
		req.Inputs.SiteID,
		req.Inputs.FileName,
		hash,
		req.Inputs.ParentFolder,
	)
	if err != nil {
		return infer.CreateResponse[AssetState]{}, fmt.Errorf("failed to create asset: %w", err)
	}
	if uploadResp.ID == "" {
		return infer.CreateResponse[AssetState]{}, errors.New(
			"webflow API returned empty asset ID - this is unexpected and may indicate an API issue")
	}

	log = log.WithField("assetId", uploadResp.ID)
	log.Debug("Uploading asset file to storage")
	if err := UploadAssetFile(ctx, uploadResp.UploadURL, uploadResp.UploadDetails, req.Inputs.FileName, data); err != nil {
		// Best effort: do not leave an orphaned metadata record behind.
		if delErr := DeleteAsset(ctx, client, uploadResp.ID); delErr != nil {
			log.Warnf("Failed to clean up asset metadata after upload failure: %v", delErr)
		}
		return infer.CreateResponse[AssetState]{}, fmt.Errorf("failed to upload asset file: %w", err)
	}
	log.Info("Asset uploaded successfully")

	state.AssetID = uploadResp.ID
	state.UploadURL = uploadResp.UploadURL
	state.UploadDetails = uploadResp.UploadDetails
	state.AssetURL = uploadResp.AssetURL
	state.HostedURL = uploadResp.HostedURL
	state.ContentType = uploadResp.ContentType
	state.Size = len(data)
	state.FolderID = uploadResp.ParentFolder
	state.CreatedOn = uploadResp.CreatedOn
	state.LastUpdated = uploadResp.LastUpdated

	return infer.CreateResponse[AssetState]{
		ID:     GenerateAssetResourceID(req.Inputs.SiteID, uploadResp.ID),
		Output: state,
	}, nil
}

// Read retrieves the current state of an asset from Webflow (GET /v2/assets/{asset_id}).
// Used for drift detection and import operations.
func (r *Asset) Read(
	ctx context.Context, req infer.ReadRequest[AssetArgs, AssetState],
) (infer.ReadResponse[AssetArgs, AssetState], error) {
	siteID, assetID, err := ExtractIDsFromAssetResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[AssetArgs, AssetState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateSiteID(siteID); err != nil {
		return infer.ReadResponse[AssetArgs, AssetState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateAssetID(assetID); err != nil {
		return infer.ReadResponse[AssetArgs, AssetState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[AssetArgs, AssetState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	asset, err := GetAsset(ctx, client, assetID)
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[AssetArgs, AssetState]{ID: ""}, nil
		}
		return infer.ReadResponse[AssetArgs, AssetState]{}, fmt.Errorf("failed to read asset: %w", err)
	}

	// The GET endpoint does not return the hash, source, or upload details; carry them from state.
	currentInputs := AssetArgs{
		SiteID:       siteID,
		FileName:     asset.OriginalFileName,
		FileSource:   req.State.FileSource,
		FileHash:     req.State.FileHash,
		ParentFolder: req.State.ParentFolder,
	}
	folderID := asset.FolderID
	if folderID == "" {
		folderID = req.State.FolderID
	}
	currentState := AssetState{
		AssetArgs:     currentInputs,
		AssetID:       asset.ID,
		UploadURL:     req.State.UploadURL,
		UploadDetails: req.State.UploadDetails,
		AssetURL:      req.State.AssetURL,
		HostedURL:     asset.HostedURL,
		ContentType:   asset.ContentType,
		Size:          asset.Size,
		FolderID:      folderID,
		CreatedOn:     asset.CreatedOn,
		LastUpdated:   asset.LastUpdated,
	}

	return infer.ReadResponse[AssetArgs, AssetState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  currentState,
	}, nil
}

// Update is not supported for assets - they are immutable.
// Any changes will trigger a replacement (delete + create).
func (r *Asset) Update(
	ctx context.Context, req infer.UpdateRequest[AssetArgs, AssetState],
) (infer.UpdateResponse[AssetState], error) {
	return infer.UpdateResponse[AssetState]{}, errors.New(
		"assets are immutable and cannot be updated in place. " +
			"Any changes will trigger a replacement (delete and recreate)")
}

// Delete removes an asset from the Webflow site.
func (r *Asset) Delete(ctx context.Context, req infer.DeleteRequest[AssetState]) (infer.DeleteResponse, error) {
	_, assetID, err := ExtractIDsFromAssetResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateAssetID(assetID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	if err := DeleteAsset(ctx, client, assetID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete asset: %w", err)
	}
	return infer.DeleteResponse{}, nil
}
