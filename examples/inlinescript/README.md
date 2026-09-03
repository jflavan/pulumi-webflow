# InlineScript Resource Examples

This directory contains examples demonstrating how to register and manage inline custom code scripts in Webflow's script registry using Pulumi.

## What You'll Learn

- Register inline JavaScript code snippets directly (no external hosting needed)
- Manage script versions with semantic versioning
- Control script copying behavior during site duplication
- Understand the 2000-character source code limit

## Available Languages

| Language   | Directory    | Entry Point    | Dependencies        |
|------------|--------------|----------------|---------------------|
| TypeScript | `typescript/`| `index.ts`     | `package.json`      |

## Required Token

The script registry endpoints are only available to **Data Client (OAuth app) tokens** with the
`custom_code:read` and `custom_code:write` scopes; site API tokens receive `401`/`403`. Pass the
app's access token as `webflow:apiToken` (or `WEBFLOW_API_TOKEN`).

## Quick Start

### TypeScript

```bash
cd typescript
npm install
pulumi stack init dev
pulumi config set siteId your-site-id --secret
pulumi up
```

## Registry Lifecycle

Webflow's registry is append-only: there is **no unregister endpoint**.

- `pulumi destroy` removes the resource from Pulumi state without calling the API; the
  registration stays in Webflow. A site can hold at most **800** registered scripts (inline and
  hosted combined).
- `displayName` and `scriptVersion` identify a registration. Changing either - or the
  `sourceCode` - registers a **new** script and leaves the previous one in place (Pulumi reports a
  replacement; nothing is deleted).

## Examples Included

### 1. Analytics Tracking Snippet

Register a small analytics tracking script directly as inline code.

```typescript
const analyticsSnippet = new webflow.InlineScript("analytics-snippet", {
  siteId: siteId,
  displayName: "AnalyticsSnippet",
  sourceCode: `(function() {
  window.dataLayer = window.dataLayer || [];
  function gtag() { dataLayer.push(arguments); }
  gtag('js', new Date());
  gtag('config', 'G-XXXXXXXXXX');
})();`,
  scriptVersion: "1.0.0",
  canCopy: true,
});
```

### 2. Cookie Consent Banner

Register an inline script that adds a cookie consent banner to your site.

```typescript
const cookieConsent = new webflow.InlineScript("cookie-consent", {
  siteId: siteId,
  displayName: "CookieConsent",
  sourceCode: `document.addEventListener('DOMContentLoaded', function() {
  // ... cookie consent logic
});`,
  scriptVersion: "1.2.0",
  canCopy: true,
});
```

### 3. Scroll-to-Top Button

Register a UI enhancement script for a floating scroll-to-top button.

```typescript
const scrollToTop = new webflow.InlineScript("scroll-to-top", {
  siteId: siteId,
  displayName: "ScrollToTop",
  sourceCode: `document.addEventListener('DOMContentLoaded', function() {
  // ... scroll-to-top logic
});`,
  scriptVersion: "2.0.0",
  canCopy: false,
});
```

## Configuration

Each example requires the following configuration:

| Config Key        | Required | Description                              |
|-------------------|----------|------------------------------------------|
| `siteId`  | Yes      | Your Webflow site ID (stored as secret)  |
| `environment`     | No       | Deployment environment (default: development) |

## Expected Output

After successful deployment, you'll see exports like:

```
Outputs:
    deployedSiteId               : [secret]
    analyticsSnippetId           : "abc123..."
    cookieConsentId              : "def456..."
    scrollToTopId                : "ghi789..."
    analyticsSnippetCreatedOn    : "2025-01-06T12:34:56Z"
    analyticsSnippetLastUpdated  : "2025-01-06T12:34:56Z"
```

## InlineScript vs RegisteredScript

| Feature | InlineScript | RegisteredScript |
|---------|-------------|-----------------|
| Code location | Embedded directly | Externally hosted URL |
| Max size | 2000 characters | No limit (hosted externally) |
| Use case | Small snippets, trackers | Large libraries, frameworks |
| SRI hash | Optional | Required |
| API endpoint | `/registered_scripts/inline` | `/registered_scripts/hosted` |

**When to use InlineScript:**
- Small tracking pixels and analytics snippets
- Simple UI enhancements (scroll buttons, banners)
- Configuration scripts under 2000 characters
- Scripts that don't need external hosting infrastructure

**When to use RegisteredScript:**
- Large JavaScript libraries
- Scripts served from a CDN
- Code that exceeds the 2000-character limit
- Scripts requiring SRI hash verification

## Script Requirements

### Display Name
- 1-50 alphanumeric characters only
- Examples: `AnalyticsSnippet`, `CookieConsent`, `ScrollToTop`
- Invalid: `my-script`, `script_v1`, `cookie consent`

### Source Code
- Maximum 2000 characters
- Must be valid JavaScript
- If your script exceeds 2000 characters, use RegisteredScript with external hosting instead

### Version
- Must follow Semantic Versioning (SemVer): `major.minor.patch`
- Examples: `1.0.0`, `2.3.1`, `0.1.0`
- Invalid: `v1.0`, `1.0`, `1.0.0-beta`

## Using Inline Scripts

Once registered, inline scripts can be deployed using:

1. **SiteCustomCode** - Apply scripts to all pages of a site
2. **PageCustomCode** - Apply scripts to specific pages

See the `sitecustomcode` and `pagecustomcode` examples for usage.

## Cleanup

To stop managing the inline scripts:

```bash
pulumi destroy
pulumi stack rm dev
```

**Note:** `pulumi destroy` does not call the Webflow API - the registry has no unregister
endpoint, so the scripts remain registered (and still count toward the 800-per-site limit). Pages
and site-wide configurations that reference the script keep working; remove those references with
`SiteCustomCode` / `PageCustomCode` if the script should stop loading.

## Troubleshooting

### "sourceCode is too long" Error

Inline scripts are limited to 2000 characters. If your script is larger, consider:
- Minifying the code to reduce size
- Using the `RegisteredScript` resource with a `hostedLocation` instead

### "Invalid displayName" Error

Display names must be 1-50 alphanumeric characters only:
- Valid: `AnalyticsSnippet`, `MyScript123`
- Invalid: `my-script`, `Analytics Snippet`

### "Invalid scriptVersion" Error

Version must be valid SemVer:
- Valid: `1.0.0`, `2.3.1`
- Invalid: `v1.0.0`, `1.0`, `1.0.0-beta`

### 401 / 403 From the Registry Endpoints

The token is a site API token or lacks `custom_code:write`. Use a Data Client (OAuth app) token
with `custom_code:read` and `custom_code:write`.

### Script Not Loading on Page

1. Verify the script was registered successfully (check Pulumi outputs)
2. Check browser console for JavaScript errors in your inline code
3. Ensure the script is applied via SiteCustomCode or PageCustomCode

## Related Resources

- [RegisteredScript Example](../registeredscript/) - For externally hosted scripts
- [SiteCustomCode Example](../sitecustomcode/)
- [PageCustomCode Example](../pagecustomcode/)
- [Main Examples Index](../README.md)
- [Semantic Versioning Guide](https://semver.org/)
