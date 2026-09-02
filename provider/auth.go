// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"crypto/tls"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Error codes for programmatic error handling.
// Use these codes to identify specific error types in automation and scripts.
const (
	// ErrCodeAuthNotConfigured indicates the API token is missing.
	ErrCodeAuthNotConfigured = "WEBFLOW_AUTH_001"
	// ErrCodeAuthEmpty indicates an empty API token was provided.
	ErrCodeAuthEmpty = "WEBFLOW_AUTH_002"
	// ErrCodeAuthInvalid indicates the API token format is invalid.
	ErrCodeAuthInvalid = "WEBFLOW_AUTH_003"
)

// ErrTokenNotConfigured is returned when no API token is available.
var ErrTokenNotConfigured = errors.New("[" + ErrCodeAuthNotConfigured + "] Webflow API token not configured. " +
	"Configure using: pulumi config set webflow:apiToken <token> --secret " +
	"OR set WEBFLOW_API_TOKEN environment variable. " +
	"See: https://github.com/jdetmar/pulumi-webflow/blob/main/docs/troubleshooting.md#api-token-not-configured")

// getEnvToken retrieves the Webflow API token from the environment variable.
func getEnvToken() string {
	return os.Getenv("WEBFLOW_API_TOKEN")
}

// ValidateToken performs basic validation on the API token.
// Checks that the token is non-empty and has reasonable length.
func ValidateToken(token string) error {
	if token == "" {
		return errors.New("[" + ErrCodeAuthEmpty + "] API token cannot be empty. " +
			"Provide a valid Webflow API token via config or environment variable. " +
			"See: https://github.com/jdetmar/pulumi-webflow/blob/main/docs/troubleshooting.md#api-token-not-configured")
	}

	// Basic sanity check - Webflow tokens should be reasonably long
	if len(token) < 10 {
		return errors.New("[" + ErrCodeAuthInvalid + "] API token appears invalid (too short). " +
			"Webflow API tokens are typically 40+ characters. " +
			"See: https://github.com/jdetmar/pulumi-webflow/blob/main/docs/troubleshooting.md#invalid-or-expired-token")
	}

	return nil
}

// RedactToken returns a redacted version of the token for logging.
// Always returns "[REDACTED]" to prevent token leakage in logs.
func RedactToken(token string) string {
	return RedactSensitiveData(token)
}

// Default retry configuration for rate-limit and transient-failure handling.
const (
	// DefaultMaxRetries is the maximum number of retry attempts for retryable responses.
	DefaultMaxRetries = 3
	// DefaultBaseDelay is the initial delay before the first retry.
	DefaultBaseDelay = 1 * time.Second
	// DefaultMaxDelay caps the maximum delay between retries.
	DefaultMaxDelay = 60 * time.Second
)

// retryBaseDelay and retryMaxDelay are the delays used by clients built with CreateHTTPClient.
// Tests lower them so retry paths run in milliseconds.
var (
	retryBaseDelay = DefaultBaseDelay
	retryMaxDelay  = DefaultMaxDelay
)

// retryTransport is an http.RoundTripper that retries rate-limited (429) and transient
// server-error (502, 503, 504) responses with exponential backoff, honouring Retry-After.
type retryTransport struct {
	transport  http.RoundTripper // Underlying transport for actual HTTP requests
	maxRetries int               // Maximum number of retry attempts
	baseDelay  time.Duration     // Initial delay before first retry
	maxDelay   time.Duration     // Maximum delay between retries
}

// isIdempotentMethod reports whether a request may be repeated without risking a duplicate
// side effect. POST and PATCH are excluded: a gateway error returned after Webflow already
// committed a create would otherwise be re-sent and create a second resource.
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
}

// isRetryableStatus reports whether a response status should be retried for the given method.
// 429 is retried for every method because the request was never processed. Transient
// server errors (502, 503, 504) are retried only for idempotent methods.
func isRetryableStatus(method string, status int) bool {
	switch status {
	case http.StatusTooManyRequests:
		return true
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return isIdempotentMethod(method)
	}
	return false
}

// RoundTrip implements http.RoundTripper with retry logic.
//
// A request body can only be read once, so each retry rewinds it through req.GetBody.
// http.NewRequest sets GetBody for bytes and strings readers, which is what doRequest uses;
// a request with a body but no GetBody is sent once and never retried.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	log := NewLogContext(ctx).
		WithField("method", req.Method).
		WithField("url", req.URL.Path)

	canRewind := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil

	for attempt := 0; ; attempt++ {
		attemptReq := req
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			attemptReq = req.Clone(ctx)
			attemptReq.Body = body
		}

		resp, err := t.transport.RoundTrip(attemptReq)
		if err != nil {
			log.WithField("attempt", attempt+1).Debugf("HTTP request failed: %v", err)
			return nil, err
		}

		log.WithField("status", resp.StatusCode).WithField("attempt", attempt+1).Debug("HTTP request completed")

		retryable := isRetryableStatus(req.Method, resp.StatusCode)
		if !retryable || attempt >= t.maxRetries || !canRewind {
			if retryable {
				log.WithField("status", resp.StatusCode).WithField("maxRetries", t.maxRetries).
					Warn("Retryable response, max retries exhausted")
			}
			return resp, nil
		}

		delay := t.calculateDelay(resp, attempt)
		// Drain and close so the connection can be reused for the retry.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()

		log.WithField("status", resp.StatusCode).WithField("attempt", attempt+1).WithField("retryAfter", delay.String()).
			Warnf("Retrying after %v", delay)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// calculateDelay determines how long to wait before the next retry.
// It respects a Retry-After header in seconds if present, otherwise uses exponential backoff.
func (t *retryTransport) calculateDelay(resp *http.Response, attempt int) time.Duration {
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds >= 0 {
			// Clamp before multiplying so a huge header value cannot overflow to a negative delay.
			if seconds >= int64(t.maxDelay/time.Second) {
				return t.maxDelay
			}
			return time.Duration(seconds) * time.Second
		}
	}

	delay := time.Duration(float64(t.baseDelay) * math.Pow(2, float64(attempt)))
	if delay > t.maxDelay {
		return t.maxDelay
	}
	return delay
}

// authenticatedTransport is an http.RoundTripper that adds authentication headers.
type authenticatedTransport struct {
	token     string            // Webflow API token for Bearer authentication
	version   string            // Provider version for User-Agent header
	transport http.RoundTripper // Underlying transport for actual HTTP requests
}

// RoundTrip implements http.RoundTripper interface, adding authentication headers.
func (t *authenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	clonedReq := req.Clone(req.Context())

	clonedReq.Header.Set("Authorization", "Bearer "+t.token)
	clonedReq.Header.Set("User-Agent", "pulumi-webflow/"+t.version)
	clonedReq.Header.Set("Accept-Version", "2.0.0")

	return t.transport.RoundTrip(clonedReq)
}

var (
	baseTransportOnce sync.Once
	baseTransport     *http.Transport
)

// sharedBaseTransport returns the process-wide TLS transport used by every client.
// It is a clone of http.DefaultTransport, so proxies (HTTPS_PROXY, NO_PROXY), dial and
// TLS-handshake timeouts, and idle-connection reaping all behave as in the standard library,
// with TLS 1.2 enforced as the minimum version.
func sharedBaseTransport() *http.Transport {
	baseTransportOnce.Do(func() {
		baseTransport = http.DefaultTransport.(*http.Transport).Clone()
		baseTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		baseTransport.ResponseHeaderTimeout = 60 * time.Second
		baseTransport.MaxIdleConnsPerHost = 8
	})
	return baseTransport
}

// CreateHTTPClient creates an HTTP client configured for Webflow API v2.
// The client enforces TLS 1.2, adds authentication headers, retries rate-limited and
// transient server-error responses with exponential backoff, and shares one connection
// pool across all clients created in the process.
//
// The client sets no overall timeout: Pulumi's request context controls cancellation, and
// a per-response header timeout guards against a hung server. This keeps long Retry-After
// waits from being misreported as network timeouts.
func CreateHTTPClient(token, version string) (*http.Client, error) {
	if token == "" {
		return nil, errors.New("[" + ErrCodeAuthEmpty + "] cannot create HTTP client with empty token. " +
			"See: https://github.com/jdetmar/pulumi-webflow/blob/main/docs/troubleshooting.md#api-token-not-configured")
	}

	authTransport := &authenticatedTransport{
		token:     token,
		version:   version,
		transport: sharedBaseTransport(),
	}

	retry := &retryTransport{
		transport:  authTransport,
		maxRetries: DefaultMaxRetries,
		baseDelay:  retryBaseDelay,
		maxDelay:   retryMaxDelay,
	}

	return &http.Client{Transport: retry}, nil
}
