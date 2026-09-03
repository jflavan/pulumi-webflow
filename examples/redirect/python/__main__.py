# Copyright 2025, Justin Detmar.
# SPDX-License-Identifier: MIT

import pulumi
import pulumi_webflow as webflow

# Create a Pulumi config object
config = pulumi.Config()

# Get configuration values
site_id = config.require_secret("siteId")

"""
Redirect Example - Creating and Managing URL Redirects

This example demonstrates how to manage URL redirects for your Webflow sites.
Webflow redirects are always permanent (HTTP 301); the deprecated `status_code`
input is ignored by the provider, so it is simply omitted here.
"""

# Example 1: Content Move Redirect
permanent_redirect = webflow.Redirect("old-blog-to-new-blog",
    site_id=site_id,
    source_path="/blog/old-article",
    destination_path="/blog/articles/updated-article")

# Example 2: Campaign Redirect
# Point a short, memorable path at the current campaign landing page.
campaign_redirect = webflow.Redirect("campaign-landing-page",
    site_id=site_id,
    source_path="/old-campaign",
    destination_path="/new-campaign-2025")

# Example 3: External Redirect
external_redirect = webflow.Redirect("external-partner-link",
    site_id=site_id,
    source_path="/partner",
    destination_path="https://partner-site.com")

# Example 4: Bulk Redirects
bulk_redirects = []
redirect_mappings = [
    {"old": "/product-a", "new": "/products/product-a"},
    {"old": "/product-b", "new": "/products/product-b"},
    {"old": "/product-c", "new": "/products/product-c"},
]

for i, mapping in enumerate(redirect_mappings):
    redirect = webflow.Redirect(f"bulk-redirect-{i}",
        site_id=site_id,
        source_path=mapping["old"],
        destination_path=mapping["new"])
    bulk_redirects.append(redirect)

# Export values
pulumi.export("deployed_site_id", site_id)
pulumi.export("permanent_redirect_id", permanent_redirect.id)
pulumi.export("campaign_redirect_id", campaign_redirect.id)
pulumi.export("external_redirect_id", external_redirect.id)
pulumi.export("bulk_redirect_ids", [r.id for r in bulk_redirects])

# Print success message
redirect_count = len(bulk_redirects) + 3
site_id.apply(lambda s: print(f"✅ Deployed {redirect_count} redirects to site {s}"))
