// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"errors"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Deprecated: PageData never worked as a resource (Create always failed and import was broken).
// It has been replaced by the getPage / getPages functions and the PageMetadata resource.
// This compile-only shim remains solely because provider.go still registers it; remove the
// `infer.Resource(&PageData{})` registration and delete this file.
type PageData struct{}

// PageDataArgs is retained only so the deprecated PageData shim compiles.
type PageDataArgs struct {
	SiteID string `pulumi:"siteId"`
	PageID string `pulumi:"pageId,optional"`
}

// PageDataState is retained only so the deprecated PageData shim compiles.
type PageDataState struct {
	PageDataArgs
}

// errPageDataDeprecated explains what to use instead.
var errPageDataDeprecated = errors.New("the PageData resource is deprecated and no longer functional. " +
	"Use the getPage or getPages function to read page metadata, and the PageMetadata resource to " +
	"manage page settings")

// Annotate marks the shim as deprecated in the schema.
func (r *PageData) Annotate(a infer.Annotator) {
	a.SetToken("index", "PageData")
	a.Deprecate(r, "Use the getPage/getPages functions or the PageMetadata resource instead.")
	a.Describe(r, "Deprecated and non-functional. Use the getPage/getPages functions to read pages "+
		"and the PageMetadata resource to manage page settings.")
}

// Create always fails; see errPageDataDeprecated.
func (r *PageData) Create(
	context.Context, infer.CreateRequest[PageDataArgs],
) (infer.CreateResponse[PageDataState], error) {
	return infer.CreateResponse[PageDataState]{}, errPageDataDeprecated
}

// Read always fails; see errPageDataDeprecated.
func (r *PageData) Read(
	context.Context, infer.ReadRequest[PageDataArgs, PageDataState],
) (infer.ReadResponse[PageDataArgs, PageDataState], error) {
	return infer.ReadResponse[PageDataArgs, PageDataState]{}, errPageDataDeprecated
}

// Update always fails; see errPageDataDeprecated.
func (r *PageData) Update(
	context.Context, infer.UpdateRequest[PageDataArgs, PageDataState],
) (infer.UpdateResponse[PageDataState], error) {
	return infer.UpdateResponse[PageDataState]{}, errPageDataDeprecated
}

// Delete succeeds so stale entries can be dropped from state.
func (r *PageData) Delete(context.Context, infer.DeleteRequest[PageDataState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}
