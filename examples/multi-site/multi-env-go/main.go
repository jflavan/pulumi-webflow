// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"

	"github.com/JDetmar/pulumi-webflow/sdk/go/webflow"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		// Load environment-specific configuration
		stackName := ctx.Stack()
		workspaceID := cfg.Require("workspaceId") // Enterprise workspace ID
		sitePrefix := cfg.Require("sitePrefix")   // e.g., "dev", "staging", "prod"
		siteCount := cfg.RequireInt("siteCount")  // Number of sites to create

		ctx.Log.Info(fmt.Sprintf("Deploying %d %s sites to stack: %s", siteCount,
			sitePrefix, stackName), nil)

		// Environment marker embedded in every site's robots.txt
		robotsContent := fmt.Sprintf(`User-agent: *
Allow: /
# Environment: %s`, sitePrefix)

		// Create environment-specific site fleet
		siteIDs := make(pulumi.StringArray, 0, siteCount)

		for i := 0; i < siteCount; i++ {
			// Create unique site name with environment prefix.
			// Webflow derives the short name (URL slug) from displayName; it is an output.
			siteName := fmt.Sprintf("%s-site-%d", sitePrefix, i+1)
			displayName := fmt.Sprintf("%s Site %d",
				strings.ToUpper(sitePrefix), i+1)

			// Create the site
			site, err := webflow.NewSite(ctx, siteName, &webflow.SiteArgs{
				WorkspaceId: pulumi.String(workspaceID),
				DisplayName: pulumi.String(displayName),
			})
			if err != nil {
				return fmt.Errorf("failed to create site %s: %w", siteName, err)
			}

			siteIDs = append(siteIDs, site.ID().ToStringOutput())

			// Add robots.txt with environment marker
			_, err = webflow.NewRobotsTxt(ctx, fmt.Sprintf("%s-robots", siteName),
				&webflow.RobotsTxtArgs{
					SiteId:  site.ID(),
					Content: pulumi.String(robotsContent),
				})
			if err != nil {
				return fmt.Errorf("failed to create robots.txt for %s: %w",
					siteName, err)
			}

			// Add environment-specific redirect
			if sitePrefix == "prod" {
				// Production: redirect old domains
				_, err = webflow.NewRedirect(ctx, fmt.Sprintf("%s-domain-redirect", siteName),
					&webflow.RedirectArgs{
						SiteId:          site.ID(),
						SourcePath:      pulumi.String("/old-domain"),
						DestinationPath: pulumi.String("/"),
						StatusCode:      pulumi.Int(301),
					})
				if err != nil {
					return fmt.Errorf("failed to create redirect for %s: %w",
						siteName, err)
				}
			}

			// Export individual site ID and the generated short name / time zone
			ctx.Export(fmt.Sprintf("%s-id", siteName), site.ID())
			ctx.Export(fmt.Sprintf("%s-short-name", siteName), site.ShortName)
			ctx.Export(fmt.Sprintf("%s-time-zone", siteName), site.TimeZone)
		}

		// Export summary information
		ctx.Export(fmt.Sprintf("%s-total-sites", sitePrefix), pulumi.Int(siteCount))
		ctx.Export(fmt.Sprintf("%s-site-ids", sitePrefix), siteIDs)

		return nil
	})
}
