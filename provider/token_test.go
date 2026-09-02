// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// tokenServer serves both token endpoints; status controls the response of every request.
func tokenServer(t *testing.T, status, rateLimitFirst *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if rateLimitFirst != nil && *rateLimitFirst > 0 {
			*rateLimitFirst--
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"Too Many Requests"}`))
			return
		}
		if *status != http.StatusOK {
			w.WriteHeader(*status)
			_, _ = w.Write([]byte("error details"))
			return
		}
		switch r.URL.Path {
		case "/v2/token/introspect":
			_ = json.NewEncoder(w).Encode(TokenIntrospectResponse{
				Authorization: Authorization{
					ID: "auth123", CreatedOn: "2024-01-01T00:00:00Z", LastUsed: "2024-06-15T12:00:00Z",
					GrantType: "authorization_code", RateLimit: 60, Scope: "sites:read sites:write",
					AuthorizedTo: AuthorizedTo{SiteIDs: []string{"site1", "site2"}, WorkspaceIDs: []string{"ws1"}},
				},
				Application: Application{
					ID:          "app123",
					Description: "Test App",
					Homepage:    "https://example.com",
					DisplayName: "My Test App",
				},
			})
		case "/v2/token/authorized_by":
			_ = json.NewEncoder(w).Encode(AuthorizedByResponse{
				ID: "user123", Email: "test@example.com", FirstName: "John", LastName: "Doe",
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGetTokenIntrospect(t *testing.T) {
	status := http.StatusOK
	client := useMockAPI(t, tokenServer(t, &status, nil))

	result, err := GetTokenIntrospect(context.Background(), client)
	if err != nil {
		t.Fatalf("GetTokenIntrospect: %v", err)
	}
	if result.Authorization.ID != "auth123" || result.Authorization.RateLimit != 60 ||
		len(result.Authorization.AuthorizedTo.SiteIDs) != 2 ||
		result.Application.DisplayName != "My Test App" {
		t.Errorf("unexpected result %+v", result)
	}

	tests := []struct {
		status   int
		contains []string
	}{
		{http.StatusUnauthorized, []string{"unauthorized", "expired", "error details"}},
		{http.StatusForbidden, []string{"forbidden", "scope"}},
		{http.StatusNotFound, []string{"not found"}},
		{http.StatusInternalServerError, []string{"server error", "temporary"}},
		{http.StatusTeapot, []string{"unexpected error (HTTP 418)"}},
	}
	for _, tt := range tests {
		status = tt.status
		_, err := GetTokenIntrospect(context.Background(), client)
		if err == nil {
			t.Errorf("status %d: expected error", tt.status)
			continue
		}
		for _, want := range tt.contains {
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
				t.Errorf("status %d: error %q should contain %q", tt.status, err, want)
			}
		}
	}
}

func TestGetTokenIntrospect_RateLimitedThenOK(t *testing.T) {
	status := http.StatusOK
	rateLimits := 1
	client := useMockAPI(t, tokenServer(t, &status, &rateLimits))

	result, err := GetTokenIntrospect(context.Background(), client)
	if err != nil {
		t.Fatalf("expected success after 429 retry: %v", err)
	}
	if result.Authorization.ID != "auth123" || rateLimits != 0 {
		t.Errorf("result=%+v remaining=%d", result, rateLimits)
	}
}

func TestGetTokenIntrospect_RateLimitExhausted(t *testing.T) {
	status := http.StatusOK
	rateLimits := DefaultMaxRetries + 5
	client := useMockAPI(t, tokenServer(t, &status, &rateLimits))

	_, err := GetTokenIntrospect(context.Background(), client)
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limited error, got %v", err)
	}
}

func TestGetAuthorizedBy(t *testing.T) {
	status := http.StatusOK
	client := useMockAPI(t, tokenServer(t, &status, nil))

	result, err := GetAuthorizedBy(context.Background(), client)
	if err != nil {
		t.Fatalf("GetAuthorizedBy: %v", err)
	}
	if result.ID != "user123" || result.Email != "test@example.com" || result.FirstName != "John" ||
		result.LastName != "Doe" {
		t.Errorf("unexpected result %+v", result)
	}

	status = http.StatusForbidden
	if _, err := GetAuthorizedBy(context.Background(), client); err == nil ||
		!strings.Contains(err.Error(), "authorized_user:read") {
		t.Errorf("expected scope guidance, got %v", err)
	}
	status = http.StatusUnauthorized
	if _, err := GetAuthorizedBy(context.Background(), client); err == nil ||
		!strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected unauthorized, got %v", err)
	}
}

func TestGetAuthorizedBy_RateLimitedThenOK(t *testing.T) {
	status := http.StatusOK
	rateLimits := 2
	client := useMockAPI(t, tokenServer(t, &status, &rateLimits))

	result, err := GetAuthorizedBy(context.Background(), client)
	if err != nil || result.ID != "user123" {
		t.Fatalf("expected success after retries: %+v %v", result, err)
	}
}

func TestHandleTokenError_TruncatesBody(t *testing.T) {
	err := handleTokenError(http.StatusUnauthorized, []byte(strings.Repeat("x", 10000)))
	if len(err.Error()) > 1500 {
		t.Errorf("expected truncated body, got len=%d", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected truncation marker: %v", err)
	}
}

func TestTokenFunctions_ContextCancellation(t *testing.T) {
	status := http.StatusOK
	client := useMockAPI(t, tokenServer(t, &status, nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := GetTokenIntrospect(ctx, client); err == nil || !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("expected context cancelled, got %v", err)
	}
	if _, err := GetAuthorizedBy(ctx, client); err == nil || !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("expected context cancelled, got %v", err)
	}
}

func TestGetTokenInfoFunction_Invoke(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	status := http.StatusOK
	rateLimits := 1
	useMockAPI(t, tokenServer(t, &status, &rateLimits))

	resp, err := (&GetTokenInfo{}).Invoke(context.Background(), infer.FunctionRequest[GetTokenInfoInput]{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	out := resp.Output
	if out.Authorization.ID != "auth123" || out.Authorization.Scope != "sites:read sites:write" ||
		len(out.Authorization.AuthorizedTo.SiteIDs) != 2 || out.Application.ID != "app123" {
		t.Errorf("unexpected output %+v", out)
	}
	if out.Authorization.AuthorizedTo.UserIDs == nil {
		t.Error("nil slices must be normalised to empty")
	}

	status = http.StatusUnauthorized
	if _, err := (&GetTokenInfo{}).Invoke(context.Background(), infer.FunctionRequest[GetTokenInfoInput]{}); err == nil ||
		!strings.Contains(err.Error(), "failed to get token info") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestGetAuthorizedUserFunction_Invoke(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	status := http.StatusOK
	useMockAPI(t, tokenServer(t, &status, nil))

	resp, err := (&GetAuthorizedUser{}).Invoke(context.Background(), infer.FunctionRequest[GetAuthorizedUserInput]{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Output.UserID != "user123" || resp.Output.Email != "test@example.com" || resp.Output.LastName != "Doe" {
		t.Errorf("unexpected output %+v", resp.Output)
	}

	t.Setenv("WEBFLOW_API_TOKEN", "")
	if _, err := (&GetAuthorizedUser{}).Invoke(
		context.Background(), infer.FunctionRequest[GetAuthorizedUserInput]{},
	); err == nil {
		t.Error("expected token error")
	}
}
