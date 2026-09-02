# Changelog

All notable changes to the Pulumi Webflow provider will be documented in this file.

## [Unreleased]

### Breaking Changes

- feat!: remove the `PageData` resource. It could never be created or imported. Use the new `getPage` / `getPages` functions to read pages and the new `PageMetadata` resource to manage page settings.
  - **Migration:** delete `PageData` resources from your program and run `pulumi state delete <URN>` for any that exist in state.
- feat!(asset): `fileSource` is now required and is uploaded. `Asset` previously only reserved an asset slot and never sent the file; it now reads a local path or URL, computes the MD5 `fileHash`, and completes the S3 upload. `uploadUrl` and `uploadDetails` are now secret outputs.
- feat!(site): the publish request body now matches the API: `publishToWebflowSubdomain`, `publishCustomDomains` (custom domain IDs) and `publishPageId` replace the undocumented `domains` field. `publish: true` with no target publishes to the webflow.io subdomain.
- feat!(collectionfield): `type: "Video"` is now `"VideoLink"` (the documented enum); `slug`, `type`, `validations` and `metadata` changes force replacement because the API cannot update them.
- fix!(config): explicit `webflow:apiToken` now takes precedence over `WEBFLOW_API_TOKEN`; the environment variable is only a fallback.
- The Go module now requires Go 1.26 (dependency upgrade to pulumi-go-provider v1.6 and pulumi/sdk v3.260).

### Features

- feat: `GoogleTag` resource for the Google Tag Manager API (list/upsert/delete tag IDs on a site).
- feat: `PageSchemaMarkup` resource and `getPageSchemaMarkup` function for JSON-LD schema markup (beta API).
- feat: `PageMetadata` resource (title, slug, SEO and Open Graph, per locale) and `getPage` / `getPages` functions with `translatable` and locale support.
- feat: Analyze API (beta) functions: `getAnalyticsTraffic`, `getAnalyticsTopPages`, `getAnalyticsTopDimensions`, `getAnalyticsTopEvents`, `getAnalyticsTimeOnPage`.
- feat(site): publish a single page with `publishPageId`; `publishScope` output.
- feat(asset): `folderId` output; list filtering by folder.
- feat(collectionitem): `live` input publishes the item after create/update; `cmsLocaleId` is sent on reads; `lastPublished` is populated.
- feat(collectionfield): `metadata` input for Option and Reference fields.
- feat(pagecontent): `localeId` input; empty text can clear a node.
- feat(webhook): all documented trigger types, including `comment_created`.
- feat(redirect): `destinationPath` and `statusCode` update in place.

### Bug Fixes

- fix(collectionitem): `fieldData` changes were never detected; omitted `isDraft`/`isArchived` no longer flip-flop after refresh.
- fix(collection): omitting `slug` and running `pulumi refresh` no longer replaces (deletes) the collection.
- fix(collectionfield): update uses PATCH with the documented fields; `isRequired: false` is sent.
- fix(robotstxt): no more permanent diff from content normalization; delete sends the documented body and accepts 200.
- fix(pagecontent): use POST with `localeId` and fail on node-level `errors`.
- fix(pagecustomcode): only 404 marks the resource deleted; scripts are read back so drift and import work; `Content-Type` is sent.
- fix(redirect, registeredscript, inlinescript): Read follows pagination instead of dropping resources from state.
- fix(inlinescript): user-omitted `integrityHash` no longer causes replace loops after refresh.
- fix(sitecustomcode, pagecustomcode): nested attribute values no longer panic in Diff; script order is ignored.
- fix(site): `parentFolderId` can be cleared.
- fix(auth): retries after 429 resend request bodies; transient 502/503/504 are retried; no whole-request timeout swallows long `Retry-After` waits; one shared connection pool honouring `HTTPS_PROXY`.
- fix: "not found" is detected from the HTTP status, never from error text; validation runs after the dry-run branch so previews with unknown inputs work; previews no longer fabricate IDs, timestamps or currencies.
- fix(release): publish the Node SDK from `sdk/nodejs/bin` (previous releases shipped no JavaScript); publish steps can fail the workflow; SLSA attestation covers the archives; actions pinned to SHAs.
- fix(build): correct the `-X` version symbol, so Makefile builds report their version; `make test_all` runs the integration harness.
- fix(docs): examples compile again in all five languages (removed `shortName` input, `scriptVersion`, correct Go/C#/Java package paths, real version pins); install commands include `--server`; NuGet package name corrected; license aligned to MIT everywhere.

## [v0.10.1] - 2026-03-19

### Bug Fixes

- fix: add automatic state migrations for `version`→`scriptVersion` rename (#90)
  - Upgrades from v0.9.x to v0.10.x no longer fail on `pulumi preview` for stacks with `InlineScript`, `RegisteredScript`, `SiteCustomCode`, or `PageCustomCode` resources
  - Old state containing the `version` property is automatically migrated to `scriptVersion` on first access

## [v0.10.0] - 2026-02-20

### Breaking Changes

- fix!: rename `version` to `scriptVersion` on RegisteredScript and InlineScript resources to avoid pulumi-go-provider framework collision (#88)
  - The `version` input/output has been renamed to `scriptVersion` on both `RegisteredScript` and `InlineScript` resources
  - **Migration:** Replace `version` with `scriptVersion` in your resource inputs (e.g., `version: "1.0.0"` → `scriptVersion: "1.0.0"`)

## [v0.9.4] - 2026-02-09

### Breaking Changes

- feat!(site): remove deprecated `shortName` input property (#84)
  - `shortName` is no longer accepted as an input to the Site resource
  - It remains available as a **read-only output** on the Site state (auto-generated by Webflow from `displayName`)
  - **Migration:** Remove any `shortName` from your Site resource inputs. To access the value, use the output property instead (e.g., `site.shortName` in TypeScript, `site.short_name` in Python)
- feat!: remove deprecated User resource stub (#80)
  - The `webflow:index:User` resource has been fully removed (Webflow API deprecated)
  - **Migration:** Remove any `User` resources from your program and run `pulumi state delete <URN>` to clean up existing state

## [v0.9.3] - 2026-02-06

### Bug Fixes

- fix(site): preserve TimeZone in Update preview to prevent forced replace
- fix: defer custom code validation to support unknown inputs during preview

## [v0.9.2] - 2026-02-04

### Features

- feat: add InlineScript resource for registering inline custom code scripts

### Bug Fixes

- fix(site): deprecate shortName input and fix PATCH field name
- fix: add parentFolderId support to PatchSite and lint fixes
- fix(site): make timezone a read-only output field

### Breaking Changes

- feat!: remove User Accounts resource (Webflow API deprecated)

## [v0.9.1] - 2026-01-14

### Bug Fixes

- fix(provider): add pluginDownloadURL for automatic provider installation
- fix(examples): correct package references for C#, Go, and Java

## [v0.9.0] - 2026-01-14

### Features

- feat: add rate limit handling, security policy, and performance docs
- feat(devcontainer): improve dev environment setup

### Bug Fixes

- fix(invoke): prevent crash in getTokenInfo/getAuthorizedUser functions
- fix(asset): parse variants as array instead of map
- fix(registeredscript): resolve version diff detection issue
- fix(registeredscript): all changes now trigger replacement instead of update
- fix: exclude unchanged slug from CollectionItem PATCH to prevent duplicate slug error
- fix: resolve drift detection issues and asset creation
- fix: add collectionId output and fix provider issues
- fix: release pipeline and npm publishing

[Unreleased]: https://github.com/JDetmar/pulumi-webflow/compare/v0.10.1...HEAD
[v0.10.1]: https://github.com/JDetmar/pulumi-webflow/compare/v0.10.0...v0.10.1
[v0.10.0]: https://github.com/JDetmar/pulumi-webflow/compare/v0.9.4...v0.10.0
[v0.9.4]: https://github.com/JDetmar/pulumi-webflow/compare/v0.9.3...v0.9.4
[v0.9.3]: https://github.com/JDetmar/pulumi-webflow/compare/v0.9.2...v0.9.3
[v0.9.2]: https://github.com/JDetmar/pulumi-webflow/compare/v0.9.1...v0.9.2
[v0.9.1]: https://github.com/JDetmar/pulumi-webflow/compare/v0.9.0...v0.9.1
[v0.9.0]: https://github.com/JDetmar/pulumi-webflow/releases/tag/v0.9.0
