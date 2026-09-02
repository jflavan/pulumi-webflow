# Redirect Resource Examples

This directory contains examples demonstrating how to create and manage URL redirects for Webflow sites using Pulumi in all supported languages.

## What You'll Learn

- Create permanent (301) redirects for SEO-friendly content moves
- Keep short campaign paths pointing at the current landing page
- Set up external domain redirects
- Implement bulk redirect patterns for URL migrations

> **Webflow redirects are always HTTP 301.** The API has no status-code setting, so the provider's
> `statusCode` input is deprecated and ignored; omit it. Temporary (302) redirects cannot be created
> through Webflow.

## Available Languages

| Language   | Directory    | Entry Point    | Dependencies        |
|------------|--------------|----------------|---------------------|
| TypeScript | `typescript/`| `index.ts`     | `package.json`      |
| Python     | `python/`    | `__main__.py`  | `requirements.txt`  |
| Go         | `go/`        | `main.go`      | `go.mod`            |
| C#         | `csharp/`    | `Program.cs`   | `.csproj`           |
| Java       | `java/`      | `App.java`     | `pom.xml`           |

## Quick Start

### TypeScript

```bash
cd typescript
npm install
pulumi stack init dev
pulumi config set siteId your-site-id --secret
pulumi up
```

### Python

```bash
cd python
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
pulumi stack init dev
pulumi config set siteId your-site-id --secret
pulumi up
```

### Go

```bash
cd go
go mod download
pulumi stack init dev
pulumi config set siteId your-site-id --secret
pulumi up
```

### C# (.NET)

```bash
cd csharp
dotnet restore
pulumi stack init dev
pulumi config set siteId your-site-id --secret
pulumi up
```

### Java

```bash
cd java
mvn install
pulumi stack init dev
pulumi config set siteId your-site-id --secret
pulumi up
```

## Examples Included

### 1. Content Move Redirect

Best for content that has permanently moved. Preserves SEO value by telling search engines the new location.

```
/blog/old-article → /blog/articles/updated-article
```

### 2. Campaign Redirect

A short, memorable path that always points at the current campaign page. Because
`destinationPath` updates in place, retargeting the path for the next campaign is a one-line change.

```
/old-campaign → /new-campaign-2025
```

### 3. External Redirect

Redirect to external domains for partner links or moved subdomains.

```
/partner → https://partner-site.com
```

### 4. Bulk Redirects

Efficient pattern for migrating multiple URLs at once during site restructuring.

```
/product-a → /products/product-a
/product-b → /products/product-b
/product-c → /products/product-c
```

## Configuration

Each example requires the following configuration:

| Config Key        | Required | Description                              |
|-------------------|----------|------------------------------------------|
| `siteId`  | Yes      | Your Webflow site ID (stored as secret)  |
| `environment`     | No       | Deployment environment (default: development) |

## Updating Redirects

Changing `destinationPath` updates the redirect in place (no replacement); changing `sourcePath` or
`siteId` replaces it. `statusCode` is deprecated and ignored: every Webflow redirect is a 301.

## Required Scopes

Webflow's endpoint reference lists `sites:read` / `sites:write` for the redirect endpoints while
the scopes page lists `site_config:read` / `site_config:write`; grant both pairs to be safe.

## Expected Output

After successful deployment, you'll see exports like:

```
Outputs:
    deployedSiteId        : [secret]
    permanentRedirectId   : "abc123..."
    campaignRedirectId    : "def456..."
    externalRedirectId    : "ghi789..."
    bulkRedirectIds       : ["jkl...", "mno...", "pqr..."]
```

Redirect IDs are assigned by Webflow on creation; `pulumi preview` shows them as `output<string>`.

## Cleanup

To remove all created redirects:

```bash
pulumi destroy
pulumi stack rm dev
```

## Troubleshooting

### "Site not found" Error

1. Verify your site ID in Webflow: Settings → General
2. Ensure correct format: `abc123def456`
3. Check API token has access to the site

### "Redirect already exists" Error

The source path already has a redirect. Either:
1. Import the existing redirect: `pulumi import webflow:index:Redirect name id`
2. Delete the existing redirect in Webflow first

## Related Resources

- [Main Examples Index](../README.md)
- [Webflow Redirects Documentation](https://university.webflow.com/lesson/301-redirects)
- [Webflow Redirects API](https://developers.webflow.com/reference/redirects)
