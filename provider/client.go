// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// webflowAPIBaseURL is the production base URL for the Webflow Data API v2.
const webflowAPIBaseURL = "https://api.webflow.com"

// apiBaseURLOverride lets tests point every API call at a local mock server.
// Production code must leave it empty.
var apiBaseURLOverride = ""

// apiBaseURL returns the base URL to use for API calls, honouring the test override.
func apiBaseURL() string {
	if apiBaseURLOverride != "" {
		return apiBaseURLOverride
	}
	return webflowAPIBaseURL
}

// apiURL builds an absolute API URL from a v2 path such as "/v2/sites/%s".
func apiURL(format string, args ...any) string {
	return apiBaseURL() + fmt.Sprintf(format, args...)
}

// maxErrorBodyLength caps how much of a response body is echoed into an error message.
const maxErrorBodyLength = 512

// ErrNotFound is returned (wrapped) when the Webflow API answers 404.
// Callers must test for it with errors.Is(err, ErrNotFound); never match on message text.
var ErrNotFound = errors.New("not found")

// APIError is a non-success response from the Webflow API.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

// Error returns an actionable message for the status code.
func (e *APIError) Error() string {
	details := strings.TrimSpace(e.Body)
	if details == "" {
		details = "no response body"
	}
	where := ""
	if e.Method != "" && e.Path != "" {
		where = " (" + e.Method + " " + e.Path + ")"
	}
	switch e.StatusCode {
	case http.StatusBadRequest:
		return fmt.Sprintf("bad request%s: the Webflow API rejected the request. Details: %s. "+
			"Check the resource configuration and ensure all required fields have valid values", where, details)
	case http.StatusUnauthorized:
		return "unauthorized: authentication failed. Your Webflow API token is invalid or has expired. " +
			"To fix this: 1) Verify the token in the Webflow dashboard (Site settings > Apps & integrations > API access), " +
			"2) Ensure the token has the scopes this resource needs, " +
			"3) Update your Pulumi config with: 'pulumi config set webflow:apiToken <your-token> --secret'"
	case http.StatusForbidden:
		return fmt.Sprintf("forbidden%s: access denied. Your API token does not have permission for this operation. "+
			"To fix this: 1) Verify the site or resource ID is correct, "+
			"2) Ensure the token has the scopes this resource needs (see the resource documentation), "+
			"3) Check that the site belongs to the workspace the token was created in. Details: %s", where, details)
	case http.StatusNotFound:
		return fmt.Sprintf("not found%s: the Webflow resource does not exist. "+
			"To fix this: 1) Verify the ID is correct (Webflow IDs are 24-character lowercase hex strings), "+
			"2) Check that the site or resource still exists in the Webflow dashboard. Details: %s", where, details)
	case http.StatusConflict:
		return fmt.Sprintf("conflict%s: the request conflicts with the current state in Webflow. Details: %s", where, details)
	case http.StatusTooManyRequests:
		return "rate limited: too many requests to the Webflow API. " +
			"The provider retried with exponential backoff and the limit was still exceeded. " +
			"Please wait a few minutes before trying again, or reduce the number of concurrent operations"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Sprintf("server error (HTTP %d)%s: the Webflow API encountered a temporary problem. Details: %s. "+
			"Please wait a few minutes and retry. If the problem persists, check https://status.webflow.com",
			e.StatusCode, where, details)
	default:
		return fmt.Sprintf("unexpected error (HTTP %d)%s: %s. "+
			"This is an unexpected response from the Webflow API. Check https://status.webflow.com or contact Webflow support",
			e.StatusCode, where, details)
	}
}

// Is lets errors.Is(err, ErrNotFound) match 404 responses.
func (e *APIError) Is(target error) bool {
	return target == ErrNotFound && e.StatusCode == http.StatusNotFound
}

// IsNotFound reports whether err represents a 404 from the Webflow API.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// newAPIError builds an APIError with a truncated body.
func newAPIError(statusCode int, method, path string, body []byte) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Method:     method,
		Path:       path,
		Body:       TruncateForLogging(string(body), maxErrorBodyLength),
	}
}

// handleWebflowError converts a non-success response into an APIError.
// Kept for callers that still handle status codes themselves; new code should use doRequest.
func handleWebflowError(statusCode int, body []byte) error {
	return newAPIError(statusCode, "", "", body)
}

// handleNetworkError wraps transport-level failures with recovery guidance.
func handleNetworkError(err error) error {
	msg := err.Error()
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("request cancelled: %w", err)
	case errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded"):
		return fmt.Errorf("network timeout: the request to the Webflow API timed out. "+
			"This may indicate connectivity problems or that Webflow is slow to respond. "+
			"To fix this: 1) Check your internet connection, 2) Check https://status.webflow.com, "+
			"3) Wait a few minutes and retry: %w", err)
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host"):
		return fmt.Errorf("connection failed: unable to connect to the Webflow API. "+
			"To fix this: 1) Check your internet connection, 2) Verify DNS resolution (try: nslookup api.webflow.com), "+
			"3) Check firewall or proxy settings (HTTPS_PROXY is honoured): %w", err)
	default:
		return fmt.Errorf("network error: the request to the Webflow API failed. "+
			"To fix this: 1) Check your internet connection, 2) Check https://status.webflow.com, "+
			"3) Wait a few minutes and retry: %w", err)
	}
}

// defaultOKStatuses are the statuses doRequest accepts when the caller passes none.
var defaultOKStatuses = []int{http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent}

// doRequest performs one Webflow API call and decodes the JSON response.
//
//   - body, when non-nil, is JSON-encoded and sent with Content-Type: application/json.
//   - out, when non-nil, receives the decoded JSON response body (ignored for empty bodies).
//   - okStatuses lists the status codes treated as success; defaults to 200, 201, 202, 204.
//
// Rate limiting (429) and transient 5xx responses are retried by the HTTP client's
// transport, so callers see only the final outcome. Non-success responses are returned
// as *APIError; 404 satisfies errors.Is(err, ErrNotFound). Transport failures are wrapped
// by handleNetworkError. The returned status code is set whenever a response was received.
func doRequest(
	ctx context.Context, client *http.Client, method, url string, body, out any, okStatuses ...int,
) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("context cancelled: %w", err)
	}

	var reader io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, handleNetworkError(err)
	}
	respBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return resp.StatusCode, fmt.Errorf("failed to read response body: %w", readErr)
	}

	accepted := okStatuses
	if len(accepted) == 0 {
		accepted = defaultOKStatuses
	}
	ok := false
	for _, s := range accepted {
		if resp.StatusCode == s {
			ok = true
			break
		}
	}
	if !ok {
		return resp.StatusCode, newAPIError(resp.StatusCode, method, req.URL.Path, respBody)
	}

	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, fmt.Errorf("failed to parse response from %s %s: %w", method, req.URL.Path, err)
		}
	}
	return resp.StatusCode, nil
}

// doDelete issues a DELETE and treats 404 as success so deletes are idempotent.
func doDelete(ctx context.Context, client *http.Client, url string, body any) error {
	_, err := doRequest(ctx, client, http.MethodDelete, url, body, nil,
		http.StatusOK, http.StatusAccepted, http.StatusNoContent, http.StatusNotFound)
	return err
}
