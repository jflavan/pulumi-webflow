// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const siteId = config.require("siteId");
// A GA4 measurement ID such as "G-1A2B3C4D5E" (see the README for how to find yours)
const ga4TagId = config.get("ga4TagId") || "G-1A2B3C4D5E";

/**
 * GoogleTag Example - Managing Google Tag IDs on a Webflow Site
 *
 * This example demonstrates the `webflow.GoogleTag` resource, which manages one
 * entry in a site's Google Tag Manager integration list. Each resource owns a
 * single tag ID, so a site can have several GoogleTag resources. Tags that were
 * added in the Webflow dashboard and are not managed by Pulumi are left untouched.
 *
 * Accepted tag IDs: GA4 measurement IDs (G-...), Google Tags (GT-...),
 * Google Ads (AW-...) and Campaign Manager (DC-...). Legacy Universal
 * Analytics "UA-" IDs are rejected by Webflow.
 *
 * Required token scopes: sites:read and sites:write.
 */

// Example 1: Google Analytics 4 measurement ID
// Only the required inputs are set; Webflow assigns the display position.
const analyticsTag = new webflow.GoogleTag("primary-analytics", {
  siteId: siteId,
  tagId: ga4TagId,
  displayName: "Primary Google Analytics",
});

// Example 2: Google Ads conversion tag with an explicit position
// `order` controls where the tag appears in the site's tag list. Webflow
// renormalizes positions after deletions, so only set it when ordering matters.
const adsTag = new webflow.GoogleTag("google-ads", {
  siteId: siteId,
  tagId: "AW-123456789",
  displayName: "Google Ads conversions",
  order: 2,
});

// Example 3: Several tags from a list
// Webflow allows up to 25 tags per site.
const extraTags = [
  { name: "campaign-manager", tagId: "DC-1234567", displayName: "Campaign Manager" },
  { name: "google-tag", tagId: "GT-ABC123", displayName: "Google Tag (server-side)" },
];
const extraTagResources = extraTags.map(
  (tag) =>
    new webflow.GoogleTag(tag.name, {
      siteId: siteId,
      tagId: tag.tagId,
      displayName: tag.displayName,
    })
);

// Export the managed tag IDs and the positions Webflow reports for them
export const analyticsTagId = analyticsTag.tagId;
export const analyticsDisplayName = analyticsTag.displayName;
// `effectiveOrder` is read-only and may differ from the `order` you requested
export const analyticsEffectiveOrder = analyticsTag.effectiveOrder;
export const adsTagId = adsTag.tagId;
export const adsEffectiveOrder = adsTag.effectiveOrder;
export const extraTagIds = extraTagResources.map((t) => t.tagId);

// Print a short summary once the tags are created
pulumi
  .all([analyticsTag.tagId, adsTag.tagId, ...extraTagResources.map((t) => t.tagId)])
  .apply((ids) => {
    console.log(`Managing ${ids.length} Google Tag IDs on site ${siteId}: ${ids.join(", ")}`);
  });
