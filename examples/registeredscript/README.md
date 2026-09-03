# RegisteredScript Resource Examples

This directory contains examples demonstrating how to register and manage custom JavaScript scripts in Webflow's script registry using Pulumi.

## What You'll Learn

- Register externally hosted JavaScript scripts with version control
- Implement Sub-Resource Integrity (SRI) hashes for security
- Manage multiple versions of the same script
- Control script copying behavior during site duplication

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
  registration stays in Webflow. A site can hold at most **800** registered scripts.
- `displayName` and `scriptVersion` identify a registration. Changing either registers a **new**
  script and leaves the previous one in place (Pulumi reports a replacement; nothing is deleted).
- `scriptVersion` is required. Register a new version whenever the hosted file changes so
  `SiteCustomCode` / `PageCustomCode` can pin the version they load.

## Examples Included

### 1. Analytics Script Registration

Register a CDN-hosted analytics tracking script with SRI hash validation.

```typescript
const analyticsScript = new webflow.RegisteredScript("analytics-script", {
  siteId: siteId,
  displayName: "AnalyticsTracker",
  hostedLocation: "https://cdn.example.com/analytics-tracker.js",
  integrityHash: "sha384-...",
  scriptVersion: "1.0.0",
  canCopy: true,
});
```

### 2. CMS Slider Script

Register a custom CMS slider for enhanced content presentation.

```typescript
const cmsSliderScript = new webflow.RegisteredScript("cms-slider", {
  siteId: siteId,
  displayName: "CmsSlider",
  hostedLocation: "https://cdn.example.com/cms-slider.min.js",
  integrityHash: "sha384-...",
  scriptVersion: "2.1.5",
  canCopy: false,
});
```

### 3. Multiple Script Versions

Register multiple versions of the same script for gradual rollouts or A/B testing.

```typescript
const marketingScriptV1 = new webflow.RegisteredScript("marketing-v1", {
  displayName: "MarketingPixel",
  scriptVersion: "1.0.0",
  // ... other properties
});

const marketingScriptV2 = new webflow.RegisteredScript("marketing-v2", {
  displayName: "MarketingPixel",
  scriptVersion: "2.0.0",
  // ... other properties
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
    deployedSiteId           : [secret]
    analyticsScriptId        : "abc123..."
    cmsSliderScriptId        : "def456..."
    customWidgetScriptId     : "ghi789..."
    marketingV1ScriptId      : "jkl012..."
    marketingV2ScriptId      : "mno345..."
    analyticsScriptCreatedOn : "2025-01-06T12:34:56Z"
```

## Generating SRI Hashes

Before registering a script, you must generate its Sub-Resource Integrity (SRI) hash:

### Online Tool
Visit [https://www.srihash.org/](https://www.srihash.org/) and enter your script URL.

### Command Line (OpenSSL)

```bash
# Download the script
curl https://cdn.example.com/your-script.js -o script.js

# Generate SHA-384 hash (recommended)
cat script.js | openssl dgst -sha384 -binary | openssl base64 -A
# Output: oqVuAfXRKap7fdgcCY5uykM6+R9GqQ8K/uxy9rx7HNQlGYl1kPzQho1wx4JwY8wC

# Format as: sha384-{hash}
# Result: sha384-oqVuAfXRKap7fdgcCY5uykM6+R9GqQ8K/uxy9rx7HNQlGYl1kPzQho1wx4JwY8wC
```

### Node.js Script

```javascript
const crypto = require('crypto');
const fs = require('fs');

const script = fs.readFileSync('script.js');
const hash = crypto.createHash('sha384').update(script).digest('base64');
console.log(`sha384-${hash}`);
```

## Script Requirements

### Display Name
- 1-50 alphanumeric characters only
- Examples: `AnalyticsTracker`, `CmsSlider`, `MyCustomScript123`
- Invalid: `my-script`, `script_v1`, `analytics tracker`

### Hosted Location
- Must be a valid HTTP or HTTPS URL
- Script should be publicly accessible
- Configure CORS if needed for cross-origin requests

### Integrity Hash
- Format: `sha256-{hash}`, `sha384-{hash}`, or `sha512-{hash}`
- SHA-384 is recommended for balance of security and performance
- Hash must match the hosted script exactly

### Version
- Required
- Must follow Semantic Versioning (SemVer): `major.minor.patch`
- Examples: `1.0.0`, `2.3.1`, `0.1.0`
- Invalid: `v1.0`, `1.0`, `1.0.0-beta`

## Using Registered Scripts

Once registered, scripts can be deployed using:

1. **SiteCustomCode** - Apply scripts to all pages of a site
2. **PageCustomCode** - Apply scripts to specific pages

See the `sitecustomcode` and `pagecustomcode` examples for usage.

## Cleanup

To stop managing the registered scripts:

```bash
pulumi destroy
pulumi stack rm dev
```

**Note:** `pulumi destroy` does not call the Webflow API - the registry has no unregister
endpoint, so the scripts remain registered (and still count toward the 800-per-site limit). Pages
and site-wide configurations that reference the script keep working; remove those references with
`SiteCustomCode` / `PageCustomCode` if the script should stop loading.

## Troubleshooting

### "Invalid displayName" Error

Display names must be 1-50 alphanumeric characters only:
- Valid: `AnalyticsTracker`, `MyScript123`
- Invalid: `my-script`, `Analytics Tracker`

### "Invalid integrityHash" Error

Ensure hash format is correct:
```
sha384-oqVuAfXRKap7fdgcCY5uykM6+R9GqQ8K/uxy9rx7HNQlGYl1kPzQho1wx4JwY8wC
sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=
sha512-...
```

### "Invalid scriptVersion" / "scriptVersion is required" Error

`scriptVersion` is required and must be valid SemVer:
- Valid: `1.0.0`, `2.3.1`
- Invalid: `v1.0.0`, `1.0`, `1.0.0-beta`

### 401 / 403 From the Registry Endpoints

The token is a site API token or lacks `custom_code:write`. Use a Data Client (OAuth app) token
with `custom_code:read` and `custom_code:write`.

### "Too many registered scripts"

The site has reached the 800-script limit. Because scripts cannot be unregistered through the
API, reuse existing registrations (same `displayName` + `scriptVersion`) instead of creating new
ones, or remove unused scripts in the Webflow dashboard.

### Script Not Loading on Page

1. Verify the script URL is publicly accessible
2. Check browser console for CORS errors
3. Ensure SRI hash matches the hosted file exactly
4. Verify the script is applied via SiteCustomCode or PageCustomCode

## Related Resources

- [SiteCustomCode Example](../sitecustomcode/)
- [PageCustomCode Example](../pagecustomcode/)
- [Main Examples Index](../README.md)
- [SRI Hash Generator](https://www.srihash.org/)
- [Semantic Versioning Guide](https://semver.org/)
