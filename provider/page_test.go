// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const (
	testPageSiteID = "5f0c8c9e1c9d440000e8d8c3"
	testPageID     = "5f0c8c9e1c9d440000e8d8c4"
	testLocaleID   = "653fd9af6a07fc9cfd7a5e57"
)

func TestValidatePageID_Format(t *testing.T) {
	for _, id := range []string{"5f0c8c9e1c9d440000e8d8c4", "507f1f77bcf86cd799439011", "abcdef0123456789abcdef01"} {
		if err := ValidatePageID(id); err != nil {
			t.Errorf("ValidatePageID(%q) = %v", id, err)
		}
	}
	if err := ValidatePageID(""); err == nil || !strings.Contains(err.Error(), "required") ||
		!strings.Contains(err.Error(), "24-character") {
		t.Errorf("empty: %v", err)
	}
	for _, id := range []string{
		"5f0c8c9e1c9d", "5f0c8c9e1c9d440000e8d8c4000", "5F0C8C9E1C9D440000E8D8C4",
		"5g0c8c9e1c9d440000e8d8c4", "5f0c8c9e-1c9d-4400-00e8d8c4",
	} {
		if err := ValidatePageID(id); err == nil || !strings.Contains(err.Error(), "invalid format") {
			t.Errorf("ValidatePageID(%q) = %v, want invalid format", id, err)
		}
	}
}

func TestValidateLocaleID(t *testing.T) {
	if err := ValidateLocaleID(""); err != nil {
		t.Errorf("empty locale must be allowed: %v", err)
	}
	if err := ValidateLocaleID(testLocaleID); err != nil {
		t.Errorf("valid locale: %v", err)
	}
	if err := ValidateLocaleID("en-US"); err == nil || !strings.Contains(err.Error(), "localeId has invalid format") {
		t.Errorf("expected invalid format, got %v", err)
	}
}

func TestPageResourceIDRoundTrip(t *testing.T) {
	id := GeneratePageResourceID(testPageSiteID, testPageID)
	if id != testPageSiteID+"/pages/"+testPageID {
		t.Fatalf("unexpected id %q", id)
	}
	siteID, pageID, err := ExtractIDsFromPageResourceID(id)
	if err != nil || siteID != testPageSiteID || pageID != testPageID {
		t.Fatalf("round trip: %q %q %v", siteID, pageID, err)
	}
	for _, bad := range []string{
		"", testPageSiteID + "/" + testPageID, testPageSiteID + "/redirects/" + testPageID, testPageSiteID,
		"/pages/" + testPageID, testPageSiteID + "/pages/",
	} {
		if _, _, err := ExtractIDsFromPageResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

// pageListServer serves total pages across paginated requests and records the queries it saw.
func pageListServer(t *testing.T, total int) (server *httptest.Server, queries *[]string) {
	t.Helper()
	var seen []string
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testPageSiteID+"/pages" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		seen = append(seen, r.URL.RawQuery)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 || limit > 100 {
			t.Errorf("limit must be 1..100, got %d", limit)
		}
		var pages []Page
		for i := offset; i < total && i < offset+limit; i++ {
			pages = append(
				pages,
				Page{
					ID:     fmt.Sprintf("%024d", i),
					SiteID: testPageSiteID,
					Title:  fmt.Sprintf("Page %d", i),
					Slug:   fmt.Sprintf("page-%d", i),
				},
			)
		}
		resp := PagesResponse{Pages: pages}
		resp.Pagination.Limit, resp.Pagination.Offset, resp.Pagination.Total = limit, offset, total
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func TestListPages_FollowsPagination(t *testing.T) {
	server, queries := pageListServer(t, 250)
	client := useMockAPI(t, server)

	pages, err := ListPages(context.Background(), client, testPageSiteID, "")
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(pages) != 250 {
		t.Fatalf("expected 250 pages, got %d", len(pages))
	}
	if pages[249].Title != "Page 249" || pages[0].Title != "Page 0" {
		t.Errorf("pages out of order: first=%q last=%q", pages[0].Title, pages[249].Title)
	}
	if len(*queries) != 3 {
		t.Errorf("expected 3 requests (100+100+50), got %d: %v", len(*queries), *queries)
	}
	for _, q := range *queries {
		if strings.Contains(q, "localeId") {
			t.Errorf("localeId must be omitted when empty: %q", q)
		}
	}
}

func TestListPages_ExactMultipleStopsAtTotal(t *testing.T) {
	server, queries := pageListServer(t, 200)
	client := useMockAPI(t, server)

	pages, err := ListPages(context.Background(), client, testPageSiteID, testLocaleID)
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(pages) != 200 || len(*queries) != 2 {
		t.Errorf("expected 200 pages in 2 requests, got %d pages, queries %v", len(pages), *queries)
	}
	for _, q := range *queries {
		if !strings.Contains(q, "localeId="+testLocaleID) {
			t.Errorf("expected localeId in query, got %q", q)
		}
	}
}

func TestListPages_EmptyAndErrors(t *testing.T) {
	server, _ := pageListServer(t, 0)
	client := useMockAPI(t, server)
	pages, err := ListPages(context.Background(), client, testPageSiteID, "")
	if err != nil || pages == nil || len(pages) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v %v", pages, err)
	}

	for _, status := range []int{
		http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError,
	} {
		errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("error body"))
		}))
		errClient := useMockAPI(t, errServer)
		_, err := ListPages(context.Background(), errClient, testPageSiteID, "")
		errServer.Close()
		if err == nil {
			t.Errorf("status %d: expected error", status)
			continue
		}
		var apiErr *APIError
		if !asAPIError(err, &apiErr) || apiErr.StatusCode != status {
			t.Errorf("status %d: expected APIError, got %v", status, err)
		}
	}
}

func TestGetPageMetadata(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/pages/"+testPageID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"id":"` + testPageID + `","siteId":"` + testPageSiteID + `","title":"Home","slug":"home",` +
			`"parentId":"5f0c8c9e1c9d440000e8d8c5","collectionId":"5f0c8c9e1c9d440000e8d8c6",` +
			`"createdOn":"2024-01-01T00:00:00Z","lastUpdated":"2024-01-02T00:00:00Z",` +
			`"archived":false,"draft":true,"canBranch":true,"isBranch":false,"branchId":null,` +
			`"seo":{"title":"SEO Home","description":"SEO desc"},` +
			`"openGraph":{"title":"OG Home","titleCopied":false,"description":"OG desc","descriptionCopied":true},` +
			`"localeId":"` + testLocaleID + `","publishedPath":"/home"}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	page, err := GetPageMetadata(context.Background(), client, testPageID, "", "")
	if err != nil {
		t.Fatalf("GetPageMetadata: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("expected no query, got %q", gotQuery)
	}
	if page.Title != "Home" || page.ParentID != "5f0c8c9e1c9d440000e8d8c5" || !page.Draft || !page.CanBranch ||
		page.SEO == nil || page.SEO.Description != "SEO desc" || page.OpenGraph == nil || !page.OpenGraph.DescriptionCopied ||
		page.LocaleID != testLocaleID || page.PublishedPath != "/home" {
		t.Errorf("unexpected page %+v", page)
	}

	if _, err := GetPageMetadata(context.Background(), client, testPageID, testLocaleID, testLocaleID); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "localeId="+testLocaleID+"&translatable="+testLocaleID {
		t.Errorf("translatable must be sent verbatim as the locale ID, got query %q", gotQuery)
	}
}

func TestGetPageMetadata_NotFoundIsTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("page not found"))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	if _, err := GetPageMetadata(context.Background(), client, testPageID, "", ""); !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
}

func TestPutPageMetadata(t *testing.T) {
	var gotQuery, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v2/pages/"+testPageID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		gotQuery = r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(Page{ID: testPageID, Title: "New", Slug: "home"})
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	body := PageMetadataUpdateRequest{
		Title: ptr("New"), SEO: &PageSEOUpdate{Description: ptr("d")},
		OpenGraph: &PageOpenGraphUpdate{TitleCopied: ptr(false)},
	}

	page, err := PutPageMetadata(context.Background(), client, testPageID, testLocaleID, body)
	if err != nil {
		t.Fatalf("PutPageMetadata: %v", err)
	}
	if gotQuery != "localeId="+testLocaleID || page.Title != "New" {
		t.Errorf("query=%q page=%+v", gotQuery, page)
	}
	want := `{"title":"New","seo":{"description":"d"},"openGraph":{"titleCopied":false}}`
	if gotBody != want {
		t.Errorf("update request must only contain set fields:\n got %s\nwant %s", gotBody, want)
	}
	if _, err := PutPageMetadata(context.Background(), client, testPageID, "", body); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "" {
		t.Errorf("expected no query without locale, got %q", gotQuery)
	}
}
