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

const (
	testSchemaPageID   = "6596da6045e56dee495bcbba"
	testSchemaSiteID   = "6258612d1ee792848f805dcf"
	testSchemaLocaleID = "653fd9af6a07fc9cfd7a5e57"
	testSchemaPath     = "/beta/pages/" + testSchemaPageID + "/schema-markup"
	testSchemaAuth     = "test-token-abc123def456"
	// testSchemaPretty is a user-formatted document with unsorted keys, HTML characters and a big integer.
	testSchemaPretty = "{\n  \"@type\": \"Organization\",\n  \"@context\": \"https://schema.org\",\n" +
		"  \"name\": \"Acme <b>\", \"n\": 12345678901234567890 }"
	// testSchemaCanon is the canonical encoding of testSchemaPretty.
	testSchemaCanon = `{"@context":"https://schema.org","@type":"Organization","n":12345678901234567890,` +
		`"name":"Acme <b>"}`
	// testSchemaReordered is testSchemaCanon with a different key order, as Webflow might return it.
	testSchemaReordered = `{"name":"Acme <b>","@type":"Organization","n":12345678901234567890,` +
		`"@context":"https://schema.org"}`
)

type schemaReadReq = infer.ReadRequest[PageSchemaMarkupArgs, PageSchemaMarkupState]

// schemaMarkupResponse builds a schema markup API response body around the given jsonLdSchema value.
func schemaMarkupResponse(jsonLD string, inherited bool) string {
	return `{"id":"` + testSchemaPageID + `","siteId":"` + testSchemaSiteID + `","localeId":null,` +
		`"effectiveLocaleId":"` + testSchemaLocaleID + `","publishedPath":"/guide",` +
		`"lastUpdated":"2024-03-11T10:42:42.000Z","jsonLdSchema":` + jsonLD + `,"rawJsonLdSchema":null,` +
		`"isInherited":` + boolString(inherited) + `}`
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// newSchemaServer starts a mock server for the schema markup endpoint and points the client at it.
func newSchemaServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte)) {
	t.Helper()
	t.Setenv("WEBFLOW_API_TOKEN", testSchemaAuth)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		handler(w, r, body)
	}))
	t.Cleanup(server.Close)
	useMockAPI(t, server)
}

// schemaStatusServer replies to every request with a fixed status and body.
func schemaStatusServer(t *testing.T, status int, body string) {
	t.Helper()
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

func TestNormalizeSchemaMarkup(t *testing.T) {
	got, err := NormalizeSchemaMarkup(testSchemaPretty)
	if err != nil {
		t.Fatalf("NormalizeSchemaMarkup: %v", err)
	}
	if got != testSchemaCanon {
		t.Errorf("canonical = %s, want %s", got, testSchemaCanon)
	}
	for name, bad := range map[string]string{
		"empty":    "   ",
		"invalid":  "{not json",
		"array":    `[{"@type":"Thing"}]`,
		"string":   `"hello"`,
		"trailing": `{"a":1} {"b":2}`,
		"too big":  "{\"a\":\"" + strings.Repeat("x", maxSchemaMarkupBytes) + "\"}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeSchemaMarkup(bad); err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
}

func TestPageSchemaMarkupResourceID(t *testing.T) {
	if got := GeneratePageSchemaMarkupResourceID(testSchemaPageID, ""); got != testSchemaPageID+"/schema-markup" {
		t.Errorf("unexpected ID %q", got)
	}
	withLocale := GeneratePageSchemaMarkupResourceID(testSchemaPageID, testSchemaLocaleID)
	pageID, localeID, err := ExtractIDsFromPageSchemaMarkupResourceID(withLocale)
	if err != nil || pageID != testSchemaPageID || localeID != testSchemaLocaleID {
		t.Errorf("Extract(%q) = %q, %q, %v", withLocale, pageID, localeID, err)
	}
	pageID, localeID, err = ExtractIDsFromPageSchemaMarkupResourceID(testSchemaPageID + "/schema-markup")
	if err != nil || pageID != testSchemaPageID || localeID != "" {
		t.Errorf("Extract primary = %q, %q, %v", pageID, localeID, err)
	}
	invalid := []string{"", "abc", "abc/custom_code", "/schema-markup", "abc/schema-markup/", "a/schema-markup/b/c"}
	for _, bad := range invalid {
		if _, _, err := ExtractIDsFromPageSchemaMarkupResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestPageSchemaMarkupCreate_PutsCanonicalJSON(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody string
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath, gotQuery, gotBody = r.Method, r.URL.Path, r.URL.RawQuery, string(body)
		_, _ = w.Write([]byte(schemaMarkupResponse(testSchemaCanon, false)))
	})

	resp, err := (&PageSchemaMarkup{}).Create(context.Background(), infer.CreateRequest[PageSchemaMarkupArgs]{
		Inputs: PageSchemaMarkupArgs{PageID: testSchemaPageID, LocaleID: testSchemaLocaleID, SchemaMarkup: testSchemaPretty},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != testSchemaPath {
		t.Errorf("expected PUT %s, got %s %s", testSchemaPath, gotMethod, gotPath)
	}
	if gotQuery != "localeId="+testSchemaLocaleID {
		t.Errorf("expected localeId query, got %q", gotQuery)
	}
	// The wire body must be the canonical document (sorted keys, no whitespace) as encoding/json renders
	// it; encoding/json HTML-escapes '<' and '>' inside strings, which the API decodes identically.
	wantBytes, err := json.Marshal(PageSchemaMarkupRequest{JSONLDSchema: json.RawMessage(testSchemaCanon)})
	if err != nil {
		t.Fatalf("marshal expected body: %v", err)
	}
	wantBody := string(wantBytes)
	if !strings.Contains(wantBody, `"@context":"https://schema.org","@type":"Organization","n":12345678901234567890`) {
		t.Fatalf("expected body is not canonical: %s", wantBody)
	}
	if gotBody != wantBody {
		t.Errorf("body = %s\nwant %s", gotBody, wantBody)
	}
	if resp.ID != testSchemaPageID+"/schema-markup/"+testSchemaLocaleID {
		t.Errorf("unexpected ID %q", resp.ID)
	}
	if resp.Output.SchemaMarkup != testSchemaPretty {
		t.Errorf("state should keep the user's formatting, got %q", resp.Output.SchemaMarkup)
	}
	out := resp.Output
	if out.SiteID != testSchemaSiteID || out.PublishedPath != "/guide" || out.EffectiveLocaleID != testSchemaLocaleID ||
		out.LastUpdated == "" || out.IsInherited {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestPageSchemaMarkupCreate_DryRun(t *testing.T) {
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		t.Errorf("no API call expected during preview")
	})

	resp, err := (&PageSchemaMarkup{}).Create(context.Background(), infer.CreateRequest[PageSchemaMarkupArgs]{
		Inputs: PageSchemaMarkupArgs{PageID: "", SchemaMarkup: ""},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create dry-run: %v", err)
	}
	if resp.Output.SiteID != "" || resp.Output.LastUpdated != "" {
		t.Errorf("preview must not fabricate outputs: %+v", resp.Output)
	}
}

func TestPageSchemaMarkupCreate_ValidationErrors(t *testing.T) {
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		t.Errorf("no API call expected on validation failure")
	})

	cases := []struct {
		name string
		args PageSchemaMarkupArgs
		want string
	}{
		{"bad page", PageSchemaMarkupArgs{PageID: "x", SchemaMarkup: "{}"}, "pageId"},
		{"bad locale", PageSchemaMarkupArgs{PageID: testSchemaPageID, LocaleID: "en", SchemaMarkup: "{}"}, "localeId"},
		{"bad json", PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: "{"}, "not valid JSON"},
		{"not object", PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: "[]"}, "JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&PageSchemaMarkup{}).Create(context.Background(),
				infer.CreateRequest[PageSchemaMarkupArgs]{Inputs: tc.args})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPageSchemaMarkupRead_Success(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(schemaMarkupResponse(testSchemaReordered, false)))
	})

	id := GeneratePageSchemaMarkupResourceID(testSchemaPageID, "")
	resp, err := (&PageSchemaMarkup{}).Read(context.Background(), schemaReadReq{
		ID:     id,
		Inputs: PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: testSchemaPretty},
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != testSchemaPath || gotQuery != "" {
		t.Errorf("expected GET %s without query, got %s %s?%s", testSchemaPath, gotMethod, gotPath, gotQuery)
	}
	if resp.ID != id {
		t.Errorf("expected ID preserved, got %q", resp.ID)
	}
	if resp.Inputs.SchemaMarkup != testSchemaPretty {
		t.Errorf("semantically equal markup should keep user formatting, got %q", resp.Inputs.SchemaMarkup)
	}
	if resp.State.PublishedPath != "/guide" {
		t.Errorf("unexpected state: %+v", resp.State)
	}
}

func TestPageSchemaMarkupRead_Drift(t *testing.T) {
	schemaStatusServer(t, http.StatusOK, schemaMarkupResponse(`{"@type":"Thing"}`, false))

	resp, err := (&PageSchemaMarkup{}).Read(context.Background(), schemaReadReq{
		ID:     GeneratePageSchemaMarkupResourceID(testSchemaPageID, ""),
		Inputs: PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: testSchemaPretty},
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.Inputs.SchemaMarkup != `{"@type":"Thing"}` {
		t.Errorf("expected drifted canonical markup, got %q", resp.Inputs.SchemaMarkup)
	}
}

func TestPageSchemaMarkupRead_GoneCases(t *testing.T) {
	cases := map[string]struct {
		status   int
		body     string
		localeID string
	}{
		"404":       {http.StatusNotFound, `{"message":"not found"}`, ""},
		"cleared":   {http.StatusOK, schemaMarkupResponse("null", false), ""},
		"inherited": {http.StatusOK, schemaMarkupResponse(`{"@type":"Thing"}`, true), testSchemaLocaleID},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			schemaStatusServer(t, tc.status, tc.body)

			resp, err := (&PageSchemaMarkup{}).Read(context.Background(), schemaReadReq{
				ID: GeneratePageSchemaMarkupResourceID(testSchemaPageID, tc.localeID),
			})
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if resp.ID != "" {
				t.Errorf("expected empty ID, got %q", resp.ID)
			}
		})
	}
}

func TestPageSchemaMarkupRead_ServerErrorPropagates(t *testing.T) {
	schemaStatusServer(t, http.StatusInternalServerError, `{"message":"page not found upstream"}`)

	_, err := (&PageSchemaMarkup{}).Read(context.Background(), schemaReadReq{
		ID: GeneratePageSchemaMarkupResourceID(testSchemaPageID, ""),
	})
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error, got %v", err)
	}
}

func TestPageSchemaMarkupUpdate_Puts(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, string(body)
		_, _ = w.Write([]byte(schemaMarkupResponse(`{"@type":"Thing"}`, false)))
	})

	resp, err := (&PageSchemaMarkup{}).Update(context.Background(),
		infer.UpdateRequest[PageSchemaMarkupArgs, PageSchemaMarkupState]{
			ID:     GeneratePageSchemaMarkupResourceID(testSchemaPageID, ""),
			Inputs: PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: ` { "@type" : "Thing" } `},
			State: PageSchemaMarkupState{
				PageSchemaMarkupArgs: PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: "{}"},
			},
		})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != testSchemaPath {
		t.Errorf("expected PUT %s, got %s %s", testSchemaPath, gotMethod, gotPath)
	}
	if gotBody != `{"jsonLdSchema":{"@type":"Thing"}}` {
		t.Errorf("unexpected body %s", gotBody)
	}
	if resp.Output.SchemaMarkup != ` { "@type" : "Thing" } ` || resp.Output.LastUpdated == "" {
		t.Errorf("unexpected output %+v", resp.Output)
	}
}

func TestPageSchemaMarkupUpdate_DryRun(t *testing.T) {
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		t.Errorf("no API call expected during preview")
	})

	resp, err := (&PageSchemaMarkup{}).Update(context.Background(),
		infer.UpdateRequest[PageSchemaMarkupArgs, PageSchemaMarkupState]{
			Inputs: PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: "{}"},
			State:  PageSchemaMarkupState{SiteID: "site", LastUpdated: "yesterday"},
			DryRun: true,
		})
	if err != nil {
		t.Fatalf("Update dry-run: %v", err)
	}
	if resp.Output.SchemaMarkup != "{}" || resp.Output.SiteID != "site" || resp.Output.LastUpdated != "yesterday" {
		t.Errorf("unexpected preview output %+v", resp.Output)
	}
}

func TestPageSchemaMarkupDelete_PutsNull(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody string
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath, gotQuery, gotBody = r.Method, r.URL.Path, r.URL.RawQuery, string(body)
		_, _ = w.Write([]byte(schemaMarkupResponse("null", false)))
	})

	_, err := (&PageSchemaMarkup{}).Delete(context.Background(), infer.DeleteRequest[PageSchemaMarkupState]{
		ID: GeneratePageSchemaMarkupResourceID(testSchemaPageID, testSchemaLocaleID),
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != testSchemaPath || gotQuery != "localeId="+testSchemaLocaleID {
		t.Errorf("expected PUT %s?localeId=..., got %s %s?%s", testSchemaPath, gotMethod, gotPath, gotQuery)
	}
	if gotBody != `{"jsonLdSchema":null}` {
		t.Errorf("expected null body, got %s", gotBody)
	}
}

func TestPageSchemaMarkupDelete_NotFoundIsSuccess(t *testing.T) {
	schemaStatusServer(t, http.StatusNotFound, "")

	if _, err := (&PageSchemaMarkup{}).Delete(context.Background(), infer.DeleteRequest[PageSchemaMarkupState]{
		ID: GeneratePageSchemaMarkupResourceID(testSchemaPageID, ""),
	}); err != nil {
		t.Fatalf("Delete should treat 404 as success, got %v", err)
	}
}

func TestPageSchemaMarkupDiff(t *testing.T) {
	base := PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: testSchemaPretty}
	tests := []struct {
		name        string
		inputs      PageSchemaMarkupArgs
		wantChanges bool
		wantReplace bool
		wantKey     string
	}{
		{"identical", base, false, false, ""},
		{
			"reordered keys",
			PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: testSchemaReordered},
			false, false, "",
		},
		{
			"content change",
			PageSchemaMarkupArgs{PageID: testSchemaPageID, SchemaMarkup: `{"@type":"Thing"}`},
			true, false, "schemaMarkup",
		},
		{
			"page change",
			PageSchemaMarkupArgs{PageID: "6596da6045e56dee495bcbbb", SchemaMarkup: testSchemaPretty},
			true, true, "pageId",
		},
		{"locale change", PageSchemaMarkupArgs{
			PageID: testSchemaPageID, LocaleID: testSchemaLocaleID, SchemaMarkup: testSchemaPretty,
		}, true, true, "localeId"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := (&PageSchemaMarkup{}).Diff(context.Background(),
				infer.DiffRequest[PageSchemaMarkupArgs, PageSchemaMarkupState]{
					Inputs: tt.inputs,
					State:  PageSchemaMarkupState{PageSchemaMarkupArgs: base},
				})
			if err != nil {
				t.Fatalf("Diff: %v", err)
			}
			if resp.HasChanges != tt.wantChanges || resp.DeleteBeforeReplace != tt.wantReplace {
				t.Fatalf("HasChanges=%v DeleteBeforeReplace=%v, want %v/%v",
					resp.HasChanges, resp.DeleteBeforeReplace, tt.wantChanges, tt.wantReplace)
			}
			if tt.wantKey == "" {
				if len(resp.DetailedDiff) != 0 {
					t.Errorf("expected no detailed diff, got %v", resp.DetailedDiff)
				}
				return
			}
			d, ok := resp.DetailedDiff[tt.wantKey]
			if !ok {
				t.Fatalf("expected %q in detailed diff: %v", tt.wantKey, resp.DetailedDiff)
			}
			if tt.wantReplace && d.Kind != p.UpdateReplace || !tt.wantReplace && d.Kind != p.Update {
				t.Errorf("unexpected diff kind %v for %q", d.Kind, tt.wantKey)
			}
		})
	}
}

func TestGetPageSchemaMarkupFunction_Success(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"id":"` + testSchemaPageID + `","siteId":"` + testSchemaSiteID + `",` +
			`"localeId":"` + testSchemaLocaleID + `","effectiveLocaleId":"aaaaaaaaaaaaaaaaaaaaaaaa",` +
			`"publishedPath":"/guide","lastUpdated":"2024-03-11T10:42:42.000Z",` +
			`"jsonLdSchema":{"name":"Acme","@type":"Organization"},"rawJsonLdSchema":"<script>raw</script>",` +
			`"isInherited":true}`))
	})

	resp, err := (&GetPageSchemaMarkup{}).Invoke(context.Background(), infer.FunctionRequest[GetPageSchemaMarkupInput]{
		Input: GetPageSchemaMarkupInput{PageID: testSchemaPageID, LocaleID: testSchemaLocaleID},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != testSchemaPath || gotQuery != "localeId="+testSchemaLocaleID {
		t.Errorf("expected GET %s?localeId=..., got %s %s?%s", testSchemaPath, gotMethod, gotPath, gotQuery)
	}
	out := resp.Output
	if out.PageID != testSchemaPageID || out.SiteID != testSchemaSiteID || out.LocaleID != testSchemaLocaleID ||
		out.EffectiveLocaleID != "aaaaaaaaaaaaaaaaaaaaaaaa" || out.PublishedPath != "/guide" || out.LastUpdated == "" {
		t.Errorf("unexpected identity fields %+v", out)
	}
	if out.SchemaMarkup != `{"@type":"Organization","name":"Acme"}` || out.RawSchemaMarkup != "<script>raw</script>" ||
		!out.IsInherited {
		t.Errorf("unexpected markup fields %+v", out)
	}
}

func TestGetPageSchemaMarkupFunction_NoMarkup(t *testing.T) {
	schemaStatusServer(t, http.StatusOK,
		`{"id":"`+testSchemaPageID+`","jsonLdSchema":null,"effectiveLocaleId":null,"isInherited":false}`)

	resp, err := (&GetPageSchemaMarkup{}).Invoke(context.Background(), infer.FunctionRequest[GetPageSchemaMarkupInput]{
		Input: GetPageSchemaMarkupInput{PageID: testSchemaPageID},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Output.SchemaMarkup != "" || resp.Output.EffectiveLocaleID != "" {
		t.Errorf("expected empty markup, got %+v", resp.Output)
	}
}

func TestGetPageSchemaMarkupFunction_NotFound(t *testing.T) {
	schemaStatusServer(t, http.StatusNotFound, `{"message":"Requested resource not found"}`)

	_, err := (&GetPageSchemaMarkup{}).Invoke(context.Background(), infer.FunctionRequest[GetPageSchemaMarkupInput]{
		Input: GetPageSchemaMarkupInput{PageID: testSchemaPageID},
	})
	if err == nil || !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestGetPageSchemaMarkupFunction_Validation(t *testing.T) {
	_, err := (&GetPageSchemaMarkup{}).Invoke(context.Background(), infer.FunctionRequest[GetPageSchemaMarkupInput]{
		Input: GetPageSchemaMarkupInput{PageID: "bad"},
	})
	if err == nil || !strings.Contains(err.Error(), "pageId") {
		t.Fatalf("expected pageId validation error, got %v", err)
	}
}
