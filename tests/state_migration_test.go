// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// These tests drive real state through the provider's Diff path so the framework's
// migration selection is exercised: a v0.9 state (property "version") must be upgraded
// to "scriptVersion" before Diff runs, and a v0.10 state must pass through untouched.
// Diff for these resources is pure (no API calls), so no mock server is needed.

func inlineScriptInputs(version string) map[string]property.Value {
	return map[string]property.Value{
		"siteId":        property.New("5f0c8c9e1c9d440000e8d8c3"),
		"sourceCode":    property.New("console.log('hi')"),
		"scriptVersion": property.New(version),
		"displayName":   property.New("Hello"),
		"canCopy":       property.New(false),
	}
}

func TestInlineScriptStateMigratesFromV09(t *testing.T) {
	t.Parallel()
	prov := provider(t)

	// v0.9 state shape: the version property was called "version".
	olds := property.NewMap(map[string]property.Value{
		"siteId":         property.New("5f0c8c9e1c9d440000e8d8c3"),
		"sourceCode":     property.New("console.log('hi')"),
		"version":        property.New("1.0.0"),
		"displayName":    property.New("Hello"),
		"canCopy":        property.New(false),
		"scriptId":       property.New("hello"),
		"hostedLocation": property.New("https://cdn.example.com/hello.js"),
	})

	resp, err := prov.Diff(p.DiffRequest{
		Urn:    urn("InlineScript"),
		ID:     "5f0c8c9e1c9d440000e8d8c3/inline_scripts/hello",
		State:  olds,
		Inputs: property.NewMap(inlineScriptInputs("1.0.0")),
	})
	require.NoError(t, err)
	require.False(t, resp.HasChanges,
		"a v0.9 state with the same version must diff clean after migration, got %+v", resp.DetailedDiff)
}

func TestInlineScriptStateV010PassesThrough(t *testing.T) {
	t.Parallel()
	prov := provider(t)

	// v0.10 state shape must not be mangled by the v0.9 migration.
	olds := property.NewMap(map[string]property.Value{
		"siteId":         property.New("5f0c8c9e1c9d440000e8d8c3"),
		"sourceCode":     property.New("console.log('hi')"),
		"scriptVersion":  property.New("1.0.0"),
		"displayName":    property.New("Hello"),
		"canCopy":        property.New(false),
		"scriptId":       property.New("hello"),
		"hostedLocation": property.New("https://cdn.example.com/hello.js"),
	})

	same, err := prov.Diff(p.DiffRequest{
		Urn:    urn("InlineScript"),
		ID:     "5f0c8c9e1c9d440000e8d8c3/inline_scripts/hello",
		State:  olds,
		Inputs: property.NewMap(inlineScriptInputs("1.0.0")),
	})
	require.NoError(t, err)
	require.False(t, same.HasChanges, "unchanged v0.10 state must diff clean, got %+v", same.DetailedDiff)

	changed, err := prov.Diff(p.DiffRequest{
		Urn:    urn("InlineScript"),
		ID:     "5f0c8c9e1c9d440000e8d8c3/inline_scripts/hello",
		State:  olds,
		Inputs: property.NewMap(inlineScriptInputs("2.0.0")),
	})
	require.NoError(t, err)
	require.True(t, changed.HasChanges, "a scriptVersion change must be detected")
}

func TestSiteCustomCodeStateMigratesFromV09(t *testing.T) {
	t.Parallel()
	prov := provider(t)

	script := func(versionKey string) property.Value {
		return property.New(property.NewMap(map[string]property.Value{
			"id":       property.New("hello"),
			versionKey: property.New("1.0.0"),
			"location": property.New("header"),
		}))
	}
	olds := property.NewMap(map[string]property.Value{
		"siteId":  property.New("5f0c8c9e1c9d440000e8d8c3"),
		"scripts": property.New([]property.Value{script("version")}),
	})
	news := property.NewMap(map[string]property.Value{
		"siteId":  property.New("5f0c8c9e1c9d440000e8d8c3"),
		"scripts": property.New([]property.Value{script("scriptVersion")}),
	})

	resp, err := prov.Diff(p.DiffRequest{
		Urn:    urn("SiteCustomCode"),
		ID:     "5f0c8c9e1c9d440000e8d8c3/custom_code",
		State:  olds,
		Inputs: news,
	})
	require.NoError(t, err)
	require.False(t, resp.HasChanges,
		"a v0.9 SiteCustomCode state must diff clean after migration, got %+v", resp.DetailedDiff)
}
