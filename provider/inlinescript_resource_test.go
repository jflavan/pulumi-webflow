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
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const testInlineScriptID = "test-inline-script-123"

func validInlineScriptArgs() InlineScriptArgs {
	return InlineScriptArgs{
		SiteID: testSiteID, SourceCode: "console.log('hello');", Version: "1.0.0", DisplayName: "TestScript",
	}
}

// TestInlineScriptCreate_ValidationErrors tests input validation in Create
func TestInlineScriptCreate_ValidationErrors(t *testing.T) {
	resource := &InlineScript{}
	tests := []struct {
		name   string
		modify func(a *InlineScriptArgs)
		want   string
	}{
		{"invalid siteId", func(a *InlineScriptArgs) { a.SiteID = "invalid" }, "validation failed"},
		{"missing sourceCode", func(a *InlineScriptArgs) { a.SourceCode = "" }, "sourceCode is required"},
		{"sourceCode too long", func(a *InlineScriptArgs) { a.SourceCode = strings.Repeat("a", 2001) }, "too long"},
		{"missing version", func(a *InlineScriptArgs) { a.Version = "" }, "version is required"},
		{"invalid version format", func(a *InlineScriptArgs) { a.Version = "1" }, "Semantic Version format"},
		{"missing displayName", func(a *InlineScriptArgs) { a.DisplayName = "" }, "displayName is required"},
		{"displayName too long", func(a *InlineScriptArgs) { a.DisplayName = strings.Repeat("a", 51) }, "too long"},
		{
			"displayName with special chars",
			func(a *InlineScriptArgs) { a.DisplayName = "Script-With-Dashes" },
			"invalid characters",
		},
		{
			"invalid integrityHash format",
			func(a *InlineScriptArgs) { a.IntegrityHash = "md5-abc123" },
			"must start with 'sha'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := validInlineScriptArgs()
			tt.modify(&inputs)
			// No server and no token: validation must fail before any API call.
			resp, err := resource.Create(context.Background(), infer.CreateRequest[InlineScriptArgs]{Inputs: inputs})
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

// TestInlineScriptCreate_DryRun tests dry-run behavior, including that an optional
// integrityHash may be set or omitted and that validation is deferred to apply time.
func TestInlineScriptCreate_DryRun(t *testing.T) {
	resource := &InlineScript{}
	withHash := validInlineScriptArgs()
	withHash.IntegrityHash = "sha384-abc123"
	withHash.CanCopy = true
	for name, inputs := range map[string]InlineScriptArgs{
		"valid":                validInlineScriptArgs(),
		"with integrityHash":   withHash,
		"unknown inputs":       {},
		"invalid site (defer)": {SiteID: "bad"},
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := resource.Create(context.Background(),
				infer.CreateRequest[InlineScriptArgs]{Inputs: inputs, DryRun: true})
			if err != nil {
				t.Fatalf("Create() dry-run failed: %v", err)
			}
			if resp.ID == "" || resp.Output.CreatedOn == "" || resp.Output.LastUpdated == "" {
				t.Errorf("Create() dry-run should return ID and timestamps: %+v", resp)
			}
		})
	}
}

// TestPostInlineScript_Success tests successful creation via API
func TestPostInlineScript_Success(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testSiteID+"/registered_scripts/inline" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing Content-Type header")
		}
		var req InlineScriptRequest
		decodeJSONBody(t, r, &req)
		if req.SourceCode != "console.log('hello');" || req.DisplayName != "TestScript" || req.Version != "1.0.0" ||
			!req.CanCopy || req.IntegrityHash != "sha384-abc123" {
			t.Errorf("unexpected request body: %+v", req)
		}
		writeJSON(t, w, http.StatusCreated, InlineScriptResponse{
			ID: testInlineScriptID, DisplayName: req.DisplayName, SourceCode: req.SourceCode,
			HostedLocation: "https://cdn.webflow.com/inline/test-script.js", IntegrityHash: req.IntegrityHash,
			Version: req.Version, CanCopy: req.CanCopy, CreatedOn: "2025-01-01T00:00:00Z", LastUpdated: "2025-01-01T00:00:00Z",
		})
	})

	resp, err := PostInlineScript(context.Background(), client, testSiteID, InlineScriptRequest{
		SourceCode: "console.log('hello');", Version: "1.0.0", DisplayName: "TestScript",
		CanCopy: true, IntegrityHash: "sha384-abc123",
	})
	if err != nil {
		t.Fatalf("PostInlineScript() failed: %v", err)
	}
	if resp.ID != testInlineScriptID || resp.DisplayName != "TestScript" || !resp.CanCopy || resp.HostedLocation == "" {
		t.Errorf("PostInlineScript() = %+v", resp)
	}
}

// TestPostInlineScript_RateLimit tests that a 429 is retried by the shared transport.
func TestPostInlineScript_RateLimit(t *testing.T) {
	attempt := 0
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		attempt++
		if attempt == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, http.StatusCreated, InlineScriptResponse{ID: testInlineScriptID})
	})
	resp, err := PostInlineScript(context.Background(), client, testSiteID,
		InlineScriptRequest{SourceCode: "x", Version: "1.0.0", DisplayName: "T"})
	if err != nil {
		t.Fatalf("PostInlineScript() should retry on rate limit: %v", err)
	}
	if resp.ID != testInlineScriptID || attempt != 2 {
		t.Errorf("PostInlineScript() = %+v after %d attempts", resp, attempt)
	}
}

// TestPostInlineScript_ServerError tests server error handling
func TestPostInlineScript_ServerError(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	})
	if _, err := PostInlineScript(context.Background(), client, testSiteID, InlineScriptRequest{}); err == nil {
		t.Fatal("PostInlineScript() should fail on server error")
	}
}

// TestInlineScriptResourceID tests ID generation and extraction
func TestInlineScriptResourceID(t *testing.T) {
	resourceID := GenerateInlineScriptResourceID(testSiteID, testInlineScriptID)
	if want := fmt.Sprintf("%s/inline_scripts/%s", testSiteID, testInlineScriptID); resourceID != want {
		t.Errorf("GenerateInlineScriptResourceID() = %s, want %s", resourceID, want)
	}
	siteID, scriptID, err := ExtractIDsFromInlineScriptResourceID(resourceID)
	if err != nil {
		t.Fatalf("ExtractIDsFromInlineScriptResourceID() failed: %v", err)
	}
	if siteID != testSiteID || scriptID != testInlineScriptID {
		t.Errorf("ExtractIDsFromInlineScriptResourceID() = %s, %s", siteID, scriptID)
	}
}

// TestInlineScriptResourceID_Invalid tests error handling for invalid IDs
func TestInlineScriptResourceID_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		inputID string
		wantErr string
	}{
		{"empty ID", "", "cannot be empty"},
		{"invalid format", "invalid-format", "invalid resource ID format"},
		{"wrong resource type", fmt.Sprintf("%s/webhooks/%s", testSiteID, testInlineScriptID), "invalid resource ID format"},
		{"empty site id", "/inline_scripts/" + testInlineScriptID, "invalid resource ID format"},
		{"empty script id", testSiteID + "/inline_scripts/", "invalid resource ID format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ExtractIDsFromInlineScriptResourceID(tt.inputID)
			if err == nil {
				t.Fatalf("ExtractIDsFromInlineScriptResourceID() expected error, got nil")
			}
			if !containsStr(err.Error(), tt.wantErr) {
				t.Errorf("ExtractIDsFromInlineScriptResourceID() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

// TestValidateSourceCode tests source code validation
func TestValidateSourceCode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid short code", "console.log('hello');", false},
		{"valid at max length", strings.Repeat("a", 2000), false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 2001), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateSourceCode(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("ValidateSourceCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// =============================================================================
// InlineScript Diff Tests
// =============================================================================

func TestInlineScriptDiff_NoChange(t *testing.T) {
	resource := &InlineScript{}
	args := InlineScriptArgs{
		SiteID: "site123", SourceCode: "console.log('hello');", Version: "1.0.0", DisplayName: "TestScript",
	}
	withVersion := func(version string) InlineScriptArgs {
		a := args
		a.Version = version
		return a
	}
	withHash := func(hash string) InlineScriptArgs {
		a := args
		a.IntegrityHash = hash
		return a
	}

	tests := []struct {
		name   string
		inputs InlineScriptArgs
		state  InlineScriptArgs
	}{
		{"identical", args, args},
		{"empty state version (pre-scriptVersion state)", args, withVersion("")},
		{"omitted integrityHash ignores the registered hash", args, withHash("sha384-fromapi")},
		{"configured integrityHash matches state", withHash("sha384-abc"), withHash("sha384-abc")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffResp, err := resource.Diff(context.Background(), infer.DiffRequest[InlineScriptArgs, InlineScriptState]{
				Inputs: tt.inputs, State: InlineScriptState{InlineScriptArgs: tt.state},
			})
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if diffResp.HasChanges {
				t.Errorf("Diff() incorrectly detected changes: %+v", diffResp.DetailedDiff)
			}
		})
	}
}

// TestInlineScriptDiff_ChangesRequireReplacement tests that all property changes
// trigger UpdateReplace since Webflow API doesn't support PATCH for inline scripts.
func TestInlineScriptDiff_ChangesRequireReplacement(t *testing.T) {
	resource := &InlineScript{}
	baseInputs := InlineScriptArgs{
		SiteID: "site123", SourceCode: "console.log('hello');", Version: "1.0.0", DisplayName: "TestScript",
		CanCopy: false, IntegrityHash: "sha384-abc123",
	}
	baseState := InlineScriptState{InlineScriptArgs: baseInputs}

	tests := []struct {
		name      string
		modifyFn  func(args *InlineScriptArgs)
		fieldName string
	}{
		{"siteId change", func(a *InlineScriptArgs) { a.SiteID = "site456" }, "siteId"},
		{"sourceCode change", func(a *InlineScriptArgs) { a.SourceCode = "console.log('world');" }, "sourceCode"},
		{"displayName change", func(a *InlineScriptArgs) { a.DisplayName = "NewScriptName" }, "displayName"},
		{"integrityHash change", func(a *InlineScriptArgs) { a.IntegrityHash = "sha384-def456" }, "integrityHash"},
		{"version change", func(a *InlineScriptArgs) { a.Version = "2.0.0" }, "scriptVersion"},
		{"canCopy change", func(a *InlineScriptArgs) { a.CanCopy = true }, "canCopy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifiedInputs := baseInputs
			tt.modifyFn(&modifiedInputs)
			diffResp, err := resource.Diff(context.Background(), infer.DiffRequest[InlineScriptArgs, InlineScriptState]{
				Inputs: modifiedInputs, State: baseState,
			})
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if !diffResp.HasChanges || !diffResp.DeleteBeforeReplace {
				t.Errorf("Diff() should detect a replacement for %s: %+v", tt.fieldName, diffResp)
			}
			if d, ok := diffResp.DetailedDiff[tt.fieldName]; !ok || d.Kind != p.UpdateReplace {
				t.Errorf("Diff() DetailedDiff[%s] = %+v (present=%v), want UpdateReplace", tt.fieldName, d, ok)
			}
		})
	}
}

// TestInlineScriptUpdate_ReturnsError tests that Update method returns an error
// since Webflow API doesn't support PATCH for inline scripts.
func TestInlineScriptUpdate_ReturnsError(t *testing.T) {
	resource := &InlineScript{}
	_, err := resource.Update(context.Background(), infer.UpdateRequest[InlineScriptArgs, InlineScriptState]{
		ID: "site123/inline_scripts/script456",
	})
	if err == nil {
		t.Fatal("Update() should return an error")
	}
	if !containsStr(err.Error(), "cannot be updated in-place") || !containsStr(err.Error(), "PATCH") {
		t.Errorf("Update() error should mention in-place updates and PATCH not supported, got: %v", err)
	}
}

// =============================================================================
// InlineScript resource-level CRUD tests
// =============================================================================

func TestInlineScriptCreate(t *testing.T) {
	var got InlineScriptRequest
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testSiteID+"/registered_scripts/inline" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		decodeJSONBody(t, r, &got)
		writeJSON(t, w, http.StatusCreated, InlineScriptResponse{
			ID: "testscript", DisplayName: got.DisplayName, SourceCode: got.SourceCode, Version: got.Version,
			HostedLocation: "https://cdn.webflow.com/inline/testscript.js", IntegrityHash: "sha384-computed",
			CreatedOn: "2025-01-01T00:00:00Z", LastUpdated: "2025-01-01T00:00:00Z",
		})
	})

	resource := &InlineScript{}
	resp, err := resource.Create(context.Background(),
		infer.CreateRequest[InlineScriptArgs]{Inputs: validInlineScriptArgs()})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.ID != testSiteID+"/inline_scripts/testscript" || resp.Output.ScriptID != "testscript" ||
		resp.Output.HostedLocation == "" || resp.Output.CreatedOn != "2025-01-01T00:00:00Z" {
		t.Errorf("Create() = %+v", resp)
	}
	if resp.Output.IntegrityHash != "sha384-computed" {
		t.Errorf("Create() should record the hash Webflow reports in state, got %q", resp.Output.IntegrityHash)
	}
	if got.SourceCode != "console.log('hello');" || got.DisplayName != "TestScript" ||
		got.Version != "1.0.0" || got.IntegrityHash != "" {
		t.Errorf("request body = %+v", got)
	}
}

func TestInlineScriptCreate_EmptyIDFromAPI(t *testing.T) {
	mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusCreated, InlineScriptResponse{DisplayName: "TestScript"})
	})
	resource := &InlineScript{}
	_, err := resource.Create(context.Background(),
		infer.CreateRequest[InlineScriptArgs]{Inputs: validInlineScriptArgs()})
	if err == nil || !strings.Contains(err.Error(), "empty inline script ID") {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestInlineScriptRead(t *testing.T) {
	resource := &InlineScript{}
	scripts := manyScripts(120)
	scripts[110] = RegisteredScript{
		ID: "inline110", DisplayName: "Inline110", HostedLocation: "https://cdn.webflow.com/inline/inline110.js",
		IntegrityHash: "sha384-registered", Version: "1.0.0", CanCopy: true, CreatedOn: "2025-01-01T00:00:00Z",
	}
	resourceID := GenerateInlineScriptResourceID(testSiteID, "inline110")

	t.Run("finds the script on a later page and preserves an omitted integrityHash", func(t *testing.T) {
		var offsets []int
		mockAPI(t, pagedScriptsHandler(t, scripts, 100, &offsets))
		program := InlineScriptArgs{
			SiteID: testSiteID, SourceCode: "console.log('x');", Version: "1.0.0", DisplayName: "Inline110",
		}
		resp, err := resource.Read(context.Background(), infer.ReadRequest[InlineScriptArgs, InlineScriptState]{
			ID:     resourceID,
			Inputs: program,
			State:  InlineScriptState{InlineScriptArgs: program, ScriptID: "inline110"},
		})
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if len(offsets) != 2 {
			t.Errorf("expected pagination to be followed, offsets = %v", offsets)
		}
		if resp.ID != resourceID || resp.State.ScriptID != "inline110" || !resp.State.CanCopy || resp.State.CreatedOn == "" {
			t.Errorf("Read() state = %+v", resp.State)
		}
		if resp.State.SourceCode != "console.log('x');" {
			t.Errorf("Read() should preserve sourceCode the list endpoint does not return, got %q", resp.State.SourceCode)
		}
		if resp.Inputs.IntegrityHash != "" {
			t.Errorf("Read() must not copy the API integrityHash into inputs the user did not set, got %q",
				resp.Inputs.IntegrityHash)
		}
		if resp.State.IntegrityHash != "sha384-registered" {
			t.Errorf("Read() state should carry the registered integrityHash, got %q", resp.State.IntegrityHash)
		}
		// The refreshed state must not produce a diff against the unchanged program.
		program.CanCopy = true
		diffResp, err := resource.Diff(context.Background(), infer.DiffRequest[InlineScriptArgs, InlineScriptState]{
			Inputs: program,
			State:  resp.State,
		})
		if err != nil || diffResp.HasChanges {
			t.Errorf("Diff() after Read() = %+v, %v; want no changes", diffResp.DetailedDiff, err)
		}
	})

	t.Run("configured integrityHash reflects the registered value", func(t *testing.T) {
		var offsets []int
		mockAPI(t, pagedScriptsHandler(t, scripts, 100, &offsets))
		resp, err := resource.Read(context.Background(), infer.ReadRequest[InlineScriptArgs, InlineScriptState]{
			ID:     resourceID,
			Inputs: InlineScriptArgs{SiteID: testSiteID, IntegrityHash: "sha384-configured"},
		})
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.Inputs.IntegrityHash != "sha384-registered" || resp.State.IntegrityHash != "sha384-registered" {
			t.Errorf("Read() integrityHash = %q/%q, want registered value", resp.Inputs.IntegrityHash, resp.State.IntegrityHash)
		}
	})

	t.Run("integrityHash omitted by the API keeps the recorded value", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, RegisteredScriptsResponse{
				RegisteredScripts: []RegisteredScript{{ID: "inline110", DisplayName: "Inline110", Version: "1.0.0"}},
				Pagination:        PaginationInfo{Limit: 100, Total: 1},
			})
		})
		resp, err := resource.Read(context.Background(), infer.ReadRequest[InlineScriptArgs, InlineScriptState]{
			ID:    resourceID,
			State: InlineScriptState{InlineScriptArgs: InlineScriptArgs{IntegrityHash: "sha384-previous"}},
		})
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.State.IntegrityHash != "sha384-previous" || resp.Inputs.IntegrityHash != "" {
			t.Errorf("Read() integrityHash = state %q / inputs %q", resp.State.IntegrityHash, resp.Inputs.IntegrityHash)
		}
	})

	t.Run("script missing signals deletion", func(t *testing.T) {
		var offsets []int
		mockAPI(t, pagedScriptsHandler(t, scripts[:5], 100, &offsets))
		resp, err := resource.Read(context.Background(),
			infer.ReadRequest[InlineScriptArgs, InlineScriptState]{ID: resourceID})
		if err != nil || resp.ID != "" {
			t.Fatalf("Read() = (%q, %v), want empty ID and nil error", resp.ID, err)
		}
	})

	t.Run("500 is an error", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		_, err := resource.Read(context.Background(),
			infer.ReadRequest[InlineScriptArgs, InlineScriptState]{ID: resourceID})
		if err == nil {
			t.Fatal("Read() should return an error for 500")
		}
	})

	t.Run("invalid site id in resource id", func(t *testing.T) {
		mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected") })
		_, err := resource.Read(context.Background(),
			infer.ReadRequest[InlineScriptArgs, InlineScriptState]{ID: "site123/inline_scripts/x"})
		if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
			t.Fatalf("Read() error = %v", err)
		}
	})
}

func TestInlineScriptDelete(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
				wantPath := "/v2/sites/" + testSiteID + "/registered_scripts/" + testInlineScriptID
				if r.Method != http.MethodDelete || r.URL.Path != wantPath {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(status)
			})
			resource := &InlineScript{}
			if _, err := resource.Delete(context.Background(), infer.DeleteRequest[InlineScriptState]{
				ID: GenerateInlineScriptResourceID(testSiteID, testInlineScriptID),
			}); err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
		})
	}
	t.Run("invalid site id", func(t *testing.T) {
		mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected") })
		resource := &InlineScript{}
		_, err := resource.Delete(context.Background(), infer.DeleteRequest[InlineScriptState]{ID: "bad/inline_scripts/x"})
		if err == nil {
			t.Error("Delete() should reject an invalid site id before calling the API")
		}
	})
}
