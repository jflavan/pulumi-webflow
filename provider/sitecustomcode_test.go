// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestGenerateSiteCustomCodeResourceID(t *testing.T) {
	siteID := "5f0c8c9e1c9d440000e8d8c3"
	expected := "5f0c8c9e1c9d440000e8d8c3/custom_code"

	result := GenerateSiteCustomCodeResourceID(siteID)
	if result != expected {
		t.Errorf("GenerateSiteCustomCodeResourceID() = %v, want %v", result, expected)
	}
}

func TestExtractSiteIDFromSiteCustomCodeResourceID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr bool
	}{
		{"valid", "5f0c8c9e1c9d440000e8d8c3/custom_code", "5f0c8c9e1c9d440000e8d8c3", false},
		{"empty", "", "", true},
		{"invalid suffix", "5f0c8c9e1c9d440000e8d8c3/robots.txt", "", true},
		{"missing suffix", "5f0c8c9e1c9d440000e8d8c3", "", true},
		{"empty site id", "/custom_code", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractSiteIDFromSiteCustomCodeResourceID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractSiteIDFromSiteCustomCodeResourceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.wantID {
				t.Errorf("ExtractSiteIDFromSiteCustomCodeResourceID() = %v, want %v", result, tt.wantID)
			}
		})
	}
}

func TestGetSiteCustomCode(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v2/sites/"+testSiteID+"/custom_code" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{
			Scripts: []CustomCodeScript{{
				ID: "cms_slider", Location: "header", Version: "1.0.0",
				Attributes: map[string]interface{}{"data-config": "my-value"},
			}},
			LastUpdated: "2025-01-03T00:00:00Z",
			CreatedOn:   "2025-01-03T00:00:00Z",
		})
	})

	resp, err := GetSiteCustomCode(context.Background(), client, testSiteID)
	if err != nil {
		t.Fatalf("GetSiteCustomCode() error = %v", err)
	}
	if len(resp.Scripts) != 1 || resp.Scripts[0].ID != "cms_slider" || resp.Scripts[0].Location != "header" {
		t.Errorf("unexpected scripts: %+v", resp.Scripts)
	}
	if resp.Scripts[0].Attributes["data-config"] != "my-value" {
		t.Errorf("attributes not decoded: %v", resp.Scripts[0].Attributes)
	}
}

func TestGetSiteCustomCode_NotFoundIsTyped(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := GetSiteCustomCode(context.Background(), client, testSiteID)
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound error, got %v", err)
	}
}

func TestPutSiteCustomCode(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v2/sites/"+testSiteID+"/custom_code" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var req CustomCodeRequest
		decodeJSONBody(t, r, &req)
		if len(req.Scripts) != 1 || req.Scripts[0].ID != "cms_slider" || req.Scripts[0].Version != "1.0.0" ||
			req.Scripts[0].Location != "header" || req.Scripts[0].Attributes["data-config"] != "x" {
			t.Errorf("unexpected request body: %+v", req)
		}
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{
			Scripts:     req.Scripts,
			LastUpdated: "2025-01-03T00:00:00Z",
			CreatedOn:   "2025-01-03T00:00:00Z",
		})
	})

	resp, err := PutSiteCustomCode(context.Background(), client, testSiteID, []CustomCodeScript{{
		ID: "cms_slider", Location: "header", Version: "1.0.0",
		Attributes: map[string]interface{}{"data-config": "x"},
	}})
	if err != nil {
		t.Fatalf("PutSiteCustomCode() error = %v", err)
	}
	if len(resp.Scripts) != 1 {
		t.Errorf("Expected 1 script in response, got %d", len(resp.Scripts))
	}
}

func TestPutSiteCustomCode_EmptyListSendsEmptyArray(t *testing.T) {
	var body string
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		body = strings.TrimSpace(string(raw))
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{Scripts: []CustomCodeScript{}})
	})
	if _, err := PutSiteCustomCode(context.Background(), client, testSiteID, nil); err != nil {
		t.Fatalf("PutSiteCustomCode() error = %v", err)
	}
	if body != `{"scripts":[]}` {
		t.Errorf("request body = %s, want {\"scripts\":[]}", body)
	}
}

func TestDeleteSiteCustomCode(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("Expected DELETE, got %s", r.Method)
				}
				w.WriteHeader(status)
			})
			if err := DeleteSiteCustomCode(context.Background(), client, testSiteID); err != nil {
				t.Fatalf("DeleteSiteCustomCode() error = %v", err)
			}
		})
	}
}

func TestDeleteSiteCustomCode_ServerError(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if err := DeleteSiteCustomCode(context.Background(), client, testSiteID); err == nil {
		t.Fatal("expected error for 500")
	}
}

// TestSiteCustomCodeCreate_DryRun_WithUnknownScriptIDs verifies that preview succeeds
// when script IDs are unknown (empty strings from the infer framework), makes no API
// call, returns an empty ID and fabricates no timestamps.
func TestSiteCustomCodeCreate_DryRun_WithUnknownScriptIDs(t *testing.T) {
	mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected during preview") })
	resource := &SiteCustomCode{}
	req := infer.CreateRequest[SiteCustomCodeArgs]{
		Inputs: SiteCustomCodeArgs{
			SiteID:  "",
			Scripts: []CustomScriptArgs{{ID: "", Version: "", Location: ""}},
		},
		DryRun: true,
	}

	resp, err := resource.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create() DryRun with unknown inputs should succeed, got error: %v", err)
	}
	if resp.ID != "" {
		t.Errorf("Create() DryRun must return an empty ID, got %q", resp.ID)
	}
	if resp.Output.LastUpdated != "" || resp.Output.CreatedOn != "" {
		t.Errorf("Create() DryRun must not fabricate timestamps, got %+v", resp.Output)
	}
	if len(resp.Output.Scripts) != 1 {
		t.Errorf("Create() DryRun should echo inputs, got %+v", resp.Output)
	}
}

// TestSiteCustomCodeUpdate_DryRun_WithUnknownScriptIDs verifies that preview succeeds
// for updates when script IDs are unknown and that recorded timestamps are carried over
// rather than fabricated.
func TestSiteCustomCodeUpdate_DryRun_WithUnknownScriptIDs(t *testing.T) {
	mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected during preview") })
	resource := &SiteCustomCode{}
	req := infer.UpdateRequest[SiteCustomCodeArgs, SiteCustomCodeState]{
		ID:     testSiteID + "/custom_code",
		Inputs: SiteCustomCodeArgs{SiteID: testSiteID, Scripts: []CustomScriptArgs{{}}},
		State: SiteCustomCodeState{
			SiteCustomCodeArgs: SiteCustomCodeArgs{
				SiteID: testSiteID, Scripts: []CustomScriptArgs{{ID: "old_script", Version: "1.0.0", Location: "header"}},
			},
			LastUpdated: "2025-02-01T00:00:00Z", CreatedOn: "2025-01-01T00:00:00Z",
		},
		DryRun: true,
	}

	resp, err := resource.Update(context.Background(), req)
	if err != nil {
		t.Fatalf("Update() DryRun with unknown inputs should succeed, got error: %v", err)
	}
	if resp.Output.LastUpdated != "2025-02-01T00:00:00Z" || resp.Output.CreatedOn != "2025-01-01T00:00:00Z" {
		t.Errorf("Update() DryRun must carry over recorded timestamps, got %+v", resp.Output)
	}
}

// TestSiteCustomCodeCheck verifies preview-time validation of known values, skipping
// computed script entries and fields.
func TestSiteCustomCodeCheck(t *testing.T) {
	resource := &SiteCustomCode{}
	script := func(id, version, location property.Value) property.Value {
		return property.New(property.NewMap(map[string]property.Value{
			"id": id, "scriptVersion": version, "location": location,
		}))
	}
	known := script(property.New("cms_slider"), property.New("1.0.0"), property.New("header"))
	computed := property.New(property.Computed)
	check := func(t *testing.T, inputs map[string]property.Value) infer.CheckResponse[SiteCustomCodeArgs] {
		t.Helper()
		resp, err := resource.Check(context.Background(), infer.CheckRequest{NewInputs: property.NewMap(inputs)})
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		return resp
	}

	t.Run("valid inputs pass", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"siteId":  property.New(testSiteID),
			"scripts": property.New([]property.Value{known}),
		})
		if len(resp.Failures) != 0 {
			t.Fatalf("Check() = %+v; want no failures", resp.Failures)
		}
		if resp.Inputs.SiteID != testSiteID || len(resp.Inputs.Scripts) != 1 || resp.Inputs.Scripts[0].ID != "cms_slider" {
			t.Errorf("inputs not decoded: %+v", resp.Inputs)
		}
	})

	t.Run("computed values are skipped", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"siteId": computed,
			"scripts": property.New([]property.Value{
				computed, // whole entry unknown
				script(computed, property.New("1.0.0"), property.New("footer")), // id from a RegisteredScript output
				known,
			}),
		})
		if len(resp.Failures) != 0 {
			t.Fatalf("Check() with computed values = %+v; want no failures", resp.Failures)
		}
	})

	t.Run("computed scripts list is skipped", func(t *testing.T) {
		resp := check(t, map[string]property.Value{"siteId": property.New(testSiteID), "scripts": computed})
		if len(resp.Failures) != 0 {
			t.Fatalf("Check() with a computed list = %+v; want no failures", resp.Failures)
		}
	})

	t.Run("known bad values fail", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"siteId": property.New("bad"),
			"scripts": property.New([]property.Value{
				script(computed, property.New("1.0.0"), property.New("body")),
				script(property.New(""), property.New(""), property.New("footer")),
			}),
		})
		want := []struct{ property, reason string }{
			{"siteId", "invalid format"},
			{"scripts", "scripts[0].location"},
			{"scripts", "scripts[1].id"},
			{"scripts", "scripts[1].scriptVersion"},
		}
		for _, w := range want {
			found := false
			for _, f := range resp.Failures {
				if f.Property == w.property && strings.Contains(f.Reason, w.reason) {
					found = true
				}
			}
			if !found {
				t.Errorf("Check() failures = %+v, want one on %q containing %q", resp.Failures, w.property, w.reason)
			}
		}
		if len(resp.Failures) != len(want) {
			t.Errorf("Check() returned %d failures, want %d: %+v", len(resp.Failures), len(want), resp.Failures)
		}
	})
}

func TestSiteCustomCodeCreate_ValidationErrors(t *testing.T) {
	resource := &SiteCustomCode{}
	valid := CustomScriptArgs{ID: "cms_slider", Version: "1.0.0", Location: "header"}
	tests := []struct {
		name   string
		inputs SiteCustomCodeArgs
		want   string
	}{
		{"invalid siteId", SiteCustomCodeArgs{SiteID: "bad", Scripts: []CustomScriptArgs{valid}}, "validation failed"},
		{
			"missing script id",
			SiteCustomCodeArgs{SiteID: testSiteID, Scripts: []CustomScriptArgs{{Version: "1.0.0", Location: "header"}}},
			"scripts[0]",
		},
		{
			"bad location",
			SiteCustomCodeArgs{SiteID: testSiteID, Scripts: []CustomScriptArgs{{ID: "x", Version: "1.0.0", Location: "body"}}},
			"header",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No server: validation must fail before any API call.
			_, err := resource.Create(context.Background(), infer.CreateRequest[SiteCustomCodeArgs]{Inputs: tt.inputs})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Create() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestSiteCustomCodeDiff(t *testing.T) {
	resource := &SiteCustomCode{}
	nested := func(theme string) map[string]interface{} {
		return map[string]interface{}{"data-config": map[string]interface{}{"theme": theme, "items": []interface{}{"a", 1.0}}}
	}
	scriptA := CustomScriptArgs{ID: "a", Version: "1.0.0", Location: "header", Attributes: nested("dark")}
	scriptB := CustomScriptArgs{ID: "b", Version: "2.0.0", Location: "footer"}
	args := func(scripts ...CustomScriptArgs) SiteCustomCodeArgs {
		return SiteCustomCodeArgs{SiteID: testSiteID, Scripts: scripts}
	}

	tests := []struct {
		name        string
		state       SiteCustomCodeArgs
		inputs      SiteCustomCodeArgs
		wantChanges bool
		wantKey     string
		wantKind    p.DiffKind
	}{
		{
			name:   "no change",
			state:  args(scriptA, scriptB),
			inputs: args(scriptA, scriptB),
		},
		{
			name:   "reordered scripts are not a change",
			state:  args(scriptA, scriptB),
			inputs: args(scriptB, scriptA),
		},
		{
			name:   "nil and empty attributes are not a change",
			state:  args(CustomScriptArgs{ID: "b", Version: "2.0.0", Location: "footer", Attributes: map[string]interface{}{}}),
			inputs: args(scriptB),
		},
		{
			name:        "nested attribute value changed",
			state:       args(scriptA),
			inputs:      args(CustomScriptArgs{ID: "a", Version: "1.0.0", Location: "header", Attributes: nested("light")}),
			wantChanges: true, wantKey: "scripts", wantKind: p.Update,
		},
		{
			name:        "script version changed",
			state:       args(scriptB),
			inputs:      args(CustomScriptArgs{ID: "b", Version: "2.0.1", Location: "footer"}),
			wantChanges: true, wantKey: "scripts", wantKind: p.Update,
		},
		{
			name:        "script moved to another location",
			state:       args(scriptB),
			inputs:      args(CustomScriptArgs{ID: "b", Version: "2.0.0", Location: "header"}),
			wantChanges: true, wantKey: "scripts", wantKind: p.Update,
		},
		{
			name:        "script removed",
			state:       args(scriptA, scriptB),
			inputs:      args(scriptA),
			wantChanges: true, wantKey: "scripts", wantKind: p.Update,
		},
		{
			name:        "siteId changed requires replacement",
			state:       args(scriptA),
			inputs:      SiteCustomCodeArgs{SiteID: "5f0c8c9e1c9d440000e8d8c4", Scripts: []CustomScriptArgs{scriptA}},
			wantChanges: true, wantKey: "siteId", wantKind: p.UpdateReplace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resource.Diff(context.Background(), infer.DiffRequest[SiteCustomCodeArgs, SiteCustomCodeState]{
				ID:     GenerateSiteCustomCodeResourceID(tt.state.SiteID),
				Inputs: tt.inputs,
				State:  SiteCustomCodeState{SiteCustomCodeArgs: tt.state},
			})
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if resp.HasChanges != tt.wantChanges {
				t.Fatalf("Diff() HasChanges = %v, want %v (detail: %+v)", resp.HasChanges, tt.wantChanges, resp.DetailedDiff)
			}
			if tt.wantChanges {
				d, ok := resp.DetailedDiff[tt.wantKey]
				if !ok || d.Kind != tt.wantKind {
					t.Errorf("Diff() DetailedDiff[%s] = %+v (present=%v), want kind %v", tt.wantKey, d, ok, tt.wantKind)
				}
			}
		})
	}
}

func TestSiteCustomCodeCreate(t *testing.T) {
	var got CustomCodeRequest
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v2/sites/"+testSiteID+"/custom_code" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		decodeJSONBody(t, r, &got)
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{
			Scripts: got.Scripts, LastUpdated: "2025-02-01T00:00:00Z", CreatedOn: "2025-01-01T00:00:00Z",
		})
	})

	resource := &SiteCustomCode{}
	inputs := SiteCustomCodeArgs{SiteID: testSiteID, Scripts: []CustomScriptArgs{
		{ID: "cms_slider", Version: "1.0.0", Location: "header", Attributes: map[string]interface{}{"data-a": "1"}},
	}}
	resp, err := resource.Create(context.Background(), infer.CreateRequest[SiteCustomCodeArgs]{Inputs: inputs})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.ID != testSiteID+"/custom_code" {
		t.Errorf("Create() ID = %s", resp.ID)
	}
	if resp.Output.LastUpdated != "2025-02-01T00:00:00Z" || resp.Output.CreatedOn != "2025-01-01T00:00:00Z" {
		t.Errorf("Create() timestamps not taken from response: %+v", resp.Output)
	}
	if len(got.Scripts) != 1 || got.Scripts[0].ID != "cms_slider" || got.Scripts[0].Attributes["data-a"] != "1" {
		t.Errorf("request body = %+v", got)
	}
}

func TestSiteCustomCodeRead(t *testing.T) {
	resource := &SiteCustomCode{}
	readReq := infer.ReadRequest[SiteCustomCodeArgs, SiteCustomCodeState]{ID: testSiteID + "/custom_code"}

	t.Run("reads scripts back from the API", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testSiteID+"/custom_code" {
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			writeJSON(t, w, http.StatusOK, CustomCodeResponse{
				Scripts: []CustomCodeScript{
					{ID: "a", Version: "1.0.0", Location: "header", Attributes: map[string]interface{}{"k": "v"}},
					{ID: "b", Version: "2.0.0", Location: "footer"},
				},
				LastUpdated: "2025-02-01T00:00:00Z", CreatedOn: "2025-01-01T00:00:00Z",
			})
		})
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.ID != readReq.ID || resp.Inputs.SiteID != testSiteID {
			t.Errorf("Read() ID/siteId = %s/%s", resp.ID, resp.Inputs.SiteID)
		}
		if len(resp.State.Scripts) != 2 || resp.State.Scripts[0].Attributes["k"] != "v" || resp.Inputs.Scripts[1].ID != "b" {
			t.Errorf("Read() scripts = %+v", resp.State.Scripts)
		}
		if resp.State.LastUpdated != "2025-02-01T00:00:00Z" {
			t.Errorf("Read() LastUpdated = %s", resp.State.LastUpdated)
		}
	})

	t.Run("404 signals deletion", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil || resp.ID != "" {
			t.Fatalf("Read() = (%q, %v), want empty ID and nil error", resp.ID, err)
		}
	})

	t.Run("500 is an error, not a deletion", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"not found in cache"}`))
		})
		_, err := resource.Read(context.Background(), readReq)
		if err == nil {
			t.Fatal("Read() should return an error for 500")
		}
	})

	t.Run("invalid site id in resource id", func(t *testing.T) {
		mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected") })
		_, err := resource.Read(context.Background(),
			infer.ReadRequest[SiteCustomCodeArgs, SiteCustomCodeState]{ID: "not-hex/custom_code"})
		if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
			t.Fatalf("Read() error = %v", err)
		}
	})
}

func TestSiteCustomCodeUpdate(t *testing.T) {
	var got CustomCodeRequest
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method %s", r.Method)
		}
		decodeJSONBody(t, r, &got)
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{
			Scripts: got.Scripts, LastUpdated: "2025-03-01T00:00:00Z", CreatedOn: "2025-01-01T00:00:00Z",
		})
	})

	resource := &SiteCustomCode{}
	resp, err := resource.Update(context.Background(), infer.UpdateRequest[SiteCustomCodeArgs, SiteCustomCodeState]{
		ID: testSiteID + "/custom_code",
		Inputs: SiteCustomCodeArgs{
			SiteID:  testSiteID,
			Scripts: []CustomScriptArgs{{ID: "b", Version: "2.0.0", Location: "footer"}},
		},
		State: SiteCustomCodeState{
			SiteCustomCodeArgs: SiteCustomCodeArgs{
				SiteID:  testSiteID,
				Scripts: []CustomScriptArgs{{ID: "a", Version: "1.0.0", Location: "header"}},
			},
			CreatedOn: "2025-01-01T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(got.Scripts) != 1 || got.Scripts[0].ID != "b" {
		t.Errorf("Update() sent %+v, want only script b", got.Scripts)
	}
	if resp.Output.LastUpdated != "2025-03-01T00:00:00Z" || resp.Output.CreatedOn != "2025-01-01T00:00:00Z" {
		t.Errorf("Update() output = %+v", resp.Output)
	}
}

func TestSiteCustomCodeDelete(t *testing.T) {
	calls := 0
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodDelete || r.URL.Path != "/v2/sites/"+testSiteID+"/custom_code" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	resource := &SiteCustomCode{}
	_, err := resource.Delete(context.Background(),
		infer.DeleteRequest[SiteCustomCodeState]{ID: testSiteID + "/custom_code"})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 API call, got %d", calls)
	}
	_, err = resource.Delete(context.Background(), infer.DeleteRequest[SiteCustomCodeState]{ID: "bad/custom_code"})
	if err == nil {
		t.Error("Delete() should reject an invalid site id before calling the API")
	}
	if calls != 1 {
		t.Errorf("invalid ID must not reach the API, got %d calls", calls)
	}
}

func TestSiteCustomCodeCreate_RetriesRateLimit(t *testing.T) {
	calls := 0
	mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{})
	})
	resource := &SiteCustomCode{}
	_, err := resource.Create(context.Background(), infer.CreateRequest[SiteCustomCodeArgs]{
		Inputs: SiteCustomCodeArgs{
			SiteID:  testSiteID,
			Scripts: []CustomScriptArgs{{ID: "a", Version: "1.0.0", Location: "header"}},
		},
	})
	if err != nil {
		t.Fatalf("Create() should succeed after a 429 retry: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 attempts, got %d", calls)
	}
}
