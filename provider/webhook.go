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

// WebhookResponse represents a webhook configuration in Webflow.
// This struct matches the Webflow API v2 response format for webhooks.
type WebhookResponse struct {
	ID            string                 `json:"id,omitempty"`            // Webflow-assigned webhook ID
	TriggerType   string                 `json:"triggerType"`             // Event that triggers the webhook
	URL           string                 `json:"url"`                     // HTTPS endpoint to receive webhook
	WorkspaceID   string                 `json:"workspaceId,omitempty"`   // Workspace ID (read-only)
	SiteID        string                 `json:"siteId"`                  // Site ID
	LastTriggered string                 `json:"lastTriggered,omitempty"` // Last trigger timestamp (read-only)
	CreatedOn     string                 `json:"createdOn,omitempty"`     // Creation timestamp (read-only)
	Filter        map[string]interface{} `json:"filter,omitempty"`        // Optional event filter
}

// WebhooksListResponse represents the Webflow API response for listing webhooks.
// The list endpoint is not paginated.
type WebhooksListResponse struct {
	Webhooks []WebhookResponse `json:"webhooks"` // List of webhooks
}

// WebhookRequest represents the request body for POST webhooks.
type WebhookRequest struct {
	TriggerType string                 `json:"triggerType"`      // Event that triggers the webhook
	URL         string                 `json:"url"`              // HTTPS endpoint to receive webhook
	Filter      map[string]interface{} `json:"filter,omitempty"` // Optional event filter
}

// webhookIDPattern is the regex pattern for validating Webflow webhook IDs.
// Webhook IDs are 24-character lowercase hexadecimal strings (same format as site IDs).
var webhookIDPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

// validTriggerTypeList is the set of webhook trigger types documented by Webflow
// (Data API v2 webhook `triggerType` enum, see https://developers.webflow.com/data/reference/all-events).
var validTriggerTypeList = []string{
	"form_submission",
	"site_publish",
	"page_created",
	"page_metadata_updated",
	"page_deleted",
	"ecomm_new_order",
	"ecomm_order_changed",
	"ecomm_inventory_changed",
	"collection_item_created",
	"collection_item_changed",
	"collection_item_deleted",
	"collection_item_published",
	"collection_item_unpublished",
	"comment_created",
}

// validTriggerTypes indexes validTriggerTypeList for O(1) lookup.
var validTriggerTypes = func() map[string]bool {
	m := make(map[string]bool, len(validTriggerTypeList))
	for _, t := range validTriggerTypeList {
		m[t] = true
	}
	return m
}()

// ValidateWebhookID validates that a webhookID matches the Webflow webhook ID format.
// Webhook IDs are 24-character lowercase hexadecimal strings.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateWebhookID(webhookID string) error {
	if webhookID == "" {
		return errors.New("webhookId is required but was not provided. " +
			"Please provide a valid Webflow webhook ID " +
			"(24-character lowercase hexadecimal string, e.g., '5f0c8c9e1c9d440000e8d8c3'). " +
			"Webhook IDs are assigned by Webflow when a webhook is created")
	}
	if !webhookIDPattern.MatchString(webhookID) {
		return fmt.Errorf("webhookId has invalid format: got '%s'. "+
			"Expected a 24-character lowercase hexadecimal string "+
			"(e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"Please check the webhook ID and ensure it contains only lowercase letters (a-f) and digits (0-9)", webhookID)
	}
	return nil
}

// ValidateWebhookURL validates that a webhook URL is a valid HTTPS endpoint.
// Webflow requires webhook URLs to use HTTPS for security.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateWebhookURL(url string) error {
	if url == "" {
		return errors.New("url is required but was not provided. " +
			"Please provide a valid HTTPS URL where Webflow should send webhook events " +
			"(e.g., 'https://example.com/webhooks/webflow', 'https://api.example.com/events'). " +
			"Note: Webflow requires HTTPS URLs for security")
	}
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("url must use HTTPS protocol: got '%s'. "+
			"Webflow requires webhook URLs to use HTTPS for security. "+
			"Example valid URLs: 'https://example.com/webhooks/webflow', 'https://api.example.com/events'. "+
			"Please update the URL to use HTTPS instead of HTTP", url)
	}
	// Basic URL format validation
	if !strings.Contains(url[8:], ".") {
		return fmt.Errorf("url appears to be invalid: got '%s'. "+
			"Expected format: https://domain.com/path. "+
			"Example valid URLs: 'https://example.com/webhooks', 'https://api.example.com/events'. "+
			"Please provide a valid HTTPS URL", url)
	}
	return nil
}

// ValidateTriggerType validates that a triggerType is a recognized Webflow event.
// Returns actionable error messages listing all valid trigger types.
func ValidateTriggerType(triggerType string) error {
	valid := strings.Join(validTriggerTypeList, ", ")
	if triggerType == "" {
		return fmt.Errorf("triggerType is required but was not provided, "+
			"please specify which Webflow event should trigger this webhook, "+
			"valid trigger types: %s, "+
			"example: 'form_submission' for form submissions, 'site_publish' for site publishes", valid)
	}
	if !validTriggerTypes[triggerType] {
		return fmt.Errorf("triggerType '%s' is not a valid Webflow event type, "+
			"valid trigger types are: %s, "+
			"please use one of these valid trigger types, "+
			"example: 'form_submission' for form submissions, 'site_publish' for site publishes", triggerType, valid)
	}
	return nil
}

// GenerateWebhookResourceID generates a Pulumi resource ID for a Webhook resource.
// Format: {siteID}/webhooks/{webhookID}
func GenerateWebhookResourceID(siteID, webhookID string) string {
	return fmt.Sprintf("%s/webhooks/%s", siteID, webhookID)
}

// ExtractIDsFromWebhookResourceID extracts the siteID and webhookID from a Webhook resource ID.
// Expected format: {siteID}/webhooks/{webhookID}. Both IDs must be non-empty.
func ExtractIDsFromWebhookResourceID(resourceID string) (siteID, webhookID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.SplitN(resourceID, "/", 3)
	if len(parts) != 3 || parts[1] != "webhooks" || parts[0] == "" || parts[2] == "" {
		return "", "", fmt.Errorf("invalid resource ID format: expected {siteId}/webhooks/{webhookId}, got: %s", resourceID)
	}

	return parts[0], parts[2], nil
}

// GetWebhooks retrieves all App-created webhooks for a Webflow site.
// It calls GET /v2/sites/{site_id}/webhooks.
func GetWebhooks(ctx context.Context, client *http.Client, siteID string) (*WebhooksListResponse, error) {
	var out WebhooksListResponse
	if _, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/sites/%s/webhooks", siteID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindWebhook lists the site's webhooks and returns the one with webhookID.
// When it does not exist the returned error satisfies IsNotFound.
func FindWebhook(ctx context.Context, client *http.Client, siteID, webhookID string) (*WebhookResponse, error) {
	response, err := GetWebhooks(ctx, client, siteID)
	if err != nil {
		return nil, err
	}
	for i := range response.Webhooks {
		if response.Webhooks[i].ID == webhookID {
			webhook := response.Webhooks[i]
			return &webhook, nil
		}
	}
	return nil, fmt.Errorf("webhook '%s' on site '%s': %w", webhookID, siteID, ErrNotFound)
}

// PostWebhook creates a new webhook for a Webflow site.
// It calls POST /v2/sites/{site_id}/webhooks.
func PostWebhook(
	ctx context.Context, client *http.Client, siteID string, request WebhookRequest,
) (*WebhookResponse, error) {
	var out WebhookResponse
	if _, err := doRequest(ctx, client, http.MethodPost,
		apiURL("/v2/sites/%s/webhooks", siteID), request, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhook removes a webhook from Webflow.
// It calls DELETE /v2/webhooks/{webhook_id}; 404 is treated as success (idempotent).
func DeleteWebhook(ctx context.Context, client *http.Client, webhookID string) error {
	return doDelete(ctx, client, apiURL("/v2/webhooks/%s", webhookID), nil)
}
