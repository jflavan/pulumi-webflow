import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const siteId = config.requireSecret("siteId");
const pageId = config.get("pageId"); // Optional: set to also read a single page

/**
 * Page Functions Example - Reading Page Information
 *
 * This example demonstrates how to read page information from a Webflow site
 * with the `getPages` and `getPage` functions. Pages cannot be created via the
 * API - they must be created in the Webflow Designer. The functions let you
 * discover page IDs and use page metadata in your infrastructure code.
 *
 * Use cases:
 * - Find page IDs for PageMetadata, PageContent, PageCustomCode or PageSchemaMarkup
 * - List all pages in a site for documentation
 * - Query page properties (draft, archived, SEO, ...) for conditional logic
 *
 * Required token scope: pages:read.
 */

// Example 1: List all pages of the site
// getPages follows API pagination and returns every page.
const allPages = webflow.getPagesOutput({
  siteId: siteId,
  // localeId: "your-locale-id", // optional: list a secondary locale instead
});

const pageList = allPages.pages;

// Example 2: Read a single page by ID (only when pageId is configured)
let specificPage: pulumi.Output<webflow.GetPageResult> | undefined;
if (pageId) {
  specificPage = webflow.getPageOutput({
    pageId: pageId,
    // localeId: "your-locale-id", // optional secondary locale
    // translatable: true,         // return the locale's own translation, not inherited content
  });
}

// Export outputs for the listing
export const sitePages = pageList.apply((pages) =>
  pages.map((page) => ({
    id: page.pageId,
    title: page.title,
    slug: page.slug,
    publishedPath: page.publishedPath,
    draft: page.draft,
    archived: page.archived,
  }))
);

export const pageCount = pageList.apply((pages) => pages.length);

// Export the full list of page IDs for reference
export const pageIds = pageList.apply((pages) => pages.map((p) => p.pageId));

// Filter pages by their properties
export const draftPageSlugs = pageList.apply((pages) => pages.filter((p) => p.draft).map((p) => p.slug));
export const collectionTemplateSlugs = pageList.apply((pages) =>
  pages.filter((p) => p.collectionId !== "").map((p) => p.slug)
);

// Export outputs for the single-page scenario (undefined when pageId is not configured).
// Stack outputs in TypeScript are top-level `export`s; there is no pulumi.export().
export const pageTitle = specificPage?.title;
export const pageSlug = specificPage?.slug;
export const pagePublishedPath = specificPage?.publishedPath;
export const pageCreatedOn = specificPage?.createdOn;
export const pageLastUpdated = specificPage?.lastUpdated;
export const pageIsDraft = specificPage?.draft;
export const pageIsArchived = specificPage?.archived;
export const pageParentId = specificPage?.parentId;
export const pageCollectionId = specificPage?.collectionId;
export const pageSeoTitle = specificPage?.seo.title;
export const pageSeoDescription = specificPage?.seo.description;
export const pageOpenGraphTitle = specificPage?.openGraph.title;

// Print helpful information
pageList.apply((pages) => {
  console.log(`\nFound ${pages.length} pages in the site`);

  // Show a sample of pages
  const sampleSize = Math.min(5, pages.length);
  if (sampleSize > 0) {
    console.log(`\nFirst ${sampleSize} pages:`);
    pages.slice(0, sampleSize).forEach((page, idx) => {
      console.log(`  ${idx + 1}. "${page.title}" (${page.publishedPath || "/" + page.slug}) id=${page.pageId}`);
    });

    if (pages.length > sampleSize) {
      console.log(`  ... and ${pages.length - sampleSize} more`);
    }
  }
});

if (specificPage) {
  specificPage.title.apply((title) => {
    console.log(`\nRetrieved page: "${title}"`);
  });
}
