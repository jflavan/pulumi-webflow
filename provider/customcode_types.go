// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import "reflect"

// CustomCodeScript is a registered script applied to a site or a page.
// It is the wire format shared by the site and page custom code endpoints
// (GET/PUT /v2/sites/{site_id}/custom_code and GET/PUT /v2/pages/{page_id}/custom_code).
type CustomCodeScript struct {
	// ID is the unique identifier of the registered custom code script.
	ID string `json:"id"`
	// Version is the semantic version string for the registered script (e.g., "1.0.0").
	Version string `json:"version"`
	// Location is where the script is placed: "header" or "footer".
	Location string `json:"location"`
	// Attributes are developer-specified key/value pairs applied as attributes to the script tag.
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// CustomCodeResponse is the response body shared by the site and page custom code endpoints.
type CustomCodeResponse struct {
	// Scripts is the list of scripts applied to the site or page.
	Scripts []CustomCodeScript `json:"scripts"`
	// LastUpdated is when the scripts were last updated (read-only).
	LastUpdated string `json:"lastUpdated,omitempty"`
	// CreatedOn is when the scripts were first created (read-only).
	CreatedOn string `json:"createdOn,omitempty"`
}

// CustomCodeRequest is the PUT request body shared by the site and page custom code endpoints.
// Scripts is always sent (as [] when empty) because the API requires the field.
type CustomCodeRequest struct {
	Scripts []CustomCodeScript `json:"scripts"`
}

// customCodeScriptInput is the set of Pulumi input types that describe one custom code script.
// CustomScriptArgs (SiteCustomCode) and PageCustomCodeScript (PageCustomCode) stay distinct
// because their names are part of the generated schema; they have identical fields.
type customCodeScriptInput interface {
	CustomScriptArgs | PageCustomCodeScript
}

// toAPIScripts converts Pulumi script inputs to the API wire format.
// The result is never nil so it serializes as [] rather than null.
func toAPIScripts[T customCodeScriptInput](in []T) []CustomCodeScript {
	out := make([]CustomCodeScript, len(in))
	for i, s := range in {
		script := CustomCodeScript(s)
		script.Attributes = cloneAttributes(script.Attributes)
		out[i] = script
	}
	return out
}

// fromAPIScripts converts API scripts to Pulumi script inputs.
func fromAPIScripts[T customCodeScriptInput](in []CustomCodeScript) []T {
	out := make([]T, len(in))
	for i, s := range in {
		s.Attributes = cloneAttributes(s.Attributes)
		out[i] = T(s)
	}
	return out
}

// cloneAttributes returns a shallow copy of attrs, or nil when attrs is empty.
func cloneAttributes(attrs map[string]interface{}) map[string]interface{} {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	return out
}

// scriptListsEqual reports whether two script lists describe the same scripts, ignoring order.
// Scripts are matched by id and location; version and attributes must then be deeply equal.
func scriptListsEqual[T customCodeScriptInput](a, b []T) bool {
	return customCodeScriptsEqual(toAPIScripts(a), toAPIScripts(b))
}

// customCodeScriptsEqual is the order-insensitive comparison behind scriptListsEqual.
func customCodeScriptsEqual(a, b []CustomCodeScript) bool {
	if len(a) != len(b) {
		return false
	}
	matched := make([]bool, len(b))
	for _, sa := range a {
		found := false
		for j, sb := range b {
			if matched[j] || sa.ID != sb.ID || sa.Location != sb.Location {
				continue
			}
			if sa.Version != sb.Version || !attributesEqual(sa.Attributes, sb.Attributes) {
				continue
			}
			matched[j] = true
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return true
}

// attributesEqual compares attribute maps, treating nil and empty as equal and
// comparing nested values structurally (reflect.DeepEqual never panics on
// uncomparable values such as nested maps or slices).
func attributesEqual(a, b map[string]interface{}) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
