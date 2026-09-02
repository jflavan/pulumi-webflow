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
	"regexp"
	"strings"
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

// RobotsTxtRequest represents the request body for PUT/PATCH/DELETE robots.txt.
type RobotsTxtRequest struct {
	Rules   []RobotsTxtRule `json:"rules,omitempty"`   // List of user-agent rules
	Sitemap string          `json:"sitemap,omitempty"` // URL to the sitemap
}

// siteIDPattern is the regex pattern for validating Webflow site IDs.
// Site IDs are 24-character lowercase hexadecimal strings.
var siteIDPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

// ValidateSiteID validates that a siteID matches the Webflow site ID format.
// Webflow site IDs are 24-character lowercase hexadecimal strings.
// Unknown values never reach this function: during preview an unknown siteId is either
// skipped by Check or arrives zeroed in a DryRun Create/Update, which do not validate.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateSiteID(siteID string) error {
	if siteID == "" {
		return errors.New("siteId is required but was not provided. " +
			"Please provide a valid Webflow site ID " +
			"(24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c3'). " +
			"You can find your site ID in the Webflow dashboard under Site Settings")
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
// Returns the parsed rules and sitemap URL. Comments and directives the Webflow API cannot
// store are dropped; use ParseRobotsTxtContentWithWarnings to learn what was dropped.
func ParseRobotsTxtContent(content string) (rules []RobotsTxtRule, sitemap string) {
	rules, sitemap, _ = ParseRobotsTxtContentWithWarnings(content)
	return rules, sitemap
}

// ParseRobotsTxtContentWithWarnings parses robots.txt content and also reports, one message per
// line, every comment or directive that the Webflow API cannot represent and that is therefore
// not sent (and will not come back on refresh).
func ParseRobotsTxtContentWithWarnings(content string) (rules []RobotsTxtRule, sitemap string, warnings []string) {
	if content == "" {
		return []RobotsTxtRule{}, "", nil
	}

	var currentRule *RobotsTxtRule

	lines := strings.Split(content, "\n")
	for i, rawLine := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			warnings = append(
				warnings,
				fmt.Sprintf("line %d: comment %q is not stored by the Webflow API and will be dropped", lineNo, line),
			)
			continue
		}

		// Strip trailing inline comments ("Disallow: /admin # private")
		if idx := strings.Index(line, "#"); idx > 0 {
			warnings = append(
				warnings,
				fmt.Sprintf("line %d: inline comment %q will be dropped", lineNo, strings.TrimSpace(line[idx:])),
			)
			line = strings.TrimSpace(line[:idx])
		}

		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "sitemap:"):
			sitemap = strings.TrimSpace(line[8:])
		case strings.HasPrefix(lower, "user-agent:"):
			if currentRule != nil {
				rules = append(rules, *currentRule)
			}
			currentRule = &RobotsTxtRule{
				UserAgent: strings.TrimSpace(line[11:]),
				Allows:    []string{},
				Disallows: []string{},
			}
		case strings.HasPrefix(lower, "allow:"):
			path := strings.TrimSpace(line[6:])
			if currentRule == nil {
				warnings = append(
					warnings,
					fmt.Sprintf("line %d: %q appears before any User-agent directive and will be dropped", lineNo, line),
				)
			} else if path != "" {
				currentRule.Allows = append(currentRule.Allows, path)
			}
		case strings.HasPrefix(lower, "disallow:"):
			path := strings.TrimSpace(line[9:])
			if currentRule == nil {
				warnings = append(
					warnings,
					fmt.Sprintf("line %d: %q appears before any User-agent directive and will be dropped", lineNo, line),
				)
			} else if path != "" {
				currentRule.Disallows = append(currentRule.Disallows, path)
			}
		default:
			warnings = append(warnings, fmt.Sprintf("line %d: directive %q is not supported by the Webflow API "+
				"(only User-agent, Allow, Disallow and Sitemap are stored) and will be dropped", lineNo, line))
		}
	}

	// Don't forget the last rule
	if currentRule != nil {
		rules = append(rules, *currentRule)
	}

	return rules, sitemap, warnings
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

// RobotsTxtContentEqual reports whether two robots.txt documents describe the same rules and
// sitemap once parsed, ignoring formatting, blank lines, comments and directive casing.
func RobotsTxtContentEqual(a, b string) bool {
	rulesA, sitemapA := ParseRobotsTxtContent(a)
	rulesB, sitemapB := ParseRobotsTxtContent(b)
	return sitemapA == sitemapB && robotsTxtRulesEqual(rulesA, rulesB)
}

// robotsTxtRulesEqual compares two rule lists, treating nil and empty slices as equal.
func robotsTxtRulesEqual(a, b []RobotsTxtRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].UserAgent != b[i].UserAgent ||
			!stringSlicesEqual(a[i].Allows, b[i].Allows) ||
			!stringSlicesEqual(a[i].Disallows, b[i].Disallows) {
			return false
		}
	}
	return true
}

// stringSlicesEqual compares two string slices element-wise, treating nil and empty as equal.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GetRobotsTxt retrieves the robots.txt configuration for a Webflow site.
// It calls GET /v2/sites/{site_id}/robots_txt endpoint.
// Returns the parsed response or an error if the request fails.
func GetRobotsTxt(ctx context.Context, client *http.Client, siteID string) (*RobotsTxtResponse, error) {
	var response RobotsTxtResponse
	_, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/sites/%s/robots_txt", siteID),
		nil, &response, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// PutRobotsTxt replaces the robots.txt configuration for a Webflow site.
// It calls PUT /v2/sites/{site_id}/robots_txt endpoint.
// Returns the updated response or an error if the request fails.
func PutRobotsTxt(
	ctx context.Context, client *http.Client,
	siteID string, rules []RobotsTxtRule, sitemap string,
) (*RobotsTxtResponse, error) {
	requestBody := RobotsTxtRequest{
		Rules:   rules,
		Sitemap: sitemap,
	}

	var response RobotsTxtResponse
	_, err := doRequest(ctx, client, http.MethodPut, apiURL("/v2/sites/%s/robots_txt", siteID),
		requestBody, &response, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteRobotsTxt removes rules from a site's robots.txt configuration.
// It calls DELETE /v2/sites/{site_id}/robots_txt, which requires a body listing the rules
// (and optional sitemap) to remove and answers 200 with the remaining configuration.
// Returns nil on success (200/204, and 404 for idempotency) or an error if the request fails.
// See https://developers.webflow.com/data/reference/enterprise/site-configuration/robots-txt/delete
func DeleteRobotsTxt(
	ctx context.Context, client *http.Client, siteID string, rules []RobotsTxtRule, sitemap string,
) error {
	requestBody := RobotsTxtRequest{
		Rules:   rules,
		Sitemap: sitemap,
	}
	if requestBody.Rules == nil {
		requestBody.Rules = []RobotsTxtRule{}
	}
	return doDelete(ctx, client, apiURL("/v2/sites/%s/robots_txt", siteID), requestBody)
}
