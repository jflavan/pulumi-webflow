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

// EcommerceSettingsResponse represents the Webflow API response for ecommerce settings.
// This struct matches the GET /v2/sites/{site_id}/ecommerce/settings endpoint response.
type EcommerceSettingsResponse struct {
	// SiteID is the identifier of the Site.
	SiteID string `json:"siteId"`
	// CreatedOn is the date when the ecommerce settings were created (ISO 8601 format).
	CreatedOn string `json:"createdOn"`
	// DefaultCurrency is the three-letter ISO currency code for the Site (e.g., "USD", "EUR").
	DefaultCurrency string `json:"defaultCurrency"`
}

// currencyCodePattern is the regex pattern for validating ISO 4217 currency codes.
var currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

// ValidateCurrencyCode validates that a currency code is a valid 3-letter ISO 4217 code.
func ValidateCurrencyCode(code string) error {
	if code == "" {
		return errors.New("defaultCurrency is required but was not provided. " +
			"Please provide a valid 3-letter ISO 4217 currency code (e.g., 'USD', 'EUR', 'GBP'). " +
			"You can find a full list of currency codes at https://www.iso.org/iso-4217-currency-codes.html")
	}
	if !currencyCodePattern.MatchString(code) {
		return fmt.Errorf("defaultCurrency has invalid format: got '%s'. "+
			"Expected a 3-letter uppercase ISO 4217 currency code (e.g., 'USD', 'EUR', 'GBP'). "+
			"Currency codes must be exactly 3 uppercase letters. "+
			"Common codes: USD (US Dollar), EUR (Euro), GBP (British Pound), JPY (Japanese Yen)", code)
	}
	return nil
}

// GenerateEcommerceSettingsResourceID generates a Pulumi resource ID: {siteID}/ecommerce/settings
func GenerateEcommerceSettingsResourceID(siteID string) string {
	return siteID + "/ecommerce/settings"
}

// ExtractSiteIDFromEcommerceSettingsResourceID extracts the siteID from {siteID}/ecommerce/settings.
func ExtractSiteIDFromEcommerceSettingsResourceID(resourceID string) (string, error) {
	if resourceID == "" {
		return "", errors.New("resourceId cannot be empty")
	}

	suffix := "/ecommerce/settings"
	if len(resourceID) <= len(suffix) || !strings.HasSuffix(resourceID, suffix) {
		return "", fmt.Errorf("invalid resource ID format: expected {siteId}/ecommerce/settings, got: %s", resourceID)
	}

	siteID := strings.TrimSuffix(resourceID, suffix)
	if siteID == "" {
		return "", fmt.Errorf("invalid resource ID format: expected {siteId}/ecommerce/settings, got: %s", resourceID)
	}

	return siteID, nil
}

// EcommerceNotEnabledError is returned when Webflow answers 409 Conflict for the ecommerce
// settings endpoint, which means ecommerce is not enabled on the site.
type EcommerceNotEnabledError struct {
	// Details is the (truncated) response body from Webflow.
	Details string
}

// Error returns actionable guidance.
func (e *EcommerceNotEnabledError) Error() string {
	details := strings.TrimSpace(e.Details)
	if details == "" {
		details = "no response body"
	}
	return "ecommerce not enabled: the site does not have ecommerce enabled. " +
		"To fix this: 1) Log into your Webflow dashboard, 2) Go to Site Settings > Ecommerce, " +
		"3) Enable ecommerce for this site, 4) Set up your payment provider and currency settings, " +
		"5) Retry this operation. Details: " + details
}

// IsEcommerceNotEnabled reports whether err is an EcommerceNotEnabledError.
func IsEcommerceNotEnabled(err error) bool {
	var target *EcommerceNotEnabledError
	return errors.As(err, &target)
}

// handleEcommerceNotEnabledError creates the typed error from a response body.
func handleEcommerceNotEnabledError(body []byte) error {
	return &EcommerceNotEnabledError{Details: TruncateForLogging(string(body), maxErrorBodyLength)}
}

// GetEcommerceSettings retrieves the ecommerce settings for a Webflow site.
// It calls GET /v2/sites/{site_id}/ecommerce/settings. Requires the ecommerce:read scope.
// A 409 Conflict (ecommerce not enabled) is returned as *EcommerceNotEnabledError; other
// non-success responses are *APIError.
func GetEcommerceSettings(ctx context.Context, client *http.Client, siteID string) (*EcommerceSettingsResponse, error) {
	var response EcommerceSettingsResponse
	_, err := doRequest(ctx, client, http.MethodGet, apiURL("/v2/sites/%s/ecommerce/settings", siteID), nil, &response)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			return nil, handleEcommerceNotEnabledError([]byte(apiErr.Body))
		}
		return nil, err
	}
	return &response, nil
}
