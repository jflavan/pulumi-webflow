# GoogleTag Resource Example

This example demonstrates how to use the `webflow.GoogleTag` resource to manage the Google Tag IDs
(Google Analytics 4, Google Tag, Google Ads, Campaign Manager) configured on a Webflow site through
the Google Tag Manager integration API.

## What This Example Does

- Adds a GA4 measurement ID to a site with only the required inputs (`siteId`, `tagId`, `displayName`)
- Adds a Google Ads conversion tag with an explicit display position (`order`)
- Creates several tags from a list
- Exports the tag IDs and the positions Webflow reports for them (`effectiveOrder`)

Each `GoogleTag` resource manages exactly one entry in the site's tag list, so several resources
can target the same site. Tags that were added in the Webflow dashboard and are not managed by
Pulumi are left untouched. Webflow allows up to 25 tags per site and rejects legacy Universal
Analytics `UA-` IDs.

## Prerequisites

- Pulumi CLI installed
- A Webflow API token with the **`sites:read`** and **`sites:write`** scopes, set as
  `WEBFLOW_API_TOKEN` or via `pulumi config set webflow:apiToken --secret`
- The ID of a Webflow site (24-character hexadecimal string)
- Optionally a real GA4 measurement ID from your Google Analytics property
  (Admin -> Data Streams -> your stream -> Measurement ID)

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
pulumi config set ga4TagId G-XXXXXXXXXX   # optional, defaults to a placeholder
pulumi up
```

## Key Features Demonstrated

### 1. Minimal tag

```typescript
const analyticsTag = new webflow.GoogleTag("primary-analytics", {
  siteId: siteId,
  tagId: "G-1A2B3C4D5E",
  displayName: "Primary Google Analytics",
});
```

### 2. Explicit ordering

```typescript
const adsTag = new webflow.GoogleTag("google-ads", {
  siteId: siteId,
  tagId: "AW-123456789",
  displayName: "Google Ads conversions",
  order: 2, // optional; Webflow assigns a position when omitted
});
```

`order` is optional. Webflow renormalizes positions after a tag is deleted, so the read-only
`effectiveOrder` output may differ from the `order` you requested.

### 3. Immutable identity

`siteId` and `tagId` identify the tag; changing either replaces the resource. `displayName` and
`order` are updated in place.

## Configuration

| Config Key  | Required | Description                                             |
|-------------|----------|---------------------------------------------------------|
| `siteId`    | Yes      | The Webflow site ID                                     |
| `ga4TagId`  | No       | GA4 measurement ID to configure (default `G-1A2B3C4D5E`) |

## Outputs

- `analyticsTagId`, `analyticsDisplayName`, `analyticsEffectiveOrder`: the GA4 tag and its position
- `adsTagId`, `adsEffectiveOrder`: the Google Ads tag and its position
- `extraTagIds`: the tag IDs created from the list

## Cleanup

```bash
pulumi destroy   # removes only the tags managed by this stack
pulumi stack rm dev
```

## Troubleshooting

- **"UA-" IDs are rejected**: Universal Analytics has been retired; use a GA4 `G-` measurement ID.
- **Too many tags**: Webflow allows at most 25 tag IDs per site.
- **403 errors**: make sure the token has both `sites:read` and `sites:write`.

## Related Resources

- [SiteCustomCode Example](../sitecustomcode/) - for scripts that are not Google tags
- [Main Examples Index](../README.md)
- [Webflow Google Tag Manager API](https://developers.webflow.com/data/reference/sites/google-tag-manager)
