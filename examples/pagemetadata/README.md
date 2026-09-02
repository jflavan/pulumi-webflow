# PageMetadata Example

This example demonstrates how to use the `webflow.PageMetadata` resource together with the
`webflow.getPages` and `webflow.getPage` functions to manage the settings of a page that was
created in the Webflow Designer.

## What This Example Does

1. Lists every page of a site with `getPages` and finds the one whose slug matches the
   `pageSlug` config value
2. Manages that page's title, SEO title/description and Open Graph settings with `PageMetadata`
3. Reads the page back with `getPage` after the update and exports what Webflow reports

Pages cannot be created through the Webflow API. `PageMetadata` adopts an existing page: only the
fields you set are sent (`PUT /v2/pages/{page_id}`), everything else keeps the value managed in the
Designer, and destroying the resource only removes it from Pulumi state.

## Prerequisites

- Pulumi CLI installed
- A Webflow API token with the **`pages:read`** and **`pages:write`** scopes, set as
  `WEBFLOW_API_TOKEN` or via `pulumi config set webflow:apiToken --secret`
  (`getPages` and `getPage` need only `pages:read`)
- A Webflow site with at least one static page (the example defaults to the slug `about`)

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
pulumi config set pageSlug about          # optional, defaults to "about"
pulumi config set localeId your-locale-id # optional, secondary locale only
pulumi up
```

If no page has the configured slug, the program fails with the list of available slugs.

## Key Features Demonstrated

### 1. Finding a page with `getPages`

```typescript
const allPages = webflow.getPagesOutput({ siteId });
const targetPage = allPages.pages.apply((pages) => pages.find((p) => p.slug === "about")!);
```

`getPages` follows API pagination and returns every page with its `pageId`, `title`, `slug`,
`publishedPath`, `seo`, `openGraph`, timestamps and `draft`/`archived` flags.

### 2. Managing settings with `PageMetadata`

```typescript
const pageSettings = new webflow.PageMetadata("about-page-settings", {
  pageId: targetPage.pageId,
  title: "About Us",
  seo: { title: "About Us | Example Co", description: "..." },
  openGraph: { title: "About Example Co", description: "...", titleCopied: false, descriptionCopied: false },
});
```

- `slug` can also be set, but Webflow silently ignores it for the home page, collection template
  pages, utility pages (404, password, search) and for secondary locales without an
  Advanced/Enterprise localization plan. The provider warns when the returned slug differs; the
  read-only `currentSlug` output shows what Webflow actually uses.
- `localeId` targets a secondary locale; changing it (or `pageId`) replaces the resource.
- Set `titleCopied` / `descriptionCopied` to `true` to reuse the SEO values for Open Graph.

### 3. Reading a single page with `getPage`

```typescript
const updatedPage = webflow.getPageOutput({ pageId: pageSettings.pageId }, { dependsOn: [pageSettings] });
export const seoTitle = updatedPage.seo.title;
```

`getPage` also accepts `localeId` and `translatable: true` (returns the secondary locale's own
translation instead of content inherited from the primary locale).

## Configuration

| Config Key  | Required | Description                                             |
|-------------|----------|---------------------------------------------------------|
| `siteId`    | Yes      | The Webflow site ID                                     |
| `pageSlug`  | No       | Slug of the page to manage (default `about`)            |
| `localeId`  | No       | Secondary locale ID (omit for the primary locale)       |

## Outputs

- `pageCount`, `pageSlugs`: from the `getPages` listing
- `managedPageId`, `managedTitle`, `managedCurrentSlug`, `managedPublishedPath`, `managedLastUpdated`:
  from the `PageMetadata` resource
- `readSeoTitle`, `readSeoDescription`, `readOpenGraphTitle`, `readIsDraft`: read back with `getPage`

## Cleanup

```bash
pulumi destroy   # removes the resource from state; the page keeps its settings
pulumi stack rm dev
```

## Related Resources

- [Page Functions Example](../page/) - `getPages` / `getPage` in TypeScript, Python and Go
- [PageContent Example](../pagecontent/) - text inside the page's DOM nodes
- [PageSchemaMarkup Example](../pageschemamarkup/) - JSON-LD structured data
- [Main Examples Index](../README.md)
- [Webflow Pages API](https://developers.webflow.com/data/reference/pages-and-components/pages)
