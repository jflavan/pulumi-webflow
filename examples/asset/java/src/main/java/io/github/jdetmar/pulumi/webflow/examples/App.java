// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

package io.github.jdetmar.pulumi.webflow.examples;

import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import io.github.jdetmar.pulumi.webflow.Asset;
import io.github.jdetmar.pulumi.webflow.AssetArgs;

import java.util.List;
import java.util.ArrayList;
import java.util.stream.Collectors;

/**
 * Asset Example - Uploading Files to Webflow.
 *
 * fileSource points at a local file (resolved relative to this program's
 * directory) or an http(s) URL. At apply time the provider reads the content,
 * computes its MD5 fileHash, registers the asset with Webflow and completes the
 * S3 upload for you. A content change of a local file replaces the asset.
 * uploadUrl / uploadDetails are secret outputs; folderId reports the folder
 * Webflow placed the asset in.
 *
 * Required token scopes: assets:read and assets:write.
 */
public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            // Get configuration values
            var config = ctx.config();
            var siteId = config.requireSecret("siteId");
            // Optional: an http(s) URL to upload as a second asset
            var heroImageUrl = config.get("heroImageUrl");

            // Example 1: Upload a local file shipped with this example
            var logoAsset = new Asset("company-logo",
                AssetArgs.builder()
                    .siteId(siteId)
                    .fileName("logo.svg")
                    .fileSource("./assets/logo.svg")
                    .build());

            // Example 2: Upload from a URL, optionally into a folder
            // Set `pulumi config set heroImageUrl https://.../hero.jpg` to enable it.
            Asset heroAsset = null;
            if (heroImageUrl.isPresent()) {
                heroAsset = new Asset("hero-image",
                    AssetArgs.builder()
                        .siteId(siteId)
                        .fileName("hero-banner.jpg")
                        .fileSource(heroImageUrl.get())
                        // .parentFolder("folder-id-here") // Uncomment to organize in an asset folder
                        .build());
            }

            // Example 3: Bulk upload of local files
            var icons = List.of(
                new String[]{"icon-home", "icon-home.svg", "./assets/icons/home.svg"},
                new String[]{"icon-user", "icon-user.svg", "./assets/icons/user.svg"}
            );

            var iconAssets = new ArrayList<Asset>();
            for (var icon : icons) {
                var asset = new Asset(icon[0],
                    AssetArgs.builder()
                        .siteId(siteId)
                        .fileName(icon[1])
                        .fileSource(icon[2])
                        .build());
                iconAssets.add(asset);
            }

            // Export values for the logo asset
            ctx.export("logoAssetId", logoAsset.assetId());
            ctx.export("logoHostedUrl", logoAsset.hostedUrl()); // public CDN URL of the uploaded file
            ctx.export("logoAssetUrl", logoAsset.assetUrl());
            ctx.export("logoFileHash", logoAsset.fileHash()); // computed from the file content
            ctx.export("logoContentType", logoAsset.contentType());
            ctx.export("logoSize", logoAsset.size());
            ctx.export("logoFolderId", logoAsset.folderId());

            // Export hero asset info (only when heroImageUrl is configured)
            if (heroAsset != null) {
                ctx.export("heroAssetId", heroAsset.assetId());
                ctx.export("heroHostedUrl", heroAsset.hostedUrl());
            }

            // Export icon asset IDs
            ctx.export("iconAssetIds", Output.all(iconAssets.stream()
                .map(Asset::assetId)
                .collect(Collectors.toList())));

            // Print deployment message
            int assetCount = icons.size() + 1 + (heroAsset != null ? 1 : 0);
            logoAsset.hostedUrl().applyValue(url -> {
                System.out.println(String.format("Uploaded %d assets. Logo is available at %s", assetCount, url));
                return null;
            });
        });
    }
}
