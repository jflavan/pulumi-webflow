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
	"strings"
	"testing"
)

// Well-known IDs used by the Site, Redirect and RobotsTxt tests.
const (
	testSiteID      = "5f0c8c9e1c9d440000e8d8c3"
	testOtherSiteID = "6f1d9d0f2d0e551111f9e9d4"
	testWorkspaceID = "7a2b3c4d5e6f708192a3b4c5"
	testToken       = "test-token-abc123def456"
)

// containsStr checks if string contains substring (case-insensitive).
func containsStr(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// mockWebflowAPI starts an httptest server, points every API call at it, sets the token in
// the environment so resource methods can build a client, and returns the server.
// Retries run with millisecond delays for the duration of the test.
func mockWebflowAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	useMockAPI(t, server)
	t.Setenv("WEBFLOW_API_TOKEN", testToken)
	return server
}

// unreachableAPI points every API call at a closed port so transport errors surface quickly.
func unreachableAPI(t *testing.T) {
	t.Helper()
	useFastRetries(t)
	old := apiBaseURLOverride
	apiBaseURLOverride = "http://127.0.0.1:1"
	t.Cleanup(func() { apiBaseURLOverride = old })
	t.Setenv("WEBFLOW_API_TOKEN", testToken)
}

// readBody returns the raw request body.
func readBody(t *testing.T, r *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	return body
}

// readJSONBody decodes the request body into a generic map for assertions on exact fields.
func readJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	body := readBody(t, r)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("request body is not a JSON object: %v: %s", err, string(body))
	}
	return out
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if s, ok := v.(string); ok {
		_, _ = w.Write([]byte(s))
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encoding response: %v", err)
	}
}
