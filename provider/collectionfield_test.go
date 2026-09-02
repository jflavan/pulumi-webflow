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
	"sort"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// TestValidateFieldType tests the field type validation function against the documented v2 enum.
func TestValidateFieldType(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		wantErr   bool
	}{
		{"valid PlainText", FieldTypePlainText, false},
		{"valid RichText", FieldTypeRichText, false},
		{"valid Image", FieldTypeImage, false},
		{"valid Number", FieldTypeNumber, false},
		{"valid DateTime", FieldTypeDateTime, false},
		{"valid Reference", FieldTypeReference, false},
		{"valid VideoLink", "VideoLink", false},
		{"valid Option", "Option", false},
		{"invalid legacy Video", "Video", true},
		{"invalid ExtFileRef (read-only type)", "ExtFileRef", true},
		{"empty", "", true},
		{"invalid lowercase", "plaintext", true},
		{"invalid unknown", "InvalidType", true},
		{"invalid mixed case", "plainText", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldType(tt.fieldType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFieldType() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "type") {
				t.Errorf("ValidateFieldType() error should mention 'type': %v", err)
			}
		})
	}

	// Every documented type is accepted.
	for _, ft := range strings.Split(supportedFieldTypeList, ", ") {
		if !ValidFieldTypes[ft] {
			t.Errorf("documented type %q missing from ValidFieldTypes", ft)
		}
	}
	if len(ValidFieldTypes) != 16 {
		t.Errorf("expected 16 field types, got %d", len(ValidFieldTypes))
	}
}

// TestValidateFieldDisplayName tests the field displayName validation function.
func TestValidateFieldDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		wantErr     bool
	}{
		{"valid short", "Title", false},
		{"valid long", strings.Repeat("a", 255), false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 256), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldDisplayName(tt.displayName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFieldDisplayName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateFieldMetadata tests the type-specific metadata requirements.
func TestValidateFieldMetadata(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		metadata  map[string]interface{}
		wantErr   string
	}{
		{"plain text without metadata", FieldTypePlainText, nil, ""},
		{
			"plain text with metadata",
			FieldTypePlainText,
			map[string]interface{}{"options": []interface{}{}},
			"only supported",
		},
		{
			"option with options", FieldTypeOption,
			map[string]interface{}{"options": []interface{}{map[string]interface{}{"name": "Draft"}}},
			"",
		},
		{"option without metadata", FieldTypeOption, nil, "metadata.options is required"},
		{"option with empty options", FieldTypeOption, map[string]interface{}{"options": []interface{}{}}, "non-empty"},
		{
			"option with unnamed option", FieldTypeOption,
			map[string]interface{}{"options": []interface{}{map[string]interface{}{"id": "x"}}},
			"name is required",
		},
		{"reference with collectionId", FieldTypeReference, map[string]interface{}{"collectionId": testCollectionID}, ""},
		{
			"multi reference with collectionId",
			FieldTypeMultiReference,
			map[string]interface{}{"collectionId": testCollectionID},
			"",
		},
		{"reference without collectionId", FieldTypeReference, nil, "metadata.collectionId is required"},
		{"reference with bad collectionId", FieldTypeReference, map[string]interface{}{"collectionId": "nope"}, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldMetadata(tt.fieldType, tt.metadata)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestValidateFieldID tests the path-segment guard for field IDs.
func TestValidateFieldID(t *testing.T) {
	for _, ok := range []string{testFieldID, "field_1", "abc-123"} {
		if err := ValidateFieldID(ok); err != nil {
			t.Errorf("ValidateFieldID(%q) unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a/b", "..", "x y", "%2e%2e", strings.Repeat("a", 65)} {
		if err := ValidateFieldID(bad); err == nil {
			t.Errorf("ValidateFieldID(%q) expected error", bad)
		}
	}
}

// TestGenerateCollectionFieldResourceID tests resource ID generation.
func TestGenerateCollectionFieldResourceID(t *testing.T) {
	got := GenerateCollectionFieldResourceID(testCollectionID, testFieldID)
	if want := testCollectionID + "/fields/" + testFieldID; got != want {
		t.Errorf("GenerateCollectionFieldResourceID() = %v, want %v", got, want)
	}
}

// TestExtractIDsFromCollectionFieldResourceID tests resource ID extraction.
func TestExtractIDsFromCollectionFieldResourceID(t *testing.T) {
	tests := []struct {
		name             string
		resourceID       string
		wantCollectionID string
		wantFieldID      string
		wantErr          bool
	}{
		{"valid resource ID", testCollectionID + "/fields/" + testFieldID, testCollectionID, testFieldID, false},
		{"fieldID with slashes", "abc123/fields/field/with/slashes", "abc123", "field/with/slashes", false},
		{"empty resource ID", "", "", "", true},
		{"invalid format - missing fields", testCollectionID, "", "", true},
		{"invalid format - wrong separator", testCollectionID + "/wrongkey/" + testFieldID, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectionID, fieldID, err := ExtractIDsFromCollectionFieldResourceID(tt.resourceID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if collectionID != tt.wantCollectionID || fieldID != tt.wantFieldID {
				t.Errorf("got (%q, %q), want (%q, %q)", collectionID, fieldID, tt.wantCollectionID, tt.wantFieldID)
			}
		})
	}
}

// fieldResourceID is the Pulumi ID used by the resource-level tests below.
var fieldResourceID = testCollectionID + "/fields/" + testFieldID

// collectionWithFields returns a GET /v2/collections/{id} payload with the given fields.
func collectionWithFields(fields ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id": testCollectionID, "displayName": "Blog Posts", "singularName": "Blog Post", "slug": "blog-posts",
		"fields": fields,
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// =============================================================================
// CollectionField resource: Create
// =============================================================================

func TestCollectionFieldResource_Create(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(
		http.MethodPost,
		"/v2/collections/"+testCollectionID+"/fields",
		func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusCreated, CollectionFieldResponse{
				ID: testFieldID, Type: "PlainText", DisplayName: "Title", Slug: "title", IsEditable: true, IsRequired: false,
			})
		},
	)

	resp, err := (&CollectionField{}).Create(context.Background(), infer.CreateRequest[CollectionFieldArgs]{
		Inputs: CollectionFieldArgs{
			CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Title", Slug: "custom-slug",
			HelpText: "The post title", Validations: map[string]interface{}{"maxLength": 120},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.ID != fieldResourceID {
		t.Errorf("ID = %q, want %q", resp.ID, fieldResourceID)
	}
	if resp.Output.FieldID != testFieldID || !resp.Output.IsEditable {
		t.Errorf("unexpected output: %+v", resp.Output)
	}
	if resp.Output.Slug != "title" {
		t.Errorf("state slug should record the Webflow-generated slug, got %q", resp.Output.Slug)
	}

	calls := mock.callsTo(http.MethodPost, "/v2/collections/"+testCollectionID+"/fields")
	if len(calls) != 1 {
		t.Fatalf("expected 1 POST, got %d", len(calls))
	}
	body := calls[0].Body
	if keys := sortedKeys(body); strings.Join(keys, ",") != "displayName,helpText,isRequired,type" {
		t.Errorf("POST body must contain only displayName, helpText, isRequired, type; got keys %v", keys)
	}
	if body["type"] != "PlainText" || body["displayName"] != "Title" || body["helpText"] != "The post title" {
		t.Errorf("unexpected POST body: %v", body)
	}
	if v, ok := body["isRequired"]; !ok || v != false {
		t.Errorf("isRequired:false must be present in the POST body, got %v", body)
	}
	if _, ok := body["validations"]; ok {
		t.Errorf("validations are not accepted by the Create Field endpoint and must not be sent: %v", body)
	}
	if _, ok := body["slug"]; ok {
		t.Errorf("slug is not accepted by the Create Field endpoint and must not be sent: %v", body)
	}
}

func TestCollectionFieldResource_Create_RecordsValidationsFromResponse(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(
		http.MethodPost,
		"/v2/collections/"+testCollectionID+"/fields",
		func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusCreated, CollectionFieldResponse{
				ID: testFieldID, Type: "PlainText", DisplayName: "Title", Slug: "title", IsEditable: true,
				Validations: map[string]interface{}{"singleLine": true},
			})
		},
	)

	resp, err := (&CollectionField{}).Create(context.Background(), infer.CreateRequest[CollectionFieldArgs]{
		Inputs: CollectionFieldArgs{CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Title"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Output.Validations["singleLine"] != true {
		t.Errorf("state validations should come from the API response, got %v", resp.Output.Validations)
	}
}

func TestCollectionFieldResource_Create_OptionFieldSendsMetadata(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(
		http.MethodPost,
		"/v2/collections/"+testCollectionID+"/fields",
		func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(
				w,
				http.StatusCreated,
				CollectionFieldResponse{ID: testFieldID, Type: "Option", DisplayName: "Status", Slug: "status"},
			)
		},
	)

	isRequired := true
	_, err := (&CollectionField{}).Create(context.Background(), infer.CreateRequest[CollectionFieldArgs]{
		Inputs: CollectionFieldArgs{
			CollectionID: testCollectionID, Type: "Option", DisplayName: "Status", IsRequired: isRequired,
			Metadata: map[string]interface{}{"options": []interface{}{
				map[string]interface{}{"name": "Draft"}, map[string]interface{}{"name": "Published"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	body := mock.callsTo(http.MethodPost, "/v2/collections/"+testCollectionID+"/fields")[0].Body
	meta, _ := body["metadata"].(map[string]interface{})
	options, _ := meta["options"].([]interface{})
	if len(options) != 2 {
		t.Errorf("expected metadata.options with 2 entries in POST body, got %v", body)
	}
	if body["isRequired"] != true {
		t.Errorf("isRequired:true missing from POST body: %v", body)
	}
}

func TestCollectionFieldResource_Create_DryRunSkipsValidationAndAPI(t *testing.T) {
	mock := newCMSMock(t)
	resp, err := (&CollectionField{}).Create(context.Background(), infer.CreateRequest[CollectionFieldArgs]{
		Inputs: CollectionFieldArgs{CollectionID: "", Type: "PlainText", DisplayName: "Title"},
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
	if resp.Output.FieldID != "" {
		t.Errorf("dry run must not fabricate a field ID: %+v", resp.Output)
	}
}

// =============================================================================
// CollectionField resource: Check
// =============================================================================

func TestCollectionFieldResource_Check(t *testing.T) {
	check := func(t *testing.T, inputs map[string]property.Value) infer.CheckResponse[CollectionFieldArgs] {
		t.Helper()
		resp, err := (&CollectionField{}).Check(
			context.Background(), infer.CheckRequest{NewInputs: property.NewMap(inputs)},
		)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		return resp
	}
	failedProps := func(resp infer.CheckResponse[CollectionFieldArgs]) string {
		props := make([]string, 0, len(resp.Failures))
		for _, f := range resp.Failures {
			props = append(props, f.Property)
		}
		sort.Strings(props)
		return strings.Join(props, ",")
	}
	options := func(names ...string) property.Value {
		list := make([]property.Value, 0, len(names))
		for _, name := range names {
			list = append(list, property.New(map[string]property.Value{"name": property.New(name)}))
		}
		return property.New(map[string]property.Value{"options": property.New(list)})
	}

	t.Run("valid plain text field passes", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID), "type": property.New("PlainText"),
			"displayName": property.New("Title"),
		})
		if len(resp.Failures) != 0 {
			t.Errorf("unexpected failures: %+v", resp.Failures)
		}
		if resp.Inputs.CollectionID != testCollectionID || resp.Inputs.Type != "PlainText" {
			t.Errorf("inputs not decoded: %+v", resp.Inputs)
		}
	})

	t.Run("valid option field with metadata passes", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID), "type": property.New("Option"),
			"displayName": property.New("Status"), "metadata": options("Draft", "Published"),
		})
		if len(resp.Failures) != 0 {
			t.Errorf("unexpected failures: %+v", resp.Failures)
		}
	})

	t.Run("malformed known values fail per property", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New("bad"), "type": property.New("Video"), "displayName": property.New(""),
		})
		if got := failedProps(resp); got != "collectionId,displayName,type" {
			t.Errorf("failures = %q, want collectionId,displayName,type: %+v", got, resp.Failures)
		}
	})

	t.Run("reference field without metadata fails", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID), "type": property.New("Reference"),
			"displayName": property.New("Author"),
		})
		if got := failedProps(resp); got != "metadata" {
			t.Errorf("failures = %q, want metadata: %+v", got, resp.Failures)
		}
	})

	t.Run("metadata on a plain text field fails", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID), "type": property.New("PlainText"),
			"displayName": property.New("Title"), "metadata": options("Draft"),
		})
		if got := failedProps(resp); got != "metadata" {
			t.Errorf("failures = %q, want metadata: %+v", got, resp.Failures)
		}
	})

	t.Run("unknown values are skipped", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(property.Computed), "type": property.New(property.Computed),
			"displayName": property.New(property.Computed), "metadata": property.New(property.Computed),
		})
		if len(resp.Failures) != 0 {
			t.Errorf("computed inputs must not fail: %+v", resp.Failures)
		}
	})

	t.Run("metadata with an unknown collectionId is skipped", func(t *testing.T) {
		resp := check(t, map[string]property.Value{
			"collectionId": property.New(testCollectionID), "type": property.New("Reference"),
			"displayName": property.New("Author"),
			"metadata": property.New(map[string]property.Value{
				"collectionId": property.New(property.Computed),
			}),
		})
		if len(resp.Failures) != 0 {
			t.Errorf("metadata that depends on another resource must not fail: %+v", resp.Failures)
		}
	})
}

func TestCollectionFieldResource_Create_ValidationErrors(t *testing.T) {
	mock := newCMSMock(t)
	tests := []struct {
		name   string
		inputs CollectionFieldArgs
		want   string
	}{
		{
			"invalid collectionId",
			CollectionFieldArgs{CollectionID: "bad", Type: "PlainText", DisplayName: "T"},
			"collectionId",
		},
		{
			"legacy Video type",
			CollectionFieldArgs{CollectionID: testCollectionID, Type: "Video", DisplayName: "T"},
			"VideoLink",
		},
		{"missing displayName", CollectionFieldArgs{CollectionID: testCollectionID, Type: "PlainText"}, "displayName"},
		{
			"reference without metadata",
			CollectionFieldArgs{CollectionID: testCollectionID, Type: "Reference", DisplayName: "Author"},
			"metadata.collectionId",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&CollectionField{}).Create(
				context.Background(),
				infer.CreateRequest[CollectionFieldArgs]{Inputs: tt.inputs},
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
// CollectionField resource: Update
// =============================================================================

func TestCollectionFieldResource_Update_UsesPatchWithMutableFieldsOnly(t *testing.T) {
	mock := newCMSMock(t)
	path := "/v2/collections/" + testCollectionID + "/fields/" + testFieldID
	mock.handle(http.MethodPatch, path, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, CollectionFieldResponse{
			ID: testFieldID, Type: "PlainText", DisplayName: "Headline", Slug: "title", IsEditable: true, IsRequired: false,
		})
	})

	resp, err := (&CollectionField{}).Update(
		context.Background(),
		infer.UpdateRequest[CollectionFieldArgs, CollectionFieldState]{
			ID: fieldResourceID,
			State: CollectionFieldState{
				CollectionFieldArgs: CollectionFieldArgs{
					CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Title", Slug: "title", IsRequired: true,
					Validations: map[string]interface{}{"maxLength": float64(120)},
				},
				FieldID: testFieldID, IsEditable: true,
			},
			Inputs: CollectionFieldArgs{
				CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Headline", IsRequired: false, HelpText: "",
				Validations: map[string]interface{}{"maxLength": float64(120)},
			},
		},
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if resp.Output.FieldID != testFieldID || resp.Output.DisplayName != "Headline" || resp.Output.Slug != "title" {
		t.Errorf("unexpected output: %+v", resp.Output)
	}

	calls := mock.requests()
	if len(calls) != 1 || calls[0].Method != http.MethodPatch || calls[0].Path != path {
		t.Fatalf("expected exactly one PATCH %s, got %+v", path, calls)
	}
	body := calls[0].Body
	if keys := sortedKeys(body); strings.Join(keys, ",") != "displayName,helpText,isRequired" {
		t.Errorf("PATCH body must contain only displayName, helpText, isRequired; got keys %v", keys)
	}
	if v, ok := body["isRequired"]; !ok || v != false {
		t.Errorf("isRequired:false must be present in the PATCH body, got %v", body)
	}
	if body["displayName"] != "Headline" {
		t.Errorf("displayName not sent: %v", body)
	}
}

func TestCollectionFieldResource_Update_DryRun(t *testing.T) {
	mock := newCMSMock(t)
	resp, err := (&CollectionField{}).Update(
		context.Background(),
		infer.UpdateRequest[CollectionFieldArgs, CollectionFieldState]{
			ID: fieldResourceID,
			State: CollectionFieldState{
				FieldID:             testFieldID,
				IsEditable:          true,
				CollectionFieldArgs: CollectionFieldArgs{Slug: "title"},
			},
			Inputs: CollectionFieldArgs{CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Headline"},
			DryRun: true,
		},
	)
	if err != nil {
		t.Fatalf("Update(DryRun) error = %v", err)
	}
	if len(mock.requests()) != 0 {
		t.Errorf("dry run must not call the API")
	}
	if resp.Output.FieldID != testFieldID || resp.Output.Slug != "title" {
		t.Errorf("dry run should preserve fieldId and the recorded slug: %+v", resp.Output)
	}
}

func TestCollectionFieldResource_Update_APIError(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPatch, "/v2/collections/"+testCollectionID+"/fields/"+testFieldID,
		func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusNotFound, map[string]string{"message": "field not found"})
		})
	_, err := (&CollectionField{}).Update(
		context.Background(),
		infer.UpdateRequest[CollectionFieldArgs, CollectionFieldState]{
			ID:     fieldResourceID,
			Inputs: CollectionFieldArgs{CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Headline"},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "failed to update collection field") {
		t.Fatalf("expected update error, got %v", err)
	}
}

// =============================================================================
// CollectionField resource: Read
// =============================================================================

func TestCollectionFieldResource_Read(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, collectionWithFields(
			map[string]interface{}{"id": "other000000000000000000", "type": "RichText", "displayName": "Body", "slug": "body"},
			map[string]interface{}{
				"id": testFieldID, "type": "PlainText", "displayName": "Title", "slug": "title",
				"isEditable": true, "isRequired": true, "helpText": "Post title",
				"validations": map[string]interface{}{"maxLength": 120, "singleLine": true},
			},
		))
	})

	t.Run("omitted inputs stay omitted, state reflects API", func(t *testing.T) {
		resp, err := (&CollectionField{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{
				ID:     fieldResourceID,
				Inputs: CollectionFieldArgs{CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Title"},
			},
		)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.ID != fieldResourceID {
			t.Errorf("ID = %q", resp.ID)
		}
		if resp.Inputs.Slug != "" || resp.Inputs.Validations != nil {
			t.Errorf("omitted slug/validations must not be filled into inputs: %+v", resp.Inputs)
		}
		if !resp.Inputs.IsRequired || resp.Inputs.HelpText != "Post title" {
			t.Errorf("mutable properties should be refreshed into inputs: %+v", resp.Inputs)
		}
		if resp.State.Slug != "title" || resp.State.FieldID != testFieldID || !resp.State.IsEditable ||
			resp.State.Validations["maxLength"] != float64(120) {
			t.Errorf("unexpected state: %+v", resp.State)
		}
	})

	t.Run("deprecated slug and validations inputs are left untouched", func(t *testing.T) {
		resp, err := (&CollectionField{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{
				ID: fieldResourceID,
				Inputs: CollectionFieldArgs{
					CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Title", Slug: "old",
					Validations: map[string]interface{}{"maxLength": float64(50)},
				},
			},
		)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.Inputs.Slug != "old" || resp.Inputs.Validations["maxLength"] != float64(50) {
			t.Errorf("slug/validations are not API inputs and must stay as written: %+v", resp.Inputs)
		}
		if resp.State.Slug != "title" || resp.State.Validations["maxLength"] != float64(120) {
			t.Errorf("state should reflect the API values: %+v", resp.State)
		}
	})
}

// optionFieldFromAPI is an Option field as GET /v2/collections/{id} reports it: no metadata,
// the choices are under validations.options with Webflow-assigned IDs.
func optionFieldFromAPI() map[string]interface{} {
	return map[string]interface{}{
		"id": testFieldID, "type": "Option", "displayName": "Status", "slug": "status", "isEditable": true,
		"validations": map[string]interface{}{"options": []interface{}{
			map[string]interface{}{"name": "Draft", "id": "a1b2c3d4e5f60718293a4b5c"},
			map[string]interface{}{"name": "Published", "id": "b2c3d4e5f60718293a4b5c6d"},
		}},
	}
}

// referenceFieldFromAPI is a Reference field as GET /v2/collections/{id} reports it: no
// metadata, the referenced collection is under validations.collectionId.
func referenceFieldFromAPI(fieldType string) map[string]interface{} {
	return map[string]interface{}{
		"id": testFieldID, "type": fieldType, "displayName": "Author", "slug": "author", "isEditable": true,
		"validations": map[string]interface{}{"collectionId": testSiteID},
	}
}

// readThenDiff runs Read with the given program inputs and then Diff between the refreshed
// state and the same program inputs, mirroring `pulumi refresh` followed by `pulumi up`.
func readThenDiff(t *testing.T, inputs CollectionFieldArgs) (
	read infer.ReadResponse[CollectionFieldArgs, CollectionFieldState], diff infer.DiffResponse,
) {
	t.Helper()
	read, err := (&CollectionField{}).Read(
		context.Background(),
		infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{ID: fieldResourceID, Inputs: inputs},
	)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	diff, err = (&CollectionField{}).Diff(
		context.Background(),
		infer.DiffRequest[CollectionFieldArgs, CollectionFieldState]{State: read.State, Inputs: inputs},
	)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	return read, diff
}

func TestCollectionFieldResource_ReadThenDiff_OptionField(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, collectionWithFields(optionFieldFromAPI()))
	})
	inputs := CollectionFieldArgs{
		CollectionID: testCollectionID, Type: "Option", DisplayName: "Status",
		Metadata: map[string]interface{}{"options": []interface{}{
			map[string]interface{}{"name": "Draft"}, map[string]interface{}{"name": "Published"},
		}},
	}

	read, diff := readThenDiff(t, inputs)
	if diff.HasChanges || len(diff.DetailedDiff) != 0 {
		t.Errorf("an unchanged Option field must not diff after refresh: %+v (state %+v)", diff, read.State)
	}

	// Inputs carry the user-facing shape (names only), state keeps the Webflow option IDs.
	if !reflect.DeepEqual(read.Inputs.Metadata, inputs.Metadata) {
		t.Errorf("refreshed inputs metadata = %v, want %v", read.Inputs.Metadata, inputs.Metadata)
	}
	stateOptions, _ := read.State.Metadata["options"].([]interface{})
	if len(stateOptions) != 2 {
		t.Fatalf("state metadata should be reconstructed from validations, got %v", read.State.Metadata)
	}
	if first, _ := stateOptions[0].(map[string]interface{}); first["id"] != "a1b2c3d4e5f60718293a4b5c" {
		t.Errorf("state metadata should keep the option IDs, got %v", stateOptions)
	}
	if read.State.Validations == nil {
		t.Errorf("state validations should reflect the API, got %+v", read.State)
	}

	// The same field with a renamed choice still replaces.
	renamed := inputs
	renamed.Metadata = map[string]interface{}{"options": []interface{}{
		map[string]interface{}{"name": "Draft"}, map[string]interface{}{"name": "Archived"},
	}}
	_, diff = readThenDiff(t, renamed)
	if d, ok := diff.DetailedDiff["metadata"]; !ok || d.Kind != p.UpdateReplace {
		t.Errorf("a renamed option must replace the field: %+v", diff)
	}
}

func TestCollectionFieldResource_ReadThenDiff_ReferenceFields(t *testing.T) {
	for _, fieldType := range []string{FieldTypeReference, FieldTypeMultiReference} {
		t.Run(fieldType, func(t *testing.T) {
			mock := newCMSMock(t)
			mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
				writeCMSJSON(w, http.StatusOK, collectionWithFields(referenceFieldFromAPI(fieldType)))
			})
			inputs := CollectionFieldArgs{
				CollectionID: testCollectionID, Type: fieldType, DisplayName: "Author",
				Metadata: map[string]interface{}{"collectionId": testSiteID},
			}

			read, diff := readThenDiff(t, inputs)
			if diff.HasChanges || len(diff.DetailedDiff) != 0 {
				t.Errorf("an unchanged %s field must not diff after refresh: %+v (state %+v)", fieldType, diff, read.State)
			}
			if read.State.Metadata["collectionId"] != testSiteID || read.Inputs.Metadata["collectionId"] != testSiteID {
				t.Errorf("metadata.collectionId should be reconstructed from validations: %+v", read)
			}

			// Pointing the field at another collection replaces it.
			other := inputs
			other.Metadata = map[string]interface{}{"collectionId": testCollectionID}
			_, diff = readThenDiff(t, other)
			if d, ok := diff.DetailedDiff["metadata"]; !ok || d.Kind != p.UpdateReplace {
				t.Errorf("a different referenced collection must replace the field: %+v", diff)
			}
		})
	}
}

func TestCollectionFieldResource_Read_ImportReconstructsMetadataOnlyInState(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, collectionWithFields(optionFieldFromAPI()))
	})
	read, err := (&CollectionField{}).Read(
		context.Background(),
		infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{ID: fieldResourceID},
	)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if read.Inputs.Metadata != nil {
		t.Errorf("omitted metadata input must stay omitted, got %v", read.Inputs.Metadata)
	}
	if _, ok := read.State.Metadata["options"]; !ok {
		t.Errorf("state metadata should be reconstructed for Option fields, got %v", read.State.Metadata)
	}
}

func TestMetadataFromValidations(t *testing.T) {
	optionValidations := map[string]interface{}{"options": []interface{}{
		map[string]interface{}{"name": "Draft", "id": "aaa"},
		"not an object",
	}}
	if got := metadataFromValidations(FieldTypeOption, optionValidations, false); !reflect.DeepEqual(got,
		map[string]interface{}{"options": []interface{}{map[string]interface{}{"name": "Draft"}}}) {
		t.Errorf("names only: got %v", got)
	}
	if got := metadataFromValidations(FieldTypeOption, optionValidations, true); !reflect.DeepEqual(got,
		map[string]interface{}{"options": []interface{}{map[string]interface{}{"name": "Draft", "id": "aaa"}}}) {
		t.Errorf("with ids: got %v", got)
	}
	if got := metadataFromValidations(FieldTypeOption, map[string]interface{}{"singleLine": true}, true); got != nil {
		t.Errorf("Option without options: got %v, want nil", got)
	}
	if got := metadataFromValidations(FieldTypeReference, map[string]interface{}{"collectionId": ""}, true); got != nil {
		t.Errorf("Reference with empty collectionId: got %v, want nil", got)
	}
	if got := metadataFromValidations(FieldTypePlainText, map[string]interface{}{"maxLength": 5}, true); got != nil {
		t.Errorf("PlainText: got %v, want nil", got)
	}
}

func TestCollectionFieldResource_Read_Missing(t *testing.T) {
	t.Run("field not in collection", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusOK, collectionWithFields(
				map[string]interface{}{"id": "other000000000000000000", "type": "RichText", "displayName": "Body"},
			))
		})
		resp, err := (&CollectionField{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{ID: fieldResourceID},
		)
		if err != nil || resp.ID != "" {
			t.Errorf("expected empty ID and nil error, got ID=%q err=%v", resp.ID, err)
		}
	})

	t.Run("collection 404", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
			writeCMSJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
		})
		resp, err := (&CollectionField{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{ID: fieldResourceID},
		)
		if err != nil || resp.ID != "" {
			t.Errorf("expected empty ID and nil error, got ID=%q err=%v", resp.ID, err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		mock := newCMSMock(t)
		mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		if _, err := (&CollectionField{}).Read(
			context.Background(), infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{ID: fieldResourceID},
		); err == nil {
			t.Error("expected error for 500")
		}
	})

	t.Run("invalid resource ID", func(t *testing.T) {
		mock := newCMSMock(t)
		for _, id := range []string{"", "bad/fields/" + testFieldID, testCollectionID + "/fields/a/b"} {
			if _, err := (&CollectionField{}).Read(
				context.Background(), infer.ReadRequest[CollectionFieldArgs, CollectionFieldState]{ID: id},
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
// CollectionField resource: Delete
// =============================================================================

func TestCollectionFieldResource_Delete(t *testing.T) {
	path := "/v2/collections/" + testCollectionID + "/fields/" + testFieldID
	for _, tt := range []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"204", http.StatusNoContent, false},
		{"404 idempotent", http.StatusNotFound, false},
		{"403", http.StatusForbidden, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mock := newCMSMock(t)
			mock.handle(http.MethodDelete, path, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status) })
			_, err := (&CollectionField{}).Delete(
				context.Background(),
				infer.DeleteRequest[CollectionFieldState]{ID: fieldResourceID},
			)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(mock.callsTo(http.MethodDelete, path)) != 1 {
				t.Errorf("expected one DELETE %s, got %+v", path, mock.requests())
			}
		})
	}

	t.Run("invalid resource ID", func(t *testing.T) {
		mock := newCMSMock(t)
		if _, err := (&CollectionField{}).Delete(context.Background(), infer.DeleteRequest[CollectionFieldState]{
			ID: testCollectionID + "/fields/../evil",
		}); err == nil {
			t.Error("expected error")
		}
		if len(mock.requests()) != 0 {
			t.Errorf("invalid IDs must be rejected before any API call")
		}
	})
}

// =============================================================================
// CollectionField resource: Diff
// =============================================================================

func TestCollectionFieldDiff(t *testing.T) {
	base := CollectionFieldArgs{
		CollectionID: testCollectionID, Type: "PlainText", DisplayName: "Title", Slug: "title",
		IsRequired: true, HelpText: "help",
		Validations: map[string]interface{}{"maxLength": float64(500)},
	}
	withArgs := func(mutate func(a *CollectionFieldArgs)) CollectionFieldArgs {
		a := base
		a.Validations = map[string]interface{}{"maxLength": float64(500)}
		mutate(&a)
		return a
	}

	tests := []struct {
		name      string
		state     CollectionFieldArgs
		inputs    CollectionFieldArgs
		wantKinds map[string]p.DiffKind
	}{
		{"no changes", base, withArgs(func(a *CollectionFieldArgs) {}), nil},
		{
			"validations are ignored by the API and never diff", base,
			withArgs(func(a *CollectionFieldArgs) { a.Validations = map[string]interface{}{"maxLength": float64(1000)} }),
			nil,
		},
		{
			"validations omitted after refresh does not diff", base,
			withArgs(func(a *CollectionFieldArgs) { a.Validations = nil }), nil,
		},
		{
			"slug omitted after refresh does not diff", base,
			withArgs(func(a *CollectionFieldArgs) { a.Slug = "" }), nil,
		},
		{
			"slug is ignored by the API and never diffs", base,
			withArgs(func(a *CollectionFieldArgs) { a.Slug = "headline" }), nil,
		},
		{
			"type change replaces", base,
			withArgs(func(a *CollectionFieldArgs) { a.Type = "RichText" }),
			map[string]p.DiffKind{"type": p.UpdateReplace},
		},
		{
			"collectionId change replaces", base,
			withArgs(func(a *CollectionFieldArgs) { a.CollectionID = testSiteID }),
			map[string]p.DiffKind{"collectionId": p.UpdateReplace},
		},
		{
			"displayName, isRequired and helpText update in place", base,
			withArgs(func(a *CollectionFieldArgs) { a.DisplayName = "Headline"; a.IsRequired = false; a.HelpText = "" }),
			map[string]p.DiffKind{"displayName": p.Update, "isRequired": p.Update, "helpText": p.Update},
		},
		{
			"metadata change replaces",
			withArgs(func(a *CollectionFieldArgs) {
				a.Type = "Reference"
				a.Metadata = map[string]interface{}{"collectionId": testCollectionID}
			}),
			withArgs(func(a *CollectionFieldArgs) {
				a.Type = "Reference"
				a.Metadata = map[string]interface{}{"collectionId": testSiteID}
			}),
			map[string]p.DiffKind{"metadata": p.UpdateReplace},
		},
		{
			"metadata options with API-assigned ids do not diff",
			withArgs(func(a *CollectionFieldArgs) {
				a.Type = "Option"
				a.Metadata = map[string]interface{}{"options": []interface{}{
					map[string]interface{}{"name": "Draft", "id": "aaa"}, map[string]interface{}{"name": "Published", "id": "bbb"},
				}}
			}),
			withArgs(func(a *CollectionFieldArgs) {
				a.Type = "Option"
				a.Metadata = map[string]interface{}{"options": []interface{}{
					map[string]interface{}{"name": "Draft"}, map[string]interface{}{"name": "Published"},
				}}
			}), nil,
		},
		{
			"metadata option renamed replaces",
			withArgs(func(a *CollectionFieldArgs) {
				a.Type = "Option"
				a.Metadata = map[string]interface{}{"options": []interface{}{map[string]interface{}{"name": "Draft", "id": "aaa"}}}
			}),
			withArgs(func(a *CollectionFieldArgs) {
				a.Type = "Option"
				a.Metadata = map[string]interface{}{"options": []interface{}{map[string]interface{}{"name": "Archived"}}}
			}),
			map[string]p.DiffKind{"metadata": p.UpdateReplace},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := (&CollectionField{}).Diff(
				context.Background(),
				infer.DiffRequest[CollectionFieldArgs, CollectionFieldState]{
					State:  CollectionFieldState{CollectionFieldArgs: tt.state, FieldID: testFieldID},
					Inputs: tt.inputs,
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
			if !resp.HasChanges {
				t.Errorf("expected HasChanges")
			}
			if len(resp.DetailedDiff) != len(tt.wantKinds) {
				t.Errorf("DetailedDiff = %v, want keys %v", resp.DetailedDiff, tt.wantKinds)
			}
			wantReplace := false
			for key, kind := range tt.wantKinds {
				if d, ok := resp.DetailedDiff[key]; !ok || d.Kind != kind {
					t.Errorf("DetailedDiff[%q] = %+v, want kind %v", key, d, kind)
				}
				if kind == p.UpdateReplace {
					wantReplace = true
				}
			}
			if resp.DeleteBeforeReplace != wantReplace {
				t.Errorf("DeleteBeforeReplace = %v, want %v", resp.DeleteBeforeReplace, wantReplace)
			}
		})
	}
}
