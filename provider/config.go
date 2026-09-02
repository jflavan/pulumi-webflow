// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Config defines the provider configuration.
// The apiToken field is marked as a secret and will be automatically handled by Pulumi.
type Config struct {
	// APIToken is the Webflow API v2 bearer token for authentication.
	// Explicit provider configuration (`pulumi config set webflow:apiToken <value> --secret`)
	// takes precedence; the WEBFLOW_API_TOKEN environment variable is the fallback.
	APIToken string `pulumi:"apiToken,optional" provider:"secret"`
}

// Annotate adds descriptions and defaults to the Config fields for schema generation.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.APIToken, "Webflow API v2 bearer token for authentication. "+
		"Explicit configuration takes precedence over the WEBFLOW_API_TOKEN environment variable, "+
		"which is used as a fallback when no token is configured.")
	a.SetDefault(&c.APIToken, nil, "WEBFLOW_API_TOKEN")
}

// Configure validates the configuration once, before any resource operation runs.
// A missing token is not an error here so that previews of stacks that only use
// unrelated resources still work; resources report ErrTokenNotConfigured when they need it.
func (c *Config) Configure(ctx context.Context) error {
	if c.APIToken == "" {
		return nil
	}
	if err := ValidateToken(c.APIToken); err != nil {
		return fmt.Errorf("invalid webflow:apiToken: %w", err)
	}
	return nil
}

// safeGetConfigToken retrieves the API token from provider config.
// infer.GetConfig panics when no config is attached to the context, which happens in
// unit tests that call resource methods directly; treat that as "no config".
func safeGetConfigToken(ctx context.Context) (token string) {
	defer func() {
		if r := recover(); r != nil {
			token = ""
		}
	}()

	config := infer.GetConfig[*Config](ctx)
	if config != nil {
		return config.APIToken
	}
	return ""
}

// resolveToken returns the token to use, preferring explicit provider config.
func resolveToken(ctx context.Context) string {
	if token := safeGetConfigToken(ctx); token != "" {
		return token
	}
	return getEnvToken()
}

// GetHTTPClient returns an HTTP client for Webflow API calls.
// Token precedence: 1) provider config (webflow:apiToken), 2) WEBFLOW_API_TOKEN environment variable.
// Clients share one connection pool, so calling this per operation is cheap.
func GetHTTPClient(ctx context.Context, version string) (*http.Client, error) {
	token := resolveToken(ctx)
	if token == "" {
		return nil, ErrTokenNotConfigured
	}
	if err := ValidateToken(token); err != nil {
		return nil, err
	}
	return CreateHTTPClient(token, version)
}
