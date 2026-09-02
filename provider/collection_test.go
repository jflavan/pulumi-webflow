// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Shared fixtures for the collection, collection field and collection item resource tests.
const (
	testCollectionID = "6a1b2c3d4e5f607182930a1b"
	testFieldID      = "7b2c3d4e5f60718293a4b5c6"
	testItemID       = "8c3d4e5f60718293a4b5c6d7"
	testAPIToken     = "test-token-abc123def456"
)

// cmsCall is one request received by the mock Webflow API.
type cmsCall struct {
	Method string
	Path   string
	Query  url.Values
	Body   map[string]interface{}
}

// cmsMock is a small method+path router for resource-level tests. It records every
// request so tests can assert on HTTP methods, paths, query strings and JSON bodies.
type cmsMock struct {
	t      *testing.T
	mu     sync.Mutex
	calls  []cmsCall
	routes map[string]http.HandlerFunc
}

// newCMSMock starts a mock API, points the provider at it (fast retries, no real sleeps)
// and sets the API token in the environment for GetHTTPClient.
func newCMSMock(t *testing.T) *cmsMock {
	t.Helper()
	m := &cmsMock{t: t, routes: map[string]http.HandlerFunc{}}
	server := httptest.NewServer(m)
	t.Cleanup(server.Close)
	useMockAPI(t, server)
	t.Setenv("WEBFLOW_API_TOKEN", testAPIToken)
	return m
}

// handle registers a handler for METHOD path.
func (m *cmsMock) handle(method, path string, h http.HandlerFunc) {
	m.routes[method+" "+path] = h
}

func (m *cmsMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	call := cmsCall{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &call.Body); err != nil {
			m.t.Errorf("%s %s: request body is not a JSON object: %v", r.Method, r.URL.Path, err)
		}
	}
	m.mu.Lock()
	m.calls = append(m.calls, call)
	m.mu.Unlock()

	h, ok := m.routes[r.Method+" "+r.URL.Path]
	if !ok {
		m.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	h(w, r)
}

// requests returns a copy of every call received so far.
func (m *cmsMock) requests() []cmsCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]cmsCall(nil), m.calls...)
}

// callsTo returns the calls matching METHOD path.
func (m *cmsMock) callsTo(method, path string) []cmsCall {
	var out []cmsCall
	for _, c := range m.requests() {
		if c.Method == method && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

// writeCMSJSON writes a JSON response with the given status (headers before WriteHeader).
func writeCMSJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ptrBool(b bool) *bool { return &b }

// TestValidateCollectionID tests collectionID validation
func TestValidateCollectionID(t *testing.T) {
	tests := []struct {
		name         string
		collectionID string
		wantErr      bool
	}{
		{"valid collection ID", "5f0c8c9e1c9d440000e8d8c3", false},
		{"valid collection ID 2", "abcdef0123456789abcdef01", false},
		{"empty collection ID", "", true},
		{"too short", "5f0c8c9e1c9d440000e8d8", true},
		{"too long", "5f0c8c9e1c9d440000e8d8c3aa", true},
		{"uppercase letters", "5F0C8C9E1C9D440000E8D8C3", true},
		{"invalid characters", "5f0c8c9e1c9d440000e8d8cg", true},
		{"with spaces", "5f0c8c9e 1c9d440000e8d8c3", true},
		{"with dashes", "5f0c8c9e-1c9d-4400-00e8d8c3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCollectionID(tt.collectionID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCollectionID(%s) error = %v, wantErr %v", tt.collectionID, err, tt.wantErr)
			}
		})
	}
}

// TestValidateCollectionDisplayName tests displayName validation
func TestValidateCollectionDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		wantErr     bool
	}{
		{"valid name", "Blog Posts", false},
		{"valid name with numbers", "Products 2024", false},
		{"valid name with special chars", "Team Members & Partners", false},
		{"empty name", "", true},
		{"very long name", strings.Repeat("a", 256), true},
		{"max length name", strings.Repeat("a", 255), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCollectionDisplayName(tt.displayName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCollectionDisplayName(%s) error = %v, wantErr %v", tt.displayName, err, tt.wantErr)
			}
		})
	}
}

// TestValidateSingularName tests singularName validation
func TestValidateSingularName(t *testing.T) {
	tests := []struct {
		name         string
		singularName string
		wantErr      bool
	}{
		{"valid singular name", "Blog Post", false},
		{"valid singular name 2", "Product", false},
		{"empty singular name", "", true},
		{"very long name", strings.Repeat("a", 256), true},
		{"max length name", strings.Repeat("a", 255), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSingularName(tt.singularName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSingularName(%s) error = %v, wantErr %v", tt.singularName, err, tt.wantErr)
			}
		})
	}
}

// TestGenerateCollectionResourceID tests resource ID generation
func TestGenerateCollectionResourceID(t *testing.T) {
	resourceID := GenerateCollectionResourceID(testSiteID, testCollectionID)
	expected := testSiteID + "/collections/" + testCollectionID
	if resourceID != expected {
		t.Errorf("GenerateCollectionResourceID() = %q, want %q", resourceID, expected)
	}
}

// TestExtractIDsFromCollectionResourceID_Valid tests extracting IDs from valid resource ID
func TestExtractIDsFromCollectionResourceID_Valid(t *testing.T) {
	siteID, collectionID, err := ExtractIDsFromCollectionResourceID(testSiteID + "/collections/" + testCollectionID)
	if err != nil {
		t.Errorf("ExtractIDsFromCollectionResourceID() error = %v, want nil", err)
	}
	if siteID != testSiteID {
		t.Errorf("siteID = %q, want %q", siteID, testSiteID)
	}
	if collectionID != testCollectionID {
		t.Errorf("collectionID = %q, want %q", collectionID, testCollectionID)
	}
}

// TestExtractIDsFromCollectionResourceID_InvalidFormat tests invalid resource IDs
func TestExtractIDsFromCollectionResourceID_InvalidFormat(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
	}{
		{"empty", ""},
		{"missing collections part", testSiteID + "/" + testCollectionID},
		{"wrong middle part", testSiteID + "/redirects/" + testCollectionID},
		{"too few parts", testSiteID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := ExtractIDsFromCollectionResourceID(tt.resourceID); err == nil {
				t.Errorf("ExtractIDsFromCollectionResourceID(%q) error = nil, want error", tt.resourceID)
			}
		})
	}
}

// TestCollectionErrorMessagesAreActionable verifies error messages contain guidance
func TestCollectionErrorMessagesAreActionable(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func() error
		contains []string
	}{
		{
			"ValidateCollectionID empty",
			func() error { return ValidateCollectionID("") },
			[]string{"required", "24-character"},
		},
		{
			"ValidateCollectionID invalid format", func() error { return ValidateCollectionID("invalid") },
			[]string{"invalid format", "24-character", "hexadecimal"},
		},
		{
			"ValidateCollectionDisplayName empty", func() error { return ValidateCollectionDisplayName("") },
			[]string{"required", "name"},
		},
		{"ValidateSingularName empty", func() error { return ValidateSingularName("") }, []string{"required", "singular"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tt.name)
			}
			for _, expectedStr := range tt.contains {
				if !strings.Contains(err.Error(), expectedStr) {
					t.Errorf("%s: error message missing %q. Got: %s", tt.name, expectedStr, err.Error())
				}
			}
		})
	}
}

// =============================================================================
// Collection resource: Create
// =============================================================================

func TestCollectionResource_Create_RecordsGeneratedSlug(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPost, "/v2/sites/"+testSiteID+"/collections", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["displayName"] != "Blog Posts" || body["singularName"] != "Blog Post" {
			t.Errorf("unexpected body: %v", body)
		}
		if _, ok := body["slug"]; ok {
			t.Errorf("omitted slug must not be sent, got body %v", body)
		}
		writeCMSJSON(w, http.StatusCreated, Collection{
			ID: testCollectionID, DisplayName: "Blog Posts", SingularName: "Blog Post", Slug: "blog-posts",
			CreatedOn: "2024-01-01T00:00:00Z", LastUpdated: "2024-01-01T00:00:00Z",
		})
	})

	resp, err := (&CollectionResource{}).Create(context.Background(), infer.CreateRequest[CollectionArgs]{
		Inputs: CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if want := testSiteID + "/collections/" + testCollectionID; resp.ID != want {
		t.Errorf("ID = %q, want %q", resp.ID, want)
	}
	if resp.Output.CollectionID != testCollectionID {
		t.Errorf("CollectionID = %q, want %q", resp.Output.CollectionID, testCollectionID)
	}
	if resp.Output.Slug != "blog-posts" {
		t.Errorf("state slug = %q, want the Webflow-generated %q", resp.Output.Slug, "blog-posts")
	}
	if resp.Output.CreatedOn != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedOn = %q, want API value", resp.Output.CreatedOn)
	}
	if len(mock.requests()) != 1 {
		t.Errorf("expected exactly 1 API call, got %d", len(mock.requests()))
	}
}

func TestCollectionResource_Create_SendsExplicitSlug(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPost, "/v2/sites/"+testSiteID+"/collections", func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, Collection{ID: testCollectionID, Slug: "posts"})
	})

	_, err := (&CollectionResource{}).Create(context.Background(), infer.CreateRequest[CollectionArgs]{
		Inputs: CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post", Slug: "posts"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	calls := mock.callsTo(http.MethodPost, "/v2/sites/"+testSiteID+"/collections")
	if len(calls) != 1 || calls[0].Body["slug"] != "posts" {
		t.Errorf("expected slug in POST body, got %+v", calls)
	}
}

func TestCollectionResource_Create_DryRunSkipsValidationAndAPI(t *testing.T) {
	mock := newCMSMock(t)

	// siteId is unknown during preview when it comes from another resource
	resp, err := (&CollectionResource{}).Create(context.Background(), infer.CreateRequest[CollectionArgs]{
		Inputs: CollectionArgs{SiteID: "", DisplayName: "Blog Posts", SingularName: "Blog Post"},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create(DryRun) error = %v", err)
	}
	if len(mock.requests()) != 0 {
		t.Errorf("dry run must not call the API, got %d calls", len(mock.requests()))
	}
	if resp.Output.CreatedOn != "" || resp.Output.LastUpdated != "" || resp.Output.CollectionID != "" {
		t.Errorf("dry run must not fabricate server-assigned outputs: %+v", resp.Output)
	}
}

func TestCollectionResource_Create_ValidationError(t *testing.T) {
	mock := newCMSMock(t)

	_, err := (&CollectionResource{}).Create(context.Background(), infer.CreateRequest[CollectionArgs]{
		Inputs: CollectionArgs{SiteID: "invalid", DisplayName: "Blog Posts", SingularName: "Blog Post"},
	})
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("expected validation error, got %v", err)
	}
	if len(mock.requests()) != 0 {
		t.Errorf("validation failure must not call the API")
	}
}

func TestCollectionResource_Create_APIError(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodPost, "/v2/sites/"+testSiteID+"/collections", func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid collection configuration"})
	})

	_, err := (&CollectionResource{}).Create(context.Background(), infer.CreateRequest[CollectionArgs]{
		Inputs: CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post"},
	})
	if err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("expected bad request error, got %v", err)
	}
}

func TestCollectionResource_Create_RetriesRateLimit(t *testing.T) {
	mock := newCMSMock(t)
	attempts := 0
	mock.handle(http.MethodPost, "/v2/sites/"+testSiteID+"/collections", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeCMSJSON(w, http.StatusCreated, Collection{ID: testCollectionID})
	})

	resp, err := (&CollectionResource{}).Create(context.Background(), infer.CreateRequest[CollectionArgs]{
		Inputs: CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post"},
	})
	if err != nil {
		t.Fatalf("Create() should succeed after rate limit retry, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if resp.Output.CollectionID != testCollectionID {
		t.Errorf("CollectionID = %q", resp.Output.CollectionID)
	}
}

// =============================================================================
// Collection resource: Read
// =============================================================================

func TestCollectionResource_Read(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusOK, Collection{
			ID: testCollectionID, DisplayName: "Blog Posts", SingularName: "Blog Post", Slug: "blog-posts",
			CreatedOn: "2024-01-01T00:00:00Z", LastUpdated: "2024-01-02T00:00:00Z",
		})
	})
	resourceID := testSiteID + "/collections/" + testCollectionID

	t.Run("omitted slug stays omitted in inputs but is recorded in state", func(t *testing.T) {
		resp, err := (&CollectionResource{}).Read(context.Background(), infer.ReadRequest[CollectionArgs, CollectionState]{
			ID:     resourceID,
			Inputs: CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post"},
		})
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.ID != resourceID {
			t.Errorf("ID = %q, want %q", resp.ID, resourceID)
		}
		if resp.Inputs.Slug != "" {
			t.Errorf("Read must not overwrite an omitted slug input, got %q", resp.Inputs.Slug)
		}
		if resp.State.Slug != "blog-posts" || resp.State.CollectionID != testCollectionID ||
			resp.State.LastUpdated != "2024-01-02T00:00:00Z" || resp.State.DisplayName != "Blog Posts" {
			t.Errorf("unexpected state: %+v", resp.State)
		}
	})

	t.Run("explicit slug is refreshed from the API", func(t *testing.T) {
		resp, err := (&CollectionResource{}).Read(context.Background(), infer.ReadRequest[CollectionArgs, CollectionState]{
			ID:     resourceID,
			Inputs: CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post", Slug: "old"},
		})
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.Inputs.Slug != "blog-posts" {
			t.Errorf("explicit slug should reflect API drift, got %q", resp.Inputs.Slug)
		}
	})

	if got := mock.callsTo(http.MethodGet, "/v2/collections/"+testCollectionID); len(got) != 2 {
		t.Errorf("expected 2 GET calls, got %d", len(got))
	}
}

func TestCollectionResource_Read_NotFound(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusNotFound, map[string]string{"message": "Requested resource not found"})
	})

	resp, err := (&CollectionResource{}).Read(context.Background(), infer.ReadRequest[CollectionArgs, CollectionState]{
		ID: testSiteID + "/collections/" + testCollectionID,
	})
	if err != nil {
		t.Fatalf("Read() on 404 should not error, got %v", err)
	}
	if resp.ID != "" {
		t.Errorf("Read() on 404 should return empty ID, got %q", resp.ID)
	}
}

func TestCollectionResource_Read_ServerErrorIsNotTreatedAsMissing(t *testing.T) {
	mock := newCMSMock(t)
	mock.handle(http.MethodGet, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
		writeCMSJSON(w, http.StatusInternalServerError, map[string]string{"message": "collection not found in cache"})
	})

	_, err := (&CollectionResource{}).Read(context.Background(), infer.ReadRequest[CollectionArgs, CollectionState]{
		ID: testSiteID + "/collections/" + testCollectionID,
	})
	if err == nil {
		t.Fatal("expected error for 500 even when the body mentions 'not found'")
	}
}

func TestCollectionResource_Read_InvalidResourceID(t *testing.T) {
	mock := newCMSMock(t)
	for _, id := range []string{"", "bad/collections/" + testCollectionID, testSiteID + "/collections/../x"} {
		_, err := (&CollectionResource{}).Read(
			context.Background(),
			infer.ReadRequest[CollectionArgs, CollectionState]{ID: id},
		)
		if err == nil {
			t.Errorf("Read(%q) expected error", id)
		}
	}
	if len(mock.requests()) != 0 {
		t.Errorf("invalid IDs must be rejected before any API call, got %d calls", len(mock.requests()))
	}
}

// =============================================================================
// Collection resource: Delete
// =============================================================================

func TestCollectionResource_Delete(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"204 success", http.StatusNoContent, false},
		{"404 idempotent", http.StatusNotFound, false},
		{"500 error", http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newCMSMock(t)
			mock.handle(http.MethodDelete, "/v2/collections/"+testCollectionID, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})
			_, err := (&CollectionResource{}).Delete(context.Background(), infer.DeleteRequest[CollectionState]{
				ID: testSiteID + "/collections/" + testCollectionID,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(mock.callsTo(http.MethodDelete, "/v2/collections/"+testCollectionID)) != 1 {
				t.Errorf("expected one DELETE call, got %+v", mock.requests())
			}
		})
	}
}

func TestCollectionResource_Delete_InvalidResourceID(t *testing.T) {
	mock := newCMSMock(t)
	_, err := (&CollectionResource{}).Delete(context.Background(), infer.DeleteRequest[CollectionState]{
		ID: testSiteID + "/collections/not-a-valid-id",
	})
	if err == nil {
		t.Fatal("expected error for invalid collection ID")
	}
	if len(mock.requests()) != 0 {
		t.Errorf("invalid IDs must be rejected before any API call")
	}
}

// =============================================================================
// Collection resource: Diff
// =============================================================================

func TestCollectionDiff_OmittedSlugAfterRefreshDoesNotReplace(t *testing.T) {
	// State as recorded by Create/Read: Webflow generated the slug.
	state := CollectionState{
		CollectionArgs: CollectionArgs{
			SiteID:       testSiteID,
			DisplayName:  "Blog Posts",
			SingularName: "Blog Post",
			Slug:         "blog-posts",
		},
		CollectionID: testCollectionID,
	}
	// Program inputs: slug omitted.
	inputs := CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post"}

	resp, err := (&CollectionResource{}).Diff(context.Background(), infer.DiffRequest[CollectionArgs, CollectionState]{
		State: state, Inputs: inputs,
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if resp.HasChanges || len(resp.DetailedDiff) != 0 {
		t.Errorf("omitted slug must not diff against the generated slug: %+v", resp)
	}
}

func TestCollectionDiff_ExplicitSlugChangeReplaces(t *testing.T) {
	state := CollectionState{
		CollectionArgs: CollectionArgs{
			SiteID:       testSiteID,
			DisplayName:  "Blog Posts",
			SingularName: "Blog Post",
			Slug:         "blog-posts",
		},
	}
	inputs := CollectionArgs{SiteID: testSiteID, DisplayName: "Blog Posts", SingularName: "Blog Post", Slug: "posts"}

	resp, err := (&CollectionResource{}).Diff(context.Background(), infer.DiffRequest[CollectionArgs, CollectionState]{
		State: state, Inputs: inputs,
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if d, ok := resp.DetailedDiff["slug"]; !ok || d.Kind != p.UpdateReplace || !resp.DeleteBeforeReplace {
		t.Errorf("explicit slug change must replace: %+v", resp)
	}
}

// TestCollectionDiff_MultipleChanges tests that Diff accumulates all changes in DetailedDiff
func TestCollectionDiff_MultipleChanges(t *testing.T) {
	resource := &CollectionResource{}
	ctx := context.Background()

	tests := []struct {
		name         string
		oldState     CollectionState
		newInputs    CollectionArgs
		expectedKeys []string
	}{
		{
			name: "all fields changed",
			oldState: CollectionState{CollectionArgs: CollectionArgs{
				SiteID: "oldsite123456789012345678", DisplayName: "Old Name", SingularName: "Old Singular", Slug: "old-slug",
			}},
			newInputs: CollectionArgs{
				SiteID: "newsite123456789012345678", DisplayName: "New Name", SingularName: "New Singular", Slug: "new-slug",
			},
			expectedKeys: []string{"siteId", "displayName", "singularName", "slug"},
		},
		{
			name: "two fields changed",
			oldState: CollectionState{CollectionArgs: CollectionArgs{
				SiteID: "site123456789012345678901", DisplayName: "Old Name", SingularName: "Old Singular", Slug: "same-slug",
			}},
			newInputs: CollectionArgs{
				SiteID: "site123456789012345678901", DisplayName: "New Name", SingularName: "New Singular", Slug: "same-slug",
			},
			expectedKeys: []string{"displayName", "singularName"},
		},
		{
			name: "no changes",
			oldState: CollectionState{CollectionArgs: CollectionArgs{
				SiteID: "site123456789012345678901", DisplayName: "Same Name", SingularName: "Same Singular", Slug: "same-slug",
			}},
			newInputs: CollectionArgs{
				SiteID: "site123456789012345678901", DisplayName: "Same Name", SingularName: "Same Singular", Slug: "same-slug",
			},
			expectedKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resource.Diff(ctx, infer.DiffRequest[CollectionArgs, CollectionState]{
				State: tt.oldState, Inputs: tt.newInputs,
			})
			if err != nil {
				t.Fatalf("Diff() error = %v, want nil", err)
			}
			if len(result.DetailedDiff) != len(tt.expectedKeys) {
				t.Errorf(
					"Expected %d changes in DetailedDiff, got %d: %v",
					len(tt.expectedKeys),
					len(result.DetailedDiff),
					result.DetailedDiff,
				)
			}
			if len(tt.expectedKeys) > 0 {
				if !result.HasChanges || !result.DeleteBeforeReplace {
					t.Error("Expected HasChanges and DeleteBeforeReplace when changes detected")
				}
			} else if result.HasChanges {
				t.Error("Expected HasChanges=false when no changes")
			}
			for _, key := range tt.expectedKeys {
				if d, found := result.DetailedDiff[key]; !found || d.Kind != p.UpdateReplace {
					t.Errorf("Expected key %q with UpdateReplace in DetailedDiff, got %+v", key, result.DetailedDiff)
				}
			}
		})
	}
}
