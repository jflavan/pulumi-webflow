// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const pageId = config.requireSecret("pageId");
// The Webflow API can only update the content of a *secondary* locale, so the
// locale ID is required. Find it under Site settings -> Localization or via
// GET /v2/sites/{site_id} (the `locales.secondary[].id` values).
const localeId = config.require("localeId");

/**
 * PageContent Example - Managing Static Text Content on Webflow Pages
 *
 * This example demonstrates how to manage static text content within existing DOM nodes
 * on a Webflow page. This is useful for:
 * - Programmatically updating localized text content across multiple pages
 * - Maintaining consistent messaging via infrastructure-as-code
 * - Automating content updates as part of deployments
 *
 * IMPORTANT NOTES:
 * - This resource does NOT manage page structure or layout
 * - It only updates text content within existing DOM nodes
 * - `localeId` is required and must be a secondary locale; the primary locale's
 *   content cannot be edited through the API
 * - Node IDs must be retrieved from the page's DOM structure first
 *   (GET /v2/pages/{page_id}/dom?localeId=...)
 * - `text` must be non-empty: an empty string does not clear a node
 * - At most 1000 nodes can be updated per resource (one API request)
 * - Drift detection is limited: only verifies the page exists, not content
 */

/**
 * Example 1: Update Hero Section Text
 *
 * Updates the main heading and subtitle in a hero section.
 * You would get these node IDs by fetching the page DOM first.
 */
const heroContent = new webflow.PageContent("hero-section-content", {
  pageId: pageId,
  localeId: localeId,
  nodes: [
    {
      nodeId: "hero-heading-node-id",
      text: "Welcome to Our Platform",
    },
    {
      nodeId: "hero-subtitle-node-id",
      text: "Build amazing experiences with our tools",
    },
  ],
});

/**
 * Example 2: Update Footer Copyright Text
 *
 * Keeps copyright year and company information up-to-date.
 */
const currentYear = new Date().getFullYear();
const footerContent = new webflow.PageContent("footer-content", {
  pageId: pageId,
  localeId: localeId,
  nodes: [
    {
      nodeId: "footer-copyright-node-id",
      text: `© ${currentYear} Your Company Name. All rights reserved.`,
    },
  ],
});

/**
 * Example 3: Update Multiple Text Blocks
 *
 * Update multiple content sections at once, such as feature descriptions.
 */
const featureContent = new webflow.PageContent("feature-section-content", {
  pageId: pageId,
  localeId: localeId,
  nodes: [
    {
      nodeId: "feature-1-title-node-id",
      text: "Fast Performance",
    },
    {
      nodeId: "feature-1-description-node-id",
      text: "Lightning-fast load times for the best user experience.",
    },
    {
      nodeId: "feature-2-title-node-id",
      text: "Secure & Reliable",
    },
    {
      nodeId: "feature-2-description-node-id",
      text: "Enterprise-grade security with 99.9% uptime guarantee.",
    },
  ],
});

// Export resource information for reference
export const deployedPageId = pageId;
export const deployedLocaleId = localeId;
export const heroContentId = heroContent.id;
export const heroNodeCount = heroContent.nodes.apply((nodes) => nodes.length);
export const footerContentId = footerContent.id;
export const featureContentId = featureContent.id;

// Print deployment success message
const message = pulumi.interpolate`✅ Successfully updated page content for page ${pageId} (locale ${localeId})`;
message.apply((m) => console.log(m));
