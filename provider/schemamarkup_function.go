// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// GetPageSchemaMarkup is a Pulumi Function that reads the JSON-LD schema markup of a page.
// It calls GET /beta/pages/{page_id}/schema-markup.
type GetPageSchemaMarkup struct{}

// GetPageSchemaMarkupInput defines the input parameters for the GetPageSchemaMarkup function.
type GetPageSchemaMarkupInput struct {
	// PageID is the Webflow page ID (24-character lowercase hexadecimal string).
	PageID string `pulumi:"pageId"`
	// LocaleID optionally targets a secondary locale. Omit for the primary locale.
	LocaleID string `pulumi:"localeId,optional"`
}

// GetPageSchemaMarkupOutput defines the output of the GetPageSchemaMarkup function.
type GetPageSchemaMarkupOutput struct {
	// PageID is the page identifier.
	PageID string `pulumi:"pageId"`
	// SiteID is the identifier of the site containing the page.
	SiteID string `pulumi:"siteId"`
	// LocaleID is the locale targeted by the request.
	LocaleID string `pulumi:"localeId"`
	// EffectiveLocaleID is the locale whose markup was returned.
	EffectiveLocaleID string `pulumi:"effectiveLocaleId"`
	// PublishedPath is the relative published URL path of the page.
	PublishedPath string `pulumi:"publishedPath"`
	// LastUpdated is the most recent update timestamp.
	LastUpdated string `pulumi:"lastUpdated"`
	// SchemaMarkup is the JSON-LD document as a canonical JSON string ("" when none).
	SchemaMarkup string `pulumi:"schemaMarkup"`
	// RawSchemaMarkup is the raw stored markup including script tags (legacy formats only).
	RawSchemaMarkup string `pulumi:"rawSchemaMarkup"`
	// IsInherited is true when the primary locale's markup was returned for a secondary locale.
	IsInherited bool `pulumi:"isInherited"`
}

// Annotate adds descriptions to the GetPageSchemaMarkup function.
func (f *GetPageSchemaMarkup) Annotate(a infer.Annotator) {
	a.Describe(f, "Reads the JSON-LD schema markup (structured data) of a Webflow page, optionally for a "+
		"secondary locale, using the Pages schema markup API (beta). When a secondary locale has no markup "+
		"of its own, the primary locale's markup is returned and 'isInherited' is true. "+
		"Requires the 'pages:read' scope.")
}

// Annotate adds descriptions to the GetPageSchemaMarkupInput fields.
func (i *GetPageSchemaMarkupInput) Annotate(a infer.Annotator) {
	a.Describe(&i.PageID,
		"The Webflow page ID (24-character lowercase hexadecimal string, e.g., '6596da6045e56dee495bcbba').")
	a.Describe(&i.LocaleID,
		"Optional secondary locale ID (24-character lowercase hexadecimal string). "+
			"Omit to read the primary locale's markup.")
}

// Annotate adds descriptions to the GetPageSchemaMarkupOutput fields.
func (o *GetPageSchemaMarkupOutput) Annotate(a infer.Annotator) {
	a.Describe(&o.PageID, "The page identifier.")
	a.Describe(&o.SiteID, "The ID of the Webflow site containing the page.")
	a.Describe(&o.LocaleID, "The locale targeted by the request (empty when Webflow reports none).")
	a.Describe(&o.EffectiveLocaleID,
		"The locale whose markup was returned; differs from 'localeId' when a secondary locale falls back "+
			"to the primary locale. Empty when no markup exists.")
	a.Describe(&o.PublishedPath, "The relative published URL path of the page (e.g., '/about').")
	a.Describe(&o.LastUpdated, "The timestamp of the most recent update to the markup (RFC3339).")
	a.Describe(&o.SchemaMarkup,
		"The JSON-LD document as a compact JSON string with sorted keys, or an empty string when the page "+
			"has no schema markup. Parse it with JSON.parse / json.loads to work with it as an object.")
	a.Describe(&o.RawSchemaMarkup,
		"The raw stored markup including <script> tags. Only populated for legacy multi-block markup that "+
			"cannot be represented as a single JSON object; empty otherwise.")
	a.Describe(&o.IsInherited,
		"True when the targeted secondary locale has no markup of its own and the primary locale's markup "+
			"was returned.")
}

// Invoke implements the infer.Fn interface to read a page's schema markup.
func (f *GetPageSchemaMarkup) Invoke(
	ctx context.Context,
	req infer.FunctionRequest[GetPageSchemaMarkupInput],
) (infer.FunctionResponse[GetPageSchemaMarkupOutput], error) {
	if err := ValidatePageID(req.Input.PageID); err != nil {
		return infer.FunctionResponse[GetPageSchemaMarkupOutput]{},
			fmt.Errorf("validation failed for GetPageSchemaMarkup: %w", err)
	}
	if err := ValidateLocaleID(req.Input.LocaleID); err != nil {
		return infer.FunctionResponse[GetPageSchemaMarkupOutput]{},
			fmt.Errorf("validation failed for GetPageSchemaMarkup: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.FunctionResponse[GetPageSchemaMarkupOutput]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	resp, err := GetPageSchemaMarkupAPI(ctx, client, req.Input.PageID, req.Input.LocaleID)
	if err != nil {
		return infer.FunctionResponse[GetPageSchemaMarkupOutput]{}, fmt.Errorf("failed to get page schema markup: %w", err)
	}

	markup, err := canonicalizeRawJSON(resp.JSONLDSchema)
	if err != nil {
		return infer.FunctionResponse[GetPageSchemaMarkupOutput]{}, err
	}

	pageID := resp.ID
	if pageID == "" {
		pageID = req.Input.PageID
	}

	return infer.FunctionResponse[GetPageSchemaMarkupOutput]{Output: GetPageSchemaMarkupOutput{
		PageID:            pageID,
		SiteID:            resp.SiteID,
		LocaleID:          derefString(resp.LocaleID),
		EffectiveLocaleID: derefString(resp.EffectiveLocaleID),
		PublishedPath:     derefString(resp.PublishedPath),
		LastUpdated:       derefString(resp.LastUpdated),
		SchemaMarkup:      markup,
		RawSchemaMarkup:   derefString(resp.RawJSONLDSchema),
		IsInherited:       resp.IsInherited,
	}}, nil
}
