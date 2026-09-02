import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const siteId = config.requireSecret("siteId");
const pageId = config.get("pageId"); // Optional: set to get a specific page

/**
 * Page Data Source Example - Reading Page Information
 *
 * This example demonstrates how to read page information from a Webflow site.
 * Pages cannot be created via the API - they must be created in the Webflow designer.
 * This data source allows you to retrieve page metadata for use in your infrastructure code.
 *
 * Use cases:
 * - Reference existing pages in your infrastructure
 * - Get page metadata for custom code injection
 * - List all pages in a site for documentation
 * - Query page properties for conditional logic
 */

// Example 1: Get all pages for a site
// When pageId is not specified, retrieves all pages
const allPages = new webflow.PageData("all-pages", {
  siteId: siteId,
});

// `pages` is optional in the schema; normalise it to an array once for the exports below
const pageList = allPages.pages.apply((pages) => pages ?? []);

// Example 2: Get a specific page by ID (conditional on config)
// When pageId is specified, retrieves only that page's details
let specificPage: webflow.PageData | undefined;
if (pageId) {
  specificPage = new webflow.PageData("specific-page", {
    siteId: siteId,
    pageId: pageId,
  });
}

// Export outputs for all pages scenario
export const sitePages = pageList.apply((pages) => {
  // Transform the pages array into a readable format
  return pages.map((page) => ({
    id: page.pageId,
    title: page.title,
    slug: page.slug,
    draft: page.draft,
    archived: page.archived,
  }));
});

export const pageCount = pageList.apply((pages) => pages.length);

// Export the full list of page IDs for reference
export const pageIds = pageList.apply((pages) =>
  pages.map((p) => p.pageId)
);

// Export outputs for specific page scenario (undefined when pageId is not configured).
// Stack outputs in TypeScript are top-level `export`s; there is no pulumi.export().
export const pageTitle = specificPage?.title;
export const pageSlug = specificPage?.slug;
export const pageWebflowId = specificPage?.webflowPageId;
export const pageCreatedOn = specificPage?.createdOn;
export const pageLastUpdated = specificPage?.lastUpdated;
export const pageIsDraft = specificPage?.draft;
export const pageIsArchived = specificPage?.archived;
export const pageParentId = specificPage?.parentId;
export const pageCollectionId = specificPage?.collectionId;

// Print helpful information
pageList.apply((pages) => {
  console.log(`\n📄 Found ${pages.length} pages in the site`);

  // Show a sample of pages
  const sampleSize = Math.min(5, pages.length);
  if (sampleSize > 0) {
    console.log(`\nFirst ${sampleSize} pages:`);
    pages.slice(0, sampleSize).forEach((page, idx) => {
      console.log(`  ${idx + 1}. "${page.title}" (/${page.slug})`);
    });

    if (pages.length > sampleSize) {
      console.log(`  ... and ${pages.length - sampleSize} more`);
    }
  }
});

if (specificPage) {
  specificPage.title.apply((title) => {
    console.log(`\n✅ Retrieved page: "${title}"`);
  });
}
