// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/JDetmar/pulumi-webflow/sdk/go/webflow"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Asset Example - Uploading Files to Webflow
//
// FileSource points at a local file (resolved relative to this program's
// directory) or an http(s) URL. At apply time the provider reads the content,
// computes its MD5 FileHash, registers the asset with Webflow and completes the
// S3 upload for you. A content change of a local file replaces the asset.
// UploadUrl / UploadDetails are secret outputs; FolderId reports the folder
// Webflow placed the asset in.
//
// Required token scopes: assets:read and assets:write.
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		siteID := cfg.RequireSecret("siteId")
		// Optional: an http(s) URL to upload as a second asset
		heroImageURL := cfg.Get("heroImageUrl")

		// Example 1: Upload a local file shipped with this example
		logoAsset, err := webflow.NewAsset(ctx, "company-logo", &webflow.AssetArgs{
			SiteId:     siteID,
			FileName:   pulumi.String("logo.svg"),
			FileSource: pulumi.String("./assets/logo.svg"),
		})
		if err != nil {
			return fmt.Errorf("failed to create logo asset: %w", err)
		}

		// Example 2: Upload from a URL, optionally into a folder
		// Set `pulumi config set heroImageUrl https://.../hero.jpg` to enable it.
		var heroAsset *webflow.Asset
		if heroImageURL != "" {
			heroAsset, err = webflow.NewAsset(ctx, "hero-image", &webflow.AssetArgs{
				SiteId:     siteID,
				FileName:   pulumi.String("hero-banner.jpg"),
				FileSource: pulumi.String(heroImageURL),
				// ParentFolder: pulumi.String("folder-id-here"), // Uncomment to organize in an asset folder
			})
			if err != nil {
				return fmt.Errorf("failed to create hero asset: %w", err)
			}
		}

		// Example 3: Bulk upload of local files
		icons := []struct {
			name       string
			fileName   string
			fileSource string
		}{
			{"icon-home", "icon-home.svg", "./assets/icons/home.svg"},
			{"icon-user", "icon-user.svg", "./assets/icons/user.svg"},
		}

		iconAssetIDs := pulumi.StringArray{}
		for _, icon := range icons {
			asset, err := webflow.NewAsset(ctx, icon.name, &webflow.AssetArgs{
				SiteId:     siteID,
				FileName:   pulumi.String(icon.fileName),
				FileSource: pulumi.String(icon.fileSource),
			})
			if err != nil {
				return fmt.Errorf("failed to create icon asset %s: %w", icon.name, err)
			}
			iconAssetIDs = append(iconAssetIDs, asset.AssetId.Elem())
		}

		// Export values for the logo asset
		ctx.Export("logoAssetId", logoAsset.AssetId)
		ctx.Export("logoHostedUrl", logoAsset.HostedUrl) // public CDN URL of the uploaded file
		ctx.Export("logoAssetUrl", logoAsset.AssetUrl)
		ctx.Export("logoFileHash", logoAsset.FileHash) // computed from the file content
		ctx.Export("logoContentType", logoAsset.ContentType)
		ctx.Export("logoSize", logoAsset.Size)
		ctx.Export("logoFolderId", logoAsset.FolderId)

		// Export hero asset info (only when heroImageUrl is configured)
		assetCount := len(icons) + 1
		if heroAsset != nil {
			ctx.Export("heroAssetId", heroAsset.AssetId)
			ctx.Export("heroHostedUrl", heroAsset.HostedUrl)
			assetCount++
		}

		// Export icon asset IDs
		ctx.Export("iconAssetIds", iconAssetIDs)

		logoAsset.HostedUrl.ApplyT(func(url *string) interface{} {
			hosted := ""
			if url != nil {
				hosted = *url
			}
			ctx.Log.Info(fmt.Sprintf("Uploaded %d assets. Logo is available at %s", assetCount, hosted), nil)
			return nil
		})

		return nil
	})
}
