# Asset Resource Examples

This directory contains examples demonstrating how to upload assets (images, files, documents) to
Webflow sites using Pulumi in all supported languages.

## What You'll Learn

- Upload a local file shipped with the example (`assets/logo.svg`)
- Upload a file from an http(s) URL
- Organize assets into folders with `parentFolder`
- Read the computed `fileHash`, the public `hostedUrl` and the `folderId` as outputs

## How Uploads Work

`fileSource` is required and tells the provider where the file bytes come from:

- a **local path**, resolved relative to the Pulumi program's working directory
  (e.g. `./assets/logo.svg`), or
- an **http(s) URL** (e.g. `https://example.com/hero.jpg`).

At apply time the provider reads the content, computes its MD5 `fileHash`, registers the asset
with Webflow and completes the S3 upload. You no longer have to upload anything yourself:
`hostedUrl` is usable as soon as `pulumi up` finishes.

- `fileHash` is optional and computed for you; if you set it explicitly it must match the content.
- For local files, a content change (different hash) replaces the asset.
- `uploadUrl` and `uploadDetails` (the presigned S3 form Webflow returns) are still exposed, as
  **secret** outputs, but you do not need them.
- `folderId` reports the folder Webflow placed the asset in.

## Available Languages

| Language   | Directory    | Entry Point    | Dependencies        |
|------------|--------------|----------------|---------------------|
| TypeScript | `typescript/`| `index.ts`     | `package.json`      |
| Python     | `python/`    | `__main__.py`  | `requirements.txt`  |
| Go         | `go/`        | `main.go`      | `go.mod`            |
| C#         | `csharp/`    | `Program.cs`   | `.csproj`           |
| Java       | `java/`      | `App.java`     | `pom.xml`           |

Every language directory ships the same sample files under `assets/`:

```
assets/
├── logo.svg
└── icons/
    ├── home.svg
    └── user.svg
```

## Prerequisites

- Pulumi CLI installed
- A Webflow API token with the **`assets:read`** and **`assets:write`** scopes, set as
  `WEBFLOW_API_TOKEN` or via `pulumi config set webflow:apiToken --secret`
- Your Webflow site ID

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

> The Go example's `go.mod` contains a `replace` directive pointing at the SDK in this repository
> (`../../../sdk/go/webflow`) because `fileSource` is newer than the published `v0.10.1` Go
> module. Once the next release is published you can drop the directive and depend on that
> version instead.

### C#

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
mvn clean install
pulumi stack init dev
pulumi config set siteId your-site-id --secret
pulumi up
```

## Examples Included

### 1. Upload a Local File

```typescript
const logoAsset = new webflow.Asset("company-logo", {
  siteId: siteId,
  fileName: "logo.svg",
  fileSource: "./assets/logo.svg", // relative to the program directory
});

export const logoHostedUrl = logoAsset.hostedUrl;
export const logoFileHash = logoAsset.fileHash; // computed by the provider
```

### 2. Upload from a URL (optional)

Set `pulumi config set heroImageUrl https://example.com/hero.jpg` to enable this part of the
example. `parentFolder` places the asset in an existing asset folder (see the
[AssetFolder example](../assetfolder/)).

```typescript
const heroImage = new webflow.Asset("hero-image", {
  siteId: siteId,
  fileName: "hero-banner.jpg",
  fileSource: heroImageUrl,
  // parentFolder: imagesFolder.folderId,
});
```

### 3. Bulk Upload

```typescript
const icons = [
  { name: "icon-home", fileName: "icon-home.svg", fileSource: "./assets/icons/home.svg" },
  { name: "icon-user", fileName: "icon-user.svg", fileSource: "./assets/icons/user.svg" },
];

icons.forEach((icon) => {
  new webflow.Asset(icon.name, {
    siteId: siteId,
    fileName: icon.fileName,
    fileSource: icon.fileSource,
  });
});
```

## Configuration

| Config Key      | Required | Description                                                |
|-----------------|----------|------------------------------------------------------------|
| `siteId`        | Yes      | Your Webflow site ID (stored as secret)                    |
| `heroImageUrl`  | No       | An http(s) URL to upload as the `hero-image` asset         |

## Expected Output

After a successful deployment you'll see exports like:

```
Outputs:
    iconAssetIds    : ["5f0c8c9e1c9d440000e8d8c5", "5f0c8c9e1c9d440000e8d8c6"]
    logoAssetId     : "5f0c8c9e1c9d440000e8d8c4"
    logoAssetUrl    : "https://s3.amazonaws.com/webflow-prod-assets/..."
    logoContentType : "image/svg+xml"
    logoFileHash    : "3c1f0b9d4a2e6f8b7c5d1e9a0b2c4d6e"
    logoFolderId    : ""
    logoHostedUrl   : "https://cdn.prod.website-files.com/.../logo.svg"
    logoSize        : 412
```

## Updating an Asset

Edit `assets/logo.svg` and run `pulumi up` again: the provider notices the new MD5 hash and
replaces the asset (Webflow assets are immutable, so a new asset ID is created and the old one
deleted). Changing `fileName`, `siteId` or `parentFolder` replaces the asset as well.

## Cleanup

```bash
pulumi destroy
pulumi stack rm dev
```

## Troubleshooting

### "fileSource is required" Error

Every asset needs a `fileSource` (local path or URL). The former "register only, upload later"
flow with a hand-computed `fileHash` is no longer supported.

### "no such file or directory"

Local paths are resolved relative to the Pulumi program's working directory (the directory
containing `Pulumi.yaml`). Use `./assets/logo.svg`, not a path relative to your shell.

### "fileHash does not match"

If you set `fileHash` explicitly it must be the MD5 of the actual content. Simply omit it and let
the provider compute it.

## Related Resources

- [AssetFolder Resource](../assetfolder/)
- [Main Examples Index](../README.md)
- [Webflow Asset API](https://developers.webflow.com/data/reference/assets)
