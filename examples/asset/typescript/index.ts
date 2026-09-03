// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Create a Pulumi config object
const config = new pulumi.Config();

// Get configuration values
const siteId = config.requireSecret("siteId");
// Optional: an http(s) URL to upload as a second asset
const heroImageUrl = config.get("heroImageUrl");

/**
 * Asset Example - Uploading Files to Webflow
 *
 * This example demonstrates how to upload assets with the `webflow.Asset`
 * resource. `fileSource` points at a local file (resolved relative to this
 * program's directory) or an http(s) URL. At apply time the provider reads the
 * content, computes its MD5 `fileHash`, registers the asset with Webflow and
 * completes the S3 upload for you.
 *
 * - A content change of a local file (different hash) replaces the asset.
 * - `uploadUrl` / `uploadDetails` are secret outputs; you no longer need them.
 * - `folderId` reports the folder Webflow placed the asset in.
 *
 * Required token scopes: assets:read and assets:write.
 */

// Example 1: Upload a local file shipped with this example
const logoAsset = new webflow.Asset("company-logo", {
  siteId: siteId,
  fileName: "logo.svg",
  fileSource: "./assets/logo.svg",
});

// Example 2: Upload from a URL, optionally into a folder
// Set `pulumi config set heroImageUrl https://.../hero.jpg` to enable it.
let heroAsset: webflow.Asset | undefined;
if (heroImageUrl) {
  heroAsset = new webflow.Asset("hero-image", {
    siteId: siteId,
    fileName: "hero-banner.jpg",
    fileSource: heroImageUrl,
    // parentFolder: "folder-id-here", // Uncomment to organize in an asset folder
  });
}

// Example 3: Bulk upload of local files
const iconAssets: webflow.Asset[] = [];
const icons = [
  { name: "icon-home", fileName: "icon-home.svg", fileSource: "./assets/icons/home.svg" },
  { name: "icon-user", fileName: "icon-user.svg", fileSource: "./assets/icons/user.svg" },
];

icons.forEach((icon) => {
  const asset = new webflow.Asset(icon.name, {
    siteId: siteId,
    fileName: icon.fileName,
    fileSource: icon.fileSource,
  });
  iconAssets.push(asset);
});

// Export values for the logo asset
export const logoAssetId = logoAsset.assetId;
export const logoHostedUrl = logoAsset.hostedUrl; // public CDN URL of the uploaded file
export const logoAssetUrl = logoAsset.assetUrl;
export const logoFileHash = logoAsset.fileHash; // computed from the file content
export const logoContentType = logoAsset.contentType;
export const logoSize = logoAsset.size;
export const logoFolderId = logoAsset.folderId;

// Export hero asset info (undefined when heroImageUrl is not configured)
export const heroAssetId = heroAsset?.assetId;
export const heroHostedUrl = heroAsset?.hostedUrl;

// Export icon asset IDs
export const iconAssetIds = iconAssets.map((a) => a.assetId);

// Print deployment message
const assetCount = icons.length + 1 + (heroAsset ? 1 : 0);
logoAsset.hostedUrl.apply((url) => {
  console.log(`Uploaded ${assetCount} assets. Logo is available at ${url}`);
});
