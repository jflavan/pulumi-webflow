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
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// This file contains the HTTP layer and shared types for the Webflow Analyze API:
// GET /v2/sites/{site_id}/analyze/reports/{report}. All reports require the 'sites:read'
// scope and a workspace with the Analyze add-on. Each access token may have only one Analyze
// request in flight at a time; additional concurrent requests receive 429 and are retried.
//
// Path prefix: the current reference (every code sample and the working-with-analyze guide)
// documents the reports under /v2/. The older v2.0.0-beta snapshot and the June 9 2026
// changelog still show /beta/. The provider therefore tries /v2/ first and, when that path
// answers 404, retries once at /beta/ so either deployment works during the transition.

// webflowV2PathPrefix is the URL prefix of the stable Webflow Data API v2 surface.
const webflowV2PathPrefix = "/v2"

// analyzePathPrefixes lists the prefixes tried for an Analyze report, in order.
var analyzePathPrefixes = []string{webflowV2PathPrefix, webflowBetaPathPrefix}

// analyzeEarliestStartTime is the earliest startTime the Analyze API accepts.
var analyzeEarliestStartTime = time.Date(2025, time.April, 9, 0, 0, 0, 0, time.UTC)

// analyzeMaxWindow is the maximum reporting window the Analyze API accepts.
const analyzeMaxWindow = 100 * 24 * time.Hour

// Analyze report names, used as the final path segment.
const (
	analyzeReportTraffic       = "traffic"
	analyzeReportTopPages      = "top_pages"
	analyzeReportTopDimensions = "top_dimensions"
	analyzeReportTopEvents     = "top_events"
	analyzeReportTimeOnPage    = "time_on_page"
)

// Allowed enum values documented by the Analyze API.
var (
	analyzeMetricScopes       = []string{"session", "user", "pageview"}
	analyzeTopDimensionScopes = []string{"session", "user"}
	analyzeDeviceTypes        = []string{"desktop", "mobile", "tablet"}
	analyzeGranularityPeriods = []string{"day", "week"}
	analyzeTopDimensionValues = []string{
		"country", "region", "deviceType", "os", "browser", "language", "locale", "referrer",
		"trafficSource", "utmCampaign", "utmContent", "utmMedium", "utmSource", "utmTerm", "audienceIds",
	}
	analyzeFilterDimensionValues = []string{
		"audienceIds", "browser", "collectionId", "country", "dayOfWeek", "deviceBrand", "deviceType",
		"domain", "itemSlug", "language", "locale", "nextCollectionId", "nextItemSlug", "nextPageId", "os",
		"pageId", "pagePath", "previousCollectionId", "previousItemSlug", "previousPageId", "referrer",
		"region", "timeOfDay", "timezone", "trafficSource", "utmCampaign", "utmContent", "utmMedium",
		"utmSource", "utmTerm", "visitStatus",
	}
	// analyzeTopEventsFilterDimensionValues is the narrower filter schema of the top_events
	// report: it has no referrer and no next*/previous* navigation dimensions.
	analyzeTopEventsFilterDimensionValues = []string{
		"audienceIds", "browser", "collectionId", "country", "dayOfWeek", "deviceBrand", "deviceType",
		"domain", "itemSlug", "language", "locale", "os", "pageId", "pagePath", "region", "timeOfDay",
		"timezone", "trafficSource", "utmCampaign", "utmContent", "utmMedium", "utmSource", "utmTerm",
		"visitStatus",
	}
)

// Limits documented per report.
const (
	analyzeTopPagesMaxLimit      = 250
	analyzeTopEventsMaxLimit     = 250
	analyzeTopDimensionsMaxLimit = 100
)

// AnalyticsDimensionFilter holds the operators applied to one dimension in the advanced
// 'filter' query parameter. At least one operator must be set.
// Encoded as filter[dimension][eq]=v, filter[dimension][in][0]=v1&filter[dimension][in][1]=v2, ...
type AnalyticsDimensionFilter struct {
	// Eq keeps rows whose dimension equals this value.
	Eq string `pulumi:"eq,optional" json:"eq,omitempty"`
	// In keeps rows whose dimension is any of these values.
	In []string `pulumi:"in,optional" json:"in,omitempty"`
	// Ne drops rows whose dimension equals this value.
	Ne string `pulumi:"ne,optional" json:"ne,omitempty"`
	// Nin drops rows whose dimension is any of these values.
	Nin []string `pulumi:"nin,optional" json:"nin,omitempty"`
}

// Annotate adds descriptions to the AnalyticsDimensionFilter fields.
func (f *AnalyticsDimensionFilter) Annotate(a infer.Annotator) {
	a.Describe(&f.Eq, "Keep only rows whose dimension value equals this value.")
	a.Describe(&f.In, "Keep only rows whose dimension value is one of these values.")
	a.Describe(&f.Ne, "Exclude rows whose dimension value equals this value.")
	a.Describe(&f.Nin, "Exclude rows whose dimension value is one of these values.")
}

// isEmpty reports whether no operator is set.
func (f AnalyticsDimensionFilter) isEmpty() bool {
	return f.Eq == "" && len(f.In) == 0 && f.Ne == "" && len(f.Nin) == 0
}

// AnalyticsWindow is the reporting window echoed by every Analyze response.
type AnalyticsWindow struct {
	// StartTime is the inclusive start of the reporting window (ISO 8601).
	StartTime string `pulumi:"startTime" json:"startTime"`
	// EndTime is the exclusive end of the reporting window (ISO 8601).
	EndTime string `pulumi:"endTime" json:"endTime"`
}

// Annotate adds descriptions to the AnalyticsWindow fields.
func (w *AnalyticsWindow) Annotate(a infer.Annotator) {
	a.Describe(&w.StartTime, "Inclusive start of the reporting window, in ISO 8601 / RFC 3339 format.")
	a.Describe(&w.EndTime, "Exclusive end of the reporting window, in ISO 8601 / RFC 3339 format.")
}

// AnalyticsBucketing describes how time-series data points were bucketed.
type AnalyticsBucketing struct {
	// GranularityPeriod is the bucket size ("day" or "week").
	GranularityPeriod string `pulumi:"granularityPeriod" json:"granularityPeriod"`
	// BucketTimeZone is the IANA time zone used to align bucket boundaries.
	BucketTimeZone string `pulumi:"bucketTimeZone" json:"bucketTimeZone"`
}

// Annotate adds descriptions to the AnalyticsBucketing fields.
func (b *AnalyticsBucketing) Annotate(a infer.Annotator) {
	a.Describe(&b.GranularityPeriod, "The size of each time bucket: 'day' or 'week'.")
	a.Describe(&b.BucketTimeZone, "The IANA time zone used to align bucket boundaries (e.g., 'UTC', 'America/New_York').")
}

// AnalyticsCountPoint is one time-series data point with an integer count.
type AnalyticsCountPoint struct {
	// Timestamp is the bucket start as a UTC instant (ISO 8601).
	Timestamp string `pulumi:"timestamp" json:"timestamp"`
	// Count is the metric value for the bucket.
	Count int `pulumi:"count" json:"count"`
}

// Annotate adds descriptions to the AnalyticsCountPoint fields.
func (c *AnalyticsCountPoint) Annotate(a infer.Annotator) {
	a.Describe(&c.Timestamp, "Start of the time bucket as a UTC instant in ISO 8601 / RFC 3339 format.")
	a.Describe(&c.Count, "Number of sessions, users, pageviews or events in the bucket, per the report's metric scope.")
}

// AnalyticsCommonFilters holds the single-value filter query parameters shared by every report.
type AnalyticsCommonFilters struct {
	// DeviceType restricts the report to a single device type.
	DeviceType string `pulumi:"deviceType,optional"`
	// Country restricts the report to a single ISO 3166-1 alpha-2 country code.
	Country string `pulumi:"country,optional"`
	// PagePath restricts the report to a single page path.
	PagePath string `pulumi:"pagePath,optional"`
	// TrafficSource restricts the report to a single traffic source code.
	TrafficSource string `pulumi:"trafficSource,optional"`
	// Referrer restricts the report to a single referrer domain.
	Referrer string `pulumi:"referrer,optional"`
	// Browser restricts the report to a single browser.
	Browser string `pulumi:"browser,optional"`
	// UtmCampaign restricts the report to a single utm_campaign value.
	UtmCampaign string `pulumi:"utmCampaign,optional"`
	// UtmMedium restricts the report to a single utm_medium value.
	UtmMedium string `pulumi:"utmMedium,optional"`
	// UtmSource restricts the report to a single utm_source value.
	UtmSource string `pulumi:"utmSource,optional"`
	// Filters is the advanced per-dimension filter object.
	Filters map[string]AnalyticsDimensionFilter `pulumi:"filters,optional"`
}

// Annotate adds descriptions to the AnalyticsCommonFilters fields.
func (c *AnalyticsCommonFilters) Annotate(a infer.Annotator) {
	a.Describe(&c.DeviceType, "Restrict the report to a single device type: 'desktop', 'mobile' or 'tablet'.")
	a.Describe(&c.Country,
		"Restrict the report to a single country, as an ISO 3166-1 alpha-2 code (e.g., 'US'). Normalized to uppercase.")
	a.Describe(&c.PagePath, "Restrict the report to a single page path (e.g., '/pricing').")
	a.Describe(&c.TrafficSource,
		"Restrict the report to a single traffic source code (e.g., 'SO' for Organic Search).")
	a.Describe(&c.Referrer, "Restrict the report to a single referrer domain (e.g., 'google.com').")
	a.Describe(&c.Browser, "Restrict the report to a single browser name (e.g., 'Chrome').")
	a.Describe(&c.UtmCampaign, "Restrict the report to a single utm_campaign value.")
	a.Describe(&c.UtmMedium, "Restrict the report to a single utm_medium value.")
	a.Describe(&c.UtmSource, "Restrict the report to a single utm_source value.")
	a.Describe(&c.Filters,
		"Advanced filters keyed by dimension (e.g., 'country', 'deviceType', 'pagePath', 'referrer', "+
			"'trafficSource', 'utmCampaign', 'os', 'language', 'locale', 'region', 'audienceIds', "+
			"'collectionId', 'itemSlug', 'pageId', 'dayOfWeek', 'timeOfDay', 'visitStatus', ...). "+
			"Each entry supports 'eq', 'in', 'ne' and 'nin'. Filter a given dimension either here or with "+
			"the matching single-value input, not both.")
}

// analyzeQuery accumulates query parameters for an Analyze request.
type analyzeQuery struct {
	values url.Values
}

func newAnalyzeQuery() *analyzeQuery {
	return &analyzeQuery{values: url.Values{}}
}

// set adds a parameter when the value is non-empty.
func (q *analyzeQuery) set(key, value string) {
	if value != "" {
		q.values.Set(key, value)
	}
}

// setInt adds an integer parameter when it is non-zero.
func (q *analyzeQuery) setInt(key string, value int) {
	if value != 0 {
		q.values.Set(key, strconv.Itoa(value))
	}
}

// setJSON adds a JSON-encoded object parameter. It is used for 'timeseries', which the
// reference samples send as a JSON-encoded object in the query string, exactly like
// timeseries={"bucketTimeZone":"UTC"} (URL-encoded on the wire); the API does not accept
// the bracketed timeseries[bucketTimeZone]=UTC form used by 'filter'.
func (q *analyzeQuery) setJSON(key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode %s parameter: %w", key, err)
	}
	q.values.Set(key, string(encoded))
	return nil
}

// addCommonFilters adds the shared single-value filters and the advanced filter object.
func (q *analyzeQuery) addCommonFilters(f AnalyticsCommonFilters) {
	q.set("deviceType", f.DeviceType)
	q.set("country", f.Country)
	q.set("pagePath", f.PagePath)
	q.set("trafficSource", f.TrafficSource)
	q.set("referrer", f.Referrer)
	q.set("browser", f.Browser)
	q.set("utmCampaign", f.UtmCampaign)
	q.set("utmMedium", f.UtmMedium)
	q.set("utmSource", f.UtmSource)

	dims := make([]string, 0, len(f.Filters))
	for dim := range f.Filters {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	for _, dim := range dims {
		df := f.Filters[dim]
		q.set(fmt.Sprintf("filter[%s][eq]", dim), df.Eq)
		q.set(fmt.Sprintf("filter[%s][ne]", dim), df.Ne)
		for i, v := range df.In {
			q.values.Set(fmt.Sprintf("filter[%s][in][%d]", dim, i), v)
		}
		for i, v := range df.Nin {
			q.values.Set(fmt.Sprintf("filter[%s][nin][%d]", dim, i), v)
		}
	}
}

// encode returns the sorted, URL-encoded query string.
func (q *analyzeQuery) encode() string {
	return q.values.Encode()
}

// analyzeReportURL builds the URL of an Analyze report for a site under the given path prefix
// ("/v2" or "/beta").
func analyzeReportURL(prefix, siteID, report string, q *analyzeQuery) string {
	u := apiURL(prefix+"/sites/%s/analyze/reports/%s", siteID, report)
	if encoded := q.encode(); encoded != "" {
		u += "?" + encoded
	}
	return u
}

// getAnalyzeReport performs GET on an Analyze report and decodes the response into out.
// The /v2 path is tried first; a 404 there is retried once at the transitional /beta path.
// Only when both answer 404 is the not-found error returned.
func getAnalyzeReport(ctx context.Context, client *http.Client, siteID, report string, q *analyzeQuery, out any) error {
	var err error
	for i, prefix := range analyzePathPrefixes {
		_, err = doRequest(ctx, client, http.MethodGet, analyzeReportURL(prefix, siteID, report, q), nil, out,
			http.StatusOK)
		if err == nil || !IsNotFound(err) || i == len(analyzePathPrefixes)-1 {
			return err
		}
		NewLogContext(ctx).
			WithField("siteId", siteID).
			WithField("report", report).
			WithField("prefix", prefix).
			WithField("fallback", analyzePathPrefixes[i+1]).
			Debug("Analyze report path answered 404; retrying at the transitional path prefix")
	}
	return err
}

// ValidateAnalyzeWindow validates the startTime/endTime pair shared by all reports.
// Both must be UTC timestamps ending in 'Z'; endTime must be after startTime and within 100 days.
func ValidateAnalyzeWindow(startTime, endTime string) error {
	start, err := parseAnalyzeTimestamp("startTime", startTime)
	if err != nil {
		return err
	}
	end, err := parseAnalyzeTimestamp("endTime", endTime)
	if err != nil {
		return err
	}
	if start.Before(analyzeEarliestStartTime) {
		return fmt.Errorf("startTime '%s' is before the earliest supported date %s. "+
			"Analyze data is only available from 2025-04-09 onwards",
			startTime, analyzeEarliestStartTime.Format(time.RFC3339))
	}
	if !end.After(start) {
		return fmt.Errorf("endTime '%s' must be after startTime '%s'. "+
			"endTime is exclusive, so a one-day window is e.g. startTime=2026-04-01T00:00:00Z, "+
			"endTime=2026-04-02T00:00:00Z", endTime, startTime)
	}
	if end.Sub(start) > analyzeMaxWindow {
		return fmt.Errorf("the reporting window from '%s' to '%s' is longer than the 100-day maximum. "+
			"Split the request into windows of at most 100 days", startTime, endTime)
	}
	return nil
}

// parseAnalyzeTimestamp parses a UTC ISO 8601 timestamp that must end in 'Z'.
func parseAnalyzeTimestamp(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("%s is required but was not provided. "+
			"Provide a UTC timestamp in ISO 8601 / RFC 3339 format ending in 'Z' (e.g., '2026-04-01T00:00:00Z')", name)
	}
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, fmt.Errorf("%s '%s' must be a UTC timestamp ending in 'Z' (e.g., '2026-04-01T00:00:00Z'). "+
			"Numeric offsets such as '+00:00' or '-04:00' are not accepted by the Analyze API", name, value)
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s '%s' is not a valid ISO 8601 / RFC 3339 timestamp "+
			"(e.g., '2026-04-01T00:00:00Z'): %w", name, value, err)
	}
	return t, nil
}

// validateEnum checks that value is one of allowed (or empty when optional).
func validateEnum(name, value string, allowed []string, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s is required but was not provided. Valid values: %s",
				name, strings.Join(allowed, ", "))
		}
		return nil
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s '%s' is not valid. Valid values: %s", name, value, strings.Join(allowed, ", "))
}

// validateLimit checks an optional row limit against the report's maximum. Zero means the
// input was omitted: the parameter is not sent and Webflow applies its default of 25.
func validateLimit(limit, maxLimit int) error {
	if limit < 0 || limit > maxLimit {
		return fmt.Errorf("limit %d is out of range. Provide a value between 1 and %d, or omit it "+
			"(leave it at 0) so the parameter is not sent and Webflow applies its default of 25", limit, maxLimit)
	}
	return nil
}

// unsupportedFilterDimension returns the first (alphabetically) advanced filter dimension that
// is not in allowed, or "" when every dimension is allowed.
func unsupportedFilterDimension(filters map[string]AnalyticsDimensionFilter, allowed []string) string {
	dims := make([]string, 0, len(filters))
	for dim := range filters {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	for _, dim := range dims {
		if validateEnum("filters dimension", dim, allowed, true) != nil {
			return dim
		}
	}
	return ""
}

// validateBucketTimeZone checks that a time zone is a valid IANA name.
func validateBucketTimeZone(name, tz string, required bool) error {
	if tz == "" {
		if required {
			return fmt.Errorf("%s is required but was not provided. "+
				"Provide a canonical IANA time zone such as 'UTC' or 'America/New_York'", name)
		}
		return nil
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("%s '%s' is not a valid IANA time zone. "+
			"Use a canonical name such as 'UTC' or 'America/New_York'", name, tz)
	}
	return nil
}

// ValidateAnalyticsCommonFilters validates the shared filter inputs.
func ValidateAnalyticsCommonFilters(f AnalyticsCommonFilters) error {
	if err := validateEnum("deviceType", f.DeviceType, analyzeDeviceTypes, false); err != nil {
		return err
	}
	if f.Country != "" && len(f.Country) != 2 {
		return fmt.Errorf("country '%s' must be an ISO 3166-1 alpha-2 code (two letters, e.g., 'US')", f.Country)
	}
	if dim := unsupportedFilterDimension(f.Filters, analyzeFilterDimensionValues); dim != "" {
		return validateEnum("filters dimension", dim, analyzeFilterDimensionValues, true)
	}
	for dim, df := range f.Filters {
		if df.isEmpty() {
			return fmt.Errorf("filters['%s'] must specify at least one of 'eq', 'in', 'ne' or 'nin'", dim)
		}
	}
	// The API rejects a dimension filtered both at the top level and in the filter object.
	single := map[string]string{
		"deviceType": f.DeviceType, "country": f.Country, "pagePath": f.PagePath, "trafficSource": f.TrafficSource,
		"referrer": f.Referrer, "browser": f.Browser, "utmCampaign": f.UtmCampaign, "utmMedium": f.UtmMedium,
		"utmSource": f.UtmSource,
	}
	for dim, v := range single {
		if _, ok := f.Filters[dim]; ok && v != "" {
			return fmt.Errorf("dimension '%s' is filtered both by the '%s' input and by filters['%s']. "+
				"Filter a dimension in one place only", dim, dim, dim)
		}
	}
	return nil
}
