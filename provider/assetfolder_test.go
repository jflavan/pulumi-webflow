// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	testFolderSiteID = "5f0c8c9e1c9d440000e8d8c3"
	testFolderID     = "6390c49774a71f0e3c1a08ee"
	testFolderParent = "5f0c8c9e1c9d440000e8d8c9"
	testFolderToken  = "test-token-12345678901234567890"
)

// Note: ValidateDisplayName tests are in site_test.go since it's a shared function.

func TestValidateAssetFolderID(t *testing.T) {
	for _, id := range []string{"5f0c8c9e1c9d440000e8d8c3", "abcdef1234567890abcdef12", "123456789012345678901234"} {
		if err := ValidateAssetFolderID(id); err != nil {
			t.Errorf("ValidateAssetFolderID(%q) = %v", id, err)
		}
	}
	if err := ValidateAssetFolderID(""); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("empty: %v", err)
	}
	for _, id := range []string{"5f0c8c9e1c9d44", "5f0c8c9e1c9d440000e8d8c3extra", "5F0C8C9E1C9D440000E8D8C3", "5f0c8c9e1c9d440000e8d8g3"} {
		if err := ValidateAssetFolderID(id); err == nil || !strings.Contains(err.Error(), "invalid format") {
			t.Errorf("ValidateAssetFolderID(%q) = %v, want invalid format", id, err)
		}
	}
}

func TestAssetFolderResourceIDRoundTrip(t *testing.T) {
	id := GenerateAssetFolderResourceID(testFolderSiteID, testFolderID)
	if id != testFolderSiteID+"/asset-folders/"+testFolderID {
		t.Fatalf("unexpected id %q", id)
	}
	siteID, folderID, err := ExtractIDsFromAssetFolderResourceID(id)
	if err != nil || siteID != testFolderSiteID || folderID != testFolderID {
		t.Fatalf("round trip: %q %q %v", siteID, folderID, err)
	}
	for _, bad := range []string{"", testFolderSiteID + "/" + testFolderID, testFolderSiteID + "/folders/" + testFolderID, testFolderSiteID} {
		if _, _, err := ExtractIDsFromAssetFolderResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestListAssetFolders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testFolderSiteID+"/asset_folders" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(AssetFolderListResponse{AssetFolders: []AssetFolderResponse{
			{ID: testFolderID, DisplayName: "Images", Assets: []string{"a1", "a2"}, SiteID: testFolderSiteID},
		}})
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	resp, err := ListAssetFolders(context.Background(), client, testFolderSiteID)
	if err != nil {
		t.Fatalf("ListAssetFolders: %v", err)
	}
	if len(resp.AssetFolders) != 1 || resp.AssetFolders[0].DisplayName != "Images" {
		t.Errorf("unexpected response %+v", resp)
	}
}

func TestGetAssetFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/asset_folders/"+testFolderID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(AssetFolderResponse{ID: testFolderID, DisplayName: "Documents", ParentFolder: testFolderParent})
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	folder, err := GetAssetFolder(context.Background(), client, testFolderID)
	if err != nil {
		t.Fatalf("GetAssetFolder: %v", err)
	}
	if folder.DisplayName != "Documents" || folder.ParentFolder != testFolderParent {
		t.Errorf("unexpected folder %+v", folder)
	}
}

func TestGetAssetFolder_NotFoundIsTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("folder not found"))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	_, err := GetAssetFolder(context.Background(), client, testFolderID)
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
}

func TestPostAssetFolder(t *testing.T) {
	var got AssetFolderCreateRequest
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testFolderSiteID+"/asset_folders" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		got = AssetFolderCreateRequest{}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		if status >= 400 {
			_, _ = w.Write([]byte("invalid folder configuration"))
			return
		}
		_ = json.NewEncoder(w).Encode(AssetFolderResponse{ID: testFolderID, DisplayName: got.DisplayName, ParentFolder: got.ParentFolder})
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	folder, err := PostAssetFolder(context.Background(), client, testFolderSiteID, "New Folder", testFolderParent)
	if err != nil {
		t.Fatalf("PostAssetFolder: %v", err)
	}
	if got.DisplayName != "New Folder" || got.ParentFolder != testFolderParent || folder.ID != testFolderID {
		t.Errorf("request %+v response %+v", got, folder)
	}

	if _, err := PostAssetFolder(context.Background(), client, testFolderSiteID, "Root", ""); err != nil {
		t.Fatalf("PostAssetFolder root: %v", err)
	}
	if got.ParentFolder != "" {
		t.Errorf("expected parentFolder omitted, got %q", got.ParentFolder)
	}

	status = http.StatusBadRequest
	if _, err := PostAssetFolder(context.Background(), client, testFolderSiteID, "", ""); err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected bad request error, got %v", err)
	}
}

func TestAssetFolderCreate(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testFolderToken)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testFolderSiteID+"/asset_folders" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(AssetFolderResponse{
			ID: testFolderID, DisplayName: "Images", ParentFolder: testFolderParent, Assets: []string{},
			SiteID: testFolderSiteID, CreatedOn: "2024-01-01T00:00:00Z", LastUpdated: "2024-01-01T00:00:00Z",
		})
	}))
	defer server.Close()
	useMockAPI(t, server)

	resp, err := (&AssetFolder{}).Create(context.Background(), infer.CreateRequest[AssetFolderArgs]{
		Inputs: AssetFolderArgs{SiteID: testFolderSiteID, DisplayName: "Images", ParentFolder: testFolderParent},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.ID != GenerateAssetFolderResourceID(testFolderSiteID, testFolderID) || resp.Output.FolderID != testFolderID ||
		resp.Output.CreatedOn == "" || calls != 1 {
		t.Errorf("unexpected response %+v (calls=%d)", resp, calls)
	}
}

func TestAssetFolderCreate_DryRunThenValidation(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testFolderToken)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("no API call expected, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	useMockAPI(t, server)

	// Invalid (unknown) inputs are fine during preview and produce no fabricated values.
	resp, err := (&AssetFolder{}).Create(context.Background(), infer.CreateRequest[AssetFolderArgs]{
		Inputs: AssetFolderArgs{SiteID: "", DisplayName: ""}, DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if resp.ID != "" || resp.Output.FolderID != "" || resp.Output.CreatedOn != "" {
		t.Errorf("dry run must not fabricate values: id=%q %+v", resp.ID, resp.Output)
	}

	// The same inputs fail validation at apply time before any API call.
	tests := []struct {
		name string
		args AssetFolderArgs
		want string
	}{
		{"invalid siteId", AssetFolderArgs{SiteID: "bad", DisplayName: "Images"}, "siteId has invalid format"},
		{"empty displayName", AssetFolderArgs{SiteID: testFolderSiteID, DisplayName: ""}, "displayName"},
		{"invalid parentFolder", AssetFolderArgs{SiteID: testFolderSiteID, DisplayName: "Images", ParentFolder: "bad"}, "parentFolder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&AssetFolder{}).Create(context.Background(), infer.CreateRequest[AssetFolderArgs]{Inputs: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestAssetFolderRead(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testFolderToken)
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/asset_folders/"+testFolderID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(`{"message":"not found"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(AssetFolderResponse{
			ID: testFolderID, DisplayName: "Renamed", ParentFolder: testFolderParent, Assets: []string{"a1"},
			SiteID: testFolderSiteID, CreatedOn: "2024-01-01T00:00:00Z", LastUpdated: "2024-02-01T00:00:00Z",
		})
	}))
	defer server.Close()
	useMockAPI(t, server)

	id := GenerateAssetFolderResourceID(testFolderSiteID, testFolderID)
	resp, err := (&AssetFolder{}).Read(context.Background(), infer.ReadRequest[AssetFolderArgs, AssetFolderState]{ID: id})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.ID != id || resp.Inputs.DisplayName != "Renamed" || resp.Inputs.ParentFolder != testFolderParent ||
		resp.State.FolderID != testFolderID || len(resp.State.Assets) != 1 || resp.State.LastUpdated != "2024-02-01T00:00:00Z" {
		t.Errorf("unexpected response %+v", resp)
	}

	status = http.StatusNotFound
	resp, err = (&AssetFolder{}).Read(context.Background(), infer.ReadRequest[AssetFolderArgs, AssetFolderState]{ID: id})
	if err != nil || resp.ID != "" {
		t.Errorf("404 should clear the resource: id=%q err=%v", resp.ID, err)
	}

	status = http.StatusInternalServerError
	if _, err := (&AssetFolder{}).Read(context.Background(), infer.ReadRequest[AssetFolderArgs, AssetFolderState]{ID: id}); err == nil {
		t.Error("500 must propagate as an error")
	}

	for _, bad := range []string{"", "x/asset-folders/y", testFolderSiteID + "/asset-folders/nope"} {
		if _, err := (&AssetFolder{}).Read(context.Background(), infer.ReadRequest[AssetFolderArgs, AssetFolderState]{ID: bad}); err == nil {
			t.Errorf("expected invalid ID error for %q", bad)
		}
	}
}

func TestAssetFolderDiff(t *testing.T) {
	base := AssetFolderArgs{SiteID: testFolderSiteID, DisplayName: "Images", ParentFolder: testFolderParent}
	state := AssetFolderState{AssetFolderArgs: base, FolderID: testFolderID}

	resp, err := (&AssetFolder{}).Diff(context.Background(), infer.DiffRequest[AssetFolderArgs, AssetFolderState]{Inputs: base, State: state})
	if err != nil || resp.HasChanges {
		t.Fatalf("expected no changes: %+v %v", resp, err)
	}

	tests := []struct {
		field  string
		modify func(a *AssetFolderArgs)
	}{
		{"siteId", func(a *AssetFolderArgs) { a.SiteID = "6f1d9d0f2d0e551111f9e9d4" }},
		{"displayName", func(a *AssetFolderArgs) { a.DisplayName = "Photos" }},
		{"parentFolder", func(a *AssetFolderArgs) { a.ParentFolder = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			in := base
			tt.modify(&in)
			resp, err := (&AssetFolder{}).Diff(context.Background(), infer.DiffRequest[AssetFolderArgs, AssetFolderState]{Inputs: in, State: state})
			if err != nil {
				t.Fatal(err)
			}
			if !resp.HasChanges || resp.DetailedDiff[tt.field].Kind != p.UpdateReplace {
				t.Errorf("expected %s UpdateReplace, got %+v", tt.field, resp)
			}
			if resp.DeleteBeforeReplace {
				t.Error("asset folders cannot be deleted, so DeleteBeforeReplace must be false")
			}
		})
	}
}

func TestAssetFolderDeleteAndUpdate(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testFolderToken)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("delete must not call the API, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	useMockAPI(t, server)

	_, err := (&AssetFolder{}).Delete(context.Background(), infer.DeleteRequest[AssetFolderState]{
		ID:    GenerateAssetFolderResourceID(testFolderSiteID, testFolderID),
		State: AssetFolderState{AssetFolderArgs: AssetFolderArgs{SiteID: testFolderSiteID, DisplayName: "Images"}, FolderID: testFolderID},
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := (&AssetFolder{}).Update(context.Background(), infer.UpdateRequest[AssetFolderArgs, AssetFolderState]{}); err == nil {
		t.Fatal("Update must return an error")
	}
}
