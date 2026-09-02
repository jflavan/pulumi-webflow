# RobotsTxt Resource Examples

This directory contains examples demonstrating how to manage the `robots.txt` file of a Webflow site using Pulumi in all supported languages.

## What You'll Learn

- Create and update a site's `robots.txt` from code
- Allow all crawlers (the default for public sites)
- Block specific directories or crawlers (useful for staging environments)
- Restrict sensitive directories such as `/api/` or `/internal/`

## Available Languages

| Language   | Directory    | Entry Point    | Dependencies        |
|------------|--------------|----------------|---------------------|
| TypeScript | `typescript/`| `index.ts`     | `package.json`      |
| Python     | `python/`    | `__main__.py`  | `requirements.txt`  |
| Go         | `go/`        | `main.go`      | `go.mod`            |
| C#         | `csharp/`    | `Program.cs`   | `.csproj`           |
| Java       | `java/`      | `App.java`     | `pom.xml`           |

## Quick Start

All examples read the site ID from the project config key `siteId` and expect the
Webflow API token in `webflow:apiToken` (or the `WEBFLOW_API_TOKEN` environment variable).

### TypeScript

```bash
cd typescript
npm install
pulumi stack init dev
pulumi config set webflow:apiToken --secret
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
pulumi config set webflow:apiToken --secret
pulumi config set siteId your-site-id --secret
pulumi up
```

### Go

```bash
cd go
go mod download
pulumi stack init dev
pulumi config set webflow:apiToken --secret
pulumi config set siteId your-site-id --secret
pulumi up
```

### C# (.NET)

```bash
cd csharp
dotnet restore
pulumi stack init dev
pulumi config set webflow:apiToken --secret
pulumi config set siteId your-site-id --secret
pulumi up
```

### Java

```bash
cd java
mvn install
pulumi stack init dev
pulumi config set webflow:apiToken --secret
pulumi config set siteId your-site-id --secret
pulumi up
```

## Example Contents

Each language implements the same three patterns:

1. **Allow All** - Standard configuration allowing every crawler
2. **Selective Blocking** - Blocks specific directories and crawlers
3. **Restrict Directories** - Protects directories such as `/api/` and `/internal/`

```typescript
const robotsTxt = new webflow.RobotsTxt("allow-all-robots", {
  siteId: siteId,
  content: `User-agent: *
Allow: /`,
});
```

## Configuration

| Config Key         | Required | Description                                   |
|--------------------|----------|-----------------------------------------------|
| `webflow:apiToken` | Yes      | Webflow API token (secret)                    |
| `siteId`           | Yes      | Webflow site ID, 24-character hex (secret)    |
| `environment`      | No       | Deployment environment label (default: development) |

## Notes

- A Webflow site has exactly one `robots.txt`. Creating a second `RobotsTxt` resource for the same site overwrites the first, so manage it with a single resource per site.
- The API token needs the `site_config:read` and `site_config:write` scopes.

## Cleanup

```bash
pulumi destroy
pulumi stack rm dev
```

## Related Resources

- [TypeScript walkthrough](typescript/README.md)
- [Redirect examples](../redirect/)
- [Main Examples Index](../README.md)
- [Webflow robots.txt API](https://developers.webflow.com/data/reference/enterprise/robots-txt/get)
