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
	"reflect"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	testRobotsContent    = "User-agent: *\nAllow: /\nDisallow: /admin/\nSitemap: https://example.com/sitemap.xml"
	testRobotsResourceID = testSiteID + "/robots.txt"
)

// ============================================================================
// Parsing / formatting
// ============================================================================

func TestParseRobotsTxtContent(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		expectedRules   []RobotsTxtRule
		expectedSitemap string
	}{
		{
			name:          "simple allow all",
			content:       "User-agent: *\nAllow: /",
			expectedRules: []RobotsTxtRule{{UserAgent: "*", Allows: []string{"/"}, Disallows: []string{}}},
		},
		{
			name:          "with disallow",
			content:       "User-agent: *\nAllow: /\nDisallow: /admin/",
			expectedRules: []RobotsTxtRule{{UserAgent: "*", Allows: []string{"/"}, Disallows: []string{"/admin/"}}},
		},
		{
			name:            "with sitemap",
			content:         "User-agent: *\nAllow: /\nSitemap: https://example.com/sitemap.xml",
			expectedRules:   []RobotsTxtRule{{UserAgent: "*", Allows: []string{"/"}, Disallows: []string{}}},
			expectedSitemap: "https://example.com/sitemap.xml",
		},
		{
			name:    "multiple user agents, mixed case and CRLF",
			content: "user-agent: *\r\nallow: /\r\n\r\nUSER-AGENT: Googlebot\r\nDISALLOW: /private/",
			expectedRules: []RobotsTxtRule{
				{UserAgent: "*", Allows: []string{"/"}, Disallows: []string{}},
				{UserAgent: "Googlebot", Allows: []string{}, Disallows: []string{"/private/"}},
			},
		},
		{
			name:          "empty content",
			content:       "",
			expectedRules: []RobotsTxtRule{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, sitemap := ParseRobotsTxtContent(tt.content)
			if !reflect.DeepEqual(rules, tt.expectedRules) {
				t.Errorf("rules: got %+v, want %+v", rules, tt.expectedRules)
			}
			if sitemap != tt.expectedSitemap {
				t.Errorf("sitemap: got %q, want %q", sitemap, tt.expectedSitemap)
			}
		})
	}
}

func TestParseRobotsTxtContentWithWarnings(t *testing.T) {
	content := "# generated\nDisallow: /early\nUser-agent: *\nAllow: / # inline\nCrawl-delay: 10\nDisallow: /admin/\n"
	rules, _, warnings := ParseRobotsTxtContentWithWarnings(content)

	if len(rules) != 1 || rules[0].UserAgent != "*" || !reflect.DeepEqual(rules[0].Allows, []string{"/"}) ||
		!reflect.DeepEqual(rules[0].Disallows, []string{"/admin/"}) {
		t.Errorf("unexpected rules: %+v", rules)
	}
	if len(warnings) != 4 {
		t.Fatalf(
			"expected 4 warnings (comment, early directive, inline comment, unknown directive), got %d: %v",
			len(warnings),
			warnings,
		)
	}
	for i, want := range []string{
		"line 1: comment", "line 2:", "line 4: inline comment", "line 5: directive \"Crawl-delay: 10\" is not supported",
	} {
		if !strings.Contains(warnings[i], want) {
			t.Errorf("warning %d = %q, want it to contain %q", i, warnings[i], want)
		}
	}
	// No warnings for clean content
	if _, _, w := ParseRobotsTxtContentWithWarnings(testRobotsContent); len(w) != 0 {
		t.Errorf("clean content produced warnings: %v", w)
	}
}

func TestFormatRobotsTxtContent(t *testing.T) {
	tests := []struct {
		name     string
		rules    []RobotsTxtRule
		sitemap  string
		expected string
	}{
		{"simple allow all", []RobotsTxtRule{{UserAgent: "*", Allows: []string{"/"}}}, "", "User-agent: *\nAllow: /\n"},
		{
			"with disallow",
			[]RobotsTxtRule{{UserAgent: "*", Allows: []string{"/"}, Disallows: []string{"/admin/"}}},
			"",
			"User-agent: *\nAllow: /\nDisallow: /admin/\n",
		},
		{
			"with sitemap",
			[]RobotsTxtRule{{UserAgent: "*", Allows: []string{"/"}}},
			"https://example.com/sitemap.xml",
			"User-agent: *\nAllow: /\n\nSitemap: https://example.com/sitemap.xml\n",
		},
		{"empty rules", []RobotsTxtRule{}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatRobotsTxtContent(tt.rules, tt.sitemap); got != tt.expected {
				t.Errorf("expected:\n%q\ngot:\n%q", tt.expected, got)
			}
		})
	}
}

func TestRobotsTxtContentEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", testRobotsContent, testRobotsContent, true},
		{
			"formatting only", "user-agent: *\n\n  allow: /\nDisallow:   /admin/\n\nsitemap: https://example.com/sitemap.xml\n",
			FormatRobotsTxtContent(ParseRobotsTxtContent(testRobotsContent)), true,
		},
		{"comments ignored", "# hi\nUser-agent: *\nAllow: /", "User-agent: *\nAllow: /\n", true},
		{"different path", "User-agent: *\nDisallow: /a", "User-agent: *\nDisallow: /b", false},
		{"different sitemap", "User-agent: *\nSitemap: https://a", "User-agent: *\nSitemap: https://b", false},
		{"different agent count", "User-agent: *\nAllow: /", "User-agent: *\nAllow: /\nUser-agent: x\nAllow: /", false},
		{"both empty", "", "", true},
		{"empty vs rule", "", "User-agent: *\nAllow: /", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RobotsTxtContentEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("RobotsTxtContentEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ============================================================================
// IDs
// ============================================================================

func TestValidateSiteID(t *testing.T) {
	tests := []struct {
		name    string
		siteID  string
		wantErr bool
	}{
		{"valid 24-char hex", testSiteID, false},
		{"valid all lowercase", "abcdef0123456789abcdef01", false},
		{"preview placeholder", "preview-1234567890", false},
		{"too short", "5f0c8c9e1c9d44", true},
		{"too long", testSiteID + "abc", true},
		{"invalid characters", "5f0c8c9e1c9d440000e8d8XY", true},
		{"empty", "", true},
		{"uppercase hex", "5F0C8C9E1C9D440000E8D8C3", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateSiteID(tt.siteID); (err != nil) != tt.wantErr {
				t.Errorf("ValidateSiteID(%s) error = %v, wantErr %v", tt.siteID, err, tt.wantErr)
			}
		})
	}
}

func TestRobotsTxtResourceID_RoundTrip(t *testing.T) {
	if got := GenerateRobotsTxtResourceID(testSiteID); got != testRobotsResourceID {
		t.Errorf("expected %q, got %q", testRobotsResourceID, got)
	}
	siteID, err := ExtractSiteIDFromResourceID(testRobotsResourceID)
	if err != nil || siteID != testSiteID {
		t.Errorf("ExtractSiteIDFromResourceID = %q, %v", siteID, err)
	}
	for _, bad := range []string{"", "invalid", testSiteID + "/robots", testSiteID + "/robots.txt/extra"} {
		if _, err := ExtractSiteIDFromResourceID(bad); err == nil {
			t.Errorf("ExtractSiteIDFromResourceID(%q) = nil, want error", bad)
		}
	}
}

// ============================================================================
// API functions
// ============================================================================

func TestGetRobotsTxt_Success(t *testing.T) {
	var gotMethod, gotPath string
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		writeJSON(
			t,
			w,
			http.StatusOK,
			`{"rules":[{"userAgent":"*","allows":["/"],"disallows":["/admin/"]}],"sitemap":"https://example.com/sitemap.xml"}`,
		)
	})
	client := useMockAPI(t, server)

	response, err := GetRobotsTxt(context.Background(), client, testSiteID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/v2/sites/"+testSiteID+"/robots_txt" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	if len(response.Rules) != 1 || response.Rules[0].UserAgent != "*" ||
		response.Sitemap != "https://example.com/sitemap.xml" {
		t.Errorf("unexpected response: %+v", response)
	}
}

func TestGetRobotsTxt_NotFound(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, `{"message":"Site not found"}`)
	})
	client := useMockAPI(t, server)
	_, err := GetRobotsTxt(context.Background(), client, testSiteID)
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound error, got: %v", err)
	}
}

func TestGetRobotsTxt_Unauthorized(t *testing.T) {
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, `{"message":"Unauthorized"}`)
	})
	client := useMockAPI(t, server)
	_, err := GetRobotsTxt(context.Background(), client, testSiteID)
	if err == nil || !containsStr(err.Error(), "unauthorized") || !containsStr(err.Error(), "token") {
		t.Errorf("expected actionable unauthorized error, got: %v", err)
	}
}

func TestRobotsTxt_RateLimitRetry(t *testing.T) {
	calls := 0
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			w.Header().Set("Retry-After", "0")
			writeJSON(t, w, http.StatusTooManyRequests, `{"message":"Rate limited"}`)
			return
		}
		writeJSON(t, w, http.StatusOK, `{"rules":[],"sitemap":""}`)
	})
	client := useMockAPI(t, server)

	if _, err := GetRobotsTxt(context.Background(), client, testSiteID); err != nil {
		t.Fatalf("expected retries to succeed, got error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (2 retries), got %d", calls)
	}
}

func TestRobotsTxt_MaxRetriesExceeded(t *testing.T) {
	calls := 0
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(t, w, http.StatusTooManyRequests, `{"message":"Rate limited"}`)
	})
	client := useMockAPI(t, server)

	_, err := GetRobotsTxt(context.Background(), client, testSiteID)
	if err == nil {
		t.Fatal("expected error when max retries exceeded")
	}
	if !containsStr(err.Error(), "rate limited") || !containsStr(err.Error(), "exponential backoff") {
		t.Errorf("expected rate limit guidance in error, got: %v", err)
	}
	if calls != DefaultMaxRetries+1 {
		t.Errorf("expected %d attempts, got %d", DefaultMaxRetries+1, calls)
	}
}

func TestPutRobotsTxt_SendsBody(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotContentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		gotBody = readJSONBody(t, r)
		writeJSON(
			t,
			w,
			http.StatusOK,
			`{"rules":[{"userAgent":"*","allows":["/"]}],"sitemap":"https://example.com/sitemap.xml"}`,
		)
	})
	client := useMockAPI(t, server)

	rules := []RobotsTxtRule{{UserAgent: "*", Allows: []string{"/"}, Disallows: []string{}}}
	response, err := PutRobotsTxt(context.Background(), client, testSiteID, rules, "https://example.com/sitemap.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/v2/sites/"+testSiteID+"/robots_txt" ||
		gotContentType != "application/json" {
		t.Errorf("unexpected request %s %s (%s)", gotMethod, gotPath, gotContentType)
	}
	sent, _ := gotBody["rules"].([]any)
	if len(sent) != 1 || sent[0].(map[string]any)["userAgent"] != "*" ||
		gotBody["sitemap"] != "https://example.com/sitemap.xml" {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if len(response.Rules) != 1 {
		t.Errorf("expected 1 rule in response, got %d", len(response.Rules))
	}
}

func TestDeleteRobotsTxt_SendsRulesAndAccepts200(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, `{"rules":[],"sitemap":""}`)
	})
	client := useMockAPI(t, server)

	rules := []RobotsTxtRule{{UserAgent: "*", Allows: []string{}, Disallows: []string{"/bubbles"}}}
	if err := DeleteRobotsTxt(
		context.Background(), client, testSiteID, rules, "https://example.com/sitemap.xml",
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v2/sites/"+testSiteID+"/robots_txt" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	sent, _ := gotBody["rules"].([]any)
	if len(sent) != 1 {
		t.Fatalf("expected rules in DELETE body, got %v", gotBody)
	}
	rule := sent[0].(map[string]any)
	disallows, _ := rule["disallows"].([]any)
	if rule["userAgent"] != "*" || len(disallows) != 1 || disallows[0] != "/bubbles" ||
		gotBody["sitemap"] != "https://example.com/sitemap.xml" {
		t.Errorf("unexpected DELETE body: %v", gotBody)
	}
}

func TestDeleteRobotsTxt_StatusHandling(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"204", http.StatusNoContent, false},
		{"404 idempotent", http.StatusNotFound, false},
		{"403", http.StatusForbidden, true},
		{"500", http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, tt.status, `{"message":"x"}`)
			})
			client := useMockAPI(t, server)
			err := DeleteRobotsTxt(context.Background(), client, testSiteID, nil, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("status %d: err = %v, wantErr %v", tt.status, err, tt.wantErr)
			}
		})
	}
}

func TestSetProviderVersion(t *testing.T) {
	SetProviderVersion("1.2.3")
	if currentProviderVersion() != "1.2.3" {
		t.Errorf("expected version '1.2.3', got '%s'", currentProviderVersion())
	}
	SetProviderVersion("0.0.0")
}

// ============================================================================
// Resource: Create
// ============================================================================

func TestRobotsTxt_Create_DryRun_SkipsValidationAndAPI(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	resource := &RobotsTxt{}

	// siteId arrives zeroed during preview when it is an unknown output of another resource
	resp, err := resource.Create(context.Background(), infer.CreateRequest[RobotsTxtArgs]{
		Inputs: RobotsTxtArgs{SiteID: "", Content: "User-agent: *\nAllow: /"},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create (DryRun) failed: %v", err)
	}
	if called {
		t.Error("API must not be called in DryRun mode")
	}
	if resp.ID != "/robots.txt" || resp.Output.Content != "User-agent: *\nAllow: /" || resp.Output.LastModified == "" {
		t.Errorf("unexpected preview response: id=%q output=%+v", resp.ID, resp.Output)
	}

	resp, err = resource.Create(context.Background(), infer.CreateRequest[RobotsTxtArgs]{
		Inputs: RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"},
		DryRun: true,
	})
	if err != nil || resp.ID != testRobotsResourceID {
		t.Errorf("expected preview ID %q, got %q (err %v)", testRobotsResourceID, resp.ID, err)
	}
}

func TestRobotsTxt_Create_ValidationErrors(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	resource := &RobotsTxt{}

	tests := []struct {
		name string
		args RobotsTxtArgs
		want []string
	}{
		{"empty siteId", RobotsTxtArgs{SiteID: "", Content: "User-agent: *\nAllow: /"}, []string{"siteId", "required"}},
		{
			"invalid siteId",
			RobotsTxtArgs{SiteID: "invalid-format", Content: "User-agent: *\nAllow: /"},
			[]string{"24-character", "hexadecimal"},
		},
		{"empty content", RobotsTxtArgs{SiteID: testSiteID, Content: ""}, []string{"content", "required"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resource.Create(context.Background(), infer.CreateRequest[RobotsTxtArgs]{Inputs: tt.args})
			if err == nil {
				t.Fatal("expected validation error")
			}
			for _, want := range tt.want {
				if !containsStr(err.Error(), want) {
					t.Errorf("error %q should contain %q", err.Error(), want)
				}
			}
			if containsStr(err.Error(), "HTTP") {
				t.Errorf("validation error must not mention HTTP: %v", err)
			}
		})
	}
	if called {
		t.Error("API must not be called when validation fails")
	}
}

func TestRobotsTxt_Create_Success_KeepsRawContent(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = readJSONBody(t, r)
		writeJSON(
			t,
			w,
			http.StatusOK,
			`{"rules":[{"userAgent":"*","allows":["/"],"disallows":["/admin/"]}],"sitemap":"https://example.com/sitemap.xml"}`,
		)
	})
	resource := &RobotsTxt{}

	raw := "# managed by pulumi\nuser-agent: *\n\nallow: /\ndisallow: /admin/\n\nsitemap: https://example.com/sitemap.xml"
	resp, err := resource.Create(context.Background(), infer.CreateRequest[RobotsTxtArgs]{
		Inputs: RobotsTxtArgs{SiteID: testSiteID, Content: raw},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/v2/sites/"+testSiteID+"/robots_txt" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	rules, _ := gotBody["rules"].([]any)
	if len(rules) != 1 || gotBody["sitemap"] != "https://example.com/sitemap.xml" {
		t.Errorf("unexpected PUT body: %v", gotBody)
	}
	rule := rules[0].(map[string]any)
	if rule["userAgent"] != "*" || len(rule["allows"].([]any)) != 1 || len(rule["disallows"].([]any)) != 1 {
		t.Errorf("unexpected rule in PUT body: %v", rule)
	}
	if resp.ID != testRobotsResourceID {
		t.Errorf("unexpected ID %q", resp.ID)
	}
	if resp.Output.Content != raw {
		t.Errorf("state must keep the user's raw content, got %q", resp.Output.Content)
	}
	if resp.Output.LastModified == "" {
		t.Error("expected lastModified to be set")
	}
}

// ============================================================================
// Resource: Read + Diff (no diff after refresh)
// ============================================================================

func TestRobotsTxt_Read_PreservesRawContentWhenEquivalent(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testSiteID+"/robots_txt" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		writeJSON(
			t,
			w,
			http.StatusOK,
			`{"rules":[{"userAgent":"*","allows":["/"],"disallows":["/admin/"]}],"sitemap":"https://example.com/sitemap.xml"}`,
		)
	})
	resource := &RobotsTxt{}

	raw := "user-agent: *\n\nallow: /\ndisallow: /admin/\n\nsitemap: https://example.com/sitemap.xml"
	readResp, err := resource.Read(context.Background(), infer.ReadRequest[RobotsTxtArgs, RobotsTxtState]{
		ID:     testRobotsResourceID,
		Inputs: RobotsTxtArgs{SiteID: testSiteID, Content: raw},
		State: RobotsTxtState{
			RobotsTxtArgs: RobotsTxtArgs{SiteID: testSiteID, Content: raw},
			LastModified:  "2025-12-10T12:00:00Z",
		},
	})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readResp.ID != testRobotsResourceID || readResp.Inputs.SiteID != testSiteID {
		t.Errorf("unexpected read response: %+v", readResp)
	}
	if readResp.Inputs.Content != raw || readResp.State.Content != raw {
		t.Errorf("raw content must be preserved when Webflow reports the same rules; got inputs=%q state=%q",
			readResp.Inputs.Content, readResp.State.Content)
	}
	if readResp.State.LastModified != "2025-12-10T12:00:00Z" {
		t.Errorf("lastModified must be preserved, got %q", readResp.State.LastModified)
	}

	// The refreshed state must not produce a diff against the program's content.
	diff, err := resource.Diff(context.Background(), infer.DiffRequest[RobotsTxtArgs, RobotsTxtState]{
		Inputs: RobotsTxtArgs{SiteID: testSiteID, Content: raw},
		State:  readResp.State,
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff.HasChanges {
		t.Errorf("expected no diff after refresh, got %+v", diff.DetailedDiff)
	}
}

func TestRobotsTxt_Read_ReportsDrift(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(
			t,
			w,
			http.StatusOK,
			`{"rules":[{"userAgent":"*","allows":[],"disallows":["/changed-in-ui/"]}],"sitemap":""}`,
		)
	})
	resource := &RobotsTxt{}

	program := "User-agent: *\nAllow: /"
	readResp, err := resource.Read(context.Background(), infer.ReadRequest[RobotsTxtArgs, RobotsTxtState]{
		ID:     testRobotsResourceID,
		Inputs: RobotsTxtArgs{SiteID: testSiteID, Content: program},
		State:  RobotsTxtState{RobotsTxtArgs: RobotsTxtArgs{SiteID: testSiteID, Content: program}},
	})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readResp.Inputs.Content != "User-agent: *\nDisallow: /changed-in-ui/\n" {
		t.Errorf("drifted content should come from the API, got %q", readResp.Inputs.Content)
	}
	diff, err := resource.Diff(context.Background(), infer.DiffRequest[RobotsTxtArgs, RobotsTxtState]{
		Inputs: RobotsTxtArgs{SiteID: testSiteID, Content: program},
		State:  readResp.State,
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if !diff.HasChanges || diff.DetailedDiff["content"].Kind != p.Update || diff.DeleteBeforeReplace {
		t.Errorf("expected an in-place content update for drift, got %+v", diff)
	}
}

func TestRobotsTxt_Read_ImportWithoutState(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, `{"rules":[{"userAgent":"*","allows":["/"],"disallows":[]}],"sitemap":""}`)
	})
	resp, err := (&RobotsTxt{}).Read(
		context.Background(),
		infer.ReadRequest[RobotsTxtArgs, RobotsTxtState]{ID: testRobotsResourceID},
	)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if resp.Inputs.Content != "User-agent: *\nAllow: /\n" || resp.Inputs.SiteID != testSiteID {
		t.Errorf("unexpected imported inputs: %+v", resp.Inputs)
	}
}

func TestRobotsTxt_Read_NotFoundSignalsDeletion(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, `{"message":"Not found"}`)
	})
	resp, err := (&RobotsTxt{}).Read(
		context.Background(),
		infer.ReadRequest[RobotsTxtArgs, RobotsTxtState]{ID: testRobotsResourceID},
	)
	if err != nil || resp.ID != "" {
		t.Errorf("expected empty ID without error for 404, got id=%q err=%v", resp.ID, err)
	}
}

func TestRobotsTxt_Read_OtherErrorsAreErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
		mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, status, `{"message":"not found"}`)
		})
		_, err := (&RobotsTxt{}).Read(
			context.Background(),
			infer.ReadRequest[RobotsTxtArgs, RobotsTxtState]{ID: testRobotsResourceID},
		)
		if err == nil {
			t.Errorf("status %d must surface as an error, not deletion", status)
		}
	}
}

func TestRobotsTxt_Read_InvalidID(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	for _, id := range []string{"invalid", "not-hex/robots.txt", ""} {
		_, err := (&RobotsTxt{}).Read(context.Background(), infer.ReadRequest[RobotsTxtArgs, RobotsTxtState]{ID: id})
		if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
			t.Errorf("ID %q: expected invalid resource ID error, got %v", id, err)
		}
	}
	if called {
		t.Error("API must not be called with an invalid ID")
	}
}

// ============================================================================
// Resource: Diff
// ============================================================================

func robotsDiff(t *testing.T, inputs, state RobotsTxtArgs) infer.DiffResponse {
	t.Helper()
	resp, err := (&RobotsTxt{}).Diff(context.Background(), infer.DiffRequest[RobotsTxtArgs, RobotsTxtState]{
		Inputs: inputs,
		State:  RobotsTxtState{RobotsTxtArgs: state, LastModified: "2025-12-10T12:00:00Z"},
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	return resp
}

func TestRobotsTxt_Diff(t *testing.T) {
	tests := []struct {
		name       string
		inputs     RobotsTxtArgs
		state      RobotsTxtArgs
		wantChange bool
		wantField  string
		wantKind   p.DiffKind
		wantDBR    bool
	}{
		{
			name:   "identical",
			inputs: RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"},
			state:  RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"},
		},
		{
			name:   "normalized state vs raw input is not a change",
			inputs: RobotsTxtArgs{SiteID: testSiteID, Content: "user-agent: *\nallow: /\n# note\n"},
			state:  RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /\n"},
		},
		{
			name:       "content change",
			inputs:     RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /\nDisallow: /admin/"},
			state:      RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"},
			wantChange: true, wantField: "content", wantKind: p.Update,
		},
		{
			name:       "empty content is a change",
			inputs:     RobotsTxtArgs{SiteID: testSiteID, Content: ""},
			state:      RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"},
			wantChange: true, wantField: "content", wantKind: p.Update,
		},
		{
			name:       "siteId change replaces",
			inputs:     RobotsTxtArgs{SiteID: "ffffffffffffffffffffffff", Content: "User-agent: *\nAllow: /"},
			state:      RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"},
			wantChange: true, wantField: "siteId", wantKind: p.UpdateReplace, wantDBR: true,
		},
		{
			name:       "empty state is a change",
			inputs:     RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"},
			state:      RobotsTxtArgs{},
			wantChange: true, wantField: "siteId", wantKind: p.UpdateReplace, wantDBR: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := robotsDiff(t, tt.inputs, tt.state)
			if resp.HasChanges != tt.wantChange {
				t.Fatalf("HasChanges = %v, want %v (%+v)", resp.HasChanges, tt.wantChange, resp.DetailedDiff)
			}
			if !tt.wantChange {
				if len(resp.DetailedDiff) != 0 {
					t.Errorf("expected empty DetailedDiff, got %+v", resp.DetailedDiff)
				}
				return
			}
			if pd, ok := resp.DetailedDiff[tt.wantField]; !ok || pd.Kind != tt.wantKind {
				t.Errorf("expected %s with kind %v, got %+v", tt.wantField, tt.wantKind, resp.DetailedDiff)
			}
			if resp.DeleteBeforeReplace != tt.wantDBR {
				t.Errorf("DeleteBeforeReplace = %v, want %v", resp.DeleteBeforeReplace, tt.wantDBR)
			}
		})
	}
}

// ============================================================================
// Resource: Update / Delete
// ============================================================================

func TestRobotsTxt_Update_DryRun(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	resp, err := (&RobotsTxt{}).Update(context.Background(), infer.UpdateRequest[RobotsTxtArgs, RobotsTxtState]{
		ID:     testRobotsResourceID,
		Inputs: RobotsTxtArgs{SiteID: "", Content: "User-agent: *\nAllow: /\nDisallow: /new/"},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Update (DryRun) failed: %v", err)
	}
	if called {
		t.Error("API must not be called in DryRun mode")
	}
	if resp.Output.Content != "User-agent: *\nAllow: /\nDisallow: /new/" || resp.Output.LastModified == "" {
		t.Errorf("unexpected preview state: %+v", resp.Output)
	}
}

func TestRobotsTxt_Update_Success(t *testing.T) {
	var gotMethod string
	var gotBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, `{"rules":[{"userAgent":"*","allows":["/"],"disallows":["/new/"]}],"sitemap":""}`)
	})
	raw := "User-agent: *\nAllow: /\nDisallow: /new/"
	resp, err := (&RobotsTxt{}).Update(context.Background(), infer.UpdateRequest[RobotsTxtArgs, RobotsTxtState]{
		ID:     testRobotsResourceID,
		Inputs: RobotsTxtArgs{SiteID: testSiteID, Content: raw},
		State:  RobotsTxtState{RobotsTxtArgs: RobotsTxtArgs{SiteID: testSiteID, Content: "User-agent: *\nAllow: /"}},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	rules, _ := gotBody["rules"].([]any)
	if len(rules) != 1 || len(rules[0].(map[string]any)["disallows"].([]any)) != 1 {
		t.Errorf("unexpected PUT body: %v", gotBody)
	}
	if resp.Output.Content != raw {
		t.Errorf("state must keep the raw content, got %q", resp.Output.Content)
	}
}

func TestRobotsTxt_Update_ValidationErrors(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	for _, args := range []RobotsTxtArgs{{SiteID: "bad", Content: "User-agent: *"}, {SiteID: testSiteID, Content: ""}} {
		_, err := (&RobotsTxt{}).Update(context.Background(), infer.UpdateRequest[RobotsTxtArgs, RobotsTxtState]{
			ID: testRobotsResourceID, Inputs: args,
		})
		if err == nil || !containsStr(err.Error(), "validation failed") {
			t.Errorf("inputs %+v: expected validation error, got %v", args, err)
		}
	}
	if called {
		t.Error("API must not be called when validation fails")
	}
}

func TestRobotsTxt_Delete_SendsStateRules(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, `{"rules":[],"sitemap":""}`)
	})
	_, err := (&RobotsTxt{}).Delete(context.Background(), infer.DeleteRequest[RobotsTxtState]{
		ID:    testRobotsResourceID,
		State: RobotsTxtState{RobotsTxtArgs: RobotsTxtArgs{SiteID: testSiteID, Content: testRobotsContent}},
	})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v2/sites/"+testSiteID+"/robots_txt" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	rules, _ := gotBody["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("expected the rules from state in the DELETE body, got %v", gotBody)
	}
	rule := rules[0].(map[string]any)
	if rule["userAgent"] != "*" || len(rule["allows"].([]any)) != 1 || len(rule["disallows"].([]any)) != 1 ||
		gotBody["sitemap"] != "https://example.com/sitemap.xml" {
		t.Errorf("unexpected DELETE body: %v", gotBody)
	}
}

func TestRobotsTxt_Delete_NotFoundIsIdempotent(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, `{"message":"Not found"}`)
	})
	_, err := (&RobotsTxt{}).Delete(context.Background(), infer.DeleteRequest[RobotsTxtState]{ID: testRobotsResourceID})
	if err != nil {
		t.Errorf("404 on delete must be treated as success, got %v", err)
	}
}

func TestRobotsTxt_Delete_InvalidID(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	_, err := (&RobotsTxt{}).Delete(context.Background(), infer.DeleteRequest[RobotsTxtState]{ID: "../x/robots.txt"})
	if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
		t.Errorf("expected invalid resource ID error, got %v", err)
	}
	if called {
		t.Error("API must not be called with an invalid ID")
	}
}

// ============================================================================
// Network error guidance (transport failures wrapped by doRequest)
// ============================================================================

func TestNetworkErrorGuidance(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains []string
	}{
		{"timeout", errors.New("context deadline exceeded"), []string{"timeout", "fix this"}},
		{"connection refused", errors.New("connection refused"), []string{"connection failed", "DNS", "firewall"}},
		{"dns failure", errors.New("no such host"), []string{"connection failed", "DNS"}},
		{"generic", errors.New("network unreachable"), []string{"network error", "fix this"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &http.Client{Transport: &mockRoundTripper{
				handler: func(req *http.Request) (*http.Response, error) { return nil, tt.err },
			}}
			_, err := GetRobotsTxt(context.Background(), mockClient, testSiteID)
			if err == nil {
				t.Fatal("expected error")
			}
			for _, want := range tt.contains {
				if !containsStr(err.Error(), want) {
					t.Errorf("error %q should contain %q", err.Error(), want)
				}
			}
		})
	}
}

// TestHandleWebflowError checks the shared status-code messages used by every resource.
func TestHandleWebflowError(t *testing.T) {
	tests := []struct {
		status   int
		contains []string
	}{
		{400, []string{"bad request", "check"}},
		{401, []string{"unauthorized", "token"}},
		{403, []string{"forbidden", "permission"}},
		{404, []string{"not found", "verify"}},
		{429, []string{"rate limited", "wait"}},
		{500, []string{"server error", "retry"}},
		{502, []string{"502", "status.webflow.com"}},
		{418, []string{"418", "unexpected"}},
	}
	for _, tt := range tests {
		err := newAPIError(tt.status, "", "", []byte(`{"error":"x"}`))
		if err == nil {
			t.Fatalf("status %d: expected error", tt.status)
		}
		for _, want := range tt.contains {
			if !containsStr(err.Error(), want) {
				t.Errorf("status %d: error %q should contain %q", tt.status, err.Error(), want)
			}
		}
	}
	// A 404 from handleWebflowError is typed as well
	if !IsNotFound(newAPIError(404, "", "", nil)) {
		t.Error("newAPIError(404) must satisfy IsNotFound")
	}
}

func TestSecret_TokenMarkedAsSecret(t *testing.T) {
	configType := reflect.TypeOf(Config{})
	tokenField, exists := configType.FieldByName("APIToken")
	if !exists {
		t.Fatal("Config struct missing APIToken field")
	}
	if tokenField.Tag.Get("provider") != "secret" {
		t.Errorf("APIToken field missing provider:\"secret\" tag - got provider:%q", tokenField.Tag.Get("provider"))
	}
	if tokenField.Tag.Get("pulumi") == "" {
		t.Error("APIToken field missing pulumi tag")
	}
}
