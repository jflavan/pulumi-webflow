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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const (
	testAssetSiteID  = "5f0c8c9e1c9d440000e8d8c3"
	testAssetID      = "6789abcdef1234567890abcd"
	testAssetFolder  = "5f0c8c9e1c9d440000e8d8c9"
	testAssetToken   = "test-token-12345678901234567890"
	testAssetContent = "png-bytes-for-upload"
)

// assetMock stands in for both the Webflow API and S3 during an asset Create.
type assetMock struct {
	server        *httptest.Server
	createCalls   int
	uploadCalls   int
	deleteCalls   int
	createBody    AssetUploadRequest
	uploadFields  map[string]string
	uploadOrder   []string
	uploadFile    string
	uploadData    []byte
	uploadAuth    string
	uploadStatus  int
	createStatus  int
	uploadDetails map[string]string
	// onUpload, when set, runs when the S3 upload request arrives, before it is answered.
	onUpload func()
}

func newAssetMock(t *testing.T) *assetMock {
	t.Helper()
	m := &assetMock{
		uploadStatus: http.StatusCreated,
		createStatus: http.StatusAccepted,
		uploadDetails: map[string]string{
			"acl": "public-read", "bucket": "webflow-prod-assets", "key": "assets/logo.png",
			"Content-Type": "image/png", "Policy": "policy", "X-Amz-Signature": "sig", "success_action_status": "201",
		},
	}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sites/"+testAssetSiteID+"/assets":
			m.createCalls++
			_ = json.NewDecoder(r.Body).Decode(&m.createBody)
			w.WriteHeader(m.createStatus)
			_ = json.NewEncoder(w).Encode(AssetUploadResponse{
				ID: testAssetID, UploadURL: m.server.URL + "/s3-upload", UploadDetails: m.uploadDetails,
				AssetURL:    "https://s3.amazonaws.com/webflow-prod-assets/assets/logo.png",
				HostedURL:   "https://cdn.prod.website-files.com/" + testAssetSiteID + "/logo.png",
				ContentType: "image/png", OriginalFileName: "logo.png", ParentFolder: testAssetFolder,
				CreatedOn: "2025-01-12T10:00:00Z", LastUpdated: "2025-01-12T10:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/s3-upload":
			m.uploadCalls++
			if m.onUpload != nil {
				m.onUpload()
			}
			m.uploadAuth = r.Header.Get("Authorization")
			m.uploadFields, m.uploadOrder, m.uploadFile, m.uploadData = readMultipartParts(t, r)
			w.WriteHeader(m.uploadStatus)
		case r.Method == http.MethodDelete && r.URL.Path == "/v2/assets/"+testAssetID:
			m.deleteCalls++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(m.server.Close)
	useMockAPI(t, m.server)
	t.Setenv("WEBFLOW_API_TOKEN", testAssetToken)
	return m
}

func writeTempAsset(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAssetCreate_UploadsFile(t *testing.T) {
	m := newAssetMock(t)
	path := writeTempAsset(t, testAssetContent)
	wantHash := ComputeFileHash([]byte(testAssetContent))

	resp, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path, ParentFolder: testAssetFolder},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if m.createCalls != 1 || m.uploadCalls != 1 || m.deleteCalls != 0 {
		t.Fatalf("calls create=%d upload=%d delete=%d", m.createCalls, m.uploadCalls, m.deleteCalls)
	}
	if m.createBody.FileName != "logo.png" || m.createBody.FileHash != wantHash ||
		m.createBody.ParentFolder != testAssetFolder {
		t.Errorf("metadata request = %+v (want hash %s)", m.createBody, wantHash)
	}
	for k, v := range m.uploadDetails {
		if m.uploadFields[k] != v {
			t.Errorf("upload form field %s = %q, want %q", k, m.uploadFields[k], v)
		}
	}
	if len(m.uploadOrder) != len(m.uploadDetails)+1 || m.uploadOrder[len(m.uploadOrder)-1] != "file" {
		t.Errorf("upload part order = %v", m.uploadOrder)
	}
	if m.uploadFile != "logo.png" || string(m.uploadData) != testAssetContent {
		t.Errorf("upload file part = %q %q", m.uploadFile, m.uploadData)
	}
	if m.uploadAuth != "" {
		t.Errorf("S3 upload must not receive the bearer token, got %q", m.uploadAuth)
	}

	if resp.ID != GenerateAssetResourceID(testAssetSiteID, testAssetID) {
		t.Errorf("ID = %q", resp.ID)
	}
	out := resp.Output
	if out.AssetID != testAssetID || out.FileHash != wantHash || out.Size != len(testAssetContent) ||
		out.FolderID != testAssetFolder || out.ContentType != "image/png" ||
		out.HostedURL == "" || out.AssetURL == "" || out.UploadURL != m.server.URL+"/s3-upload" ||
		out.UploadDetails["Policy"] != "policy" || out.CreatedOn == "" {
		t.Errorf("unexpected output state %+v", out)
	}
}

func TestAssetCreate_RemoteSource(t *testing.T) {
	m := newAssetMock(t)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testAssetContent))
	}))
	defer source.Close()

	resp, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: source.URL + "/logo.png"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if string(m.uploadData) != testAssetContent || resp.Output.FileHash != ComputeFileHash([]byte(testAssetContent)) {
		t.Errorf("remote content was not uploaded: %q / %s", m.uploadData, resp.Output.FileHash)
	}
	if m.createBody.ParentFolder != "" {
		t.Errorf("parentFolder should be omitted, got %q", m.createBody.ParentFolder)
	}
}

func TestAssetCreate_ExplicitHashMustMatch(t *testing.T) {
	m := newAssetMock(t)
	path := writeTempAsset(t, testAssetContent)

	_, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{
			SiteID:     testAssetSiteID,
			FileName:   "logo.png",
			FileSource: path,
			FileHash:   "d41d8cd98f00b204e9800998ecf8427e",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected hash mismatch error, got %v", err)
	}
	if m.createCalls != 0 {
		t.Error("no API call should be made when the hash mismatches")
	}

	// A matching explicit hash (any case) is accepted.
	resp, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{
			SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path,
			FileHash: strings.ToUpper(ComputeFileHash([]byte(testAssetContent))),
		},
	})
	if err != nil {
		t.Fatalf("Create with matching hash: %v", err)
	}
	if resp.Output.FileHash != ComputeFileHash([]byte(testAssetContent)) {
		t.Errorf("state hash should be normalised lowercase, got %s", resp.Output.FileHash)
	}
}

func TestAssetCreate_UploadFailureCleansUpMetadata(t *testing.T) {
	m := newAssetMock(t)
	m.uploadStatus = http.StatusForbidden
	path := writeTempAsset(t, testAssetContent)

	_, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to upload asset file") {
		t.Fatalf("expected upload failure, got %v", err)
	}
	if m.deleteCalls != 1 {
		t.Errorf("expected orphaned metadata to be deleted, delete calls = %d", m.deleteCalls)
	}
}

// TestAssetCreate_CancelledApplyStillCleansUp verifies that when the apply context is cancelled
// while the file is being uploaded (the metadata already exists), the orphaned metadata is
// still deleted on a context detached from the cancellation.
func TestAssetCreate_CancelledApplyStillCleansUp(t *testing.T) {
	m := newAssetMock(t)
	path := writeTempAsset(t, testAssetContent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.uploadStatus = http.StatusForbidden
	m.onUpload = cancel // the apply is cancelled mid-upload

	_, err := (&Asset{}).Create(ctx, infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to upload asset file") {
		t.Fatalf("expected the upload to fail, got %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("test setup: the apply context should have been cancelled during the upload")
	}
	if m.deleteCalls != 1 {
		t.Errorf("orphaned metadata must be deleted despite the cancelled apply, delete calls = %d", m.deleteCalls)
	}
}

func TestAssetCreate_APIError(t *testing.T) {
	m := newAssetMock(t)
	m.createStatus = http.StatusBadRequest
	path := writeTempAsset(t, testAssetContent)

	_, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to create asset") ||
		!strings.Contains(err.Error(), "bad request") {
		t.Fatalf("expected bad request error, got %v", err)
	}
	if m.uploadCalls != 0 {
		t.Error("upload must not happen when metadata creation fails")
	}
}

func TestAssetCreate_DryRunSkipsValidationAndAPI(t *testing.T) {
	m := newAssetMock(t)

	// Unknown inputs arrive as zero values during preview; nothing may be validated or called.
	resp, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{
		Inputs: AssetArgs{SiteID: "", FileName: "", FileSource: ""},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run should not fail: %v", err)
	}
	if resp.ID != "" {
		t.Errorf("dry run must not fabricate an ID, got %q", resp.ID)
	}
	if resp.Output.AssetID != "" || resp.Output.HostedURL != "" || resp.Output.CreatedOn != "" ||
		resp.Output.FileHash != "" {
		t.Errorf("dry run must not fabricate outputs: %+v", resp.Output)
	}
	if m.createCalls != 0 || m.uploadCalls != 0 {
		t.Error("dry run must not call the API")
	}
}

func TestAssetCreate_ValidationAfterDryRun(t *testing.T) {
	m := newAssetMock(t)
	path := writeTempAsset(t, testAssetContent)

	tests := []struct {
		name   string
		inputs AssetArgs
		want   string
	}{
		{"invalid siteId", AssetArgs{SiteID: "invalid", FileName: "logo.png", FileSource: path}, "siteId has invalid format"},
		{"missing fileName", AssetArgs{SiteID: testAssetSiteID, FileName: "", FileSource: path}, "fileName is required"},
		{
			"invalid fileName",
			AssetArgs{SiteID: testAssetSiteID, FileName: "logo<>.png", FileSource: path},
			"invalid character",
		},
		{"missing fileSource", AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png"}, "fileSource is required"},
		{
			"invalid parentFolder",
			AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path, ParentFolder: "nope"},
			"assetFolderId has invalid format",
		},
		{
			"invalid fileHash",
			AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path, FileHash: "xyz"},
			"fileHash has invalid format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&Asset{}).Create(context.Background(), infer.CreateRequest[AssetArgs]{Inputs: tt.inputs})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
	if m.createCalls != 0 {
		t.Error("validation failures must not reach the API")
	}
}

func TestAssetRead_Success(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testAssetToken)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/assets/"+testAssetID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(AssetResponse{
			ID: testAssetID, ContentType: "image/png", Size: 12345, SiteID: testAssetSiteID,
			HostedURL: "https://cdn.prod.website-files.com/x/logo.png", OriginalFileName: "logo.png",
			CreatedOn: "2025-01-12T10:00:00Z", LastUpdated: "2025-01-13T10:00:00Z",
		})
	}))
	defer server.Close()
	useMockAPI(t, server)

	id := GenerateAssetResourceID(testAssetSiteID, testAssetID)
	prev := AssetState{
		AssetArgs: AssetArgs{
			SiteID:       testAssetSiteID,
			FileName:     "logo.png",
			FileSource:   "./logo.png",
			FileHash:     "abc",
			ParentFolder: testAssetFolder,
		},
		AssetID:       testAssetID,
		UploadURL:     "https://s3/upload",
		UploadDetails: map[string]string{"key": "k"},
		AssetURL:      "https://s3/asset",
		FolderID:      testAssetFolder,
	}
	resp, err := (&Asset{}).Read(context.Background(), infer.ReadRequest[AssetArgs, AssetState]{ID: id, State: prev})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if resp.ID != id {
		t.Errorf("ID = %q", resp.ID)
	}
	s := resp.State
	if s.Size != 12345 || s.HostedURL != "https://cdn.prod.website-files.com/x/logo.png" ||
		s.LastUpdated != "2025-01-13T10:00:00Z" {
		t.Errorf("API values not applied: %+v", s)
	}
	if s.FileHash != "abc" || s.FileSource != "./logo.png" || s.ParentFolder != testAssetFolder ||
		s.UploadURL != "https://s3/upload" || s.UploadDetails["key"] != "k" || s.AssetURL != "https://s3/asset" ||
		s.FolderID != testAssetFolder {
		t.Errorf("state values not carried through: %+v", s)
	}
	if resp.Inputs.FileName != "logo.png" {
		t.Errorf("inputs = %+v", resp.Inputs)
	}
}

// TestAssetRead_ImportPopulatesFromAPI verifies that an import (empty state) captures the
// identity fields from the API and leaves fileSource/fileHash empty for Diff to adopt later.
func TestAssetRead_ImportPopulatesFromAPI(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testAssetToken)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(AssetResponse{
			ID: testAssetID, ContentType: "image/png", Size: 42, SiteID: testAssetSiteID,
			HostedURL: "https://cdn.prod.website-files.com/x/logo.png", OriginalFileName: "logo.png",
			FolderID: testAssetFolder,
		})
	}))
	defer server.Close()
	useMockAPI(t, server)

	id := GenerateAssetResourceID(testAssetSiteID, testAssetID)
	resp, err := (&Asset{}).Read(context.Background(), infer.ReadRequest[AssetArgs, AssetState]{ID: id})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	in := resp.Inputs
	if in.SiteID != testAssetSiteID || in.FileName != "logo.png" || in.ParentFolder != testAssetFolder {
		t.Errorf("import should capture identity fields from the API: %+v", in)
	}
	if in.FileSource != "" || in.FileHash != "" {
		t.Errorf("import cannot know fileSource/fileHash, got %+v", in)
	}
	if resp.State.AssetID != testAssetID || resp.State.FolderID != testAssetFolder || resp.State.Size != 42 {
		t.Errorf("unexpected state %+v", resp.State)
	}
}

func TestAssetCheck(t *testing.T) {
	inputs := property.NewMap(map[string]property.Value{
		"siteId":       property.New("bad"),
		"fileName":     property.New(strings.Repeat("a", 100)),
		"fileSource":   property.New("  "),
		"parentFolder": property.New("nope"),
		"fileHash":     property.New("xyz"),
	})
	resp, err := (&Asset{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	got := map[string]bool{}
	for _, f := range resp.Failures {
		got[f.Property] = true
	}
	for _, want := range []string{"siteId", "fileName", "fileSource", "parentFolder", "fileHash"} {
		if !got[want] {
			t.Errorf("expected a failure on %q, got %+v", want, resp.Failures)
		}
	}
	if len(resp.Failures) != 5 {
		t.Errorf("expected 5 failures, got %+v", resp.Failures)
	}

	unknown := property.NewMap(map[string]property.Value{
		"siteId":       property.New(property.Computed),
		"fileName":     property.New(property.Computed),
		"fileSource":   property.New(property.Computed),
		"parentFolder": property.New(property.Computed),
	})
	resp, err = (&Asset{}).Check(context.Background(), infer.CheckRequest{NewInputs: unknown})
	if err != nil || len(resp.Failures) != 0 {
		t.Errorf("unknown inputs must not fail Check: %+v %v", resp.Failures, err)
	}

	valid := property.NewMap(map[string]property.Value{
		"siteId":       property.New(testAssetSiteID),
		"fileName":     property.New("logo.png"),
		"fileSource":   property.New("./logo.png"),
		"parentFolder": property.New(testAssetFolder),
	})
	resp, err = (&Asset{}).Check(context.Background(), infer.CheckRequest{NewInputs: valid})
	if err != nil || len(resp.Failures) != 0 {
		t.Errorf("valid inputs must pass Check: %+v %v", resp.Failures, err)
	}
	if resp.Inputs.SiteID != testAssetSiteID || resp.Inputs.FileSource != "./logo.png" {
		t.Errorf("inputs not decoded: %+v", resp.Inputs)
	}
}

// TestAssetCreate_UploadOutputsAreSecret drives a real Create through the infer server and
// checks that the presigned upload URL and form fields come back marked secret, regardless of
// how the calling SDK flags them.
func TestAssetCreate_UploadOutputsAreSecret(t *testing.T) {
	newAssetMock(t)
	path := writeTempAsset(t, testAssetContent)

	srv, err := integration.NewServer(context.Background(), Name, semver.MustParse("0.0.1"),
		integration.WithProvider(Provider()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{
		"apiToken": property.New(testAssetToken),
	})}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	resp, err := srv.Create(p.CreateRequest{
		Urn: resource.NewURN("stack", "proj", "", tokens.Type(Name+":index:Asset"), "logo"),
		Properties: property.NewMap(map[string]property.Value{
			"siteId":     property.New(testAssetSiteID),
			"fileName":   property.New("logo.png"),
			"fileSource": property.New(path),
		}),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, key := range []string{"uploadUrl", "uploadDetails"} {
		v, ok := resp.Properties.GetOk(key)
		if !ok {
			t.Errorf("output %q missing: %v", key, resp.Properties)
			continue
		}
		if !v.Secret() {
			t.Errorf("output %q must be secret, got %#v", key, v)
		}
	}
	for _, key := range []string{"hostedUrl", "assetId", "fileHash"} {
		if v, ok := resp.Properties.GetOk(key); !ok || v.Secret() {
			t.Errorf("output %q should be present and public, got %#v (ok=%v)", key, v, ok)
		}
	}
}

func TestAssetRead_NotFoundAndErrors(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testAssetToken)
	status := http.StatusNotFound
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()
	useMockAPI(t, server)

	req := infer.ReadRequest[AssetArgs, AssetState]{ID: GenerateAssetResourceID(testAssetSiteID, testAssetID)}
	resp, err := (&Asset{}).Read(context.Background(), req)
	if err != nil || resp.ID != "" {
		t.Fatalf("404 should clear the resource: id=%q err=%v", resp.ID, err)
	}

	// A 500 whose body says "not found" must be an error, not a deletion.
	status = http.StatusInternalServerError
	if _, err := (&Asset{}).Read(context.Background(), req); err == nil {
		t.Fatal("500 must propagate as an error")
	}
}

func TestAssetRead_InvalidIDsRejectedBeforeAPI(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testAssetToken)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("no API call expected, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	useMockAPI(t, server)

	for _, id := range []string{"", "bad", "site/assets/../x", testAssetSiteID + "/assets/not-hex"} {
		if _, err := (&Asset{}).Read(context.Background(), infer.ReadRequest[AssetArgs, AssetState]{ID: id}); err == nil {
			t.Errorf("expected error for id %q", id)
		}
	}
	if _, err := (&Asset{}).Delete(
		context.Background(), infer.DeleteRequest[AssetState]{ID: testAssetSiteID + "/assets/nope"},
	); err == nil {
		t.Error("Delete should reject an invalid asset ID")
	}
}

func TestAssetDelete(t *testing.T) {
	t.Setenv("WEBFLOW_API_TOKEN", testAssetToken)
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete || r.URL.Path != "/v2/assets/"+testAssetID {
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(status)
		}))
		useMockAPI(t, server)
		_, err := (&Asset{}).Delete(context.Background(), infer.DeleteRequest[AssetState]{
			ID: GenerateAssetResourceID(testAssetSiteID, testAssetID),
		})
		server.Close()
		if err != nil {
			t.Errorf("Delete with %d: %v", status, err)
		}
	}
}

func TestAssetUpdate_Unsupported(t *testing.T) {
	if _, err := (&Asset{}).Update(context.Background(), infer.UpdateRequest[AssetArgs, AssetState]{}); err == nil {
		t.Fatal("Update must return an error")
	}
}

func TestAssetDiff(t *testing.T) {
	path := writeTempAsset(t, testAssetContent)
	hash := ComputeFileHash([]byte(testAssetContent))
	base := AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", FileSource: path, ParentFolder: testAssetFolder}
	state := AssetState{AssetArgs: base, AssetID: testAssetID}
	state.FileHash = hash

	t.Run("no changes", func(t *testing.T) {
		resp, err := (&Asset{}).Diff(
			context.Background(),
			infer.DiffRequest[AssetArgs, AssetState]{Inputs: base, State: state},
		)
		if err != nil || resp.HasChanges {
			t.Fatalf("expected no changes: %+v %v", resp, err)
		}
	})

	t.Run("explicit matching hash is not a change", func(t *testing.T) {
		in := base
		in.FileHash = strings.ToUpper(hash)
		resp, err := (&Asset{}).Diff(context.Background(), infer.DiffRequest[AssetArgs, AssetState]{Inputs: in, State: state})
		if err != nil || resp.HasChanges {
			t.Fatalf("expected no changes: %+v %v", resp, err)
		}
	})

	replacements := []struct {
		field  string
		modify func(a *AssetArgs)
	}{
		{"siteId", func(a *AssetArgs) { a.SiteID = "6f1d9d0f2d0e551111f9e9d4" }},
		{"fileName", func(a *AssetArgs) { a.FileName = "hero.jpg" }},
		{"parentFolder", func(a *AssetArgs) { a.ParentFolder = "" }},
		{"fileSource", func(a *AssetArgs) { a.FileSource = "https://example.com/logo.png" }},
		{"fileHash", func(a *AssetArgs) { a.FileHash = "e9800998ecf8427ed41d8cd98f00b204" }},
	}
	for _, tt := range replacements {
		t.Run(tt.field+" replaces", func(t *testing.T) {
			in := base
			tt.modify(&in)
			resp, err := (&Asset{}).Diff(
				context.Background(),
				infer.DiffRequest[AssetArgs, AssetState]{Inputs: in, State: state},
			)
			if err != nil {
				t.Fatal(err)
			}
			if !resp.HasChanges || resp.DetailedDiff[tt.field].Kind != p.UpdateReplace {
				t.Errorf("expected %s UpdateReplace, got %+v", tt.field, resp)
			}
		})
	}

	t.Run("local file content change replaces", func(t *testing.T) {
		if err := os.WriteFile(path, []byte("new-content"), 0o600); err != nil {
			t.Fatal(err)
		}
		resp, err := (&Asset{}).Diff(
			context.Background(),
			infer.DiffRequest[AssetArgs, AssetState]{Inputs: base, State: state},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !resp.HasChanges || resp.DetailedDiff["fileHash"].Kind != p.UpdateReplace {
			t.Errorf("expected fileHash UpdateReplace, got %+v", resp)
		}
	})

	t.Run("missing local file skips the content comparison", func(t *testing.T) {
		in := base
		in.FileSource = filepath.Join(t.TempDir(), "missing.png")
		st := state
		st.FileSource = in.FileSource
		resp, err := (&Asset{}).Diff(
			context.Background(), infer.DiffRequest[AssetArgs, AssetState]{Inputs: in, State: st},
		)
		if err != nil {
			t.Fatalf("a missing file must not fail the preview: %v", err)
		}
		if resp.HasChanges {
			t.Errorf("a missing file must not be reported as a change: %+v", resp)
		}
	})

	t.Run("unreadable local file is still an error", func(t *testing.T) {
		in := base
		in.FileSource = t.TempDir() // a directory exists but cannot be hashed
		st := state
		st.FileSource = in.FileSource
		if _, err := (&Asset{}).Diff(
			context.Background(), infer.DiffRequest[AssetArgs, AssetState]{Inputs: in, State: st},
		); err == nil || !strings.Contains(err.Error(), "directory") {
			t.Errorf("expected directory error, got %v", err)
		}
	})

	t.Run("imported state without fileSource and fileHash is not a change", func(t *testing.T) {
		// After 'pulumi import' the state carries only what GET /v2/assets returns.
		imported := AssetState{
			AssetArgs: AssetArgs{SiteID: testAssetSiteID, FileName: "logo.png", ParentFolder: testAssetFolder},
			AssetID:   testAssetID,
		}
		remote := base
		remote.FileSource = "https://example.invalid/logo.png"
		explicit := base
		explicit.FileHash = "e9800998ecf8427ed41d8cd98f00b204"
		for name, in := range map[string]AssetArgs{"local source": base, "remote source": remote, "explicit hash": explicit} {
			resp, err := (&Asset{}).Diff(
				context.Background(), infer.DiffRequest[AssetArgs, AssetState]{Inputs: in, State: imported},
			)
			if err != nil || resp.HasChanges {
				t.Errorf("%s: imported asset must not be replaced: %+v %v", name, resp, err)
			}
		}
		// Identity fields are still compared after an import.
		other := base
		other.FileName = "other.png"
		resp, err := (&Asset{}).Diff(
			context.Background(), infer.DiffRequest[AssetArgs, AssetState]{Inputs: other, State: imported},
		)
		if err != nil || !resp.HasChanges || resp.DetailedDiff["fileName"].Kind != p.UpdateReplace {
			t.Errorf("fileName change after import must still replace: %+v %v", resp, err)
		}
	})

	t.Run("remote source without explicit hash is not fetched", func(t *testing.T) {
		in := base
		in.FileSource = "https://example.invalid/logo.png"
		st := state
		st.FileSource = in.FileSource
		resp, err := (&Asset{}).Diff(context.Background(), infer.DiffRequest[AssetArgs, AssetState]{Inputs: in, State: st})
		if err != nil || resp.HasChanges {
			t.Fatalf("expected no network and no changes: %+v %v", resp, err)
		}
	})
}
