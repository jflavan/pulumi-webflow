# Analytics Functions Example

This example demonstrates the five analytics functions that read reports from Webflow's Analyze API:

| Function                    | What it returns                                                     |
|-----------------------------|---------------------------------------------------------------------|
| `getAnalyticsTraffic`       | A daily time series of sessions, users or pageviews                 |
| `getAnalyticsTopPages`      | The most visited pages, ranked by sessions, users or pageviews      |
| `getAnalyticsTopDimensions` | The top values of one dimension (country, device type, browser, ...)|
| `getAnalyticsTopEvents`     | The most fired events, optionally with a daily time series          |
| `getAnalyticsTimeOnPage`    | Average time on a page, as one value or bucketed by day/week        |

> **Beta API.** Webflow's Analyze endpoints are in beta and may change. The provider follows the
> current API; check the [Webflow changelog](https://developers.webflow.com/data/changelog) if
> requests start failing after a Webflow update.

## What This Example Does

- Reads the daily session count for the last 30 days (and the mobile-only subset)
- Lists the top 10 pages by pageviews and the top 5 countries by sessions
- Lists the top events with a per-event daily time series
- Reads the weekly average time visitors spend on the home page
- Exports the results as stack outputs and prints a short summary

The functions are read-only; nothing is created in Webflow.

## Prerequisites

- Pulumi CLI installed
- A Webflow API token with the **`sites:read`** scope, set as `WEBFLOW_API_TOKEN` or via
  `pulumi config set webflow:apiToken --secret`
- A Webflow workspace with the **Analyze add-on** (the API returns an error without it)
- A published site with some traffic

## Available Languages

| Language   | Directory    | Entry Point    | Dependencies        |
|------------|--------------|----------------|---------------------|
| TypeScript | `typescript/`| `index.ts`     | `package.json`      |

## Running the Example

### TypeScript

```bash
cd typescript
npm install
pulumi stack init dev
pulumi config set siteId your-site-id-here
# Optional: pick an explicit window (UTC timestamps ending in "Z")
pulumi config set startTime 2026-04-01T00:00:00Z
pulumi config set endTime   2026-05-01T00:00:00Z
pulumi up
```

Reporting windows must start on or after `2025-04-09T00:00:00Z` and span at most 100 days.
`startTime` is inclusive, `endTime` is exclusive, and numeric UTC offsets are not accepted.

## Key Features Demonstrated

### Reporting window and metric scope

```typescript
const traffic = webflow.getAnalyticsTrafficOutput({
  siteId,
  startTime: "2026-04-01T00:00:00Z",
  endTime: "2026-05-01T00:00:00Z",
  metricScope: "session",   // "session", "user" or "pageview"
  bucketTimeZone: "UTC",    // IANA zone that aligns the daily buckets
});
```

### Filters

Every function accepts the single-value filters `deviceType`, `country`, `pagePath`, `referrer`,
`trafficSource`, `browser`, `utmSource`, `utmMedium` and `utmCampaign`:

```typescript
webflow.getAnalyticsTrafficOutput({ ..., deviceType: "mobile" });
```

For `in` / `ne` / `nin` matching, or dimensions without a single-value input (`os`, `language`,
`locale`, `region`, `audienceIds`, `collectionId`, `itemSlug`, `pageId`, `dayOfWeek`, `timeOfDay`,
`visitStatus`, ...), use the `filters` map instead - but filter a given dimension in only one of
the two places:

```typescript
webflow.getAnalyticsTopPagesOutput({ ..., filters: { country: { in: ["US", "CA"] } } });
```

The top-events report does not support the `referrer` filter.

### Ranked reports

`getAnalyticsTopPages` and `getAnalyticsTopEvents` accept `limit` (default 25, max 250) and an
optional `timeseries: { bucketTimeZone }` that adds a daily series to every row.
`getAnalyticsTopDimensions` needs a `dimension` and a `metricScope` of `"session"` or `"user"`
(`limit` max 100).

### Time on page

`getAnalyticsTimeOnPage` returns one aggregate data point, or one per bucket when
`timeseries: { granularityPeriod: "day" | "week", bucketTimeZone }` is set. Use `pagePath` to
target a single page.

### Concurrency

Webflow allows one Analyze request in flight per access token. The provider retries when a request
is rejected for that reason, so a program can call several analytics functions at once.

## Configuration

| Config Key  | Required | Description                                                       |
|-------------|----------|-------------------------------------------------------------------|
| `siteId`    | Yes      | The Webflow site ID                                               |
| `startTime` | No       | Inclusive window start (UTC, `...Z`); defaults to 30 days ago     |
| `endTime`   | No       | Exclusive window end (UTC, `...Z`); defaults to today's midnight  |

## Outputs

- `reportingWindow`: the window Webflow applied
- `totalSessions`, `dailySessions`, `mobileSessions`: from `getAnalyticsTraffic`
- `topPagesByPageviews`: from `getAnalyticsTopPages`
- `topCountriesBySessions`: from `getAnalyticsTopDimensions`
- `topEventsByCount`: from `getAnalyticsTopEvents`
- `averageSecondsOnHomePage`: from `getAnalyticsTimeOnPage`

## Cleanup

The functions create nothing, so there is nothing to destroy:

```bash
pulumi stack rm dev
```

## Troubleshooting

- **"Analyze is not enabled" / 403**: the workspace needs the Analyze add-on and the token needs `sites:read`.
- **Invalid window**: check that both timestamps end in `Z`, `startTime >= 2025-04-09`, and the span is at most 100 days.
- **Empty `data`**: the site had no traffic in the window, or the filters excluded everything.

## Related Resources

- [Token Functions Example](../token/) - verify the token's scopes before reading reports
- [Main Examples Index](../README.md)
- [Webflow Analyze API](https://developers.webflow.com/data/reference/analyze)
