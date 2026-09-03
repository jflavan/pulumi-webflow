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
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// PageMetadata is the resource controller that manages the settings of an existing Webflow page
// (title, slug, SEO and Open Graph). Pages themselves are created in the Webflow Designer.
type PageMetadata struct{}

var _ infer.CustomCheck[PageMetadataArgs] = (*PageMetadata)(nil)

// Check validates the known pageId and localeId formats at preview time. Unknown values, and
// the "at least one managed field" rule, are validated at apply time.
func (r *PageMetadata) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[PageMetadataArgs], error) {
	inputs, failures, err := checkStrings[PageMetadataArgs](ctx, req.NewInputs,
		stringValidator{property: "pageId", validate: ValidatePageID},
		stringValidator{property: "localeId", validate: ValidateLocaleID},
	)
	return infer.CheckResponse[PageMetadataArgs]{Inputs: inputs, Failures: failures}, err
}

// PageSEOArgs configures the SEO settings of a page.
type PageSEOArgs struct {
	// Title is the SEO title. Empty means "not managed".
	Title string `pulumi:"title,optional"`
	// Description is the SEO meta description. Empty means "not managed".
	Description string `pulumi:"description,optional"`
}

// PageOpenGraphArgs configures the Open Graph settings of a page.
type PageOpenGraphArgs struct {
	// Title is the Open Graph title. Empty means "not managed".
	Title string `pulumi:"title,optional"`
	// TitleCopied copies the SEO title into the Open Graph title. Unset means "not managed".
	TitleCopied *bool `pulumi:"titleCopied,optional"`
	// Description is the Open Graph description. Empty means "not managed".
	Description string `pulumi:"description,optional"`
	// DescriptionCopied copies the SEO description into the Open Graph description. Unset means "not managed".
	DescriptionCopied *bool `pulumi:"descriptionCopied,optional"`
}

// PageMetadataArgs defines the input properties for the PageMetadata resource.
type PageMetadataArgs struct {
	// PageID is the Webflow page whose settings are managed. Changing it replaces the resource.
	PageID string `pulumi:"pageId"`
	// LocaleID optionally targets a secondary locale. Changing it replaces the resource.
	LocaleID string `pulumi:"localeId,optional"`
	// Title is the page title. Empty means "not managed".
	Title string `pulumi:"title,optional"`
	// Slug is the URL slug. Empty means "not managed".
	Slug string `pulumi:"slug,optional"`
	// SEO holds the SEO settings to manage.
	SEO *PageSEOArgs `pulumi:"seo,optional"`
	// OpenGraph holds the Open Graph settings to manage.
	OpenGraph *PageOpenGraphArgs `pulumi:"openGraph,optional"`
}

// PageMetadataState defines the output properties for the PageMetadata resource.
type PageMetadataState struct {
	PageMetadataArgs
	// SiteID is the site the page belongs to (read-only).
	SiteID string `pulumi:"siteId,optional"`
	// CurrentSlug is the slug Webflow reports after the update (read-only). It differs from slug
	// when Webflow ignored the requested slug (home, collection template and utility pages, or
	// secondary locales without an Advanced/Enterprise localization plan).
	CurrentSlug string `pulumi:"currentSlug,optional"`
	// PublishedPath is the relative URL path of the published page (read-only).
	PublishedPath string `pulumi:"publishedPath,optional"`
	// ParentID is the parent folder ID (read-only).
	ParentID string `pulumi:"parentId,optional"`
	// CollectionID is the CMS collection ID for collection template pages (read-only).
	CollectionID string `pulumi:"collectionId,optional"`
	// Archived indicates if the page is archived (read-only).
	Archived bool `pulumi:"archived,optional"`
	// Draft indicates if the page is a draft (read-only).
	Draft bool `pulumi:"draft,optional"`
	// CreatedOn is when the page was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
	// LastUpdated is when the page was last updated (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
}

// Annotate adds descriptions and constraints to the PageMetadata resource.
func (r *PageMetadata) Annotate(a infer.Annotator) {
	a.SetToken("index", "PageMetadata")
	a.Describe(r, "Manages the settings of an existing Webflow page: title, slug, SEO title/description "+
		"and Open Graph settings. Pages are created in the Webflow Designer; use the getPages function to "+
		"find page IDs. Only the fields you set are sent to Webflow (PUT /v2/pages/{page_id}). "+
		"Webflow silently ignores slug for the home page, collection template pages, utility pages "+
		"(404, password, search) and for secondary locales without an Advanced/Enterprise localization plan; "+
		"the provider warns when the returned slug differs from the requested one. "+
		"Destroying this resource only removes it from Pulumi state; the page keeps its current settings.")
}

// Annotate adds descriptions to the PageMetadataArgs fields.
func (args *PageMetadataArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.PageID, "The Webflow page ID (24-character lowercase hexadecimal string). "+
		"Changing it replaces the resource.")
	a.Describe(&args.LocaleID, "Optional locale ID to update a secondary locale's settings. "+
		"When omitted the primary locale is updated. Changing it replaces the resource.")
	a.Describe(&args.Title, "The page title. Leave unset to keep the value managed in the Designer.")
	a.Describe(&args.Slug, "The URL slug. Leave unset to keep the value managed in the Designer. "+
		"Ignored by Webflow for home, collection template and utility pages.")
	a.Describe(&args.SEO, "SEO title and description. Only the fields you set are updated.")
	a.Describe(&args.OpenGraph, "Open Graph title, description and copied flags. Only the fields you set are updated.")
}

// Annotate adds descriptions to the PageSEOArgs fields.
func (args *PageSEOArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Title, "The SEO title of the page.")
	a.Describe(&args.Description, "The SEO meta description of the page.")
}

// Annotate adds descriptions to the PageOpenGraphArgs fields.
func (args *PageOpenGraphArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Title, "The Open Graph title of the page.")
	a.Describe(&args.TitleCopied, "Whether the Open Graph title is copied from the SEO title.")
	a.Describe(&args.Description, "The Open Graph description of the page.")
	a.Describe(&args.DescriptionCopied, "Whether the Open Graph description is copied from the SEO description.")
}

// Annotate adds descriptions to the PageMetadataState fields.
func (state *PageMetadataState) Annotate(a infer.Annotator) {
	a.Describe(&state.SiteID, "The site the page belongs to (read-only).")
	a.Describe(&state.CurrentSlug, "The slug Webflow reports for the page (read-only). "+
		"Differs from slug when Webflow ignored the requested value.")
	a.Describe(&state.PublishedPath, "The relative URL path of the published page (read-only).")
	a.Describe(&state.ParentID, "The parent folder ID (read-only).")
	a.Describe(&state.CollectionID, "The CMS collection ID for collection template pages (read-only).")
	a.Describe(&state.Archived, "Whether the page is archived (read-only).")
	a.Describe(&state.Draft, "Whether the page is a draft (read-only).")
	a.Describe(&state.CreatedOn, "When the page was created (RFC3339 format, read-only).")
	a.Describe(&state.LastUpdated, "When the page was last updated (RFC3339 format, read-only).")
}

// GeneratePageMetadataResourceID builds the resource ID: {pageId}/metadata or {pageId}/metadata/{localeId}.
func GeneratePageMetadataResourceID(pageID, localeID string) string {
	if localeID != "" {
		return pageID + "/metadata/" + localeID
	}
	return pageID + "/metadata"
}

// ExtractIDsFromPageMetadataResourceID parses {pageId}/metadata[/{localeId}].
func ExtractIDsFromPageMetadataResourceID(resourceID string) (pageID, localeID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}
	parts := strings.Split(resourceID, "/")
	if (len(parts) != 2 && len(parts) != 3) || parts[1] != "metadata" || parts[0] == "" {
		return "", "", fmt.Errorf(
			"invalid resource ID format: expected {pageId}/metadata or {pageId}/metadata/{localeId}, got: %s", resourceID)
	}
	if len(parts) == 3 {
		if parts[2] == "" {
			return "", "", fmt.Errorf(
				"invalid resource ID format: expected {pageId}/metadata/{localeId}, got: %s", resourceID)
		}
		localeID = parts[2]
	}
	return parts[0], localeID, nil
}

// pageMetadataHasManagedField reports whether at least one updatable field is set.
func pageMetadataHasManagedField(args PageMetadataArgs) bool {
	if args.Title != "" || args.Slug != "" {
		return true
	}
	if args.SEO != nil && (args.SEO.Title != "" || args.SEO.Description != "") {
		return true
	}
	if args.OpenGraph != nil && (args.OpenGraph.Title != "" || args.OpenGraph.Description != "" ||
		args.OpenGraph.TitleCopied != nil || args.OpenGraph.DescriptionCopied != nil) {
		return true
	}
	return false
}

// pageMetadataUpdateRequest builds the PUT body containing only the fields the user set.
func pageMetadataUpdateRequest(args PageMetadataArgs) PageMetadataUpdateRequest {
	body := PageMetadataUpdateRequest{}
	if args.Title != "" {
		body.Title = ptr(args.Title)
	}
	if args.Slug != "" {
		body.Slug = ptr(args.Slug)
	}
	if args.SEO != nil && (args.SEO.Title != "" || args.SEO.Description != "") {
		body.SEO = &PageSEOUpdate{}
		if args.SEO.Title != "" {
			body.SEO.Title = ptr(args.SEO.Title)
		}
		if args.SEO.Description != "" {
			body.SEO.Description = ptr(args.SEO.Description)
		}
	}
	if args.OpenGraph != nil {
		og := &PageOpenGraphUpdate{}
		if args.OpenGraph.Title != "" {
			og.Title = ptr(args.OpenGraph.Title)
		}
		if args.OpenGraph.Description != "" {
			og.Description = ptr(args.OpenGraph.Description)
		}
		og.TitleCopied = args.OpenGraph.TitleCopied
		og.DescriptionCopied = args.OpenGraph.DescriptionCopied
		if og.Title != nil || og.Description != nil || og.TitleCopied != nil || og.DescriptionCopied != nil {
			body.OpenGraph = og
		}
	}
	return body
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// boolPtrEqual compares two optional booleans.
func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// pageMetadataSEOEqual compares two optional SEO blocks, treating nil and empty as equal.
func pageMetadataSEOEqual(a, b *PageSEOArgs) bool {
	if a == nil {
		a = &PageSEOArgs{}
	}
	if b == nil {
		b = &PageSEOArgs{}
	}
	return *a == *b
}

// pageMetadataOpenGraphEqual compares two optional Open Graph blocks, treating nil and empty as equal.
func pageMetadataOpenGraphEqual(a, b *PageOpenGraphArgs) bool {
	if a == nil {
		a = &PageOpenGraphArgs{}
	}
	if b == nil {
		b = &PageOpenGraphArgs{}
	}
	return a.Title == b.Title && a.Description == b.Description &&
		boolPtrEqual(a.TitleCopied, b.TitleCopied) && boolPtrEqual(a.DescriptionCopied, b.DescriptionCopied)
}

// validatePageMetadataArgs validates fully-resolved inputs at apply time.
func validatePageMetadataArgs(args PageMetadataArgs) error {
	if err := ValidatePageID(args.PageID); err != nil {
		return fmt.Errorf("validation failed for PageMetadata resource: %w", err)
	}
	if err := ValidateLocaleID(args.LocaleID); err != nil {
		return fmt.Errorf("validation failed for PageMetadata resource: %w", err)
	}
	if !pageMetadataHasManagedField(args) {
		return errors.New("validation failed for PageMetadata resource: set at least one of title, slug, " +
			"seo.title, seo.description, openGraph.title, openGraph.titleCopied, openGraph.description or " +
			"openGraph.descriptionCopied")
	}
	return nil
}

// applyPageMetadata sends the PUT and folds the response into state, warning when slug was ignored.
func (r *PageMetadata) applyPageMetadata(ctx context.Context, args PageMetadataArgs) (PageMetadataState, error) {
	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return PageMetadataState{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	page, err := PutPageMetadata(ctx, client, args.PageID, args.LocaleID, pageMetadataUpdateRequest(args))
	if err != nil {
		return PageMetadataState{}, fmt.Errorf("failed to update page settings: %w", err)
	}

	if args.Slug != "" && page.Slug != "" && page.Slug != args.Slug {
		NewLogContext(ctx).
			WithField("pageId", args.PageID).
			WithField("requestedSlug", args.Slug).
			WithField("actualSlug", page.Slug).
			Warn("Webflow ignored the requested slug. Slugs cannot be changed on the home page, " +
				"collection template pages, utility pages (404, password, search), or on secondary locales " +
				"without an Advanced/Enterprise localization plan. Remove slug from this resource to stop " +
				"requesting the change")
	}

	return pageMetadataStateFromAPI(args, page), nil
}

// pageMetadataStateFromAPI builds state from the managed inputs and the API page.
func pageMetadataStateFromAPI(args PageMetadataArgs, page *Page) PageMetadataState {
	return PageMetadataState{
		PageMetadataArgs: args,
		SiteID:           page.SiteID,
		CurrentSlug:      page.Slug,
		PublishedPath:    page.PublishedPath,
		ParentID:         page.ParentID,
		CollectionID:     page.CollectionID,
		Archived:         page.Archived,
		Draft:            page.Draft,
		CreatedOn:        page.CreatedOn,
		LastUpdated:      page.LastUpdated,
	}
}

// Diff replaces on pageId/localeId changes and updates in place for every other field.
func (r *PageMetadata) Diff(
	ctx context.Context, req infer.DiffRequest[PageMetadataArgs, PageMetadataState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{DetailedDiff: map[string]p.PropertyDiff{}}

	if req.State.PageID != req.Inputs.PageID {
		diff.DetailedDiff["pageId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.State.LocaleID != req.Inputs.LocaleID {
		diff.DetailedDiff["localeId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.State.Title != req.Inputs.Title {
		diff.DetailedDiff["title"] = p.PropertyDiff{Kind: p.Update}
	}
	if req.State.Slug != req.Inputs.Slug {
		diff.DetailedDiff["slug"] = p.PropertyDiff{Kind: p.Update}
	}
	if !pageMetadataSEOEqual(req.State.SEO, req.Inputs.SEO) {
		diff.DetailedDiff["seo"] = p.PropertyDiff{Kind: p.Update}
	}
	if !pageMetadataOpenGraphEqual(req.State.OpenGraph, req.Inputs.OpenGraph) {
		diff.DetailedDiff["openGraph"] = p.PropertyDiff{Kind: p.Update}
	}

	diff.HasChanges = len(diff.DetailedDiff) > 0
	return diff, nil
}

// Create applies the configured settings to the page.
func (r *PageMetadata) Create(
	ctx context.Context, req infer.CreateRequest[PageMetadataArgs],
) (infer.CreateResponse[PageMetadataState], error) {
	id := GeneratePageMetadataResourceID(req.Inputs.PageID, req.Inputs.LocaleID)

	// During preview, return the inputs without calling the API. Inputs may be unknown at this
	// point, so validation is deferred to apply time and no ID is reported while pageId is unknown.
	if req.DryRun {
		if req.Inputs.PageID == "" {
			id = ""
		}
		return infer.CreateResponse[PageMetadataState]{
			ID:     id,
			Output: PageMetadataState{PageMetadataArgs: req.Inputs},
		}, nil
	}

	if err := validatePageMetadataArgs(req.Inputs); err != nil {
		return infer.CreateResponse[PageMetadataState]{}, err
	}

	state, err := r.applyPageMetadata(ctx, req.Inputs)
	if err != nil {
		return infer.CreateResponse[PageMetadataState]{}, err
	}
	return infer.CreateResponse[PageMetadataState]{ID: id, Output: state}, nil
}

// Read fetches the page (GET /v2/pages/{page_id}) and reports the live values of the managed fields.
func (r *PageMetadata) Read(
	ctx context.Context, req infer.ReadRequest[PageMetadataArgs, PageMetadataState],
) (infer.ReadResponse[PageMetadataArgs, PageMetadataState], error) {
	pageID, localeID, err := ExtractIDsFromPageMetadataResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[PageMetadataArgs, PageMetadataState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidatePageID(pageID); err != nil {
		return infer.ReadResponse[PageMetadataArgs, PageMetadataState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateLocaleID(localeID); err != nil {
		return infer.ReadResponse[PageMetadataArgs, PageMetadataState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[PageMetadataArgs, PageMetadataState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	page, err := GetPageMetadata(ctx, client, pageID, localeID, "")
	if err != nil {
		if IsNotFound(err) {
			return infer.ReadResponse[PageMetadataArgs, PageMetadataState]{ID: ""}, nil
		}
		return infer.ReadResponse[PageMetadataArgs, PageMetadataState]{}, fmt.Errorf("failed to read page: %w", err)
	}

	// Report live values only for the fields this resource manages, so drift is detected for
	// them while unmanaged fields stay out of the diff. When importing (empty state) every
	// updatable field is captured.
	prev := req.State.PageMetadataArgs
	importing := !pageMetadataHasManagedField(prev)
	inputs := PageMetadataArgs{PageID: pageID, LocaleID: localeID}
	if importing || prev.Title != "" {
		inputs.Title = page.Title
	}
	if importing || prev.Slug != "" {
		inputs.Slug = page.Slug
	}
	if page.SEO != nil && (importing || prev.SEO != nil) {
		seo := &PageSEOArgs{}
		if importing || prev.SEO.Title != "" {
			seo.Title = page.SEO.Title
		}
		if importing || prev.SEO.Description != "" {
			seo.Description = page.SEO.Description
		}
		inputs.SEO = seo
	}
	if page.OpenGraph != nil && (importing || prev.OpenGraph != nil) {
		og := &PageOpenGraphArgs{}
		if importing || prev.OpenGraph.Title != "" {
			og.Title = page.OpenGraph.Title
		}
		if importing || prev.OpenGraph.Description != "" {
			og.Description = page.OpenGraph.Description
		}
		if importing || prev.OpenGraph.TitleCopied != nil {
			og.TitleCopied = ptr(page.OpenGraph.TitleCopied)
		}
		if importing || prev.OpenGraph.DescriptionCopied != nil {
			og.DescriptionCopied = ptr(page.OpenGraph.DescriptionCopied)
		}
		inputs.OpenGraph = og
	}

	return infer.ReadResponse[PageMetadataArgs, PageMetadataState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  pageMetadataStateFromAPI(inputs, page),
	}, nil
}

// Update re-applies the configured settings to the page.
func (r *PageMetadata) Update(
	ctx context.Context, req infer.UpdateRequest[PageMetadataArgs, PageMetadataState],
) (infer.UpdateResponse[PageMetadataState], error) {
	if req.DryRun {
		state := req.State
		state.PageMetadataArgs = req.Inputs
		return infer.UpdateResponse[PageMetadataState]{Output: state}, nil
	}

	if err := validatePageMetadataArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[PageMetadataState]{}, err
	}

	state, err := r.applyPageMetadata(ctx, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[PageMetadataState]{}, err
	}
	return infer.UpdateResponse[PageMetadataState]{Output: state}, nil
}

// Delete is a no-op: the page keeps whatever settings it currently has and is only removed
// from Pulumi state. There is no API to "unset" page settings, and reverting to the values
// that existed before Pulumi managed them would be surprising.
func (r *PageMetadata) Delete(
	ctx context.Context, req infer.DeleteRequest[PageMetadataState],
) (infer.DeleteResponse, error) {
	NewLogContext(ctx).
		WithField("pageId", req.State.PageID).
		Debug("PageMetadata removed from state; the page keeps its current settings in Webflow")
	return infer.DeleteResponse{}, nil
}
