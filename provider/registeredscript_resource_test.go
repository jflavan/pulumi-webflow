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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const testScriptID = "test-script-123"

// Short names for the infer request types used throughout these tests.
type (
	rsDiffRequest   = infer.DiffRequest[RegisteredScriptResourceArgs, RegisteredScriptResourceState]
	rsReadRequest   = infer.ReadRequest[RegisteredScriptResourceArgs, RegisteredScriptResourceState]
	rsUpdateRequest = infer.UpdateRequest[RegisteredScriptResourceArgs, RegisteredScriptResourceState]
)

// pagedScriptsHandler serves scripts in pages of pageSize, honouring the offset query parameter,
// and records the offsets requested.
func pagedScriptsHandler(t *testing.T, scripts []RegisteredScript, pageSize int, offsets *[]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testSiteID+"/registered_scripts" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		offset := 0
		if raw := r.URL.Query().Get("offset"); raw != "" {
			var err error
			if offset, err = strconv.Atoi(raw); err != nil {
				t.Errorf("bad offset %q", raw)
			}
		}
		*offsets = append(*offsets, offset)
		end := offset + pageSize
		if end > len(scripts) {
			end = len(scripts)
		}
		page := []RegisteredScript{}
		if offset < len(scripts) {
			page = scripts[offset:end]
		}
		writeJSON(t, w, http.StatusOK, RegisteredScriptsResponse{
			RegisteredScripts: page,
			Pagination:        PaginationInfo{Limit: pageSize, Offset: offset, Total: len(scripts)},
		})
	}
}

func manyScripts(n int) []RegisteredScript {
	scripts := make([]RegisteredScript, n)
	for i := range scripts {
		scripts[i] = RegisteredScript{
			ID: fmt.Sprintf("script%03d", i), DisplayName: fmt.Sprintf("Script%03d", i),
			HostedLocation: "https://cdn.example.com/s.js", IntegrityHash: "sha384-abc", Version: "1.0.0",
		}
	}
	return scripts
}

// TestRegisteredScriptCreate_ValidationErrors tests input validation in Create
func TestRegisteredScriptCreate_ValidationErrors(t *testing.T) {
	resource := &RegisteredScriptResource{}
	valid := RegisteredScriptResourceArgs{
		SiteID: testSiteID, DisplayName: "TestScript", HostedLocation: "https://example.com/script.js",
		IntegrityHash: "sha384-abc123", Version: "1.0.0",
	}
	tests := []struct {
		name   string
		modify func(a *RegisteredScriptResourceArgs)
		want   string
	}{
		{"invalid siteId", func(a *RegisteredScriptResourceArgs) { a.SiteID = "invalid" }, "validation failed"},
		{"missing displayName", func(a *RegisteredScriptResourceArgs) { a.DisplayName = "" }, "displayName is required"},
		{
			"displayName too long",
			func(a *RegisteredScriptResourceArgs) { a.DisplayName = strings.Repeat("a", 51) },
			"too long",
		},
		{
			"displayName with special chars",
			func(a *RegisteredScriptResourceArgs) { a.DisplayName = "Script-With-Dashes" },
			"invalid characters",
		},
		{
			"missing hostedLocation",
			func(a *RegisteredScriptResourceArgs) { a.HostedLocation = "" },
			"hostedLocation is required",
		},
		{
			"hostedLocation without https",
			func(a *RegisteredScriptResourceArgs) { a.HostedLocation = "ftp://example.com/script.js" },
			"must start with 'http://' or 'https://'",
		},
		{
			"missing integrityHash",
			func(a *RegisteredScriptResourceArgs) { a.IntegrityHash = "" },
			"integrityHash is required",
		},
		{
			"integrityHash invalid format",
			func(a *RegisteredScriptResourceArgs) { a.IntegrityHash = "md5-abc123" },
			"must start with 'sha'",
		},
		{"missing version", func(a *RegisteredScriptResourceArgs) { a.Version = "" }, "version is required"},
		{"invalid version format", func(a *RegisteredScriptResourceArgs) { a.Version = "1" }, "Semantic Version format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := valid
			tt.modify(&inputs)
			// No server and no token: validation must fail before any API call.
			resp, err := resource.Create(context.Background(), infer.CreateRequest[RegisteredScriptResourceArgs]{Inputs: inputs})
			if err == nil {
				t.Fatalf("Create() expected error, got nil")
			}
			if !containsStr(err.Error(), tt.want) {
				t.Errorf("Create() error = %v, want substring %q", err, tt.want)
			}
			if resp.ID != "" {
				t.Errorf("Create() returned ID when expecting error: %s", resp.ID)
			}
		})
	}
}

// TestRegisteredScriptCreate_DryRun tests dry-run behavior: no API call, an empty ID (so
// dependents see unknown) and no fabricated outputs.
func TestRegisteredScriptCreate_DryRun(t *testing.T) {
	mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected during preview") })
	resource := &RegisteredScriptResource{}
	inputs := RegisteredScriptResourceArgs{
		SiteID: testSiteID, DisplayName: "Test Script", HostedLocation: "https://example.com/script.js",
		IntegrityHash: "sha384-abc123", Version: "1.0.0", CanCopy: true,
	}
	resp, err := resource.Create(context.Background(), infer.CreateRequest[RegisteredScriptResourceArgs]{
		Inputs: inputs, DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create() dry-run failed: %v", err)
	}
	if resp.ID != "" {
		t.Errorf("Create() dry-run must return an empty ID, got %q", resp.ID)
	}
	if resp.Output.CreatedOn != "" || resp.Output.LastUpdated != "" || resp.Output.ScriptID != "" {
		t.Errorf("Create() dry-run must not fabricate outputs: %+v", resp.Output)
	}
	if resp.Output.RegisteredScriptResourceArgs != inputs {
		t.Errorf("Create() dry-run should echo inputs: %+v", resp.Output)
	}
}

// TestRegisteredScriptCreate_DryRun_DefersValidation verifies preview succeeds with unknown
// (zero-value) inputs, since validation happens at apply time.
func TestRegisteredScriptCreate_DryRun_DefersValidation(t *testing.T) {
	resource := &RegisteredScriptResource{}
	if _, err := resource.Create(context.Background(), infer.CreateRequest[RegisteredScriptResourceArgs]{
		Inputs: RegisteredScriptResourceArgs{}, DryRun: true,
	}); err != nil {
		t.Fatalf("Create() dry-run with unknown inputs should succeed: %v", err)
	}
}

// TestRegisteredScriptCheck verifies preview-time validation of known values only.
func TestRegisteredScriptCheck(t *testing.T) {
	resource := &RegisteredScriptResource{}
	valid := map[string]property.Value{
		"siteId":         property.New(testSiteID),
		"displayName":    property.New("CMS Slider"),
		"hostedLocation": property.New("https://cdn.example.com/slider.js"),
		"integrityHash":  property.New("sha384-abc123"),
		"scriptVersion":  property.New("1.0.0"),
	}
	with := func(overrides map[string]property.Value) property.Map {
		m := make(map[string]property.Value, len(valid))
		for k, v := range valid {
			m[k] = v
		}
		for k, v := range overrides {
			if v.IsNull() {
				delete(m, k) // a null override removes the property entirely
				continue
			}
			m[k] = v
		}
		return property.NewMap(m)
	}

	t.Run("valid inputs (display name with a space) pass", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{NewInputs: with(nil)})
		if err != nil || len(resp.Failures) != 0 {
			t.Fatalf("Check() = %+v, %v; want no failures", resp.Failures, err)
		}
		if resp.Inputs.DisplayName != "CMS Slider" || resp.Inputs.Version != "1.0.0" {
			t.Errorf("inputs not decoded: %+v", resp.Inputs)
		}
	})

	t.Run("unknown values are skipped", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{NewInputs: with(map[string]property.Value{
			"siteId":         property.New(property.Computed),
			"hostedLocation": property.New(property.Computed),
			"integrityHash":  property.New(property.Computed),
			"scriptVersion":  property.New(property.Computed),
		})})
		if err != nil || len(resp.Failures) != 0 {
			t.Fatalf("Check() with computed values = %+v, %v; want no failures", resp.Failures, err)
		}
	})

	tests := []struct {
		name     string
		override map[string]property.Value
		property string
		reason   string
	}{
		{"bad siteId", map[string]property.Value{"siteId": property.New("nope")}, "siteId", "invalid format"},
		{
			"bad displayName",
			map[string]property.Value{"displayName": property.New("a-b")},
			"displayName", "invalid characters",
		},
		{
			"bad hostedLocation",
			map[string]property.Value{"hostedLocation": property.New("ftp://x")},
			"hostedLocation", "http://",
		},
		{"bad integrityHash", map[string]property.Value{"integrityHash": property.New("md5-x")}, "integrityHash", "sha"},
		{"bad scriptVersion", map[string]property.Value{"scriptVersion": property.New("1")}, "scriptVersion", "Semantic"},
		{
			"missing scriptVersion (required by the schema)",
			map[string]property.Value{"scriptVersion": property.New(property.Null)},
			"scriptVersion", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resource.Check(context.Background(), infer.CheckRequest{NewInputs: with(tt.override)})
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			found := false
			for _, f := range resp.Failures {
				if f.Property == tt.property && strings.Contains(f.Reason, tt.reason) {
					found = true
				}
			}
			if !found {
				t.Errorf("Check() failures = %+v, want one on %q containing %q", resp.Failures, tt.property, tt.reason)
			}
		})
	}
}

func TestPostRegisteredScript_Success(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testSiteID+"/registered_scripts/hosted" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing Content-Type header")
		}
		var req RegisteredScriptRequest
		decodeJSONBody(t, r, &req)
		if req.DisplayName != "TestScript" || req.HostedLocation != "https://example.com/script.js" ||
			req.IntegrityHash != "sha384-abc123" || req.Version != "1.0.0" || !req.CanCopy {
			t.Errorf("unexpected request body: %+v", req)
		}
		writeJSON(t, w, http.StatusCreated, RegisteredScript{
			ID: testScriptID, DisplayName: req.DisplayName, HostedLocation: req.HostedLocation,
			IntegrityHash: req.IntegrityHash, Version: req.Version, CanCopy: req.CanCopy,
			CreatedOn: "2025-01-01T00:00:00Z", LastUpdated: "2025-01-01T00:00:00Z",
		})
	})

	resp, err := PostRegisteredScript(context.Background(), client, testSiteID, RegisteredScriptRequest{
		DisplayName: "TestScript", HostedLocation: "https://example.com/script.js",
		IntegrityHash: "sha384-abc123", Version: "1.0.0", CanCopy: true,
	})
	if err != nil {
		t.Fatalf("PostRegisteredScript() failed: %v", err)
	}
	if resp.ID != testScriptID || resp.DisplayName != "TestScript" || !resp.CanCopy {
		t.Errorf("PostRegisteredScript() = %+v", resp)
	}
}

func TestGetRegisteredScripts(t *testing.T) {
	var offsets []int
	client := mockAPI(t, pagedScriptsHandler(t, manyScripts(3), 2, &offsets))

	resp, err := GetRegisteredScripts(context.Background(), client, testSiteID, 0)
	if err != nil {
		t.Fatalf("GetRegisteredScripts() failed: %v", err)
	}
	if len(resp.RegisteredScripts) != 2 || resp.Pagination.Total != 3 || resp.RegisteredScripts[0].ID != "script000" {
		t.Errorf("GetRegisteredScripts(offset 0) = %+v", resp)
	}
	resp, err = GetRegisteredScripts(context.Background(), client, testSiteID, 2)
	if err != nil {
		t.Fatalf("GetRegisteredScripts(offset 2) failed: %v", err)
	}
	if len(resp.RegisteredScripts) != 1 || resp.RegisteredScripts[0].ID != "script002" {
		t.Errorf("GetRegisteredScripts(offset 2) = %+v", resp)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 2 {
		t.Errorf("requested offsets = %v, want [0 2]", offsets)
	}
}

func TestFindRegisteredScript_FollowsPagination(t *testing.T) {
	var offsets []int
	client := mockAPI(t, pagedScriptsHandler(t, manyScripts(250), 100, &offsets))

	script, err := FindRegisteredScript(context.Background(), client, testSiteID, "script225")
	if err != nil {
		t.Fatalf("FindRegisteredScript() failed: %v", err)
	}
	if script.ID != "script225" || script.DisplayName != "Script225" {
		t.Errorf("FindRegisteredScript() = %+v", script)
	}
	if len(offsets) != 3 || offsets[0] != 0 || offsets[1] != 100 || offsets[2] != 200 {
		t.Errorf("requested offsets = %v, want [0 100 200]", offsets)
	}
}

func TestFindRegisteredScript_StopsOnFirstMatch(t *testing.T) {
	var offsets []int
	client := mockAPI(t, pagedScriptsHandler(t, manyScripts(250), 100, &offsets))
	if _, err := FindRegisteredScript(context.Background(), client, testSiteID, "script005"); err != nil {
		t.Fatalf("FindRegisteredScript() failed: %v", err)
	}
	if len(offsets) != 1 {
		t.Errorf("expected a single page request, got offsets %v", offsets)
	}
}

func TestFindRegisteredScript_ExhaustedIsNotFound(t *testing.T) {
	var offsets []int
	client := mockAPI(t, pagedScriptsHandler(t, manyScripts(150), 100, &offsets))
	_, err := FindRegisteredScript(context.Background(), client, testSiteID, "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound error after exhausting pages, got %v", err)
	}
	if len(offsets) != 2 {
		t.Errorf("expected 2 page requests before giving up, got offsets %v", offsets)
	}
}

func TestFindRegisteredScript_EmptyListIsNotFound(t *testing.T) {
	calls := 0
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		writeJSON(t, w, http.StatusOK, RegisteredScriptsResponse{RegisteredScripts: []RegisteredScript{}})
	})
	if _, err := FindRegisteredScript(context.Background(), client, testSiteID, "x"); !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call for an empty list, got %d", calls)
	}
}

func TestFindRegisteredScript_SiteNotFound(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	if _, err := FindRegisteredScript(context.Background(), client, testSiteID, "x"); !IsNotFound(err) {
		t.Fatalf("expected IsNotFound for 404 site, got %v", err)
	}
}

func TestFindRegisteredScript_ServerErrorIsNotNotFound(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})
	_, err := FindRegisteredScript(context.Background(), client, testSiteID, "x")
	if err == nil || IsNotFound(err) {
		t.Fatalf("500 must be a plain error, got %v", err)
	}
}

// TestRegisteredScriptResourceID tests ID generation and extraction
func TestRegisteredScriptResourceID(t *testing.T) {
	resourceID := GenerateRegisteredScriptResourceID(testSiteID, testScriptID)
	if want := fmt.Sprintf("%s/registered_scripts/%s", testSiteID, testScriptID); resourceID != want {
		t.Errorf("GenerateRegisteredScriptResourceID() = %s, want %s", resourceID, want)
	}
	siteID, scriptID, err := ExtractIDsFromRegisteredScriptResourceID(resourceID)
	if err != nil {
		t.Fatalf("ExtractIDsFromRegisteredScriptResourceID() failed: %v", err)
	}
	if siteID != testSiteID || scriptID != testScriptID {
		t.Errorf("ExtractIDsFromRegisteredScriptResourceID() = %s, %s", siteID, scriptID)
	}
	// Script IDs containing slashes are preserved.
	_, scriptID, err = ExtractIDsFromRegisteredScriptResourceID(testSiteID + "/registered_scripts/a/b")
	if err != nil || scriptID != "a/b" {
		t.Errorf("script id with slash: %q, %v", scriptID, err)
	}
}

// TestRegisteredScriptResourceID_Invalid tests error handling for invalid IDs
func TestRegisteredScriptResourceID_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		inputID string
		wantErr string
	}{
		{"empty ID", "", "cannot be empty"},
		{"invalid format", "invalid-format", "invalid resource ID format"},
		{"wrong resource type", fmt.Sprintf("%s/webhooks/%s", testSiteID, testScriptID), "invalid resource ID format"},
		{"empty site id", "/registered_scripts/" + testScriptID, "invalid resource ID format"},
		{"empty script id", testSiteID + "/registered_scripts/", "invalid resource ID format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ExtractIDsFromRegisteredScriptResourceID(tt.inputID)
			if err == nil {
				t.Fatalf("ExtractIDsFromRegisteredScriptResourceID() expected error, got nil")
			}
			if !containsStr(err.Error(), tt.wantErr) {
				t.Errorf("ExtractIDsFromRegisteredScriptResourceID() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

// TestValidateScriptDisplayName tests display name validation
func TestValidateScriptDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid alphanumeric", "TestScript123", false},
		{"valid single char", "A", false},
		{"valid 50 chars", strings.Repeat("a", 50), false},
		{"documented example with a space", "CMS Slider", false},
		{"with spaces", "Test Script 2", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 51), true},
		{"with dashes", "Test-Script", true},
		{"with underscores", "Test_Script", true},
		{"non-ascii letters", "Café", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateScriptDisplayName(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateScriptDisplayName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateHostedLocation tests URL validation
func TestValidateHostedLocation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid https", "https://example.com/script.js", false},
		{"valid http", "http://example.com/script.js", false},
		{"empty", "", true},
		{"missing scheme", "example.com/script.js", true},
		{"ftp", "ftp://example.com/script.js", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateHostedLocation(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostedLocation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateIntegrityHash tests hash validation
func TestValidateIntegrityHash(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid sha384", "sha384-abc123def456", false},
		{"valid sha256", "sha256-abc123def456", false},
		{"valid sha512", "sha512-abc123def456", false},
		{"empty", "", true},
		{"md5", "md5-abc123", true},
		{"no algorithm", "abc123def456", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateIntegrityHash(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateIntegrityHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateVersion tests version validation
func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid semver", "1.0.0", false},
		{"valid semver minor", "2.3.1", false},
		{"valid semver patch", "0.0.1", false},
		{"valid two-part version", "1.0", false},
		{"empty", "", true},
		{"no dots", "1", true},
		{"no dots long", "version123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateVersion(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// =============================================================================
// RegisteredScript Diff Tests
// =============================================================================

// TestRegisteredScriptDiff_SameVersion_NoChange tests that Diff correctly
// reports NO changes when user input version matches state version.
func TestRegisteredScriptDiff_SameVersion_NoChange(t *testing.T) {
	resource := &RegisteredScriptResource{}
	args := RegisteredScriptResourceArgs{
		SiteID: "site123", DisplayName: "TestScript", HostedLocation: "https://cdn.example.com/script.js",
		IntegrityHash: "sha384-abc123", Version: "1.0.0", CanCopy: false,
	}
	diffResp, err := resource.Diff(context.Background(), rsDiffRequest{
		Inputs: args, State: RegisteredScriptResourceState{RegisteredScriptResourceArgs: args},
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if diffResp.HasChanges {
		t.Errorf("Diff() incorrectly detected changes when values are identical: %+v", diffResp.DetailedDiff)
	}
}

// TestRegisteredScriptDiff_EmptyStateVersion_NoChange tests that state written before
// scriptVersion existed does not force a replacement.
func TestRegisteredScriptDiff_EmptyStateVersion_NoChange(t *testing.T) {
	resource := &RegisteredScriptResource{}
	inputs := RegisteredScriptResourceArgs{
		SiteID: "site123", DisplayName: "TestScript", HostedLocation: "https://cdn.example.com/script.js",
		IntegrityHash: "sha384-abc123", Version: "1.0.0",
	}
	state := inputs
	state.Version = ""
	diffResp, err := resource.Diff(context.Background(), rsDiffRequest{
		Inputs: inputs, State: RegisteredScriptResourceState{RegisteredScriptResourceArgs: state},
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if diffResp.HasChanges {
		t.Errorf("Diff() should not flag a change for an empty state version: %+v", diffResp.DetailedDiff)
	}
}

// TestRegisteredScriptDiff_ChangesRequireReplacement tests that all property changes
// trigger UpdateReplace since Webflow API doesn't support PATCH for registered scripts.
// Delete cannot unregister the old script, so the replacement is create-before-delete.
func TestRegisteredScriptDiff_ChangesRequireReplacement(t *testing.T) {
	resource := &RegisteredScriptResource{}
	baseInputs := RegisteredScriptResourceArgs{
		SiteID: "site123", DisplayName: "TestScript", HostedLocation: "https://cdn.example.com/script.js",
		IntegrityHash: "sha384-abc123", Version: "1.0.0", CanCopy: false,
	}
	baseState := RegisteredScriptResourceState{RegisteredScriptResourceArgs: baseInputs}

	tests := []struct {
		name      string
		modifyFn  func(args *RegisteredScriptResourceArgs)
		fieldName string
	}{
		{"siteId change", func(a *RegisteredScriptResourceArgs) { a.SiteID = "site456" }, "siteId"},
		{"displayName change", func(a *RegisteredScriptResourceArgs) { a.DisplayName = "NewScriptName" }, "displayName"},
		{
			"hostedLocation change",
			func(a *RegisteredScriptResourceArgs) { a.HostedLocation = "https://cdn.example.com/script-v2.js" },
			"hostedLocation",
		},
		{
			"integrityHash change",
			func(a *RegisteredScriptResourceArgs) { a.IntegrityHash = "sha384-def456" },
			"integrityHash",
		},
		{"version change", func(a *RegisteredScriptResourceArgs) { a.Version = "2.0.0" }, "scriptVersion"},
		{"canCopy change", func(a *RegisteredScriptResourceArgs) { a.CanCopy = true }, "canCopy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifiedInputs := baseInputs
			tt.modifyFn(&modifiedInputs)
			diffResp, err := resource.Diff(context.Background(), rsDiffRequest{
				Inputs: modifiedInputs, State: baseState,
			})
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if !diffResp.HasChanges {
				t.Errorf("Diff() should detect a replacement for %s: %+v", tt.fieldName, diffResp)
			}
			if diffResp.DeleteBeforeReplace {
				t.Errorf("Diff() must not delete first: Delete is a no-op and a failed registration would drop state")
			}
			if d, ok := diffResp.DetailedDiff[tt.fieldName]; !ok || d.Kind != p.UpdateReplace {
				t.Errorf("Diff() DetailedDiff[%s] = %+v (present=%v), want UpdateReplace", tt.fieldName, d, ok)
			}
		})
	}
}

// TestRegisteredScriptUpdate_ReturnsError tests that Update method returns an error
// since Webflow API doesn't support PATCH for registered scripts.
func TestRegisteredScriptUpdate_ReturnsError(t *testing.T) {
	resource := &RegisteredScriptResource{}
	_, err := resource.Update(context.Background(), rsUpdateRequest{
		ID: "site123/registered_scripts/script456",
	})
	if err == nil {
		t.Fatal("Update() should return an error")
	}
	if !containsStr(err.Error(), "cannot be updated in-place") || !containsStr(err.Error(), "PATCH") {
		t.Errorf("Update() error should mention in-place updates and PATCH not supported, got: %v", err)
	}
}

// =============================================================================
// RegisteredScript resource-level CRUD tests
// =============================================================================

func TestRegisteredScriptCreate(t *testing.T) {
	var got RegisteredScriptRequest
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testSiteID+"/registered_scripts/hosted" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		decodeJSONBody(t, r, &got)
		writeJSON(t, w, http.StatusCreated, RegisteredScript{
			ID: "testscript", DisplayName: got.DisplayName, HostedLocation: got.HostedLocation,
			IntegrityHash: got.IntegrityHash, Version: got.Version, CanCopy: got.CanCopy,
			CreatedOn: "2025-01-01T00:00:00Z", LastUpdated: "2025-01-01T00:00:00Z",
		})
	})

	resource := &RegisteredScriptResource{}
	resp, err := resource.Create(context.Background(), infer.CreateRequest[RegisteredScriptResourceArgs]{
		Inputs: RegisteredScriptResourceArgs{
			SiteID: testSiteID, DisplayName: "TestScript", HostedLocation: "https://example.com/script.js",
			IntegrityHash: "sha384-abc123", Version: "1.2.3", CanCopy: true,
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.ID != testSiteID+"/registered_scripts/testscript" || resp.Output.ScriptID != "testscript" {
		t.Errorf("Create() = ID %q, ScriptID %q", resp.ID, resp.Output.ScriptID)
	}
	if resp.Output.CreatedOn != "2025-01-01T00:00:00Z" {
		t.Errorf("Create() CreatedOn = %q", resp.Output.CreatedOn)
	}
	if got.DisplayName != "TestScript" || got.HostedLocation != "https://example.com/script.js" ||
		got.IntegrityHash != "sha384-abc123" || got.Version != "1.2.3" || !got.CanCopy {
		t.Errorf("request body = %+v", got)
	}
}

func TestRegisteredScriptCreate_EmptyIDFromAPI(t *testing.T) {
	mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusCreated, RegisteredScript{DisplayName: "TestScript"})
	})
	resource := &RegisteredScriptResource{}
	_, err := resource.Create(context.Background(), infer.CreateRequest[RegisteredScriptResourceArgs]{
		Inputs: RegisteredScriptResourceArgs{
			SiteID: testSiteID, DisplayName: "TestScript", HostedLocation: "https://example.com/script.js",
			IntegrityHash: "sha384-abc123", Version: "1.0.0",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "empty registered script ID") {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestRegisteredScriptRead(t *testing.T) {
	resource := &RegisteredScriptResource{}
	scripts := manyScripts(150)
	scripts[120].CanCopy = true
	scripts[120].CreatedOn = "2025-01-01T00:00:00Z"
	readReq := rsReadRequest{
		ID: GenerateRegisteredScriptResourceID(testSiteID, "script120"),
	}

	t.Run("finds the script on a later page", func(t *testing.T) {
		var offsets []int
		mockAPI(t, pagedScriptsHandler(t, scripts, 100, &offsets))
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.ID != readReq.ID || resp.State.ScriptID != "script120" || !resp.State.CanCopy ||
			resp.Inputs.DisplayName != "Script120" || resp.State.Version != "1.0.0" || resp.State.CreatedOn == "" {
			t.Errorf("Read() = %+v", resp)
		}
		if len(offsets) != 2 {
			t.Errorf("expected pagination to be followed, offsets = %v", offsets)
		}
	})

	t.Run("script missing signals deletion", func(t *testing.T) {
		var offsets []int
		mockAPI(t, pagedScriptsHandler(t, scripts[:10], 100, &offsets))
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil || resp.ID != "" {
			t.Fatalf("Read() = (%q, %v), want empty ID and nil error", resp.ID, err)
		}
	})

	t.Run("site 404 signals deletion", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil || resp.ID != "" {
			t.Fatalf("Read() = (%q, %v), want empty ID and nil error", resp.ID, err)
		}
	})

	t.Run("500 is an error", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		if _, err := resource.Read(context.Background(), readReq); err == nil {
			t.Fatal("Read() should return an error for 500")
		}
	})

	t.Run("version falls back to inputs when the API omits it", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, RegisteredScriptsResponse{
				RegisteredScripts: []RegisteredScript{{
					ID: "script120", DisplayName: "Script120", HostedLocation: "https://x.example.com/s.js", IntegrityHash: "sha384-x",
				}},
				Pagination: PaginationInfo{Limit: 100, Offset: 0, Total: 1},
			})
		})
		req := readReq
		req.Inputs = RegisteredScriptResourceArgs{Version: "3.2.1"}
		resp, err := resource.Read(context.Background(), req)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.State.Version != "3.2.1" || resp.Inputs.Version != "3.2.1" {
			t.Errorf("Read() version = %q/%q, want 3.2.1", resp.State.Version, resp.Inputs.Version)
		}
	})

	t.Run("import: omitted version stays empty and does not force a replace", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, RegisteredScriptsResponse{
				RegisteredScripts: []RegisteredScript{{
					ID: "script120", DisplayName: "Script120", HostedLocation: "https://x.example.com/s.js", IntegrityHash: "sha384-x",
				}},
				Pagination: PaginationInfo{Limit: 100, Offset: 0, Total: 1},
			})
		})
		// Import: no inputs and no state.
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.State.Version != "" || resp.Inputs.Version != "" {
			t.Errorf("Read() must not fabricate a version, got %q/%q", resp.State.Version, resp.Inputs.Version)
		}
		program := resp.Inputs
		program.Version = "2.0.0"
		diffResp, err := resource.Diff(context.Background(), rsDiffRequest{Inputs: program, State: resp.State})
		if err != nil || diffResp.HasChanges {
			t.Errorf("Diff() after import = %+v, %v; the unknown version must not force a replace", diffResp.DetailedDiff, err)
		}
	})

	t.Run("invalid site id in resource id", func(t *testing.T) {
		mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected") })
		_, err := resource.Read(context.Background(), rsReadRequest{
			ID: "site123/registered_scripts/x",
		})
		if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
			t.Fatalf("Read() error = %v", err)
		}
	})
}

// TestRegisteredScriptDelete verifies Delete is a no-op: Webflow has no endpoint to
// unregister a script, so no HTTP request may be made.
func TestRegisteredScriptDelete(t *testing.T) {
	calls := 0
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		t.Errorf("unexpected request %s %s: there is no unregister endpoint", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	resource := &RegisteredScriptResource{}
	if _, err := resource.Delete(context.Background(), infer.DeleteRequest[RegisteredScriptResourceState]{
		ID: GenerateRegisteredScriptResourceID(testSiteID, testScriptID),
	}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := resource.Delete(context.Background(), infer.DeleteRequest[RegisteredScriptResourceState]{
		ID: "not-a-resource-id",
	}); err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
		t.Errorf("Delete() should reject a malformed resource ID, got %v", err)
	}
	if calls != 0 {
		t.Errorf("Delete() must not call the API, got %d calls", calls)
	}
}
