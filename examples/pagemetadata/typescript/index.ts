import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const siteId = config.require("siteId");
// Slug of the page to manage (pages are created in the Designer, not via the API)
const pageSlug = config.get("pageSlug") || "about";
// Optional: manage a secondary locale's settings instead of the primary locale
const localeId = config.get("localeId");

/**
 * PageMetadata Example - Managing Page Settings
 *
 * This example demonstrates:
 * - `getPages`: list every page of a site (used here to find a page by slug)
 * - `PageMetadata`: manage a page's title, slug, SEO and Open Graph settings
 * - `getPage`: read a single page's metadata back
 *
 * Only the fields you set on PageMetadata are sent to Webflow; everything else
 * keeps the value managed in the Designer. Destroying the resource only removes
 * it from Pulumi state - the page keeps its current settings.
 *
 * Required token scopes: pages:read (functions) and pages:write (resource).
 */

// Step 1: List all pages of the site and pick the one with the configured slug
const allPages = webflow.getPagesOutput({ siteId: siteId, localeId: localeId });

const targetPage = allPages.pages.apply((pages) => {
  const page = pages.find((p) => p.slug === pageSlug);
  if (!page) {
    const available = pages.map((p) => p.slug).join(", ");
    throw new Error(`No page with slug "${pageSlug}" in site ${siteId}. Available slugs: ${available}`);
  }
  return page;
});

// Step 2: Manage the page's settings
const pageSettings = new webflow.PageMetadata("about-page-settings", {
  pageId: targetPage.pageId,
  // Optional locale; omit to update the primary locale
  localeId: localeId,
  // Page title shown in the browser tab (leave unset to keep the Designer value)
  title: "About Us",
  // The slug can be changed too, but Webflow ignores it for the home page,
  // collection template pages and utility pages (404, password, search):
  // slug: "about-us",
  seo: {
    title: "About Us | Example Co",
    description: "Learn who we are, what we build and how to get in touch with the Example Co team.",
  },
  openGraph: {
    title: "About Example Co",
    description: "Meet the team behind Example Co.",
    // Set the *Copied flags to true to reuse the SEO title/description instead
    titleCopied: false,
    descriptionCopied: false,
  },
});

// Step 3: Read the page back with getPage after the update has been applied
const updatedPage = webflow.getPageOutput(
  { pageId: pageSettings.pageId, localeId: localeId },
  { dependsOn: [pageSettings] }
);

// Exports from the getPages listing
export const pageCount = allPages.pages.apply((pages) => pages.length);
export const pageSlugs = allPages.pages.apply((pages) => pages.map((p) => `${p.slug} (${p.pageId})`));
export const managedPageId = targetPage.pageId;

// Exports from the PageMetadata resource
export const managedTitle = pageSettings.title;
// `currentSlug` is what Webflow reports; it differs from `slug` when Webflow ignored the request
export const managedCurrentSlug = pageSettings.currentSlug;
export const managedPublishedPath = pageSettings.publishedPath;
export const managedLastUpdated = pageSettings.lastUpdated;

// Exports from getPage
export const readSeoTitle = updatedPage.seo.title;
export const readSeoDescription = updatedPage.seo.description;
export const readOpenGraphTitle = updatedPage.openGraph.title;
export const readIsDraft = updatedPage.draft;

// Print a short summary
pulumi.all([pageCount, updatedPage.title, updatedPage.publishedPath]).apply(([count, title, path]) => {
  console.log(`Site has ${count} page(s); managed page "${title}" is published at ${path}`);
});
