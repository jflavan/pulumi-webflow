package main

import (
	"fmt"

	"github.com/JDetmar/pulumi-webflow/sdk/go/webflow"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Webhooks require a Data Client (OAuth) token with sites:write plus the read
// scope of each event family you subscribe to (forms:read, cms:read, pages:read,
// ecommerce:read, comments:read). Site API tokens cannot manage webhooks.
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		siteID := cfg.RequireSecret("siteId")
		environment := cfg.Get("environment")
		if environment == "" {
			environment = "development"
		}

		// Example 1: Form Submission Webhook
		// Receive notifications when users submit any form on your site
		formWebhook, err := webflow.NewWebhook(ctx, "form-submission-webhook", &webflow.WebhookArgs{
			SiteId:      siteID,
			TriggerType: pulumi.String("form_submission"),
			Url:         pulumi.String("https://your-api.example.com/webhooks/webflow/forms"),
		})
		if err != nil {
			return fmt.Errorf("failed to create form webhook: %w", err)
		}

		// Example 2: Site Publish Webhook
		// Get notified when your site is published
		publishWebhook, err := webflow.NewWebhook(ctx, "site-publish-webhook", &webflow.WebhookArgs{
			SiteId:      siteID,
			TriggerType: pulumi.String("site_publish"),
			Url:         pulumi.String("https://your-api.example.com/webhooks/webflow/publish"),
		})
		if err != nil {
			return fmt.Errorf("failed to create publish webhook: %w", err)
		}

		// Example 3: E-commerce Order Webhook
		// Track new orders in your Webflow e-commerce store
		ecommWebhook, err := webflow.NewWebhook(ctx, "ecomm-order-webhook", &webflow.WebhookArgs{
			SiteId:      siteID,
			TriggerType: pulumi.String("ecomm_new_order"),
			Url:         pulumi.String("https://your-api.example.com/webhooks/webflow/orders"),
		})
		if err != nil {
			return fmt.Errorf("failed to create ecomm webhook: %w", err)
		}

		// Example 4: Collection Item Webhook
		// Monitor newly created CMS items. Webflow fires collection item events for
		// every collection of the site; the API has no per-collection filter.
		collectionWebhook, err := webflow.NewWebhook(ctx, "collection-item-webhook", &webflow.WebhookArgs{
			SiteId:      siteID,
			TriggerType: pulumi.String("collection_item_created"),
			Url:         pulumi.String("https://your-api.example.com/webhooks/webflow/collection"),
		})
		if err != nil {
			return fmt.Errorf("failed to create collection webhook: %w", err)
		}

		// Example 5: Page Metadata Update Webhook
		// Track when page metadata changes (title, description, SEO settings)
		pageMetadataWebhook, err := webflow.NewWebhook(ctx, "page-metadata-webhook", &webflow.WebhookArgs{
			SiteId:      siteID,
			TriggerType: pulumi.String("page_metadata_updated"),
			Url:         pulumi.String("https://your-api.example.com/webhooks/webflow/pages"),
		})
		if err != nil {
			return fmt.Errorf("failed to create page metadata webhook: %w", err)
		}

		// Example 6: Filtered Form Submission Webhook
		// `filter` is only valid for form_submission and has a single field, `name`:
		// the webhook fires only for submissions of the form with that name.
		contactFormWebhook, err := webflow.NewWebhook(ctx, "contact-form-webhook", &webflow.WebhookArgs{
			SiteId:      siteID,
			TriggerType: pulumi.String("form_submission"),
			Url:         pulumi.String("https://your-api.example.com/webhooks/webflow/contact"),
			Filter: pulumi.Map{
				"name": pulumi.String("Contact Form"),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create contact form webhook: %w", err)
		}

		// Export webhook IDs and timestamps for reference
		ctx.Export("deployedSiteId", siteID)
		ctx.Export("formWebhookId", formWebhook.ID())
		ctx.Export("formWebhookCreated", formWebhook.CreatedOn)
		ctx.Export("publishWebhookId", publishWebhook.ID())
		ctx.Export("ecommWebhookId", ecommWebhook.ID())
		ctx.Export("collectionWebhookId", collectionWebhook.ID())
		ctx.Export("pageMetadataWebhookId", pageMetadataWebhook.ID())
		ctx.Export("contactFormWebhookId", contactFormWebhook.ID())

		webhookCount := 6
		ctx.Log.Info(
			fmt.Sprintf("✅ Successfully deployed %d webhooks in %s environment", webhookCount, environment),
			nil,
		)

		return nil
	})
}
