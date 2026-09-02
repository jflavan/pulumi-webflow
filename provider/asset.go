// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // Webflow requires an MD5 digest as the asset fileHash; not used for security.
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// AssetVariant represents different size variants of an uploaded asset.
// These variants are created by Webflow to serve the site responsively.
type AssetVariant struct {
	HostedURL        string `json:"hostedUrl,omitempty"`
	OriginalFileName string `json:"originalFileName,omitempty"`
	DisplayName      string `json:"displayName,omitempty"`
	Format           string `json:"format,omitempty"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	Quality          int    `json:"quality,omitempty"`
	Error            string `json:"error,omitempty"`
}

// AssetResponse represents a Webflow asset from the API.
type AssetResponse struct {
	ID               string `json:"id"`
	ContentType      string `json:"contentType"`
	Size             int    `json:"size"`
	SiteID           string `json:"siteId"`
	HostedURL        string `json:"hostedUrl"`
	OriginalFileName string `json:"originalFileName"`
	DisplayName      string `json:"displayName,omitempty"`
	AltText          string `json:"altText,omitempty"`
	// FolderID is the folder the asset belongs to, or empty at the site root.
	// Returned by List Assets (May 2026 API change); may be absent from Get Asset.
	FolderID    string         `json:"folderId,omitempty"`
	CreatedOn   string         `json:"createdOn"`
	LastUpdated string         `json:"lastUpdated"`
	Variants    []AssetVariant `json:"variants,omitempty"`
}

// AssetListResponse represents the response from listing assets.
type AssetListResponse struct {
	Assets     []AssetResponse `json:"assets"`
	Pagination struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"pagination,omitempty"`
}

// AssetUploadResponse represents the response from requesting an asset upload URL.
// This response contains the asset ID and all metadata needed for S3 upload.
type AssetUploadResponse struct {
	// ID is the Webflow-assigned asset ID (available immediately after POST)
	ID string `json:"id"`
	// UploadURL is the presigned S3 URL for uploading the file
	UploadURL string `json:"uploadUrl"`
	// UploadDetails contains AWS S3 POST form fields (acl, bucket, key, signature, etc.)
	UploadDetails map[string]string `json:"uploadDetails"`
	// AssetURL is the direct S3 link to the asset
	AssetURL string `json:"assetUrl"`
	// HostedURL is the Webflow CDN URL for the asset
	HostedURL string `json:"hostedUrl"`
	// ContentType is the MIME type of the asset
	ContentType string `json:"contentType"`
	// OriginalFileName is the original filename
	OriginalFileName string `json:"originalFileName"`
	// ParentFolder is the parent folder ID (if specified)
	ParentFolder string `json:"parentFolder,omitempty"`
	// CreatedOn is the creation timestamp
	CreatedOn string `json:"createdOn"`
	// LastUpdated is the last modification timestamp
	LastUpdated string `json:"lastUpdated"`
}

// AssetUploadRequest represents the request body for initiating an asset upload.
type AssetUploadRequest struct {
	FileName     string `json:"fileName"`               // Required: file name with extension
	FileHash     string `json:"fileHash"`               // Required: MD5 hash of file content
	ParentFolder string `json:"parentFolder,omitempty"` // Optional: folder ID
}

// assetIDPattern is the regex pattern for validating Webflow asset IDs.
// Asset IDs are typically 24-character hexadecimal strings.
var assetIDPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

// md5HashPattern is the regex pattern for validating MD5 file hashes.
// MD5 hashes are 32-character hexadecimal strings.
var md5HashPattern = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

// ValidateAssetID validates that an assetID matches the Webflow asset ID format.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateAssetID(assetID string) error {
	if assetID == "" {
		return errors.New("assetId is required but was not provided. " +
			"Please provide a valid Webflow asset ID " +
			"(24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c3'). " +
			"You can find asset IDs in the Webflow dashboard under Assets")
	}
	if !assetIDPattern.MatchString(assetID) {
		return fmt.Errorf("assetId has invalid format: got '%s'. "+
			"Expected a 24-character lowercase hexadecimal string "+
			"(e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"Please check your asset ID in the Webflow dashboard "+
			"and ensure it contains only lowercase letters (a-f) and digits (0-9)", assetID)
	}
	return nil
}

// ValidateFileName validates that a fileName is non-empty and has a reasonable format.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateFileName(fileName string) error {
	if fileName == "" {
		return errors.New("fileName is required but was not provided. " +
			"Please provide a valid file name with extension " +
			"(e.g., 'logo.png', 'hero-image.jpg', 'document.pdf')")
	}

	// Check for reasonable length
	if len(fileName) > 255 {
		return fmt.Errorf("fileName is too long: '%s' exceeds maximum length of 255 characters. "+
			"Please use a shorter file name", fileName)
	}

	// Check for common invalid characters (most filesystems disallow these)
	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(fileName, char) {
			return fmt.Errorf("fileName contains invalid character '%s': got '%s'. "+
				"Please remove invalid characters from the file name. "+
				"Valid characters: letters, numbers, hyphens, underscores, dots, spaces", char, fileName)
		}
	}

	return nil
}

// ValidateFileHash validates that a fileHash is a valid MD5 hash.
// MD5 hashes are 32-character hexadecimal strings.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateFileHash(fileHash string) error {
	if fileHash == "" {
		return errors.New("fileHash is required but was not provided. " +
			"Please provide the MD5 hash of your file content " +
			"(32-character hexadecimal string, e.g., 'd41d8cd98f00b204e9800998ecf8427e'). " +
			"You can generate an MD5 hash using: md5sum <filename> (Linux) or md5 <filename> (macOS)")
	}
	if !md5HashPattern.MatchString(fileHash) {
		return fmt.Errorf("fileHash has invalid format: got '%s'. "+
			"Expected a 32-character hexadecimal string (MD5 hash). "+
			"Example: 'd41d8cd98f00b204e9800998ecf8427e'. "+
			"You can generate an MD5 hash using: md5sum <filename> (Linux) or md5 <filename> (macOS)", fileHash)
	}
	return nil
}

// GenerateAssetResourceID generates a Pulumi resource ID for an Asset resource.
// Format: {siteID}/assets/{assetID}
func GenerateAssetResourceID(siteID, assetID string) string {
	return fmt.Sprintf("%s/assets/%s", siteID, assetID)
}

// ExtractIDsFromAssetResourceID extracts the siteID and assetID from an Asset resource ID.
// Expected format: {siteID}/assets/{assetID}
func ExtractIDsFromAssetResourceID(resourceID string) (siteID, assetID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.Split(resourceID, "/")
	if len(parts) < 3 || parts[1] != "assets" {
		return "", "", fmt.Errorf("invalid resource ID format: expected {siteId}/assets/{assetId}, got: %s", resourceID)
	}

	siteID = parts[0]
	assetID = strings.Join(parts[2:], "/") // Handle assetID that might contain slashes

	return siteID, assetID, nil
}

// ComputeFileHash returns the lowercase hexadecimal MD5 digest of data, which is the
// fileHash value the Webflow Create Asset Metadata endpoint expects.
func ComputeFileHash(data []byte) string {
	sum := md5.Sum(data) //nolint:gosec // Webflow mandates MD5 for asset deduplication.
	return hex.EncodeToString(sum[:])
}

// assetSourceHTTPClient fetches remote fileSource URLs. It deliberately carries no Webflow
// credentials because the URL is user-controlled and may point anywhere.
var assetSourceHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// assetUploadHTTPClient performs the S3 multipart upload. Like assetSourceHTTPClient it is
// unauthenticated: the presigned upload URL must never receive the Webflow bearer token.
var assetUploadHTTPClient = &http.Client{Timeout: 10 * time.Minute}

// maxAssetSourceBytes caps how much data is read from a fileSource (local or remote).
const maxAssetSourceBytes = 512 << 20 // 512 MiB

// IsRemoteAssetSource reports whether fileSource is an http(s) URL rather than a local path.
func IsRemoteAssetSource(fileSource string) bool {
	lower := strings.ToLower(fileSource)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// ReadAssetSource returns the bytes of the file referenced by fileSource.
// fileSource is either an http(s) URL, which is fetched with a plain (unauthenticated) client,
// or a local file path, which is cleaned and read relative to the Pulumi program's working
// directory. Empty sources are rejected.
func ReadAssetSource(ctx context.Context, fileSource string) ([]byte, error) {
	fileSource = strings.TrimSpace(fileSource)
	if fileSource == "" {
		return nil, errors.New("fileSource is required but was not provided. " +
			"Provide a local file path (e.g., './assets/logo.png') or an http(s) URL " +
			"(e.g., 'https://example.com/logo.png') for the file to upload")
	}

	if IsRemoteAssetSource(fileSource) {
		return readRemoteAssetSource(ctx, fileSource)
	}

	path := filepath.Clean(fileSource)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("fileSource '%s' cannot be read: %w. "+
			"Paths are resolved relative to the Pulumi program's working directory", fileSource, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("fileSource '%s' is a directory; provide the path to a file", fileSource)
	}
	if info.Size() > maxAssetSourceBytes {
		return nil, fmt.Errorf("fileSource '%s' is %d bytes, which exceeds the %d byte limit",
			fileSource, info.Size(), maxAssetSourceBytes)
	}
	data, err := os.ReadFile(path) //nolint:gosec // The path is intentionally user-supplied.
	if err != nil {
		return nil, fmt.Errorf("fileSource '%s' cannot be read: %w", fileSource, err)
	}
	return data, nil
}

// readRemoteAssetSource downloads an http(s) fileSource.
func readRemoteAssetSource(ctx context.Context, rawURL string) ([]byte, error) {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, fmt.Errorf("fileSource '%s' is not a valid URL: %w", rawURL, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for fileSource '%s': %w", rawURL, err)
	}
	resp, err := assetSourceHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download fileSource '%s': %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("failed to download fileSource '%s': HTTP %d. "+
			"Check that the URL is correct and publicly readable", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetSourceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read fileSource '%s': %w", rawURL, err)
	}
	if len(data) > maxAssetSourceBytes {
		return nil, fmt.Errorf("fileSource '%s' exceeds the %d byte limit", rawURL, maxAssetSourceBytes)
	}
	return data, nil
}

// GetAsset retrieves a single asset by ID from Webflow.
// It calls GET /v2/assets/{asset_id} endpoint.
func GetAsset(ctx context.Context, client *http.Client, assetID string) (*AssetResponse, error) {
	var asset AssetResponse
	if _, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/assets/%s", assetID), nil, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

// ListAssets retrieves assets for a Webflow site.
// It calls GET /v2/sites/{site_id}/assets, adding ?folderId= when folderID is non-empty so only
// assets in that folder (and its descendants) are returned.
func ListAssets(ctx context.Context, client *http.Client, siteID, folderID string) (*AssetListResponse, error) {
	u := apiURL("/v2/sites/%s/assets", siteID)
	if folderID != "" {
		u += "?folderId=" + url.QueryEscape(folderID)
	}
	var response AssetListResponse
	if _, err := doRequest(ctx, client, http.MethodGet, u, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// PostAssetUploadURL registers asset metadata with Webflow and returns the presigned S3 upload
// URL and form fields. This is step 1 of the 2-step asset upload process
// (POST /v2/sites/{site_id}/assets). Webflow answers 200, 201 or 202.
func PostAssetUploadURL(
	ctx context.Context, client *http.Client,
	siteID, fileName, fileHash, parentFolder string,
) (*AssetUploadResponse, error) {
	body := AssetUploadRequest{FileName: fileName, FileHash: fileHash, ParentFolder: parentFolder}
	var uploadResp AssetUploadResponse
	_, err := doRequest(ctx, client, http.MethodPost, apiURL("/v2/sites/%s/assets", siteID), body, &uploadResp,
		http.StatusOK, http.StatusCreated, http.StatusAccepted)
	if err != nil {
		return nil, err
	}
	return &uploadResp, nil
}

// UploadAssetFile performs step 2 of the asset upload: a multipart/form-data POST to the
// presigned S3 uploadUrl. Every uploadDetails entry is written as a form field first (S3
// ignores anything after the file part), then the file itself is sent as the "file" part.
// S3 answers with the success_action_status from the policy (usually 201), 200 or 204.
func UploadAssetFile(
	ctx context.Context, uploadURL string, uploadDetails map[string]string, fileName string, data []byte,
) error {
	if uploadURL == "" {
		return errors.New("webflow did not return an uploadUrl for the asset; cannot upload the file")
	}
	if len(uploadDetails) == 0 {
		return errors.New("webflow did not return uploadDetails for the asset; cannot upload the file")
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	keys := make([]string, 0, len(uploadDetails))
	for k := range uploadDetails {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := writer.WriteField(k, uploadDetails[k]); err != nil {
			return fmt.Errorf("failed to build upload form field %q: %w", k, err)
		}
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("failed to build upload file part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("failed to write upload file part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalise upload form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := assetUploadHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload asset file to storage: %w", err)
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyLength))
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("asset file upload rejected by storage (HTTP %d): %s. "+
			"The presigned upload may have expired or the file may not match the registered fileHash; "+
			"re-run the operation to request a fresh upload URL",
			resp.StatusCode, TruncateForLogging(strings.TrimSpace(string(respBody)), maxErrorBodyLength))
	}
}

// DeleteAsset deletes an asset from Webflow (DELETE /v2/assets/{asset_id}).
// A 404 is treated as success so deletes are idempotent.
func DeleteAsset(ctx context.Context, client *http.Client, assetID string) error {
	return doDelete(ctx, client, apiURL("/v2/assets/%s", assetID), nil)
}
