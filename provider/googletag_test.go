// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const (
	testGoogleTagSiteID = "5f0c8c9e1c9d440000e8d8c3"
	testGoogleTagID     = "G-1A2B3C4D5E"
	testGoogleTagPath   = "/v2/sites/" + testGoogleTagSiteID + "/integrations/google_tags"
	testGoogleTagAuth   = "test-token-abc123def456"
)

func intPtr(v int) *int { return &v }

// googleTagArgs builds inputs for the standard test site and tag.
func googleTagArgs(displayName string, order *int) GoogleTagArgs {
	return GoogleTagArgs{SiteID: testGoogleTagSiteID, TagID: testGoogleTagID, DisplayName: displayName, Order: order}
}

func TestValidateGoogleTagID(t *testing.T) {
	tests := []struct {
		name    string
		tagID   string
		wantErr string
	}{
		{"ga4", "G-1A2B3C4D5E", ""},
		{"google tag", "GT-ABC123", ""},
		{"ads", "AW-123456789", ""},
		{"campaign manager", "DC-1234567", ""},
		{"empty", "", "required"},
		{"universal analytics", "UA-12345-1", "UA-"},
		{"lowercase ua", "ua-12345-1", "UA-"},
		{"unknown prefix", "XX-123", "invalid format"},
		{"no prefix", "1A2B3C4D5E", "invalid format"},
		{"bad chars", "G-1A2B/3C", "invalid format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGoogleTagID(tt.tagID)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateGoogleTagID(%q) unexpected error: %v", tt.tagID, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateGoogleTagID(%q) = %v, want error containing %q", tt.tagID, err, tt.wantErr)
			}
		})
	}
}

func TestGoogleTagResourceID(t *testing.T) {
	id := GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID)
	if id != testGoogleTagSiteID+"/google_tags/"+testGoogleTagID {
		t.Fatalf("unexpected resource ID %q", id)
	}
	siteID, tagID, err := ExtractIDsFromGoogleTagResourceID(id)
	if err != nil || siteID != testGoogleTagSiteID || tagID != testGoogleTagID {
		t.Fatalf("ExtractIDsFromGoogleTagResourceID(%q) = %q, %q, %v", id, siteID, tagID, err)
	}
	invalid := []string{"", "abc", "abc/webhooks/G-1", "/google_tags/G-1", "abc/google_tags/", "a/google_tags/b/c"}
	for _, bad := range invalid {
		if _, _, err := ExtractIDsFromGoogleTagResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

// newGoogleTagServer returns a mock server for the Google Tags endpoints that records requests.
func newGoogleTagServer(
	t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte),
) *httptest.Server {
	t.Helper()
	t.Setenv("WEBFLOW_API_TOKEN", testGoogleTagAuth)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		handler(w, r, body)
	}))
	t.Cleanup(server.Close)
	useMockAPI(t, server)
	return server
}

// noAPICall fails the test if any request reaches the mock server.
func noAPICall(t *testing.T) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		t.Errorf("no API call expected, got %s %s", r.Method, r.URL.Path)
	}
}

func TestGoogleTagCreate_UpsertsAndFindsTag(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, string(body)
		_, _ = w.Write([]byte(`{"googleTagIds":[
			{"displayName":"Other","tagId":"GT-OTHER","order":0},
			{"displayName":"Primary Google Analytics","tagId":"G-1A2B3C4D5E","order":1}]}`))
	})

	resp, err := (&GoogleTag{}).Create(context.Background(), infer.CreateRequest[GoogleTagArgs]{
		Inputs: googleTagArgs("Primary Google Analytics", nil),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != testGoogleTagPath {
		t.Errorf("expected PATCH %s, got %s %s", testGoogleTagPath, gotMethod, gotPath)
	}
	if gotBody != `{"googleTagIds":[{"displayName":"Primary Google Analytics","tagId":"G-1A2B3C4D5E"}]}` {
		t.Errorf("unexpected body: %s", gotBody)
	}
	if resp.ID != testGoogleTagSiteID+"/google_tags/"+testGoogleTagID {
		t.Errorf("unexpected ID %q", resp.ID)
	}
	if resp.Output.EffectiveOrder == nil || *resp.Output.EffectiveOrder != 1 {
		t.Errorf("expected effectiveOrder 1, got %v", resp.Output.EffectiveOrder)
	}
	if resp.Output.Order != nil {
		t.Errorf("order input should stay unset, got %v", *resp.Output.Order)
	}
}

func TestGoogleTagCreate_SendsExplicitOrder(t *testing.T) {
	var gotBody string
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotBody = string(body)
		_, _ = w.Write([]byte(`{"googleTagIds":[{"displayName":"Ads","tagId":"AW-123","order":2}]}`))
	})

	resp, err := (&GoogleTag{}).Create(context.Background(), infer.CreateRequest[GoogleTagArgs]{
		Inputs: GoogleTagArgs{SiteID: testGoogleTagSiteID, TagID: "AW-123", DisplayName: "Ads", Order: intPtr(2)},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotBody != `{"googleTagIds":[{"displayName":"Ads","tagId":"AW-123","order":2}]}` {
		t.Errorf("unexpected body: %s", gotBody)
	}
	if resp.Output.Order == nil || *resp.Output.Order != 2 {
		t.Errorf("expected order 2 preserved, got %v", resp.Output.Order)
	}
}

func TestGoogleTagCreate_DryRunSkipsAPIAndValidation(t *testing.T) {
	newGoogleTagServer(t, noAPICall(t))

	resp, err := (&GoogleTag{}).Create(context.Background(), infer.CreateRequest[GoogleTagArgs]{
		Inputs: GoogleTagArgs{SiteID: "", TagID: "", DisplayName: ""},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create dry-run: %v", err)
	}
	if resp.Output.EffectiveOrder != nil {
		t.Errorf("preview must not fabricate effectiveOrder")
	}
	if resp.ID != "" {
		t.Errorf("preview with unknown siteId/tagId must not fabricate an ID, got %q", resp.ID)
	}
}

func TestGoogleTagCreate_ValidationErrors(t *testing.T) {
	newGoogleTagServer(t, noAPICall(t))

	cases := []struct {
		name string
		args GoogleTagArgs
		want string
	}{
		{"bad site", GoogleTagArgs{SiteID: "nope", TagID: testGoogleTagID, DisplayName: "x"}, "siteId"},
		{"ua tag", GoogleTagArgs{SiteID: testGoogleTagSiteID, TagID: "UA-1-1", DisplayName: "x"}, "UA-"},
		{"no name", googleTagArgs(" ", nil), "displayName"},
		{"neg order", googleTagArgs("x", intPtr(-1)), "order"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&GoogleTag{}).Create(context.Background(), infer.CreateRequest[GoogleTagArgs]{Inputs: tc.args})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestGoogleTagCreate_TagMissingFromResponse(t *testing.T) {
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		_, _ = w.Write([]byte(`{"googleTagIds":[]}`))
	})

	_, err := (&GoogleTag{}).Create(context.Background(), infer.CreateRequest[GoogleTagArgs]{
		Inputs: googleTagArgs("x", nil),
	})
	if err == nil || !strings.Contains(err.Error(), "missing from the returned tag list") {
		t.Fatalf("expected missing-tag error, got %v", err)
	}
}

func TestGoogleTagRead_ListsAndFindsTag(t *testing.T) {
	var gotMethod, gotPath string
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"googleTagIds":[
			{"displayName":"Other","tagId":"GT-OTHER","order":0},
			{"displayName":"Renamed in dashboard","tagId":"g-1a2b3c4d5e","order":3}]}`))
	})

	id := GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID)
	resp, err := (&GoogleTag{}).Read(context.Background(), infer.ReadRequest[GoogleTagArgs, GoogleTagState]{
		ID:     id,
		Inputs: googleTagArgs("Primary", nil),
		State:  GoogleTagState{GoogleTagArgs: googleTagArgs("Primary", nil)},
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != testGoogleTagPath {
		t.Errorf("expected GET %s, got %s %s", testGoogleTagPath, gotMethod, gotPath)
	}
	if resp.ID != id {
		t.Errorf("expected ID preserved, got %q", resp.ID)
	}
	if resp.Inputs.DisplayName != "Renamed in dashboard" {
		t.Errorf("expected drifted displayName, got %q", resp.Inputs.DisplayName)
	}
	if resp.Inputs.TagID != testGoogleTagID {
		t.Errorf("the user's tagId casing must be kept when Webflow only normalized case, got %q", resp.Inputs.TagID)
	}
	if resp.Inputs.Order != nil {
		t.Errorf("order should stay unset when not configured, got %v", *resp.Inputs.Order)
	}
	if resp.State.EffectiveOrder == nil || *resp.State.EffectiveOrder != 3 {
		t.Errorf("expected effectiveOrder 3, got %v", resp.State.EffectiveOrder)
	}
}

func TestGoogleTagRead_ExplicitOrderDrift(t *testing.T) {
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		_, _ = w.Write([]byte(`{"googleTagIds":[{"displayName":"P","tagId":"G-1A2B3C4D5E","order":5}]}`))
	})

	resp, err := (&GoogleTag{}).Read(context.Background(), infer.ReadRequest[GoogleTagArgs, GoogleTagState]{
		ID:     GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID),
		Inputs: googleTagArgs("P", intPtr(1)),
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.Inputs.Order == nil || *resp.Inputs.Order != 5 {
		t.Errorf("expected drifted order 5, got %v", resp.Inputs.Order)
	}
}

func TestGoogleTagRead_ImportUsesAPITagID(t *testing.T) {
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		_, _ = w.Write([]byte(`{"googleTagIds":[{"displayName":"P","tagId":"G-1A2B3C4D5E","order":0}]}`))
	})

	// Import: no inputs, and the ID may be spelled differently from the API.
	resp, err := (&GoogleTag{}).Read(context.Background(), infer.ReadRequest[GoogleTagArgs, GoogleTagState]{
		ID: GenerateGoogleTagResourceID(testGoogleTagSiteID, "g-1a2b3c4d5e"),
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.Inputs.TagID != "G-1A2B3C4D5E" || resp.Inputs.DisplayName != "P" || resp.Inputs.SiteID != testGoogleTagSiteID {
		t.Errorf("import should take the API values, got %+v", resp.Inputs)
	}
}

func TestGoogleTagReadDelete_InvalidSiteIDRejectedBeforeAPI(t *testing.T) {
	newGoogleTagServer(t, noAPICall(t))
	for _, id := range []string{"nope/google_tags/" + testGoogleTagID, "5F0C8C9E1C9D440000E8D8C3/google_tags/G-1"} {
		if _, err := (&GoogleTag{}).Read(
			context.Background(), infer.ReadRequest[GoogleTagArgs, GoogleTagState]{ID: id},
		); err == nil || !strings.Contains(err.Error(), "siteId") {
			t.Errorf("Read(%q): expected siteId error, got %v", id, err)
		}
		if _, err := (&GoogleTag{}).Delete(
			context.Background(), infer.DeleteRequest[GoogleTagState]{ID: id},
		); err == nil || !strings.Contains(err.Error(), "siteId") {
			t.Errorf("Delete(%q): expected siteId error, got %v", id, err)
		}
	}
}

func TestGoogleTagCheck(t *testing.T) {
	inputs := property.NewMap(map[string]property.Value{
		"siteId":      property.New("bad"),
		"tagId":       property.New("UA-1-1"),
		"displayName": property.New("  "),
		"order":       property.New(-1.0),
	})
	resp, err := (&GoogleTag{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	got := map[string]string{}
	for _, f := range resp.Failures {
		got[f.Property] = f.Reason
	}
	if len(got) != 4 || !strings.Contains(got["siteId"], "invalid format") || !strings.Contains(got["tagId"], "UA-") ||
		!strings.Contains(got["displayName"], "required") || !strings.Contains(got["order"], "negative") {
		t.Errorf("unexpected failures %+v", resp.Failures)
	}

	unknown := property.NewMap(map[string]property.Value{
		"siteId":      property.New(property.Computed),
		"tagId":       property.New(property.Computed),
		"displayName": property.New(property.Computed),
		"order":       property.New(property.Computed),
	})
	if resp, err := (&GoogleTag{}).Check(
		context.Background(), infer.CheckRequest{NewInputs: unknown},
	); err != nil || len(resp.Failures) != 0 {
		t.Errorf("unknown inputs must not fail Check: %+v %v", resp.Failures, err)
	}

	valid := property.NewMap(map[string]property.Value{
		"siteId":      property.New(testGoogleTagSiteID),
		"tagId":       property.New(testGoogleTagID),
		"displayName": property.New("Primary"),
		"order":       property.New(2.0),
	})
	resp, err = (&GoogleTag{}).Check(context.Background(), infer.CheckRequest{NewInputs: valid})
	if err != nil || len(resp.Failures) != 0 || resp.Inputs.TagID != testGoogleTagID || resp.Inputs.Order == nil ||
		*resp.Inputs.Order != 2 {
		t.Errorf("valid inputs must pass Check: %+v %v", resp, err)
	}
}

func TestGoogleTagRead_TagGone(t *testing.T) {
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		_, _ = w.Write([]byte(`{"googleTagIds":[{"displayName":"Other","tagId":"GT-OTHER","order":0}]}`))
	})

	resp, err := (&GoogleTag{}).Read(context.Background(), infer.ReadRequest[GoogleTagArgs, GoogleTagState]{
		ID: GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID),
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.ID != "" {
		t.Errorf("expected empty ID for removed tag, got %q", resp.ID)
	}
}

func TestGoogleTagRead_SiteNotFound(t *testing.T) {
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Requested resource not found"}`))
	})

	resp, err := (&GoogleTag{}).Read(context.Background(), infer.ReadRequest[GoogleTagArgs, GoogleTagState]{
		ID: GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID),
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.ID != "" {
		t.Errorf("expected empty ID for 404, got %q", resp.ID)
	}
}

func TestGoogleTagRead_ServerErrorPropagates(t *testing.T) {
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"not found in scope"}`))
	})

	_, err := (&GoogleTag{}).Read(context.Background(), infer.ReadRequest[GoogleTagArgs, GoogleTagState]{
		ID: GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID),
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestGoogleTagUpdate_UpsertsChangedFields(t *testing.T) {
	var gotMethod, gotPath string
	var gotReq GoogleTagsRequest
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.Unmarshal(body, &gotReq)
		_, _ = w.Write([]byte(`{"googleTagIds":[{"displayName":"New name","tagId":"G-1A2B3C4D5E","order":2}]}`))
	})

	resp, err := (&GoogleTag{}).Update(context.Background(), infer.UpdateRequest[GoogleTagArgs, GoogleTagState]{
		ID:     GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID),
		Inputs: googleTagArgs("New name", intPtr(2)),
		State:  GoogleTagState{GoogleTagArgs: googleTagArgs("Old", nil), EffectiveOrder: intPtr(1)},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != testGoogleTagPath {
		t.Errorf("expected PATCH %s, got %s %s", testGoogleTagPath, gotMethod, gotPath)
	}
	if len(gotReq.GoogleTagIDs) != 1 || gotReq.GoogleTagIDs[0].DisplayName != "New name" ||
		gotReq.GoogleTagIDs[0].Order == nil || *gotReq.GoogleTagIDs[0].Order != 2 {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	if resp.Output.DisplayName != "New name" || resp.Output.EffectiveOrder == nil || *resp.Output.EffectiveOrder != 2 {
		t.Errorf("unexpected output: %+v", resp.Output)
	}
}

func TestGoogleTagUpdate_DryRun(t *testing.T) {
	newGoogleTagServer(t, noAPICall(t))

	resp, err := (&GoogleTag{}).Update(context.Background(), infer.UpdateRequest[GoogleTagArgs, GoogleTagState]{
		Inputs: googleTagArgs("New", nil),
		State:  GoogleTagState{EffectiveOrder: intPtr(4)},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Update dry-run: %v", err)
	}
	if resp.Output.DisplayName != "New" || resp.Output.EffectiveOrder == nil || *resp.Output.EffectiveOrder != 4 {
		t.Errorf("unexpected preview output: %+v", resp.Output)
	}
}

func TestGoogleTagDelete_DeletesSingleTag(t *testing.T) {
	var gotMethod, gotPath string
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"googleTagIds":[]}`))
	})

	_, err := (&GoogleTag{}).Delete(context.Background(), infer.DeleteRequest[GoogleTagState]{
		ID: GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID),
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != testGoogleTagPath+"/"+testGoogleTagID {
		t.Errorf("expected DELETE %s/%s, got %s %s", testGoogleTagPath, testGoogleTagID, gotMethod, gotPath)
	}
}

func TestGoogleTagDelete_NotFoundIsSuccess(t *testing.T) {
	newGoogleTagServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := (&GoogleTag{}).Delete(context.Background(), infer.DeleteRequest[GoogleTagState]{
		ID: GenerateGoogleTagResourceID(testGoogleTagSiteID, testGoogleTagID),
	}); err != nil {
		t.Fatalf("Delete should treat 404 as success, got %v", err)
	}
}

func TestGoogleTagDelete_InvalidID(t *testing.T) {
	_, err := (&GoogleTag{}).Delete(context.Background(), infer.DeleteRequest[GoogleTagState]{ID: "bad"})
	if err == nil {
		t.Fatal("expected error for invalid resource ID")
	}
}

func TestGoogleTagDiff(t *testing.T) {
	base := googleTagArgs("Primary", nil)
	otherSite := googleTagArgs("Primary", nil)
	otherSite.SiteID = "6f0c8c9e1c9d440000e8d8c3"
	otherTag := googleTagArgs("Primary", nil)
	otherTag.TagID = "G-OTHER"
	sameTagLower := googleTagArgs("Primary", nil)
	sameTagLower.TagID = "g-1a2b3c4d5e"

	tests := []struct {
		name        string
		inputs      GoogleTagArgs
		state       GoogleTagArgs
		wantChanges bool
		wantReplace bool
		wantKeys    []string
	}{
		{"no change", base, base, false, false, nil},
		{"site change", otherSite, base, true, true, []string{"siteId"}},
		{"tag change", otherTag, base, true, true, []string{"tagId"}},
		{"tag case only", sameTagLower, base, false, false, nil},
		{"name change", googleTagArgs("New", nil), base, true, false, []string{"displayName"}},
		{"set order", googleTagArgs("Primary", intPtr(1)), base, true, false, []string{"order"}},
		{"same order", googleTagArgs("Primary", intPtr(1)), googleTagArgs("Primary", intPtr(1)), false, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := (&GoogleTag{}).Diff(context.Background(), infer.DiffRequest[GoogleTagArgs, GoogleTagState]{
				Inputs: tt.inputs,
				State:  GoogleTagState{GoogleTagArgs: tt.state},
			})
			if err != nil {
				t.Fatalf("Diff: %v", err)
			}
			if resp.HasChanges != tt.wantChanges || resp.DeleteBeforeReplace != tt.wantReplace {
				t.Fatalf("HasChanges=%v DeleteBeforeReplace=%v, want %v/%v",
					resp.HasChanges, resp.DeleteBeforeReplace, tt.wantChanges, tt.wantReplace)
			}
			for _, k := range tt.wantKeys {
				d, ok := resp.DetailedDiff[k]
				if !ok {
					t.Errorf("expected %q in detailed diff: %v", k, resp.DetailedDiff)
					continue
				}
				if tt.wantReplace && d.Kind != p.UpdateReplace {
					t.Errorf("expected %q to be UpdateReplace, got %v", k, d.Kind)
				}
				if !tt.wantReplace && d.Kind != p.Update {
					t.Errorf("expected %q to be Update, got %v", k, d.Kind)
				}
			}
			if len(resp.DetailedDiff) != len(tt.wantKeys) {
				t.Errorf("unexpected detailed diff: %v", resp.DetailedDiff)
			}
		})
	}
}
