// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/JDetmar/pulumi-webflow/sdk/go/webflow"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Webflow redirects are always permanent (HTTP 301). The deprecated StatusCode
// input is ignored by the provider, so it is simply omitted here.
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		siteID := cfg.RequireSecret("siteId")
		environment := cfg.Get("environment")
		if environment == "" {
			environment = "development"
		}

		// Example 1: Content Move Redirect
		permanentRedirect, err := webflow.NewRedirect(ctx, "old-blog-to-new-blog", &webflow.RedirectArgs{
			SiteId:          siteID,
			SourcePath:      pulumi.String("/blog/old-article"),
			DestinationPath: pulumi.String("/blog/articles/updated-article"),
		})
		if err != nil {
			return fmt.Errorf("failed to create permanent redirect: %w", err)
		}

		// Example 2: Campaign Redirect
		// Point a short, memorable path at the current campaign landing page.
		campaignRedirect, err := webflow.NewRedirect(ctx, "campaign-landing-page", &webflow.RedirectArgs{
			SiteId:          siteID,
			SourcePath:      pulumi.String("/old-campaign"),
			DestinationPath: pulumi.String("/new-campaign-2025"),
		})
		if err != nil {
			return fmt.Errorf("failed to create campaign redirect: %w", err)
		}

		// Example 3: External Redirect
		externalRedirect, err := webflow.NewRedirect(ctx, "external-partner-link", &webflow.RedirectArgs{
			SiteId:          siteID,
			SourcePath:      pulumi.String("/partner"),
			DestinationPath: pulumi.String("https://partner-site.com"),
		})
		if err != nil {
			return fmt.Errorf("failed to create external redirect: %w", err)
		}

		// Example 4: Bulk Redirects
		redirectMappings := []struct {
			old string
			new string
		}{
			{"/product-a", "/products/product-a"},
			{"/product-b", "/products/product-b"},
			{"/product-c", "/products/product-c"},
		}

		bulkRedirectIds := pulumi.StringArray{}
		for i, mapping := range redirectMappings {
			redirect, err := webflow.NewRedirect(ctx, fmt.Sprintf("bulk-redirect-%d", i), &webflow.RedirectArgs{
				SiteId:          siteID,
				SourcePath:      pulumi.String(mapping.old),
				DestinationPath: pulumi.String(mapping.new),
			})
			if err != nil {
				return fmt.Errorf("failed to create bulk redirect %d: %w", i, err)
			}
			bulkRedirectIds = append(bulkRedirectIds, redirect.ID().ToStringOutput())
		}

		// Export values
		ctx.Export("deployedSiteId", siteID)
		ctx.Export("permanentRedirectId", permanentRedirect.ID())
		ctx.Export("campaignRedirectId", campaignRedirect.ID())
		ctx.Export("externalRedirectId", externalRedirect.ID())
		ctx.Export("bulkRedirectIds", bulkRedirectIds)

		ctx.Log.Info(
			fmt.Sprintf("✅ Successfully deployed %d redirects in %s environment", len(redirectMappings)+3, environment),
			nil,
		)

		return nil
	})
}
