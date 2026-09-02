// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const testEcomSiteID = "5f0c8c9e1c9d440000e8d8c3"

func TestValidateCurrencyCode(t *testing.T) {
	for _, code := range []string{"USD", "EUR", "GBP", "JPY"} {
		if err := ValidateCurrencyCode(code); err != nil {
			t.Errorf("ValidateCurrencyCode(%q) = %v", code, err)
		}
	}
	if err := ValidateCurrencyCode(""); err == nil || !strings.Contains(err.Error(), "required") || !strings.Contains(err.Error(), "ISO 4217") {
		t.Errorf("empty: %v", err)
	}
	for _, code := range []string{"usd", "US", "USDD", "US1", "US$", "Usd"} {
		if err := ValidateCurrencyCode(code); err == nil || !strings.Contains(err.Error(), "invalid format") {
			t.Errorf("ValidateCurrencyCode(%q) = %v", code, err)
		}
	}
}

func TestEcommerceSettingsResourceIDRoundTrip(t *testing.T) {
	id := GenerateEcommerceSettingsResourceID(testEcomSiteID)
	if id != testEcomSiteID+"/ecommerce/settings" {
		t.Fatalf("id = %q", id)
	}
	siteID, err := ExtractSiteIDFromEcommerceSettingsResourceID(id)
	if err != nil || siteID != testEcomSiteID {
		t.Fatalf("extract: %q %v", siteID, err)
	}
	for _, bad := range []string{"", testEcomSiteID, testEcomSiteID + "/ecommerce", testEcomSiteID + "/settings", testEcomSiteID + "/redirects/123", "/ecommerce/settings"} {
		if _, err := ExtractSiteIDFromEcommerceSettingsResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

// ecomServer answers GET /v2/sites/{id}/ecommerce/settings with the given status.
func ecomServer(t *testing.T, status *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testEcomSiteID+"/ecommerce/settings" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(*status)
		switch *status {
		case http.StatusOK:
			_ = json.NewEncoder(w).Encode(EcommerceSettingsResponse{SiteID: testEcomSiteID, CreatedOn: "2024-01-15T10:30:00Z", DefaultCurrency: "EUR"})
		case http.StatusConflict:
			_, _ = w.Write([]byte(`{"code":"ecommerce_not_enabled","message":"Site does not have ecommerce enabled"}`))
		default:
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGetEcommerceSettings(t *testing.T) {
	status := http.StatusOK
	client := useMockAPI(t, ecomServer(t, &status))

	resp, err := GetEcommerceSettings(context.Background(), client, testEcomSiteID)
	if err != nil {
		t.Fatalf("GetEcommerceSettings: %v", err)
	}
	if resp.DefaultCurrency != "EUR" || resp.CreatedOn != "2024-01-15T10:30:00Z" {
		t.Errorf("unexpected response %+v", resp)
	}

	status = http.StatusNotFound
	if _, err := GetEcommerceSettings(context.Background(), client, testEcomSiteID); !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}

	status = http.StatusConflict
	_, err = GetEcommerceSettings(context.Background(), client, testEcomSiteID)
	if !IsEcommerceNotEnabled(err) || IsNotFound(err) {
		t.Errorf("expected EcommerceNotEnabledError, got %v", err)
	}
	if !strings.Contains(err.Error(), "ecommerce not enabled") || !strings.Contains(err.Error(), "Webflow dashboard") ||
		!strings.Contains(err.Error(), "ecommerce_not_enabled") {
		t.Errorf("error should be actionable and include details: %v", err)
	}

	for _, s := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		status = s
		_, err := GetEcommerceSettings(context.Background(), client, testEcomSiteID)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != s || IsEcommerceNotEnabled(err) {
			t.Errorf("status %d: expected APIError, got %v", s, err)
		}
	}
}

func TestHandleEcommerceNotEnabledError_TruncatesBody(t *testing.T) {
	err := handleEcommerceNotEnabledError([]byte(strings.Repeat("x", 5000)))
	if !IsEcommerceNotEnabled(err) || len(err.Error()) > 1200 {
		t.Errorf("expected truncated typed error, got len=%d", len(err.Error()))
	}
}

func TestGetEcommerceSettings_ContextCancellation(t *testing.T) {
	status := http.StatusOK
	client := useMockAPI(t, ecomServer(t, &status))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GetEcommerceSettings(ctx, client, testEcomSiteID); err == nil || !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("expected context cancelled, got %v", err)
	}
}

func TestEcommerceSettingsCreate(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	status := http.StatusOK
	useMockAPI(t, ecomServer(t, &status))
	args := EcommerceSettingsArgs{SiteID: testEcomSiteID}

	dry, err := (&EcommerceSettings{}).Create(context.Background(), infer.CreateRequest[EcommerceSettingsArgs]{Inputs: EcommerceSettingsArgs{SiteID: "bad"}, DryRun: true})
	if err != nil {
		t.Fatalf("dry run must not validate or call the API: %v", err)
	}
	if dry.Output.DefaultCurrency != "" || dry.Output.CreatedOn != "" {
		t.Errorf("dry run must not fabricate a currency: %+v", dry.Output)
	}

	resp, err := (&EcommerceSettings{}).Create(context.Background(), infer.CreateRequest[EcommerceSettingsArgs]{Inputs: args})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.ID != GenerateEcommerceSettingsResourceID(testEcomSiteID) || resp.Output.DefaultCurrency != "EUR" || resp.Output.CreatedOn == "" {
		t.Errorf("unexpected response %+v", resp)
	}

	if _, err := (&EcommerceSettings{}).Create(context.Background(), infer.CreateRequest[EcommerceSettingsArgs]{Inputs: EcommerceSettingsArgs{SiteID: "bad"}}); err == nil ||
		!strings.Contains(err.Error(), "siteId has invalid format") {
		t.Errorf("expected validation error, got %v", err)
	}

	status = http.StatusConflict
	_, err = (&EcommerceSettings{}).Create(context.Background(), infer.CreateRequest[EcommerceSettingsArgs]{Inputs: args})
	if err == nil || !IsEcommerceNotEnabled(err) {
		t.Errorf("expected ecommerce not enabled error, got %v", err)
	}
}

func TestEcommerceSettingsRead(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	status := http.StatusOK
	useMockAPI(t, ecomServer(t, &status))
	id := GenerateEcommerceSettingsResourceID(testEcomSiteID)

	resp, err := (&EcommerceSettings{}).Read(context.Background(), infer.ReadRequest[EcommerceSettingsArgs, EcommerceSettingsState]{ID: id})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.ID != id || resp.Inputs.SiteID != testEcomSiteID || resp.State.DefaultCurrency != "EUR" {
		t.Errorf("unexpected response %+v", resp)
	}

	status = http.StatusNotFound
	resp, err = (&EcommerceSettings{}).Read(context.Background(), infer.ReadRequest[EcommerceSettingsArgs, EcommerceSettingsState]{ID: id})
	if err != nil || resp.ID != "" {
		t.Errorf("404 should clear the resource: id=%q err=%v", resp.ID, err)
	}

	status = http.StatusConflict
	if _, err := (&EcommerceSettings{}).Read(context.Background(), infer.ReadRequest[EcommerceSettingsArgs, EcommerceSettingsState]{ID: id}); err == nil || !IsEcommerceNotEnabled(err) {
		t.Errorf("409 must surface the ecommerce-not-enabled error, got %v", err)
	}

	status = http.StatusInternalServerError
	if _, err := (&EcommerceSettings{}).Read(context.Background(), infer.ReadRequest[EcommerceSettingsArgs, EcommerceSettingsState]{ID: id}); err == nil {
		t.Error("500 must propagate")
	}

	for _, bad := range []string{"", "x/ecommerce/settings", testEcomSiteID} {
		if _, err := (&EcommerceSettings{}).Read(context.Background(), infer.ReadRequest[EcommerceSettingsArgs, EcommerceSettingsState]{ID: bad}); err == nil {
			t.Errorf("expected invalid ID error for %q", bad)
		}
	}
}

func TestEcommerceSettingsDiffUpdateDelete(t *testing.T) {
	args := EcommerceSettingsArgs{SiteID: testEcomSiteID}
	state := EcommerceSettingsState{EcommerceSettingsArgs: args, DefaultCurrency: "EUR", CreatedOn: "2024-01-15T10:30:00Z"}

	resp, err := (&EcommerceSettings{}).Diff(context.Background(), infer.DiffRequest[EcommerceSettingsArgs, EcommerceSettingsState]{Inputs: args, State: state})
	if err != nil || resp.HasChanges {
		t.Fatalf("expected no changes: %+v %v", resp, err)
	}
	resp, err = (&EcommerceSettings{}).Diff(context.Background(), infer.DiffRequest[EcommerceSettingsArgs, EcommerceSettingsState]{
		Inputs: EcommerceSettingsArgs{SiteID: "6f1d9d0f2d0e551111f9e9d4"}, State: state,
	})
	if err != nil || !resp.HasChanges || resp.DetailedDiff["siteId"].Kind != p.UpdateReplace {
		t.Errorf("expected siteId replace, got %+v %v", resp, err)
	}

	up, err := (&EcommerceSettings{}).Update(context.Background(), infer.UpdateRequest[EcommerceSettingsArgs, EcommerceSettingsState]{Inputs: args, State: state})
	if err != nil || up.Output.DefaultCurrency != "EUR" {
		t.Errorf("Update: %+v %v", up, err)
	}

	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("delete must not call the API, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	useMockAPI(t, server)
	if _, err := (&EcommerceSettings{}).Delete(context.Background(), infer.DeleteRequest[EcommerceSettingsState]{ID: GenerateEcommerceSettingsResourceID(testEcomSiteID), State: state}); err != nil {
		t.Errorf("Delete: %v", err)
	}
}
