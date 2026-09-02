# Page Functions Examples

This directory contains examples demonstrating how to read page information from Webflow sites
with the `getPages` and `getPage` functions using Pulumi in TypeScript, Python and Go.

## What You'll Learn

- List all pages of a Webflow site with `getPages`
- Read the metadata of a single page with `getPage`
- Access page properties (title, slug, published path, SEO, Open Graph, timestamps, flags)
- Filter pages by their properties and use page IDs in other resources

## Important Note

**Pages cannot be created through the Webflow API.** They are built in the Webflow Designer. The
functions in this example read existing pages; to change a page's title, slug, SEO or Open Graph
settings, use the [`PageMetadata` resource](../pagemetadata/).

> The `PageData` resource that earlier versions used for this was removed. Replace
> `new webflow.PageData(...)` with `webflow.getPagesOutput({ siteId })` (all pages) or
> `webflow.getPageOutput({ pageId })` (one page) and run `pulumi state delete <URN>` for any
> `PageData` resources still in state.

## Available Languages

| Language   | Directory    | Entry Point    | Dependencies        |
|------------|--------------|----------------|---------------------|
| TypeScript | `typescript/`| `index.ts`     | `package.json`      |
| Python     | `python/`    | `__main__.py`  | `requirements.txt`  |
| Go         | `go/`        | `main.go`      | `go.mod`            |

## Prerequisites

- Pulumi CLI installed
- A Webflow API token with the **`pages:read`** scope, set as `WEBFLOW_API_TOKEN` or via
  `pulumi config set webflow:apiToken --secret`
- Your Webflow site ID

## Quick Start

### TypeScript

```bash
cd typescript
npm install
pulumi stack init dev
pulumi config set siteId your-site-id --secret

# List all pages in the site
pulumi up

# Also read a specific page by ID
pulumi config set pageId your-page-id
pulumi up
```

### Python

```bash
cd python
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
pulumi stack init dev
pulumi config set siteId your-site-id --secret

# List all pages in the site
pulumi up

# Also read a specific page by ID
pulumi config set pageId your-page-id
pulumi up
```

### Go

```bash
cd go
go mod download
pulumi stack init dev
pulumi config set siteId your-site-id --secret

# List all pages in the site
pulumi up

# Also read a specific page by ID
pulumi config set pageId your-page-id
pulumi up
```

> The Go example's `go.mod` contains a `replace` directive pointing at the SDK in this repository
> (`../../../sdk/go/webflow`) because the page functions are newer than the published `v0.10.1`
> Go module. Once the next release is published you can drop the directive and depend on that
> version instead.

## Use Cases

### 1. List All Pages

Query all pages in a site to understand your site structure, generate documentation, or find
page IDs for other resources.

```typescript
const allPages = webflow.getPagesOutput({ siteId: siteId });

export const pageIds = allPages.pages.apply((pages) => pages.map((p) => p.pageId));
```

### 2. Get a Specific Page

Retrieve detailed metadata for a single page when you know its ID.

```typescript
const homePage = webflow.getPageOutput({ pageId: "5f0c8c9e1c9d440000e8d8c4" });

export const homePageTitle = homePage.title;
export const homePageSeoTitle = homePage.seo.title;
```

Both functions accept an optional `localeId`; `getPage` also accepts `translatable: true` to return
a secondary locale's own translation instead of content inherited from the primary locale.

### 3. Reference Pages in Other Resources

Use page IDs when configuring page settings, custom code or content.

```typescript
const aboutPage = allPages.pages.apply((pages) => pages.find((p) => p.slug === "about")!);

const aboutSettings = new webflow.PageMetadata("about-settings", {
  pageId: aboutPage.pageId,
  seo: { title: "About Us | Example Co" },
});

const aboutScripts = new webflow.PageCustomCode("about-scripts", {
  pageId: aboutPage.pageId,
  scripts: [/* ... */],
});
```

### 4. Filter Pages by Properties

```typescript
export const draftPages = allPages.pages.apply((pages) => pages.filter((page) => page.draft));
export const collectionTemplates = allPages.pages.apply((pages) =>
  pages.filter((page) => page.collectionId !== "")
);
```

## Configuration

Each example reads the following configuration:

| Config Key  | Required | Description                                             |
|-------------|----------|---------------------------------------------------------|
| `siteId`    | Yes      | Your Webflow site ID (stored as secret)                 |
| `pageId`    | No       | A page ID to read with `getPage` in addition to listing |

## Getting Your Site and Page IDs

1. **Site ID**:
   - Log in to Webflow
   - Go to Site Settings -> General
   - Find your site ID (24-character hexadecimal string)

2. **Page ID**:
   - Run the example without `pageId` configured; the `sitePages` output lists every page with its ID
   - Or use the Webflow API to list pages

## Expected Output

### Listing all pages (pageId not set)

```
Outputs:
    collectionTemplateSlugs : ["detail_post"]
    draftPageSlugs          : ["coming-soon"]
    pageCount               : 12
    pageIds                 : ["5f0c...", "5f0d...", "5f0e...", ...]
    sitePages               : [
        {
            id            : "5f0c8c9e1c9d440000e8d8c4"
            title         : "Home"
            slug          : "home"
            publishedPath : "/"
            draft         : false
            archived      : false
        },
        ...
    ]
```

### Also reading a specific page (pageId set)

```
Outputs:
    pageCollectionId    : ""
    pageCreatedOn       : "2024-01-15T10:30:00Z"
    pageIsArchived      : false
    pageIsDraft         : false
    pageLastUpdated     : "2024-03-20T14:22:00Z"
    pageOpenGraphTitle  : "Home"
    pageParentId        : ""
    pagePublishedPath   : "/"
    pageSeoDescription  : "Welcome to Example Co"
    pageSeoTitle        : "Home | Example Co"
    pageSlug            : "home"
    pageTitle           : "Home"
```

## Page Properties

`getPage` and each entry of `getPages().pages` expose:

| Property        | Type    | Description                                                     |
|-----------------|---------|-----------------------------------------------------------------|
| `pageId`        | string  | The Webflow page ID                                             |
| `siteId`        | string  | The site the page belongs to                                    |
| `title`         | string  | Page title (shown in browser tabs)                              |
| `slug`          | string  | URL slug (e.g., "about" for "/about")                           |
| `publishedPath` | string  | Relative URL of the published page                              |
| `parentId`      | string  | Parent folder ID (empty at the root)                            |
| `collectionId`  | string  | CMS collection ID for collection template pages (empty otherwise)|
| `seo`           | object  | `title` and `description`                                       |
| `openGraph`     | object  | `title`, `description`, `titleCopied`, `descriptionCopied`      |
| `createdOn`     | string  | Creation timestamp (RFC3339)                                    |
| `lastUpdated`   | string  | Last update timestamp (RFC3339)                                 |
| `archived`      | boolean | Whether the page is archived                                    |
| `draft`         | boolean | Whether the page is a draft                                     |
| `canBranch`     | boolean | Whether the page can be branched                                |
| `isBranch`      | boolean | Whether the page is a branch of another page                    |
| `branchId`      | string  | Parent branch ID (empty otherwise)                              |
| `localeId`      | string  | Locale of the returned data (empty for the primary locale)      |

## Cleanup

Functions don't create resources in Webflow, so there's nothing to clean up:

```bash
pulumi stack rm dev
```

## Troubleshooting

### "Site not found" Error

1. Verify your site ID in Webflow: Settings -> General
2. Ensure correct format: 24-character lowercase hexadecimal
3. Check the API token has access to the site

### "Page not found" Error

1. Verify the page exists in your Webflow site
2. Check the page ID is correct (24-character hexadecimal)
3. List all pages first (omit `pageId`) to see the available IDs

### Empty Pages Array

1. Verify the site has pages in the Designer
2. Check that the API token has the `pages:read` scope
3. Ensure you're using the correct site ID

## Related Resources

- [PageMetadata Resource](../pagemetadata/) - manage title, slug, SEO and Open Graph settings
- [PageContent Resource](../pagecontent/)
- [PageCustomCode Resource](../pagecustomcode/)
- [PageSchemaMarkup Resource](../pageschemamarkup/)
- [Main Examples Index](../README.md)
- [Webflow Pages Documentation](https://university.webflow.com/lesson/intro-to-pages)
- [Webflow Pages API](https://developers.webflow.com/data/reference/pages-and-components/pages)
