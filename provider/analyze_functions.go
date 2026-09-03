// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// This file defines the five Pulumi invoke functions backed by the Webflow Analyze API
// (GET /v2/sites/{site_id}/analyze/reports/{report}, with a transitional /beta fallback; see
// analyze.go). All of them require the 'sites:read' scope and a workspace with the Analyze add-on.

const analyzeScopeNote = " Requires the 'sites:read' scope and a Webflow workspace with the Analyze add-on. " +
	"Webflow allows one Analyze request in flight per access token; concurrent requests are retried. " +
	"The reporting window must start on or after 2025-04-09 and span at most 100 days."

const analyzeTimeDescription = "UTC timestamp in ISO 8601 / RFC 3339 format ending in 'Z' " +
	"(e.g., '2026-04-01T00:00:00Z'). Numeric offsets are not accepted."

const (
	analyzeSiteIDDescription    = "The Webflow site ID (24-character lowercase hexadecimal string)."
	analyzeStartTimeDescription = "Inclusive start of the reporting window. " + analyzeTimeDescription +
		" Must be on or after 2025-04-09T00:00:00Z."
	analyzeEndTimeDescription = "Exclusive end of the reporting window. " + analyzeTimeDescription +
		" Must be after startTime and within 100 days of it."
	analyzeWindowDescription  = "The reporting window that was applied."
	analyzeFiltersDescription = "The filters that were applied, keyed by dimension."
	analyzeLimitDescription   = "The row cap that was applied (echoes the request value, including the default)."
)

// AnalyticsTimeseriesArgs requests a daily per-row time series (top pages and top events).
type AnalyticsTimeseriesArgs struct {
	// BucketTimeZone is the IANA time zone used to align daily bucket boundaries.
	BucketTimeZone string `pulumi:"bucketTimeZone"`
}

// Annotate adds descriptions to the AnalyticsTimeseriesArgs fields.
func (t *AnalyticsTimeseriesArgs) Annotate(a infer.Annotator) {
	a.Describe(&t.BucketTimeZone,
		"IANA time zone used to align daily bucket boundaries (e.g., 'UTC', 'America/New_York'). "+
			"Bucket timestamps are returned as UTC instants of local midnight in this zone.")
}

// AnalyticsTimeOnPageTimeseriesArgs requests bucketed averages for the time-on-page report.
type AnalyticsTimeOnPageTimeseriesArgs struct {
	// GranularityPeriod is the bucket size: "day" or "week".
	GranularityPeriod string `pulumi:"granularityPeriod"`
	// BucketTimeZone is the IANA time zone used to align bucket boundaries.
	BucketTimeZone string `pulumi:"bucketTimeZone"`
}

// Annotate adds descriptions to the AnalyticsTimeOnPageTimeseriesArgs fields.
func (t *AnalyticsTimeOnPageTimeseriesArgs) Annotate(a infer.Annotator) {
	a.Describe(&t.GranularityPeriod, "Size of each bucket: 'day' or 'week'.")
	a.Describe(&t.BucketTimeZone,
		"IANA time zone used to align bucket boundaries (e.g., 'UTC', 'America/New_York').")
}

// ---------------------------------------------------------------------------
// Traffic
// ---------------------------------------------------------------------------

// GetAnalyticsTraffic returns a daily time series of sessions, users or pageviews.
// It calls GET /v2/sites/{site_id}/analyze/reports/traffic.
type GetAnalyticsTraffic struct{}

// GetAnalyticsTrafficInput defines the input parameters for GetAnalyticsTraffic.
type GetAnalyticsTrafficInput struct {
	// SiteID is the Webflow site ID.
	SiteID string `pulumi:"siteId"`
	// StartTime is the inclusive start of the reporting window.
	StartTime string `pulumi:"startTime"`
	// EndTime is the exclusive end of the reporting window.
	EndTime string `pulumi:"endTime"`
	// MetricScope is the unit of each count: "session", "user" or "pageview".
	MetricScope string `pulumi:"metricScope"`
	// BucketTimeZone is the IANA time zone used to align daily buckets.
	BucketTimeZone string `pulumi:"bucketTimeZone"`
	AnalyticsCommonFilters
}

// GetAnalyticsTrafficOutput defines the output of GetAnalyticsTraffic.
type GetAnalyticsTrafficOutput struct {
	// Report is the report name ("traffic").
	Report string `pulumi:"report" json:"report"`
	// Window is the reporting window applied.
	Window AnalyticsWindow `pulumi:"window" json:"window"`
	// MetricScope is the unit of each count.
	MetricScope string `pulumi:"metricScope" json:"metricScope"`
	// Bucketing describes the daily bucketing applied.
	Bucketing *AnalyticsBucketing `pulumi:"bucketing,optional" json:"bucketing,omitempty"`
	// Data is the daily time series.
	Data []AnalyticsCountPoint `pulumi:"data" json:"data"`
	// Filters echoes the filters applied.
	Filters map[string]AnalyticsDimensionFilter `pulumi:"filters,optional" json:"filter,omitempty"`
}

// Annotate adds descriptions to the GetAnalyticsTraffic function.
func (f *GetAnalyticsTraffic) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns a daily time series of sessions, users or pageviews for a Webflow site over a "+
		"chosen window, optionally filtered by device, country, page, traffic source, referrer, browser or "+
		"UTM parameters (Analyze API, beta)."+analyzeScopeNote)
}

// Annotate adds descriptions to the GetAnalyticsTrafficInput fields.
func (i *GetAnalyticsTrafficInput) Annotate(a infer.Annotator) {
	a.Describe(&i.SiteID, analyzeSiteIDDescription)
	a.Describe(&i.StartTime, analyzeStartTimeDescription)
	a.Describe(&i.EndTime, analyzeEndTimeDescription)
	a.Describe(&i.MetricScope,
		"Unit of each data point: 'session' (sessions), 'user' (unique users) or 'pageview'.")
	a.Describe(&i.BucketTimeZone,
		"IANA time zone used to align daily bucket boundaries (e.g., 'UTC', 'America/New_York').")
}

// Annotate adds descriptions to the GetAnalyticsTrafficOutput fields.
func (o *GetAnalyticsTrafficOutput) Annotate(a infer.Annotator) {
	a.Describe(&o.Report, "The report name, always 'traffic'.")
	a.Describe(&o.Window, analyzeWindowDescription)
	a.Describe(&o.MetricScope, "The unit each count is measured in: 'session', 'user' or 'pageview'.")
	a.Describe(&o.Bucketing, "The bucketing applied to the time series (daily, in the requested time zone).")
	a.Describe(&o.Data, "One data point per day, ordered by timestamp.")
	a.Describe(&o.Filters, analyzeFiltersDescription)
}

// buildQuery validates the inputs and builds the query string.
func (i GetAnalyticsTrafficInput) buildQuery() (*analyzeQuery, error) {
	const fn = "GetAnalyticsTraffic"
	if err := validateAnalyzeCommon(fn, i.SiteID, i.StartTime, i.EndTime, i.AnalyticsCommonFilters); err != nil {
		return nil, err
	}
	if err := validateEnum("metricScope", i.MetricScope, analyzeMetricScopes, true); err != nil {
		return nil, validationError(fn, err)
	}
	if err := validateBucketTimeZone("bucketTimeZone", i.BucketTimeZone, true); err != nil {
		return nil, validationError(fn, err)
	}

	q := newAnalyzeQuery()
	q.set("startTime", i.StartTime)
	q.set("endTime", i.EndTime)
	q.set("metricScope", i.MetricScope)
	q.set("bucketTimeZone", i.BucketTimeZone)
	q.addCommonFilters(i.AnalyticsCommonFilters)
	return q, nil
}

// Invoke implements infer.Fn for GetAnalyticsTraffic.
func (f *GetAnalyticsTraffic) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetAnalyticsTrafficInput],
) (infer.FunctionResponse[GetAnalyticsTrafficOutput], error) {
	var out GetAnalyticsTrafficOutput
	q, err := req.Input.buildQuery()
	if err != nil {
		return infer.FunctionResponse[GetAnalyticsTrafficOutput]{}, err
	}
	if err := invokeAnalyze(ctx, req.Input.SiteID, analyzeReportTraffic, q, &out); err != nil {
		return infer.FunctionResponse[GetAnalyticsTrafficOutput]{}, err
	}
	if out.Data == nil {
		out.Data = []AnalyticsCountPoint{}
	}
	return infer.FunctionResponse[GetAnalyticsTrafficOutput]{Output: out}, nil
}

// ---------------------------------------------------------------------------
// Top pages
// ---------------------------------------------------------------------------

// GetAnalyticsTopPages returns the most-visited pages of a site.
// It calls GET /v2/sites/{site_id}/analyze/reports/top_pages.
type GetAnalyticsTopPages struct{}

// GetAnalyticsTopPagesInput defines the input parameters for GetAnalyticsTopPages.
type GetAnalyticsTopPagesInput struct {
	// SiteID is the Webflow site ID.
	SiteID string `pulumi:"siteId"`
	// StartTime is the inclusive start of the reporting window.
	StartTime string `pulumi:"startTime"`
	// EndTime is the exclusive end of the reporting window.
	EndTime string `pulumi:"endTime"`
	// SortBy ranks rows by "session" (default), "user" or "pageview".
	SortBy string `pulumi:"sortBy,optional"`
	// Limit caps the number of rows (default 25, max 250).
	Limit int `pulumi:"limit,optional"`
	// Timeseries requests a daily pageview series per page.
	Timeseries *AnalyticsTimeseriesArgs `pulumi:"timeseries,optional"`
	AnalyticsCommonFilters
}

// AnalyticsPageviewPoint is one daily pageview data point of a top-pages row.
type AnalyticsPageviewPoint struct {
	// Timestamp is the bucket start as a UTC instant.
	Timestamp string `pulumi:"timestamp" json:"timestamp"`
	// PageviewCount is the number of pageviews in the bucket.
	PageviewCount int `pulumi:"pageviewCount" json:"pageviewCount"`
}

// Annotate adds descriptions to the AnalyticsPageviewPoint fields.
func (p *AnalyticsPageviewPoint) Annotate(a infer.Annotator) {
	a.Describe(&p.Timestamp, "Start of the daily bucket as a UTC instant in ISO 8601 / RFC 3339 format.")
	a.Describe(&p.PageviewCount, "Number of pageviews in the bucket.")
}

// AnalyticsTopPageRow is one ranked page in the top-pages report.
type AnalyticsTopPageRow struct {
	// PageID is the Webflow page ID.
	PageID string `pulumi:"pageId" json:"pageId"`
	// Title is the page title.
	Title string `pulumi:"title" json:"title"`
	// SessionCount is the number of sessions that included the page.
	SessionCount int `pulumi:"sessionCount" json:"sessionCount"`
	// UserCount is the number of unique users who viewed the page.
	UserCount int `pulumi:"userCount" json:"userCount"`
	// PageviewCount is the number of pageviews.
	PageviewCount int `pulumi:"pageviewCount" json:"pageviewCount"`
	// CollectionID is set for CMS collection pages.
	CollectionID string `pulumi:"collectionId,optional" json:"collectionId,omitempty"`
	// ItemSlug is set for CMS collection pages.
	ItemSlug string `pulumi:"itemSlug,optional" json:"itemSlug,omitempty"`
	// Timeseries is the daily pageview series when requested.
	Timeseries []AnalyticsPageviewPoint `pulumi:"timeseries,optional" json:"timeseries,omitempty"`
}

// Annotate adds descriptions to the AnalyticsTopPageRow fields.
func (r *AnalyticsTopPageRow) Annotate(a infer.Annotator) {
	a.Describe(&r.PageID, "The Webflow page ID.")
	a.Describe(&r.Title, "The page title.")
	a.Describe(&r.SessionCount, "Number of sessions that included a view of this page.")
	a.Describe(&r.UserCount, "Number of unique users who viewed this page.")
	a.Describe(&r.PageviewCount, "Number of pageviews of this page.")
	a.Describe(&r.CollectionID, "The CMS collection ID, for collection template pages.")
	a.Describe(&r.ItemSlug, "The CMS item slug, for collection template pages.")
	a.Describe(&r.Timeseries,
		"Daily pageview counts for this page; present only when 'timeseries' was requested.")
}

// GetAnalyticsTopPagesOutput defines the output of GetAnalyticsTopPages.
type GetAnalyticsTopPagesOutput struct {
	// Report is the report name ("top_pages").
	Report string `pulumi:"report" json:"report"`
	// Window is the reporting window applied.
	Window AnalyticsWindow `pulumi:"window" json:"window"`
	// SortBy is the metric rows were ranked by.
	SortBy string `pulumi:"sortBy" json:"sortBy"`
	// Limit is the row cap that was applied.
	Limit int `pulumi:"limit" json:"limit"`
	// Data is the ranked list of pages.
	Data []AnalyticsTopPageRow `pulumi:"data" json:"data"`
	// Bucketing describes the time-series bucketing when requested.
	Bucketing *AnalyticsBucketing `pulumi:"bucketing,optional" json:"bucketing,omitempty"`
	// Filters echoes the filters applied.
	Filters map[string]AnalyticsDimensionFilter `pulumi:"filters,optional" json:"filter,omitempty"`
}

// Annotate adds descriptions to the GetAnalyticsTopPages function.
func (f *GetAnalyticsTopPages) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the most-visited pages of a Webflow site, ranked by sessions, users or pageviews, "+
		"with an optional daily pageview time series per page (Analyze API, beta)."+analyzeScopeNote)
}

// Annotate adds descriptions to the GetAnalyticsTopPagesInput fields.
func (i *GetAnalyticsTopPagesInput) Annotate(a infer.Annotator) {
	a.Describe(&i.SiteID, analyzeSiteIDDescription)
	a.Describe(&i.StartTime, analyzeStartTimeDescription)
	a.Describe(&i.EndTime, analyzeEndTimeDescription)
	a.Describe(&i.SortBy, "Metric used to rank rows, descending: 'session' (default), 'user' or 'pageview'.")
	a.Describe(&i.Limit, "Maximum number of rows to return. Defaults to 25, up to a maximum of 250.")
	a.Describe(&i.Timeseries,
		"Include a daily pageview time series for each page, bucketed in the given IANA time zone. "+
			"Omit to return ranked rows only.")
}

// Annotate adds descriptions to the GetAnalyticsTopPagesOutput fields.
func (o *GetAnalyticsTopPagesOutput) Annotate(a infer.Annotator) {
	a.Describe(&o.Report, "The report name, always 'top_pages'.")
	a.Describe(&o.Window, analyzeWindowDescription)
	a.Describe(&o.SortBy, "The metric rows were ranked by: 'session', 'user' or 'pageview'.")
	a.Describe(&o.Limit, analyzeLimitDescription)
	a.Describe(&o.Data, "The ranked pages, most visited first.")
	a.Describe(&o.Bucketing,
		"The bucketing applied to per-page time series; present only when 'timeseries' was requested.")
	a.Describe(&o.Filters, analyzeFiltersDescription)
}

// buildQuery validates the inputs and builds the query string.
func (i GetAnalyticsTopPagesInput) buildQuery() (*analyzeQuery, error) {
	const fn = "GetAnalyticsTopPages"
	if err := validateAnalyzeCommon(fn, i.SiteID, i.StartTime, i.EndTime, i.AnalyticsCommonFilters); err != nil {
		return nil, err
	}
	if err := validateEnum("sortBy", i.SortBy, analyzeMetricScopes, false); err != nil {
		return nil, validationError(fn, err)
	}
	if err := validateLimit(i.Limit, analyzeTopPagesMaxLimit); err != nil {
		return nil, validationError(fn, err)
	}

	q := newAnalyzeQuery()
	q.set("startTime", i.StartTime)
	q.set("endTime", i.EndTime)
	q.set("sortBy", i.SortBy)
	q.setInt("limit", i.Limit)
	if i.Timeseries != nil {
		if err := q.setDailyTimeseries(i.Timeseries.BucketTimeZone); err != nil {
			return nil, validationError(fn, err)
		}
	}
	q.addCommonFilters(i.AnalyticsCommonFilters)
	return q, nil
}

// Invoke implements infer.Fn for GetAnalyticsTopPages.
func (f *GetAnalyticsTopPages) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetAnalyticsTopPagesInput],
) (infer.FunctionResponse[GetAnalyticsTopPagesOutput], error) {
	var out GetAnalyticsTopPagesOutput
	q, err := req.Input.buildQuery()
	if err != nil {
		return infer.FunctionResponse[GetAnalyticsTopPagesOutput]{}, err
	}
	if err := invokeAnalyze(ctx, req.Input.SiteID, analyzeReportTopPages, q, &out); err != nil {
		return infer.FunctionResponse[GetAnalyticsTopPagesOutput]{}, err
	}
	if out.Data == nil {
		out.Data = []AnalyticsTopPageRow{}
	}
	return infer.FunctionResponse[GetAnalyticsTopPagesOutput]{Output: out}, nil
}

// ---------------------------------------------------------------------------
// Top dimensions
// ---------------------------------------------------------------------------

// GetAnalyticsTopDimensions returns the top values of a chosen dimension.
// It calls GET /v2/sites/{site_id}/analyze/reports/top_dimensions.
type GetAnalyticsTopDimensions struct{}

// GetAnalyticsTopDimensionsInput defines the input parameters for GetAnalyticsTopDimensions.
type GetAnalyticsTopDimensionsInput struct {
	// SiteID is the Webflow site ID.
	SiteID string `pulumi:"siteId"`
	// StartTime is the inclusive start of the reporting window.
	StartTime string `pulumi:"startTime"`
	// EndTime is the exclusive end of the reporting window.
	EndTime string `pulumi:"endTime"`
	// Dimension is the dimension whose top values are ranked.
	Dimension string `pulumi:"dimension"`
	// MetricScope is the unit of each count: "session" or "user".
	MetricScope string `pulumi:"metricScope"`
	// Limit caps the number of rows (default 25, max 100).
	Limit int `pulumi:"limit,optional"`
	AnalyticsCommonFilters
}

// AnalyticsDimensionRow is one ranked value in the top-dimensions report.
type AnalyticsDimensionRow struct {
	// AttributeKey is the raw dimension value (e.g., "US-CA").
	AttributeKey string `pulumi:"attributeKey" json:"attributeKey"`
	// Name is the human-readable label (e.g., "California, United States").
	Name string `pulumi:"name" json:"name"`
	// Count is the number of sessions or users.
	Count int `pulumi:"count" json:"count"`
}

// Annotate adds descriptions to the AnalyticsDimensionRow fields.
func (r *AnalyticsDimensionRow) Annotate(a infer.Annotator) {
	a.Describe(&r.AttributeKey,
		"The raw value of the dimension (e.g., 'US-CA' for a region, 'SO' for a traffic source).")
	a.Describe(&r.Name, "A human-readable label for the value (e.g., 'California, United States').")
	a.Describe(&r.Count, "Number of sessions or users with this value, per the requested metric scope.")
}

// GetAnalyticsTopDimensionsOutput defines the output of GetAnalyticsTopDimensions.
type GetAnalyticsTopDimensionsOutput struct {
	// Report is the report name ("top_dimensions").
	Report string `pulumi:"report" json:"report"`
	// Window is the reporting window applied.
	Window AnalyticsWindow `pulumi:"window" json:"window"`
	// Dimension is the dimension that was ranked.
	Dimension string `pulumi:"dimension" json:"dimension"`
	// MetricScope is the unit of each count.
	MetricScope string `pulumi:"metricScope" json:"metricScope"`
	// Limit is the row cap that was applied.
	Limit int `pulumi:"limit" json:"limit"`
	// Data is the ranked list of dimension values.
	Data []AnalyticsDimensionRow `pulumi:"data" json:"data"`
	// Filters echoes the filters applied.
	Filters map[string]AnalyticsDimensionFilter `pulumi:"filters,optional" json:"filter,omitempty"`
}

// Annotate adds descriptions to the GetAnalyticsTopDimensions function.
func (f *GetAnalyticsTopDimensions) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the top values of a chosen dimension (country, region, device type, OS, browser, "+
		"language, locale, referrer, traffic source, UTM parameters or audience) for a Webflow site, ranked by "+
		"sessions or users (Analyze API, beta)."+analyzeScopeNote)
}

// Annotate adds descriptions to the GetAnalyticsTopDimensionsInput fields.
func (i *GetAnalyticsTopDimensionsInput) Annotate(a infer.Annotator) {
	a.Describe(&i.SiteID, analyzeSiteIDDescription)
	a.Describe(&i.StartTime, analyzeStartTimeDescription)
	a.Describe(&i.EndTime, analyzeEndTimeDescription)
	a.Describe(&i.Dimension, "The dimension to rank. One of: 'country', 'region', 'deviceType', 'os', 'browser', "+
		"'language', 'locale', 'referrer', 'trafficSource', 'utmCampaign', 'utmContent', 'utmMedium', "+
		"'utmSource', 'utmTerm', 'audienceIds'.")
	a.Describe(&i.MetricScope, "Unit of each row's count: 'session' or 'user'.")
	a.Describe(&i.Limit, "Maximum number of rows to return. Defaults to 25, up to a maximum of 100.")
}

// Annotate adds descriptions to the GetAnalyticsTopDimensionsOutput fields.
func (o *GetAnalyticsTopDimensionsOutput) Annotate(a infer.Annotator) {
	a.Describe(&o.Report, "The report name, always 'top_dimensions'.")
	a.Describe(&o.Window, analyzeWindowDescription)
	a.Describe(&o.Dimension, "The dimension whose values were ranked.")
	a.Describe(&o.MetricScope, "The unit each count is measured in: 'session' or 'user'.")
	a.Describe(&o.Limit, analyzeLimitDescription)
	a.Describe(&o.Data, "The ranked dimension values, highest count first.")
	a.Describe(&o.Filters, analyzeFiltersDescription)
}

// buildQuery validates the inputs and builds the query string.
func (i GetAnalyticsTopDimensionsInput) buildQuery() (*analyzeQuery, error) {
	const fn = "GetAnalyticsTopDimensions"
	if err := validateAnalyzeCommon(fn, i.SiteID, i.StartTime, i.EndTime, i.AnalyticsCommonFilters); err != nil {
		return nil, err
	}
	if err := validateEnum("dimension", i.Dimension, analyzeTopDimensionValues, true); err != nil {
		return nil, validationError(fn, err)
	}
	if err := validateEnum("metricScope", i.MetricScope, analyzeTopDimensionScopes, true); err != nil {
		return nil, validationError(fn, err)
	}
	if err := validateLimit(i.Limit, analyzeTopDimensionsMaxLimit); err != nil {
		return nil, validationError(fn, err)
	}

	q := newAnalyzeQuery()
	q.set("startTime", i.StartTime)
	q.set("endTime", i.EndTime)
	q.set("dimension", i.Dimension)
	q.set("metricScope", i.MetricScope)
	q.setInt("limit", i.Limit)
	q.addCommonFilters(i.AnalyticsCommonFilters)
	return q, nil
}

// Invoke implements infer.Fn for GetAnalyticsTopDimensions.
func (f *GetAnalyticsTopDimensions) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetAnalyticsTopDimensionsInput],
) (infer.FunctionResponse[GetAnalyticsTopDimensionsOutput], error) {
	var out GetAnalyticsTopDimensionsOutput
	q, err := req.Input.buildQuery()
	if err != nil {
		return infer.FunctionResponse[GetAnalyticsTopDimensionsOutput]{}, err
	}
	if err := invokeAnalyze(ctx, req.Input.SiteID, analyzeReportTopDimensions, q, &out); err != nil {
		return infer.FunctionResponse[GetAnalyticsTopDimensionsOutput]{}, err
	}
	if out.Data == nil {
		out.Data = []AnalyticsDimensionRow{}
	}
	return infer.FunctionResponse[GetAnalyticsTopDimensionsOutput]{Output: out}, nil
}

// ---------------------------------------------------------------------------
// Top events
// ---------------------------------------------------------------------------

// GetAnalyticsTopEvents returns the most-fired events of a site.
// It calls GET /v2/sites/{site_id}/analyze/reports/top_events.
type GetAnalyticsTopEvents struct{}

// GetAnalyticsTopEventsInput defines the input parameters for GetAnalyticsTopEvents.
type GetAnalyticsTopEventsInput struct {
	// SiteID is the Webflow site ID.
	SiteID string `pulumi:"siteId"`
	// StartTime is the inclusive start of the reporting window.
	StartTime string `pulumi:"startTime"`
	// EndTime is the exclusive end of the reporting window.
	EndTime string `pulumi:"endTime"`
	// Limit caps the number of rows (default 25, max 250).
	Limit int `pulumi:"limit,optional"`
	// Timeseries requests a daily count series per event.
	Timeseries *AnalyticsTimeseriesArgs `pulumi:"timeseries,optional"`
	AnalyticsCommonFilters
}

// AnalyticsComponentContext identifies the component instance an event fired from.
type AnalyticsComponentContext struct {
	// ComponentID is the Webflow component ID.
	ComponentID string `pulumi:"componentId" json:"componentId"`
	// InstanceID is the component instance ID.
	InstanceID string `pulumi:"instanceId" json:"instanceId"`
}

// Annotate adds descriptions to the AnalyticsComponentContext fields.
func (c *AnalyticsComponentContext) Annotate(a infer.Annotator) {
	a.Describe(&c.ComponentID, "The Webflow component ID the event is attached to.")
	a.Describe(&c.InstanceID, "The ID of the component instance the event fired from.")
}

// AnalyticsCmsContext identifies the CMS item an event fired from.
type AnalyticsCmsContext struct {
	// CollectionID is the CMS collection ID.
	CollectionID string `pulumi:"collectionId" json:"collectionId"`
	// ItemID is the CMS item ID.
	ItemID string `pulumi:"itemId" json:"itemId"`
}

// Annotate adds descriptions to the AnalyticsCmsContext fields.
func (c *AnalyticsCmsContext) Annotate(a infer.Annotator) {
	a.Describe(&c.CollectionID, "The CMS collection ID.")
	a.Describe(&c.ItemID, "The CMS item ID.")
}

// AnalyticsTopEventRow is one ranked event in the top-events report.
type AnalyticsTopEventRow struct {
	// EventID is the event identifier.
	EventID string `pulumi:"eventId" json:"eventId"`
	// Count is the number of times the event fired.
	Count int `pulumi:"count" json:"count"`
	// Name is the event name.
	Name string `pulumi:"name,optional" json:"name,omitempty"`
	// PageID is the page the event is attached to.
	PageID string `pulumi:"pageId,optional" json:"pageId,omitempty"`
	// PageName is the name of that page.
	PageName string `pulumi:"pageName,optional" json:"pageName,omitempty"`
	// ComponentContext lists component instances the event fired from.
	ComponentContext []AnalyticsComponentContext `pulumi:"componentContext,optional" json:"componentContext,omitempty"`
	// CmsContext lists CMS items the event fired from.
	CmsContext []AnalyticsCmsContext `pulumi:"cmsContext,optional" json:"cmsContext,omitempty"`
	// CollectionID is set for events on CMS collection pages.
	CollectionID string `pulumi:"collectionId,optional" json:"collectionId,omitempty"`
	// ItemSlug is set for events on CMS collection pages.
	ItemSlug string `pulumi:"itemSlug,optional" json:"itemSlug,omitempty"`
	// Timeseries is the daily count series when requested.
	Timeseries []AnalyticsCountPoint `pulumi:"timeseries,optional" json:"timeseries,omitempty"`
}

// Annotate adds descriptions to the AnalyticsTopEventRow fields.
func (r *AnalyticsTopEventRow) Annotate(a infer.Annotator) {
	a.Describe(&r.EventID, "The event identifier.")
	a.Describe(&r.Count, "Number of times the event fired in the window.")
	a.Describe(&r.Name, "The event name, when available.")
	a.Describe(&r.PageID, "The ID of the page the event is attached to, when available.")
	a.Describe(&r.PageName, "The name of the page the event is attached to, when available.")
	a.Describe(&r.ComponentContext, "Component instances the event fired from, when applicable.")
	a.Describe(&r.CmsContext, "CMS items the event fired from, when applicable.")
	a.Describe(&r.CollectionID, "The CMS collection ID, for events on collection template pages.")
	a.Describe(&r.ItemSlug, "The CMS item slug, for events on collection template pages.")
	a.Describe(&r.Timeseries, "Daily event counts; present only when 'timeseries' was requested.")
}

// GetAnalyticsTopEventsOutput defines the output of GetAnalyticsTopEvents.
type GetAnalyticsTopEventsOutput struct {
	// Report is the report name ("top_events").
	Report string `pulumi:"report" json:"report"`
	// Window is the reporting window applied.
	Window AnalyticsWindow `pulumi:"window" json:"window"`
	// Limit is the row cap that was applied.
	Limit int `pulumi:"limit" json:"limit"`
	// Data is the ranked list of events.
	Data []AnalyticsTopEventRow `pulumi:"data" json:"data"`
	// Bucketing describes the time-series bucketing when requested.
	Bucketing *AnalyticsBucketing `pulumi:"bucketing,optional" json:"bucketing,omitempty"`
	// Filters echoes the filters applied.
	Filters map[string]AnalyticsDimensionFilter `pulumi:"filters,optional" json:"filter,omitempty"`
}

// Annotate adds descriptions to the GetAnalyticsTopEvents function.
func (f *GetAnalyticsTopEvents) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the most-fired events of a Webflow site, ranked by how often they fired, with an "+
		"optional daily time series per event (Analyze API, beta). The top-events report does not support "+
		"the 'referrer' filter, nor the 'nextCollectionId', 'nextItemSlug', 'nextPageId', 'previousCollectionId', "+
		"'previousItemSlug' and 'previousPageId' filter dimensions."+analyzeScopeNote)
}

// Annotate adds descriptions to the GetAnalyticsTopEventsInput fields.
func (i *GetAnalyticsTopEventsInput) Annotate(a infer.Annotator) {
	a.Describe(&i.SiteID, analyzeSiteIDDescription)
	a.Describe(&i.StartTime, analyzeStartTimeDescription)
	a.Describe(&i.EndTime, analyzeEndTimeDescription)
	a.Describe(&i.Limit, "Maximum number of rows to return. Defaults to 25, up to a maximum of 250.")
	a.Describe(&i.Timeseries,
		"Include a daily event count time series for each event, bucketed in the given IANA time zone. "+
			"Omit to return ranked rows only.")
}

// Annotate adds descriptions to the GetAnalyticsTopEventsOutput fields.
func (o *GetAnalyticsTopEventsOutput) Annotate(a infer.Annotator) {
	a.Describe(&o.Report, "The report name, always 'top_events'.")
	a.Describe(&o.Window, analyzeWindowDescription)
	a.Describe(&o.Limit, analyzeLimitDescription)
	a.Describe(&o.Data, "The ranked events, most fired first.")
	a.Describe(&o.Bucketing,
		"The bucketing applied to per-event time series; present only when 'timeseries' was requested.")
	a.Describe(&o.Filters, analyzeFiltersDescription)
}

// buildQuery validates the inputs and builds the query string.
func (i GetAnalyticsTopEventsInput) buildQuery() (*analyzeQuery, error) {
	const fn = "GetAnalyticsTopEvents"
	if err := validateAnalyzeCommon(fn, i.SiteID, i.StartTime, i.EndTime, i.AnalyticsCommonFilters); err != nil {
		return nil, err
	}
	if i.Referrer != "" {
		return nil, validationError(fn, fmt.Errorf("the top-events report does not support the 'referrer' "+
			"filter (got '%s'). Remove it or use a different report", i.Referrer))
	}
	// top_events has a narrower filter schema than the other reports (no referrer, no
	// next*/previous* dimensions); reject those here rather than letting Webflow answer 400.
	if dim := unsupportedFilterDimension(i.Filters, analyzeTopEventsFilterDimensionValues); dim != "" {
		return nil, validationError(fn, fmt.Errorf("the top-events report does not support filtering by '%s'. "+
			"Valid filters dimensions for top_events: %s", dim, strings.Join(analyzeTopEventsFilterDimensionValues, ", ")))
	}
	if err := validateLimit(i.Limit, analyzeTopEventsMaxLimit); err != nil {
		return nil, validationError(fn, err)
	}

	q := newAnalyzeQuery()
	q.set("startTime", i.StartTime)
	q.set("endTime", i.EndTime)
	q.setInt("limit", i.Limit)
	if i.Timeseries != nil {
		if err := q.setDailyTimeseries(i.Timeseries.BucketTimeZone); err != nil {
			return nil, validationError(fn, err)
		}
	}
	q.addCommonFilters(i.AnalyticsCommonFilters)
	return q, nil
}

// Invoke implements infer.Fn for GetAnalyticsTopEvents.
func (f *GetAnalyticsTopEvents) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetAnalyticsTopEventsInput],
) (infer.FunctionResponse[GetAnalyticsTopEventsOutput], error) {
	var out GetAnalyticsTopEventsOutput
	q, err := req.Input.buildQuery()
	if err != nil {
		return infer.FunctionResponse[GetAnalyticsTopEventsOutput]{}, err
	}
	if err := invokeAnalyze(ctx, req.Input.SiteID, analyzeReportTopEvents, q, &out); err != nil {
		return infer.FunctionResponse[GetAnalyticsTopEventsOutput]{}, err
	}
	if out.Data == nil {
		out.Data = []AnalyticsTopEventRow{}
	}
	return infer.FunctionResponse[GetAnalyticsTopEventsOutput]{Output: out}, nil
}

// ---------------------------------------------------------------------------
// Time on page
// ---------------------------------------------------------------------------

// GetAnalyticsTimeOnPage returns the average time spent on a page.
// It calls GET /v2/sites/{site_id}/analyze/reports/time_on_page.
type GetAnalyticsTimeOnPage struct{}

// GetAnalyticsTimeOnPageInput defines the input parameters for GetAnalyticsTimeOnPage.
type GetAnalyticsTimeOnPageInput struct {
	// SiteID is the Webflow site ID.
	SiteID string `pulumi:"siteId"`
	// StartTime is the inclusive start of the reporting window.
	StartTime string `pulumi:"startTime"`
	// EndTime is the exclusive end of the reporting window.
	EndTime string `pulumi:"endTime"`
	// MetricScope is how the average is computed: "session", "user" or "pageview".
	MetricScope string `pulumi:"metricScope"`
	// Timeseries requests bucketed averages by day or week.
	Timeseries *AnalyticsTimeOnPageTimeseriesArgs `pulumi:"timeseries,optional"`
	AnalyticsCommonFilters
}

// AnalyticsAverageSecondsPoint is one data point of the time-on-page report.
type AnalyticsAverageSecondsPoint struct {
	// Timestamp is the bucket start (or window start for a single aggregate).
	Timestamp string `pulumi:"timestamp" json:"timestamp"`
	// AverageSeconds is the average time on page in seconds.
	AverageSeconds float64 `pulumi:"averageSeconds" json:"averageSeconds"`
}

// Annotate adds descriptions to the AnalyticsAverageSecondsPoint fields.
func (p *AnalyticsAverageSecondsPoint) Annotate(a infer.Annotator) {
	a.Describe(&p.Timestamp, "Start of the bucket as a UTC instant in ISO 8601 / RFC 3339 format.")
	a.Describe(&p.AverageSeconds, "Average time spent on the page in seconds.")
}

// GetAnalyticsTimeOnPageOutput defines the output of GetAnalyticsTimeOnPage.
type GetAnalyticsTimeOnPageOutput struct {
	// Report is the report name ("time_on_page").
	Report string `pulumi:"report" json:"report"`
	// Window is the reporting window applied.
	Window AnalyticsWindow `pulumi:"window" json:"window"`
	// MetricScope is how the average was computed.
	MetricScope string `pulumi:"metricScope" json:"metricScope"`
	// Data holds one aggregate point, or one point per bucket when a time series was requested.
	Data []AnalyticsAverageSecondsPoint `pulumi:"data" json:"data"`
	// Bucketing describes the bucketing when requested.
	Bucketing *AnalyticsBucketing `pulumi:"bucketing,optional" json:"bucketing,omitempty"`
	// Filters echoes the filters applied.
	Filters map[string]AnalyticsDimensionFilter `pulumi:"filters,optional" json:"filter,omitempty"`
}

// Annotate adds descriptions to the GetAnalyticsTimeOnPage function.
func (f *GetAnalyticsTimeOnPage) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the average time visitors spend on a page of a Webflow site, as a single value or "+
		"bucketed by day or week (Analyze API, beta). Use the 'pagePath' filter to target one page."+
		analyzeScopeNote)
}

// Annotate adds descriptions to the GetAnalyticsTimeOnPageInput fields.
func (i *GetAnalyticsTimeOnPageInput) Annotate(a infer.Annotator) {
	a.Describe(&i.SiteID, analyzeSiteIDDescription)
	a.Describe(&i.StartTime, analyzeStartTimeDescription)
	a.Describe(&i.EndTime, analyzeEndTimeDescription)
	a.Describe(&i.MetricScope, "How the average is computed: per 'session', per 'user' or per 'pageview'.")
	a.Describe(&i.Timeseries,
		"Include bucketed averages using the given granularity ('day' or 'week') and IANA time zone. "+
			"Omit to return a single aggregate value for the window.")
}

// Annotate adds descriptions to the GetAnalyticsTimeOnPageOutput fields.
func (o *GetAnalyticsTimeOnPageOutput) Annotate(a infer.Annotator) {
	a.Describe(&o.Report, "The report name, always 'time_on_page'.")
	a.Describe(&o.Window, analyzeWindowDescription)
	a.Describe(&o.MetricScope, "How the average was computed: 'session', 'user' or 'pageview'.")
	a.Describe(&o.Data, "A single aggregate data point, or one point per bucket when 'timeseries' was requested.")
	a.Describe(&o.Bucketing, "The bucketing applied; present only when 'timeseries' was requested.")
	a.Describe(&o.Filters, analyzeFiltersDescription)
}

// buildQuery validates the inputs and builds the query string.
func (i GetAnalyticsTimeOnPageInput) buildQuery() (*analyzeQuery, error) {
	const fn = "GetAnalyticsTimeOnPage"
	if err := validateAnalyzeCommon(fn, i.SiteID, i.StartTime, i.EndTime, i.AnalyticsCommonFilters); err != nil {
		return nil, err
	}
	if err := validateEnum("metricScope", i.MetricScope, analyzeMetricScopes, true); err != nil {
		return nil, validationError(fn, err)
	}

	q := newAnalyzeQuery()
	q.set("startTime", i.StartTime)
	q.set("endTime", i.EndTime)
	q.set("metricScope", i.MetricScope)
	if ts := i.Timeseries; ts != nil {
		err := validateEnum("timeseries.granularityPeriod", ts.GranularityPeriod, analyzeGranularityPeriods, true)
		if err != nil {
			return nil, validationError(fn, err)
		}
		if err := validateBucketTimeZone("timeseries.bucketTimeZone", ts.BucketTimeZone, true); err != nil {
			return nil, validationError(fn, err)
		}
		if err := q.setJSON("timeseries", map[string]string{
			"granularityPeriod": ts.GranularityPeriod,
			"bucketTimeZone":    ts.BucketTimeZone,
		}); err != nil {
			return nil, err
		}
	}
	q.addCommonFilters(i.AnalyticsCommonFilters)
	return q, nil
}

// Invoke implements infer.Fn for GetAnalyticsTimeOnPage.
func (f *GetAnalyticsTimeOnPage) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetAnalyticsTimeOnPageInput],
) (infer.FunctionResponse[GetAnalyticsTimeOnPageOutput], error) {
	var out GetAnalyticsTimeOnPageOutput
	q, err := req.Input.buildQuery()
	if err != nil {
		return infer.FunctionResponse[GetAnalyticsTimeOnPageOutput]{}, err
	}
	if err := invokeAnalyze(ctx, req.Input.SiteID, analyzeReportTimeOnPage, q, &out); err != nil {
		return infer.FunctionResponse[GetAnalyticsTimeOnPageOutput]{}, err
	}
	if out.Data == nil {
		out.Data = []AnalyticsAverageSecondsPoint{}
	}
	return infer.FunctionResponse[GetAnalyticsTimeOnPageOutput]{Output: out}, nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// validationError prefixes an input validation error with the function name.
func validationError(fn string, err error) error {
	return fmt.Errorf("validation failed for %s: %w", fn, err)
}

// validateAnalyzeCommon validates the inputs every report shares.
func validateAnalyzeCommon(fn, siteID, startTime, endTime string, filters AnalyticsCommonFilters) error {
	if err := ValidateSiteID(siteID); err != nil {
		return validationError(fn, err)
	}
	if err := ValidateAnalyzeWindow(startTime, endTime); err != nil {
		return validationError(fn, err)
	}
	if err := ValidateAnalyticsCommonFilters(filters); err != nil {
		return validationError(fn, err)
	}
	return nil
}

// setDailyTimeseries validates the time zone and adds the JSON 'timeseries' parameter used by the
// top-pages and top-events reports.
func (q *analyzeQuery) setDailyTimeseries(bucketTimeZone string) error {
	if err := validateBucketTimeZone("timeseries.bucketTimeZone", bucketTimeZone, true); err != nil {
		return err
	}
	return q.setJSON("timeseries", map[string]string{"bucketTimeZone": bucketTimeZone})
}

// invokeAnalyze obtains a client and fetches one report into out.
func invokeAnalyze(ctx context.Context, siteID, report string, q *analyzeQuery, out any) error {
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	if err := getAnalyzeReport(ctx, client, siteID, report, q, out); err != nil {
		return fmt.Errorf("failed to get %s report: %w", report, err)
	}
	return nil
}
