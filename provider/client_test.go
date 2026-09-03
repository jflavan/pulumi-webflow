// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// useFastRetries makes retry paths run in milliseconds for the duration of a test.
func useFastRetries(t *testing.T) {
	t.Helper()
	oldBase, oldMax := retryBaseDelay, retryMaxDelay
	retryBaseDelay = time.Millisecond
	retryMaxDelay = 5 * time.Millisecond
	t.Cleanup(func() { retryBaseDelay, retryMaxDelay = oldBase, oldMax })
}

// useMockAPI points every API call at server and returns an authenticated client.
func useMockAPI(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	useFastRetries(t)
	old := apiBaseURLOverride
	apiBaseURLOverride = server.URL
	t.Cleanup(func() { apiBaseURLOverride = old })
	client, err := CreateHTTPClient("test-token-abc123def456", "test")
	if err != nil {
		t.Fatalf("CreateHTTPClient: %v", err)
	}
	return client
}

func TestRetryTransport_ResendsBodyAfterRateLimit(t *testing.T) {
	var bodies []string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		calls++
		if calls <= 2 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"Too Many Requests","code":"too_many_requests"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	var out struct {
		ID string `json:"id"`
	}
	status, err := doRequest(context.Background(), client, http.MethodPost, server.URL+"/v2/things",
		map[string]string{"hello": "world"}, &out)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	if status != http.StatusCreated || out.ID != "abc" {
		t.Fatalf("unexpected result status=%d out=%+v", status, out)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
	for i, b := range bodies {
		if b != `{"hello":"world"}` {
			t.Errorf("attempt %d body = %q, want full JSON body", i+1, b)
		}
	}
}

func TestRetryTransport_RetriesTransientServerErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	if _, err := doRequest(context.Background(), client, http.MethodGet, server.URL+"/v2/x", nil, nil); err != nil {
		t.Fatalf("expected success after 502 retry, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}

func TestRetryTransport_GivesUpAfterMaxRetries(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	_, err := doRequest(context.Background(), client, http.MethodGet, server.URL+"/v2/x", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected APIError 429, got %v", err)
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error should mention rate limiting: %v", err)
	}
	if calls != DefaultMaxRetries+1 {
		t.Fatalf("expected %d attempts, got %d", DefaultMaxRetries+1, calls)
	}
}

func TestDoRequest_NotFoundIsTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Requested resource not found"}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	_, err := doRequest(context.Background(), client, http.MethodGet, server.URL+"/v2/sites/x", nil, nil)
	if !IsNotFound(err) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// A 500 whose body happens to say "not found" must NOT be treated as not found.
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"upstream not found"}`))
	}))
	defer server2.Close()
	_, err = doRequest(context.Background(), client, http.MethodGet, server2.URL+"/v2/sites/x", nil, nil)
	if err == nil || IsNotFound(err) {
		t.Fatalf("500 must not satisfy IsNotFound: %v", err)
	}
}

func TestDoRequest_TruncatesLongErrorBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", 10000)))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	_, err := doRequest(context.Background(), client, http.MethodGet, server.URL+"/v2/x", nil, nil)
	if err == nil || len(err.Error()) > 1000 {
		t.Fatalf("expected truncated error, got len=%d", len(err.Error()))
	}
}

func TestDoDelete_TreatsNotFoundAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := useMockAPI(t, server)
	if err := doDelete(context.Background(), client, server.URL+"/v2/x", nil); err != nil {
		t.Fatalf("expected idempotent delete, got %v", err)
	}
}

func TestDoRequest_SendsHeaders(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)
	_, err := doRequest(context.Background(), client, http.MethodPut, server.URL+"/v2/x", map[string]int{"a": 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("Content-Type") != "application/json" || got.Get("Accept-Version") != "2.0.0" ||
		!strings.HasPrefix(got.Get("Authorization"), "Bearer ") ||
		!strings.HasPrefix(got.Get("User-Agent"), "pulumi-webflow/") {
		t.Errorf("missing headers: %v", got)
	}
}

func TestResolveToken_ConfigBeatsEnvironment(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "env-token-abc123def456")
	if got := resolveToken(context.Background()); got != "env-token-abc123def456" {
		t.Fatalf("expected env fallback, got %q", got)
	}
	// Config precedence is exercised through the provider harness in config_test.go.
}
