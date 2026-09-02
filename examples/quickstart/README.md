# Quickstart Examples

Deploy your first Webflow resource (a `robots.txt` file) with the Webflow Pulumi Provider. Each language directory is a complete, standalone Pulumi program with its own step-by-step README.

| Language   | Directory     | Guide                              |
|------------|---------------|------------------------------------|
| TypeScript | `typescript/` | [README](typescript/README.md)     |
| Python     | `python/`     | [README](python/README.md)         |
| Go         | `go/`         | [README](go/README.md)             |

## What the Quickstart Does

1. Reads the Webflow API token from `webflow:apiToken` (or `WEBFLOW_API_TOKEN`)
2. Reads the target site ID from the project config key `siteId`
3. Deploys a `RobotsTxt` resource that allows all crawlers
4. Exports the site ID and the resource ID as stack outputs

## Common Setup

```bash
# Choose a language
cd typescript   # or python, go

# Install dependencies
npm install                       # TypeScript
pip install -r requirements.txt   # Python
go mod download                   # Go

# Create a stack and configure it
pulumi stack init dev
pulumi config set webflow:apiToken --secret   # paste your token when prompted
pulumi config set siteId --secret             # paste your 24-character site ID

# Preview, deploy, clean up
pulumi preview
pulumi up
pulumi destroy
```

The provider plugin is installed automatically on the first `pulumi up`. To install it manually:

```bash
pulumi plugin install resource webflow --server github://api.github.com/JDetmar/pulumi-webflow
```

## Next Steps

- Per-resource examples: [../](../)
- Multi-environment stacks: [../stack-config/](../stack-config/)
- Managing many sites: [../multi-site/](../multi-site/)
- Main project README: [../../README.md](../../README.md)
