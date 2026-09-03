# Copyright 2025, Justin Detmar.
# SPDX-License-Identifier: MIT

import pulumi
import pulumi_webflow as webflow

# Create a Pulumi config object
config = pulumi.Config()

# Get configuration values
site_id = config.require_secret("siteId")
# Optional: an http(s) URL to upload as a second asset
hero_image_url = config.get("heroImageUrl")

"""
Asset Example - Uploading Files to Webflow

This example demonstrates how to upload assets with the `webflow.Asset`
resource. `file_source` points at a local file (resolved relative to this
program's directory) or an http(s) URL. At apply time the provider reads the
content, computes its MD5 `file_hash`, registers the asset with Webflow and
completes the S3 upload for you.

- A content change of a local file (different hash) replaces the asset.
- `upload_url` / `upload_details` are secret outputs; you no longer need them.
- `folder_id` reports the folder Webflow placed the asset in.

Required token scopes: assets:read and assets:write.
"""

# Example 1: Upload a local file shipped with this example
logo_asset = webflow.Asset("company-logo",
    site_id=site_id,
    file_name="logo.svg",
    file_source="./assets/logo.svg")

# Example 2: Upload from a URL, optionally into a folder
# Set `pulumi config set heroImageUrl https://.../hero.jpg` to enable it.
hero_asset = None
if hero_image_url:
    hero_asset = webflow.Asset("hero-image",
        site_id=site_id,
        file_name="hero-banner.jpg",
        file_source=hero_image_url)
        # parent_folder="folder-id-here"  # Uncomment to organize in an asset folder

# Example 3: Bulk upload of local files
icon_assets = []
icons = [
    {"name": "icon-home", "file_name": "icon-home.svg", "file_source": "./assets/icons/home.svg"},
    {"name": "icon-user", "file_name": "icon-user.svg", "file_source": "./assets/icons/user.svg"},
]

for icon in icons:
    asset = webflow.Asset(icon["name"],
        site_id=site_id,
        file_name=icon["file_name"],
        file_source=icon["file_source"])
    icon_assets.append(asset)

# Export values for the logo asset
pulumi.export("logo_asset_id", logo_asset.asset_id)
pulumi.export("logo_hosted_url", logo_asset.hosted_url)  # public CDN URL of the uploaded file
pulumi.export("logo_asset_url", logo_asset.asset_url)
pulumi.export("logo_file_hash", logo_asset.file_hash)  # computed from the file content
pulumi.export("logo_content_type", logo_asset.content_type)
pulumi.export("logo_size", logo_asset.size)
pulumi.export("logo_folder_id", logo_asset.folder_id)

# Export hero asset info (only when heroImageUrl is configured)
if hero_asset:
    pulumi.export("hero_asset_id", hero_asset.asset_id)
    pulumi.export("hero_hosted_url", hero_asset.hosted_url)

# Export icon asset IDs
pulumi.export("icon_asset_ids", [a.asset_id for a in icon_assets])

# Print success message
asset_count = len(icons) + 1 + (1 if hero_asset else 0)
logo_asset.hosted_url.apply(lambda url: print(f"Uploaded {asset_count} assets. Logo is available at {url}"))
