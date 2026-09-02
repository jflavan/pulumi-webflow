import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const siteId = config.require("siteId");

/**
 * Analytics Example - Reading Site Analytics with the Analyze API (beta)
 *
 * This example demonstrates the five analytics functions:
 * - getAnalyticsTraffic       daily sessions / users / pageviews
 * - getAnalyticsTopPages      most visited pages
 * - getAnalyticsTopDimensions top values of a dimension (country, device, ...)
 * - getAnalyticsTopEvents     most fired events
 * - getAnalyticsTimeOnPage    average time on a page
 *
 * Requirements:
 * - a token with the sites:read scope
 * - a Webflow workspace with the Analyze add-on
 * - a reporting window that starts on or after 2025-04-09 and spans at most
 *   100 days; timestamps are UTC and must end in "Z"
 *
 * Webflow allows one Analyze request in flight per access token; the provider
 * retries concurrent requests, so several functions can run in one program.
 */

// Reporting window: the last 30 full days ending at today's UTC midnight,
// unless startTime/endTime are configured explicitly.
const todayUtc = new Date();
todayUtc.setUTCHours(0, 0, 0, 0);
const thirtyDaysAgo = new Date(todayUtc.getTime() - 30 * 24 * 60 * 60 * 1000);

const startTime = config.get("startTime") || thirtyDaysAgo.toISOString();
const endTime = config.get("endTime") || todayUtc.toISOString();

// Example 1: Daily traffic time series (sessions per day, UTC buckets)
const traffic = webflow.getAnalyticsTrafficOutput({
  siteId: siteId,
  startTime: startTime,
  endTime: endTime,
  metricScope: "session", // "session", "user" or "pageview"
  bucketTimeZone: "UTC", // IANA zone used to align the daily buckets
});

// Example 2: Mobile-only traffic using one of the single-value filters
// (deviceType, country, pagePath, referrer, trafficSource, browser, utm*).
// For "in"/"ne"/"nin" style filters use the `filters` map instead.
const mobileTraffic = webflow.getAnalyticsTrafficOutput({
  siteId: siteId,
  startTime: startTime,
  endTime: endTime,
  metricScope: "session",
  bucketTimeZone: "UTC",
  deviceType: "mobile",
});

// Example 3: Top 10 pages ranked by pageviews
const topPages = webflow.getAnalyticsTopPagesOutput({
  siteId: siteId,
  startTime: startTime,
  endTime: endTime,
  sortBy: "pageview", // "session" (default), "user" or "pageview"
  limit: 10, // default 25, max 250
});

// Example 4: Top 5 countries by sessions
const topCountries = webflow.getAnalyticsTopDimensionsOutput({
  siteId: siteId,
  startTime: startTime,
  endTime: endTime,
  dimension: "country", // country, region, deviceType, os, browser, language, locale,
  //                       referrer, trafficSource, utmCampaign, utmContent, utmMedium,
  //                       utmSource, utmTerm or audienceIds
  metricScope: "session", // "session" or "user"
  limit: 5,
});

// Example 5: Top events with a daily time series per event
const topEvents = webflow.getAnalyticsTopEventsOutput({
  siteId: siteId,
  startTime: startTime,
  endTime: endTime,
  limit: 10,
  timeseries: { bucketTimeZone: "UTC" }, // omit to get ranked rows only
});

// Example 6: Average time on the home page, bucketed by week
const timeOnHomePage = webflow.getAnalyticsTimeOnPageOutput({
  siteId: siteId,
  startTime: startTime,
  endTime: endTime,
  metricScope: "session", // per "session", "user" or "pageview"
  pagePath: "/", // target a single page
  timeseries: { granularityPeriod: "week", bucketTimeZone: "UTC" }, // omit for one aggregate value
});

// Exports - traffic
export const reportingWindow = traffic.window;
export const totalSessions = traffic.data.apply((points) => points.reduce((sum, p) => sum + p.count, 0));
export const dailySessions = traffic.data.apply((points) =>
  points.map((p) => ({ day: p.timestamp.slice(0, 10), sessions: p.count }))
);
export const mobileSessions = mobileTraffic.data.apply((points) => points.reduce((sum, p) => sum + p.count, 0));

// Exports - top pages
export const topPagesByPageviews = topPages.data.apply((rows) =>
  rows.map((row) => ({
    title: row.title,
    pageId: row.pageId,
    sessions: row.sessionCount,
    users: row.userCount,
    pageviews: row.pageviewCount,
  }))
);

// Exports - top dimensions
export const topCountriesBySessions = topCountries.data.apply((rows) =>
  rows.map((row) => ({ country: row.name, code: row.attributeKey, sessions: row.count }))
);

// Exports - top events
export const topEventsByCount = topEvents.data.apply((rows) =>
  rows.map((row) => ({
    event: row.name ?? row.eventId,
    page: row.pageName,
    count: row.count,
    days: row.timeseries?.length ?? 0,
  }))
);

// Exports - time on page
export const averageSecondsOnHomePage = timeOnHomePage.data.apply((points) =>
  points.map((p) => ({ week: p.timestamp.slice(0, 10), averageSeconds: Math.round(p.averageSeconds) }))
);

// Print a short summary
pulumi
  .all([totalSessions, mobileSessions, topPagesByPageviews, topCountriesBySessions])
  .apply(([sessions, mobile, pages, countries]) => {
    console.log(`\nAnalytics for site ${siteId} (${startTime} - ${endTime})`);
    console.log(`  Sessions: ${sessions} (${mobile} on mobile)`);
    if (pages.length > 0) {
      console.log(`  Top page: "${pages[0].title}" with ${pages[0].pageviews} pageviews`);
    }
    if (countries.length > 0) {
      console.log(`  Top country: ${countries[0].country} with ${countries[0].sessions} sessions`);
    }
  });
