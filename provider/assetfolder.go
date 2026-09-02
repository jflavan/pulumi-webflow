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
	"net/http"
	"strings"
)

// AssetFolderResponse represents a Webflow asset folder from the API.
type AssetFolderResponse struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"displayName"`
	ParentFolder string   `json:"parentFolder,omitempty"`
	Assets       []string `json:"assets,omitempty"`
	SiteID       string   `json:"siteId"`
	CreatedOn    string   `json:"createdOn"`
	LastUpdated  string   `json:"lastUpdated"`
}

// AssetFolderListResponse represents the response from listing asset folders.
type AssetFolderListResponse struct {
	AssetFolders []AssetFolderResponse `json:"assetFolders"`
	Pagination   struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"pagination,omitempty"`
}

// AssetFolderCreateRequest represents the request body for creating an asset folder.
type AssetFolderCreateRequest struct {
	DisplayName  string `json:"displayName"`
	ParentFolder string `json:"parentFolder,omitempty"`
}

// ValidateAssetFolderID validates that an assetFolderID matches the Webflow asset folder ID format.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateAssetFolderID(assetFolderID string) error {
	if assetFolderID == "" {
		return errors.New("assetFolderId is required but was not provided. " +
			"Please provide a valid Webflow asset folder ID " +
			"(24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c3'). " +
			"You can find asset folder IDs in the Webflow dashboard under Assets")
	}
	if !siteIDPattern.MatchString(assetFolderID) {
		return fmt.Errorf("assetFolderId has invalid format: got '%s'. "+
			"Expected a 24-character lowercase hexadecimal string "+
			"(e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"Please check your asset folder ID in the Webflow dashboard "+
			"and ensure it contains only lowercase letters (a-f) and digits (0-9)", assetFolderID)
	}
	return nil
}

// GenerateAssetFolderResourceID generates a Pulumi resource ID for an AssetFolder resource.
// Format: {siteID}/asset-folders/{assetFolderID}
func GenerateAssetFolderResourceID(siteID, assetFolderID string) string {
	return fmt.Sprintf("%s/asset-folders/%s", siteID, assetFolderID)
}

// ExtractIDsFromAssetFolderResourceID extracts the siteID and assetFolderID from an AssetFolder resource ID.
// Expected format: {siteID}/asset-folders/{assetFolderID}
func ExtractIDsFromAssetFolderResourceID(resourceID string) (siteID, assetFolderID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.Split(resourceID, "/")
	if len(parts) < 3 || parts[1] != "asset-folders" {
		return "", "", fmt.Errorf(
			"invalid resource ID format: expected {siteId}/asset-folders/{assetFolderId}, got: %s", resourceID)
	}

	siteID = parts[0]
	assetFolderID = strings.Join(parts[2:], "/") // Handle ID that might contain slashes

	return siteID, assetFolderID, nil
}

// ListAssetFolders retrieves all asset folders for a Webflow site.
// It calls GET /v2/sites/{site_id}/asset_folders endpoint.
func ListAssetFolders(ctx context.Context, client *http.Client, siteID string) (*AssetFolderListResponse, error) {
	var response AssetFolderListResponse
	_, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/sites/%s/asset_folders", siteID), nil, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAssetFolder retrieves a single asset folder by ID from Webflow.
// It calls GET /v2/asset_folders/{asset_folder_id} endpoint.
func GetAssetFolder(ctx context.Context, client *http.Client, assetFolderID string) (*AssetFolderResponse, error) {
	var folder AssetFolderResponse
	_, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/asset_folders/%s", assetFolderID), nil, &folder)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

// PostAssetFolder creates a new asset folder in a Webflow site.
// It calls POST /v2/sites/{site_id}/asset_folders endpoint.
func PostAssetFolder(
	ctx context.Context, client *http.Client,
	siteID, displayName, parentFolder string,
) (*AssetFolderResponse, error) {
	body := AssetFolderCreateRequest{DisplayName: displayName, ParentFolder: parentFolder}
	var folder AssetFolderResponse
	_, err := doRequest(ctx, client, http.MethodPost, apiURL("/v2/sites/%s/asset_folders", siteID), body, &folder)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}
