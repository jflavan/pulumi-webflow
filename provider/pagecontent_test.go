// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestValidateNodeID(t *testing.T) {
	if err := ValidateNodeID(""); err == nil || !strings.Contains(err.Error(), "/v2/pages/{page_id}/dom") {
		t.Errorf("empty nodeId must be rejected with a /v2 hint, got %v", err)
	}
	for _, id := range []string{"node-12345", "550e8400-e29b-41d4-a716-446655440000"} {
		if err := ValidateNodeID(id); err != nil {
			t.Errorf("ValidateNodeID(%q) = %v", id, err)
		}
	}
}

func TestValidateNodeText(t *testing.T) {
	for _, bad := range []string{"", "   ", "\n\t"} {
		if err := ValidateNodeText(bad); err == nil || !strings.Contains(err.Error(), "text is required") {
			t.Errorf("ValidateNodeText(%q) should be rejected, got %v", bad, err)
		}
	}
	if err := ValidateNodeText("<p>Hello</p>"); err != nil {
		t.Errorf("valid text rejected: %v", err)
	}
}

func TestValidatePageContentLocaleID(t *testing.T) {
	if err := ValidatePageContentLocaleID(""); err == nil || !strings.Contains(err.Error(), "localeId is required") ||
		!strings.Contains(err.Error(), "secondary locale") {
		t.Errorf("empty localeId must be rejected as required, got %v", err)
	}
	if err := ValidatePageContentLocaleID("en"); err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("malformed localeId must be rejected, got %v", err)
	}
	if err := ValidatePageContentLocaleID(testLocaleID); err != nil {
		t.Errorf("valid localeId rejected: %v", err)
	}
}

func TestPageContentResourceIDRoundTrip(t *testing.T) {
	id := GeneratePageContentResourceID(testPageID, testLocaleID)
	if id != testPageID+"/content/"+testLocaleID {
		t.Fatalf("id = %q", id)
	}
	pageID, localeID, err := ExtractIDsFromPageContentResourceID(id)
	if err != nil || pageID != testPageID || localeID != testLocaleID {
		t.Fatalf("extract: %q %q %v", pageID, localeID, err)
	}
	// The legacy form without a locale is still parsed; the locale comes back empty.
	pageID, localeID, err = ExtractIDsFromPageContentResourceID(testPageID + "/content")
	if err != nil || pageID != testPageID || localeID != "" {
		t.Fatalf("legacy extract: %q %q %v", pageID, localeID, err)
	}
	for _, bad := range []string{
		"", testPageID, testPageID + "/nodes", "/content", testPageID + "/content/", "a/content/b/c",
	} {
		if _, _, err := ExtractIDsFromPageContentResourceID(bad); err == nil {
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

// domNodesServer serves a DOM of total text nodes (ids t0..tN) interleaved with one image node,
// honouring limit/offset, and records the queries it saw.
func domNodesServer(t *testing.T, total int) (server *httptest.Server, queries *[]string) {
	t.Helper()
	var seen []string
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/pages/"+testPageID+"/dom" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		seen = append(seen, r.URL.RawQuery)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 || limit > 100 {
			t.Errorf("limit must be 1..100, got %d", limit)
		}
		// Node 0 is an image, the rest are text nodes.
		var nodes []DOMNode
		for i := offset; i < total+1 && i < offset+limit; i++ {
			if i == 0 {
				nodes = append(nodes, DOMNode{ID: "img0", Type: "image"})
				continue
			}
			nodes = append(nodes, DOMNode{
				ID: fmt.Sprintf("t%d", i), Type: "text",
				Text: &DOMText{HTML: fmt.Sprintf("<p>Text %d</p>", i), Text: fmt.Sprintf("Text %d", i)},
			})
		}
		resp := PageContentResponse{PageID: testPageID, Nodes: nodes}
		resp.Pagination.Limit, resp.Pagination.Offset, resp.Pagination.Total = limit, offset, total+1
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func TestListPageTextNodes_FollowsPagination(t *testing.T) {
	server, queries := domNodesServer(t, 249)
	client := useMockAPI(t, server)

	nodes, err := ListPageTextNodes(context.Background(), client, testPageID, testLocaleID)
	if err != nil {
		t.Fatalf("ListPageTextNodes: %v", err)
	}
	if len(nodes) != 249 || nodes[0].ID != "t1" || nodes[248].Text.HTML != "<p>Text 249</p>" {
		t.Fatalf("expected 249 text nodes in order, got %d (%+v)", len(nodes), nodes[0])
	}
	if len(*queries) != 3 {
		t.Errorf("expected 3 paginated requests (100+100+50), got %v", *queries)
	}
	for i, q := range *queries {
		if !strings.Contains(q, "limit=100") || !strings.Contains(q, "localeId="+testLocaleID) ||
			!strings.Contains(q, "offset="+strconv.Itoa(i*100)) {
			t.Errorf("request %d: unexpected query %q", i, q)
		}
	}
}

func TestListPageTextNodes_EmptyAndNotFound(t *testing.T) {
	server, queries := domNodesServer(t, 0)
	client := useMockAPI(t, server)
	nodes, err := ListPageTextNodes(context.Background(), client, testPageID, testLocaleID)
	if err != nil || nodes == nil || len(nodes) != 0 || len(*queries) != 1 {
		t.Fatalf("expected empty non-nil slice after one request, got %v %v (%v)", nodes, err, *queries)
	}

	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer errServer.Close()
	errClient := useMockAPI(t, errServer)
	if _, err := ListPageTextNodes(context.Background(), errClient, testPageID, testLocaleID); !IsNotFound(err) {
		t.Errorf("expected not-found error, got %v", err)
	}
}

// pageContentMock records GET and POST /v2/pages/{id}/dom requests.
type pageContentMock struct {
	server    *httptest.Server
	postCalls int
	getCalls  int
	query     string
	getQuery  string
	body      string
	errors    []string
	status    int
	domNodes  []DOMNode
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
			m.getQuery = r.URL.RawQuery
			w.WriteHeader(m.status)
			resp := PageContentResponse{PageID: testPageID, Nodes: m.domNodes}
			resp.Pagination.Limit, resp.Pagination.Total = pageDOMPageSize, len(m.domNodes)
			_ = json.NewEncoder(w).Encode(resp)
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
	nodes := []DOMNodeUpdate{{NodeID: "n1", Text: ptr("<p>Hello</p>")}, {NodeID: "n2", Text: ptr("<h1>Hi</h1>")}}

	if _, err := PostPageContent(context.Background(), client, testPageID, testLocaleID, nodes); err != nil {
		t.Fatalf("PostPageContent: %v", err)
	}
	if m.query != "localeId="+testLocaleID {
		t.Errorf("localeId must always be sent, got query %q", m.query)
	}
	// encoding/json escapes angle brackets on the wire, so compare the decoded request.
	var sent PageContentRequest
	if err := json.Unmarshal([]byte(m.body), &sent); err != nil {
		t.Fatalf("decode body %s: %v", m.body, err)
	}
	if len(sent.Nodes) != 2 || sent.Nodes[0].NodeID != "n1" || sent.Nodes[0].Text == nil ||
		*sent.Nodes[0].Text != "<p>Hello</p>" || sent.Nodes[1].NodeID != "n2" || sent.Nodes[1].Text == nil ||
		*sent.Nodes[1].Text != "<h1>Hi</h1>" {
		t.Errorf("body = %s", m.body)
	}

	// localeId is required: an empty value never reaches the API.
	calls := m.postCalls
	if _, err := PostPageContent(context.Background(), client, testPageID, "", nodes); err == nil ||
		!strings.Contains(err.Error(), "localeId is required") {
		t.Errorf("expected localeId required error, got %v", err)
	}
	if m.postCalls != calls {
		t.Error("a missing localeId must not reach the API")
	}

	// The documented 1000-node cap is enforced before the request is sent.
	tooMany := make([]DOMNodeUpdate, maxPageContentNodes+1)
	for i := range tooMany {
		tooMany[i] = DOMNodeUpdate{NodeID: fmt.Sprintf("n%d", i), Text: ptr("x")}
	}
	if _, err := PostPageContent(context.Background(), client, testPageID, testLocaleID, tooMany); err == nil ||
		!strings.Contains(err.Error(), "at most 1000 nodes") {
		t.Errorf("expected node cap error, got %v", err)
	}
	if m.postCalls != calls {
		t.Error("an over-sized request must not reach the API")
	}

	m.errors = []string{"Node n1 not found", "Node n2 is not a text node"}
	_, err := PostPageContent(context.Background(), client, testPageID, testLocaleID, nodes)
	if err == nil || !strings.Contains(err.Error(), "rejected 2 node update(s)") ||
		!strings.Contains(err.Error(), "Node n2 is not a text node") {
		t.Errorf("expected errors surfaced, got %v", err)
	}

	m.errors = nil
	m.status = http.StatusBadRequest
	if _, err := PostPageContent(context.Background(), client, testPageID, testLocaleID, nodes); err == nil ||
		!strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected bad request, got %v", err)
	}
}

func TestPageContentCreate(t *testing.T) {
	m := newPageContentMock(t)
	args := PageContentArgs{
		PageID:   testPageID,
		LocaleID: testLocaleID,
		Nodes:    []NodeContentUpdate{{NodeID: "n1", Text: "Hi"}, {NodeID: "n2", Text: "<p>Two</p>"}},
	}

	resp, err := (&PageContent{}).Create(context.Background(), infer.CreateRequest[PageContentArgs]{Inputs: args})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.postCalls != 1 || m.query != "localeId="+testLocaleID ||
		!strings.Contains(m.body, `{"nodeId":"n1","text":"Hi"}`) {
		t.Errorf("calls=%d query=%q body=%s", m.postCalls, m.query, m.body)
	}
	if resp.ID != testPageID+"/content/"+testLocaleID || len(resp.Output.Nodes) != 2 ||
		resp.Output.LocaleID != testLocaleID {
		t.Errorf("unexpected response %+v", resp)
	}
}

func TestPageContentCreate_ErrorsFromAPI(t *testing.T) {
	m := newPageContentMock(t)
	m.errors = []string{"Node n1 not found"}
	_, err := (&PageContent{}).Create(context.Background(), infer.CreateRequest[PageContentArgs]{
		Inputs: PageContentArgs{
			PageID: testPageID, LocaleID: testLocaleID, Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "Hi"}},
		},
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
	if resp.ID != "" {
		t.Errorf("preview with unknown pageId/localeId must not fabricate an ID, got %q", resp.ID)
	}
	resp, err = (&PageContent{}).Create(context.Background(), infer.CreateRequest[PageContentArgs]{
		Inputs: PageContentArgs{PageID: testPageID, LocaleID: testLocaleID}, DryRun: true,
	})
	if err != nil || resp.ID != testPageID+"/content/"+testLocaleID {
		t.Errorf("preview with known IDs should report the deterministic ID, got %q %v", resp.ID, err)
	}

	one := []NodeContentUpdate{{NodeID: "n1", Text: "x"}}
	tooMany := make([]NodeContentUpdate, maxPageContentNodes+1)
	for i := range tooMany {
		tooMany[i] = NodeContentUpdate{NodeID: fmt.Sprintf("n%d", i), Text: "x"}
	}
	tests := []struct {
		name string
		args PageContentArgs
		want string
	}{
		{"invalid pageId", PageContentArgs{PageID: "bad", LocaleID: testLocaleID, Nodes: one}, "pageId has invalid format"},
		{"missing localeId", PageContentArgs{PageID: testPageID, Nodes: one}, "localeId is required"},
		{"invalid localeId", PageContentArgs{PageID: testPageID, LocaleID: "en", Nodes: one}, "localeId has invalid format"},
		{"no nodes", PageContentArgs{PageID: testPageID, LocaleID: testLocaleID}, "at least one node"},
		{"too many nodes", PageContentArgs{PageID: testPageID, LocaleID: testLocaleID, Nodes: tooMany}, "at most 1000"},
		{
			"empty nodeId",
			PageContentArgs{PageID: testPageID, LocaleID: testLocaleID, Nodes: []NodeContentUpdate{{NodeID: "", Text: "x"}}},
			"nodeId is required",
		},
		{
			"empty text",
			PageContentArgs{PageID: testPageID, LocaleID: testLocaleID, Nodes: []NodeContentUpdate{{NodeID: "n1", Text: ""}}},
			"text is required",
		},
		{
			"duplicate nodeId",
			PageContentArgs{
				PageID: testPageID, LocaleID: testLocaleID,
				Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "a"}, {NodeID: "n1", Text: "b"}},
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

// pageContentNode builds one entry of the 'nodes' input for Check tests.
func pageContentNode(id, text property.Value) property.Value {
	return property.New(property.NewMap(map[string]property.Value{"nodeId": id, "text": text}))
}

// checkFailureProperties returns the Property of every failure, for assertions.
func checkFailureProperties(failures []p.CheckFailure) []string {
	props := make([]string, 0, len(failures))
	for _, f := range failures {
		props = append(props, f.Property)
	}
	return props
}

func TestPageContentCheck_KnownValues(t *testing.T) {
	inputs := property.NewMap(map[string]property.Value{
		"pageId":   property.New("bad"),
		"localeId": property.New(""),
		"nodes": property.New([]property.Value{
			pageContentNode(property.New("n1"), property.New("a")),
			pageContentNode(property.New("n2"), property.New("")),
			pageContentNode(property.New("n1"), property.New("c")),
			pageContentNode(property.New(""), property.New("d")),
		}),
	})

	resp, err := (&PageContent{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	got := strings.Join(checkFailureProperties(resp.Failures), ",")
	for _, want := range []string{"pageId", "localeId", "nodes[1].text", "nodes", "nodes[3].nodeId"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected a failure on %q, got %v", want, resp.Failures)
		}
	}
	if len(resp.Failures) != 5 {
		t.Errorf("expected 5 failures, got %d: %+v", len(resp.Failures), resp.Failures)
	}
	for _, f := range resp.Failures {
		if f.Property == "localeId" && !strings.Contains(f.Reason, "localeId is required") {
			t.Errorf("localeId failure should say it is required: %s", f.Reason)
		}
	}
	if resp.Inputs.PageID != "bad" || len(resp.Inputs.Nodes) != 4 {
		t.Errorf("inputs not decoded: %+v", resp.Inputs)
	}

	ok := property.NewMap(map[string]property.Value{
		"pageId":   property.New(testPageID),
		"localeId": property.New(testLocaleID),
		"nodes": property.New([]property.Value{
			pageContentNode(property.New("n1"), property.New("a")),
			pageContentNode(property.New("n2"), property.New("<p>b</p>")),
		}),
	})
	resp, err = (&PageContent{}).Check(context.Background(), infer.CheckRequest{NewInputs: ok})
	if err != nil || len(resp.Failures) != 0 {
		t.Errorf("valid inputs must pass: %+v %v", resp.Failures, err)
	}
}

func TestPageContentCheck_NodeCount(t *testing.T) {
	nodes := make([]property.Value, maxPageContentNodes+1)
	for i := range nodes {
		nodes[i] = pageContentNode(property.New(fmt.Sprintf("n%d", i)), property.New("x"))
	}
	for name, list := range map[string][]property.Value{"too many": nodes, "none": {}} {
		t.Run(name, func(t *testing.T) {
			inputs := property.NewMap(map[string]property.Value{
				"pageId":   property.New(testPageID),
				"localeId": property.New(testLocaleID),
				"nodes":    property.New(list),
			})
			resp, err := (&PageContent{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if len(resp.Failures) != 1 || resp.Failures[0].Property != "nodes" {
				t.Errorf("expected one nodes failure, got %+v", resp.Failures)
			}
		})
	}
}

func TestPageContentCheck_UnknownValuesAreSkipped(t *testing.T) {
	inputs := property.NewMap(map[string]property.Value{
		"pageId":   property.New(property.Computed),
		"localeId": property.New(property.Computed),
		"nodes": property.New([]property.Value{
			pageContentNode(property.New(property.Computed), property.New(property.Computed)),
			pageContentNode(property.New("n2"), property.New(property.Computed)),
			property.New(property.Computed),
		}),
	})
	resp, err := (&PageContent{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(resp.Failures) != 0 {
		t.Errorf("unknown values must not fail Check, got %+v", resp.Failures)
	}

	// A wholly unknown node list is skipped too.
	allUnknown := property.NewMap(map[string]property.Value{
		"pageId":   property.New(testPageID),
		"localeId": property.New(testLocaleID),
		"nodes":    property.New(property.Computed),
	})
	resp, err = (&PageContent{}).Check(context.Background(), infer.CheckRequest{NewInputs: allUnknown})
	if err != nil || len(resp.Failures) != 0 {
		t.Errorf("unknown node list must not fail Check, got %+v %v", resp.Failures, err)
	}
}

func TestPageContentRead_RefreshesManagedNodes(t *testing.T) {
	m := newPageContentMock(t)
	m.domNodes = []DOMNode{
		{ID: "n1", Type: "text", Text: &DOMText{HTML: "<p>Changed</p>", Text: "Changed"}},
		{ID: "img", Type: "image"},
		{ID: "n3", Type: "text", Text: &DOMText{HTML: "<p>Unmanaged</p>", Text: "Unmanaged"}},
	}
	id := GeneratePageContentResourceID(testPageID, testLocaleID)
	state := PageContentState{PageContentArgs: PageContentArgs{
		PageID: testPageID, LocaleID: testLocaleID,
		Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "<p>Hi</p>"}, {NodeID: "gone", Text: "<p>Old</p>"}},
	}}

	resp, err := (&PageContent{}).Read(
		context.Background(),
		infer.ReadRequest[PageContentArgs, PageContentState]{ID: id, State: state},
	)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if m.getCalls != 1 || resp.ID != id || resp.Inputs.LocaleID != testLocaleID {
		t.Errorf("unexpected response %+v (get calls %d)", resp, m.getCalls)
	}
	if !strings.Contains(m.getQuery, "localeId="+testLocaleID) || !strings.Contains(m.getQuery, "limit=100") ||
		!strings.Contains(m.getQuery, "offset=0") {
		t.Errorf("DOM read must be paginated and locale-scoped, got query %q", m.getQuery)
	}
	nodes := resp.Inputs.Nodes
	if len(nodes) != 2 || nodes[0].NodeID != "n1" || nodes[0].Text != "<p>Changed</p>" {
		t.Errorf("managed node text must be refreshed from text.html: %+v", nodes)
	}
	if nodes[1].NodeID != "gone" || nodes[1].Text != "<p>Old</p>" {
		t.Errorf("a managed node missing from the DOM keeps its state value: %+v", nodes)
	}
	if len(resp.State.Nodes) != 2 {
		t.Errorf("state must mirror inputs: %+v", resp.State)
	}
}

func TestPageContentRead_ImportCapturesTextNodes(t *testing.T) {
	m := newPageContentMock(t)
	m.domNodes = []DOMNode{
		{ID: "n1", Type: "text", Text: &DOMText{HTML: "<h1>Title</h1>", Text: "Title"}},
		{ID: "img", Type: "image"},
		{ID: "n2", Type: "text", Text: &DOMText{HTML: "<p>Body</p>", Text: "Body"}},
		{ID: "n3", Type: "text"},
	}
	id := GeneratePageContentResourceID(testPageID, testLocaleID)

	resp, err := (&PageContent{}).Read(
		context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: id},
	)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	nodes := resp.Inputs.Nodes
	if len(nodes) != 3 || nodes[0].NodeID != "n1" || nodes[0].Text != "<h1>Title</h1>" ||
		nodes[1].NodeID != "n2" || nodes[1].Text != "<p>Body</p>" || nodes[2].NodeID != "n3" || nodes[2].Text != "" {
		t.Errorf("import should capture every text node with its html: %+v", nodes)
	}
	if resp.Inputs.PageID != testPageID || resp.Inputs.LocaleID != testLocaleID {
		t.Errorf("IDs not taken from the resource ID: %+v", resp.Inputs)
	}
}

func TestPageContentRead_LegacyIDAndErrors(t *testing.T) {
	m := newPageContentMock(t)
	m.domNodes = []DOMNode{{ID: "n1", Type: "text", Text: &DOMText{HTML: "<p>x</p>"}}}
	legacy := testPageID + "/content"
	state := PageContentState{PageContentArgs: PageContentArgs{
		PageID: testPageID, LocaleID: testLocaleID, Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "old"}},
	}}

	// A legacy ID takes the locale from state.
	resp, err := (&PageContent{}).Read(
		context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: legacy, State: state},
	)
	if err != nil || resp.ID != legacy || resp.Inputs.LocaleID != testLocaleID || resp.Inputs.Nodes[0].Text != "<p>x</p>" {
		t.Errorf("legacy ID read: %+v %v", resp, err)
	}
	if !strings.Contains(m.getQuery, "localeId="+testLocaleID) {
		t.Errorf("legacy read must still scope the DOM read to the locale, got %q", m.getQuery)
	}

	// A legacy ID without a locale in state cannot be read (import needs the full ID).
	calls := m.getCalls
	_, err = (&PageContent{}).Read(context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: legacy})
	if err == nil || !strings.Contains(err.Error(), "{pageId}/content/{localeId}") {
		t.Errorf("expected import guidance, got %v", err)
	}
	if _, err := (&PageContent{}).Read(
		context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: "bad/content/" + testLocaleID},
	); err == nil {
		t.Error("invalid page ID must be rejected")
	}
	if _, err := (&PageContent{}).Read(
		context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: testPageID + "/content/en"},
	); err == nil {
		t.Error("invalid locale ID must be rejected")
	}
	if m.getCalls != calls {
		t.Error("invalid IDs must not reach the API")
	}

	id := GeneratePageContentResourceID(testPageID, testLocaleID)
	m.status = http.StatusNotFound
	resp, err = (&PageContent{}).Read(
		context.Background(), infer.ReadRequest[PageContentArgs, PageContentState]{ID: id, State: state},
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
}

func TestPageContentUpdate(t *testing.T) {
	m := newPageContentMock(t)
	args := PageContentArgs{
		PageID: testPageID, LocaleID: testLocaleID, Nodes: []NodeContentUpdate{{NodeID: "n1", Text: "New"}},
	}

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
	if m.postCalls != 1 || m.query != "localeId="+testLocaleID || m.body != `{"nodes":[{"nodeId":"n1","text":"New"}]}` ||
		resp.Output.Nodes[0].Text != "New" {
		t.Errorf("calls=%d query=%q body=%s", m.postCalls, m.query, m.body)
	}
	deleteID := GeneratePageContentResourceID(testPageID, testLocaleID)
	if _, err := (&PageContent{}).Delete(
		context.Background(), infer.DeleteRequest[PageContentState]{ID: deleteID},
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
		{"localeId", p.UpdateReplace, func(a *PageContentArgs) { a.LocaleID = "5f0c8c9e1c9d440000e8d8c9" }},
		{"nodes", p.Update, func(a *PageContentArgs) {
			a.Nodes = []NodeContentUpdate{{NodeID: "n1", Text: "changed"}, {NodeID: "n2", Text: "b"}}
		}},
		{"nodes", p.Update, func(a *PageContentArgs) { a.Nodes = []NodeContentUpdate{{NodeID: "n1", Text: "a"}} }},
		{
			"nodes",
			p.Update,
			func(a *PageContentArgs) { a.Nodes = append(a.Nodes, NodeContentUpdate{NodeID: "n3", Text: "c"}) },
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
