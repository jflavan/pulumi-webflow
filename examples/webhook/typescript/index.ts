import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const siteId = config.requireSecret("siteId");

/**
 * Webhook Example - Creating and Managing Webflow Webhooks
 *
 * This example demonstrates how to set up webhooks for your Webflow sites.
 * Webhooks allow you to receive real-time notifications when events occur, such as:
 * - Form submissions
 * - Site publishes
 * - Page updates
 * - E-commerce orders
 * - Collection item changes
 * - Comments
 *
 * Webhooks require a Data Client (OAuth) token with sites:write plus the read
 * scope of each event family you subscribe to (forms:read, cms:read, pages:read,
 * ecommerce:read, comments:read). Site API tokens cannot manage webhooks.
 */

// Example 1: Form Submission Webhook
// Receive notifications when users submit any form on your site
const formWebhook = new webflow.Webhook("form-submission-webhook", {
  siteId: siteId,
  triggerType: "form_submission",
  url: "https://your-api.example.com/webhooks/webflow/forms",
});

// Example 2: Site Publish Webhook
// Get notified when your site is published
const publishWebhook = new webflow.Webhook("site-publish-webhook", {
  siteId: siteId,
  triggerType: "site_publish",
  url: "https://your-api.example.com/webhooks/webflow/publish",
});

// Example 3: E-commerce Order Webhook
// Track new orders in your Webflow e-commerce store
const ecommWebhook = new webflow.Webhook("ecomm-order-webhook", {
  siteId: siteId,
  triggerType: "ecomm_new_order",
  url: "https://your-api.example.com/webhooks/webflow/orders",
});

// Example 4: Collection Item Webhook
// Monitor newly created CMS items. Webflow fires collection item events for
// every collection of the site; the API has no per-collection filter.
const collectionWebhook = new webflow.Webhook("collection-item-webhook", {
  siteId: siteId,
  triggerType: "collection_item_created",
  url: "https://your-api.example.com/webhooks/webflow/collection",
});

// Example 5: Page Metadata Update Webhook
// Track when page metadata changes (title, description, SEO settings)
const pageMetadataWebhook = new webflow.Webhook("page-metadata-webhook", {
  siteId: siteId,
  triggerType: "page_metadata_updated",
  url: "https://your-api.example.com/webhooks/webflow/pages",
});

// Example 6: Filtered Form Submission Webhook
// `filter` is only valid for form_submission and has a single field, `name`:
// the webhook fires only for submissions of the form with that name.
const contactFormWebhook = new webflow.Webhook("contact-form-webhook", {
  siteId: siteId,
  triggerType: "form_submission",
  url: "https://your-api.example.com/webhooks/webflow/contact",
  filter: {
    name: "Contact Form",
  },
});

// Export webhook IDs and timestamps for reference
export const deployedSiteId = siteId;
export const formWebhookId = formWebhook.id;
export const formWebhookCreated = formWebhook.createdOn;
export const publishWebhookId = publishWebhook.id;
export const ecommWebhookId = ecommWebhook.id;
export const collectionWebhookId = collectionWebhook.id;
export const pageMetadataWebhookId = pageMetadataWebhook.id;
export const contactFormWebhookId = contactFormWebhook.id;

// Print deployment success message
const webhookCount = 6;
const message = pulumi.interpolate`✅ Successfully deployed ${webhookCount} webhooks to site ${siteId}`;
message.apply((m) => console.log(m));
