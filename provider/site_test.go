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
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Well-known 24-character hex IDs for folder and page inputs.
const (
	testParentFolderID = "6a1b2c3d4e5f60718293a4b5"
	testPublishPageID  = "5f0c8c9e1c9d440000e8d8c4"
)

// ============================================================================
// Validation
// ============================================================================

func TestValidateDisplayName_Valid(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
	}{
		{"simple name", "My Site"},
		{"name with multiple words", "My Marketing Site"},
		{"name with numbers", "Company Blog 2024"},
		{"name with special characters", "Joe's Restaurant & Bar"},
		{"max length name", strings.Repeat("a", 255)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateDisplayName(tt.displayName); err != nil {
				t.Errorf("ValidateDisplayName(%q) returned unexpected error: %v", tt.displayName, err)
			}
		})
	}
}

func TestValidateDisplayName_Empty(t *testing.T) {
	err := ValidateDisplayName("")
	if err == nil {
		t.Fatal("ValidateDisplayName(\"\") should return error for empty string")
	}
	if !strings.Contains(err.Error(), "displayName is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateDisplayName_TooLong(t *testing.T) {
	err := ValidateDisplayName(strings.Repeat("a", 256))
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Errorf("expected 'too long' error, got: %v", err)
	}
}

func TestValidateShortName(t *testing.T) {
	valid := []string{"", "my-site", "company-blog-2024", "blog", "123", "site123abc"}
	for _, s := range valid {
		if err := ValidateShortName(s); err != nil {
			t.Errorf("ValidateShortName(%q) unexpected error: %v", s, err)
		}
	}
	invalid := map[string]string{
		"MY-SITE":   "lowercase",
		"my site":   "invalid characters",
		"my_site":   "invalid characters",
		"-my-site":  "leading/trailing",
		"my-site-":  "leading/trailing",
		"my.site":   "invalid characters",
		"my-site#a": "invalid characters",
	}
	for s, want := range invalid {
		err := ValidateShortName(s)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateShortName(%q) = %v, want error containing %q", s, err, want)
		}
	}
}

func TestValidateWorkspaceID(t *testing.T) {
	if err := ValidateWorkspaceID(testWorkspaceID); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err := ValidateWorkspaceID("")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
	for _, bad := range []string{"new-workspace", "7A2B3C4D5E6F708192A3B4C5", "7a2b3c4d5e6f", "../evil"} {
		err := ValidateWorkspaceID(bad)
		if err == nil || !strings.Contains(err.Error(), "invalid format") {
			t.Errorf("ValidateWorkspaceID(%q) = %v, want invalid format error", bad, err)
		}
	}
}

func TestValidateParentFolderID(t *testing.T) {
	for _, ok := range []string{"", testParentFolderID} {
		if err := ValidateParentFolderID(ok); err != nil {
			t.Errorf("ValidateParentFolderID(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"folder123", "FOLDER", "6a1b2c3d4e5f60718293a4b5x"} {
		if err := ValidateParentFolderID(bad); err == nil || !strings.Contains(err.Error(), "parentFolderId") {
			t.Errorf("ValidateParentFolderID(%q) = %v, want parentFolderId error", bad, err)
		}
	}
}

func TestValidateTemplateName(t *testing.T) {
	for _, ok := range []string{"", "blank", "mast-framework", "Template_v2.1"} {
		if err := ValidateTemplateName(ok); err != nil {
			t.Errorf("ValidateTemplateName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"my template", "-leading", "tpl/../x", strings.Repeat("a", 256)} {
		if err := ValidateTemplateName(bad); err == nil || !strings.Contains(err.Error(), "templateName") {
			t.Errorf("ValidateTemplateName(%q) = %v, want templateName error", bad, err)
		}
	}
}

func TestValidatePublishPageID(t *testing.T) {
	for _, ok := range []string{"", testPublishPageID} {
		if err := ValidatePublishPageID(ok); err != nil {
			t.Errorf("ValidatePublishPageID(%q) = %v, want nil", ok, err)
		}
	}
	if err := ValidatePublishPageID("page1"); err == nil || !strings.Contains(err.Error(), "publishPageId") {
		t.Errorf("ValidatePublishPageID(page1) = %v, want publishPageId error", err)
	}
}

// ============================================================================
// Request encoding
// ============================================================================

func TestSiteUpdateRequest_MarshalJSON(t *testing.T) {
	folder := "folder789"
	cleared := ""
	tests := []struct {
		name string
		req  SiteUpdateRequest
		want string
	}{
		{"name only", SiteUpdateRequest{Name: "Site"}, `{"name":"Site"}`},
		{
			"set folder",
			SiteUpdateRequest{Name: "Site", ParentFolderID: &folder},
			`{"name":"Site","parentFolderId":"folder789"}`,
		},
		{
			"clear folder sends null",
			SiteUpdateRequest{Name: "Site", ParentFolderID: &cleared},
			`{"name":"Site","parentFolderId":null}`,
		},
		{"empty", SiteUpdateRequest{}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSiteCustomDomains_UnmarshalObjectsAndStrings(t *testing.T) {
	var site Site
	err := json.Unmarshal([]byte(`{"id":"x","customDomains":[{"id":"d1","url":"example.com"},"www.example.com"]}`), &site)
	if err != nil {
		t.Fatal(err)
	}
	if len(site.CustomDomains) != 2 || site.CustomDomains[0] != "example.com" ||
		site.CustomDomains[1] != "www.example.com" {
		t.Errorf("unexpected custom domains: %v", site.CustomDomains)
	}
}

func TestSiteCustomDomains_SkipsEmptyElements(t *testing.T) {
	var site Site
	raw := `{"id":"x","customDomains":[null, "", {}, {"id":"d1"}, {"id":"d2","url":"example.com"}, "www.example.com"]}`
	if err := json.Unmarshal([]byte(raw), &site); err != nil {
		t.Fatal(err)
	}
	want := []string{"d1", "example.com", "www.example.com"}
	if len(site.CustomDomains) != len(want) {
		t.Fatalf("null/empty elements must be skipped, got %v", site.CustomDomains)
	}
	for i := range want {
		if site.CustomDomains[i] != want[i] {
			t.Errorf("customDomains[%d] = %q, want %q", i, site.CustomDomains[i], want[i])
		}
	}
	if err := json.Unmarshal([]byte(`{"customDomains":[42]}`), &site); err == nil {
		t.Error("unexpected element types must be rejected")
	}
}

// ============================================================================
// PostSite
// ============================================================================

func TestPostSite_Success(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusCreated, Site{
			ID: testSiteID, WorkspaceID: testWorkspaceID, DisplayName: "My Test Site", ShortName: "my-test-site",
		})
	})
	client := useMockAPI(t, server)

	site, err := PostSite(context.Background(), client, testWorkspaceID, "My Test Site", testParentFolderID, "")
	if err != nil {
		t.Fatalf("PostSite failed: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v2/workspaces/"+testWorkspaceID+"/sites" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotBody["name"] != "My Test Site" || gotBody["parentFolderId"] != testParentFolderID {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if _, ok := gotBody["templateName"]; ok {
		t.Errorf("empty templateName must be omitted, got body %v", gotBody)
	}
	if site.ID != testSiteID || site.DisplayName != "My Test Site" || site.WorkspaceID != testWorkspaceID {
		t.Errorf("unexpected site: %+v", site)
	}
}

func TestPostSite_RateLimiting(t *testing.T) {
	attempts := 0
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			writeJSON(t, w, http.StatusTooManyRequests, `{"message":"Rate limit exceeded"}`)
			return
		}
		writeJSON(t, w, http.StatusCreated, Site{ID: testSiteID, DisplayName: "My Test Site"})
	})
	client := useMockAPI(t, server)

	site, err := PostSite(context.Background(), client, testWorkspaceID, "My Test Site", "", "")
	if err != nil {
		t.Fatalf("PostSite should succeed after retry, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if site.ID != testSiteID {
		t.Errorf("unexpected site ID %q", site.ID)
	}
}

func TestPostSite_AcceptsOnlyDocumentedStatuses(t *testing.T) {
	tests := []struct {
		status  int
		wantErr bool
	}{
		{http.StatusCreated, false},
		{http.StatusOK, false}, // tolerated for older API behaviour
		{http.StatusAccepted, true},
		{http.StatusConflict, true},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, tt.status, Site{ID: testSiteID, DisplayName: "Site"})
			})
			client := useMockAPI(t, server)
			_, err := PostSite(context.Background(), client, testWorkspaceID, "Site", "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("status %d: err = %v, wantErr %v", tt.status, err, tt.wantErr)
			}
		})
	}
}

func TestPublishSite_AcceptsOnlyDocumentedStatuses(t *testing.T) {
	for _, status := range []int{http.StatusAccepted, http.StatusOK, http.StatusCreated, http.StatusNoContent} {
		server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, status, `{"publishScope":"site"}`)
		})
		client := useMockAPI(t, server)
		_, err := PublishSite(context.Background(), client, testSiteID, SitePublishRequest{PublishToWebflowSubdomain: true})
		wantErr := status != http.StatusAccepted && status != http.StatusOK
		if (err != nil) != wantErr {
			t.Errorf("status %d: err = %v, wantErr %v", status, err, wantErr)
		}
	}
}

func TestPostSite_Forbidden(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusForbidden, "Forbidden: Enterprise workspace required")
	})
	client := useMockAPI(t, server)

	_, err := PostSite(context.Background(), client, testWorkspaceID, "My Site", "", "")
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("expected 'forbidden' error, got: %v", err)
	}
}

func TestPostSite_ContextCancellation(t *testing.T) {
	called := false
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})
	client := useMockAPI(t, server)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PostSite(ctx, client, testWorkspaceID, "My Site", "", "")
	if err == nil || !strings.Contains(err.Error(), "cancel") {
		t.Errorf("expected cancellation error, got: %v", err)
	}
	if called {
		t.Error("request must not be sent with a cancelled context")
	}
}

func TestPostSite_InvalidJSON(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusCreated, "invalid json {{{")
	})
	client := useMockAPI(t, server)

	_, err := PostSite(context.Background(), client, testWorkspaceID, "My Site", "", "")
	if err == nil || !strings.Contains(err.Error(), "failed to parse response") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestPostSite_NetworkError(t *testing.T) {
	unreachableAPI(t)
	client, err := CreateHTTPClient(testToken, "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = PostSite(context.Background(), client, testWorkspaceID, "My Site", "", "")
	if err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("expected connection error with guidance, got: %v", err)
	}
}

// ============================================================================
// PatchSite
// ============================================================================

func TestPatchSite_SendsNameAndFolder(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, Site{ID: testSiteID, DisplayName: "Updated Site Name", ParentFolderID: "folder-new"})
	})
	client := useMockAPI(t, server)

	folder := testParentFolderID
	site, err := PatchSite(context.Background(), client, testSiteID, "Updated Site Name", &folder)
	if err != nil {
		t.Fatalf("PatchSite failed: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/v2/sites/"+testSiteID {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotBody["name"] != "Updated Site Name" || gotBody["parentFolderId"] != testParentFolderID {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if site.DisplayName != "Updated Site Name" {
		t.Errorf("unexpected displayName %q", site.DisplayName)
	}
}

func TestPatchSite_ClearsParentFolderWithNull(t *testing.T) {
	var raw string
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		raw = string(readBody(t, r))
		writeJSON(t, w, http.StatusOK, Site{ID: testSiteID, DisplayName: "Site"})
	})
	client := useMockAPI(t, server)

	cleared := ""
	if _, err := PatchSite(context.Background(), client, testSiteID, "Site", &cleared); err != nil {
		t.Fatalf("PatchSite failed: %v", err)
	}
	if !strings.Contains(raw, `"parentFolderId":null`) {
		t.Errorf("expected explicit null parentFolderId, got body %s", raw)
	}
}

func TestPatchSite_OmitsParentFolderWhenUnset(t *testing.T) {
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, Site{ID: testSiteID, DisplayName: "Site"})
	})
	client := useMockAPI(t, server)

	if _, err := PatchSite(context.Background(), client, testSiteID, "Site", nil); err != nil {
		t.Fatalf("PatchSite failed: %v", err)
	}
	if _, ok := gotBody["parentFolderId"]; ok {
		t.Errorf("parentFolderId must be omitted when nil, got body %v", gotBody)
	}
}

func TestPatchSite_NotFound(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, "Not found")
	})
	client := useMockAPI(t, server)

	_, err := PatchSite(context.Background(), client, testSiteID, "Name", nil)
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound error, got: %v", err)
	}
}

func TestPatchSite_InvalidJSON(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, "invalid json {{{")
	})
	client := useMockAPI(t, server)

	_, err := PatchSite(context.Background(), client, testSiteID, "Name", nil)
	if err == nil || !strings.Contains(err.Error(), "failed to parse response") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

// ============================================================================
// PublishSite
// ============================================================================

func TestPublishSite_SendsDocumentedBody(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusAccepted, `{
			"customDomains":[{"id":"660c6449dd97ebc7346ac629","url":"example.com","lastPublished":"2022-12-07T16:51:37.571Z"}],
			"publishToWebflowSubdomain":true,
			"publishScope":"page"
		}`)
	})
	client := useMockAPI(t, server)

	resp, err := PublishSite(context.Background(), client, testSiteID, SitePublishRequest{
		CustomDomains:             []string{"660c6449dd97ebc7346ac629", "660c6449dd97ebc7346ac62f"},
		PublishToWebflowSubdomain: true,
		PageID:                    "page123",
	})
	if err != nil {
		t.Fatalf("PublishSite failed: %v", err)
	}
	if gotPath != "/v2/sites/"+testSiteID+"/publish" {
		t.Errorf("unexpected path %s", gotPath)
	}
	if _, ok := gotBody["domains"]; ok {
		t.Errorf("undocumented 'domains' field must not be sent: %v", gotBody)
	}
	domains, _ := gotBody["customDomains"].([]any)
	if len(domains) != 2 || domains[0] != "660c6449dd97ebc7346ac629" {
		t.Errorf("unexpected customDomains: %v", gotBody["customDomains"])
	}
	if gotBody["publishToWebflowSubdomain"] != true || gotBody["pageId"] != "page123" {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if resp.PublishScope != "page" || !resp.PublishToWebflowSubdomain || len(resp.CustomDomains) != 1 ||
		resp.CustomDomains[0].URL != "example.com" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestPublishSite_OmitsEmptyOptionalFields(t *testing.T) {
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, `{"publishScope":"site"}`)
	})
	client := useMockAPI(t, server)

	resp, err := PublishSite(context.Background(), client, testSiteID, SitePublishRequest{PublishToWebflowSubdomain: true})
	if err != nil {
		t.Fatalf("PublishSite failed: %v", err)
	}
	if _, ok := gotBody["customDomains"]; ok {
		t.Errorf("empty customDomains must be omitted: %v", gotBody)
	}
	if _, ok := gotBody["pageId"]; ok {
		t.Errorf("empty pageId must be omitted: %v", gotBody)
	}
	if resp.PublishScope != "site" {
		t.Errorf("expected publishScope site, got %q", resp.PublishScope)
	}
}

func TestPublishSite_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{"not found", http.StatusNotFound, "not found"},
		{"bad request", http.StatusBadRequest, "bad request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, tt.status, `{"message":"boom"}`)
			})
			client := useMockAPI(t, server)
			_, err := PublishSite(context.Background(), client, testSiteID, SitePublishRequest{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected %q error, got: %v", tt.want, err)
			}
		})
	}
}

// ============================================================================
// DeleteSite / GetSite
// ============================================================================

func TestDeleteSite(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr string
	}{
		{"204 success", http.StatusNoContent, ""},
		{"404 idempotent", http.StatusNotFound, ""},
		{"403 forbidden", http.StatusForbidden, "forbidden"},
		{"500 server error", http.StatusInternalServerError, "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				writeJSON(t, w, tt.status, `{"message":"x"}`)
			})
			client := useMockAPI(t, server)
			err := DeleteSite(context.Background(), client, testSiteID)
			if gotMethod != http.MethodDelete || gotPath != "/v2/sites/"+testSiteID {
				t.Errorf("unexpected request %s %s", gotMethod, gotPath)
			}
			if tt.wantErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("expected %q error, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestDeleteSite_RateLimiting(t *testing.T) {
	attempts := 0
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client := useMockAPI(t, server)
	if err := DeleteSite(context.Background(), client, testSiteID); err != nil {
		t.Fatalf("DeleteSite should retry on 429: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestGetSite_AllFields(t *testing.T) {
	var gotPath string
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, http.StatusOK, `{
			"id": "`+testSiteID+`",
			"workspaceId": "`+testWorkspaceID+`",
			"displayName": "My Test Site",
			"shortName": "my-test-site",
			"timeZone": "America/New_York",
			"parentFolderId": "folder789",
			"lastPublished": "2024-01-15T10:30:00Z",
			"lastUpdated": "2024-01-15T12:00:00Z",
			"previewUrl": "https://preview.webflow.com/site123",
			"customDomains": [{"id":"d1","url":"example.com"}, {"id":"d2","url":"www.example.com"}],
			"dataCollectionEnabled": true,
			"dataCollectionType": "optOut"
		}`)
	})
	client := useMockAPI(t, server)

	site, err := GetSite(context.Background(), client, testSiteID)
	if err != nil {
		t.Fatalf("GetSite failed: %v", err)
	}
	if gotPath != "/v2/sites/"+testSiteID {
		t.Errorf("unexpected path %s", gotPath)
	}
	if site.ID != testSiteID || site.WorkspaceID != testWorkspaceID || site.DisplayName != "My Test Site" ||
		site.ShortName != "my-test-site" || site.TimeZone != "America/New_York" || site.ParentFolderID != "folder789" ||
		site.LastPublished != "2024-01-15T10:30:00Z" || site.LastUpdated != "2024-01-15T12:00:00Z" ||
		site.PreviewURL != "https://preview.webflow.com/site123" || !site.DataCollectionEnabled ||
		site.DataCollectionType != "optOut" {
		t.Errorf("unexpected site: %+v", site)
	}
	if len(site.CustomDomains) != 2 || site.CustomDomains[0] != "example.com" {
		t.Errorf("unexpected custom domains: %v", site.CustomDomains)
	}
}

func TestGetSite_NotFoundReturnsNilNil(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, `{"message":"Site not found"}`)
	})
	client := useMockAPI(t, server)
	site, err := GetSite(context.Background(), client, testSiteID)
	if err != nil || site != nil {
		t.Errorf("expected nil, nil for 404; got %v, %v", site, err)
	}
}

func TestGetSite_ErrorsAreNotTreatedAsMissing(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, status, `{"message":"not found in body must not matter"}`)
		})
		client := useMockAPI(t, server)
		site, err := GetSite(context.Background(), client, testSiteID)
		if err == nil || site != nil || IsNotFound(err) {
			t.Errorf("status %d: expected non-404 error, got site=%v err=%v", status, site, err)
		}
	}
}

func TestGetSite_MalformedJSON(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, `{invalid json`)
	})
	client := useMockAPI(t, server)
	site, err := GetSite(context.Background(), client, testSiteID)
	if err == nil || site != nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got site=%v err=%v", site, err)
	}
}

func TestGetSite_RateLimiting(t *testing.T) {
	attempts := 0
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, http.StatusOK, Site{ID: testSiteID, DisplayName: "Test Site"})
	})
	client := useMockAPI(t, server)
	site, err := GetSite(context.Background(), client, testSiteID)
	if err != nil || site == nil || site.DisplayName != "Test Site" {
		t.Fatalf("expected success after retry, got site=%v err=%v", site, err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

// ============================================================================
// SiteResource.Create
// ============================================================================

func TestSiteCreate_ValidationErrors(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	tests := []struct {
		name      string
		args      SiteArgs
		errSubstr string
	}{
		{"empty workspaceId", SiteArgs{WorkspaceID: "", DisplayName: "Site"}, "workspaceId is required"},
		{"malformed workspaceId", SiteArgs{WorkspaceID: "not-hex", DisplayName: "Site"}, "workspaceId has invalid format"},
		{"empty displayName", SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: ""}, "displayName is required"},
		{
			"malformed parentFolderId",
			SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", ParentFolderID: "folder123"},
			"parentFolderId has invalid format",
		},
		{
			"malformed publishPageId",
			SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", PublishPageID: "page1"},
			"publishPageId",
		},
		{
			"malformed templateName",
			SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", TemplateName: "my template"},
			"templateName has invalid format",
		},
	}
	resource := &SiteResource{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resource.Create(context.Background(), infer.CreateRequest[SiteArgs]{Inputs: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error containing %q, got: %v", tt.errSubstr, err)
			}
		})
	}
	if called {
		t.Error("API must not be called when validation fails")
	}
}

func TestSiteCreate_DryRun_SkipsValidationAndAPI(t *testing.T) {
	apiCalled := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusInternalServerError)
	})

	resource := &SiteResource{}
	// Unknown inputs arrive zeroed during preview; an empty workspaceId must not fail the preview.
	resp, err := resource.Create(context.Background(), infer.CreateRequest[SiteArgs]{
		Inputs: SiteArgs{WorkspaceID: "", DisplayName: "Preview Site", Publish: true},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("DryRun should succeed: %v", err)
	}
	if apiCalled {
		t.Error("API must not be called in DryRun mode")
	}
	// An empty ID makes the framework present the ID and all outputs as unknown to dependents.
	if resp.ID != "" {
		t.Errorf("preview must not fabricate an ID, got %q", resp.ID)
	}
	if resp.Output.DisplayName != "Preview Site" || !resp.Output.Publish {
		t.Errorf("inputs not preserved in preview output: %+v", resp.Output)
	}
	if resp.Output.LastPublished != "" || resp.Output.LastUpdated != "" || resp.Output.ShortName != "" {
		t.Errorf("preview must not fabricate read-only outputs: %+v", resp.Output)
	}
}

func TestSiteCheck(t *testing.T) {
	resource := &SiteResource{}

	t.Run("known invalid values fail", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{
			NewInputs: property.NewMap(map[string]property.Value{
				"workspaceId":    property.New("not-a-workspace"),
				"displayName":    property.New(""),
				"parentFolderId": property.New("folder123"),
				"publishPageId":  property.New("page1"),
				"templateName":   property.New("my template"),
			}),
		})
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		got := map[string]string{}
		for _, f := range resp.Failures {
			got[f.Property] = f.Reason
		}
		for _, want := range []string{"workspaceId", "displayName", "parentFolderId", "publishPageId", "templateName"} {
			if got[want] == "" {
				t.Errorf("expected a failure for %s, got %+v", want, resp.Failures)
			}
		}
		if len(got) != 5 {
			t.Errorf("expected exactly 5 failures, got %+v", resp.Failures)
		}
	})

	t.Run("unknown values are skipped", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{
			NewInputs: property.NewMap(map[string]property.Value{
				"workspaceId":    property.New(property.Computed),
				"displayName":    property.New(property.Computed),
				"parentFolderId": property.New(property.Computed),
				"publishPageId":  property.New(property.Computed),
				"templateName":   property.New("blank"),
			}),
		})
		if err != nil || len(resp.Failures) != 0 {
			t.Errorf("unknown inputs must not fail Check: failures=%+v err=%v", resp.Failures, err)
		}
	})

	t.Run("valid values pass", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{
			NewInputs: property.NewMap(map[string]property.Value{
				"workspaceId":          property.New(testWorkspaceID),
				"displayName":          property.New("My Site"),
				"parentFolderId":       property.New(testParentFolderID),
				"publish":              property.New(true),
				"publishCustomDomains": property.New([]property.Value{property.New("660c6449dd97ebc7346ac629")}),
				"publishPageId":        property.New(testPublishPageID),
				"templateName":         property.New("mast-framework"),
			}),
		})
		if err != nil || len(resp.Failures) != 0 {
			t.Errorf("valid inputs must pass Check: failures=%+v err=%v", resp.Failures, err)
		}
		if resp.Inputs.WorkspaceID != testWorkspaceID || !resp.Inputs.Publish ||
			len(resp.Inputs.PublishCustomDomains) != 1 || resp.Inputs.TemplateName != "mast-framework" {
			t.Errorf("inputs not decoded: %+v", resp.Inputs)
		}
	})
}

func TestSiteCreate_PublishesWithDocumentedBody(t *testing.T) {
	var publishBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/workspaces/"+testWorkspaceID+"/sites":
			body := readJSONBody(t, r)
			writeJSON(t, w, http.StatusCreated, Site{
				ID: testSiteID, WorkspaceID: testWorkspaceID, DisplayName: body["name"].(string), ShortName: "my-custom-site",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sites/"+testSiteID+"/publish":
			publishBody = readJSONBody(t, r)
			writeJSON(t, w, http.StatusAccepted, `{"publishToWebflowSubdomain":false,"publishScope":"site"}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	_ = server

	resource := &SiteResource{}
	resp, err := resource.Create(context.Background(), infer.CreateRequest[SiteArgs]{
		Inputs: SiteArgs{
			WorkspaceID:          testWorkspaceID,
			DisplayName:          "My Custom Site",
			Publish:              true,
			PublishCustomDomains: []string{"660c6449dd97ebc7346ac629"},
		},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if resp.ID != testSiteID {
		t.Errorf("expected site ID %q, got %q", testSiteID, resp.ID)
	}
	// shortName is always whatever Webflow generated
	if resp.Output.ShortName != "my-custom-site" {
		t.Errorf("expected shortName from API, got %q", resp.Output.ShortName)
	}
	if publishBody == nil {
		t.Fatal("publish endpoint was not called")
	}
	domains, _ := publishBody["customDomains"].([]any)
	if len(domains) != 1 || domains[0] != "660c6449dd97ebc7346ac629" {
		t.Errorf("unexpected customDomains in publish body: %v", publishBody)
	}
	if publishBody["publishToWebflowSubdomain"] != false {
		t.Errorf("publishToWebflowSubdomain should be false when custom domains are given: %v", publishBody)
	}
	if _, ok := publishBody["domains"]; ok {
		t.Errorf("undocumented 'domains' field must not be sent: %v", publishBody)
	}
	if resp.Output.PublishScope != "site" {
		t.Errorf("expected publishScope 'site' in state, got %q", resp.Output.PublishScope)
	}
}

func TestSiteCreate_PublishDefaultsToSubdomain(t *testing.T) {
	var publishBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/publish") {
			publishBody = readJSONBody(t, r)
			writeJSON(t, w, http.StatusAccepted, `{"publishScope":"site"}`)
			return
		}
		writeJSON(t, w, http.StatusCreated, Site{ID: testSiteID, WorkspaceID: testWorkspaceID, DisplayName: "Site"})
	})

	resource := &SiteResource{}
	_, err := resource.Create(context.Background(), infer.CreateRequest[SiteArgs]{
		Inputs: SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", Publish: true},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if publishBody == nil || publishBody["publishToWebflowSubdomain"] != true {
		t.Errorf("publish with no targets must fall back to the webflow.io subdomain, got body %v", publishBody)
	}
}

func TestSiteCreate_PublishFailureIsReported(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/publish") {
			writeJSON(t, w, http.StatusBadRequest, `{"message":"no domains"}`)
			return
		}
		writeJSON(t, w, http.StatusCreated, Site{ID: testSiteID, WorkspaceID: testWorkspaceID, DisplayName: "Site"})
	})

	resource := &SiteResource{}
	_, err := resource.Create(context.Background(), infer.CreateRequest[SiteArgs]{
		Inputs: SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", Publish: true, PublishToWebflowSubdomain: true},
	})
	if err == nil || !strings.Contains(err.Error(), "site created successfully but publishing failed") {
		t.Errorf("expected publish failure error, got: %v", err)
	}
}

func TestSiteCreate_EmptySiteIDFromAPI(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusCreated, Site{ID: "", DisplayName: "Site"})
	})
	resource := &SiteResource{}
	_, err := resource.Create(context.Background(), infer.CreateRequest[SiteArgs]{
		Inputs: SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site"},
	})
	if err == nil || !strings.Contains(err.Error(), "empty site ID") {
		t.Errorf("expected empty site ID error, got: %v", err)
	}
}

// ============================================================================
// SiteResource.Read
// ============================================================================

func TestSiteRead_Success_PreservesPublishInputs(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testSiteID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		writeJSON(t, w, http.StatusOK, `{
			"id": "`+testSiteID+`", "workspaceId": "`+testWorkspaceID+`", "displayName": "Live Name",
			"shortName": "live-name", "timeZone": "UTC", "parentFolderId": "folder789",
			"lastPublished": "2024-01-15T10:30:00Z", "customDomains": [{"id":"d1","url":"example.com"}]
		}`)
	})

	resource := &SiteResource{}
	resp, err := resource.Read(context.Background(), infer.ReadRequest[SiteArgs, SiteState]{
		ID: testSiteID,
		State: SiteState{
			SiteArgs: SiteArgs{
				WorkspaceID: testWorkspaceID, DisplayName: "Old Name",
				Publish: true, PublishToWebflowSubdomain: true, PublishCustomDomains: []string{"d1"}, PublishPageID: "pg",
				TemplateName: "blank",
			},
			PublishScope: "site",
		},
	})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if resp.ID != testSiteID {
		t.Errorf("expected ID preserved, got %q", resp.ID)
	}
	in := resp.Inputs
	if in.WorkspaceID != testWorkspaceID || in.DisplayName != "Live Name" || in.ParentFolderID != "folder789" {
		t.Errorf("inputs not taken from API: %+v", in)
	}
	if !in.Publish || !in.PublishToWebflowSubdomain || len(in.PublishCustomDomains) != 1 || in.PublishPageID != "pg" ||
		in.TemplateName != "blank" {
		t.Errorf("publish-related inputs must be preserved from state: %+v", in)
	}
	if resp.State.ShortName != "live-name" || resp.State.TimeZone != "UTC" ||
		resp.State.LastPublished != "2024-01-15T10:30:00Z" ||
		len(resp.State.CustomDomains) != 1 || resp.State.CustomDomains[0] != "example.com" ||
		resp.State.PublishScope != "site" {
		t.Errorf("state not populated: %+v", resp.State)
	}
}

func TestSiteRead_NotFoundSignalsDeletion(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, `{"message":"Site not found"}`)
	})
	resource := &SiteResource{}
	resp, err := resource.Read(context.Background(), infer.ReadRequest[SiteArgs, SiteState]{ID: testSiteID})
	if err != nil {
		t.Fatalf("404 must not be an error: %v", err)
	}
	if resp.ID != "" {
		t.Errorf("expected empty ID for deleted site, got %q", resp.ID)
	}
}

func TestSiteRead_OtherErrorsAreErrors(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, status, `{"message":"not found"}`)
		})
		resource := &SiteResource{}
		_, err := resource.Read(context.Background(), infer.ReadRequest[SiteArgs, SiteState]{ID: testSiteID})
		if err == nil {
			t.Errorf("status %d must surface as an error, not deletion", status)
		}
	}
}

func TestSiteRead_InvalidIDRejectedBeforeAPI(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	resource := &SiteResource{}
	_, err := resource.Read(context.Background(), infer.ReadRequest[SiteArgs, SiteState]{ID: "not a site id"})
	if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
		t.Errorf("expected invalid resource ID error, got: %v", err)
	}
	if called {
		t.Error("API must not be called with an invalid ID")
	}
}

// ============================================================================
// SiteResource.Update
// ============================================================================

func TestSiteUpdate_ClearsParentFolderAndPublishes(t *testing.T) {
	var patchRaw string
	var publishBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/v2/sites/"+testSiteID:
			patchRaw = string(readBody(t, r))
			writeJSON(t, w, http.StatusOK, Site{
				ID: testSiteID, WorkspaceID: testWorkspaceID, DisplayName: "New Name", ShortName: "new-name", TimeZone: "UTC",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sites/"+testSiteID+"/publish":
			publishBody = readJSONBody(t, r)
			writeJSON(t, w, http.StatusAccepted, `{"publishScope":"page"}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	resource := &SiteResource{}
	resp, err := resource.Update(context.Background(), infer.UpdateRequest[SiteArgs, SiteState]{
		ID: testSiteID,
		Inputs: SiteArgs{
			WorkspaceID: testWorkspaceID, DisplayName: "New Name",
			Publish: true, PublishToWebflowSubdomain: true, PublishPageID: testPublishPageID,
		},
		State: SiteState{
			SiteArgs: SiteArgs{
				WorkspaceID: testWorkspaceID, DisplayName: "Old Name", ParentFolderID: testParentFolderID,
			},
			CustomDomains: []string{"example.com"},
		},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if !strings.Contains(patchRaw, `"name":"New Name"`) || !strings.Contains(patchRaw, `"parentFolderId":null`) {
		t.Errorf("PATCH body must rename and clear the folder with null, got %s", patchRaw)
	}
	if publishBody == nil || publishBody["publishToWebflowSubdomain"] != true ||
		publishBody["pageId"] != testPublishPageID {
		t.Errorf("unexpected publish body: %v", publishBody)
	}
	if resp.Output.DisplayName != "New Name" || resp.Output.ShortName != "new-name" || resp.Output.ParentFolderID != "" {
		t.Errorf("state not updated from API: %+v", resp.Output)
	}
	if len(resp.Output.CustomDomains) != 1 {
		t.Errorf("read-only customDomains should be preserved when the API omits them: %+v", resp.Output.CustomDomains)
	}
	if resp.Output.PublishScope != "page" {
		t.Errorf("expected publishScope page, got %q", resp.Output.PublishScope)
	}
}

func TestSiteUpdate_OmitsParentFolderWhenNeverSet(t *testing.T) {
	var patchBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		patchBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, Site{ID: testSiteID, DisplayName: "Name"})
	})
	resource := &SiteResource{}
	_, err := resource.Update(context.Background(), infer.UpdateRequest[SiteArgs, SiteState]{
		ID:     testSiteID,
		Inputs: SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Name"},
		State:  SiteState{SiteArgs: SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Old"}},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if _, ok := patchBody["parentFolderId"]; ok {
		t.Errorf("parentFolderId must be omitted when it was never set: %v", patchBody)
	}
}

func TestSiteUpdate_ValidationError(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	resource := &SiteResource{}
	tests := []struct {
		name string
		args SiteArgs
		want string
	}{
		{"empty displayName", SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: ""}, "displayName is required"},
		{
			"malformed parentFolderId",
			SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", ParentFolderID: "folder-new"},
			"parentFolderId has invalid format",
		},
		{
			"malformed publishPageId",
			SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", Publish: true, PublishPageID: "page1"},
			"publishPageId",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resource.Update(context.Background(), infer.UpdateRequest[SiteArgs, SiteState]{
				ID:     testSiteID,
				Inputs: tt.args,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
	if called {
		t.Error("API must not be called when validation fails")
	}
}

func TestSiteUpdate_DryRun_PreservesReadOnlyOutputs(t *testing.T) {
	apiCalled := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { apiCalled = true })

	resource := &SiteResource{}
	resp, err := resource.Update(context.Background(), infer.UpdateRequest[SiteArgs, SiteState]{
		ID: testSiteID,
		// Empty displayName simulates an unknown input during preview - must not fail validation
		Inputs: SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: ""},
		State: SiteState{
			SiteArgs:              SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Old Site"},
			ShortName:             "old-site",
			TimeZone:              "America/New_York",
			LastPublished:         "2024-01-15T10:00:00Z",
			LastUpdated:           "2024-01-20T15:30:00Z",
			PreviewURL:            "https://preview.webflow.com/site123",
			CustomDomains:         []string{"example.com", "www.example.com"},
			DataCollectionEnabled: true,
			DataCollectionType:    "optOut",
			PublishScope:          "site",
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Update DryRun failed: %v", err)
	}
	if apiCalled {
		t.Error("API must not be called in DryRun mode")
	}
	o := resp.Output
	if o.TimeZone != "America/New_York" || o.LastPublished != "2024-01-15T10:00:00Z" ||
		o.LastUpdated != "2024-01-20T15:30:00Z" ||
		o.PreviewURL != "https://preview.webflow.com/site123" || len(o.CustomDomains) != 2 || !o.DataCollectionEnabled ||
		o.DataCollectionType != "optOut" || o.ShortName != "old-site" ||
		o.PublishScope != "site" {
		t.Errorf("read-only outputs not preserved: %+v", o)
	}
}

// ============================================================================
// SiteResource.Delete
// ============================================================================

func TestSiteDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	resource := &SiteResource{}
	if _, err := resource.Delete(context.Background(), infer.DeleteRequest[SiteState]{ID: testSiteID}); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v2/sites/"+testSiteID {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
}

func TestSiteDelete_InvalidID(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	resource := &SiteResource{}
	_, err := resource.Delete(context.Background(), infer.DeleteRequest[SiteState]{ID: "../evil"})
	if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
		t.Errorf("expected invalid resource ID error, got: %v", err)
	}
	if called {
		t.Error("API must not be called with an invalid ID")
	}
}

// ============================================================================
// SiteResource.Diff
// ============================================================================

func siteDiff(t *testing.T, inputs, state SiteArgs) infer.DiffResponse {
	t.Helper()
	diff, err := (&SiteResource{}).Diff(context.Background(), infer.DiffRequest[SiteArgs, SiteState]{
		Inputs: inputs,
		State:  SiteState{SiteArgs: state, ShortName: "generated", TimeZone: "UTC"},
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	return diff
}

func TestSiteDiff_NoChanges(t *testing.T) {
	args := SiteArgs{
		WorkspaceID:          testWorkspaceID,
		DisplayName:          "My Site",
		Publish:              true,
		PublishCustomDomains: []string{"d1"},
	}
	diff := siteDiff(t, args, args)
	if diff.HasChanges || len(diff.DetailedDiff) != 0 {
		t.Errorf("expected no changes (read-only shortName/timeZone must not diff), got %+v", diff)
	}
}

func TestSiteDiff_AccumulatesMutableFields(t *testing.T) {
	diff := siteDiff(t,
		SiteArgs{
			WorkspaceID: testWorkspaceID, DisplayName: "New", ParentFolderID: "new", Publish: true,
			PublishToWebflowSubdomain: true, PublishCustomDomains: []string{"d1"}, PublishPageID: "pg",
		},
		SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Old", ParentFolderID: "old"},
	)
	if !diff.HasChanges {
		t.Fatal("expected changes")
	}
	for _, field := range []string{
		"displayName", "parentFolderId", "publish", "publishToWebflowSubdomain",
		"publishCustomDomains", "publishPageId",
	} {
		pd, ok := diff.DetailedDiff[field]
		if !ok {
			t.Errorf("expected %q in DetailedDiff", field)
		} else if pd.Kind != p.Update {
			t.Errorf("%s should be an in-place Update, got %v", field, pd.Kind)
		}
	}
	if len(diff.DetailedDiff) != 6 {
		t.Errorf("expected 6 changes, got %d: %v", len(diff.DetailedDiff), diff.DetailedDiff)
	}
}

func TestSiteDiff_ImmutableFieldsReplace(t *testing.T) {
	diff := siteDiff(t,
		SiteArgs{WorkspaceID: "new-workspace", DisplayName: "Site"},
		SiteArgs{WorkspaceID: "old-workspace", DisplayName: "Site"})
	if pd := diff.DetailedDiff["workspaceId"]; pd.Kind != p.UpdateReplace {
		t.Errorf("workspaceId change should replace, got %+v", diff)
	}
	diff = siteDiff(t,
		SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", TemplateName: "new"},
		SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", TemplateName: "old"})
	if pd := diff.DetailedDiff["templateName"]; pd.Kind != p.UpdateReplace {
		t.Errorf("templateName change should replace, got %+v", diff)
	}
}

func TestSiteDiff_TemplateNameOnlyComparedWhenBothKnown(t *testing.T) {
	// templateName cannot be read back from the API, so an imported site has none in state.
	// Adding (or removing) it in the program must not replace the site.
	cases := []struct {
		name          string
		inputs, state string
	}{
		{"program sets template, state empty (import)", "blank", ""},
		{"program removed template, state has one", "", "blank"},
		{"both empty", "", ""},
		{"both equal", "blank", "blank"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			diff := siteDiff(t,
				SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", TemplateName: tt.inputs},
				SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", TemplateName: tt.state})
			if diff.HasChanges || len(diff.DetailedDiff) != 0 {
				t.Errorf("templateName must not diff when either side is empty, got %+v", diff)
			}
		})
	}
}

func TestSiteImportThenDiff_DoesNotReplace(t *testing.T) {
	// pulumi import: Read runs with empty inputs and state; the API does not report templateName.
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, `{
			"id": "`+testSiteID+`", "workspaceId": "`+testWorkspaceID+`", "displayName": "Imported",
			"shortName": "imported", "timeZone": "UTC"
		}`)
	})
	resource := &SiteResource{}
	readResp, err := resource.Read(context.Background(), infer.ReadRequest[SiteArgs, SiteState]{ID: testSiteID})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readResp.Inputs.TemplateName != "" || readResp.Inputs.WorkspaceID != testWorkspaceID {
		t.Errorf("unexpected imported inputs: %+v", readResp.Inputs)
	}

	// The program that created the site elsewhere still names the template it was built from.
	diff, err := resource.Diff(context.Background(), infer.DiffRequest[SiteArgs, SiteState]{
		ID: testSiteID,
		Inputs: SiteArgs{
			WorkspaceID: testWorkspaceID, DisplayName: "Imported", TemplateName: "mast-framework",
		},
		State: readResp.State,
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff.HasChanges {
		t.Errorf("import followed by up must not replace the site, got %+v", diff.DetailedDiff)
	}
}

func TestSiteDiff_ParentFolderRemoved(t *testing.T) {
	diff := siteDiff(t,
		SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site"},
		SiteArgs{WorkspaceID: testWorkspaceID, DisplayName: "Site", ParentFolderID: "folder"})
	if !diff.HasChanges || len(diff.DetailedDiff) != 1 || diff.DetailedDiff["parentFolderId"].Kind != p.Update {
		t.Errorf("removing parentFolderId should be a single in-place update, got %+v", diff)
	}
}
