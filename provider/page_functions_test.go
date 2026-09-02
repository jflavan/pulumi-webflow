// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// asAPIError is a tiny helper so tests read clearly.
func asAPIError(err error, target **APIError) bool { return errors.As(err, target) }

func TestGetPageFunction_Invoke(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/pages/"+testPageID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"id":"` + testPageID + `","siteId":"` + testPageSiteID + `","title":"About","slug":"about",` +
			`"createdOn":"2024-01-01T00:00:00Z","lastUpdated":"2024-01-02T00:00:00Z","archived":false,"draft":false,` +
			`"canBranch":true,"isBranch":true,"branchId":"5f0c8c9e1c9d440000e8d8c7",` +
			`"seo":{"title":"About us","description":"Who we are"},` +
			`"openGraph":{"title":"OG","titleCopied":true,"description":"OGD","descriptionCopied":false},"publishedPath":"/about"}`))
	}))
	defer server.Close()
	useMockAPI(t, server)

	resp, err := (&GetPage{}).Invoke(context.Background(), infer.FunctionRequest[GetPageInput]{
		Input: GetPageInput{PageID: testPageID, LocaleID: testLocaleID, Translatable: true},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotQuery != "localeId="+testLocaleID+"&translatable=true" {
		t.Errorf("unexpected query %q", gotQuery)
	}
	out := resp.Output
	if out.PageID != testPageID || out.Title != "About" || out.Slug != "about" || !out.IsBranch || out.BranchID != "5f0c8c9e1c9d440000e8d8c7" ||
		out.SEO.Title != "About us" || out.SEO.Description != "Who we are" || !out.OpenGraph.TitleCopied || out.OpenGraph.Description != "OGD" ||
		out.PublishedPath != "/about" || !out.CanBranch {
		t.Errorf("unexpected output %+v", out)
	}
}

func TestGetPageFunction_ValidationAndErrors(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"page not found"}`))
	}))
	defer server.Close()
	useMockAPI(t, server)

	if _, err := (&GetPage{}).Invoke(context.Background(), infer.FunctionRequest[GetPageInput]{Input: GetPageInput{PageID: "bad"}}); err == nil ||
		!strings.Contains(err.Error(), "pageId has invalid format") {
		t.Errorf("expected validation error, got %v", err)
	}
	if _, err := (&GetPage{}).Invoke(context.Background(), infer.FunctionRequest[GetPageInput]{Input: GetPageInput{PageID: testPageID, LocaleID: "en"}}); err == nil ||
		!strings.Contains(err.Error(), "localeId has invalid format") {
		t.Errorf("expected locale validation error, got %v", err)
	}
	_, err := (&GetPage{}).Invoke(context.Background(), infer.FunctionRequest[GetPageInput]{Input: GetPageInput{PageID: testPageID}})
	if err == nil || !IsNotFound(err) {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestGetPagesFunction_FollowsPagination(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	server, queries := pageListServer(t, 150)
	useMockAPI(t, server)

	resp, err := (&GetPages{}).Invoke(context.Background(), infer.FunctionRequest[GetPagesInput]{
		Input: GetPagesInput{SiteID: testPageSiteID, LocaleID: testLocaleID},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Output.SiteID != testPageSiteID || len(resp.Output.Pages) != 150 {
		t.Fatalf("expected 150 pages, got %d", len(resp.Output.Pages))
	}
	if resp.Output.Pages[149].Title != "Page 149" || resp.Output.Pages[0].SiteID != testPageSiteID {
		t.Errorf("unexpected pages %+v", resp.Output.Pages[149])
	}
	if len(*queries) != 2 {
		t.Errorf("expected 2 paginated requests, got %v", *queries)
	}
	for _, q := range *queries {
		if !strings.Contains(q, "localeId="+testLocaleID) {
			t.Errorf("expected localeId in query, got %q", q)
		}
	}
}

func TestGetPagesFunction_EmptyAndValidation(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	server, _ := pageListServer(t, 0)
	useMockAPI(t, server)

	resp, err := (&GetPages{}).Invoke(context.Background(), infer.FunctionRequest[GetPagesInput]{Input: GetPagesInput{SiteID: testPageSiteID}})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Output.Pages == nil || len(resp.Output.Pages) != 0 {
		t.Errorf("expected empty non-nil list, got %v", resp.Output.Pages)
	}
	if _, err := (&GetPages{}).Invoke(context.Background(), infer.FunctionRequest[GetPagesInput]{Input: GetPagesInput{SiteID: "bad"}}); err == nil ||
		!strings.Contains(err.Error(), "siteId has invalid format") {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestPageFunctions_RequireToken(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "")
	if _, err := (&GetPages{}).Invoke(context.Background(), infer.FunctionRequest[GetPagesInput]{Input: GetPagesInput{SiteID: testPageSiteID}}); err == nil ||
		!errors.Is(err, ErrTokenNotConfigured) {
		t.Errorf("expected ErrTokenNotConfigured, got %v", err)
	}
}
