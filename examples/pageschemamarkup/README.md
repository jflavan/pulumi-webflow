# PageSchemaMarkup Example

This example demonstrates how to use the `webflow.PageSchemaMarkup` resource and the
`webflow.getPageSchemaMarkup` function to manage the JSON-LD structured data of a Webflow page.

> **Beta API.** Webflow's page schema markup endpoints are in beta and may change. The provider
> follows the current API; check the [Webflow changelog](https://developers.webflow.com/data/changelog)
> if requests start failing after a Webflow update.

## What This Example Does

- Publishes an `FAQPage` JSON-LD document on a page, built from a plain object with `JSON.stringify`
- Shows how to target a secondary locale with `localeId`
- Reads the markup back with `getPageSchemaMarkup` and parses it
- Exports the read-only outputs (`publishedPath`, `lastUpdated`, `isInherited`, `effectiveLocaleId`)

The markup must be a single JSON object; use `"@graph"` to publish several entities on one page.
The provider compares markup semantically, so key order and whitespace never cause a diff.
Deleting the resource clears the markup, because Webflow has no delete endpoint for it.

## Prerequisites

- Pulumi CLI installed
- A Webflow API token with the **`pages:read`** and **`pages:write`** scopes, set as
  `WEBFLOW_API_TOKEN` or via `pulumi config set webflow:apiToken --secret`
  (the `getPageSchemaMarkup` function needs only `pages:read`)
- The ID of a page in your site (see the [page example](../page/) to list page IDs)

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
pulumi config set pageId your-page-id-here
# Optional: manage a secondary locale instead of the primary one
pulumi config set localeId your-locale-id-here
pulumi up
```

## Key Features Demonstrated

### 1. Writing markup from an object

```typescript
const faqSchema = new webflow.PageSchemaMarkup("faq-schema", {
  pageId: pageId,
  schemaMarkup: JSON.stringify({
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: [/* ... */],
  }),
});
```

### 2. Reading markup back

```typescript
const current = webflow.getPageSchemaMarkupOutput(
  { pageId: pageId },
  { dependsOn: [faqSchema] } // read after the resource is written
);
const parsed = current.schemaMarkup.apply((json) => (json ? JSON.parse(json) : undefined));
```

`schemaMarkup` is returned as a compact JSON string with sorted keys (empty when the page has no
markup). `rawSchemaMarkup` is only populated for legacy multi-block markup that cannot be
represented as one JSON object.

### 3. Locales

Omit `localeId` to manage the primary locale. When a secondary locale has no markup of its own,
Webflow serves the primary locale's markup and reports `isInherited: true`; `effectiveLocaleId`
tells you which locale's markup is in effect.

## Limits

Webflow limits each markup entry to 60KB, 32 levels of nesting and 5,000 nodes, and silently strips
the keys `__proto__`, `constructor` and `prototype`.

## Configuration

| Config Key  | Required | Description                                            |
|-------------|----------|--------------------------------------------------------|
| `pageId`    | Yes      | The page whose schema markup is managed                |
| `localeId`  | No       | Secondary locale ID (omit for the primary locale)      |

## Outputs

- `schemaPageId`, `schemaSiteId`, `schemaPublishedPath`, `schemaLastUpdated`: resource outputs
- `schemaIsInherited`, `schemaEffectiveLocaleId`: locale fallback information
- `readMarkupType`, `readQuestionCount`, `readIsInherited`: values read back with `getPageSchemaMarkup`

## Cleanup

```bash
pulumi destroy   # clears the markup on the page
pulumi stack rm dev
```

## Related Resources

- [PageMetadata Example](../pagemetadata/) - titles, slugs, SEO and Open Graph settings
- [Page Functions Example](../page/) - find page IDs with `getPages`
- [Main Examples Index](../README.md)
- [Webflow Page Schema Markup API](https://developers.webflow.com/data/reference/pages-and-components/pages)
