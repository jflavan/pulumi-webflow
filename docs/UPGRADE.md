# Webflow Pulumi Provider - Upgrade Guide

This document provides guidance for upgrading the Webflow Pulumi Provider between versions.

## Overview

The Webflow Pulumi Provider follows [Semantic Versioning (semver)](https://semver.org/):

- **Major version** (X.y.z): Breaking changes that require code updates
- **Minor version** (x.Y.z): New features that are backward-compatible
- **Patch version** (x.y.Z): Bug fixes and maintenance updates

## Version Compatibility

- **Minimum Pulumi CLI version:** 3.x (the language SDKs pin their own floor, e.g. `@pulumi/pulumi ^3.142.0`, `pulumi>=3.165.0`, `Pulumi [3.76.1,4)`)
- **Minimum Go version:** 1.24.7 (required by the Go SDK and for provider development)
- **Language SDK versions** are bumped with provider releases

## Upgrading the Provider

### Using Pulumi CLI

The easiest way to upgrade is using the Pulumi CLI:

```bash
# Check current provider version
pulumi plugin ls

# Upgrade to latest version
pulumi plugin install resource webflow --server github://api.github.com/JDetmar/pulumi-webflow

# Upgrade to specific version
pulumi plugin install resource webflow v0.10.1 --server github://api.github.com/JDetmar/pulumi-webflow
```

### Manual Installation

If you prefer manual installation:

1. Download the binary for your platform from [GitHub Releases](https://github.com/JDetmar/pulumi-webflow/releases)
2. Extract the binary
3. Copy it to your Pulumi plugins directory: `~/.pulumi/plugins/`
4. Verify installation: `pulumi plugin ls`

## Upgrade Paths

### 0.9.x → 0.10.x (`version` renamed to `scriptVersion`)

The `version` property on `RegisteredScript` and `InlineScript` (and in the `scripts` entries
of `SiteCustomCode` / `PageCustomCode`) was renamed to `scriptVersion` in v0.10.0 to avoid a
collision with the provider framework.

**Action required:** replace `version` with `scriptVersion` in your resource inputs
(`version: "1.0.0"` → `scriptVersion: "1.0.0"`). Existing state is migrated automatically
on first access since v0.10.1.

### 0.9.3 → 0.9.4 (`shortName` is no longer a Site input)

`shortName` was removed as an input of the `Site` resource; Webflow generates it from
`displayName`. It remains available as a read-only output.

**Action required:** remove `shortName` from `Site` inputs and read `site.shortName`
(`site.short_name` in Python) where you need the value.

### 0.1.0 → 0.2.0

No breaking changes. This is a minor release with new features:

- ✅ New Redirect resource support
- ✅ Drift detection for managed resources
- ✅ State refresh capability

**Action required:** None. Code updates are optional.

```bash
# Your existing code continues to work
pulumi up
```

### Migration Guide Template (For Breaking Changes)

When major version updates introduce breaking changes, you'll see a migration guide like this:

```bash
# EXAMPLE: Upgrading from 1.0.0 to 2.0.0 (hypothetical)

# Before (version 1.0.0)
import pulumi_webflow as webflow
site = webflow.Site("my-site", ...)

# After (version 2.0.0)
import pulumi_webflow as webflow
site = webflow.Site("my-site", ...)  # API unchanged, config different
```

## Semantic Versioning Policy

### Backward Compatibility

**Guaranteed compatible (minor/patch versions):**
- New optional resource properties with defaults
- New resources
- New outputs on existing resources
- Bug fixes

**May break compatibility (major versions):**
- Removing resource properties
- Changing resource property types
- Renaming resources or properties
- Changing default values that affect infrastructure

### Deprecation Warnings

Before removing features, they are deprecated for at least one minor version:

```go
// Example deprecation message
ctx.Log.Warn("property 'legacy_option' is deprecated and will be removed in version 2.0.0. "+
             "Use 'new_option' instead.", nil)
```

## Breaking Change Process

When a breaking change is planned:

1. **Announced** in the GitHub repository and release notes
2. **Deprecated** in a minor version (e.g., 1.5.0) with warning messages
3. **Removed** in the next major version (e.g., 2.0.0)
4. **Documented** in the upgrade guide with migration examples

## Troubleshooting Upgrades

### Provider not found after upgrade

```bash
# Verify plugin installation
pulumi plugin ls | grep webflow

# Reinstall if missing
pulumi plugin install resource webflow <VERSION> --server github://api.github.com/JDetmar/pulumi-webflow
```

### State compatibility issues

The Pulumi state file format is managed by Pulumi itself. Provider upgrades don't require state changes.

If you encounter state issues:

1. Backup your state: `pulumi stack export > backup.json`
2. Try refreshing: `pulumi refresh`
3. If problems persist, check the GitHub issues

### API errors after upgrade

Check the release notes for breaking changes:

```bash
# Compare your code against the changelog
# https://github.com/JDetmar/pulumi-webflow/releases/tag/vX.Y.Z
```

## Version Pinning

To use a specific provider version in your project:

**Python:**
```bash
pip install pulumi-webflow==0.10.1
```

**TypeScript/Node.js:**
```bash
npm install @jdetmar/pulumi-webflow@0.10.1
```

**Go:**
```bash
go get github.com/JDetmar/pulumi-webflow/sdk/go/webflow@v0.10.1
```

**C#/.NET:**
```bash
dotnet add package Community.Pulumi.Webflow --version 0.10.1
```

**Java (Maven):**
```xml
<dependency>
  <groupId>io.github.jdetmar.pulumi</groupId>
  <artifactId>pulumi-webflow</artifactId>
  <version>0.10.1</version>
</dependency>
```

## Reporting Issues

If you encounter problems during an upgrade:

1. Check the [GitHub issues](https://github.com/JDetmar/pulumi-webflow/issues) for similar problems
2. Include your version information: `pulumi plugin ls`
3. Provide error messages and stack traces
4. Describe what worked before and what changed

## See Also

- [Release Notes](https://github.com/JDetmar/pulumi-webflow/releases)
- [Pulumi Migration Guide](https://www.pulumi.com/docs/guides/adopting-pulumi/migrating-to-pulumi/)
