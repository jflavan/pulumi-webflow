# Copyright 2025, Justin Detmar.
# SPDX-License-Identifier: MIT

import pulumi
import pulumi_webflow as webflow

# Create a Pulumi config object
config = pulumi.Config()

# Get configuration values
site_id = config.require_secret("siteId")
page_id = config.get("pageId")  # Optional: set to also read a single page

"""
Page Functions Example - Reading Page Information

This example demonstrates how to read page information from a Webflow site with
the `get_pages` and `get_page` functions. Pages cannot be created via the API -
they must be created in the Webflow Designer. The functions let you discover
page IDs and use page metadata in your infrastructure code.

Required token scope: pages:read.
"""

# Example 1: List all pages of the site
# get_pages_output follows API pagination and returns every page.
all_pages = webflow.get_pages_output(
    site_id=site_id,
    # locale_id="your-locale-id",  # optional: list a secondary locale instead
)

# Example 2: Read a single page by ID (only when pageId is configured)
specific_page = None
if page_id:
    specific_page = webflow.get_page_output(
        page_id=page_id,
        # locale_id="your-locale-id",     # optional secondary locale
        # translatable="your-locale-id",  # secondary locale ID: return its own translation, not inherited content
        #                                 # (400 for the primary locale, 403 if translation exclusions are disabled)
    )


# Export outputs for the listing
def transform_pages(pages):
    """Transform the page records into a readable format"""
    return [
        {
            "id": page.page_id,
            "title": page.title,
            "slug": page.slug,
            "published_path": page.published_path,
            "draft": page.draft,
            "archived": page.archived,
        }
        for page in pages
    ]


pulumi.export("site_pages", all_pages.pages.apply(transform_pages))
pulumi.export("page_count", all_pages.pages.apply(lambda pages: len(pages)))
pulumi.export("page_ids", all_pages.pages.apply(lambda pages: [p.page_id for p in pages]))

# Filter pages by their properties
pulumi.export("draft_page_slugs",
    all_pages.pages.apply(lambda pages: [p.slug for p in pages if p.draft]))
pulumi.export("collection_template_slugs",
    all_pages.pages.apply(lambda pages: [p.slug for p in pages if p.collection_id]))

# Export outputs for the single-page scenario (if configured)
if specific_page:
    pulumi.export("page_title", specific_page.title)
    pulumi.export("page_slug", specific_page.slug)
    pulumi.export("page_published_path", specific_page.published_path)
    pulumi.export("page_created_on", specific_page.created_on)
    pulumi.export("page_last_updated", specific_page.last_updated)
    pulumi.export("page_is_draft", specific_page.draft)
    pulumi.export("page_is_archived", specific_page.archived)
    pulumi.export("page_parent_id", specific_page.parent_id)
    pulumi.export("page_collection_id", specific_page.collection_id)
    pulumi.export("page_seo_title", specific_page.seo.title)
    pulumi.export("page_seo_description", specific_page.seo.description)
    pulumi.export("page_open_graph_title", specific_page.open_graph.title)


# Print helpful information
def print_pages_info(pages):
    print(f"\nFound {len(pages)} pages in the site")

    # Show a sample of pages
    sample_size = min(5, len(pages))
    if sample_size > 0:
        print(f"\nFirst {sample_size} pages:")
        for idx, page in enumerate(pages[:sample_size]):
            path = page.published_path or f"/{page.slug}"
            print(f"  {idx + 1}. \"{page.title}\" ({path}) id={page.page_id}")

        if len(pages) > sample_size:
            print(f"  ... and {len(pages) - sample_size} more")


all_pages.pages.apply(print_pages_info)

if specific_page:
    specific_page.title.apply(lambda title: print(f"\nRetrieved page: \"{title}\""))
