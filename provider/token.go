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
)

// AuthorizedTo represents the authorization scope for a token.
type AuthorizedTo struct {
	SiteIDs      []string `json:"siteIds,omitempty"`
	WorkspaceIDs []string `json:"workspaceIds,omitempty"`
	UserIDs      []string `json:"userIds,omitempty"`
}

// Authorization represents the token authorization details.
type Authorization struct {
	ID           string       `json:"id"`
	CreatedOn    string       `json:"createdOn,omitempty"`
	LastUsed     string       `json:"lastUsed,omitempty"`
	GrantType    string       `json:"grantType,omitempty"`
	RateLimit    int          `json:"rateLimit,omitempty"`
	Scope        string       `json:"scope,omitempty"`
	AuthorizedTo AuthorizedTo `json:"authorizedTo,omitempty"`
}

// Application represents the application details for a token.
type Application struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// TokenIntrospectResponse represents the response from GET /token/introspect.
type TokenIntrospectResponse struct {
	Authorization Authorization `json:"authorization"`
	Application   Application   `json:"application,omitempty"`
}

// AuthorizedByResponse represents the response from GET /token/authorized_by.
type AuthorizedByResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
}

// tokenEndpoint identifies which token endpoint produced an error.
type tokenEndpoint string

const (
	tokenEndpointIntrospect   tokenEndpoint = "token/introspect"
	tokenEndpointAuthorizedBy tokenEndpoint = "token/authorized_by"
)

// introspectTokenTypeNote explains the most common cause of a 4xx from token/introspect:
// the endpoint only accepts OAuth access tokens issued to a Data Client App, so a site API
// token (Site settings > Apps & integrations > API access) is rejected.
const introspectTokenTypeNote = "Note: GET /v2/token/introspect only accepts Data Client App (OAuth) " +
	"access tokens, so a site API token receives a 4xx response from it. Configure the provider with an " +
	"OAuth access token issued to a Data Client App to use getTokenInfo, or keep using the site token " +
	"for the other resources and functions"

// GetTokenIntrospect retrieves token authorization information (GET /v2/token/introspect).
// The endpoint requires a Data Client App (OAuth) access token; site API tokens receive 4xx.
func GetTokenIntrospect(ctx context.Context, client *http.Client) (*TokenIntrospectResponse, error) {
	var response TokenIntrospectResponse
	if _, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/token/introspect"), nil, &response); err != nil {
		return nil, wrapTokenError(err, tokenEndpointIntrospect)
	}
	return &response, nil
}

// GetAuthorizedBy retrieves the user who authorized the token (GET /v2/token/authorized_by).
func GetAuthorizedBy(ctx context.Context, client *http.Client) (*AuthorizedByResponse, error) {
	var response AuthorizedByResponse
	if _, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/token/authorized_by"), nil, &response); err != nil {
		return nil, wrapTokenError(err, tokenEndpointAuthorizedBy)
	}
	return &response, nil
}

// wrapTokenError swaps the generic APIError message for token-endpoint-specific guidance.
// The APIError body is already truncated by doRequest. For token/introspect, a 4xx other
// than 429 additionally explains that the endpoint needs a Data Client App (OAuth) token.
func wrapTokenError(err error, endpoint tokenEndpoint) error {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	wrapped := handleTokenError(apiErr.StatusCode, []byte(apiErr.Body))
	if endpoint == tokenEndpointIntrospect && isClientErrorStatus(apiErr.StatusCode) &&
		apiErr.StatusCode != http.StatusTooManyRequests {
		return fmt.Errorf("%w. %s", wrapped, introspectTokenTypeNote)
	}
	return wrapped
}

// isClientErrorStatus reports whether status is a 4xx response.
func isClientErrorStatus(status int) bool {
	return status >= http.StatusBadRequest && status < http.StatusInternalServerError
}

// handleTokenError converts HTTP error responses to actionable error messages for token endpoints.
func handleTokenError(statusCode int, body []byte) error {
	details := TruncateForLogging(string(body), maxErrorBodyLength)
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: authentication failed (HTTP 401). "+
			"Your Webflow API token is invalid or has expired. "+
			"To fix this: 1) Verify your token in the Webflow dashboard (Settings > Integrations > API Access), "+
			"2) Ensure the token is valid and not expired, "+
			"3) Update your Pulumi config with: 'pulumi config set webflow:apiToken <your-token> --secret'. "+
			"Details: %s", details)
	case http.StatusForbidden:
		return fmt.Errorf("forbidden: access denied (HTTP 403). "+
			"Your API token does not have the required permissions. "+
			"To fix this: Ensure your API token has the 'authorized_user:read' scope "+
			"for the authorized_by endpoint. Details: %s", details)
	case http.StatusNotFound:
		return fmt.Errorf("not found: the requested endpoint does not exist (HTTP 404). "+
			"This may indicate an API version mismatch. Details: %s", details)
	case http.StatusTooManyRequests:
		return errors.New("rate limited: too many requests to the Webflow API (HTTP 429). " +
			"The provider retried with exponential backoff and the limit was still exceeded. " +
			"Please wait a few minutes before trying again")
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Errorf("server error: Webflow API encountered an internal error (HTTP %d). "+
			"This is a temporary issue on Webflow's side. "+
			"Please wait a few minutes and try again. "+
			"If the problem persists, check Webflow's status page or contact Webflow support. "+
			"Details: %s", statusCode, details)
	default:
		return fmt.Errorf("unexpected error (HTTP %d): %s. "+
			"This is an unexpected response from the Webflow API. "+
			"Please check Webflow's status page or contact Webflow support if this error persists",
			statusCode, details)
	}
}
