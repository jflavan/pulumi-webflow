// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setTestAPIToken makes GetHTTPClient succeed in tests that call resource methods directly.
func setTestAPIToken(t *testing.T) {
	t.Helper()
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-abc123def456")
}

// mockAPI starts an httptest server for handler, points every API call at it for the
// duration of the test, sets the API token, and returns an authenticated client for
// tests that call API functions directly. Retries run with millisecond delays.
func mockAPI(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	setTestAPIToken(t)
	return useMockAPI(t, server)
}

// decodeJSONBody decodes a request body into out, reporting a test error on failure.
// It uses t.Errorf (not Fatalf) because handlers run on the server goroutine.
func decodeJSONBody(t *testing.T, r *http.Request, out any) {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("failed to read request body: %v", err)
		return
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Errorf("failed to decode request body %q: %v", raw, err)
	}
}

// writeJSON writes status and v as a JSON response.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("failed to encode response: %v", err)
	}
}

func TestCustomCodeScriptsEqual(t *testing.T) {
	nested := map[string]interface{}{
		"data-config": map[string]interface{}{"theme": "dark", "items": []interface{}{"a", "b"}},
	}
	nestedCopy := map[string]interface{}{
		"data-config": map[string]interface{}{"theme": "dark", "items": []interface{}{"a", "b"}},
	}
	nestedChanged := map[string]interface{}{
		"data-config": map[string]interface{}{"theme": "light", "items": []interface{}{"a", "b"}},
	}

	tests := []struct {
		name string
		a, b []CustomCodeScript
		want bool
	}{
		{"both empty", nil, []CustomCodeScript{}, true},
		{
			"identical",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}},
			true,
		},
		{
			"reordered",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}, {ID: "b", Version: "2.0.0", Location: "footer"}},
			[]CustomCodeScript{{ID: "b", Version: "2.0.0", Location: "footer"}, {ID: "a", Version: "1.0.0", Location: "header"}},
			true,
		},
		{
			"same id different location is a different script",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "footer"}},
			false,
		},
		{
			"same id in both locations matches regardless of order",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}, {ID: "a", Version: "1.0.0", Location: "footer"}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "footer"}, {ID: "a", Version: "1.0.0", Location: "header"}},
			true,
		},
		{
			"version changed",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.1", Location: "header"}},
			false,
		},
		{
			"length differs",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}, {ID: "b", Version: "1.0.0", Location: "header"}},
			false,
		},
		{
			"nil and empty attributes are equal",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header", Attributes: nil}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header", Attributes: map[string]interface{}{}}},
			true,
		},
		{
			"nested attributes equal (would panic with !=)",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header", Attributes: nested}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header", Attributes: nestedCopy}},
			true,
		},
		{
			"nested attributes differ",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header", Attributes: nested}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header", Attributes: nestedChanged}},
			false,
		},
		{
			"duplicate entries must match one-to-one",
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}, {ID: "a", Version: "1.0.0", Location: "header"}},
			[]CustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}, {ID: "a", Version: "2.0.0", Location: "header"}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := customCodeScriptsEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("customCodeScriptsEqual() = %v, want %v", got, tt.want)
			}
			if got := customCodeScriptsEqual(tt.b, tt.a); got != tt.want {
				t.Errorf("customCodeScriptsEqual() (reversed) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScriptListsEqual_WorksForBothInputTypes(t *testing.T) {
	attrs := map[string]interface{}{"data-x": map[string]interface{}{"k": "v"}}
	site := []CustomScriptArgs{
		{ID: "a", Version: "1.0.0", Location: "header", Attributes: attrs},
		{ID: "b", Version: "1.0.0", Location: "footer"},
	}
	siteReordered := []CustomScriptArgs{site[1], site[0]}
	if !scriptListsEqual(site, siteReordered) {
		t.Error("scriptListsEqual(CustomScriptArgs) should ignore order")
	}
	page := []PageCustomCodeScript{
		{ID: "a", Version: "1.0.0", Location: "header", Attributes: attrs},
		{ID: "b", Version: "1.0.0", Location: "footer"},
	}
	pageReordered := []PageCustomCodeScript{page[1], page[0]}
	if !scriptListsEqual(page, pageReordered) {
		t.Error("scriptListsEqual(PageCustomCodeScript) should ignore order")
	}
	pageChanged := []PageCustomCodeScript{page[0], {ID: "b", Version: "1.0.1", Location: "footer"}}
	if scriptListsEqual(page, pageChanged) {
		t.Error("scriptListsEqual(PageCustomCodeScript) should detect a version change")
	}
}

func TestToAndFromAPIScripts(t *testing.T) {
	in := []CustomScriptArgs{
		{ID: "a", Version: "1.0.0", Location: "header", Attributes: map[string]interface{}{"k": "v"}},
		{ID: "b", Version: "2.0.0", Location: "footer"},
	}
	api := toAPIScripts(in)
	if len(api) != 2 || api[0].ID != "a" || api[0].Attributes["k"] != "v" || api[1].Location != "footer" {
		t.Fatalf("toAPIScripts() = %+v", api)
	}
	// The conversion must copy attributes, not alias the caller's map.
	api[0].Attributes["k"] = "changed"
	if in[0].Attributes["k"] != "v" {
		t.Error("toAPIScripts() must not alias the input attributes map")
	}
	if api[1].Attributes != nil {
		t.Errorf("empty attributes should stay nil so they are omitted from JSON, got %v", api[1].Attributes)
	}

	back := fromAPIScripts[PageCustomCodeScript](api)
	if len(back) != 2 || back[0].ID != "a" || back[0].Attributes["k"] != "changed" || back[1].Version != "2.0.0" {
		t.Fatalf("fromAPIScripts() = %+v", back)
	}

	if got := toAPIScripts[CustomScriptArgs](nil); got == nil || len(got) != 0 {
		t.Errorf("toAPIScripts(nil) should return an empty non-nil slice, got %#v", got)
	}
	raw, _ := json.Marshal(CustomCodeRequest{Scripts: toAPIScripts[CustomScriptArgs](nil)})
	if string(raw) != `{"scripts":[]}` {
		t.Errorf("empty request body = %s, want {\"scripts\":[]}", raw)
	}
}
