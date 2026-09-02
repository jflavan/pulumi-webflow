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
	"regexp"
	"strings"
	"time"
)

// RobotsTxtRule represents a single user-agent rule in robots.txt.
// This struct matches the Webflow API v2 response format for robots.txt rules.
type RobotsTxtRule struct {
	UserAgent string   `json:"userAgent"` // The user-agent this rule applies to (e.g., "*", "Googlebot")
	Allows    []string `json:"allows"`    // Paths that are allowed for this user-agent
	Disallows []string `json:"disallows"` // Paths that are disallowed for this user-agent
}

// RobotsTxtResponse represents the Webflow API response for robots.txt.
type RobotsTxtResponse struct {
	Rules   []RobotsTxtRule `json:"rules"`   // List of user-agent rules
	Sitemap string          `json:"sitemap"` // URL to the sitemap
}

// RobotsTxtRequest represents the request body for PUT/PATCH robots.txt.
type RobotsTxtRequest struct {
	Rules   []RobotsTxtRule `json:"rules,omitempty"`   // List of user-agent rules
	Sitemap string          `json:"sitemap,omitempty"` // URL to the sitemap
}

// siteIDPattern is the regex pattern for validating Webflow site IDs.
// Site IDs are 24-character lowercase hexadecimal strings.
var siteIDPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

// ValidateSiteID validates that a siteID matches the Webflow site ID format.
// Webflow site IDs are 24-character lowercase hexadecimal strings.
// During Pulumi preview, placeholder IDs (starting with "preview-") are allowed.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateSiteID(siteID string) error {
	if siteID == "" {
		return errors.New("siteId is required but was not provided. " +
			"Please provide a valid Webflow site ID " +
			"(24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c3'). " +
			"You can find your site ID in the Webflow dashboard under Site Settings")
	}
	// During Pulumi preview, dependent resources receive placeholder IDs like "preview-1234567890"
	// These must be allowed to pass validation since the real ID isn't known yet
	if strings.HasPrefix(siteID, "preview-") {
		return nil
	}
	if !siteIDPattern.MatchString(siteID) {
		return fmt.Errorf("siteId has invalid format: got '%s'. "+
			"Expected a 24-character lowercase hexadecimal string "+
			"(e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"Please check your site ID in the Webflow dashboard "+
			"and ensure it contains only lowercase letters (a-f) and digits (0-9)", siteID)
	}
	return nil
}

// GenerateRobotsTxtResourceID generates a Pulumi resource ID for a RobotsTxt resource.
// Format: {siteID}/robots.txt
func GenerateRobotsTxtResourceID(siteID string) string {
	return siteID + "/robots.txt"
}

// ExtractSiteIDFromResourceID extracts the siteID from a RobotsTxt resource ID.
// Expected format: {siteID}/robots.txt
func ExtractSiteIDFromResourceID(resourceID string) (string, error) {
	if resourceID == "" {
		return "", errors.New("resourceId cannot be empty")
	}

	parts := strings.Split(resourceID, "/")
	if len(parts) != 2 || parts[1] != "robots.txt" {
		return "", fmt.Errorf("invalid resource ID format: expected {siteId}/robots.txt, got: %s", resourceID)
	}

	return parts[0], nil
}

// ParseRobotsTxtContent parses a robots.txt content string into structured rules and sitemap.
// This converts the traditional robots.txt format into the Webflow API format.
//
// Example input:
//
//	User-agent: *
//	Allow: /
//	Disallow: /admin/
//	Sitemap: https://example.com/sitemap.xml
//
// Returns the parsed rules and sitemap URL.
func ParseRobotsTxtContent(content string) (rules []RobotsTxtRule, sitemap string) {
	if content == "" {
		return []RobotsTxtRule{}, ""
	}

	var currentRule *RobotsTxtRule

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for sitemap directive (case-insensitive)
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			sitemap = strings.TrimSpace(line[8:])
			continue
		}

		// Check for user-agent directive (case-insensitive)
		if strings.HasPrefix(strings.ToLower(line), "user-agent:") {
			// Save previous rule if exists
			if currentRule != nil {
				rules = append(rules, *currentRule)
			}
			// Start new rule
			userAgent := strings.TrimSpace(line[11:])
			currentRule = &RobotsTxtRule{
				UserAgent: userAgent,
				Allows:    []string{},
				Disallows: []string{},
			}
			continue
		}

		// Parse Allow/Disallow directives
		if currentRule != nil {
			if strings.HasPrefix(strings.ToLower(line), "allow:") {
				path := strings.TrimSpace(line[6:])
				if path != "" {
					currentRule.Allows = append(currentRule.Allows, path)
				}
			} else if strings.HasPrefix(strings.ToLower(line), "disallow:") {
				path := strings.TrimSpace(line[9:])
				if path != "" {
					currentRule.Disallows = append(currentRule.Disallows, path)
				}
			}
		}
	}

	// Don't forget the last rule
	if currentRule != nil {
		rules = append(rules, *currentRule)
	}

	return rules, sitemap
}

// FormatRobotsTxtContent formats structured rules and sitemap into a robots.txt content string.
// This converts the Webflow API format back to traditional robots.txt format.
func FormatRobotsTxtContent(rules []RobotsTxtRule, sitemap string) string {
	if len(rules) == 0 && sitemap == "" {
		return ""
	}

	var builder strings.Builder

	for _, rule := range rules {
		builder.WriteString(fmt.Sprintf("User-agent: %s\n", rule.UserAgent))

		for _, allow := range rule.Allows {
			builder.WriteString(fmt.Sprintf("Allow: %s\n", allow))
		}

		for _, disallow := range rule.Disallows {
			builder.WriteString(fmt.Sprintf("Disallow: %s\n", disallow))
		}
	}

	if sitemap != "" {
		if len(rules) > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("Sitemap: %s\n", sitemap))
	}

	return builder.String()
}

// maxRetries is the number of retries performed by the legacy per-function retry loops.
// New code relies on the retry transport and doRequest instead.
const maxRetries = 3

// GetRobotsTxt retrieves the robots.txt configuration for a Webflow site.
// It calls GET /v2/sites/{site_id}/robots_txt endpoint.
// Returns the parsed response or an error if the request fails.
func GetRobotsTxt(ctx context.Context, client *http.Client, siteID string) (*RobotsTxtResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	url := fmt.Sprintf("%s/v2/sites/%s/robots_txt", webflowAPIBaseURL, siteID)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check for Retry-After header from previous response, or use exponential backoff
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = handleNetworkError(err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Close immediately after reading
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Handle rate limiting with retry
		if resp.StatusCode == 429 {
			retryAfter := resp.Header.Get("Retry-After")
			var waitTime time.Duration
			if retryAfter != "" {
				waitTime = getRetryAfterDuration(retryAfter, time.Duration(1<<uint(attempt))*time.Second)
			} else {
				waitTime = time.Duration(1<<uint(attempt)) * time.Second
			}

			// Enhanced rate limiting error message with clear delay information
			lastErr = fmt.Errorf("rate limited: Webflow API rate limit exceeded (HTTP 429). "+
				"The provider will automatically retry with exponential backoff. "+
				"Retry attempt %d of %d, waiting %v before next attempt. "+
				"If this error persists, please wait a few minutes before trying again or contact Webflow support",
				attempt+1, maxRetries+1, waitTime)

			// Check for Retry-After header for the next retry
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
				case <-time.After(waitTime):
				}
			}
			continue
		}

		// Handle error responses
		if resp.StatusCode != 200 {
			return nil, handleWebflowError(resp.StatusCode, body)
		}

		var response RobotsTxtResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		return &response, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// PutRobotsTxt creates or updates the robots.txt configuration for a Webflow site.
// It calls PUT /v2/sites/{site_id}/robots_txt endpoint.
// Returns the updated response or an error if the request fails.
func PutRobotsTxt(
	ctx context.Context, client *http.Client,
	siteID string, rules []RobotsTxtRule, sitemap string,
) (*RobotsTxtResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	url := fmt.Sprintf("%s/v2/sites/%s/robots_txt", webflowAPIBaseURL, siteID)

	requestBody := RobotsTxtRequest{
		Rules:   rules,
		Sitemap: sitemap,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check for Retry-After header from previous response, or use exponential backoff
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = handleNetworkError(err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Close immediately after reading
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Handle rate limiting with retry
		if resp.StatusCode == 429 {
			retryAfter := resp.Header.Get("Retry-After")
			var waitTime time.Duration
			if retryAfter != "" {
				waitTime = getRetryAfterDuration(retryAfter, time.Duration(1<<uint(attempt))*time.Second)
			} else {
				waitTime = time.Duration(1<<uint(attempt)) * time.Second
			}

			// Enhanced rate limiting error message with clear delay information
			lastErr = fmt.Errorf("rate limited: Webflow API rate limit exceeded (HTTP 429). "+
				"The provider will automatically retry with exponential backoff. "+
				"Retry attempt %d of %d, waiting %v before next attempt. "+
				"If this error persists, please wait a few minutes before trying again or contact Webflow support",
				attempt+1, maxRetries+1, waitTime)

			// Check for Retry-After header for the next retry
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
				case <-time.After(waitTime):
				}
			}
			continue
		}

		// Handle error responses
		if resp.StatusCode != 200 {
			return nil, handleWebflowError(resp.StatusCode, body)
		}

		var response RobotsTxtResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		return &response, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// DeleteRobotsTxt removes the robots.txt configuration from a Webflow site.
// It calls DELETE /v2/sites/{site_id}/robots_txt endpoint.
// Returns nil on success (including 404 for idempotency) or an error if the request fails.
func DeleteRobotsTxt(ctx context.Context, client *http.Client, siteID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	url := fmt.Sprintf("%s/v2/sites/%s/robots_txt", webflowAPIBaseURL, siteID)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check for Retry-After header from previous response, or use exponential backoff
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "DELETE", url, http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = handleNetworkError(err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Close immediately after reading
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Handle rate limiting with retry
		if resp.StatusCode == 429 {
			retryAfter := resp.Header.Get("Retry-After")
			var waitTime time.Duration
			if retryAfter != "" {
				waitTime = getRetryAfterDuration(retryAfter, time.Duration(1<<uint(attempt))*time.Second)
			} else {
				waitTime = time.Duration(1<<uint(attempt)) * time.Second
			}

			// Enhanced rate limiting error message with clear delay information
			lastErr = fmt.Errorf("rate limited: Webflow API rate limit exceeded (HTTP 429). "+
				"The provider will automatically retry with exponential backoff. "+
				"Retry attempt %d of %d, waiting %v before next attempt. "+
				"If this error persists, please wait a few minutes before trying again or contact Webflow support",
				attempt+1, maxRetries+1, waitTime)

			// Check for Retry-After header for the next retry
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
				case <-time.After(waitTime):
				}
			}
			continue
		}

		// 204 No Content is success
		// 404 Not Found is also success (idempotent delete)
		if resp.StatusCode == 204 || resp.StatusCode == 404 {
			return nil
		}

		// Handle other error responses
		return handleWebflowError(resp.StatusCode, body)
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
