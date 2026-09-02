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

const customCodeTestPageID = "63c720f9347c2139b248e552"

func TestValidatePageID(t *testing.T) {
	tests := []struct {
		name    string
		pageID  string
		wantErr bool
	}{
		{"valid page ID", customCodeTestPageID, false},
		{"empty page ID", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePageID(tt.pageID); (err != nil) != tt.wantErr {
				t.Errorf("ValidatePageID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateScriptID(t *testing.T) {
	if err := ValidateScriptID("cms_slider"); err != nil {
		t.Errorf("ValidateScriptID(valid) = %v", err)
	}
	if err := ValidateScriptID(""); err == nil {
		t.Error("ValidateScriptID(\"\") should fail")
	}
}

func TestValidateScriptVersion(t *testing.T) {
	for _, v := range []string{"1.0.0", "2.5.3"} {
		if err := ValidateScriptVersion(v); err != nil {
			t.Errorf("ValidateScriptVersion(%q) = %v", v, err)
		}
	}
	if err := ValidateScriptVersion(""); err == nil {
		t.Error("ValidateScriptVersion(\"\") should fail")
	}
}

func TestValidateScriptLocation(t *testing.T) {
	tests := []struct {
		location string
		wantErr  bool
	}{
		{"header", false}, {"footer", false}, {"body", true}, {"", true},
	}
	for _, tt := range tests {
		t.Run(tt.location, func(t *testing.T) {
			if err := ValidateScriptLocation(tt.location); (err != nil) != tt.wantErr {
				t.Errorf("ValidateScriptLocation(%q) error = %v, wantErr %v", tt.location, err, tt.wantErr)
			}
		})
	}
}

func TestGeneratePageCustomCodeResourceID(t *testing.T) {
	if got := GeneratePageCustomCodeResourceID(customCodeTestPageID); got != customCodeTestPageID+"/custom-code" {
		t.Errorf("GeneratePageCustomCodeResourceID() = %v", got)
	}
}

func TestExtractPageIDFromPageCustomCodeResourceID(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		wantPageID string
		wantErr    bool
	}{
		{"valid resource ID", customCodeTestPageID + "/custom-code", customCodeTestPageID, false},
		{"empty resource ID", "", "", true},
		{"invalid format - missing suffix", customCodeTestPageID, "", true},
		{"invalid format - wrong suffix", customCodeTestPageID + "/content", "", true},
		{"invalid format - only suffix", "/custom-code", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pageID, err := ExtractPageIDFromPageCustomCodeResourceID(tt.resourceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractPageIDFromPageCustomCodeResourceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if pageID != tt.wantPageID {
				t.Errorf("ExtractPageIDFromPageCustomCodeResourceID() pageID = %v, want %v", pageID, tt.wantPageID)
			}
		})
	}
}

func TestGetPageCustomCode(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		response   CustomCodeResponse
		wantErr    bool
		wantNotFnd bool
	}{
		{
			name: "successful GET", status: http.StatusOK,
			response: CustomCodeResponse{
				Scripts: []CustomCodeScript{{
					ID: "cms_slider", Version: "1.0.0", Location: "header",
					Attributes: map[string]interface{}{"my-attribute": "some-value"},
				}},
				LastUpdated: "2022-10-26T00:28:54.191Z", CreatedOn: "2022-10-26T00:28:54.191Z",
			},
		},
		{name: "empty scripts", status: http.StatusOK, response: CustomCodeResponse{Scripts: []CustomCodeScript{}}},
		{name: "page not found", status: http.StatusNotFound, wantErr: true, wantNotFnd: true},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v2/pages/"+customCodeTestPageID+"/custom_code" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if tt.status == http.StatusOK {
					writeJSON(t, w, tt.status, tt.response)
					return
				}
				w.WriteHeader(tt.status)
			})
			resp, err := GetPageCustomCode(context.Background(), client, customCodeTestPageID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetPageCustomCode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if IsNotFound(err) != tt.wantNotFnd {
				t.Errorf("IsNotFound(%v) = %v, want %v", err, IsNotFound(err), tt.wantNotFnd)
			}
			if !tt.wantErr && len(resp.Scripts) != len(tt.response.Scripts) {
				t.Errorf("GetPageCustomCode() scripts = %+v", resp.Scripts)
			}
		})
	}
}

func TestPutPageCustomCode(t *testing.T) {
	scripts := []CustomCodeScript{{
		ID: "cms_slider", Version: "1.0.0", Location: "header", Attributes: map[string]interface{}{"a": "b"},
	}}
	client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v2/pages/"+customCodeTestPageID+"/custom_code" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		// Regression: the old implementation sent no Content-Type header on PUT.
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var req CustomCodeRequest
		decodeJSONBody(t, r, &req)
		if len(req.Scripts) != 1 || req.Scripts[0].ID != "cms_slider" || req.Scripts[0].Attributes["a"] != "b" {
			t.Errorf("unexpected request body: %+v", req)
		}
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{Scripts: req.Scripts, LastUpdated: "2022-10-26T00:28:54.191Z"})
	})

	resp, err := PutPageCustomCode(context.Background(), client, customCodeTestPageID, scripts)
	if err != nil {
		t.Fatalf("PutPageCustomCode() error = %v", err)
	}
	if len(resp.Scripts) != 1 || resp.LastUpdated == "" {
		t.Errorf("PutPageCustomCode() = %+v", resp)
	}
}

func TestDeletePageCustomCode(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("Expected DELETE, got %s", r.Method)
				}
				w.WriteHeader(status)
			})
			if err := DeletePageCustomCode(context.Background(), client, customCodeTestPageID); err != nil {
				t.Fatalf("DeletePageCustomCode() error = %v", err)
			}
		})
	}
}

// TestPageCustomCodeCreate_DryRun_WithUnknownScriptIDs verifies that preview succeeds
// when script IDs are unknown (empty strings from the infer framework).
func TestPageCustomCodeCreate_DryRun_WithUnknownScriptIDs(t *testing.T) {
	resource := &PageCustomCode{}
	resp, err := resource.Create(context.Background(), infer.CreateRequest[PageCustomCodeArgs]{
		Inputs: PageCustomCodeArgs{PageID: "", Scripts: []PageCustomCodeScript{{}}},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Create() DryRun with unknown inputs should succeed, got error: %v", err)
	}
	if resp.ID == "" || resp.Output.LastUpdated == "" || resp.Output.CreatedOn == "" {
		t.Errorf("Create() DryRun should return ID and timestamps, got %+v", resp)
	}
}

// TestPageCustomCodeUpdate_DryRun_WithUnknownScriptIDs verifies that preview succeeds
// for updates when script IDs are unknown.
func TestPageCustomCodeUpdate_DryRun_WithUnknownScriptIDs(t *testing.T) {
	resource := &PageCustomCode{}
	resp, err := resource.Update(context.Background(), infer.UpdateRequest[PageCustomCodeArgs, PageCustomCodeState]{
		ID:     customCodeTestPageID + "/custom-code",
		Inputs: PageCustomCodeArgs{PageID: customCodeTestPageID, Scripts: []PageCustomCodeScript{{}}},
		State: PageCustomCodeState{PageCustomCodeArgs: PageCustomCodeArgs{
			PageID:  customCodeTestPageID,
			Scripts: []PageCustomCodeScript{{ID: "old_script", Version: "1.0.0", Location: "header"}},
		}},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Update() DryRun with unknown inputs should succeed, got error: %v", err)
	}
	if resp.Output.LastUpdated == "" {
		t.Error("Update() DryRun should set LastUpdated timestamp")
	}
}

func TestPageCustomCodeCreate_ValidationErrors(t *testing.T) {
	resource := &PageCustomCode{}
	valid := PageCustomCodeScript{ID: "cms_slider", Version: "1.0.0", Location: "header"}
	tests := []struct {
		name   string
		inputs PageCustomCodeArgs
		want   string
	}{
		{"invalid pageId", PageCustomCodeArgs{PageID: "", Scripts: []PageCustomCodeScript{valid}}, "validation failed"},
		{"no scripts", PageCustomCodeArgs{PageID: customCodeTestPageID}, "at least one script"},
		{
			"missing version",
			PageCustomCodeArgs{PageID: customCodeTestPageID, Scripts: []PageCustomCodeScript{{ID: "x", Location: "header"}}},
			"scripts[0]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resource.Create(context.Background(), infer.CreateRequest[PageCustomCodeArgs]{Inputs: tt.inputs})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Create() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestPageCustomCodeDiff(t *testing.T) {
	resource := &PageCustomCode{}
	nested := func(theme string) map[string]interface{} {
		return map[string]interface{}{"data-config": map[string]interface{}{"theme": theme, "tags": []interface{}{"x"}}}
	}
	scriptA := PageCustomCodeScript{ID: "a", Version: "1.0.0", Location: "header", Attributes: nested("dark")}
	scriptB := PageCustomCodeScript{ID: "b", Version: "2.0.0", Location: "footer"}
	aHeader := PageCustomCodeScript{ID: "a", Version: "1.0.0", Location: "header"}
	aFooter := PageCustomCodeScript{ID: "a", Version: "1.0.0", Location: "footer"}
	args := func(scripts ...PageCustomCodeScript) PageCustomCodeArgs {
		return PageCustomCodeArgs{PageID: customCodeTestPageID, Scripts: scripts}
	}

	tests := []struct {
		name        string
		state       PageCustomCodeArgs
		inputs      PageCustomCodeArgs
		wantChanges bool
		wantKey     string
		wantKind    p.DiffKind
	}{
		{
			name:   "no change",
			state:  args(scriptA, scriptB),
			inputs: args(scriptA, scriptB),
		},
		{
			name:   "reordered scripts are not a change",
			state:  args(scriptA, scriptB),
			inputs: args(scriptB, scriptA),
		},
		{
			name:   "nil and empty attributes are not a change",
			state:  args(PageCustomCodeScript{ID: "b", Version: "2.0.0", Location: "footer", Attributes: map[string]interface{}{}}),
			inputs: args(scriptB),
		},
		{
			name:        "nested attribute value changed",
			state:       args(scriptA),
			inputs:      args(PageCustomCodeScript{ID: "a", Version: "1.0.0", Location: "header", Attributes: nested("light")}),
			wantChanges: true, wantKey: "scripts", wantKind: p.Update,
		},
		{
			name:        "same id at both locations vs one location",
			state:       args(aHeader, aFooter),
			inputs:      args(aHeader, aHeader),
			wantChanges: true, wantKey: "scripts", wantKind: p.Update,
		},
		{
			name:        "script added",
			state:       args(scriptA),
			inputs:      args(scriptA, scriptB),
			wantChanges: true, wantKey: "scripts", wantKind: p.Update,
		},
		{
			name:        "pageId changed requires replacement",
			state:       args(scriptA),
			inputs:      PageCustomCodeArgs{PageID: "63c720f9347c2139b248e553", Scripts: []PageCustomCodeScript{scriptA}},
			wantChanges: true, wantKey: "pageId", wantKind: p.UpdateReplace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resource.Diff(context.Background(), infer.DiffRequest[PageCustomCodeArgs, PageCustomCodeState]{
				ID:     GeneratePageCustomCodeResourceID(tt.state.PageID),
				Inputs: tt.inputs,
				State:  PageCustomCodeState{PageCustomCodeArgs: tt.state},
			})
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if resp.HasChanges != tt.wantChanges {
				t.Fatalf("Diff() HasChanges = %v, want %v (detail: %+v)", resp.HasChanges, tt.wantChanges, resp.DetailedDiff)
			}
			if tt.wantChanges {
				d, ok := resp.DetailedDiff[tt.wantKey]
				if !ok || d.Kind != tt.wantKind {
					t.Errorf("Diff() DetailedDiff[%s] = %+v (present=%v), want kind %v", tt.wantKey, d, ok, tt.wantKind)
				}
			}
		})
	}
}

func TestPageCustomCodeCreate(t *testing.T) {
	var got CustomCodeRequest
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v2/pages/"+customCodeTestPageID+"/custom_code" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing Content-Type header")
		}
		decodeJSONBody(t, r, &got)
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{
			Scripts: got.Scripts, LastUpdated: "2025-02-01T00:00:00Z", CreatedOn: "2025-01-01T00:00:00Z",
		})
	})

	resource := &PageCustomCode{}
	resp, err := resource.Create(context.Background(), infer.CreateRequest[PageCustomCodeArgs]{Inputs: PageCustomCodeArgs{
		PageID: customCodeTestPageID,
		Scripts: []PageCustomCodeScript{{
			ID: "cms_slider", Version: "1.0.0", Location: "footer", Attributes: map[string]interface{}{"data-a": "1"},
		}},
	}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.ID != customCodeTestPageID+"/custom-code" || resp.Output.CreatedOn != "2025-01-01T00:00:00Z" {
		t.Errorf("Create() = %+v", resp)
	}
	if len(got.Scripts) != 1 || got.Scripts[0].Location != "footer" || got.Scripts[0].Attributes["data-a"] != "1" {
		t.Errorf("request body = %+v", got)
	}
}

func TestPageCustomCodeRead(t *testing.T) {
	resource := &PageCustomCode{}
	readReq := infer.ReadRequest[PageCustomCodeArgs, PageCustomCodeState]{
		ID: customCodeTestPageID + "/custom-code",
		State: PageCustomCodeState{PageCustomCodeArgs: PageCustomCodeArgs{
			PageID: customCodeTestPageID, Scripts: []PageCustomCodeScript{{ID: "stale", Version: "0.0.1", Location: "header"}},
		}},
	}

	t.Run("reads scripts back from the API (drift and import)", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v2/pages/"+customCodeTestPageID+"/custom_code" {
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			writeJSON(t, w, http.StatusOK, CustomCodeResponse{
				Scripts: []CustomCodeScript{{
					ID: "a", Version: "1.0.0", Location: "header", Attributes: map[string]interface{}{"k": "v"},
				}},
				LastUpdated: "2025-02-01T00:00:00Z", CreatedOn: "2025-01-01T00:00:00Z",
			})
		})
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.ID != readReq.ID || resp.Inputs.PageID != customCodeTestPageID {
			t.Errorf("Read() ID/pageId = %s/%s", resp.ID, resp.Inputs.PageID)
		}
		if len(resp.State.Scripts) != 1 || resp.State.Scripts[0].ID != "a" || resp.State.Scripts[0].Attributes["k"] != "v" {
			t.Errorf("Read() must return the API scripts, not the stale state: %+v", resp.State.Scripts)
		}
		if len(resp.Inputs.Scripts) != 1 || resp.Inputs.Scripts[0].ID != "a" {
			t.Errorf("Read() inputs must reflect the API scripts for import: %+v", resp.Inputs.Scripts)
		}
	})

	t.Run("404 signals deletion", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
		resp, err := resource.Read(context.Background(), readReq)
		if err != nil || resp.ID != "" {
			t.Fatalf("Read() = (%q, %v), want empty ID and nil error", resp.ID, err)
		}
	})

	t.Run("500 does not return an empty ID", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"internal error"}`))
		})
		resp, err := resource.Read(context.Background(), readReq)
		if err == nil {
			t.Fatalf("Read() should return an error for 500, got ID %q", resp.ID)
		}
	})

	t.Run("unauthorized is an error", func(t *testing.T) {
		mockAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
		if _, err := resource.Read(context.Background(), readReq); err == nil {
			t.Fatal("Read() should return an error for 401")
		}
	})

	t.Run("invalid page id in resource id", func(t *testing.T) {
		mockAPI(t, func(_ http.ResponseWriter, _ *http.Request) { t.Error("no API call expected") })
		_, err := resource.Read(context.Background(),
			infer.ReadRequest[PageCustomCodeArgs, PageCustomCodeState]{ID: "nope/custom-code"})
		if err == nil || !strings.Contains(err.Error(), "invalid resource ID") {
			t.Fatalf("Read() error = %v", err)
		}
	})
}

func TestPageCustomCodeUpdate(t *testing.T) {
	var got CustomCodeRequest
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method %s", r.Method)
		}
		decodeJSONBody(t, r, &got)
		writeJSON(t, w, http.StatusOK, CustomCodeResponse{Scripts: got.Scripts, LastUpdated: "2025-03-01T00:00:00Z"})
	})

	resource := &PageCustomCode{}
	resp, err := resource.Update(context.Background(), infer.UpdateRequest[PageCustomCodeArgs, PageCustomCodeState]{
		ID: customCodeTestPageID + "/custom-code",
		Inputs: PageCustomCodeArgs{
			PageID:  customCodeTestPageID,
			Scripts: []PageCustomCodeScript{{ID: "b", Version: "2.0.0", Location: "footer"}},
		},
		State: PageCustomCodeState{
			PageCustomCodeArgs: PageCustomCodeArgs{
				PageID:  customCodeTestPageID,
				Scripts: []PageCustomCodeScript{{ID: "a", Version: "1.0.0", Location: "header"}},
			},
			CreatedOn: "2025-01-01T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(got.Scripts) != 1 || got.Scripts[0].ID != "b" {
		t.Errorf("Update() sent %+v, want only script b", got.Scripts)
	}
	if resp.Output.LastUpdated != "2025-03-01T00:00:00Z" || resp.Output.CreatedOn != "2025-01-01T00:00:00Z" {
		t.Errorf("Update() output = %+v", resp.Output)
	}
}

func TestPageCustomCodeDelete(t *testing.T) {
	calls := 0
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodDelete || r.URL.Path != "/v2/pages/"+customCodeTestPageID+"/custom_code" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound) // idempotent
	})
	resource := &PageCustomCode{}
	_, err := resource.Delete(context.Background(),
		infer.DeleteRequest[PageCustomCodeState]{ID: customCodeTestPageID + "/custom-code"})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = resource.Delete(context.Background(), infer.DeleteRequest[PageCustomCodeState]{ID: "nope/custom-code"})
	if err == nil {
		t.Error("Delete() should reject an invalid page id before calling the API")
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 API call, got %d", calls)
	}
}
