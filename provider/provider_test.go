// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/blang/semver"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
)

// expectedResources and expectedFunctions are the provider's public surface.
// Update both lists deliberately when adding or removing a resource: this test
// guards against a type being implemented but never registered in Provider().
var (
	expectedResources = []string{
		"Site", "Redirect", "RobotsTxt",
		"Collection", "CollectionField", "CollectionItem",
		"PageMetadata", "PageContent", "PageCustomCode", "PageSchemaMarkup",
		"Webhook", "Asset", "AssetFolder",
		"SiteCustomCode", "RegisteredScript", "InlineScript",
		"EcommerceSettings", "GoogleTag",
	}
	expectedFunctions = []string{
		"getTokenInfo", "getAuthorizedUser",
		"getPage", "getPages", "getPageSchemaMarkup",
		"getAnalyticsTraffic", "getAnalyticsTopPages", "getAnalyticsTopDimensions",
		"getAnalyticsTopEvents", "getAnalyticsTimeOnPage",
	}
)

func TestProvider_RegistersExpectedSurface(t *testing.T) {
	server, err := integration.NewServer(context.Background(), Name, semver.MustParse("0.0.1"),
		integration.WithProvider(Provider()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	resp, err := server.GetSchema(p.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}

	var schema struct {
		Resources map[string]json.RawMessage `json:"resources"`
		Functions map[string]json.RawMessage `json:"functions"`
		Config    struct {
			Variables map[string]struct {
				Secret bool `json:"secret"`
			} `json:"variables"`
		} `json:"config"`
	}
	if err := json.Unmarshal([]byte(resp.Schema), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	for _, r := range expectedResources {
		if _, ok := schema.Resources["webflow:index:"+r]; !ok {
			t.Errorf("resource %s is not registered", r)
		}
	}
	for _, f := range expectedFunctions {
		if _, ok := schema.Functions["webflow:index:"+f]; !ok {
			t.Errorf("function %s is not registered", f)
		}
	}
	if len(schema.Resources) != len(expectedResources) {
		names := make([]string, 0, len(schema.Resources))
		for name := range schema.Resources {
			names = append(names, name)
		}
		t.Errorf("unexpected resource count %d (want %d): %s",
			len(schema.Resources), len(expectedResources), strings.Join(names, ", "))
	}
	if len(schema.Functions) != len(expectedFunctions) {
		t.Errorf("unexpected function count %d (want %d)", len(schema.Functions), len(expectedFunctions))
	}
	if _, ok := schema.Resources["webflow:index:PageData"]; ok {
		t.Error("PageData must not be registered: it was replaced by getPage/getPages and PageMetadata")
	}
	if !schema.Config.Variables["apiToken"].Secret {
		t.Error("apiToken must be a secret config value")
	}
}

func TestProvider_EveryPropertyIsDescribed(t *testing.T) {
	server, err := integration.NewServer(context.Background(), Name, semver.MustParse("0.0.1"),
		integration.WithProvider(Provider()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	resp, err := server.GetSchema(p.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}

	type props struct {
		Description string `json:"description"`
		Properties  map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
		InputProperties map[string]struct {
			Description string `json:"description"`
		} `json:"inputProperties"`
	}
	var schema struct {
		Resources map[string]props `json:"resources"`
		Types     map[string]props `json:"types"`
	}
	if err := json.Unmarshal([]byte(resp.Schema), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	check := func(kind, token string, pr props) {
		// Nested object types carry their meaning on their properties; only top-level
		// resources must describe themselves.
		if kind == "resource" && pr.Description == "" {
			t.Errorf("%s %s has no description", kind, token)
		}
		for name, prop := range pr.Properties {
			if prop.Description == "" {
				t.Errorf("%s %s output property %s has no description", kind, token, name)
			}
		}
		for name, prop := range pr.InputProperties {
			if prop.Description == "" {
				t.Errorf("%s %s input property %s has no description", kind, token, name)
			}
		}
	}
	for token, pr := range schema.Resources {
		check("resource", token, pr)
	}
	for token, pr := range schema.Types {
		check("type", token, pr)
	}
}
