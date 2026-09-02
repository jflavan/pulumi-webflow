// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// PageSEORecord is the SEO block of a page as returned by the page functions.
type PageSEORecord struct {
	// Title is the SEO title.
	Title string `pulumi:"title"`
	// Description is the SEO meta description.
	Description string `pulumi:"description"`
}

// PageOpenGraphRecord is the Open Graph block of a page as returned by the page functions.
type PageOpenGraphRecord struct {
	// Title is the Open Graph title.
	Title string `pulumi:"title"`
	// TitleCopied indicates the Open Graph title is copied from the SEO title.
	TitleCopied bool `pulumi:"titleCopied"`
	// Description is the Open Graph description.
	Description string `pulumi:"description"`
	// DescriptionCopied indicates the Open Graph description is copied from the SEO description.
	DescriptionCopied bool `pulumi:"descriptionCopied"`
}

// PageRecord is the metadata of a single Webflow page as returned by getPage and getPages.
type PageRecord struct {
	// PageID is the Webflow page ID.
	PageID string `pulumi:"pageId"`
	// SiteID is the Webflow site ID this page belongs to.
	SiteID string `pulumi:"siteId"`
	// Title is the page title.
	Title string `pulumi:"title"`
	// Slug is the URL slug for the page.
	Slug string `pulumi:"slug"`
	// ParentID is the ID of the parent folder (empty when at the root).
	ParentID string `pulumi:"parentId"`
	// CollectionID is the ID of the CMS collection for collection template pages (empty otherwise).
	CollectionID string `pulumi:"collectionId"`
	// CreatedOn is the timestamp when the page was created.
	CreatedOn string `pulumi:"createdOn"`
	// LastUpdated is the timestamp when the page was last updated.
	LastUpdated string `pulumi:"lastUpdated"`
	// Archived indicates if the page is archived.
	Archived bool `pulumi:"archived"`
	// Draft indicates if the page is in draft mode.
	Draft bool `pulumi:"draft"`
	// CanBranch indicates if the page can be branched.
	CanBranch bool `pulumi:"canBranch"`
	// IsBranch indicates if the page is a branch of another page.
	IsBranch bool `pulumi:"isBranch"`
	// BranchID is the ID of the parent branch, if any.
	BranchID string `pulumi:"branchId"`
	// LocaleID is the locale of the returned page data (empty for the primary locale).
	LocaleID string `pulumi:"localeId"`
	// PublishedPath is the relative URL path of the published page.
	PublishedPath string `pulumi:"publishedPath"`
	// SEO holds the SEO title and description.
	SEO PageSEORecord `pulumi:"seo"`
	// OpenGraph holds the Open Graph settings.
	OpenGraph PageOpenGraphRecord `pulumi:"openGraph"`
}

// Annotate adds descriptions to the PageRecord fields.
func (r *PageRecord) Annotate(a infer.Annotator) {
	a.Describe(&r.PageID, "The Webflow page ID.")
	a.Describe(&r.SiteID, "The Webflow site ID this page belongs to.")
	a.Describe(&r.Title, "The page title shown in browser tabs and search results.")
	a.Describe(&r.Slug, "The URL slug of the page (e.g., 'about' for '/about').")
	a.Describe(&r.ParentID, "The ID of the parent folder, or empty when the page is at the root.")
	a.Describe(&r.CollectionID, "The CMS collection ID for collection template pages, or empty.")
	a.Describe(&r.CreatedOn, "The timestamp when the page was created (RFC3339 format).")
	a.Describe(&r.LastUpdated, "The timestamp when the page was last updated (RFC3339 format).")
	a.Describe(&r.Archived, "Whether the page is archived.")
	a.Describe(&r.Draft, "Whether the page is a draft.")
	a.Describe(&r.CanBranch, "Whether the page can be branched.")
	a.Describe(&r.IsBranch, "Whether the page is a branch of another page.")
	a.Describe(&r.BranchID, "The ID of the parent branch, or empty.")
	a.Describe(&r.LocaleID, "The locale ID of the returned page data, or empty for the primary locale.")
	a.Describe(&r.PublishedPath, "The relative URL path of the published page.")
	a.Describe(&r.SEO, "The SEO title and description of the page.")
	a.Describe(&r.OpenGraph, "The Open Graph settings of the page.")
}

// Annotate adds descriptions to the PageSEORecord fields.
func (r *PageSEORecord) Annotate(a infer.Annotator) {
	a.Describe(&r.Title, "The SEO title of the page.")
	a.Describe(&r.Description, "The SEO meta description of the page.")
}

// Annotate adds descriptions to the PageOpenGraphRecord fields.
func (r *PageOpenGraphRecord) Annotate(a infer.Annotator) {
	a.Describe(&r.Title, "The Open Graph title of the page.")
	a.Describe(&r.TitleCopied, "Whether the Open Graph title is copied from the SEO title.")
	a.Describe(&r.Description, "The Open Graph description of the page.")
	a.Describe(&r.DescriptionCopied, "Whether the Open Graph description is copied from the SEO description.")
}

// pageRecordFromAPI converts an API page into the function output shape.
func pageRecordFromAPI(page *Page) PageRecord {
	rec := PageRecord{
		PageID:        page.ID,
		SiteID:        page.SiteID,
		Title:         page.Title,
		Slug:          page.Slug,
		ParentID:      page.ParentID,
		CollectionID:  page.CollectionID,
		CreatedOn:     page.CreatedOn,
		LastUpdated:   page.LastUpdated,
		Archived:      page.Archived,
		Draft:         page.Draft,
		CanBranch:     page.CanBranch,
		IsBranch:      page.IsBranch,
		BranchID:      page.BranchID,
		LocaleID:      page.LocaleID,
		PublishedPath: page.PublishedPath,
	}
	if page.SEO != nil {
		rec.SEO = PageSEORecord{Title: page.SEO.Title, Description: page.SEO.Description}
	}
	if page.OpenGraph != nil {
		rec.OpenGraph = PageOpenGraphRecord{
			Title:             page.OpenGraph.Title,
			TitleCopied:       page.OpenGraph.TitleCopied,
			Description:       page.OpenGraph.Description,
			DescriptionCopied: page.OpenGraph.DescriptionCopied,
		}
	}
	return rec
}

// GetPage is a Pulumi function that reads the metadata of a single Webflow page.
// Pages cannot be created via the API; use this to look up pages built in the Designer.
type GetPage struct{}

// GetPageInput defines the input parameters for the GetPage function.
type GetPageInput struct {
	// PageID is the Webflow page ID to retrieve.
	PageID string `pulumi:"pageId"`
	// LocaleID optionally selects the locale to read.
	LocaleID string `pulumi:"localeId,optional"`
	// Translatable is the ID of the secondary locale being translated into; it returns the
	// page's translatable content for that locale.
	Translatable string `pulumi:"translatable,optional"`
}

// GetPageOutput defines the output of the GetPage function.
type GetPageOutput struct {
	PageRecord
}

// Annotate adds descriptions to the GetPage function.
func (f *GetPage) Annotate(a infer.Annotator) {
	a.Describe(f, "Reads the metadata (title, slug, SEO, Open Graph, timestamps, flags) of a single "+
		"Webflow page. Pages cannot be created via the API; they are built in the Webflow Designer. "+
		"Requires the pages:read scope.")
}

// Annotate adds descriptions to the GetPageInput fields.
func (i *GetPageInput) Annotate(a infer.Annotator) {
	a.Describe(&i.PageID, "The Webflow page ID (24-character lowercase hexadecimal string).")
	a.Describe(&i.LocaleID, "Optional locale ID. When omitted the primary locale is returned.")
	a.Describe(&i.Translatable, "Optional ID of the secondary locale you are translating into "+
		"(24-character lowercase hexadecimal string), sent verbatim as ?translatable=<localeId> to return the "+
		"page's translatable content for that locale. Webflow returns a 400 error when this is the primary "+
		"locale ID or any other value, and a 403 error when translation exclusions are not enabled for the site.")
}

// Invoke implements infer.Fn for GetPage.
func (f *GetPage) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetPageInput],
) (infer.FunctionResponse[GetPageOutput], error) {
	if err := ValidatePageID(req.Input.PageID); err != nil {
		return infer.FunctionResponse[GetPageOutput]{}, fmt.Errorf("validation failed for getPage: %w", err)
	}
	if err := ValidateLocaleID(req.Input.LocaleID); err != nil {
		return infer.FunctionResponse[GetPageOutput]{}, fmt.Errorf("validation failed for getPage: %w", err)
	}
	if err := ValidateLocaleID(req.Input.Translatable); err != nil {
		return infer.FunctionResponse[GetPageOutput]{}, fmt.Errorf(
			"validation failed for getPage: translatable must be the ID of a secondary locale: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.FunctionResponse[GetPageOutput]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	page, err := GetPageMetadata(ctx, client, req.Input.PageID, req.Input.LocaleID, req.Input.Translatable)
	if err != nil {
		if req.Input.Translatable != "" {
			var apiErr *APIError
			if errors.As(err, &apiErr) {
				switch apiErr.StatusCode {
				case http.StatusBadRequest:
					return infer.FunctionResponse[GetPageOutput]{}, fmt.Errorf("failed to get page: Webflow rejected "+
						"translatable='%s' (HTTP 400). translatable must be the ID of a secondary locale of the site; "+
						"the primary locale ID or any other value is rejected: %w", req.Input.Translatable, err)
				case http.StatusForbidden:
					return infer.FunctionResponse[GetPageOutput]{}, fmt.Errorf("failed to get page: Webflow refused the "+
						"translatable request (HTTP 403). Translation exclusions must be enabled for the site "+
						"(Site Settings > Localization) before translatable content can be read: %w", err)
				}
			}
		}
		return infer.FunctionResponse[GetPageOutput]{}, fmt.Errorf("failed to get page: %w", err)
	}

	return infer.FunctionResponse[GetPageOutput]{Output: GetPageOutput{PageRecord: pageRecordFromAPI(page)}}, nil
}

// GetPages is a Pulumi function that lists every page of a Webflow site.
type GetPages struct{}

// GetPagesInput defines the input parameters for the GetPages function.
type GetPagesInput struct {
	// SiteID is the Webflow site ID whose pages are listed.
	SiteID string `pulumi:"siteId"`
	// LocaleID optionally selects the locale to read.
	LocaleID string `pulumi:"localeId,optional"`
}

// GetPagesOutput defines the output of the GetPages function.
type GetPagesOutput struct {
	// SiteID echoes the site ID that was listed.
	SiteID string `pulumi:"siteId"`
	// Pages is the complete list of pages in the site (pagination is followed).
	Pages []PageRecord `pulumi:"pages"`
}

// Annotate adds descriptions to the GetPages function.
func (f *GetPages) Annotate(a infer.Annotator) {
	a.Describe(f, "Lists all pages of a Webflow site with their metadata, following API pagination. "+
		"Useful for discovering page IDs for the PageMetadata, PageContent and PageCustomCode resources. "+
		"Requires the pages:read scope.")
}

// Annotate adds descriptions to the GetPagesInput fields.
func (i *GetPagesInput) Annotate(a infer.Annotator) {
	a.Describe(&i.SiteID, "The Webflow site ID (24-character lowercase hexadecimal string).")
	a.Describe(&i.LocaleID, "Optional locale ID. When omitted the primary locale is returned.")
}

// Annotate adds descriptions to the GetPagesOutput fields.
func (o *GetPagesOutput) Annotate(a infer.Annotator) {
	a.Describe(&o.SiteID, "The site ID that was listed.")
	a.Describe(&o.Pages, "All pages of the site.")
}

// Invoke implements infer.Fn for GetPages.
func (f *GetPages) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetPagesInput],
) (infer.FunctionResponse[GetPagesOutput], error) {
	if err := ValidateSiteID(req.Input.SiteID); err != nil {
		return infer.FunctionResponse[GetPagesOutput]{}, fmt.Errorf("validation failed for getPages: %w", err)
	}
	if err := ValidateLocaleID(req.Input.LocaleID); err != nil {
		return infer.FunctionResponse[GetPagesOutput]{}, fmt.Errorf("validation failed for getPages: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.FunctionResponse[GetPagesOutput]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	pages, err := ListPages(ctx, client, req.Input.SiteID, req.Input.LocaleID)
	if err != nil {
		return infer.FunctionResponse[GetPagesOutput]{}, fmt.Errorf("failed to list pages: %w", err)
	}

	records := make([]PageRecord, len(pages))
	for i := range pages {
		records[i] = pageRecordFromAPI(&pages[i])
	}

	return infer.FunctionResponse[GetPagesOutput]{
		Output: GetPagesOutput{SiteID: req.Input.SiteID, Pages: records},
	}, nil
}
