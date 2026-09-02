import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const pageId = config.require("pageId");
// Optional: manage the markup of a secondary locale instead of the primary one
const localeId = config.get("localeId");

/**
 * PageSchemaMarkup Example - Managing JSON-LD Structured Data on a Page
 *
 * This example demonstrates the `webflow.PageSchemaMarkup` resource and the
 * `getPageSchemaMarkup` function (Webflow's schema markup API is in beta).
 *
 * - The markup is supplied as a JSON string (use JSON.stringify on an object).
 * - It must be a single JSON object; use "@graph" to publish several entities.
 * - The provider compares markup semantically, so key order and whitespace
 *   never cause a diff.
 * - Destroying the resource clears the markup (Webflow has no delete endpoint).
 *
 * Required token scopes: pages:read and pages:write.
 */

// Example 1: FAQ page markup built from a plain object
const faqMarkup = {
  "@context": "https://schema.org",
  "@type": "FAQPage",
  mainEntity: [
    {
      "@type": "Question",
      name: "How do I manage Webflow with Pulumi?",
      acceptedAnswer: {
        "@type": "Answer",
        text: "Install the @jdetmar/pulumi-webflow package and declare your Webflow resources in code.",
      },
    },
    {
      "@type": "Question",
      name: "Which token scopes are required for schema markup?",
      acceptedAnswer: {
        "@type": "Answer",
        text: "pages:read to read the markup and pages:write to change it.",
      },
    },
  ],
};

const faqSchema = new webflow.PageSchemaMarkup("faq-schema", {
  pageId: pageId,
  schemaMarkup: JSON.stringify(faqMarkup),
  // Omit localeId to manage the primary locale
  localeId: localeId,
});

// Example 2: Several entities on one page using "@graph"
// Uncomment and point pageId at another page to publish an Organization and
// a WebSite entity together. Webflow limits each entry to 60KB, 32 levels of
// nesting and 5,000 nodes.
// const orgSchema = new webflow.PageSchemaMarkup("home-schema", {
//   pageId: homePageId,
//   schemaMarkup: JSON.stringify({
//     "@context": "https://schema.org",
//     "@graph": [
//       { "@type": "Organization", name: "Example Co", url: "https://example.com" },
//       { "@type": "WebSite", name: "Example Co", url: "https://example.com" },
//     ],
//   }),
// });

// Example 3: Read the markup back with the getPageSchemaMarkup function
// `dependsOn` makes sure the read happens after the resource is written.
const currentMarkup = webflow.getPageSchemaMarkupOutput(
  { pageId: pageId, localeId: localeId },
  { dependsOn: [faqSchema] }
);

// Parse the returned JSON string to work with it as an object
const parsedMarkup = currentMarkup.schemaMarkup.apply((json) =>
  json ? (JSON.parse(json) as Record<string, unknown>) : undefined
);

// Export resource outputs
export const schemaPageId = faqSchema.pageId;
export const schemaSiteId = faqSchema.siteId;
export const schemaPublishedPath = faqSchema.publishedPath;
export const schemaLastUpdated = faqSchema.lastUpdated;
// True only when a secondary locale falls back to the primary locale's markup
export const schemaIsInherited = faqSchema.isInherited;
export const schemaEffectiveLocaleId = faqSchema.effectiveLocaleId;

// Export what the function read back
export const readMarkupType = parsedMarkup.apply((m) => (m ? m["@type"] : undefined));
export const readQuestionCount = parsedMarkup.apply((m) =>
  m && Array.isArray(m.mainEntity) ? m.mainEntity.length : 0
);
export const readIsInherited = currentMarkup.isInherited;

// Print a short summary
pulumi.all([faqSchema.publishedPath, readQuestionCount]).apply(([path, count]) => {
  console.log(`Schema markup on ${path || pageId}: FAQPage with ${count} question(s)`);
});
