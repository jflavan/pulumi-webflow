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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const testRedirectID = "42e1a2b7aa1a13f768a0042a"

func testRedirectResourceID() string {
	return GenerateRedirectResourceID(testSiteID, testRedirectID)
}

// ============================================================================
// Check
// ============================================================================

func TestRedirectCheck(t *testing.T) {
	resource := &Redirect{}

	t.Run("known invalid values fail", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{
			NewInputs: property.NewMap(map[string]property.Value{
				"siteId":          property.New("invalid"),
				"sourcePath":      property.New("old-page"),
				"destinationPath": property.New("/new page"),
				// statusCode is deprecated: any value is accepted without a failure.
				"statusCode": property.New(307.0),
			}),
		})
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		got := map[string]string{}
		for _, f := range resp.Failures {
			got[f.Property] = f.Reason
		}
		if len(got) != 3 || !containsStr(got["siteId"], "24-character") ||
			!containsStr(got["sourcePath"], "must start with '/'") ||
			!containsStr(got["destinationPath"], "invalid characters") {
			t.Errorf("unexpected failures: %+v", resp.Failures)
		}
		if resp.Inputs.StatusCode != 307 {
			t.Errorf("statusCode must be decoded untouched, got %d", resp.Inputs.StatusCode)
		}
	})

	t.Run("unknown values are skipped", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{
			NewInputs: property.NewMap(map[string]property.Value{
				"siteId":          property.New(property.Computed),
				"sourcePath":      property.New(property.Computed),
				"destinationPath": property.New("/new"),
			}),
		})
		if err != nil || len(resp.Failures) != 0 {
			t.Errorf("unknown inputs must not fail Check: failures=%+v err=%v", resp.Failures, err)
		}
	})

	t.Run("valid values pass without statusCode", func(t *testing.T) {
		resp, err := resource.Check(context.Background(), infer.CheckRequest{
			NewInputs: property.NewMap(map[string]property.Value{
				"siteId":          property.New(testSiteID),
				"sourcePath":      property.New("/old"),
				"destinationPath": property.New("/new"),
			}),
		})
		if err != nil || len(resp.Failures) != 0 {
			t.Errorf("valid inputs must pass Check: failures=%+v err=%v", resp.Failures, err)
		}
		if resp.Inputs.SiteID != testSiteID || resp.Inputs.SourcePath != "/old" || resp.Inputs.StatusCode != 0 {
			t.Errorf("inputs not decoded: %+v", resp.Inputs)
		}
	})
}

// ============================================================================
// Create
// ============================================================================

func TestRedirectCreate_ValidationErrors(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	redirect := &Redirect{}

	tests := []struct {
		name   string
		inputs RedirectArgs
		want   string
	}{
		{
			"invalid siteId",
			RedirectArgs{SiteID: "invalid", SourcePath: "/old", DestinationPath: "/new"},
			"validation failed",
		},
		{
			"missing sourcePath",
			RedirectArgs{SiteID: testSiteID, SourcePath: "", DestinationPath: "/new"},
			"sourcePath is required",
		},
		{
			"sourcePath without slash",
			RedirectArgs{SiteID: testSiteID, SourcePath: "old-page", DestinationPath: "/new"},
			"must start with '/'",
		},
		{
			"missing destinationPath",
			RedirectArgs{SiteID: testSiteID, SourcePath: "/old", DestinationPath: ""},
			"destinationPath is required",
		},
		{
			"destinationPath without slash",
			RedirectArgs{SiteID: testSiteID, SourcePath: "/old", DestinationPath: "new"},
			"must start with '/'",
		},
		{
			"sourcePath with query string",
			RedirectArgs{SiteID: testSiteID, SourcePath: "/page?query=value", DestinationPath: "/new"},
			"invalid characters",
		},
		{
			"destinationPath with hash",
			RedirectArgs{SiteID: testSiteID, SourcePath: "/old", DestinationPath: "/page#anchor"},
			"invalid characters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := redirect.Create(context.Background(), infer.CreateRequest[RedirectArgs]{Inputs: tt.inputs})
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !containsStr(err.Error(), tt.want) {
				t.Errorf("expected error containing %q, got %q", tt.want, err.Error())
			}
		})
	}
	if called {
		t.Error("API must not be called when validation fails")
	}
}

func TestRedirectCreate_DryRun_SkipsValidationAndAPI(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	redirect := &Redirect{}

	// siteId is unknown during preview when it comes from a Site resource output; it arrives zeroed.
	resp, err := redirect.Create(context.Background(), infer.CreateRequest[RedirectArgs]{
		Inputs: RedirectArgs{SiteID: "", SourcePath: "/old-page", DestinationPath: "/new-page", StatusCode: 301},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create (DryRun) failed: %v", err)
	}
	if called {
		t.Error("API must not be called in DryRun mode")
	}
	// An empty ID makes the framework present the ID and all outputs as unknown to dependents.
	if resp.ID != "" {
		t.Errorf("preview must not fabricate an ID, got %q", resp.ID)
	}
	if resp.Output.SourcePath != "/old-page" || resp.Output.DestinationPath != "/new-page" {
		t.Errorf("inputs not preserved in preview output: %+v", resp.Output)
	}
	if resp.Output.CreatedOn != "" {
		t.Errorf("createdOn must not be fabricated during preview, got %q", resp.Output.CreatedOn)
	}
}

func TestRedirectCreate_Success(t *testing.T) {
	var gotBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sites/"+testSiteID+"/redirects" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, `{"id":"`+testRedirectID+`","fromUrl":"/old-page","toUrl":"/new-page"}`)
	})
	redirect := &Redirect{}

	resp, err := redirect.Create(context.Background(), infer.CreateRequest[RedirectArgs]{
		// A deprecated statusCode (even a non-301 one) is accepted and ignored.
		Inputs: RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/new-page", StatusCode: 302},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if gotBody["fromUrl"] != "/old-page" || gotBody["toUrl"] != "/new-page" {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if _, ok := gotBody["statusCode"]; ok {
		t.Errorf("statusCode must never be sent to the API: %v", gotBody)
	}
	if resp.ID != testRedirectResourceID() {
		t.Errorf("expected resource ID %q, got %q", testRedirectResourceID(), resp.ID)
	}
	if resp.Output.StatusCode != 302 || resp.Output.SourcePath != "/old-page" {
		t.Errorf("inputs not preserved in state: %+v", resp.Output)
	}
	if resp.Output.CreatedOn != "" {
		t.Errorf("createdOn must be empty when the API does not return it, got %q", resp.Output.CreatedOn)
	}
}

func TestRedirectCreate_UsesCreatedOnFromAPI(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(
			t,
			w,
			http.StatusOK,
			`{"id":"`+testRedirectID+`","fromUrl":"/a","toUrl":"/b","createdOn":"2024-01-15T10:30:00Z"}`,
		)
	})
	resp, err := (&Redirect{}).Create(context.Background(), infer.CreateRequest[RedirectArgs]{
		Inputs: RedirectArgs{SiteID: testSiteID, SourcePath: "/a", DestinationPath: "/b"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if resp.Output.CreatedOn != "2024-01-15T10:30:00Z" {
		t.Errorf("expected createdOn from API, got %q", resp.Output.CreatedOn)
	}
}

func TestRedirectCreate_EmptyIDFromAPI(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, `{"fromUrl":"/a","toUrl":"/b"}`)
	})
	_, err := (&Redirect{}).Create(context.Background(), infer.CreateRequest[RedirectArgs]{
		Inputs: RedirectArgs{SiteID: testSiteID, SourcePath: "/a", DestinationPath: "/b"},
	})
	if err == nil || !strings.Contains(err.Error(), "empty redirect ID") {
		t.Errorf("expected empty redirect ID error, got: %v", err)
	}
}

// ============================================================================
// Read
// ============================================================================

func TestRedirectRead_FindsRedirectOnLaterPage(t *testing.T) {
	var offsets []int
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		switch offset {
		case "0", "":
			offsets = append(offsets, 0)
			writeJSON(
				t,
				w,
				http.StatusOK,
				`{"redirects":[{"id":"other-1","fromUrl":"/x","toUrl":"/y"}],"pagination":{"limit":1,"offset":0,"total":3}}`,
			)
		case "1":
			offsets = append(offsets, 1)
			writeJSON(
				t,
				w,
				http.StatusOK,
				`{"redirects":[{"id":"other-2","fromUrl":"/x2","toUrl":"/y2"}],"pagination":{"limit":1,"offset":1,"total":3}}`,
			)
		case "2":
			offsets = append(offsets, 2)
			writeJSON(t, w, http.StatusOK,
				`{"redirects":[{"id":"`+testRedirectID+`","fromUrl":"/old-page","toUrl":"/drifted"}],`+
					`"pagination":{"limit":1,"offset":2,"total":3}}`)
		default:
			t.Errorf("unexpected offset %q", offset)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	redirect := &Redirect{}

	resp, err := redirect.Read(context.Background(), infer.ReadRequest[RedirectArgs, RedirectState]{
		ID:     testRedirectResourceID(),
		Inputs: RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/new-page", StatusCode: 301},
		State: RedirectState{
			RedirectArgs: RedirectArgs{
				SiteID:          testSiteID,
				SourcePath:      "/old-page",
				DestinationPath: "/new-page",
				StatusCode:      302,
			},
			CreatedOn: "2024-01-15T10:30:00Z",
		},
	})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(offsets) != 3 {
		t.Errorf("expected three pages to be requested, got offsets %v", offsets)
	}
	if resp.ID != testRedirectResourceID() {
		t.Errorf("expected ID preserved, got %q", resp.ID)
	}
	if resp.Inputs.DestinationPath != "/drifted" || resp.Inputs.SourcePath != "/old-page" ||
		resp.Inputs.SiteID != testSiteID {
		t.Errorf("inputs not taken from API: %+v", resp.Inputs)
	}
	// statusCode is not part of the API object: the program's value wins over the stored one.
	if resp.Inputs.StatusCode != 301 || resp.State.StatusCode != 301 {
		t.Errorf("statusCode must be preserved from the program inputs, got inputs=%d state=%d",
			resp.Inputs.StatusCode, resp.State.StatusCode)
	}
	if resp.State.CreatedOn != "2024-01-15T10:30:00Z" {
		t.Errorf("createdOn must be preserved from prior state, got %q", resp.State.CreatedOn)
	}
}

func TestRedirectRead_StatusCodeHandling(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK,
			`{"redirects":[{"id":"`+testRedirectID+`","fromUrl":"/old","toUrl":"/new"}],`+
				`"pagination":{"limit":100,"offset":0,"total":1}}`)
	})

	t.Run("state value is kept when the program has none", func(t *testing.T) {
		resp, err := (&Redirect{}).Read(context.Background(), infer.ReadRequest[RedirectArgs, RedirectState]{
			ID:    testRedirectResourceID(),
			State: RedirectState{RedirectArgs: RedirectArgs{SiteID: testSiteID, StatusCode: 302}},
		})
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if resp.Inputs.StatusCode != 302 {
			t.Errorf("statusCode must be preserved from state, got %d", resp.Inputs.StatusCode)
		}
	})

	t.Run("import has no statusCode", func(t *testing.T) {
		resp, err := (&Redirect{}).Read(context.Background(), infer.ReadRequest[RedirectArgs, RedirectState]{
			ID: testRedirectResourceID(),
		})
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if resp.Inputs.StatusCode != 0 || resp.Inputs.SourcePath != "/old" || resp.Inputs.DestinationPath != "/new" {
			t.Errorf("unexpected imported inputs: %+v", resp.Inputs)
		}
		// An imported redirect must not diff against a program that still carries a statusCode.
		diff, err := (&Redirect{}).Diff(context.Background(), infer.DiffRequest[RedirectArgs, RedirectState]{
			Inputs: RedirectArgs{SiteID: testSiteID, SourcePath: "/old", DestinationPath: "/new", StatusCode: 301},
			State:  resp.State,
		})
		if err != nil || diff.HasChanges {
			t.Errorf("import followed by diff must be clean, got %+v (err %v)", diff, err)
		}
	})
}

func TestRedirectRead_MissingRedirectSignalsDeletion(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(
			t,
			w,
			http.StatusOK,
			`{"redirects":[{"id":"other","fromUrl":"/x","toUrl":"/y"}],"pagination":{"limit":100,"offset":0,"total":1}}`,
		)
	})
	resp, err := (&Redirect{}).Read(context.Background(), infer.ReadRequest[RedirectArgs, RedirectState]{
		ID: testRedirectResourceID(),
	})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if resp.ID != "" {
		t.Errorf("expected empty ID for a missing redirect, got %q", resp.ID)
	}
}

func TestRedirectRead_SiteNotFoundSignalsDeletion(t *testing.T) {
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, `{"message":"Requested resource not found"}`)
	})
	resp, err := (&Redirect{}).Read(context.Background(), infer.ReadRequest[RedirectArgs, RedirectState]{
		ID: testRedirectResourceID(),
	})
	if err != nil || resp.ID != "" {
		t.Errorf("expected empty ID without error for 404, got id=%q err=%v", resp.ID, err)
	}
}

func TestRedirectRead_OtherErrorsAreErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, status, `{"message":"not found"}`)
		})
		_, err := (&Redirect{}).Read(context.Background(), infer.ReadRequest[RedirectArgs, RedirectState]{
			ID: testRedirectResourceID(),
		})
		if err == nil {
			t.Errorf("status %d must surface as an error, not deletion", status)
		}
	}
}

func TestRedirectRead_InvalidIDs(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	for _, id := range []string{
		"bad", "not-a-site/redirects/" + testRedirectID, testSiteID + "/redirects/", testSiteID + "/redirects/a/b",
	} {
		_, err := (&Redirect{}).Read(context.Background(), infer.ReadRequest[RedirectArgs, RedirectState]{ID: id})
		if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
			t.Errorf("ID %q: expected invalid resource ID error, got: %v", id, err)
		}
	}
	if called {
		t.Error("API must not be called with an invalid ID")
	}
}

// ============================================================================
// Update
// ============================================================================

func TestRedirectUpdate_PatchesInPlace(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = readJSONBody(t, r)
		writeJSON(t, w, http.StatusOK, `{"id":"`+testRedirectID+`","fromUrl":"/old-page","toUrl":"/updated-page"}`)
	})
	redirect := &Redirect{}

	resp, err := redirect.Update(context.Background(), infer.UpdateRequest[RedirectArgs, RedirectState]{
		ID:     testRedirectResourceID(),
		Inputs: RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/updated-page", StatusCode: 302},
		State: RedirectState{
			RedirectArgs: RedirectArgs{
				SiteID:          testSiteID,
				SourcePath:      "/old-page",
				DestinationPath: "/new-page",
				StatusCode:      301,
			},
			CreatedOn: "2024-01-15T10:30:00Z",
		},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/v2/sites/"+testSiteID+"/redirects/"+testRedirectID {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotBody["toUrl"] != "/updated-page" {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if _, ok := gotBody["statusCode"]; ok {
		t.Errorf("statusCode must never be sent to the API: %v", gotBody)
	}
	if resp.Output.DestinationPath != "/updated-page" || resp.Output.StatusCode != 302 ||
		resp.Output.CreatedOn != "2024-01-15T10:30:00Z" {
		t.Errorf("unexpected state: %+v", resp.Output)
	}
}

func TestRedirectUpdate_DryRun(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	resp, err := (&Redirect{}).Update(context.Background(), infer.UpdateRequest[RedirectArgs, RedirectState]{
		ID:     testRedirectResourceID(),
		Inputs: RedirectArgs{SiteID: "", SourcePath: "/old-page", DestinationPath: "/updated-page"},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Update (DryRun) failed: %v", err)
	}
	if called {
		t.Error("API must not be called in DryRun mode")
	}
	if resp.Output.DestinationPath != "/updated-page" || resp.Output.CreatedOn != "" {
		t.Errorf("unexpected preview state: %+v", resp.Output)
	}
}

func TestRedirectUpdate_ValidationErrors(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	tests := []struct {
		name   string
		inputs RedirectArgs
		want   string
	}{
		{
			"invalid siteId",
			RedirectArgs{SiteID: "invalid", SourcePath: "/old", DestinationPath: "/new"},
			"validation failed",
		},
		{
			"missing destinationPath",
			RedirectArgs{SiteID: testSiteID, SourcePath: "/old", DestinationPath: ""},
			"destinationPath is required",
		},
		{
			"siteId mismatch",
			RedirectArgs{SiteID: testOtherSiteID, SourcePath: "/old", DestinationPath: "/new"},
			"does not match",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&Redirect{}).Update(context.Background(), infer.UpdateRequest[RedirectArgs, RedirectState]{
				ID:     testRedirectResourceID(),
				Inputs: tt.inputs,
			})
			if err == nil || !containsStr(err.Error(), tt.want) {
				t.Errorf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
	if called {
		t.Error("API must not be called when validation fails")
	}
}

// ============================================================================
// Delete
// ============================================================================

func TestRedirectDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := (&Redirect{}).Delete(
		context.Background(), infer.DeleteRequest[RedirectState]{ID: testRedirectResourceID()},
	); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v2/sites/"+testSiteID+"/redirects/"+testRedirectID {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
}

func TestRedirectDelete_InvalidID(t *testing.T) {
	called := false
	mockWebflowAPI(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	_, err := (&Redirect{}).Delete(context.Background(), infer.DeleteRequest[RedirectState]{ID: "bad/redirects/../x"})
	if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
		t.Errorf("expected invalid resource ID error, got: %v", err)
	}
	if called {
		t.Error("API must not be called with an invalid ID")
	}
}

// ============================================================================
// Diff
// ============================================================================

func redirectDiff(t *testing.T, inputs, state RedirectArgs) infer.DiffResponse {
	t.Helper()
	resp, err := (&Redirect{}).Diff(context.Background(), infer.DiffRequest[RedirectArgs, RedirectState]{
		Inputs: inputs,
		State:  RedirectState{RedirectArgs: state},
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	return resp
}

func TestRedirectDiff_NoChanges(t *testing.T) {
	args := RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/new-page", StatusCode: 301}
	resp := redirectDiff(t, args, args)
	if resp.HasChanges || len(resp.DetailedDiff) != 0 {
		t.Errorf("expected no changes, got %+v", resp)
	}
}

func TestRedirectDiff_SiteIDChangeReplaces(t *testing.T) {
	resp := redirectDiff(t,
		RedirectArgs{SiteID: testOtherSiteID, SourcePath: "/old-page", DestinationPath: "/new-page"},
		RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/new-page"})
	if !resp.HasChanges || !resp.DeleteBeforeReplace || resp.DetailedDiff["siteId"].Kind != p.UpdateReplace {
		t.Errorf("expected siteId replacement, got %+v", resp)
	}
}

func TestRedirectDiff_SourcePathChangeReplaces(t *testing.T) {
	resp := redirectDiff(t,
		RedirectArgs{SiteID: testSiteID, SourcePath: "/new-old-page", DestinationPath: "/new-page"},
		RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/new-page"})
	if !resp.HasChanges || !resp.DeleteBeforeReplace || resp.DetailedDiff["sourcePath"].Kind != p.UpdateReplace {
		t.Errorf("expected sourcePath replacement, got %+v", resp)
	}
}

func TestRedirectDiff_DestinationUpdatesInPlace(t *testing.T) {
	resp := redirectDiff(t,
		RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/updated-page"},
		RedirectArgs{SiteID: testSiteID, SourcePath: "/old-page", DestinationPath: "/new-page"})
	if !resp.HasChanges || resp.DeleteBeforeReplace {
		t.Errorf("expected in-place update, got %+v", resp)
	}
	if resp.DetailedDiff["destinationPath"].Kind != p.Update || len(resp.DetailedDiff) != 1 {
		t.Errorf("expected only destinationPath as Update, got %+v", resp.DetailedDiff)
	}
}

func TestRedirectDiff_StatusCodeNeverDiffs(t *testing.T) {
	base := RedirectArgs{SiteID: testSiteID, SourcePath: "/old", DestinationPath: "/new"}
	cases := []struct {
		name          string
		inputs, state int
	}{
		{"301 vs 302", 301, 302},
		{"0 in state (imported)", 301, 0},
		{"0 in program (removed)", 0, 301},
		{"undocumented value", 307, 301},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			inputs, state := base, base
			inputs.StatusCode, state.StatusCode = tt.inputs, tt.state
			resp := redirectDiff(t, inputs, state)
			if resp.HasChanges || len(resp.DetailedDiff) != 0 {
				t.Errorf("statusCode is deprecated and must never diff, got %+v", resp)
			}
		})
	}
}

func TestRedirectDiff_EmptyStateNeedsCreate(t *testing.T) {
	resp := redirectDiff(t,
		RedirectArgs{SiteID: testSiteID, SourcePath: "/contact", DestinationPath: "/contact-us"},
		RedirectArgs{})
	if !resp.HasChanges {
		t.Error("expected changes against an empty state")
	}
}
