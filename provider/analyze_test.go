// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/blang/semver"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

const (
	testAnalyzeSiteID = "580e63e98c9a982ac9b8b741"
	testAnalyzeStart  = "2026-04-01T00:00:00Z"
	testAnalyzeEnd    = "2026-04-08T00:00:00Z"
	testAnalyzePath   = "/beta/sites/" + testAnalyzeSiteID + "/analyze/reports/"
	testAnalyzeAuth   = "test-token-abc123def456"
	testAnalyzePageID = "65f1b2c4a8d3e5f7a9c1b2d3"
	testAnalyzeWindow = `"window":{"startTime":"2026-04-01T00:00:00Z","endTime":"2026-04-08T00:00:00Z"}`
)

// analyzeRecorder records the single request an Analyze test makes.
type analyzeRecorder struct {
	method string
	path   string
	query  url.Values
}

// newAnalyzeServer starts a mock Analyze server replying with a fixed status and body.
func newAnalyzeServer(t *testing.T, status int, body string) *analyzeRecorder {
	t.Helper()
	t.Setenv("WEBFLOW_API_TOKEN", testAnalyzeAuth)
	rec := &analyzeRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method, rec.path, rec.query = r.Method, r.URL.Path, r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	useMockAPI(t, server)
	return rec
}

// notFoundAnalyzeServer starts a mock Analyze server that answers 404.
func notFoundAnalyzeServer(t *testing.T) *analyzeRecorder {
	t.Helper()
	return newAnalyzeServer(t, http.StatusNotFound, `{"message":"Requested resource not found"}`)
}

func assertAnalyzeRequest(t *testing.T, rec *analyzeRecorder, report string, want map[string]string) {
	t.Helper()
	if rec.method != http.MethodGet {
		t.Errorf("expected GET, got %s", rec.method)
	}
	if rec.path != testAnalyzePath+report {
		t.Errorf("expected path %s, got %s", testAnalyzePath+report, rec.path)
	}
	for k, v := range want {
		if got := rec.query.Get(k); got != v {
			t.Errorf("query %s = %q, want %q (full query: %v)", k, got, v, rec.query)
		}
	}
	if len(rec.query) != len(want) {
		t.Errorf("unexpected extra query parameters: got %v, want keys %v", rec.query, want)
	}
}

// windowParams are the query parameters every report sends for the standard test window.
func windowParams(extra map[string]string) map[string]string {
	params := map[string]string{"startTime": testAnalyzeStart, "endTime": testAnalyzeEnd}
	for k, v := range extra {
		params[k] = v
	}
	return params
}

func TestValidateAnalyzeWindow(t *testing.T) {
	cases := []struct {
		name       string
		start, end string
		wantErr    string
	}{
		{"ok", testAnalyzeStart, testAnalyzeEnd, ""},
		{"missing start", "", testAnalyzeEnd, "startTime is required"},
		{"offset", "2026-04-01T00:00:00+00:00", testAnalyzeEnd, "ending in 'Z'"},
		{"garbage", "yesterday Z", testAnalyzeEnd, "not a valid"},
		{"too early", "2025-01-01T00:00:00Z", "2025-01-02T00:00:00Z", "earliest supported"},
		{"end before start", testAnalyzeEnd, testAnalyzeStart, "must be after"},
		{"too long", "2026-01-01T00:00:00Z", "2026-05-01T00:00:00Z", "100-day"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAnalyzeWindow(tc.start, tc.end)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateAnalyticsCommonFilters(t *testing.T) {
	ok := AnalyticsCommonFilters{DeviceType: "mobile", Country: "US", Filters: map[string]AnalyticsDimensionFilter{
		"os": {In: []string{"iOS", "Android"}},
	}}
	if err := ValidateAnalyticsCommonFilters(ok); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := map[string]AnalyticsCommonFilters{
		"device":     {DeviceType: "phone"},
		"country":    {Country: "USA"},
		"dimension":  {Filters: map[string]AnalyticsDimensionFilter{"colour": {Eq: "red"}}},
		"empty":      {Filters: map[string]AnalyticsDimensionFilter{"os": {}}},
		"duplicated": {Country: "US", Filters: map[string]AnalyticsDimensionFilter{"country": {Eq: "US"}}},
	}
	for name, f := range bad {
		t.Run(name, func(t *testing.T) {
			if err := ValidateAnalyticsCommonFilters(f); err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
}

func TestAnalyzeQuery_FilterEncoding(t *testing.T) {
	q := newAnalyzeQuery()
	q.addCommonFilters(AnalyticsCommonFilters{
		PagePath: "/towels",
		Filters: map[string]AnalyticsDimensionFilter{
			"country": {In: []string{"US", "CA"}, Ne: "GB"},
			"browser": {Eq: "Chrome", Nin: []string{"IE"}},
		},
	})
	got := q.encode()
	want := "filter%5Bbrowser%5D%5Beq%5D=Chrome&filter%5Bbrowser%5D%5Bnin%5D%5B0%5D=IE&" +
		"filter%5Bcountry%5D%5Bin%5D%5B0%5D=US&filter%5Bcountry%5D%5Bin%5D%5B1%5D=CA&" +
		"filter%5Bcountry%5D%5Bne%5D=GB&pagePath=%2Ftowels"
	if got != want {
		t.Errorf("encode() =\n%s\nwant\n%s", got, want)
	}
}

// ---------------------------------------------------------------------------
// Traffic
// ---------------------------------------------------------------------------

func trafficInput() GetAnalyticsTrafficInput {
	return GetAnalyticsTrafficInput{
		SiteID: testAnalyzeSiteID, StartTime: testAnalyzeStart, EndTime: testAnalyzeEnd,
		MetricScope: "session", BucketTimeZone: "UTC",
	}
}

func invokeTraffic(in GetAnalyticsTrafficInput) (GetAnalyticsTrafficOutput, error) {
	resp, err := (&GetAnalyticsTraffic{}).Invoke(context.Background(),
		infer.FunctionRequest[GetAnalyticsTrafficInput]{Input: in})
	return resp.Output, err
}

func TestGetAnalyticsTraffic_Success(t *testing.T) {
	rec := newAnalyzeServer(t, http.StatusOK, `{"report":"traffic",`+testAnalyzeWindow+`,
		"metricScope":"session","bucketing":{"granularityPeriod":"day","bucketTimeZone":"UTC"},
		"data":[{"timestamp":"2026-04-01T00:00:00Z","count":1234},{"timestamp":"2026-04-02T00:00:00Z","count":1180}],
		"filter":{"country":{"eq":"US"},"deviceType":{"eq":"desktop"},"os":{"in":["iOS","Android"]}}}`)

	in := trafficInput()
	in.AnalyticsCommonFilters = AnalyticsCommonFilters{
		DeviceType: "desktop", Country: "US",
		Filters: map[string]AnalyticsDimensionFilter{"os": {In: []string{"iOS", "Android"}}},
	}
	out, err := invokeTraffic(in)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	assertAnalyzeRequest(t, rec, "traffic", windowParams(map[string]string{
		"metricScope": "session", "bucketTimeZone": "UTC", "deviceType": "desktop", "country": "US",
		"filter[os][in][0]": "iOS", "filter[os][in][1]": "Android",
	}))
	if out.Report != "traffic" || out.Window.StartTime != testAnalyzeStart || out.Window.EndTime != testAnalyzeEnd ||
		out.MetricScope != "session" {
		t.Errorf("unexpected header fields: %+v", out)
	}
	if out.Bucketing == nil || out.Bucketing.GranularityPeriod != "day" || out.Bucketing.BucketTimeZone != "UTC" {
		t.Errorf("unexpected bucketing: %+v", out.Bucketing)
	}
	if len(out.Data) != 2 || out.Data[0].Timestamp != testAnalyzeStart || out.Data[0].Count != 1234 ||
		out.Data[1].Count != 1180 {
		t.Errorf("unexpected data: %+v", out.Data)
	}
	if out.Filters["country"].Eq != "US" || len(out.Filters["os"].In) != 2 || out.Filters["os"].In[1] != "Android" {
		t.Errorf("unexpected filters: %+v", out.Filters)
	}
}

func TestGetAnalyticsTraffic_NotFound(t *testing.T) {
	notFoundAnalyzeServer(t)

	_, err := invokeTraffic(trafficInput())
	if !IsNotFound(err) || !strings.Contains(err.Error(), "traffic report") {
		t.Fatalf("expected wrapped not-found error, got %v", err)
	}
}

func TestGetAnalyticsTraffic_Validation(t *testing.T) {
	rec := newAnalyzeServer(t, http.StatusOK, `{}`)

	cases := map[string]func(in *GetAnalyticsTrafficInput){
		"siteId":         func(in *GetAnalyticsTrafficInput) { in.SiteID = "x" },
		"metricScope":    func(in *GetAnalyticsTrafficInput) { in.MetricScope = "hits" },
		"bucketTimeZone": func(in *GetAnalyticsTrafficInput) { in.BucketTimeZone = "Mars/Olympus" },
		"endTime":        func(in *GetAnalyticsTrafficInput) { in.EndTime = "" },
		"deviceType":     func(in *GetAnalyticsTrafficInput) { in.DeviceType = "watch" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := trafficInput()
			mutate(&in)
			_, err := invokeTraffic(in)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("expected error mentioning %q, got %v", name, err)
			}
		})
	}
	if rec.method != "" {
		t.Errorf("validation failures must not reach the API, got %s %s", rec.method, rec.path)
	}
}

// ---------------------------------------------------------------------------
// Top pages
// ---------------------------------------------------------------------------

func topPagesInput() GetAnalyticsTopPagesInput {
	return GetAnalyticsTopPagesInput{SiteID: testAnalyzeSiteID, StartTime: testAnalyzeStart, EndTime: testAnalyzeEnd}
}

func invokeTopPages(in GetAnalyticsTopPagesInput) (GetAnalyticsTopPagesOutput, error) {
	resp, err := (&GetAnalyticsTopPages{}).Invoke(context.Background(),
		infer.FunctionRequest[GetAnalyticsTopPagesInput]{Input: in})
	return resp.Output, err
}

func TestGetAnalyticsTopPages_Success(t *testing.T) {
	rec := newAnalyzeServer(t, http.StatusOK, `{"report":"top_pages",`+testAnalyzeWindow+`,
		"sortBy":"pageview","limit":10,
		"data":[{"pageId":"`+testAnalyzePageID+`","title":"Towels","sessionCount":4242,"userCount":3900,
		         "pageviewCount":5400,"collectionId":"c1","itemSlug":"towels",
		         "timeseries":[{"timestamp":"2026-04-01T00:00:00Z","pageviewCount":720}]}],
		"bucketing":{"granularityPeriod":"day","bucketTimeZone":"America/New_York"}}`)

	in := topPagesInput()
	in.SortBy, in.Limit = "pageview", 10
	in.Timeseries = &AnalyticsTimeseriesArgs{BucketTimeZone: "America/New_York"}
	in.AnalyticsCommonFilters = AnalyticsCommonFilters{Referrer: "google.com", UtmSource: "newsletter"}
	out, err := invokeTopPages(in)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	assertAnalyzeRequest(t, rec, "top_pages", windowParams(map[string]string{
		"sortBy": "pageview", "limit": "10", "timeseries": `{"bucketTimeZone":"America/New_York"}`,
		"referrer": "google.com", "utmSource": "newsletter",
	}))
	if out.Report != "top_pages" || out.SortBy != "pageview" || out.Limit != 10 || out.Bucketing == nil ||
		out.Bucketing.BucketTimeZone != "America/New_York" {
		t.Errorf("unexpected header fields: %+v", out)
	}
	if len(out.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(out.Data))
	}
	row := out.Data[0]
	if row.PageID != testAnalyzePageID || row.Title != "Towels" || row.SessionCount != 4242 || row.UserCount != 3900 ||
		row.PageviewCount != 5400 || row.CollectionID != "c1" || row.ItemSlug != "towels" {
		t.Errorf("unexpected row: %+v", row)
	}
	if len(row.Timeseries) != 1 || row.Timeseries[0].PageviewCount != 720 {
		t.Errorf("unexpected timeseries: %+v", row.Timeseries)
	}
}

func TestGetAnalyticsTopPages_NotFound(t *testing.T) {
	notFoundAnalyzeServer(t)

	if _, err := invokeTopPages(topPagesInput()); !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestGetAnalyticsTopPages_Validation(t *testing.T) {
	cases := map[string]func(in *GetAnalyticsTopPagesInput){
		"limit":                     func(in *GetAnalyticsTopPagesInput) { in.Limit = 251 },
		"sortBy":                    func(in *GetAnalyticsTopPagesInput) { in.SortBy = "hits" },
		"timeseries.bucketTimeZone": func(in *GetAnalyticsTopPagesInput) { in.Timeseries = &AnalyticsTimeseriesArgs{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := topPagesInput()
			mutate(&in)
			_, err := invokeTopPages(in)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("expected error mentioning %q, got %v", name, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Top dimensions
// ---------------------------------------------------------------------------

func topDimensionsInput() GetAnalyticsTopDimensionsInput {
	return GetAnalyticsTopDimensionsInput{
		SiteID: testAnalyzeSiteID, StartTime: testAnalyzeStart, EndTime: testAnalyzeEnd,
		Dimension: "region", MetricScope: "user",
	}
}

func invokeTopDimensions(in GetAnalyticsTopDimensionsInput) (GetAnalyticsTopDimensionsOutput, error) {
	resp, err := (&GetAnalyticsTopDimensions{}).Invoke(context.Background(),
		infer.FunctionRequest[GetAnalyticsTopDimensionsInput]{Input: in})
	return resp.Output, err
}

func TestGetAnalyticsTopDimensions_Success(t *testing.T) {
	rec := newAnalyzeServer(t, http.StatusOK, `{"report":"top_dimensions",`+testAnalyzeWindow+`,
		"dimension":"region","metricScope":"user","limit":25,
		"data":[{"attributeKey":"GB-ENG","name":"England, United Kingdom","count":767},
		        {"attributeKey":"US-CA","name":"California, United States","count":686}],
		"filter":{"browser":{"eq":"Chrome"}}}`)

	in := topDimensionsInput()
	in.AnalyticsCommonFilters = AnalyticsCommonFilters{Browser: "Chrome"}
	out, err := invokeTopDimensions(in)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	assertAnalyzeRequest(t, rec, "top_dimensions", windowParams(map[string]string{
		"dimension": "region", "metricScope": "user", "browser": "Chrome",
	}))
	if out.Report != "top_dimensions" || out.Dimension != "region" || out.MetricScope != "user" || out.Limit != 25 {
		t.Errorf("unexpected header fields: %+v", out)
	}
	if len(out.Data) != 2 || out.Data[0].AttributeKey != "GB-ENG" || out.Data[0].Name != "England, United Kingdom" ||
		out.Data[0].Count != 767 || out.Data[1].Count != 686 {
		t.Errorf("unexpected data: %+v", out.Data)
	}
	if out.Filters["browser"].Eq != "Chrome" {
		t.Errorf("unexpected filters: %+v", out.Filters)
	}
}

func TestGetAnalyticsTopDimensions_NotFound(t *testing.T) {
	notFoundAnalyzeServer(t)

	if _, err := invokeTopDimensions(topDimensionsInput()); !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestGetAnalyticsTopDimensions_Validation(t *testing.T) {
	cases := map[string]func(in *GetAnalyticsTopDimensionsInput){
		"dimension":   func(in *GetAnalyticsTopDimensionsInput) { in.Dimension = "colour" },
		"metricScope": func(in *GetAnalyticsTopDimensionsInput) { in.MetricScope = "pageview" },
		"limit":       func(in *GetAnalyticsTopDimensionsInput) { in.Limit = 101 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := topDimensionsInput()
			mutate(&in)
			_, err := invokeTopDimensions(in)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("expected error mentioning %q, got %v", name, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Top events
// ---------------------------------------------------------------------------

func topEventsInput() GetAnalyticsTopEventsInput {
	return GetAnalyticsTopEventsInput{SiteID: testAnalyzeSiteID, StartTime: testAnalyzeStart, EndTime: testAnalyzeEnd}
}

func invokeTopEvents(in GetAnalyticsTopEventsInput) (GetAnalyticsTopEventsOutput, error) {
	resp, err := (&GetAnalyticsTopEvents{}).Invoke(context.Background(),
		infer.FunctionRequest[GetAnalyticsTopEventsInput]{Input: in})
	return resp.Output, err
}

func TestGetAnalyticsTopEvents_Success(t *testing.T) {
	rec := newAnalyzeServer(t, http.StatusOK, `{"report":"top_events",`+testAnalyzeWindow+`,"limit":25,
		"data":[{"eventId":"fb30dabb","count":4300,"name":"Towel preview","pageId":"`+testAnalyzePageID+`",
		         "pageName":"Towels","componentContext":[{"componentId":"comp1","instanceId":"inst1"}],
		         "cmsContext":[{"collectionId":"c1","itemId":"i1"}],"collectionId":"c1","itemSlug":"towels",
		         "timeseries":[{"timestamp":"2026-04-01T00:00:00Z","count":620}]}],
		"bucketing":{"granularityPeriod":"day","bucketTimeZone":"UTC"}}`)

	in := topEventsInput()
	in.Timeseries = &AnalyticsTimeseriesArgs{BucketTimeZone: "UTC"}
	in.AnalyticsCommonFilters = AnalyticsCommonFilters{
		Filters: map[string]AnalyticsDimensionFilter{"pageId": {Eq: testAnalyzePageID}},
	}
	out, err := invokeTopEvents(in)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	assertAnalyzeRequest(t, rec, "top_events", windowParams(map[string]string{
		"timeseries": `{"bucketTimeZone":"UTC"}`, "filter[pageId][eq]": testAnalyzePageID,
	}))
	if out.Report != "top_events" || out.Limit != 25 || out.Bucketing == nil || out.Bucketing.BucketTimeZone != "UTC" {
		t.Errorf("unexpected header fields: %+v", out)
	}
	if len(out.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(out.Data))
	}
	row := out.Data[0]
	if row.EventID != "fb30dabb" || row.Count != 4300 || row.Name != "Towel preview" || row.PageID != testAnalyzePageID ||
		row.PageName != "Towels" || row.CollectionID != "c1" || row.ItemSlug != "towels" {
		t.Errorf("unexpected row: %+v", row)
	}
	if len(row.ComponentContext) != 1 || row.ComponentContext[0].InstanceID != "inst1" ||
		len(row.CmsContext) != 1 || row.CmsContext[0].ItemID != "i1" ||
		len(row.Timeseries) != 1 || row.Timeseries[0].Count != 620 {
		t.Errorf("unexpected row context: %+v", row)
	}
}

func TestGetAnalyticsTopEvents_NotFound(t *testing.T) {
	notFoundAnalyzeServer(t)

	if _, err := invokeTopEvents(topEventsInput()); !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestGetAnalyticsTopEvents_RejectsReferrer(t *testing.T) {
	in := topEventsInput()
	in.AnalyticsCommonFilters = AnalyticsCommonFilters{Referrer: "google.com"}
	_, err := invokeTopEvents(in)
	if err == nil || !strings.Contains(err.Error(), "referrer") {
		t.Fatalf("expected referrer error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Time on page
// ---------------------------------------------------------------------------

func timeOnPageInput() GetAnalyticsTimeOnPageInput {
	return GetAnalyticsTimeOnPageInput{
		SiteID: testAnalyzeSiteID, StartTime: testAnalyzeStart, EndTime: testAnalyzeEnd, MetricScope: "session",
	}
}

func invokeTimeOnPage(in GetAnalyticsTimeOnPageInput) (GetAnalyticsTimeOnPageOutput, error) {
	resp, err := (&GetAnalyticsTimeOnPage{}).Invoke(context.Background(),
		infer.FunctionRequest[GetAnalyticsTimeOnPageInput]{Input: in})
	return resp.Output, err
}

func TestGetAnalyticsTimeOnPage_Success(t *testing.T) {
	rec := newAnalyzeServer(t, http.StatusOK, `{"report":"time_on_page",`+testAnalyzeWindow+`,
		"metricScope":"session",
		"data":[{"timestamp":"2026-04-01T00:00:00Z","averageSeconds":47.2},
		        {"timestamp":"2026-04-08T00:00:00Z","averageSeconds":51.8}],
		"bucketing":{"granularityPeriod":"week","bucketTimeZone":"UTC"},"filter":{"pagePath":{"eq":"/towels"}}}`)

	in := timeOnPageInput()
	in.Timeseries = &AnalyticsTimeOnPageTimeseriesArgs{GranularityPeriod: "week", BucketTimeZone: "UTC"}
	in.AnalyticsCommonFilters = AnalyticsCommonFilters{PagePath: "/towels"}
	out, err := invokeTimeOnPage(in)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	assertAnalyzeRequest(t, rec, "time_on_page", windowParams(map[string]string{
		"metricScope": "session", "timeseries": `{"bucketTimeZone":"UTC","granularityPeriod":"week"}`,
		"pagePath": "/towels",
	}))
	if out.Report != "time_on_page" || out.MetricScope != "session" || out.Bucketing == nil ||
		out.Bucketing.GranularityPeriod != "week" {
		t.Errorf("unexpected header fields: %+v", out)
	}
	if len(out.Data) != 2 || out.Data[0].AverageSeconds != 47.2 || out.Data[1].AverageSeconds != 51.8 {
		t.Errorf("unexpected data: %+v", out.Data)
	}
	if out.Filters["pagePath"].Eq != "/towels" {
		t.Errorf("unexpected filters: %+v", out.Filters)
	}
}

func TestGetAnalyticsTimeOnPage_SingleAggregate(t *testing.T) {
	rec := newAnalyzeServer(t, http.StatusOK, `{"report":"time_on_page",`+testAnalyzeWindow+`,
		"metricScope":"pageview","data":[{"timestamp":"2026-04-01T00:00:00Z","averageSeconds":30}]}`)

	in := timeOnPageInput()
	in.MetricScope = "pageview"
	out, err := invokeTimeOnPage(in)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	assertAnalyzeRequest(t, rec, "time_on_page", windowParams(map[string]string{"metricScope": "pageview"}))
	if out.Bucketing != nil || len(out.Data) != 1 || out.Data[0].AverageSeconds != 30 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestGetAnalyticsTimeOnPage_NotFound(t *testing.T) {
	notFoundAnalyzeServer(t)

	if _, err := invokeTimeOnPage(timeOnPageInput()); !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestGetAnalyticsTimeOnPage_Validation(t *testing.T) {
	cases := map[string]func(in *GetAnalyticsTimeOnPageInput){
		"metricScope": func(in *GetAnalyticsTimeOnPageInput) { in.MetricScope = "" },
		"timeseries.granularityPeriod": func(in *GetAnalyticsTimeOnPageInput) {
			in.Timeseries = &AnalyticsTimeOnPageTimeseriesArgs{GranularityPeriod: "month", BucketTimeZone: "UTC"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := timeOnPageInput()
			mutate(&in)
			_, err := invokeTimeOnPage(in)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("expected error mentioning %q, got %v", name, err)
			}
		})
	}
}

// TestNewFeatures_SchemaGeneration verifies that the infer framework accepts every new resource and
// function type (embedded structs, pointer optionals, maps of structs) by generating the schema.
func TestNewFeatures_SchemaGeneration(t *testing.T) {
	prov, err := infer.NewProviderBuilder().
		WithNamespace(Name).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{"provider": "index"}).
		WithResources(
			infer.Resource(&GoogleTag{}),
			infer.Resource(&PageSchemaMarkup{}),
		).
		WithFunctions(
			infer.Function(&GetPageSchemaMarkup{}),
			infer.Function(&GetAnalyticsTraffic{}),
			infer.Function(&GetAnalyticsTopPages{}),
			infer.Function(&GetAnalyticsTopDimensions{}),
			infer.Function(&GetAnalyticsTopEvents{}),
			infer.Function(&GetAnalyticsTimeOnPage{}),
		).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server, err := integration.NewServer(context.Background(), Name, semver.MustParse("0.0.1"),
		integration.WithProvider(prov))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	schema, err := server.GetSchema(p.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	for _, token := range []string{
		`"webflow:index:GoogleTag"`, `"webflow:index:PageSchemaMarkup"`,
		`"webflow:index:getPageSchemaMarkup"`, `"webflow:index:getAnalyticsTraffic"`,
		`"webflow:index:getAnalyticsTopPages"`, `"webflow:index:getAnalyticsTopDimensions"`,
		`"webflow:index:getAnalyticsTopEvents"`, `"webflow:index:getAnalyticsTimeOnPage"`,
	} {
		if !strings.Contains(schema.Schema, token) {
			t.Errorf("schema is missing %s", token)
		}
	}
}
