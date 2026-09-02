// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// ============================================================================
// Validation
// ============================================================================

func TestValidateSourcePath(t *testing.T) {
	valid := []string{"/old-page", "/blog/2023", "/old_page", "/", "/products/category/item-1",
		"/files/document.pdf", "//old-page", "/old-page/", "/.hidden", "/blog/2024/my-post_v2.html"}
	for _, path := range valid {
		if err := ValidateSourcePath(path); err != nil {
			t.Errorf("ValidateSourcePath(%q) = %v, want nil", path, err)
		}
	}
	invalid := map[string]string{
		"":             "required",
		"old-page":     "must start with '/'",
		"/old page":    "invalid characters",
		"/page?query":  "invalid characters",
		"/page#anchor": "invalid characters",
		"/page@name":   "invalid characters",
		"/path\\file":  "invalid characters",
	}
	for path, want := range invalid {
		err := ValidateSourcePath(path)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateSourcePath(%q) = %v, want error containing %q", path, err, want)
		}
	}
}

func TestValidateDestinationPath(t *testing.T) {
	for _, path := range []string{"/new-page", "/home", "/", "/files/document.pdf"} {
		if err := ValidateDestinationPath(path); err != nil {
			t.Errorf("ValidateDestinationPath(%q) = %v, want nil", path, err)
		}
	}
	invalid := map[string]string{
		"":          "required",
		"new-page":  "must start with '/'",
		"/new page": "invalid characters",
	}
	for path, want := range invalid {
		err := ValidateDestinationPath(path)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateDestinationPath(%q) = %v, want error containing %q", path, err, want)
		}
	}
}

func TestValidateStatusCode(t *testing.T) {
	for _, code := range []int{301, 302} {
		if err := ValidateStatusCode(code); err != nil {
			t.Errorf("ValidateStatusCode(%d) = %v, want nil", code, err)
		}
	}
	for _, code := range []int{0, 200, 307, 308, 400, 404, 500} {
		err := ValidateStatusCode(code)
		if err == nil || !strings.Contains(err.Error(), "301 or 302") {
			t.Errorf("ValidateStatusCode(%d) = %v, want '301 or 302' error", code, err)
		}
	}
}

func TestValidateRedirectID(t *testing.T) {
	for _, id := range []string{"42e1a2b7aa1a13f768a0042a", "redir_12345", "abc-DEF"} {
		if err := ValidateRedirectID(id); err != nil {
			t.Errorf("ValidateRedirectID(%q) = %v, want nil", id, err)
		}
	}
	for _, id := range []string{"", "a/b", "a b", "../x", "a?b=c"} {
		if err := ValidateRedirectID(id); err == nil {
			t.Errorf("ValidateRedirectID(%q) = nil, want error", id)
		}
	}
}

func TestRedirectResourceID_RoundTrip(t *testing.T) {
	resourceID := GenerateRedirectResourceID(testSiteID, "redir_12345")
	if resourceID != testSiteID+"/redirects/redir_12345" {
		t.Fatalf("unexpected resource ID %q", resourceID)
	}
	siteID, redirectID, err := ExtractIDsFromRedirectResourceID(resourceID)
	if err != nil || siteID != testSiteID || redirectID != "redir_12345" {
		t.Errorf("ExtractIDsFromRedirectResourceID(%q) = %q, %q, %v", resourceID, siteID, redirectID, err)
	}
	for _, bad := range []string{"", testSiteID + "/redir_12345", testSiteID + "/robots/redir_12345", testSiteID} {
		if _, _, err := ExtractIDsFromRedirectResourceID(bad); err == nil {
			t.Errorf("ExtractIDsFromRedirectResourceID(%q) error = nil, want error", bad)
		}
	}
}

// ============================================================================
// List / Find with pagination
// ============================================================================

// pagedRedirectServer serves `total` redirects named redirect-N in pages honouring limit/offset,
// recording every offset requested.
func pagedRedirectServer(t *testing.T, total int, offsets *[]int) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testSiteID+"/redirects" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			t.Errorf("limit query parameter missing: %s", r.URL.RawQuery)
			limit = 100
		}
		*offsets = append(*offsets, offset)
		page := RedirectResponse{Redirects: []RedirectRule{}, Pagination: RedirectPagination{Limit: limit, Offset: offset, Total: total}}
		for i := offset; i < total && i < offset+limit; i++ {
			page.Redirects = append(page.Redirects, RedirectRule{
				ID: fmt.Sprintf("redirect-%d", i), SourcePath: fmt.Sprintf("/old-%d", i), DestinationPath: fmt.Sprintf("/new-%d", i),
			})
		}
		writeJSON(t, w, http.StatusOK, page)
	}
}

func TestGetRedirects_FollowsPagination(t *testing.T) {
	var offsets []int
	server := mockWebflowAPI(t, pagedRedirectServer(t, 250, &offsets))
	client := useMockAPI(t, server)

	result, err := GetRedirects(context.Background(), client, testSiteID)
	if err != nil {
		t.Fatalf("GetRedirects failed: %v", err)
	}
	if len(result.Redirects) != 250 {
		t.Errorf("expected 250 redirects across pages, got %d", len(result.Redirects))
	}
	if len(offsets) != 3 || offsets[0] != 0 || offsets[1] != 100 || offsets[2] != 200 {
		t.Errorf("expected offsets [0 100 200], got %v", offsets)
	}
	if result.Redirects[249].ID != "redirect-249" || result.Pagination.Total != 250 {
		t.Errorf("unexpected aggregate: last=%s total=%d", result.Redirects[249].ID, result.Pagination.Total)
	}
}

func TestFindRedirect_StopsWhenFound(t *testing.T) {
	var offsets []int
	server := mockWebflowAPI(t, pagedRedirectServer(t, 350, &offsets))
	client := useMockAPI(t, server)

	found, err := FindRedirect(context.Background(), client, testSiteID, "redirect-150")
	if err != nil {
		t.Fatalf("FindRedirect failed: %v", err)
	}
	if found == nil || found.ID != "redirect-150" || found.SourcePath != "/old-150" {
		t.Errorf("unexpected result: %+v", found)
	}
	if len(offsets) != 2 {
		t.Errorf("expected to stop after the second page, requested offsets %v", offsets)
	}
}

func TestFindRedirect_ExhaustsPagesThenReturnsNil(t *testing.T) {
	var offsets []int
	server := mockWebflowAPI(t, pagedRedirectServer(t, 120, &offsets))
	client := useMockAPI(t, server)

	found, err := FindRedirect(context.Background(), client, testSiteID, "missing")
	if err != nil || found != nil {
		t.Fatalf("expected nil, nil for a missing redirect; got %v, %v", found, err)
	}
	if len(offsets) != 2 {
		t.Errorf("expected both pages to be read, requested offsets %v", offsets)
	}
}

func TestFindRedirect_StopsOnEmptyPageWhenAPIIgnoresOffset(t *testing.T) {
	calls := 0
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeJSON(t, w, http.StatusOK, RedirectResponse{
				Redirects:  []RedirectRule{{ID: "a", SourcePath: "/a", DestinationPath: "/b"}},
				Pagination: RedirectPagination{Limit: 1, Offset: 0, Total: 1000},
			})
			return
		}
		writeJSON(t, w, http.StatusOK, RedirectResponse{Redirects: []RedirectRule{}, Pagination: RedirectPagination{Total: 1000}})
	})
	client := useMockAPI(t, server)

	found, err := FindRedirect(context.Background(), client, testSiteID, "missing")
	if err != nil || found != nil {
		t.Fatalf("expected nil, nil; got %v, %v", found, err)
	}
	if calls != 2 {
		t.Errorf("expected the loop to stop on the first empty page, got %d calls", calls)
	}
}

func TestGetRedirects_NotFound(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, "site not found")
	})
	client := useMockAPI(t, server)
	_, err := GetRedirects(context.Background(), client, testSiteID)
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound error, got: %v", err)
	}
}

// ============================================================================
// Post / Patch / Delete
// ============================================================================

func TestPostRedirect_SendsBody(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusCreated, `{"id":"new-redirect-1","fromUrl":"/old","toUrl":"/new"}`)
	})
	client := useMockAPI(t, server)

	result, err := PostRedirect(context.Background(), client, testSiteID, "/old", "/new", 301)
	if err != nil {
		t.Fatalf("PostRedirect failed: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v2/sites/"+testSiteID+"/redirects" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotBody["fromUrl"] != "/old" || gotBody["toUrl"] != "/new" || gotBody["statusCode"] != float64(301) {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if result.ID != "new-redirect-1" || result.SourcePath != "/old" || result.StatusCode != 0 || result.CreatedOn != "" {
		t.Errorf("unexpected result (statusCode/createdOn must not be invented): %+v", result)
	}
}

func TestPostRedirect_BadRequest(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, "invalid redirect configuration")
	})
	client := useMockAPI(t, server)
	_, err := PostRedirect(context.Background(), client, testSiteID, "invalid", "/new", 301)
	if err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected 'bad request' error, got: %v", err)
	}
}

func TestPatchRedirect_SendsOnlyMutableFields(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, `{"id":"redirect1","fromUrl":"/old","toUrl":"/updated"}`)
	})
	client := useMockAPI(t, server)

	result, err := PatchRedirect(context.Background(), client, testSiteID, "redirect1", "/updated", 302)
	if err != nil {
		t.Fatalf("PatchRedirect failed: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/v2/sites/"+testSiteID+"/redirects/redirect1" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotBody["toUrl"] != "/updated" || gotBody["statusCode"] != float64(302) {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if _, ok := gotBody["fromUrl"]; ok {
		t.Errorf("fromUrl must not be sent on PATCH: %v", gotBody)
	}
	if result.DestinationPath != "/updated" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestPatchRedirect_NotFound(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, "redirect not found")
	})
	client := useMockAPI(t, server)
	_, err := PatchRedirect(context.Background(), client, testSiteID, "nonexistent", "/new", 301)
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound error, got: %v", err)
	}
}

func TestDeleteRedirect(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr string
	}{
		{"204 success", http.StatusNoContent, ""},
		{"200 success with body", http.StatusOK, ""},
		{"404 idempotent", http.StatusNotFound, ""},
		{"500 server error", http.StatusInternalServerError, "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				writeJSON(t, w, tt.status, `{"redirects":[]}`)
			})
			client := useMockAPI(t, server)
			err := DeleteRedirect(context.Background(), client, testSiteID, "redirect1")
			if gotMethod != http.MethodDelete || gotPath != "/v2/sites/"+testSiteID+"/redirects/redirect1" {
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
