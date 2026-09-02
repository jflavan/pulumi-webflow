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

func TestValidateNodeID(t *testing.T) {
	if err := ValidateNodeID(""); err == nil {
		t.Error("empty nodeId must be rejected")
	}
	for _, id := range []string{"node-12345", "550e8400-e29b-41d4-a716-446655440000"} {
		if err := ValidateNodeID(id); err != nil {
			t.Errorf("ValidateNodeID(%q) = %v", id, err)
		}
	}
}

func TestPageContentResourceIDRoundTrip(t *testing.T) {
	id := GeneratePageContentResourceID(testPageID)
	if id != testPageID+"/content" {
		t.Fatalf("id = %q", id)
	}
	pageID, err := ExtractPageIDFromPageContentResourceID(id)
	if err != nil || pageID != testPageID {
		t.Fatalf("extract: %q %v", pageID, err)
	}
	for _, bad := range []string{"", testPageID, testPageID + "/nodes", "/content"} {
		if _, err := ExtractPageIDFromPageContentResourceID(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestDOMTextUnmarshal(t *testing.T) {
	var node DOMNode
	if err := json.Unmarshal(
		[]byte(`{"id":"n1","type":"text","text":{"html":"<h1>Hi</h1>","text":"Hi"}}`), &node,
	); err != nil {
		t.Fatal(err)
	}
	if node.ID != "n1" || node.Text == nil || node.Text.HTML != "<h1>Hi</h1>" || node.Text.Text != "Hi" {
		t.Errorf("object form: %+v", node)
	}
	if err := json.Unmarshal([]byte(`{"id":"n2","text":"plain"}`), &node); err != nil {
		t.Fatal(err)
	}
	if node.Text == nil || node.Text.Text != "plain" || node.Text.HTML != "plain" {
		t.Errorf("string form: %+v", node.Text)
	}
	if err := json.Unmarshal([]byte(`{"id":"n3","text":null}`), &node); err != nil {
		t.Fatal(err)
	}
}

func TestGetPageContent(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/pages/"+testPageID+"/dom" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"pageId":"` + testPageID + `","nodes":[` +
			`{"id":"a1","type":"text","text":{"html":"<p>x</p>","text":"x"}},` +
			`{"id":"a2","type":"image","image":{"alt":"","assetId":"z"}}],` +
			`"pagination":{"limit":100,"offset":0,"total":2},"lastUpdated":null}`))
	}))
	defer server.Close()
	client := useMockAPI(t, server)

	resp, err := GetPageContent(context.Background(), client, testPageID, "")
	if err != nil {
		t.Fatalf("GetPageContent: %v", err)
	}
	if gotQuery != "" || resp.PageID != testPageID || len(resp.Nodes) != 2 || resp.Nodes[0].Text.Text != "x" ||
		resp.Pagination.Total != 2 {
		t.Errorf("query=%q resp=%+v", gotQuery, resp)
	}
	if _, err := GetPageContent(context.Background(), client, testPageID, testLocaleID); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "localeId="+testLocaleID {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestGetPageContent_Errors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusInternalServerError} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"error"}`))
		}))
		client := useMockAPI(t, server)
		_, err := GetPageContent(context.Background(), client, testPageID, "")
		server.Close()
		if err == nil {
			t.Errorf("status %d: expected error", status)
			continue
		}
		if (status == http.StatusNotFound) != IsNotFound(err) {
			t.Errorf("status %d: IsNotFound = %v", status, IsNotFound(err))
		}
	}
}

// pageContentMock records POST /v2/pages/{id}/dom requests.
type pageContentMock struct {
	server    *httptest.Server
	postCalls int
	getCalls  int
	query     string
	body      string
	errors    []string
	status    int
}

func newPageContentMock(t *testing.T) *pageContentMock {
	t.Helper()
	m := &pageContentMock{errors: []string{}, status: http.StatusOK}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/pages/"+testPageID+"/dom" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPost:
			m.postCalls++
			m.query = r.URL.RawQuery
			b, _ := io.ReadAll(r.Body)
			m.body = string(b)
			w.WriteHeader(m.status)
			_ = json.NewEncoder(w).Encode(PageContentUpdateResponse{Errors: m.errors})
		case http.MethodGet:
			m.getCalls++
			w.WriteHeader(m.status)
			_, _ = w.Write([]byte(`{"pageId":"` + testPageID + `","nodes":[]}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(m.server.Close)
	useMockAPI(t, m.server)
	t.Setenv("WEBFLOW_API_TOKEN", "test-token-12345678901234567890")
	return m
}

func TestPostPageContent(t *testing.T) {
	m := newPageContentMock(t)
	client := useMockAPI(t, m.server)
	empty := ""
	nodes := []DOMNodeUpdate{{NodeID: "n1", Text: ptr("<p>Hello</p>")}, {NodeID: "n2", Text: &empty}}

	if _, err := PostPageContent(context.Background(), client, testPageID, testLocaleID, nodes); err != nil {
		t.Fatalf("PostPageContent: %v", err)
	}
	if m.query != "localeId="+testLocaleID {
		t.Errorf("query = %q", m.query)
	}
	// encoding/json escapes angle brackets on the wire, so compare the decoded request.
	var sent PageContentRequest
	if err := json.Unmarshal([]byte(m.body), &sent); err != nil {
		t.Fatalf("decode body %s: %v", m.body, err)
	}
	if len(sent.Nodes) != 2 || sent.Nodes[0].NodeID != "n1" || sent.Nodes[0].Text == nil ||
		*sent.Nodes[0].Text != "<p>Hello</p>" ||
		sent.Nodes[1].NodeID != "n2" ||
		sent.Nodes[1].Text == nil ||
		*sent.Nodes[1].Text != "" {
		t.Errorf("body = %s", m.body)
	}
	if !strings.Contains(m.body, `"text":""`) {
		t.Errorf("empty text must be sent, not omitted: %s", m.body)
	}

	if _, err := PostPageContent(context.Background(), client, testPageID, "", nodes); err != nil {
		t.Fatal(err)
	}
	if m.query != "" {
		t.Errorf("localeId must be omitted when empty, got %q", m.query)
	}

	m.errors = []string{"Node n1 not found", "Node n2 is not a text node"}
	_, err := PostPageContent(context.Background(), client, testPageID, "", nodes)
	if err == nil || !strings.Contains(err.Error(), "rejected 2 node update(s)") ||
		!strings.Contains(err.Error(), "Node n2 is not a text node") {
		t.Errorf("expected errors surfaced, got %v", err)
	}

	m.errors = nil
	m.status = http.StatusBadRequest
	if _, err := PostPageContent(context.Background(), client, testPageID, "", nodes); err == nil ||
		!strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected bad request, got %v", err)
	}
}

func TestPageContentCreate(t *testing.T) {
	m := newPageContentMock(t)
	args := PageContentArgs{
		PageID:   testPageID,
		LocaleID: testLocaleID,
		Nodes:    []NodeContentUpdate{{NodeID: "n1", Text: "Hi"}, {NodeID: "n2", Text: ""}},
	}

	resp, err := (&PageContent{}).Create(context.Background(), infer.CreateRequest[PageContentArgs]{Inputs: args})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.postCalls != 1 || m.query != "localeId="+testLocaleID || !strings.Contains(m.body, `{"nodeId":"n2","text":""}`) {
		t.Errorf("calls=%d query=%q body=%s", m.postCalls, m.query, m.body)
	}
	if resp.ID != testPageID+"/content" || len(resp.Output.Nodes) != 2 || resp.Output.LocaleID != testLocaleID {
		t.Errorf("unexpected response %+v", resp)
	}
}

func TestPageContentCreate_ErrorsFromAPI(t *testing.T) {
	m := newPageContentMock(t)
	m.errors = []string{"Node n1 not found"}
	_, err := (&PageContent{}).Create(context.Background(), infer.CreateRequest[PageContentArgs]{
		Inputs: PageContentArgs{PageID: testPageID, Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "Hi"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "Node n1 not found") {
		t.Errorf("expected API errors to fail the operation, got %v", err)
	}
}

func TestPageContentCreate_DryRunThenValidation(t *testing.T) {
	m := newPageContentMock(t)

	resp, err := (&PageContent{}).Create(context.Background(), infer.CreateRequest[PageContentArgs]{
		Inputs: PageContentArgs{PageID: "", Nodes: nil}, DryRun: true,
	})
	if err != nil || m.postCalls != 0 {
		t.Fatalf("dry run: %v calls=%d", err, m.postCalls)
	}
	_ = resp

	tests := []struct {
		name string
		args PageContentArgs
		want string
	}{
		{
			"invalid pageId",
			PageContentArgs{PageID: "bad", Nodes: []NodeContentUpdate{{NodeID: "n1"}}},
			"pageId has invalid format",
		},
		{
			"invalid localeId",
			PageContentArgs{PageID: testPageID, LocaleID: "en", Nodes: []NodeContentUpdate{{NodeID: "n1"}}},
			"localeId has invalid format",
		},
		{"no nodes", PageContentArgs{PageID: testPageID}, "at least one node"},
		{"empty nodeId", PageContentArgs{PageID: testPageID, Nodes: []NodeContentUpdate{{NodeID: ""}}}, "nodeId is required"},
		{
			"duplicate nodeId",
			PageContentArgs{
				PageID: testPageID,
				Nodes:  []NodeContentUpdate{{NodeID: "n1", Text: "a"}, {NodeID: "n1", Text: "b"}},
			},
			"appears more than once",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&PageContent{}).Create(context.Background(), infer.CreateRequest[PageContentArgs]{Inputs: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected %q, got %v", tt.want, err)
			}
		})
	}
	if m.postCalls != 0 {
		t.Error("validation failures must not reach the API")
	}
}

func TestPageContentCheck_RejectsDuplicateNodeIDs(t *testing.T) {
	node := func(id, text string) property.Value {
		return property.New(
			property.NewMap(map[string]property.Value{"nodeId": property.New(id), "text": property.New(text)}),
		)
	}
	inputs := property.NewMap(map[string]property.Value{
		"pageId": property.New(testPageID),
		"nodes":  property.New([]property.Value{node("n1", "a"), node("n2", "b"), node("n1", "c")}),
	})

	resp, err := (&PageContent{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(resp.Failures) != 1 || resp.Failures[0].Property != "nodes" ||
		!strings.Contains(resp.Failures[0].Reason, "n1") {
		t.Errorf("expected one nodes failure, got %+v", resp.Failures)
	}
	if resp.Inputs.PageID != testPageID || len(resp.Inputs.Nodes) != 3 {
		t.Errorf("inputs not decoded: %+v", resp.Inputs)
	}

	ok := property.NewMap(map[string]property.Value{
		"pageId": property.New(testPageID),
		"nodes":  property.New([]property.Value{node("n1", "a"), node("n2", "")}),
	})
	resp, err = (&PageContent{}).Check(context.Background(), infer.CheckRequest{NewInputs: ok})
	if err != nil || len(resp.Failures) != 0 {
		t.Errorf("unique nodes must pass: %+v %v", resp.Failures, err)
	}
}

func TestPageContentRead(t *testing.T) {
	m := newPageContentMock(t)
	id := GeneratePageContentResourceID(testPageID)
	state := PageContentState{PageContentArgs: PageContentArgs{
		PageID: testPageID, LocaleID: testLocaleID, Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "Hi"}},
	}}

	resp, err := (&PageContent{}).Read(
		context.Background(),
		infer.ReadRequest[PageContentArgs, PageContentState]{ID: id, State: state},
	)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if m.getCalls != 1 || resp.ID != id || len(resp.Inputs.Nodes) != 1 || resp.Inputs.LocaleID != testLocaleID {
		t.Errorf("unexpected response %+v (get calls %d)", resp, m.getCalls)
	}

	m.status = http.StatusNotFound
	resp, err = (&PageContent{}).Read(
		context.Background(),
		infer.ReadRequest[PageContentArgs, PageContentState]{ID: id, State: state},
	)
	if err != nil || resp.ID != "" {
		t.Errorf("404 should clear the resource: id=%q err=%v", resp.ID, err)
	}

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		m.status = status
		if _, err := (&PageContent{}).Read(
			context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: id, State: state},
		); err == nil {
			t.Errorf("status %d must propagate", status)
		}
	}

	calls := m.getCalls
	if _, err := (&PageContent{}).Read(
		context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: "bad/content"},
	); err == nil {
		t.Error("invalid page ID must be rejected")
	}
	if m.getCalls != calls {
		t.Error("invalid IDs must not reach the API")
	}
}

func TestPageContentUpdate(t *testing.T) {
	m := newPageContentMock(t)
	args := PageContentArgs{PageID: testPageID, Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "New"}}}

	if _, err := (&PageContent{}).Update(
		context.Background(), infer.UpdateRequest[PageContentArgs, PageContentState]{Inputs: args, DryRun: true},
	); err != nil ||
		m.postCalls != 0 {
		t.Fatalf("dry run: %v calls=%d", err, m.postCalls)
	}
	resp, err := (&PageContent{}).Update(
		context.Background(),
		infer.UpdateRequest[PageContentArgs, PageContentState]{Inputs: args},
	)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if m.postCalls != 1 || m.query != "" || m.body != `{"nodes":[{"nodeId":"n1","text":"New"}]}` ||
		resp.Output.Nodes[0].Text != "New" {
		t.Errorf("calls=%d query=%q body=%s", m.postCalls, m.query, m.body)
	}
	if _, err := (&PageContent{}).Delete(
		context.Background(), infer.DeleteRequest[PageContentState]{ID: testPageID + "/content"},
	); err != nil ||
		m.postCalls != 1 {
		t.Errorf("Delete must be a no-op: %v calls=%d", err, m.postCalls)
	}
}

func TestPageContentDiff(t *testing.T) {
	base := PageContentArgs{
		PageID:   testPageID,
		LocaleID: testLocaleID,
		Nodes:    []NodeContentUpdate{{NodeID: "n1", Text: "a"}, {NodeID: "n2", Text: "b"}},
	}
	state := PageContentState{PageContentArgs: base}

	resp, err := (&PageContent{}).Diff(
		context.Background(),
		infer.DiffRequest[PageContentArgs, PageContentState]{Inputs: base, State: state},
	)
	if err != nil || resp.HasChanges {
		t.Fatalf("expected no changes: %+v %v", resp, err)
	}

	reordered := base
	reordered.Nodes = []NodeContentUpdate{{NodeID: "n2", Text: "b"}, {NodeID: "n1", Text: "a"}}
	if resp, _ := (&PageContent{}).Diff(
		context.Background(), infer.DiffRequest[PageContentArgs, PageContentState]{Inputs: reordered, State: state},
	); resp.HasChanges {
		t.Error("node order must not matter")
	}

	tests := []struct {
		field  string
		kind   p.DiffKind
		modify func(a *PageContentArgs)
	}{
		{"pageId", p.UpdateReplace, func(a *PageContentArgs) { a.PageID = "5f0c8c9e1c9d440000e8d8c9" }},
		{"localeId", p.UpdateReplace, func(a *PageContentArgs) { a.LocaleID = "" }},
		{"nodes", p.Update, func(a *PageContentArgs) {
			a.Nodes = []NodeContentUpdate{{NodeID: "n1", Text: "changed"}, {NodeID: "n2", Text: "b"}}
		}},
		{"nodes", p.Update, func(a *PageContentArgs) { a.Nodes = []NodeContentUpdate{{NodeID: "n1", Text: "a"}} }},
		{
			"nodes",
			p.Update,
			func(a *PageContentArgs) { a.Nodes = append(a.Nodes, NodeContentUpdate{NodeID: "n3", Text: ""}) },
		},
	}
	for _, tt := range tests {
		in := base
		in.Nodes = append([]NodeContentUpdate(nil), base.Nodes...)
		tt.modify(&in)
		resp, err := (&PageContent{}).Diff(
			context.Background(),
			infer.DiffRequest[PageContentArgs, PageContentState]{Inputs: in, State: state},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !resp.HasChanges || resp.DetailedDiff[tt.field].Kind != tt.kind || resp.DeleteBeforeReplace {
			t.Errorf("expected %s %s, got %+v", tt.field, tt.kind, resp)
		}
	}
}
