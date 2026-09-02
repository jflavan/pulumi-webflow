// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// TestValidateFieldData tests the ValidateFieldData function.
func TestValidateFieldData(t *testing.T) {
	tests := []struct {
		name      string
		fieldData map[string]interface{}
		wantErr   bool
	}{
		{"valid field data", map[string]interface{}{"name": "Test Item", "slug": "test-item"}, false},
		{"nil field data", nil, true},
		{"empty field data", map[string]interface{}{}, true},
		{"multiple fields", map[string]interface{}{"name": "Test", "slug": "test", "content": "Content"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldData(tt.fieldData)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFieldData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateCmsLocaleID tests the optional locale ID format check.
func TestValidateCmsLocaleID(t *testing.T) {
	for _, ok := range []string{"", testLocaleID} {
		if err := ValidateCmsLocaleID(ok); err != nil {
			t.Errorf("ValidateCmsLocaleID(%q) unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"locale-1", "653FD9AF6A07FC9CFD7A5E57", "abc", "a/b"} {
		if err := ValidateCmsLocaleID(bad); err == nil || !strings.Contains(err.Error(), "cmsLocaleId") {
			t.Errorf("ValidateCmsLocaleID(%q) expected cmsLocaleId error, got %v", bad, err)
		}
	}
}

// TestValidateItemID tests the path-segment guard for item IDs.
func TestValidateItemID(t *testing.T) {
	if err := ValidateItemID(testItemID); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	for _, bad := range []string{"", "a/b", "..", "x?y=1"} {
		if err := ValidateItemID(bad); err == nil {
			t.Errorf("ValidateItemID(%q) expected error", bad)
		}
	}
}

// TestGenerateCollectionItemResourceID tests the GenerateCollectionItemResourceID function.
func TestGenerateCollectionItemResourceID(t *testing.T) {
	got := GenerateCollectionItemResourceID(testCollectionID, testItemID)
	if want := testCollectionID + "/items/" + testItemID; got != want {
		t.Errorf("GenerateCollectionItemResourceID() = %v, want %v", got, want)
	}
}

// TestExtractIDsFromCollectionItemResourceID tests the ExtractIDsFromCollectionItemResourceID function.
func TestExtractIDsFromCollectionItemResourceID(t *testing.T) {
	tests := []struct {
		name             string
		resourceID       string
		wantCollectionID string
		wantItemID       string
		wantErr          bool
	}{
		{"valid resource ID", testCollectionID + "/items/" + testItemID, testCollectionID, testItemID, false},
		{
			"itemID with slashes",
			testCollectionID + "/items/6f1d9d0f/special/item",
			testCollectionID,
			"6f1d9d0f/special/item",
			false,
		},
		{"empty resource ID", "", "", "", true},
		{"invalid format - no items segment", testCollectionID + "/redirects/" + testItemID, "", "", true},
		{"invalid format - too few parts", testCollectionID + "/items", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCollectionID, gotItemID, err := ExtractIDsFromCollectionItemResourceID(tt.resourceID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (gotCollectionID != tt.wantCollectionID || gotItemID != tt.wantItemID) {
				t.Errorf("got (%q, %q), want (%q, %q)", gotCollectionID, gotItemID, tt.wantCollectionID, tt.wantItemID)
			}
		})
	}
}

var (
	itemResourceID = testCollectionID + "/items/" + testItemID
	itemsPath      = "/v2/collections/" + testCollectionID + "/items"
	itemPath       = itemsPath + "/" + testItemID
	itemLivePath   = itemPath + "/live"
	publishPath    = itemsPath + "/publish"
)

// apiItem returns a typical item payload as the Webflow API reports it.
func apiItem(lastPublished string) map[string]interface{} {
	return map[string]interface{}{
		"id": testItemID, "cmsLocaleId": "", "isArchived": false, "isDraft": true,
		"createdOn": "2024-01-01T00:00:00Z", "lastUpdated": "2024-01-02T00:00:00Z", "lastPublished": lastPublished,
		"fieldData": map[string]interface{}{"name": "Test Item", "slug": "test-item", "body": "Hello", "featured": false},
	}
}

// =============================================================================
// CollectionItem resource: Create
// =============================================================================

func TestCollectionItemResource_Create(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPost, itemsPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, apiItem(""))
	})

	inputs := CollectionItemArgs{
		CollectionID: testCollectionID,
		FieldData:    map[string]interface{}{"name": "Test Item", "slug": "test-item"},
		IsDraft:      ptrBool(true),
	}
	resp, err := (&CollectionItemResource{}).Create(
		context.Background(),
		infer.CreateRequest[CollectionItemArgs]{Inputs: inputs},
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.ID != itemResourceID {
		t.Errorf("ID = %q, want %q", resp.ID, itemResourceID)
	}
	out := resp.Output
	if out.ItemID != testItemID || out.CreatedOn != "2024-01-01T00:00:00Z" || out.LastUpdated != "2024-01-02T00:00:00Z" {
		t.Errorf("unexpected output: %+v", out)
	}
	if !reflect.DeepEqual(out.FieldData, inputs.FieldData) {
		t.Errorf("state fieldData must keep the user's view, got %v", out.FieldData)
	}
	if out.IsDraft == nil || !*out.IsDraft || out.IsArchived == nil || *out.IsArchived {
		t.Errorf("flags should come from the API: %+v", out)
	}
	if !out.Live {
		// live defaults to false
	}

	calls := mock.requests()
	if len(calls) != 1 || calls[0].Method != http.MethodPost {
		t.Fatalf("expected exactly one POST (no publish), got %+v", calls)
	}
	body := calls[0].Body
	fd, _ := body["fieldData"].(map[string]interface{})
	if fd["name"] != "Test Item" || fd["slug"] != "test-item" || body["isDraft"] != true {
		t.Errorf("unexpected POST body: %v", body)
	}
	if _, ok := body["isArchived"]; ok {
		t.Errorf("omitted isArchived must not be sent: %v", body)
	}
}

func TestCollectionItemResource_Create_LivePublishesAndReadsLiveCopy(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPost, itemsPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, apiItem(""))
	})
	mock.handle(http.MethodPost, publishPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, CollectionItemPublishResponse{PublishedItemIDs: []string{testItemID}})
	})
	mock.handle(http.MethodGet, itemLivePath, func(w http.ResponseWriter, r *http.Request) {
		live := apiItem("2024-01-03T00:00:00Z")
		live["isDraft"] = false
		writeCMSJSON(w, http.StatusOK, live)
	})

	resp, err := (&CollectionItemResource{}).Create(context.Background(), infer.CreateRequest[CollectionItemArgs]{
		Inputs: CollectionItemArgs{
			CollectionID: testCollectionID,
			FieldData:    map[string]interface{}{"name": "Test Item", "slug": "test-item"},
			Live:         true,
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Output.LastPublished != "2024-01-03T00:00:00Z" {
		t.Errorf("lastPublished should come from the live copy, got %q", resp.Output.LastPublished)
	}
	if resp.Output.IsDraft == nil || *resp.Output.IsDraft {
		t.Errorf("isDraft should reflect the published copy: %+v", resp.Output)
	}

	calls := mock.requests()
	if len(calls) != 3 {
		t.Fatalf("expected POST item, POST publish, GET live; got %+v", calls)
	}
	if calls[1].Method != http.MethodPost || calls[1].Path != publishPath {
		t.Errorf("second call should be the publish, got %+v", calls[1])
	}
	ids, _ := calls[1].Body["itemIds"].([]interface{})
	if len(ids) != 1 || ids[0] != testItemID {
		t.Errorf("publish body should carry itemIds [%s], got %v", testItemID, calls[1].Body)
	}
	if calls[2].Method != http.MethodGet || calls[2].Path != itemLivePath {
		t.Errorf("third call should read the live item, got %+v", calls[2])
	}
}

func TestCollectionItemResource_Create_LiveWithLocale(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPost, itemsPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, apiItem(""))
	})
	mock.handle(http.MethodPost, publishPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, CollectionItemPublishResponse{PublishedItemIDs: []string{testItemID}})
	})
	mock.handle(http.MethodGet, itemLivePath, func(w http.ResponseWriter, r *http.Request) {
		// Live copy not yet available: provider must keep the staged response.
		writeCMSJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
	})

	resp, err := (&CollectionItemResource{}).Create(context.Background(), infer.CreateRequest[CollectionItemArgs]{
		Inputs: CollectionItemArgs{
			CollectionID: testCollectionID, CmsLocaleID: testLocaleID, Live: true,
			FieldData: map[string]interface{}{"name": "Test Item", "slug": "test-item"},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Output.ItemID != testItemID {
		t.Errorf("staged response should be kept when the live copy is not available yet: %+v", resp.Output)
	}

	publish := mock.callsTo(http.MethodPost, publishPath)
	if len(publish) != 1 {
		t.Fatalf("expected one publish call")
	}
	items, _ := publish[0].Body["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("publish with a locale must use items[{id, cmsLocaleIds}], got %v", publish[0].Body)
	}
	target, _ := items[0].(map[string]interface{})
	locales, _ := target["cmsLocaleIds"].([]interface{})
	if target["id"] != testItemID || len(locales) != 1 || locales[0] != testLocaleID {
		t.Errorf("unexpected publish target: %v", target)
	}
	live := mock.callsTo(http.MethodGet, itemLivePath)
	if len(live) != 1 || live[0].Query.Get("cmsLocaleId") != testLocaleID {
		t.Errorf("live read must pass cmsLocaleId as a query parameter, got %+v", live)
	}
}

func TestCollectionItemResource_Create_LivePublishFailure(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPost, itemsPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, apiItem(""))
	})
	mock.handle(http.MethodPost, publishPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, CollectionItemPublishResponse{Errors: []string{"Staging item ID not found"}})
	})

	_, err := (&CollectionItemResource{}).Create(context.Background(), infer.CreateRequest[CollectionItemArgs]{
		Inputs: CollectionItemArgs{
			CollectionID: testCollectionID, Live: true,
			FieldData: map[string]interface{}{"name": "Test Item", "slug": "test-item"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "Staging item ID not found") ||
		!strings.Contains(err.Error(), itemResourceID) {
		t.Fatalf("expected publish error mentioning the staged item ID, got %v", err)
	}
}

func TestCollectionItemResource_Create_DryRunSkipsValidationAndAPI(t *testing.T) {
	mock := newCMSMock(t)
	resp, err := (&CollectionItemResource{}).Create(context.Background(), infer.CreateRequest[CollectionItemArgs]{
		Inputs: CollectionItemArgs{CollectionID: "", FieldData: nil},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create(DryRun) error = %v", err)
	}
	if len(mock.requests()) != 0 {
		t.Errorf("dry run must not call the API")
	}
	if resp.ID != "" {
		t.Errorf("dry run must return an empty ID so dependents see it as unknown, got %q", resp.ID)
	}
	if resp.Output.ItemID != "" || resp.Output.CreatedOn != "" || resp.Output.LastUpdated != "" {
		t.Errorf("dry run must not fabricate server-assigned outputs: %+v", resp.Output)
	}
}

// =============================================================================
// CollectionItem resource: Check
// =============================================================================

func TestCollectionItemResource_Check(t *testing.T) {
	check := func(t *testing.T, inputs map[string]property.Value) infer.CheckResponse[CollectionItemArgs] {
		t.Helper()
		resp, err := (&CollectionItemResource{}).Check(
			context.Background(), infer.CheckRequest{NewInputs: property.NewMap(inputs)},
		)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		return resp
	}
	fieldData := property.New[map[string]property.Value]
	reasons := func(resp infer.CheckResponse[CollectionItemArgs]) string {
		parts := make([]string, 0, len(resp.Failures))
		for _, f := range resp.Failures {
			parts = append(parts, f.Property+": "+f.Reason)
		}
		return strings.Join(parts, " | ")
	}

	t.Run("valid inputs pass", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID),
			"cmsLocaleId":  property.New(testLocaleID),
			"fieldData": fieldData(map[string]property.Value{
				"name": property.New("Test Item"), "slug": property.New("test-item"), "views": property.New(3.0),
			}),
		})
		if len(resp.Failures) != 0 {
			t.Errorf("unexpected failures: %+v", resp.Failures)
		}
		if resp.Inputs.CollectionID != testCollectionID || resp.Inputs.FieldData["name"] != "Test Item" {
			t.Errorf("inputs not decoded: %+v", resp.Inputs)
		}
	})

	t.Run("malformed collectionId and cmsLocaleId fail", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New("bad"), "cmsLocaleId": property.New("locale-1"),
			"fieldData": fieldData(map[string]property.Value{
				"name": property.New("Test Item"), "slug": property.New("test-item"),
			}),
		})
		got := reasons(resp)
		if len(resp.Failures) != 2 || !strings.Contains(got, "collectionId:") || !strings.Contains(got, "cmsLocaleId:") {
			t.Errorf("expected collectionId and cmsLocaleId failures, got %s", got)
		}
	})

	t.Run("fieldData without name and slug fails", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID),
			"fieldData":    fieldData(map[string]property.Value{"body": property.New("text")}),
		})
		got := reasons(resp)
		if len(resp.Failures) != 2 || !strings.Contains(got, "fieldData.name") || !strings.Contains(got, "fieldData.slug") {
			t.Errorf("expected name and slug failures, got %s", got)
		}
		for _, f := range resp.Failures {
			if f.Property != "fieldData" {
				t.Errorf("failure property = %q, want fieldData", f.Property)
			}
		}
	})

	t.Run("empty name fails", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID),
			"fieldData":    fieldData(map[string]property.Value{"name": property.New(""), "slug": property.New("s")}),
		})
		if got := reasons(resp); len(resp.Failures) != 1 || !strings.Contains(got, "fieldData.name") {
			t.Errorf("expected an empty-name failure, got %s", got)
		}
	})

	t.Run("unknown values are skipped", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(property.Computed), "cmsLocaleId": property.New(property.Computed),
			"fieldData": property.New(property.Computed),
		})
		if len(resp.Failures) != 0 {
			t.Errorf("computed inputs must not fail: %+v", resp.Failures)
		}
	})

	t.Run("unknown fieldData values are accepted when the keys are present", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID),
			"fieldData": fieldData(map[string]property.Value{
				"name": property.New(property.Computed), "slug": property.New(property.Computed),
			}),
		})
		if len(resp.Failures) != 0 {
			t.Errorf("computed name/slug must not fail: %+v", resp.Failures)
		}
	})
}

func TestCollectionItemResource_Create_ValidationErrors(t *testing.T) {
	mock := newCMSMock(t)
	for _, tt := range []struct {
		name   string
		inputs CollectionItemArgs
		want   string
	}{
		{
			"invalid collectionId",
			CollectionItemArgs{CollectionID: "bad", FieldData: map[string]interface{}{"name": "x"}},
			"collectionId",
		},
		{"empty fieldData", CollectionItemArgs{CollectionID: testCollectionID}, "fieldData is required"},
		{
			"invalid cmsLocaleId",
			CollectionItemArgs{
				CollectionID: testCollectionID, CmsLocaleID: "locale-1",
				FieldData: map[string]interface{}{"name": "x", "slug": "x"},
			},
			"cmsLocaleId",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&CollectionItemResource{}).Create(
				context.Background(),
				infer.CreateRequest[CollectionItemArgs]{Inputs: tt.inputs},
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
	if len(mock.requests()) != 0 {
		t.Errorf("validation failures must not call the API")
	}
}

// =============================================================================
// CollectionItem resource: Update
// =============================================================================

func updateRequest(
	stateFieldData, inputFieldData map[string]interface{},
) infer.UpdateRequest[CollectionItemArgs, CollectionItemState] {
	return infer.UpdateRequest[CollectionItemArgs, CollectionItemState]{
		ID: itemResourceID,
		State: CollectionItemState{
			CollectionItemArgs: CollectionItemArgs{CollectionID: testCollectionID, FieldData: stateFieldData},
			ItemID:             testItemID, CreatedOn: "2024-01-01T00:00:00Z", LastPublished: "2024-01-01T12:00:00Z",
		},
		Inputs: CollectionItemArgs{CollectionID: testCollectionID, FieldData: inputFieldData, IsDraft: ptrBool(false)},
	}
}

func TestCollectionItemResource_Update_StripsUnchangedSlug(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPatch, itemPath, func(w http.ResponseWriter, r *http.Request) {
		item := apiItem("")
		item["lastUpdated"] = "2024-02-01T00:00:00Z"
		writeCMSJSON(w, http.StatusOK, item)
	})

	req := updateRequest(
		map[string]interface{}{"name": "Old Name", "slug": "test-item"},
		map[string]interface{}{"name": "Updated Name", "slug": "test-item"},
	)
	resp, err := (&CollectionItemResource{}).Update(context.Background(), req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	calls := mock.requests()
	if len(calls) != 1 || calls[0].Method != http.MethodPatch || calls[0].Path != itemPath {
		t.Fatalf("expected exactly one PATCH %s, got %+v", itemPath, calls)
	}
	body := calls[0].Body
	fd, _ := body["fieldData"].(map[string]interface{})
	if _, hasSlug := fd["slug"]; hasSlug {
		t.Errorf("unchanged slug must be stripped from the PATCH payload, got %v", fd)
	}
	if fd["name"] != "Updated Name" {
		t.Errorf("name must be sent, got %v", fd)
	}
	if v, ok := body["isDraft"]; !ok || v != false {
		t.Errorf("isDraft:false must be present in the PATCH body, got %v", body)
	}

	out := resp.Output
	if out.ItemID != testItemID || out.CreatedOn != "2024-01-01T00:00:00Z" || out.LastUpdated != "2024-02-01T00:00:00Z" {
		t.Errorf("unexpected output: %+v", out)
	}
	if out.LastPublished != "2024-01-01T12:00:00Z" {
		t.Errorf("lastPublished should be preserved when the API reports none, got %q", out.LastPublished)
	}
	if !reflect.DeepEqual(out.FieldData, req.Inputs.FieldData) {
		t.Errorf("state fieldData must keep the user's view (including the slug), got %v", out.FieldData)
	}
}

func TestCollectionItemResource_Update_SlugHandling(t *testing.T) {
	tests := []struct {
		name         string
		stateData    map[string]interface{}
		inputData    map[string]interface{}
		wantSlugSent bool
		wantSlug     interface{}
	}{
		{
			"changed slug is sent",
			map[string]interface{}{"name": "n", "slug": "old-slug"},
			map[string]interface{}{"name": "n", "slug": "new-slug"},
			true, "new-slug",
		},
		{
			"newly added slug is sent",
			map[string]interface{}{"name": "n"},
			map[string]interface{}{"name": "n", "slug": "new-slug"},
			true, "new-slug",
		},
		{
			"removed slug is not sent",
			map[string]interface{}{"name": "n", "slug": "old-slug"},
			map[string]interface{}{"name": "n"},
			false, nil,
		},
		{
			"non-string slug is passed through",
			map[string]interface{}{"name": "n", "slug": float64(123)},
			map[string]interface{}{"name": "n", "slug": float64(123)},
			true, float64(123),
		},
		{
			"mismatched slug types are passed through",
			map[string]interface{}{"name": "n", "slug": "old"},
			map[string]interface{}{"name": "n", "slug": float64(456)},
			true, float64(456),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newCMSMock(t)
			mock.handle(http.MethodPatch, itemPath, func(w http.ResponseWriter, r *http.Request) {
				writeCMSJSON(w, http.StatusOK, apiItem(""))
			})
			if _, err := (&CollectionItemResource{}).Update(
				context.Background(), updateRequest(tt.stateData, tt.inputData),
			); err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			fd, _ := mock.callsTo(http.MethodPatch, itemPath)[0].Body["fieldData"].(map[string]interface{})
			slug, sent := fd["slug"]
			if sent != tt.wantSlugSent {
				t.Errorf("slug sent = %v, want %v (fieldData %v)", sent, tt.wantSlugSent, fd)
			}
			if sent && slug != tt.wantSlug {
				t.Errorf("slug = %v, want %v", slug, tt.wantSlug)
			}
			if _, hasName := fd["name"]; !hasName {
				t.Errorf("name must always be sent")
			}
		})
	}
}

func TestCollectionItemResource_Update_LivePublishes(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPatch, itemPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, apiItem(""))
	})
	mock.handle(http.MethodPost, publishPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusAccepted, CollectionItemPublishResponse{PublishedItemIDs: []string{testItemID}})
	})
	mock.handle(http.MethodGet, itemLivePath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, apiItem("2024-03-01T00:00:00Z"))
	})

	req := updateRequest(
		map[string]interface{}{"name": "n", "slug": "s"},
		map[string]interface{}{"name": "n2", "slug": "s"},
	)
	req.Inputs.Live = true
	resp, err := (&CollectionItemResource{}).Update(context.Background(), req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if resp.Output.LastPublished != "2024-03-01T00:00:00Z" {
		t.Errorf("lastPublished should come from the live copy, got %q", resp.Output.LastPublished)
	}
	calls := mock.requests()
	if len(calls) != 3 || calls[0].Method != http.MethodPatch || calls[1].Path != publishPath ||
		calls[2].Path != itemLivePath {
		t.Errorf("expected PATCH, publish, GET live; got %+v", calls)
	}
}

func TestCollectionItemResource_Update_DryRunPreservesTimestamps(t *testing.T) {
	mock := newCMSMock(t)
	req := updateRequest(map[string]interface{}{"name": "n"}, map[string]interface{}{"name": "n2"})
	req.State.LastUpdated = "2024-01-02T00:00:00Z"
	req.DryRun = true
	resp, err := (&CollectionItemResource{}).Update(context.Background(), req)
	if err != nil {
		t.Fatalf("Update(DryRun) error = %v", err)
	}
	if len(mock.requests()) != 0 {
		t.Errorf("dry run must not call the API")
	}
	if resp.Output.LastUpdated != "2024-01-02T00:00:00Z" || resp.Output.ItemID != testItemID {
		t.Errorf("dry run must carry over server-managed fields, got %+v", resp.Output)
	}
}

func TestCollectionItemResource_Update_APIError(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPatch, itemPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusBadRequest, map[string]string{"message": "Unique value is already in database"})
	})
	_, err := (&CollectionItemResource{}).Update(context.Background(),
		updateRequest(map[string]interface{}{"name": "n"}, map[string]interface{}{"name": "n2"}))
	if err == nil || !strings.Contains(err.Error(), "failed to update collection item") {
		t.Fatalf("expected update error, got %v", err)
	}
}

// =============================================================================
// CollectionItem resource: Read
// =============================================================================

func TestCollectionItemResource_Read(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, itemPath, func(w http.ResponseWriter, r *http.Request) {
		item := apiItem("2024-01-05T00:00:00Z")
		item["cmsLocaleId"] = "locale-1"
		item["isArchived"] = true
		writeCMSJSON(w, http.StatusOK, item)
	})

	resp, err := (&CollectionItemResource{}).Read(
		context.Background(),
		infer.ReadRequest[CollectionItemArgs, CollectionItemState]{
			ID: itemResourceID,
			Inputs: CollectionItemArgs{
				CollectionID: testCollectionID,
				FieldData:    map[string]interface{}{"name": "Stale", "slug": "test-item"},
				IsArchived:   ptrBool(false), // explicit: refreshed from API
				// IsDraft omitted: must stay omitted
				CmsLocaleID: "locale-1",
			},
		},
	)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if resp.ID != itemResourceID {
		t.Errorf("ID = %q", resp.ID)
	}

	calls := mock.callsTo(http.MethodGet, itemPath)
	if len(calls) != 1 || calls[0].Query.Get("cmsLocaleId") != "locale-1" {
		t.Errorf("GET must pass cmsLocaleId as a query parameter, got %+v", calls)
	}

	if resp.Inputs.IsDraft != nil {
		t.Errorf("Read must not populate an omitted isDraft input, got %v", *resp.Inputs.IsDraft)
	}
	if resp.Inputs.IsArchived == nil || !*resp.Inputs.IsArchived {
		t.Errorf("explicit isArchived should be refreshed from the API, got %v", resp.Inputs.IsArchived)
	}
	wantFieldData := map[string]interface{}{"name": "Test Item", "slug": "test-item"}
	if !reflect.DeepEqual(resp.Inputs.FieldData, wantFieldData) {
		t.Errorf("fieldData should be projected onto the managed keys, got %v", resp.Inputs.FieldData)
	}
	if !reflect.DeepEqual(resp.State.FieldData, wantFieldData) {
		t.Errorf("state fieldData should match the inputs view, got %v", resp.State.FieldData)
	}
	if resp.State.IsDraft == nil || !*resp.State.IsDraft {
		t.Errorf("state should carry the API draft flag: %+v", resp.State)
	}
	if resp.State.LastPublished != "2024-01-05T00:00:00Z" || resp.State.ItemID != testItemID ||
		resp.State.CmsLocaleID != "locale-1" {
		t.Errorf("unexpected state: %+v", resp.State)
	}
}

func TestCollectionItemResource_Read_ImportReturnsFullFieldData(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, itemPath, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, apiItem(""))
	})
	resp, err := (&CollectionItemResource{}).Read(
		context.Background(),
		infer.ReadRequest[CollectionItemArgs, CollectionItemState]{ID: itemResourceID},
	)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(resp.Inputs.FieldData) != 4 || resp.Inputs.CollectionID != testCollectionID {
		t.Errorf("import should return the full fieldData, got %+v", resp.Inputs)
	}
	if got := mock.callsTo(http.MethodGet, itemPath); len(got) != 1 || got[0].Query.Has("cmsLocaleId") {
		t.Errorf("no cmsLocaleId query expected, got %+v", got)
	}
}

func TestCollectionItemResource_Read_Live(t *testing.T) {
	t.Run("reads the live endpoint", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(http.MethodGet, itemLivePath, func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusOK, apiItem("2024-01-05T00:00:00Z"))
		})
		resp, err := (&CollectionItemResource{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionItemArgs, CollectionItemState]{
				ID: itemResourceID, Inputs: CollectionItemArgs{CollectionID: testCollectionID, Live: true},
			},
		)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.State.LastPublished != "2024-01-05T00:00:00Z" || !resp.Inputs.Live {
			t.Errorf("unexpected result: %+v", resp)
		}
		if calls := mock.requests(); len(calls) != 1 || calls[0].Path != itemLivePath {
			t.Errorf("expected a single GET %s, got %+v", itemLivePath, calls)
		}
	})

	t.Run("falls back to the staged item when not published", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(http.MethodGet, itemLivePath, func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
		})
		mock.handle(http.MethodGet, itemPath, func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusOK, apiItem(""))
		})
		resp, err := (&CollectionItemResource{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionItemArgs, CollectionItemState]{
				ID: itemResourceID, Inputs: CollectionItemArgs{CollectionID: testCollectionID, Live: true},
			},
		)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.ID != itemResourceID || resp.State.ItemID != testItemID {
			t.Errorf("expected the staged item, got %+v", resp)
		}
	})
}

func TestCollectionItemResource_Read_NotFoundAndErrors(t *testing.T) {
	t.Run("404 returns empty ID", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(http.MethodGet, itemPath, func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
		})
		resp, err := (&CollectionItemResource{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionItemArgs, CollectionItemState]{ID: itemResourceID},
		)
		if err != nil || resp.ID != "" {
			t.Errorf("expected empty ID and nil error, got ID=%q err=%v", resp.ID, err)
		}
	})

	t.Run("500 is an error", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(http.MethodGet, itemPath, func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusInternalServerError, map[string]string{"message": "item not found in index"})
		})
		if _, err := (&CollectionItemResource{}).Read(
			context.Background(), infer.ReadRequest[CollectionItemArgs, CollectionItemState]{ID: itemResourceID},
		); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("invalid resource ID", func(t *testing.T) {
		mock := newCMSMock(t)
		for _, id := range []string{"", "bad/items/" + testItemID, testCollectionID + "/items/a/b"} {
			if _, err := (&CollectionItemResource{}).Read(
				context.Background(), infer.ReadRequest[CollectionItemArgs, CollectionItemState]{ID: id},
			); err == nil {
				t.Errorf("Read(%q) expected error", id)
			}
		}
		if len(mock.requests()) != 0 {
			t.Errorf("invalid IDs must be rejected before any API call")
		}
	})
}

// =============================================================================
// CollectionItem resource: Delete
// =============================================================================

func TestCollectionItemResource_Delete(t *testing.T) {
	t.Run("staged item", func(t *testing.T) {
		for _, tt := range []struct {
			name    string
			status  int
			wantErr bool
		}{
			{"204", http.StatusNoContent, false},
			{"404 idempotent", http.StatusNotFound, false},
			{"401", http.StatusUnauthorized, true},
		} {
			t.Run(tt.name, func(t *testing.T) {
				mock := newCMSMock(t)
				mock.handle(http.MethodDelete, itemPath, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status) })
				_, err := (&CollectionItemResource{}).Delete(
					context.Background(),
					infer.DeleteRequest[CollectionItemState]{ID: itemResourceID},
				)
				if (err != nil) != tt.wantErr {
					t.Fatalf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				}
				if calls := mock.requests(); len(calls) != 1 || calls[0].Path != itemPath {
					t.Errorf("expected a single DELETE %s, got %+v", itemPath, calls)
				}
			})
		}
	})

	t.Run("live item is unpublished first", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(
			http.MethodDelete,
			itemLivePath,
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		)
		mock.handle(
			http.MethodDelete,
			itemPath,
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		)
		_, err := (&CollectionItemResource{}).Delete(context.Background(), infer.DeleteRequest[CollectionItemState]{
			ID: itemResourceID, State: CollectionItemState{CollectionItemArgs: CollectionItemArgs{Live: true}},
		})
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		calls := mock.requests()
		if len(calls) != 2 || calls[0].Path != itemLivePath || calls[1].Path != itemPath ||
			calls[0].Method != http.MethodDelete || calls[1].Method != http.MethodDelete {
			t.Errorf("expected DELETE live then DELETE staged, got %+v", calls)
		}
		for _, c := range calls {
			if c.Query.Has("cmsLocaleId") {
				t.Errorf("no cmsLocaleId query expected without a locale, got %+v", c)
			}
		}
	})

	t.Run("cmsLocaleId is passed to both delete calls", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(
			http.MethodDelete,
			itemLivePath,
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		)
		mock.handle(
			http.MethodDelete,
			itemPath,
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		)
		_, err := (&CollectionItemResource{}).Delete(context.Background(), infer.DeleteRequest[CollectionItemState]{
			ID: itemResourceID,
			State: CollectionItemState{
				CollectionItemArgs: CollectionItemArgs{Live: true, CmsLocaleID: testLocaleID},
			},
		})
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		calls := mock.requests()
		if len(calls) != 2 || calls[0].Path != itemLivePath || calls[1].Path != itemPath {
			t.Fatalf("expected DELETE live then DELETE staged, got %+v", calls)
		}
		for _, c := range calls {
			if c.Query.Get("cmsLocaleId") != testLocaleID {
				t.Errorf("DELETE %s must carry cmsLocaleId=%s (Webflow only deletes the primary locale otherwise), got %v",
					c.Path, testLocaleID, c.Query)
			}
		}
	})

	t.Run("invalid resource ID", func(t *testing.T) {
		mock := newCMSMock(t)
		if _, err := (&CollectionItemResource{}).Delete(context.Background(), infer.DeleteRequest[CollectionItemState]{
			ID: testCollectionID + "/items/../evil",
		}); err == nil {
			t.Error("expected error")
		}
		if len(mock.requests()) != 0 {
			t.Errorf("invalid IDs must be rejected before any API call")
		}
	})
}

// =============================================================================
// CollectionItem resource: Diff
// =============================================================================

func TestCollectionItemDiff(t *testing.T) {
	fieldData := func() map[string]interface{} {
		return map[string]interface{}{"name": "Test Item", "slug": "test-item", "views": float64(3)}
	}
	state := func(mutate func(s *CollectionItemState)) CollectionItemState {
		s := CollectionItemState{
			CollectionItemArgs: CollectionItemArgs{
				CollectionID: testCollectionID, FieldData: fieldData(),
				IsDraft: ptrBool(true), IsArchived: ptrBool(false), CmsLocaleID: "locale-1",
			},
			ItemID: testItemID,
		}
		mutate(&s)
		return s
	}
	inputs := func(mutate func(a *CollectionItemArgs)) CollectionItemArgs {
		a := CollectionItemArgs{CollectionID: testCollectionID, FieldData: fieldData()}
		mutate(&a)
		return a
	}

	tests := []struct {
		name      string
		state     CollectionItemState
		inputs    CollectionItemArgs
		wantKinds map[string]p.DiffKind
	}{
		{
			"omitted flags and locale after refresh do not diff",
			state(func(s *CollectionItemState) {}), inputs(func(a *CollectionItemArgs) {}), nil,
		},
		{
			"fieldData value change updates",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { a.FieldData["name"] = "Renamed" }),
			map[string]p.DiffKind{"fieldData": p.Update},
		},
		{
			"fieldData added key updates",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { a.FieldData["body"] = "text" }),
			map[string]p.DiffKind{"fieldData": p.Update},
		},
		{
			"fieldData removed key updates",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { delete(a.FieldData, "views") }),
			map[string]p.DiffKind{"fieldData": p.Update},
		},
		{
			"nil and empty fieldData are equal",
			state(func(s *CollectionItemState) { s.FieldData = nil }),
			inputs(func(a *CollectionItemArgs) { a.FieldData = map[string]interface{}{} }), nil,
		},
		{
			"omitted isDraft after refresh does not diff",
			state(func(s *CollectionItemState) { s.IsDraft = ptrBool(false) }),
			inputs(func(a *CollectionItemArgs) { a.IsDraft = nil }), nil,
		},
		{
			"explicit isDraft change updates",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { a.IsDraft = ptrBool(false) }),
			map[string]p.DiffKind{"isDraft": p.Update},
		},
		{
			"explicit isDraft against nil state updates",
			state(func(s *CollectionItemState) { s.IsDraft = nil }),
			inputs(func(a *CollectionItemArgs) { a.IsDraft = ptrBool(false) }),
			map[string]p.DiffKind{"isDraft": p.Update},
		},
		{
			"explicit isArchived change updates",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { a.IsArchived = ptrBool(true) }),
			map[string]p.DiffKind{"isArchived": p.Update},
		},
		{
			"explicit cmsLocaleId change updates",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { a.CmsLocaleID = "locale-2" }),
			map[string]p.DiffKind{"cmsLocaleId": p.Update},
		},
		{
			"live change updates",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { a.Live = true }),
			map[string]p.DiffKind{"live": p.Update},
		},
		{
			"collectionId change replaces",
			state(func(s *CollectionItemState) {}),
			inputs(func(a *CollectionItemArgs) { a.CollectionID = testSiteID }),
			map[string]p.DiffKind{"collectionId": p.UpdateReplace},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := (&CollectionItemResource{}).Diff(
				context.Background(),
				infer.DiffRequest[CollectionItemArgs, CollectionItemState]{
					State: tt.state, Inputs: tt.inputs,
				},
			)
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if len(tt.wantKinds) == 0 {
				if resp.HasChanges || len(resp.DetailedDiff) != 0 {
					t.Errorf("expected no changes, got %+v", resp)
				}
				return
			}
			if !resp.HasChanges || len(resp.DetailedDiff) != len(tt.wantKinds) {
				t.Errorf("DetailedDiff = %v, want %v", resp.DetailedDiff, tt.wantKinds)
			}
			for key, kind := range tt.wantKinds {
				if d, ok := resp.DetailedDiff[key]; !ok || d.Kind != kind {
					t.Errorf("DetailedDiff[%q] = %+v, want kind %v", key, d, kind)
				}
			}
			// A different collection cannot conflict with the old item, so the replacement
			// creates the new item before deleting the old one.
			if resp.DeleteBeforeReplace {
				t.Errorf("DeleteBeforeReplace must not be set: %+v", resp)
			}
		})
	}
}

// TestCollectionItemDrift_OptionalCmsLocaleId_ShouldNotTriggerChange verifies that a
// cmsLocaleId the API reports but the program never set does not cause a phantom update.
func TestCollectionItemDrift_OptionalCmsLocaleId_ShouldNotTriggerChange(t *testing.T) {
	userInputs := CollectionItemArgs{
		CollectionID: testCollectionID,
		FieldData:    map[string]interface{}{"name": "Test Item", "slug": "test-item"},
		IsDraft:      ptrBool(true),
		IsArchived:   ptrBool(false),
	}
	stateFromRead := CollectionItemState{
		CollectionItemArgs: CollectionItemArgs{
			CollectionID: testCollectionID,
			FieldData:    map[string]interface{}{"name": "Test Item", "slug": "test-item"},
			IsDraft:      ptrBool(true),
			IsArchived:   ptrBool(false),
			CmsLocaleID:  "6961ec56c0ac873557148af4", // API returned this
		},
		ItemID: testItemID,
	}

	diffResp, err := (&CollectionItemResource{}).Diff(
		context.Background(),
		infer.DiffRequest[CollectionItemArgs, CollectionItemState]{
			Inputs: userInputs, State: stateFromRead,
		},
	)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if diffResp.HasChanges {
		t.Errorf("Diff() detected phantom changes: %+v", diffResp.DetailedDiff)
	}
}
