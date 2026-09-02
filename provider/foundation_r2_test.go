// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestRetryTransport_DoesNotRetryPostOnGatewayError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	_, err := doRequest(context.Background(), client, http.MethodPost, server.URL+"/v2/sites", map[string]string{"a": "b"}, nil)
	if err == nil {
		t.Fatal("expected an error from the 502")
	}
	if calls != 1 {
		t.Fatalf("a POST must not be retried after a 502 (could duplicate the create); got %d attempts", calls)
	}
}

func TestRetryTransport_RetriesPutAndDeleteOnGatewayError(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				if calls == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()
			client := useMockAPI(t, server)

			var body any
			if method == http.MethodPut {
				body = map[string]string{"a": "b"}
			}
			if _, err := doRequest(context.Background(), client, method, server.URL+"/v2/x", body, nil); err != nil {
				t.Fatalf("%s should succeed after one 503: %v", method, err)
			}
			if calls != 2 {
				t.Fatalf("expected 2 attempts for %s, got %d", method, calls)
			}
		})
	}
}

func TestRetryTransport_StillRetriesPostOnRateLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)
	if _, err := doRequest(context.Background(), client, http.MethodPost, server.URL+"/v2/x", map[string]int{"n": 1}, nil); err != nil {
		t.Fatalf("POST after 429 should be retried: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}

func TestCalculateDelay_ClampsHugeRetryAfter(t *testing.T) {
	rt := &retryTransport{maxDelay: 10 * time.Second, baseDelay: time.Second}
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"99999999999999"}}}
	if d := rt.calculateDelay(resp, 0); d != 10*time.Second {
		t.Fatalf("expected clamp to maxDelay, got %v", d)
	}
	resp.Header.Set("Retry-After", "3")
	if d := rt.calculateDelay(resp, 0); d != 3*time.Second {
		t.Fatalf("expected 3s, got %v", d)
	}
}

func TestTruncateForLogging_RuneSafe(t *testing.T) {
	s := strings.Repeat("é", 300) // 2 bytes each
	out := TruncateForLogging(s, 5)
	if !utf8.ValidString(out) {
		t.Fatalf("truncation produced invalid UTF-8: %q", out[:8])
	}
	if !strings.HasPrefix(out, "éé...") {
		t.Fatalf("unexpected truncation: %q", out[:12])
	}
	if got := TruncateForLogging("abc", 0); !strings.Contains(got, "truncated") {
		t.Fatalf("maxLen 0 must not panic and must report truncation, got %q", got)
	}
}

// TestConfigure_ConfigTokenBeatsEnvironment drives the real provider through the integration
// harness: the stack configures webflow:apiToken while WEBFLOW_API_TOKEN holds a different
// value, and the Authorization header the mock API receives must carry the configured token.
func TestConfigure_ConfigTokenBeatsEnvironment(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "env-token-should-lose-0000000000")
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()
	old := apiBaseURLOverride
	apiBaseURLOverride = server.URL
	t.Cleanup(func() { apiBaseURLOverride = old })
	useFastRetries(t)

	srv, err := integration.NewServer(context.Background(), Name, semver.MustParse("0.0.1"),
		integration.WithProvider(Provider()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Configure(p.ConfigureRequest{
		Args: property.NewMap(map[string]property.Value{
			"apiToken": property.New("config-token-should-win-000000000"),
		}),
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	urn := resource.NewURN("stack", "proj", "", tokens.Type(Name+":index:RobotsTxt"), "r")
	// A Read on a 404 returns an empty ID without error; the request is what we care about.
	if _, err := srv.Read(p.ReadRequest{
		Urn: urn,
		ID:  "5f0c8c9e1c9d440000e8d8c3/robots.txt",
		Properties: property.NewMap(map[string]property.Value{
			"siteId":  property.New("5f0c8c9e1c9d440000e8d8c3"),
			"content": property.New("User-agent: *\nAllow: /\n"),
		}),
	}); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if gotAuth != "Bearer config-token-should-win-000000000" {
		t.Fatalf("expected the configured token to win over the environment, got %q", gotAuth)
	}
}
