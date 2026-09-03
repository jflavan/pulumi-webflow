# Webflow Pulumi Provider

[![Build Status](https://img.shields.io/github/actions/workflow/status/JDetmar/pulumi-webflow/build.yml?branch=main)](https://github.com/JDetmar/pulumi-webflow/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![npm version](https://img.shields.io/npm/v/@jdetmar/pulumi-webflow)](https://www.npmjs.com/package/@jdetmar/pulumi-webflow)
[![PyPI version](https://img.shields.io/pypi/v/pulumi-webflow)](https://pypi.org/project/pulumi-webflow/)
[![NuGet version](https://img.shields.io/nuget/v/Community.Pulumi.Webflow)](https://www.nuget.org/packages/Community.Pulumi.Webflow)
[![Go Reference](https://pkg.go.dev/badge/github.com/JDetmar/pulumi-webflow/sdk/go/webflow.svg)](https://pkg.go.dev/github.com/JDetmar/pulumi-webflow/sdk/go/webflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/JDetmar/pulumi-webflow)](https://goreportcard.com/report/github.com/JDetmar/pulumi-webflow)

> **⚠️ Unofficial Community Provider**
>
> This is an **unofficial, community-maintained** Pulumi provider for Webflow. It is **not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.** This project is an independent effort to bring infrastructure-as-code capabilities to Webflow using Pulumi.
>
> - **Not an official product** - Created and maintained by the community
> - **No warranties** - Provided "as-is" under the MIT License
> - **Community support only** - Issues and questions via [GitHub](https://github.com/JDetmar/pulumi-webflow/issues)

**Manage your Webflow sites and resources as code using Pulumi**

The Webflow Pulumi Provider lets you programmatically manage Webflow resources using the same Pulumi infrastructure-as-code approach you use for cloud resources. Manage sites, pages, collections, redirects, webhooks, assets, and more. Deploy, preview, and destroy Webflow infrastructure alongside your other cloud deployments.

## What You Can Do

- **Deploy Webflow resources as code** - Define sites, pages, collections, redirects, webhooks, assets, and more in TypeScript, Python, Go, C#, or Java
- **Preview before deploying** - Use `pulumi preview` to see exactly what will change
- **Manage multiple environments** - Create separate stacks for dev, staging, and production
- **Version control your infrastructure** - Track all changes in Git
- **Integrate with CI/CD** - Automate deployments in your GitHub Actions, GitLab CI, or other pipelines

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Quick Start](#quick-start) - Start here
4. [Resources and Functions](#resources-and-functions)
5. [Authentication](#authentication)
6. [Verification](#verification)
7. [Get Help](#get-help)
8. [Version Control & Audit Trail](#version-control--audit-trail)
9. [Multi-Language Examples](#multi-language-examples)
10. [Next Steps](#next-steps)
11. [Contributing](#contributing)

---

## Prerequisites

Before you begin, make sure you have:

### 1. **Pulumi CLI**
- Download and install from [pulumi.com/docs/install](https://www.pulumi.com/docs/install/)
- Verify installation: `pulumi version` (requires v3.0 or later)

### 2. **Programming Language Runtime** (choose at least one)
- **TypeScript**: [Node.js](https://nodejs.org/) 18.x or later
- **Python**: [Python](https://www.python.org/downloads/) 3.9 or later
- **Go**: [Go](https://golang.org/dl/) 1.24.7 or later
- **C#**: [.NET](https://dotnet.microsoft.com/download) 8.0 or later (the package is compiled for .NET 8 and .NET 10)
- **Java**: [Java](https://adoptopenjdk.net/) 11 or later

### 3. **Webflow Account**
- A Webflow account with API access enabled
- Access to at least one Webflow site (where you'll deploy your first resource)

### 4. **Webflow API Token**
- Your Webflow API token (see [Authentication](#authentication) section below)

---

## Installation

The Webflow provider installs automatically when you first run `pulumi up`. For manual installation:

```bash
# Automatic installation (recommended - happens on first pulumi up/preview)
# Just run the Quick Start below, and the provider will install automatically

# OR manual installation if you prefer
pulumi plugin install resource webflow --server github://api.github.com/JDetmar/pulumi-webflow

# Verify installation
pulumi plugin ls | grep webflow
```

---

## Quick Start

### Deploy Your First Webflow Resource in Under 20 Minutes

This quick start walks you through deploying a robots.txt resource to your Webflow site using TypeScript. The entire process takes about 5 minutes once prerequisites are met.

### Step 1: Create a New Pulumi Project (2 minutes)

```bash
# Create a new directory for your Pulumi project
mkdir my-webflow-project
cd my-webflow-project

# Initialize a new Pulumi project
pulumi new --template typescript

# When prompted:
# - Enter a project name: my-webflow-project
# - Enter a stack name: dev
# - Enter a passphrase (or leave empty for no encryption): <press enter>
```

This creates:
- `Pulumi.yaml` - Project configuration
- `Pulumi.dev.yaml` - Stack-specific settings
- `index.ts` - Your infrastructure code
- `package.json` - Node.js dependencies

### Step 2: Configure Webflow Authentication (3 minutes)

```bash
# Get your Webflow API token (see Authentication section below if you don't have one)

# Set your token in Pulumi config (encrypted in Pulumi.dev.yaml)
pulumi config set webflow:apiToken --secret

# When prompted, paste your Webflow API token and press Enter
```

**What's happening:** Your token is encrypted and stored locally in `Pulumi.dev.yaml` (which is in .gitignore). It's never stored in plain text.

### Step 3: Write Your First Resource (5 minutes)

Replace the contents of `index.ts` with:

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as webflow from "@jdetmar/pulumi-webflow";

// Get config values
const config = new pulumi.Config();
const siteId = config.requireSecret("siteId"); // We'll set this next

// Deploy a robots.txt resource
const robotsTxt = new webflow.RobotsTxt("my-robots", {
  siteId: siteId,
  content: `User-agent: *
Allow: /

User-agent: Googlebot
Allow: /
`, // Standard robots.txt allowing all crawlers
});

// Export the site ID for reference
export const deployedSiteId = siteId;
```

### Step 4: Configure Your Site ID (2 minutes)

```bash
# Find your Webflow site ID (24-character hex string) from Webflow Designer
# You can find it in: Project Settings > API & Webhooks > Site ID

# Set it in your Pulumi config
pulumi config set siteId --secret

# When prompted, paste your 24-character site ID and press Enter
```

**Need help finding your site ID?**
- In Webflow Designer, go to **Project Settings** (bottom of sidebar)
- Click **API & Webhooks**
- Your **Site ID** is displayed as a 24-character hex string (e.g., `5f0c8c9e1c9d440000e8d8c3`)

### Step 5: Preview Your Deployment (2 minutes)

```bash
# Install dependencies
npm install

# Preview the changes Pulumi will make
pulumi preview
```

Expected output:
```
Previewing update (dev):

     Type                           Name         Plan       Info
 +   webflow:RobotsTxt             my-robots    create

Resources:
    + 1 to create

Do you want to perform this update?
  > yes
    no
    details
```

### Step 6: Deploy! (2 minutes)

```bash
# Deploy to your Webflow site
pulumi up
```

When prompted, select **yes** to confirm the deployment.

Expected output:
```
     Type                           Name         Plan       Status
 +   webflow:RobotsTxt             my-robots    create     created

Outputs:
    deployedSiteId: "5f0c8c9e1c9d440000e8d8c3"

Resources:
    + 1 created

Duration: 3s
```

### Step 7: Verify in Webflow (2 minutes)

1. Open Webflow Designer
2. Go to **Project Settings** → **SEO** → **robots.txt**
3. You should see the robots.txt content you deployed!

### Step 8: Clean Up (Optional)

```bash
# Remove the resource from Webflow
pulumi destroy

# When prompted, select 'yes' to confirm
```

**Congratulations!** You've successfully deployed your first Webflow resource using Pulumi! 🎉

---

## Resources and Functions

Everything below is registered in [`provider/provider.go`](./provider/provider.go) and available in all five SDKs
(`@jdetmar/pulumi-webflow`, `pulumi_webflow`, `github.com/JDetmar/pulumi-webflow/sdk/go/webflow`,
`Community.Pulumi.Webflow`, `io.github.jdetmar.pulumi.webflow`). Every resource except `Site` is scoped to an
existing site, page or collection ID.

| Resource | What it manages | Example |
|----------|-----------------|---------|
| `Site` | Webflow sites (create/update/delete). Inputs: `workspaceId`, `displayName`, optional `parentFolderId`, `templateName`, and `publish` with `publishToWebflowSubdomain`, `publishCustomDomains` or `publishPageId` (single page). `shortName`, `timeZone` and `publishScope` are read-only outputs. | [examples/site](./examples/site/) |
| `Redirect` | Permanent (301) URL redirects for a site; Webflow has no other status code, so `statusCode` is deprecated and ignored | [examples/redirect](./examples/redirect/) |
| `RobotsTxt` | The site's `robots.txt` | [examples/robotstxt](./examples/robotstxt/) |
| `Collection` | CMS collections (`displayName`, `singularName` and `slug` update in place; `siteId` replaces) | [examples/collection](./examples/collection/) |
| `CollectionField` | Fields of a CMS collection (`validations` and `slug` are deprecated and ignored; the slug is derived from `displayName`) | [examples/collectionfield](./examples/collectionfield/) |
| `CollectionItem` | CMS items with dynamic field data | [examples/collectionitem](./examples/collectionitem/) |
| `PageMetadata` | Title, slug, SEO and Open Graph settings of a page created in the Designer (pages cannot be created via the API), per locale | [examples/pagemetadata](./examples/pagemetadata/) |
| `PageContent` | Text content inside existing DOM nodes of a page for a secondary locale (`localeId` is required; the API cannot edit primary-locale content) | [examples/pagecontent](./examples/pagecontent/) |
| `PageSchemaMarkup` | JSON-LD schema markup of a page (beta API) | [examples/pageschemamarkup](./examples/pageschemamarkup/) |
| `Webhook` | Event webhooks for the 14 Webflow trigger types (form submissions, publishes, page and CMS events, e-commerce events, comments); needs a Data Client token | [examples/webhook](./examples/webhook/) |
| `Asset` | Files and images uploaded from a local path or URL (`fileSource`); content changes replace the asset | [examples/asset](./examples/asset/) |
| `AssetFolder` | Asset folders (the API cannot delete them) | [examples/assetfolder](./examples/assetfolder/) |
| `RegisteredScript` | Externally hosted scripts in the script registry, with required `scriptVersion` and integrity hash. Webflow has no unregister endpoint: `pulumi destroy` only drops the resource from state (max 800 scripts per site) | [examples/registeredscript](./examples/registeredscript/) |
| `InlineScript` | Inline JavaScript (up to 2000 characters) in the script registry, with `scriptVersion`; same no-unregister behaviour as `RegisteredScript` | [examples/inlinescript](./examples/inlinescript/) |
| `SiteCustomCode` | Applies registered scripts site-wide (header/footer) | [examples/sitecustomcode](./examples/sitecustomcode/) |
| `PageCustomCode` | Applies registered scripts to a single page | [examples/pagecustomcode](./examples/pagecustomcode/) |
| `GoogleTag` | A Google Tag ID (GA4, Google Tag, Google Ads, Campaign Manager) on a site | [examples/googletag](./examples/googletag/) |
| `EcommerceSettings` | Read-only import of a site's e-commerce settings | [examples/ecommerce-settings](./examples/ecommerce-settings/) |

| Function | What it returns | Example |
|----------|-----------------|---------|
| `getTokenInfo` | Scopes, rate limits and authorized resources of the configured token (Data Client tokens only) | [examples/token](./examples/token/) |
| `getAuthorizedUser` | The user who authorized the API token | [examples/token](./examples/token/) |
| `getPages` | All pages of a site with their metadata (the way to find page IDs) | [examples/page](./examples/page/) |
| `getPage` | The metadata of a single page, optionally for a locale | [examples/page](./examples/page/) |
| `getPageSchemaMarkup` | The JSON-LD schema markup of a page (beta API) | [examples/pageschemamarkup](./examples/pageschemamarkup/) |
| `getAnalyticsTraffic`, `getAnalyticsTopPages`, `getAnalyticsTopDimensions`, `getAnalyticsTopEvents`, `getAnalyticsTimeOnPage` | Site analytics reports from the Analyze API (beta; requires the Analyze add-on) | [examples/analytics](./examples/analytics/) |

**Available from the next release:** the `GoogleTag`, `PageSchemaMarkup` and `PageMetadata` resources and the
`getPage`, `getPages`, `getPageSchemaMarkup` and `getAnalytics*` functions ship with the next release line, which
also removes the `PageData` resource (use `getPages`/`getPage` instead); see the [CHANGELOG](./CHANGELOG.md).

---

## Authentication

### Getting Your Webflow API Token

1. Log in to [Webflow](https://webflow.com)
2. Go to **Account Settings** (bottom left of screen)
3. Click **API Tokens** in the left sidebar
4. Click **Create New Token**
5. Name it something descriptive (e.g., "Pulumi Provider")
6. Grant the scopes for the resource families you manage. The provider only calls the endpoints
   for resources in your program, so a token needs only the rows that apply:

   | Resource family | Resources / functions | Scopes |
   |-----------------|-----------------------|--------|
   | Sites | `Site` | `sites:read`, `sites:write`; **creating** a site additionally needs `workspace:write` (creation is a workspace endpoint and requires an Enterprise workspace) |
   | Redirects | `Redirect` | `sites:read`, `sites:write` **and** `site_config:read`, `site_config:write` - Webflow's endpoint reference lists the `sites` pair, its scopes page the `site_config` pair; grant both |
   | Site configuration | `RobotsTxt` | `site_config:read`, `site_config:write` |
   | Webhooks | `Webhook` | **Data Client token only.** `sites:write` plus the read scope of the event family: `forms:read` (`form_submission`), `sites:read` (`site_publish`), `pages:read` (`page_*`), `cms:read` (`collection_item_*`), `ecommerce:read` (`ecomm_*`), `comments:read` (`comment_created`) |
   | Pages | `PageMetadata`, `PageContent`, `PageSchemaMarkup`, `getPages`, `getPage`, `getPageSchemaMarkup` | `pages:read`, `pages:write` |
   | CMS | `Collection`, `CollectionField`, `CollectionItem` | `cms:read`, `cms:write` |
   | Assets | `Asset`, `AssetFolder` | `assets:read`, `assets:write` |
   | Custom code | `RegisteredScript`, `InlineScript`, `SiteCustomCode`, `PageCustomCode` | **Data Client token only** (site tokens cannot call the custom code endpoints). `custom_code:read`, `custom_code:write`; deleting `SiteCustomCode` / `PageCustomCode` (removing applied code) also needs `sites:write` / `pages:write` |
   | Google tags | `GoogleTag` | `sites:read`, `sites:write` |
   | E-commerce | `EcommerceSettings` | `ecommerce:read` |
   | Token and user info | `getTokenInfo` (**Data Client token only**), `getAuthorizedUser` | `authorized_user:read` |
   | Analytics (beta API) | `getAnalytics*` | `sites:read`, plus a workspace with the Analyze add-on |

   Read-only scopes are enough for `pulumi preview`, `pulumi refresh` and the functions; `pulumi up` needs the write scopes.

   **Token types.** A *site API token* (Site settings → Apps & integrations → API access) is
   enough for most resources. Webflow exposes the webhook and custom-code endpoints and the
   token-introspection endpoint behind `getTokenInfo` only to [Data Client apps](https://developers.webflow.com/data/docs/getting-started-data-clients),
   so `Webhook`, `RegisteredScript`, `InlineScript`, `SiteCustomCode`, `PageCustomCode` and
   `getTokenInfo` need an OAuth access token issued to such an app (with the scopes above) passed
   as `webflow:apiToken`. A site token returns `401`/`403` for those calls.
7. Click **Create Token**
8. **Copy the token immediately** - Webflow won't show it again

### Setting Up Your Token in Pulumi

```bash
# Option 1: Pulumi config (recommended - encrypted in Pulumi.dev.yaml)
pulumi config set webflow:apiToken --secret

# Option 2: Environment variable
export WEBFLOW_API_TOKEN="your-token-here"

# Option 3: Code (NOT RECOMMENDED for production - security risk)
# Don't do this in production code!
```

**Precedence:** explicit stack configuration wins. If `webflow:apiToken` is set, the provider uses
it and ignores `WEBFLOW_API_TOKEN`; the environment variable is only a fallback for stacks that do
not configure a token. This keeps a token exported in your shell from silently reaching a stack that
carries its own encrypted token.

### Security Best Practices

- ✅ **DO** use Pulumi config with `--secret` flag (encrypts locally)
- ✅ **DO** use environment variables in CI/CD pipelines
- ✅ **DO** keep tokens in `.env` files (never commit to Git)
- ❌ **DON'T** commit tokens to Git
- ❌ **DON'T** hardcode tokens in your Pulumi program
- ❌ **DON'T** share tokens via email or chat
- 🔐 **Rotate tokens regularly** - Create new tokens and retire old ones monthly

### CI/CD Configuration

For GitHub Actions or other CI/CD:

```yaml
# .github/workflows/deploy.yml
env:
  WEBFLOW_API_TOKEN: ${{ secrets.WEBFLOW_API_TOKEN }}
  PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: pulumi/actions@v6
        with:
          command: up
```

---

## Verification

### Confirm Your Installation

```bash
# Check Pulumi is installed
pulumi version

# Check the Webflow provider is available
pulumi plugin ls | grep webflow

# Should output something like (the version matches the SDK version in your project):
# resource  webflow  0.10.1
```

### Verify Authentication Works

```bash
# Inside a Pulumi project directory:
pulumi preview

# If authentication fails, you'll see an error like:
# Error: Unauthorized - Check your Webflow API token
```

### After Deployment

1. **In Webflow Designer:**
   - Check that your resource appears in the appropriate settings (robots.txt, redirects, etc.)
   - Verify the configuration matches what you deployed

2. **Via Pulumi:**
   ```bash
   pulumi stack output deployedSiteId
   # Should output your 24-character site ID
   ```

3. **Via Command Line:**
   ```bash
   # View your stack's resources
   pulumi stack select dev
   pulumi stack

   # View detailed resource information
   pulumi stack export | jq .
   ```

---

## Get Help

- [Troubleshooting Guide](./docs/troubleshooting.md) - Comprehensive error reference and solutions
- [FAQ](./docs/faq.md) - Answers to common questions
- [Examples](./examples/) - Working code for all resources
- [GitHub Issues](https://github.com/JDetmar/pulumi-webflow/issues) - Report bugs
- [GitHub Discussions](https://github.com/JDetmar/pulumi-webflow/discussions) - Ask questions

---

## Version Control & Audit Trail

Track all infrastructure changes in Git for compliance and auditability. Features include automatic audit trails, code review via pull requests, and SOC 2/HIPAA/GDPR-ready reporting.

See the [Version Control Guide](./docs/version-control.md) for Git workflows, commit conventions, and audit report generation.

---

## Multi-Language Examples

The Quick Start uses TypeScript. Complete examples for all languages are in [examples/quickstart/](./examples/quickstart/):

- [TypeScript](./examples/quickstart/typescript/) | [Python](./examples/quickstart/python/) | [Go](./examples/quickstart/go/)

Each includes setup instructions, complete code, and a README.

---

## Next Steps

Once you've completed the Quick Start:

### 1. **Explore More Resources**
- Deploy the other resource types from the [catalog above](#resources-and-functions) (Sites, CMS collections, webhooks, custom code, ...)
- Use the [examples/](./examples/) directory for real-world patterns
- Check [docs/](./docs/) for comprehensive reference documentation

### 2. **Multi-Environment Setup**
- Create separate stacks for dev, staging, and production
- Use different site IDs for each environment
- See: [examples/stack-config/](./examples/stack-config/)

### 3. **Advanced Patterns**
- Multi-site management: [examples/multi-site/](./examples/multi-site/)
- CI/CD integration: [examples/ci-cd/](./examples/ci-cd/)
- Logging and troubleshooting: [examples/troubleshooting-logs/](./examples/troubleshooting-logs/)

### 4. **Learn Pulumi Concepts**
- [Pulumi Documentation](https://www.pulumi.com/docs/)
- [Getting Started with Pulumi](https://www.pulumi.com/docs/iac/getting-started/)
- [Pulumi Best Practices](https://www.pulumi.com/docs/using-pulumi/best-practices/)

### 5. **Connect with the Community**
- [Pulumi Community Slack](https://pulumi-community.slack.com/)
- [Pulumi GitHub Discussions](https://github.com/pulumi/pulumi/discussions)

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines.

### Ways to Contribute

- **Report bugs** - Found an issue? [Create a GitHub issue](https://github.com/JDetmar/pulumi-webflow/issues)
- **Submit improvements** - Have an idea? [Create a discussion](https://github.com/JDetmar/pulumi-webflow/discussions)
- **Contribute code** - Fork the repo, make changes, and submit a pull request
- **Improve documentation** - Help us document features and patterns

---

## License

This project is licensed under the MIT License - see [LICENSE](./LICENSE) file for details.

---

## Troubleshooting Quick Reference

| Problem | Solution |
|---------|----------|
| Plugin not found | `pulumi plugin install resource webflow --server github://api.github.com/JDetmar/pulumi-webflow` |
| Invalid token | Check Webflow Account Settings → API Tokens |
| Invalid site ID | Verify in Webflow Designer → Project Settings → API & Webhooks |
| Deployment times out | Check internet connection, try again |
| Token format error | Ensure you're using the full API token (not just a prefix) |
| Site not found | Verify site ID matches the site where you want to deploy |
| Need detailed logs | Enable debug logging: `pulumi up -v=9 --logtostderr` (the provider logs at debug level; there is no `PULUMI_LOG_LEVEL` variable) - See [Logging Guide](./docs/logging.md) |

For more troubleshooting help and detailed logging documentation:
- **[Logging and Debugging Guide](./docs/logging.md)** - Comprehensive guide to structured logging features
- **[Troubleshooting Guide](./docs/troubleshooting.md)** - Common issues and detailed solutions
- **[Changelog](./CHANGELOG.md)** - Release history and notable changes

---

**Ready to get started?** Jump to [Quick Start](#quick-start) above! 🚀
