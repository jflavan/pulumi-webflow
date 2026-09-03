// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT

using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Pulumi;
using Community.Pulumi.Webflow;
// Pulumi also defines an Asset type; alias the Webflow resource to avoid the ambiguity.
using WebflowAsset = Community.Pulumi.Webflow.Asset;

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
class Program
{
    static Task<int> Main() => Deployment.RunAsync(() =>
    {
        // Create a Pulumi config object
        var config = new Pulumi.Config();

        // Get configuration values
        var siteId = config.RequireSecret("siteId");
        // Optional: an http(s) URL to upload as a second asset
        var heroImageUrl = config.Get("heroImageUrl");

        // Example 1: Upload a local file shipped with this example
        var logoAsset = new WebflowAsset("company-logo", new AssetArgs
        {
            SiteId = siteId,
            FileName = "logo.svg",
            FileSource = "./assets/logo.svg",
        });

        // Example 2: Upload from a URL, optionally into a folder
        // Set `pulumi config set heroImageUrl https://.../hero.jpg` to enable it.
        WebflowAsset? heroAsset = null;
        if (!string.IsNullOrEmpty(heroImageUrl))
        {
            heroAsset = new WebflowAsset("hero-image", new AssetArgs
            {
                SiteId = siteId,
                FileName = "hero-banner.jpg",
                FileSource = heroImageUrl,
                // ParentFolder = "folder-id-here", // Uncomment to organize in an asset folder
            });
        }

        // Example 3: Bulk upload of local files
        var iconAssets = new List<WebflowAsset>();
        var icons = new[]
        {
            new { Name = "icon-home", FileName = "icon-home.svg", FileSource = "./assets/icons/home.svg" },
            new { Name = "icon-user", FileName = "icon-user.svg", FileSource = "./assets/icons/user.svg" },
        };

        foreach (var icon in icons)
        {
            var asset = new WebflowAsset(icon.Name, new AssetArgs
            {
                SiteId = siteId,
                FileName = icon.FileName,
                FileSource = icon.FileSource,
            });
            iconAssets.Add(asset);
        }

        // Export values for the logo asset
        var outputs = new Dictionary<string, object?>
        {
            ["logoAssetId"] = logoAsset.AssetId,
            ["logoHostedUrl"] = logoAsset.HostedUrl, // public CDN URL of the uploaded file
            ["logoAssetUrl"] = logoAsset.AssetUrl,
            ["logoFileHash"] = logoAsset.FileHash, // computed from the file content
            ["logoContentType"] = logoAsset.ContentType,
            ["logoSize"] = logoAsset.Size,
            ["logoFolderId"] = logoAsset.FolderId,

            // Export icon asset IDs
            ["iconAssetIds"] = Output.All(iconAssets.Select(a => a.AssetId).ToArray()),
        };

        // Export hero asset info (only when heroImageUrl is configured)
        if (heroAsset != null)
        {
            outputs["heroAssetId"] = heroAsset.AssetId;
            outputs["heroHostedUrl"] = heroAsset.HostedUrl;
        }

        // Print deployment message
        var assetCount = icons.Length + 1 + (heroAsset != null ? 1 : 0);
        logoAsset.HostedUrl.Apply(url =>
        {
            Console.WriteLine($"Uploaded {assetCount} assets. Logo is available at {url}");
            return url;
        });

        return outputs;
    });
}
