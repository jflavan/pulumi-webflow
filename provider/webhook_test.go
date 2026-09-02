// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const webhookTestID = "507f1f77bcf86cd799439011"

// TestValidateWebhookID_Valid tests valid webhook IDs
func TestValidateWebhookID_Valid(t *testing.T) {
	valid := []string{"5f0c8c9e1c9d440000e8d8c3", webhookTestID, "000000000000000000000000", "ffffffffffffffffffffffff"}
	for _, id := range valid {
		if err := ValidateWebhookID(id); err != nil {
			t.Errorf("ValidateWebhookID(%q) = %v, want nil", id, err)
		}
	}
}

// TestValidateWebhookID_Empty tests empty webhook ID
func TestValidateWebhookID_Empty(t *testing.T) {
	err := ValidateWebhookID("")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("ValidateWebhookID(\"\") = %v, want 'required' error", err)
	}
}

// TestValidateWebhookID_InvalidFormat tests invalid webhook ID formats
func TestValidateWebhookID_InvalidFormat(t *testing.T) {
	tests := []struct {
		name      string
		webhookID string
	}{
		{"too short", "5f0c8c9e1c9d"},
		{"too long", "5f0c8c9e1c9d440000e8d8c3extra"},
		{"uppercase", "5F0C8C9E1C9D440000E8D8C3"},
		{"invalid chars", "5f0c8c9e1c9d440000e8d8cg"},
		{"with spaces", "5f0c8c9e 1c9d440000e8d8c3"},
		{"with hyphens", "5f0c8c9e-1c9d-4400-00e8-d8c3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookID(tt.webhookID)
			if err == nil || !strings.Contains(err.Error(), "invalid format") {
				t.Errorf("ValidateWebhookID(%q) = %v, want 'invalid format' error", tt.webhookID, err)
			}
		})
	}
}

// TestValidateWebhookURL tests webhook URL validation
func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{"simple https", "https://example.com/webhook", ""},
		{"with path", "https://api.example.com/webhooks/webflow", ""},
		{"with port", "https://example.com:8443/webhook", ""},
		{"with query", "https://example.com/webhook?source=webflow", ""},
		{"subdomain", "https://webhooks.example.com/webflow", ""},
		{"empty", "", "required"},
		{"http", "http://example.com/webhook", "HTTPS"},
		{"no protocol", "example.com/webhook", "HTTPS"},
		{"ftp", "ftp://example.com/webhook", "HTTPS"},
		{"no domain", "https://", "invalid"},
		{"no tld", "https://example", "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookURL(tt.url)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateWebhookURL(%q) = %v, want nil", tt.url, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidateWebhookURL(%q) = %v, want error containing %q", tt.url, err, tt.wantErr)
			}
		})
	}
}

// TestValidateTriggerType_Valid tests every documented trigger type
func TestValidateTriggerType_Valid(t *testing.T) {
	documented := []string{
		"form_submission", "site_publish", "page_created", "page_metadata_updated", "page_deleted",
		"ecomm_new_order", "ecomm_order_changed", "ecomm_inventory_changed",
		"collection_item_created", "collection_item_changed", "collection_item_deleted",
		"collection_item_published", "collection_item_unpublished", "comment_created",
	}
	for _, triggerType := range documented {
		t.Run(triggerType, func(t *testing.T) {
			if err := ValidateTriggerType(triggerType); err != nil {
				t.Errorf("ValidateTriggerType(%q) = %v, want nil", triggerType, err)
			}
		})
	}
	if len(validTriggerTypeList) != len(documented) {
		t.Errorf("validTriggerTypeList has %d entries, documented list has %d", len(validTriggerTypeList), len(documented))
	}
}

// TestValidateTriggerType_Empty tests empty trigger type
func TestValidateTriggerType_Empty(t *testing.T) {
	err := ValidateTriggerType("")
	if err == nil || !strings.Contains(err.Error(), "required") || !strings.Contains(err.Error(), "comment_created") {
		t.Errorf("ValidateTriggerType(\"\") = %v, want 'required' error listing trigger types", err)
	}
}

// TestValidateTriggerType_Invalid tests invalid trigger types
func TestValidateTriggerType_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		triggerType string
	}{
		{"invalid type", "invalid_trigger"},
		{"typo", "form_submision"},
		{"uppercase", "FORM_SUBMISSION"},
		{"spaces", "form submission"},
		{"undocumented memberships event", "memberships_user_account_added"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTriggerType(tt.triggerType)
			if err == nil || !strings.Contains(err.Error(), "not a valid") {
				t.Errorf("ValidateTriggerType(%q) = %v, want 'not a valid' error", tt.triggerType, err)
			}
		})
	}
}

// TestGenerateWebhookResourceID tests resource ID generation
func TestGenerateWebhookResourceID(t *testing.T) {
	if got := GenerateWebhookResourceID(testSiteID, webhookTestID); got != testSiteID+"/webhooks/"+webhookTestID {
		t.Errorf("GenerateWebhookResourceID() = %q", got)
	}
}

// TestExtractIDsFromWebhookResourceID tests parsing of resource IDs
func TestExtractIDsFromWebhookResourceID(t *testing.T) {
	siteID, webhookID, err := ExtractIDsFromWebhookResourceID(testSiteID + "/webhooks/" + webhookTestID)
	if err != nil || siteID != testSiteID || webhookID != webhookTestID {
		t.Errorf("ExtractIDsFromWebhookResourceID() = %q, %q, %v", siteID, webhookID, err)
	}

	invalid := []struct {
		name       string
		resourceID string
	}{
		{"empty", ""},
		{"missing webhooks part", testSiteID + "/" + webhookTestID},
		{"wrong middle part", testSiteID + "/redirects/" + webhookTestID},
		{"too few parts", testSiteID},
		{"empty site id", "/webhooks/" + webhookTestID},
		{"empty webhook id", testSiteID + "/webhooks/"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := ExtractIDsFromWebhookResourceID(tt.resourceID); err == nil {
				t.Errorf("ExtractIDsFromWebhookResourceID(%q) error = nil, want error", tt.resourceID)
			}
		})
	}
}

func sampleWebhooks() []WebhookResponse {
	return []WebhookResponse{
		{
			ID: "webhook1", TriggerType: "form_submission", URL: "https://example.com/webhook?token=secret",
			SiteID: testSiteID, CreatedOn: "2024-01-01T00:00:00Z",
		},
		{
			ID: webhookTestID, TriggerType: "collection_item_created", URL: "https://example.com/items", SiteID: testSiteID,
			CreatedOn: "2024-01-02T00:00:00Z", LastTriggered: "2024-02-01T00:00:00Z",
			Filter: map[string]interface{}{"collectionId": "abc", "nested": map[string]interface{}{"k": "v"}},
		},
	}
}

// TestGetWebhooks_Valid tests retrieving webhooks successfully
func TestGetWebhooks_Valid(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testSiteID+"/webhooks" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		writeJSON(t, w, http.StatusOK, WebhooksListResponse{Webhooks: sampleWebhooks()})
	})

	result, err := GetWebhooks(context.Background(), client, testSiteID)
	if err != nil {
		t.Fatalf("GetWebhooks failed: %v", err)
	}
	if len(result.Webhooks) != 2 || result.Webhooks[0].ID != "webhook1" ||
		result.Webhooks[0].TriggerType != "form_submission" {
		t.Errorf("GetWebhooks() = %+v", result.Webhooks)
	}
}

// TestGetWebhooks_NotFound tests 404 handling
func TestGetWebhooks_NotFound(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("site not found"))
	})
	_, err := GetWebhooks(context.Background(), client, testSiteID)
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound error, got %v", err)
	}
}

func TestFindWebhook(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, WebhooksListResponse{Webhooks: sampleWebhooks()})
	})
	found, err := FindWebhook(context.Background(), client, testSiteID, webhookTestID)
	if err != nil || found.TriggerType != "collection_item_created" || found.Filter["collectionId"] != "abc" {
		t.Errorf("FindWebhook() = %+v, %v", found, err)
	}
	if _, err := FindWebhook(context.Background(), client, testSiteID, "missing"); !IsNotFound(err) {
		t.Errorf("FindWebhook(missing) error = %v, want IsNotFound", err)
	}
}

// TestPostWebhook tests creating webhooks, asserting the request body
func TestPostWebhook(t *testing.T) {
	filter := map[string]interface{}{"collectionId": "test-collection"}
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testSiteID+"/webhooks" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing Content-Type header")
		}
		var req WebhookRequest
		decodeJSONBody(t, r, &req)
		if req.TriggerType != "collection_item_created" || req.URL != "https://example.com/webhook" ||
			req.Filter["collectionId"] != "test-collection" {
			t.Errorf("unexpected request body: %+v", req)
		}
		writeJSON(t, w, http.StatusCreated, WebhookResponse{
			ID: "new-webhook-1", TriggerType: req.TriggerType, URL: req.URL, SiteID: testSiteID,
			CreatedOn: "2024-01-01T00:00:00Z", Filter: req.Filter,
		})
	})

	result, err := PostWebhook(context.Background(), client, testSiteID, WebhookRequest{
		TriggerType: "collection_item_created", URL: "https://example.com/webhook", Filter: filter,
	})
	if err != nil {
		t.Fatalf("PostWebhook failed: %v", err)
	}
	if result.ID != "new-webhook-1" || result.Filter["collectionId"] != "test-collection" {
		t.Errorf("PostWebhook() = %+v", result)
	}
}

// TestPostWebhook_OmitsEmptyFilter verifies an absent filter is not sent as null/{}.
func TestPostWebhook_OmitsEmptyFilter(t *testing.T) {
	var raw map[string]interface{}
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		decodeJSONBody(t, r, &raw)
		writeJSON(t, w, http.StatusCreated, WebhookResponse{ID: "x"})
	})
	_, err := PostWebhook(context.Background(), client, testSiteID,
		WebhookRequest{TriggerType: "site_publish", URL: "https://example.com/w"})
	if err != nil {
		t.Fatalf("PostWebhook failed: %v", err)
	}
	if _, present := raw["filter"]; present {
		t.Errorf("request body should omit filter when unset, got %v", raw)
	}
}

// TestPostWebhook_ValidationError tests 400 handling
func TestPostWebhook_ValidationError(t *testing.T) {
	client := mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid webhook configuration"))
	})
	_, err := PostWebhook(context.Background(), client, testSiteID,
		WebhookRequest{TriggerType: "invalid", URL: "https://example.com/webhook"})
	if err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected 'bad request' error, got: %v", err)
	}
}

// TestDeleteWebhook tests deletion outcomes
func TestDeleteWebhook(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr string
	}{
		{"success", http.StatusNoContent, ""},
		{"not found is idempotent", http.StatusNotFound, ""},
		{"server error", http.StatusInternalServerError, "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/v2/webhooks/"+webhookTestID {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tt.status)
			})
			err := DeleteWebhook(context.Background(), client, webhookTestID)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("DeleteWebhook() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("DeleteWebhook() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// TestWebhookErrorMessagesAreActionable verifies error messages contain guidance
func TestWebhookErrorMessagesAreActionable(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func() error
		contains []string
	}{
		{"ValidateWebhookID empty", func() error { return ValidateWebhookID("") }, []string{"required", "24-character"}},
		{"ValidateWebhookURL empty", func() error { return ValidateWebhookURL("") }, []string{"required", "HTTPS"}},
		{
			"ValidateWebhookURL not HTTPS",
			func() error { return ValidateWebhookURL("http://example.com") },
			[]string{"HTTPS", "security"},
		},
		{
			"ValidateTriggerType empty",
			func() error { return ValidateTriggerType("") },
			[]string{"required", "valid trigger types"},
		},
		{
			"ValidateTriggerType invalid",
			func() error { return ValidateTriggerType("invalid") },
			[]string{"not a valid", "form_submission"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tt.name)
			}
			for _, expected := range tt.contains {
				if !strings.Contains(err.Error(), expected) {
					t.Errorf("%s: error message missing %q. Got: %s", tt.name, expected, err)
				}
			}
		})
	}
}

// TestWebhookAPI_RateLimited tests that 429 responses are retried for every operation
func TestWebhookAPI_RateLimited(t *testing.T) {
	newHandler := func(attempts *int, onSuccess func(w http.ResponseWriter)) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			*attempts++
			if *attempts < 2 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			onSuccess(w)
		}
	}

	t.Run("get", func(t *testing.T) {
		attempts := 0
		client := mockAPI(t, newHandler(&attempts, func(w http.ResponseWriter) {
			writeJSON(t, w, http.StatusOK, WebhooksListResponse{Webhooks: sampleWebhooks()[:1]})
		}))
		result, err := GetWebhooks(context.Background(), client, testSiteID)
		if err != nil || len(result.Webhooks) != 1 || attempts != 2 {
			t.Fatalf("GetWebhooks() = %+v, %v after %d attempts", result, err, attempts)
		}
	})
	t.Run("post", func(t *testing.T) {
		attempts := 0
		client := mockAPI(t, newHandler(&attempts, func(w http.ResponseWriter) {
			writeJSON(t, w, http.StatusCreated, WebhookResponse{ID: "newwebhook"})
		}))
		result, err := PostWebhook(context.Background(), client, testSiteID,
			WebhookRequest{TriggerType: "form_submission", URL: "https://example.com/webhook"})
		if err != nil || result.ID != "newwebhook" || attempts != 2 {
			t.Fatalf("PostWebhook() = %+v, %v after %d attempts", result, err, attempts)
		}
	})
	t.Run("delete", func(t *testing.T) {
		attempts := 0
		client := mockAPI(t, newHandler(&attempts, func(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }))
		if err := DeleteWebhook(context.Background(), client, webhookTestID); err != nil || attempts != 2 {
			t.Fatalf("DeleteWebhook() = %v after %d attempts", err, attempts)
		}
	})
}

// TestMapsEqual tests the filter comparison utility
func TestMapsEqual(t *testing.T) {
	tests := []struct {
		name  string
		a     map[string]interface{}
		b     map[string]interface{}
		equal bool
	}{
		{"both nil", nil, nil, true},
		{"nil and empty are equal", nil, map[string]interface{}{}, true},
		{"empty maps", map[string]interface{}{}, map[string]interface{}{}, true},
		{"one nil", nil, map[string]interface{}{"key": "value"}, false},
		{"same content", map[string]interface{}{"key": "value"}, map[string]interface{}{"key": "value"}, true},
		{"different values", map[string]interface{}{"key": "value1"}, map[string]interface{}{"key": "value2"}, false},
		{"different keys", map[string]interface{}{"key1": "value"}, map[string]interface{}{"key2": "value"}, false},
		{
			"different length",
			map[string]interface{}{"key": "value"},
			map[string]interface{}{"key": "value", "key2": "value2"},
			false,
		},
		{
			"nested equal",
			map[string]interface{}{"n": map[string]interface{}{"a": []interface{}{1.0, "x"}}},
			map[string]interface{}{"n": map[string]interface{}{"a": []interface{}{1.0, "x"}}},
			true,
		},
		{
			"nested differ",
			map[string]interface{}{"n": map[string]interface{}{"a": []interface{}{1.0, "x"}}},
			map[string]interface{}{"n": map[string]interface{}{"a": []interface{}{2.0, "x"}}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapsEqual(tt.a, tt.b); got != tt.equal {
				t.Errorf("mapsEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.equal)
			}
		})
	}
}

// =============================================================================
// Webhook Diff Tests
// =============================================================================

func TestWebhookDiff(t *testing.T) {
	resource := &Webhook{}
	base := WebhookArgs{
		SiteID: testSiteID, TriggerType: "collection_item_created", URL: "https://example.com/hook",
		Filter: map[string]interface{}{"collectionId": "abc", "nested": map[string]interface{}{"k": "v"}},
	}
	// modified returns a deep copy of base with fn applied, so tests never share filter maps.
	modified := func(fn func(a *WebhookArgs)) WebhookArgs {
		a := base
		a.Filter = map[string]interface{}{"collectionId": "abc", "nested": map[string]interface{}{"k": "v"}}
		fn(&a)
		return a
	}
	noFilter := modified(func(a *WebhookArgs) { a.Filter = nil })
	emptyFilter := modified(func(a *WebhookArgs) { a.Filter = map[string]interface{}{} })

	tests := []struct {
		name    string
		inputs  WebhookArgs
		state   WebhookArgs
		wantKey string
	}{
		{"no change", base, base, ""},
		{"nested filter values equal", modified(func(*WebhookArgs) {}), base, ""},
		{"nil filter equals empty filter", noFilter, emptyFilter, ""},
		{"empty filter equals nil filter", emptyFilter, noFilter, ""},
		{"siteId change", modified(func(a *WebhookArgs) { a.SiteID = "5f0c8c9e1c9d440000e8d8c4" }), base, "siteId"},
		{"triggerType change", modified(func(a *WebhookArgs) { a.TriggerType = "site_publish" }), base, "triggerType"},
		{"url change", modified(func(a *WebhookArgs) { a.URL = "https://example.com/other" }), base, "url"},
		{"filter value change", modified(func(a *WebhookArgs) { a.Filter["collectionId"] = "def" }), base, "filter"},
		{
			"nested filter value change",
			modified(func(a *WebhookArgs) { a.Filter["nested"].(map[string]interface{})["k"] = "w" }),
			base,
			"filter",
		},
		{"filter removed", noFilter, base, "filter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resource.Diff(context.Background(), infer.DiffRequest[WebhookArgs, WebhookState]{
				ID:     GenerateWebhookResourceID(testSiteID, webhookTestID),
				Inputs: tt.inputs,
				State:  WebhookState{WebhookArgs: tt.state},
			})
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if tt.wantKey == "" {
				if resp.HasChanges {
					t.Errorf("Diff() should report no changes, got %+v", resp.DetailedDiff)
				}
				return
			}
			if !resp.HasChanges || !resp.DeleteBeforeReplace {
				t.Errorf("Diff() should require replacement for %s: %+v", tt.wantKey, resp)
			}
			if d, ok := resp.DetailedDiff[tt.wantKey]; !ok || d.Kind != p.UpdateReplace {
				t.Errorf("Diff() DetailedDiff[%s] = %+v (present=%v), want UpdateReplace", tt.wantKey, d, ok)
			}
			if len(resp.DetailedDiff) != 1 {
				t.Errorf("Diff() should only flag %s, got %+v", tt.wantKey, resp.DetailedDiff)
			}
		})
	}
}

// =============================================================================
// Webhook resource-level CRUD tests
// =============================================================================

func TestWebhookCreate(t *testing.T) {
	var got WebhookRequest
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testSiteID+"/webhooks" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		decodeJSONBody(t, r, &got)
		writeJSON(t, w, http.StatusCreated, WebhookResponse{
			ID: webhookTestID, TriggerType: got.TriggerType, URL: got.URL, SiteID: testSiteID, Filter: got.Filter,
			CreatedOn: "2024-01-01T00:00:00Z",
		})
	})

	resource := &Webhook{}
	resp, err := resource.Create(context.Background(), infer.CreateRequest[WebhookArgs]{Inputs: WebhookArgs{
		SiteID: testSiteID, TriggerType: "comment_created", URL: "https://example.com/hook?token=secret",
		Filter: map[string]interface{}{"name": "Email Form"},
	}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.ID != testSiteID+"/webhooks/"+webhookTestID || resp.Output.CreatedOn != "2024-01-01T00:00:00Z" {
		t.Errorf("Create() = %+v", resp)
	}
	if got.TriggerType != "comment_created" || got.URL != "https://example.com/hook?token=secret" ||
		got.Filter["name"] != "Email Form" {
		t.Errorf("request body = %+v", got)
	}
}

func TestWebhookCreate_ValidationErrors(t *testing.T) {
	resource := &Webhook{}
	tests := []struct {
		name   string
		inputs WebhookArgs
		want   string
	}{
		{
			"invalid siteId",
			WebhookArgs{SiteID: "bad", TriggerType: "form_submission", URL: "https://example.com/h"},
			"validation failed",
		},
		{
			"invalid triggerType",
			WebhookArgs{SiteID: testSiteID, TriggerType: "nope", URL: "https://example.com/h"},
			"not a valid",
		},
		{
			"http url",
			WebhookArgs{SiteID: testSiteID, TriggerType: "form_submission", URL: "http://example.com/h"},
			"HTTPS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No server and no token: validation must fail before any API call.
			resp, err := resource.Create(context.Background(), infer.CreateRequest[WebhookArgs]{Inputs: tt.inputs})
			if err == nil || !strings.Contains(err.Error(), tt.want) || resp.ID != "" {
				t.Fatalf("Create() = (%q, %v), want error containing %q", resp.ID, err, tt.want)
			}
		})
	}
}

func TestWebhookCreate_DryRun(t *testing.T) {
	resource := &Webhook{}
	for name, inputs := range map[string]WebhookArgs{
		"valid":                 {SiteID: testSiteID, TriggerType: "form_submission", URL: "https://example.com/h"},
		"unknown inputs":        {},
		"invalid site deferred": {SiteID: "bad", TriggerType: "nope", URL: "http://x"},
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := resource.Create(context.Background(), infer.CreateRequest[WebhookArgs]{Inputs: inputs, DryRun: true})
			if err != nil {
				t.Fatalf("Create() dry-run should succeed (validation is deferred): %v", err)
			}
			if resp.ID == "" || resp.Output.CreatedOn == "" {
				t.Errorf("Create() dry-run should return ID and timestamp: %+v", resp)
			}
		})
	}
}

func TestWebhookRead(t *testing.T) {
	resource := &Webhook{}
	readReq := infer.ReadRequest[WebhookArgs, WebhookState]{ID: GenerateWebhookResourceID(testSiteID, webhookTestID)}

	t.Run("found", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v2/sites/"+testSiteID+"/webhooks" {
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			writeJSON(t, w, http.StatusOK, WebhooksListResponse{Webhooks: sampleWebhooks()})
		})
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.ID != readReq.ID || resp.Inputs.SiteID != testSiteID ||
			resp.Inputs.TriggerType != "collection_item_created" ||
			resp.Inputs.URL != "https://example.com/items" || resp.Inputs.Filter["collectionId"] != "abc" {
			t.Errorf("Read() inputs = %+v", resp.Inputs)
		}
		if resp.State.CreatedOn != "2024-01-02T00:00:00Z" || resp.State.LastTriggered != "2024-02-01T00:00:00Z" {
			t.Errorf("Read() state = %+v", resp.State)
		}
	})

	t.Run("webhook missing signals deletion", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, WebhooksListResponse{Webhooks: sampleWebhooks()[:1]})
		})
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
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"webhook not found upstream"}`))
		})
		if _, err := resource.Read(context.Background(), readReq); err == nil {
			t.Fatal("Read() should return an error for 500")
		}
	})

	t.Run("invalid site id in resource id", func(t *testing.T) {
		mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected") })
		_, err := resource.Read(context.Background(),
			infer.ReadRequest[WebhookArgs, WebhookState]{ID: "bad/webhooks/" + webhookTestID})
		if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
			t.Fatalf("Read() error = %v", err)
		}
	})
}

func TestWebhookUpdate_ReturnsError(t *testing.T) {
	resource := &Webhook{}
	if _, err := resource.Update(context.Background(), infer.UpdateRequest[WebhookArgs, WebhookState]{}); err == nil ||
		!strings.Contains(err.Error(), "cannot be updated in-place") {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestWebhookDelete(t *testing.T) {
	tests := []struct {
		name      string
		webhookID string
		status    int
	}{
		{"success", webhookTestID, http.StatusNoContent},
		{"not found is idempotent", webhookTestID, http.StatusNotFound},
		// Create accepts whatever ID Webflow assigns, so Delete must not reject non-hex IDs.
		{"non-hex id assigned by the API", "wh_custom-format", http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodDelete || r.URL.Path != "/v2/webhooks/"+tt.webhookID {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tt.status)
			})
			resource := &Webhook{}
			if _, err := resource.Delete(context.Background(), infer.DeleteRequest[WebhookState]{
				ID: GenerateWebhookResourceID(testSiteID, tt.webhookID),
			}); err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			if calls != 1 {
				t.Errorf("expected 1 API call, got %d", calls)
			}
		})
	}

	t.Run("malformed resource id", func(t *testing.T) {
		mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected") })
		resource := &Webhook{}
		_, err := resource.Delete(context.Background(), infer.DeleteRequest[WebhookState]{ID: testSiteID + "/webhooks/"})
		if err == nil {
			t.Error("Delete() should reject a resource ID without a webhook ID")
		}
	})
}
