// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/JDetmar/pulumi-webflow/sdk/go/webflow"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// SiteConfig describes one site to create. ShortName and TimeZone are generated
// by Webflow and exposed as outputs, so they are not part of the input config.
type SiteConfig struct {
	Name        string
	DisplayName string
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Site creation requires an Enterprise workspace ID (pulumi config set workspaceId ...)
		cfg := config.New(ctx, "")
		workspaceID := cfg.Require("workspaceId")

		// Define site configurations
		siteConfigs := []SiteConfig{
			{Name: "marketing-site", DisplayName: "Marketing Site"},
			{Name: "docs-site", DisplayName: "Documentation Site"},
			{Name: "blog-site", DisplayName: "Blog Site"},
		}

		// Create sites
		siteIDs := make(pulumi.StringArray, 0, len(siteConfigs))

		for _, siteConfig := range siteConfigs {
			site, err := webflow.NewSite(ctx, siteConfig.Name, &webflow.SiteArgs{
				WorkspaceId: pulumi.String(workspaceID),
				DisplayName: pulumi.String(siteConfig.DisplayName),
			})
			if err != nil {
				return fmt.Errorf("failed to create site %s: %w", siteConfig.Name, err)
			}

			siteIDs = append(siteIDs, site.ID().ToStringOutput())

			// Create robots.txt for each site
			_, err = webflow.NewRobotsTxt(ctx, fmt.Sprintf("%s-robots", siteConfig.Name),
				&webflow.RobotsTxtArgs{
					SiteId:  site.ID(),
					Content: pulumi.String("User-agent: *\nAllow: /"),
				})
			if err != nil {
				return fmt.Errorf("failed to create robots.txt for site %s: %w",
					siteConfig.Name, err)
			}

			// Export individual site ID and the Webflow-generated short name
			ctx.Export(fmt.Sprintf("%s-id", siteConfig.Name), site.ID())
			ctx.Export(fmt.Sprintf("%s-short-name", siteConfig.Name), site.ShortName)
		}

		// Export all site IDs as array
		ctx.Export("all-site-ids", siteIDs)
		ctx.Export("site-count", pulumi.Int(len(siteConfigs)))

		return nil
	})
}
