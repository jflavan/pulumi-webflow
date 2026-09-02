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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// pageMetadataMock records PUT/GET requests against /v2/pages/{id}.
type pageMetadataMock struct {
	server    *httptest.Server
	putCalls  int
	getCalls  int
	putQuery  string
	putBody   string
	getQuery  string
	respSlug  string
	getStatus int
}

func newPageMetadataMock(t *testing.T) *pageMetadataMock {
	t.Helper()
	m := &pageMetadataMock{respSlug: "about", getStatus: http.StatusOK}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/pages/"+testPageID {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPut:
			m.putCalls++
			m.putQuery = r.URL.RawQuery
			b, _ := io.ReadAll(r.Body)
			m.putBody = string(b)
		case http.MethodGet:
			m.getCalls++
			m.getQuery = r.URL.RawQuery
			if m.getStatus != http.StatusOK {
				w.WriteHeader(m.getStatus)
				_, _ = w.Write([]byte(`{"message":"error"}`))
				return
			}
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(Page{
			ID: testPageID, SiteID: testPageSiteID, Title: "About", Slug: m.respSlug, ParentID: "5f0c8c9e1c9d440000e8d8c5",
			CreatedOn: "2024-01-01T00:00:00Z", LastUpdated: "2024-03-01T00:00:00Z", Draft: true, PublishedPath: "/" + m.respSlug,
			SEO:       &PageSEO{Title: "SEO About", Description: "SEO desc"},
			OpenGraph: &PageOpenGraph{Title: "OG About", TitleCopied: false, Description: "OG desc", DescriptionCopied: true},
		})
	}))
	t.Cleanup(m.server.Close)
	useMockAPI(t, m.server)
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	return m
}

func TestPageMetadataResourceIDRoundTrip(t *testing.T) {
	if id := GeneratePageMetadataResourceID(testPageID, ""); id != testPageID+"/metadata" {
		t.Errorf("id = %q", id)
	}
	if id := GeneratePageMetadataResourceID(testPageID, testLocaleID); id != testPageID+"/metadata/"+testLocaleID {
		t.Errorf("id = %q", id)
	}
	pageID, localeID, err := ExtractIDsFromPageMetadataResourceID(testPageID + "/metadata/" + testLocaleID)
	if err != nil || pageID != testPageID || localeID != testLocaleID {
		t.Errorf("extract: %q %q %v", pageID, localeID, err)
	}
	pageID, localeID, err = ExtractIDsFromPageMetadataResourceID(testPageID + "/metadata")
	if err != nil || pageID != testPageID || localeID != "" {
		t.Errorf("extract: %q %q %v", pageID, localeID, err)
	}
	for _, bad := range []string{
		"", "/metadata", testPageID, testPageID + "/content", testPageID + "/metadata/", testPageID + "/metadata/a/b",
	} {
		if _, _, err := ExtractIDsFromPageMetadataResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestPageMetadataCreate_SendsOnlySetFields(t *testing.T) {
	m := newPageMetadataMock(t)

	resp, err := (&PageMetadata{}).Create(context.Background(), infer.CreateRequest[PageMetadataArgs]{
		Inputs: PageMetadataArgs{
			PageID: testPageID, LocaleID: testLocaleID, Title: "About",
			SEO:       &PageSEOArgs{Description: "SEO desc"},
			OpenGraph: &PageOpenGraphArgs{DescriptionCopied: ptr(true)},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.putCalls != 1 || m.putQuery != "localeId="+testLocaleID {
		t.Errorf("put calls=%d query=%q", m.putCalls, m.putQuery)
	}
	want := `{"title":"About","seo":{"description":"SEO desc"},"openGraph":{"descriptionCopied":true}}`
	if m.putBody != want {
		t.Errorf("PUT body\n got %s\nwant %s", m.putBody, want)
	}
	if resp.ID != testPageID+"/metadata/"+testLocaleID {
		t.Errorf("ID = %q", resp.ID)
	}
	out := resp.Output
	if out.SiteID != testPageSiteID || out.CurrentSlug != "about" || out.PublishedPath != "/about" || !out.Draft ||
		out.LastUpdated != "2024-03-01T00:00:00Z" || out.Title != "About" || out.SEO.Description != "SEO desc" {
		t.Errorf("unexpected output %+v", out)
	}
}

func TestPageMetadataCreate_SlugIgnoredIsSurfaced(t *testing.T) {
	m := newPageMetadataMock(t)
	m.respSlug = "index" // Webflow keeps the original slug for restricted pages

	resp, err := (&PageMetadata{}).Create(context.Background(), infer.CreateRequest[PageMetadataArgs]{
		Inputs: PageMetadataArgs{PageID: testPageID, Slug: "new-home"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.putQuery != "" || m.putBody != `{"slug":"new-home"}` {
		t.Errorf("query=%q body=%s", m.putQuery, m.putBody)
	}
	if resp.Output.Slug != "new-home" || resp.Output.CurrentSlug != "index" {
		t.Errorf("expected requested slug kept and current slug reported: %+v", resp.Output)
	}
}

func TestPageMetadataCreate_DryRunThenValidation(t *testing.T) {
	m := newPageMetadataMock(t)

	resp, err := (&PageMetadata{}).Create(context.Background(), infer.CreateRequest[PageMetadataArgs]{
		Inputs: PageMetadataArgs{PageID: "", Title: ""}, DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if m.putCalls != 0 || resp.Output.SiteID != "" || resp.Output.LastUpdated != "" {
		t.Errorf("dry run must not call the API or fabricate values: %+v", resp.Output)
	}

	tests := []struct {
		name string
		args PageMetadataArgs
		want string
	}{
		{"invalid pageId", PageMetadataArgs{PageID: "bad", Title: "x"}, "pageId has invalid format"},
		{"invalid localeId", PageMetadataArgs{PageID: testPageID, LocaleID: "en", Title: "x"}, "localeId has invalid format"},
		{
			"nothing managed",
			PageMetadataArgs{PageID: testPageID, SEO: &PageSEOArgs{}, OpenGraph: &PageOpenGraphArgs{}},
			"set at least one of",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&PageMetadata{}).Create(context.Background(), infer.CreateRequest[PageMetadataArgs]{Inputs: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected %q, got %v", tt.want, err)
			}
		})
	}
	if m.putCalls != 0 {
		t.Error("validation failures must not reach the API")
	}
}

func TestPageMetadataRead_ReportsManagedFieldsOnly(t *testing.T) {
	m := newPageMetadataMock(t)
	id := GeneratePageMetadataResourceID(testPageID, testLocaleID)

	resp, err := (&PageMetadata{}).Read(context.Background(), infer.ReadRequest[PageMetadataArgs, PageMetadataState]{
		ID: id,
		State: PageMetadataState{PageMetadataArgs: PageMetadataArgs{
			PageID: testPageID, LocaleID: testLocaleID, Title: "Old title",
			OpenGraph: &PageOpenGraphArgs{TitleCopied: ptr(true)},
		}},
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if m.getCalls != 1 || m.getQuery != "localeId="+testLocaleID {
		t.Errorf("get calls=%d query=%q", m.getCalls, m.getQuery)
	}
	in := resp.Inputs
	if in.PageID != testPageID || in.LocaleID != testLocaleID {
		t.Errorf("ids not preserved: %+v", in)
	}
	if in.Title != "About" {
		t.Errorf("managed title should reflect live value, got %q", in.Title)
	}
	if in.Slug != "" || in.SEO != nil {
		t.Errorf("unmanaged fields must stay unset: slug=%q seo=%+v", in.Slug, in.SEO)
	}
	if in.OpenGraph == nil || in.OpenGraph.TitleCopied == nil || *in.OpenGraph.TitleCopied != false ||
		in.OpenGraph.Title != "" {
		t.Errorf("only the managed openGraph field should be read: %+v", in.OpenGraph)
	}
	if resp.State.CurrentSlug != "about" || resp.State.SiteID != testPageSiteID {
		t.Errorf("state = %+v", resp.State)
	}
}

func TestPageMetadataRead_ImportCapturesEverything(t *testing.T) {
	newPageMetadataMock(t)
	resp, err := (&PageMetadata{}).Read(context.Background(), infer.ReadRequest[PageMetadataArgs, PageMetadataState]{
		ID: GeneratePageMetadataResourceID(testPageID, ""),
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	in := resp.Inputs
	if in.Title != "About" || in.Slug != "about" || in.SEO == nil || in.SEO.Title != "SEO About" ||
		in.OpenGraph == nil || in.OpenGraph.Description != "OG desc" || in.OpenGraph.DescriptionCopied == nil ||
		!*in.OpenGraph.DescriptionCopied {
		t.Errorf("import should capture all fields: %+v", in)
	}
}

func TestPageMetadataRead_NotFoundAndErrors(t *testing.T) {
	m := newPageMetadataMock(t)
	id := GeneratePageMetadataResourceID(testPageID, "")

	m.getStatus = http.StatusNotFound
	resp, err := (&PageMetadata{}).Read(
		context.Background(),
		infer.ReadRequest[PageMetadataArgs, PageMetadataState]{ID: id},
	)
	if err != nil || resp.ID != "" {
		t.Errorf("404 should clear the resource: id=%q err=%v", resp.ID, err)
	}

	m.getStatus = http.StatusForbidden
	if _, err := (&PageMetadata{}).Read(
		context.Background(), infer.ReadRequest[PageMetadataArgs, PageMetadataState]{ID: id},
	); err == nil {
		t.Error("403 must propagate")
	}

	calls := m.getCalls
	for _, bad := range []string{"", "bad/metadata", testPageID + "/metadata/en"} {
		if _, err := (&PageMetadata{}).Read(
			context.Background(), infer.ReadRequest[PageMetadataArgs, PageMetadataState]{ID: bad},
		); err == nil {
			t.Errorf("expected invalid ID error for %q", bad)
		}
	}
	if m.getCalls != calls {
		t.Error("invalid IDs must be rejected before calling the API")
	}
}

func TestPageMetadataUpdate(t *testing.T) {
	m := newPageMetadataMock(t)
	args := PageMetadataArgs{PageID: testPageID, Title: "Renamed"}

	dry, err := (&PageMetadata{}).Update(context.Background(), infer.UpdateRequest[PageMetadataArgs, PageMetadataState]{
		Inputs: args, State: PageMetadataState{SiteID: testPageSiteID}, DryRun: true,
	})
	if err != nil || m.putCalls != 0 || dry.Output.Title != "Renamed" || dry.Output.SiteID != testPageSiteID {
		t.Fatalf("dry run update: %+v %v calls=%d", dry.Output, err, m.putCalls)
	}

	resp, err := (&PageMetadata{}).Update(
		context.Background(),
		infer.UpdateRequest[PageMetadataArgs, PageMetadataState]{Inputs: args},
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if m.putCalls != 1 || m.putBody != `{"title":"Renamed"}` || resp.Output.LastUpdated != "2024-03-01T00:00:00Z" {
		t.Errorf("calls=%d body=%s output=%+v", m.putCalls, m.putBody, resp.Output)
	}

	if _, err := (&PageMetadata{}).Update(context.Background(), infer.UpdateRequest[PageMetadataArgs, PageMetadataState]{
		Inputs: PageMetadataArgs{PageID: testPageID},
	}); err == nil {
		t.Error("update with nothing managed must fail validation")
	}
}

func TestPageMetadataDiff(t *testing.T) {
	base := PageMetadataArgs{
		PageID: testPageID, LocaleID: testLocaleID, Title: "About", Slug: "about",
		SEO:       &PageSEOArgs{Title: "SEO"},
		OpenGraph: &PageOpenGraphArgs{TitleCopied: ptr(true)},
	}
	state := PageMetadataState{PageMetadataArgs: base}

	resp, err := (&PageMetadata{}).Diff(
		context.Background(),
		infer.DiffRequest[PageMetadataArgs, PageMetadataState]{Inputs: base, State: state},
	)
	if err != nil || resp.HasChanges {
		t.Fatalf("expected no changes: %+v %v", resp, err)
	}

	// nil and empty nested blocks are equivalent.
	in := base
	in.SEO = &PageSEOArgs{Title: "SEO"}
	st := state
	st.OpenGraph = &PageOpenGraphArgs{TitleCopied: ptr(true)}
	if resp, _ := (&PageMetadata{}).Diff(
		context.Background(), infer.DiffRequest[PageMetadataArgs, PageMetadataState]{Inputs: in, State: st},
	); resp.HasChanges {
		t.Errorf("equal nested blocks must not diff: %+v", resp)
	}
	noSEO := base
	noSEO.SEO = nil
	emptySEO := state
	emptySEO.SEO = &PageSEOArgs{}
	if resp, _ := (&PageMetadata{}).Diff(
		context.Background(), infer.DiffRequest[PageMetadataArgs, PageMetadataState]{Inputs: noSEO, State: emptySEO},
	); resp.HasChanges {
		t.Errorf("nil vs empty seo must not diff: %+v", resp)
	}

	tests := []struct {
		field  string
		kind   p.DiffKind
		modify func(a *PageMetadataArgs)
	}{
		{"pageId", p.UpdateReplace, func(a *PageMetadataArgs) { a.PageID = "5f0c8c9e1c9d440000e8d8c9" }},
		{"localeId", p.UpdateReplace, func(a *PageMetadataArgs) { a.LocaleID = "" }},
		{"title", p.Update, func(a *PageMetadataArgs) { a.Title = "New" }},
		{"slug", p.Update, func(a *PageMetadataArgs) { a.Slug = "new" }},
		{"seo", p.Update, func(a *PageMetadataArgs) { a.SEO = &PageSEOArgs{Title: "SEO", Description: "d"} }},
		{"openGraph", p.Update, func(a *PageMetadataArgs) { a.OpenGraph = &PageOpenGraphArgs{TitleCopied: ptr(false)} }},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			in := base
			tt.modify(&in)
			resp, err := (&PageMetadata{}).Diff(
				context.Background(),
				infer.DiffRequest[PageMetadataArgs, PageMetadataState]{Inputs: in, State: state},
			)
			if err != nil {
				t.Fatal(err)
			}
			if !resp.HasChanges || resp.DetailedDiff[tt.field].Kind != tt.kind {
				t.Errorf("expected %s %s, got %+v", tt.field, tt.kind, resp)
			}
		})
	}
}

func TestPageMetadataDelete_NoAPICall(t *testing.T) {
	m := newPageMetadataMock(t)
	_, err := (&PageMetadata{}).Delete(context.Background(), infer.DeleteRequest[PageMetadataState]{
		ID:    GeneratePageMetadataResourceID(testPageID, ""),
		State: PageMetadataState{PageMetadataArgs: PageMetadataArgs{PageID: testPageID, Title: "About"}},
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if m.putCalls != 0 || m.getCalls != 0 {
		t.Error("Delete must not call the API")
	}
}
