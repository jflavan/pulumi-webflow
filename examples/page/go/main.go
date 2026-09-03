package main

import (
	"fmt"

	"github.com/JDetmar/pulumi-webflow/sdk/go/webflow"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Page Functions Example - Reading Page Information
//
// This example demonstrates how to read page information from a Webflow site
// with the GetPages and GetPage functions. Pages cannot be created via the API -
// they must be created in the Webflow Designer. The functions let you discover
// page IDs and use page metadata in your infrastructure code.
//
// Required token scope: pages:read.
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		siteID := cfg.RequireSecret("siteId")
		pageID := cfg.Get("pageId") // Optional: set to also read a single page

		// Example 1: List all pages of the site
		// GetPagesOutput follows API pagination and returns every page.
		allPages := webflow.GetPagesOutput(ctx, webflow.GetPagesOutputArgs{
			SiteId: siteID,
			// LocaleId: pulumi.String("your-locale-id"), // optional: list a secondary locale instead
		})

		// Example 2: Read a single page by ID (only when pageId is configured)
		var specificPage *webflow.GetPageResultOutput
		if pageID != "" {
			page := webflow.GetPageOutput(ctx, webflow.GetPageOutputArgs{
				PageId: pulumi.String(pageID),
				// LocaleId:     pulumi.String("your-locale-id"), // optional secondary locale
				// Translatable: pulumi.String("your-locale-id"), // secondary locale ID: return its own translation
				//                                                // (400 for the primary locale, 403 if exclusions are disabled)
			})
			specificPage = &page
		}

		// Export outputs for the listing
		ctx.Export("sitePages", allPages.Pages().ApplyT(func(pages []webflow.PageRecord) interface{} {
			result := make([]map[string]interface{}, len(pages))
			for i, page := range pages {
				result[i] = map[string]interface{}{
					"id":            page.PageId,
					"title":         page.Title,
					"slug":          page.Slug,
					"publishedPath": page.PublishedPath,
					"draft":         page.Draft,
					"archived":      page.Archived,
				}
			}
			return result
		}))

		ctx.Export("pageCount", allPages.Pages().ApplyT(func(pages []webflow.PageRecord) int {
			return len(pages)
		}))

		ctx.Export("pageIds", allPages.Pages().ApplyT(func(pages []webflow.PageRecord) []string {
			ids := make([]string, len(pages))
			for i, page := range pages {
				ids[i] = page.PageId
			}
			return ids
		}))

		// Filter pages by their properties
		ctx.Export("draftPageSlugs", allPages.Pages().ApplyT(func(pages []webflow.PageRecord) []string {
			slugs := []string{}
			for _, page := range pages {
				if page.Draft {
					slugs = append(slugs, page.Slug)
				}
			}
			return slugs
		}))

		ctx.Export("collectionTemplateSlugs", allPages.Pages().ApplyT(func(pages []webflow.PageRecord) []string {
			slugs := []string{}
			for _, page := range pages {
				if page.CollectionId != "" {
					slugs = append(slugs, page.Slug)
				}
			}
			return slugs
		}))

		// Export outputs for the single-page scenario (if configured)
		if specificPage != nil {
			ctx.Export("pageTitle", specificPage.Title())
			ctx.Export("pageSlug", specificPage.Slug())
			ctx.Export("pagePublishedPath", specificPage.PublishedPath())
			ctx.Export("pageCreatedOn", specificPage.CreatedOn())
			ctx.Export("pageLastUpdated", specificPage.LastUpdated())
			ctx.Export("pageIsDraft", specificPage.Draft())
			ctx.Export("pageIsArchived", specificPage.Archived())
			ctx.Export("pageParentId", specificPage.ParentId())
			ctx.Export("pageCollectionId", specificPage.CollectionId())
			ctx.Export("pageSeoTitle", specificPage.Seo().Title())
			ctx.Export("pageSeoDescription", specificPage.Seo().Description())
			ctx.Export("pageOpenGraphTitle", specificPage.OpenGraph().Title())
		}

		// Print helpful information
		allPages.Pages().ApplyT(func(pages []webflow.PageRecord) interface{} {
			ctx.Log.Info(fmt.Sprintf("\nFound %d pages in the site", len(pages)), nil)

			// Show a sample of pages
			sampleSize := len(pages)
			if sampleSize > 5 {
				sampleSize = 5
			}

			if sampleSize > 0 {
				ctx.Log.Info(fmt.Sprintf("\nFirst %d pages:", sampleSize), nil)
				for i := 0; i < sampleSize; i++ {
					path := pages[i].PublishedPath
					if path == "" {
						path = "/" + pages[i].Slug
					}
					ctx.Log.Info(fmt.Sprintf("  %d. \"%s\" (%s) id=%s", i+1, pages[i].Title, path, pages[i].PageId), nil)
				}

				if len(pages) > sampleSize {
					ctx.Log.Info(fmt.Sprintf("  ... and %d more", len(pages)-sampleSize), nil)
				}
			}
			return nil
		})

		if specificPage != nil {
			specificPage.Title().ApplyT(func(title string) interface{} {
				ctx.Log.Info(fmt.Sprintf("\nRetrieved page: \"%s\"", title), nil)
				return nil
			})
		}

		return nil
	})
}
