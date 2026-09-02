// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAssetID(t *testing.T) {
	tests := []struct {
		name    string
		assetID string
		wantErr bool
	}{
		{"valid asset ID", "5f0c8c9e1c9d440000e8d8c3", false},
		{"empty asset ID", "", true},
		{"too short", "5f0c8c9e1c9d44", true},
		{"uppercase", "5F0C8C9E1C9D440000E8D8C3", true},
		{"invalid characters", "5f0c8c9e1c9d440000e8d8g3", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateAssetID(tt.assetID); (err != nil) != tt.wantErr {
				t.Errorf("ValidateAssetID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileHash(t *testing.T) {
	tests := []struct {
		name     string
		fileHash string
		wantErr  bool
	}{
		{"lowercase", "d41d8cd98f00b204e9800998ecf8427e", false},
		{"uppercase", "D41D8CD98F00B204E9800998ECF8427E", false},
		{"empty", "", true},
		{"too short", "d41d8cd98f00b204", true},
		{"too long", "d41d8cd98f00b204e9800998ecf8427e00", true},
		{"invalid characters", "d41d8cd98f00b204e9800998ecf8427g", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateFileHash(tt.fileHash); (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileName(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		wantErr  bool
	}{
		{"with extension", "logo.png", false},
		{"multiple dots", "my.hero.image.jpg", false},
		{"spaces", "my logo image.png", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 256) + ".png", true},
		{"invalid <", "my<file.png", true},
		{"invalid :", "my:file.png", true},
		{"invalid *", "my*file.png", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateFileName(tt.fileName); (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAssetResourceIDRoundTrip(t *testing.T) {
	id := GenerateAssetResourceID("5f0c8c9e1c9d440000e8d8c3", "5f0c8c9e1c9d440000e8d8c4")
	if id != "5f0c8c9e1c9d440000e8d8c3/assets/5f0c8c9e1c9d440000e8d8c4" {
		t.Fatalf("unexpected id %q", id)
	}
	siteID, assetID, err := ExtractIDsFromAssetResourceID(id)
	if err != nil || siteID != "5f0c8c9e1c9d440000e8d8c3" || assetID != "5f0c8c9e1c9d440000e8d8c4" {
		t.Fatalf("round trip failed: %q %q %v", siteID, assetID, err)
	}
	for _, bad := range []string{"", "a/b", "5f0c8c9e1c9d440000e8d8c3/images/x"} {
		if _, _, err := ExtractIDsFromAssetResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestComputeFileHash(t *testing.T) {
	if got := ComputeFileHash(nil); got != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("empty hash = %s", got)
	}
	if got := ComputeFileHash([]byte("hello")); got != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("hello hash = %s", got)
	}
	if err := ValidateFileHash(ComputeFileHash([]byte("x"))); err != nil {
		t.Errorf("computed hash should validate: %v", err)
	}
}

func TestReadAssetSource_LocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := ReadAssetSource(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadAssetSource: %v", err)
	}
	if string(data) != "png-bytes" {
		t.Errorf("unexpected content %q", data)
	}

	// Uncleaned paths are cleaned before reading.
	messy := filepath.Join(dir, "sub", "..", "logo.png")
	if _, err := ReadAssetSource(context.Background(), messy); err != nil {
		t.Errorf("expected cleaned path to be readable: %v", err)
	}
}

func TestReadAssetSource_Errors(t *testing.T) {
	if _, err := ReadAssetSource(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "fileSource is required") {
		t.Errorf("expected empty-path error, got %v", err)
	}
	if _, err := ReadAssetSource(context.Background(), "   "); err == nil {
		t.Error("expected whitespace-only path to be rejected")
	}
	if _, err := ReadAssetSource(context.Background(), filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("expected missing file error")
	}
	if _, err := ReadAssetSource(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Errorf("expected directory error, got %v", err)
	}
}

func TestReadAssetSource_RemoteURL(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path == "/missing.png" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("remote-bytes"))
	}))
	defer server.Close()

	data, err := ReadAssetSource(context.Background(), server.URL+"/logo.png")
	if err != nil {
		t.Fatalf("ReadAssetSource: %v", err)
	}
	if string(data) != "remote-bytes" {
		t.Errorf("unexpected content %q", data)
	}
	if gotAuth != "" {
		t.Errorf("remote fileSource must be fetched without credentials, got Authorization=%q", gotAuth)
	}
	if _, err := ReadAssetSource(context.Background(), server.URL+"/missing.png"); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected HTTP 404 error, got %v", err)
	}
}

func TestGetAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/assets/5f0c8c9e1c9d440000e8d8c4" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(AssetResponse{
			ID: "5f0c8c9e1c9d440000e8d8c4", ContentType: "image/png", Size: 12345,
			HostedURL: "https://assets.website-files.com/example/logo.png", OriginalFileName: "logo.png",
			FolderID: "5f0c8c9e1c9d440000e8d8c9",
		})
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	asset, err := GetAsset(context.Background(), client, "5f0c8c9e1c9d440000e8d8c4")
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if asset.ID != "5f0c8c9e1c9d440000e8d8c4" || asset.Size != 12345 || asset.FolderID != "5f0c8c9e1c9d440000e8d8c9" {
		t.Errorf("unexpected asset %+v", asset)
	}
}

func TestGetAsset_NotFoundIsTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Asset not found"}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	_, err := GetAsset(context.Background(), client, "5f0c8c9e1c9d440000e8d8c4")
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
}

func TestGetAsset_RetriesRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(AssetResponse{ID: "5f0c8c9e1c9d440000e8d8c4"})
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	asset, err := GetAsset(context.Background(), client, "5f0c8c9e1c9d440000e8d8c4")
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if asset.ID != "5f0c8c9e1c9d440000e8d8c4" || attempts != 3 {
		t.Errorf("asset=%+v attempts=%d", asset, attempts)
	}
}

func TestListAssets(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/5f0c8c9e1c9d440000e8d8c3/assets" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"assets":[{"id":"5f0c8c9e1c9d440000e8d8c4","folderId":"5f0c8c9e1c9d440000e8d8c9"}],` +
			`"pagination":{"limit":100,"offset":0,"total":1}}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	resp, err := ListAssets(context.Background(), client, "5f0c8c9e1c9d440000e8d8c3", "")
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("expected no query without folderId, got %q", gotQuery)
	}
	if len(resp.Assets) != 1 || resp.Assets[0].FolderID != "5f0c8c9e1c9d440000e8d8c9" || resp.Pagination.Total != 1 {
		t.Errorf("unexpected response %+v", resp)
	}

	if _, err := ListAssets(context.Background(), client, "5f0c8c9e1c9d440000e8d8c3", "5f0c8c9e1c9d440000e8d8c9"); err != nil {
		t.Fatalf("ListAssets with folder: %v", err)
	}
	if gotQuery != "folderId=5f0c8c9e1c9d440000e8d8c9" {
		t.Errorf("expected folderId query, got %q", gotQuery)
	}
}

func TestPostAssetUploadURL(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/5f0c8c9e1c9d440000e8d8c3/assets" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
				}
				var req AssetUploadRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("decode body: %v", err)
				}
				if req.FileName != "logo.png" || req.FileHash != "d41d8cd98f00b204e9800998ecf8427e" || req.ParentFolder != "5f0c8c9e1c9d440000e8d8c9" {
					t.Errorf("unexpected body %+v", req)
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(AssetUploadResponse{
					ID: "5f0c8c9e1c9d440000e8d8c4", UploadURL: "https://s3.amazonaws.com/bucket",
					UploadDetails: map[string]string{"acl": "public-read", "key": "assets/logo.png"},
					HostedURL:     "https://cdn.prod.website-files.com/example/logo.png", ContentType: "image/png",
					ParentFolder: "5f0c8c9e1c9d440000e8d8c9",
				})
			}))
			defer server.Close()
			client := useMockAPI(t, server)

			resp, err := PostAssetUploadURL(context.Background(), client, "5f0c8c9e1c9d440000e8d8c3",
				"logo.png", "d41d8cd98f00b204e9800998ecf8427e", "5f0c8c9e1c9d440000e8d8c9")
			if err != nil {
				t.Fatalf("PostAssetUploadURL: %v", err)
			}
			if resp.ID != "5f0c8c9e1c9d440000e8d8c4" || resp.UploadDetails["acl"] != "public-read" || resp.ParentFolder != "5f0c8c9e1c9d440000e8d8c9" {
				t.Errorf("unexpected response %+v", resp)
			}
		})
	}
}

// readMultipartParts returns the parts of a multipart request in wire order.
func readMultipartParts(t *testing.T, r *http.Request) (fields map[string]string, order []string, fileName string, fileData []byte) {
	t.Helper()
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type %q: %v", r.Header.Get("Content-Type"), err)
	}
	reader := multipart.NewReader(r.Body, params["boundary"])
	fields = map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		data, _ := io.ReadAll(part)
		order = append(order, part.FormName())
		if part.FormName() == "file" {
			fileName = part.FileName()
			fileData = data
			continue
		}
		fields[part.FormName()] = string(data)
	}
	return fields, order, fileName, fileData
}

func TestUploadAssetFile(t *testing.T) {
	details := map[string]string{
		"acl":                   "public-read",
		"bucket":                "webflow-prod-assets",
		"key":                   "5f0c8c9e1c9d440000e8d8c3/5f0c8c9e1c9d440000e8d8c4_logo.png",
		"Content-Type":          "image/png",
		"X-Amz-Algorithm":       "AWS4-HMAC-SHA256",
		"X-Amz-Credential":      "AKIAEXAMPLE/20240101/us-east-1/s3/aws4_request",
		"X-Amz-Date":            "20240101T000000Z",
		"Policy":                "base64policy",
		"X-Amz-Signature":       "signature123",
		"success_action_status": "201",
	}
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/upload" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("upload must not carry the Webflow bearer token")
		}
		fields, order, fileName, fileData := readMultipartParts(t, r)
		for k, v := range details {
			if fields[k] != v {
				t.Errorf("form field %s = %q, want %q", k, fields[k], v)
			}
		}
		if len(order) == 0 || order[len(order)-1] != "file" {
			t.Errorf("file part must be last, got order %v", order)
		}
		if fileName != "logo.png" || string(fileData) != "png-bytes" {
			t.Errorf("file part = %q %q", fileName, fileData)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	if err := UploadAssetFile(context.Background(), server.URL+"/upload", details, "logo.png", []byte("png-bytes")); err != nil {
		t.Fatalf("UploadAssetFile: %v", err)
	}
	if !called {
		t.Fatal("upload endpoint was not called")
	}
}

func TestUploadAssetFile_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
	}))
	defer server.Close()

	err := UploadAssetFile(context.Background(), server.URL, map[string]string{"key": "k"}, "logo.png", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("expected 403 error with body, got %v", err)
	}
	if err := UploadAssetFile(context.Background(), "", map[string]string{"key": "k"}, "logo.png", nil); err == nil {
		t.Error("expected error for empty uploadUrl")
	}
	if err := UploadAssetFile(context.Background(), server.URL, nil, "logo.png", nil); err == nil {
		t.Error("expected error for missing uploadDetails")
	}
}

func TestDeleteAsset(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"204", http.StatusNoContent, false},
		{"404 idempotent", http.StatusNotFound, false},
		{"500", http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/v2/assets/5f0c8c9e1c9d440000e8d8c4" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()
			client := useMockAPI(t, server)

			err := DeleteAsset(context.Background(), client, "5f0c8c9e1c9d440000e8d8c4")
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteAsset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
