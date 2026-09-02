import pulumi
import pulumi_webflow as webflow

# Create a Pulumi config object
config = pulumi.Config()

# Get configuration values
site_id = config.require_secret("siteId")

"""
Webhook Example - Creating and Managing Webflow Webhooks

This example demonstrates how to set up webhooks for your Webflow sites.
Webhooks allow you to receive real-time notifications when events occur.

Webhooks require a Data Client (OAuth) token with sites:write plus the read
scope of each event family you subscribe to (forms:read, cms:read, pages:read,
ecommerce:read, comments:read). Site API tokens cannot manage webhooks.
"""

# Example 1: Form Submission Webhook
# Receive notifications when users submit any form on your site
form_webhook = webflow.Webhook("form-submission-webhook",
    site_id=site_id,
    trigger_type="form_submission",
    url="https://your-api.example.com/webhooks/webflow/forms")

# Example 2: Site Publish Webhook
# Get notified when your site is published
publish_webhook = webflow.Webhook("site-publish-webhook",
    site_id=site_id,
    trigger_type="site_publish",
    url="https://your-api.example.com/webhooks/webflow/publish")

# Example 3: E-commerce Order Webhook
# Track new orders in your Webflow e-commerce store
ecomm_webhook = webflow.Webhook("ecomm-order-webhook",
    site_id=site_id,
    trigger_type="ecomm_new_order",
    url="https://your-api.example.com/webhooks/webflow/orders")

# Example 4: Collection Item Webhook
# Monitor newly created CMS items. Webflow fires collection item events for
# every collection of the site; the API has no per-collection filter.
collection_webhook = webflow.Webhook("collection-item-webhook",
    site_id=site_id,
    trigger_type="collection_item_created",
    url="https://your-api.example.com/webhooks/webflow/collection")

# Example 5: Page Metadata Update Webhook
# Track when page metadata changes (title, description, SEO settings)
page_metadata_webhook = webflow.Webhook("page-metadata-webhook",
    site_id=site_id,
    trigger_type="page_metadata_updated",
    url="https://your-api.example.com/webhooks/webflow/pages")

# Example 6: Filtered Form Submission Webhook
# `filter` is only valid for form_submission and has a single field, `name`:
# the webhook fires only for submissions of the form with that name.
contact_form_webhook = webflow.Webhook("contact-form-webhook",
    site_id=site_id,
    trigger_type="form_submission",
    url="https://your-api.example.com/webhooks/webflow/contact",
    filter={
        "name": "Contact Form"
    })

# Export webhook IDs and timestamps for reference
pulumi.export("deployed_site_id", site_id)
pulumi.export("form_webhook_id", form_webhook.id)
pulumi.export("form_webhook_created", form_webhook.created_on)
pulumi.export("publish_webhook_id", publish_webhook.id)
pulumi.export("ecomm_webhook_id", ecomm_webhook.id)
pulumi.export("collection_webhook_id", collection_webhook.id)
pulumi.export("page_metadata_webhook_id", page_metadata_webhook.id)
pulumi.export("contact_form_webhook_id", contact_form_webhook.id)

# Print success message
webhook_count = 6
site_id.apply(lambda s: print(f"✅ Deployed {webhook_count} webhooks to site {s}"))
