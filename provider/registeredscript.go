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
	"unicode/utf8"

	p "github.com/pulumi/pulumi-go-provider"
)

// RegisteredScript represents a registered custom code script in Webflow.
// This struct matches the Webflow API v2 response format for registered scripts.
// The list endpoint returns both hosted and inline scripts in this shape.
type RegisteredScript struct {
	ID             string `json:"id,omitempty"`             // Human-readable ID derived from display name (read-only)
	DisplayName    string `json:"displayName"`              // User-facing name for the script (1-50 alphanumeric chars)
	HostedLocation string `json:"hostedLocation,omitempty"` // URI for the hosted script (Webflow-set for inline)
	IntegrityHash  string `json:"integrityHash,omitempty"`  // Sub-Resource Integrity Hash (SRI)
	CanCopy        bool   `json:"canCopy"`                  // Whether script can be copied on site duplication
	Version        string `json:"version"`                  // Semantic Version (SemVer) string
	CreatedOn      string `json:"createdOn,omitempty"`      // Timestamp when created (read-only)
	LastUpdated    string `json:"lastUpdated,omitempty"`    // Timestamp when last updated (read-only)
}

// RegisteredScriptsResponse represents one page of the Webflow list registered scripts response.
type RegisteredScriptsResponse struct {
	RegisteredScripts []RegisteredScript `json:"registeredScripts"`
	Pagination        PaginationInfo     `json:"pagination,omitempty"`
}

// PaginationInfo represents pagination metadata from the API response.
type PaginationInfo struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// RegisteredScriptRequest represents the request body for POST /registered_scripts/hosted.
type RegisteredScriptRequest struct {
	DisplayName    string `json:"displayName"`
	HostedLocation string `json:"hostedLocation"`
	IntegrityHash  string `json:"integrityHash"`
	CanCopy        bool   `json:"canCopy,omitempty"`
	Version        string `json:"version"`
}

// maxDisplayNameLength is the maximum number of characters in a script display name.
const maxDisplayNameLength = 50

// displayNamePattern validates that display names are 1-50 letters, digits or spaces.
// Webflow's own request example is "CMS Slider", so spaces must be accepted; anything
// stricter is left to the API.
var displayNamePattern = regexp.MustCompile(`^[a-zA-Z0-9 ]{1,50}$`)

// ValidateScriptDisplayName validates that a displayName is a valid Webflow script name:
// 1-50 characters made of letters, digits and spaces.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateScriptDisplayName(name string) error {
	if name == "" {
		return errors.New("displayName is required but was not provided. " +
			"Please provide a user-facing name for the script between 1 and 50 characters " +
			"(letters, digits and spaces). " +
			"Example valid names: 'CMS Slider', 'AnalyticsScript', 'MyCustomScript123'")
	}
	if n := utf8.RuneCountInString(name); n > maxDisplayNameLength {
		return fmt.Errorf("displayName is too long: got %d characters, maximum is %d. "+
			"Please shorten the name. "+
			"Example valid names: 'CMS Slider', 'AnalyticsScript', 'MyCustomScript123'", n, maxDisplayNameLength)
	}
	if !displayNamePattern.MatchString(name) {
		return fmt.Errorf("displayName contains invalid characters: got '%s'. "+
			"Allowed characters: A-Z, a-z, 0-9 and spaces. "+
			"Example valid names: 'CMS Slider', 'AnalyticsScript', 'MyCustomScript123'", name)
	}
	return nil
}

// ValidateHostedLocation validates that a hostedLocation is a valid HTTP/HTTPS URL.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateHostedLocation(url string) error {
	if url == "" {
		return errors.New("hostedLocation is required but was not provided. " +
			"Please provide a valid HTTP or HTTPS URL where your script is hosted. " +
			"Example: 'https://cdn.example.com/my-script.js'")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("hostedLocation must start with 'http://' or 'https://': got '%s'. "+
			"Example valid URLs: 'https://cdn.example.com/my-script.js', 'https://cdnjs.cloudflare.com/...'. "+
			"Please ensure the URL is properly formatted with a scheme", url)
	}
	return nil
}

// ValidateIntegrityHash validates that an integrityHash is a properly formatted SRI hash.
// Should be in format: sha384-<hash> or sha256-<hash> or sha512-<hash>
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateIntegrityHash(hash string) error {
	if hash == "" {
		return errors.New("integrityHash is required but was not provided. " +
			"Please provide a Sub-Resource Integrity (SRI) hash for your hosted script. " +
			"Format: 'sha384-<hash>', 'sha256-<hash>', or 'sha512-<hash>'. " +
			"You can generate an SRI hash using: https://www.srihash.org/")
	}
	if !strings.HasPrefix(hash, "sha") {
		return fmt.Errorf("integrityHash must start with 'sha': got '%s'. "+
			"Supported algorithms: sha256, sha384, sha512. "+
			"Format: 'sha384-<hash>', 'sha256-<hash>', or 'sha512-<hash>'. "+
			"You can generate an SRI hash using: https://www.srihash.org/", hash)
	}
	return nil
}

// ValidateVersion validates that a version is a proper semantic version string.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateVersion(version string) error {
	if version == "" {
		return errors.New("version is required but was not provided. " +
			"Please provide a Semantic Version (SemVer) string for your script. " +
			"Format: 'major.minor.patch' (e.g., '1.0.0', '2.3.1'). " +
			"See https://semver.org/ for more information")
	}
	if !strings.Contains(version, ".") {
		return fmt.Errorf("version must be in Semantic Version format: got '%s'. "+
			"Expected format: 'major.minor.patch' (e.g., '1.0.0', '2.3.1'). "+
			"See https://semver.org/ for more information", version)
	}
	return nil
}

// validateScriptResourceIDs validates the IDs parsed from a RegisteredScript or InlineScript
// resource ID before they are used to build API URLs.
func validateScriptResourceIDs(siteID, scriptID string) error {
	if err := ValidateSiteID(siteID); err != nil {
		return err
	}
	if scriptID == "" {
		return errors.New("script ID in resource ID cannot be empty")
	}
	return nil
}

// GenerateRegisteredScriptResourceID generates a Pulumi resource ID for a RegisteredScript resource.
// Format: {siteID}/registered_scripts/{scriptID}
func GenerateRegisteredScriptResourceID(siteID, scriptID string) string {
	return fmt.Sprintf("%s/registered_scripts/%s", siteID, scriptID)
}

// ExtractIDsFromRegisteredScriptResourceID extracts the siteID and scriptID from a RegisteredScript resource ID.
// Expected format: {siteID}/registered_scripts/{scriptID}. Both IDs must be non-empty.
func ExtractIDsFromRegisteredScriptResourceID(resourceID string) (siteID, scriptID string, err error) {
	return splitScriptResourceID(resourceID, "registered_scripts")
}

// splitScriptResourceID parses "{siteID}/{segment}/{scriptID}" resource IDs.
func splitScriptResourceID(resourceID, segment string) (siteID, scriptID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.SplitN(resourceID, "/", 3)
	if len(parts) != 3 || parts[1] != segment || parts[0] == "" || parts[2] == "" {
		return "", "",
			fmt.Errorf("invalid resource ID format: expected {siteId}/%s/{scriptId}, got: %s", segment, resourceID)
	}

	return parts[0], parts[2], nil
}

// GetRegisteredScripts retrieves one page of the scripts registered to a Webflow site.
// It calls GET /v2/sites/{site_id}/registered_scripts, adding ?offset=N when offset > 0.
// The response's pagination block says how many scripts exist in total.
// Use FindRegisteredScript to locate a specific script across pages.
func GetRegisteredScripts(
	ctx context.Context, client *http.Client, siteID string, offset int,
) (*RegisteredScriptsResponse, error) {
	url := apiURL("/v2/sites/%s/registered_scripts", siteID)
	if offset > 0 {
		url = apiURL("/v2/sites/%s/registered_scripts?offset=%d", siteID, offset)
	}
	var out RegisteredScriptsResponse
	if _, err := doRequest(ctx, client, http.MethodGet, url, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindRegisteredScript looks up a registered (hosted or inline) script by ID, following the
// list endpoint's pagination (total/limit/offset) until the script is found or the list is
// exhausted. When the script does not exist the returned error satisfies IsNotFound.
func FindRegisteredScript(
	ctx context.Context, client *http.Client, siteID, scriptID string,
) (*RegisteredScript, error) {
	offset := 0
	for {
		page, err := GetRegisteredScripts(ctx, client, siteID, offset)
		if err != nil {
			return nil, err
		}
		for i := range page.RegisteredScripts {
			if page.RegisteredScripts[i].ID == scriptID {
				script := page.RegisteredScripts[i]
				return &script, nil
			}
		}
		// Advance by the number of items actually returned so a server that ignores
		// or clamps offset cannot make this loop forever.
		n := len(page.RegisteredScripts)
		offset += n
		if n == 0 || offset >= page.Pagination.Total {
			return nil, fmt.Errorf("registered script '%s' on site '%s': %w", scriptID, siteID, ErrNotFound)
		}
	}
}

// PostRegisteredScript registers an externally hosted script on a Webflow site.
// It calls POST /v2/sites/{site_id}/registered_scripts/hosted.
func PostRegisteredScript(
	ctx context.Context, client *http.Client, siteID string, request RegisteredScriptRequest,
) (*RegisteredScript, error) {
	var out RegisteredScript
	if _, err := doRequest(ctx, client, http.MethodPost,
		apiURL("/v2/sites/%s/registered_scripts/hosted", siteID), request, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Note: the Webflow Data API has no endpoint to delete or unregister a registered script.
// The custom code reference lists only List Registered Scripts, Register Hosted Script,
// Register Inline Script and List Custom Code Blocks, so registrations are permanent (a
// site can hold up to 800). RegisteredScript.Delete and InlineScript.Delete are therefore
// no-ops; applied code is removed through SiteCustomCode and PageCustomCode instead.

// scriptDeleteNoOpWarning logs that a registered script stays registered after Delete.
func scriptDeleteNoOpWarning(ctx context.Context, resource, siteID, scriptID string) {
	p.GetLogger(ctx).Warningf(
		"%s '%s' on site '%s': the Webflow API has no endpoint to unregister a script, so the "+
			"registration stays in the site's script registry (Pulumi simply stops managing it). "+
			"Scripts applied via SiteCustomCode or PageCustomCode are removed by those resources.",
		resource, scriptID, siteID)
}

// customCodeScopesNote is the shared documentation on token requirements for the custom
// code endpoints.
const customCodeScopesNote = "**Authentication:** this resource calls Webflow custom code endpoints, which " +
	"require an OAuth Data Client app token with the `custom_code:read` and `custom_code:write` scopes. " +
	"Webflow documents that these scopes are available only to Data Client apps: site API tokens " +
	"cannot access custom code endpoints. "
