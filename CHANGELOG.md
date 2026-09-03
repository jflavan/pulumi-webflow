# Changelog

All notable changes to the Pulumi Webflow provider will be documented in this file.

## [Unreleased]

### Breaking Changes

- feat!: remove the `PageData` resource. It could never be created or imported. Use the new `getPage` / `getPages` functions to read pages and the new `PageMetadata` resource to manage page settings.
  - **Migration:** delete `PageData` resources from your program and run `pulumi state delete <URN>` for any that exist in state.
- feat!(asset): `fileSource` is now required and is uploaded. `Asset` previously only reserved an asset slot and never sent the file; it now reads a local path or URL, computes the MD5 `fileHash`, and completes the S3 upload. `uploadUrl` and `uploadDetails` are now secret outputs.
- feat!(site): the publish request body now matches the API: `publishToWebflowSubdomain`, `publishCustomDomains` (custom domain IDs) and `publishPageId` replace the undocumented `domains` field. `publish: true` with no target publishes to the webflow.io subdomain.
- feat!(collectionfield): `type: "Video"` is now `"VideoLink"` (the documented enum); `type` and `metadata` changes force replacement because the API cannot update them.
  - **Migration:** run `pulumi refresh` before the first `pulumi up` so existing `Video` fields are read back as `VideoLink`; without the refresh the rename is planned as a delete-before-replace and the field's data is lost.
- feat!(collectionfield): the `validations` and `slug` inputs are deprecated and ignored. The API accepts neither validation rules nor a slug on create; the slug is generated from `displayName` (read it from the `slug` output) and validations are configured in the Designer.
- feat!(webhook): the `memberships_user_account_added`, `memberships_user_account_updated` and `memberships_user_account_deleted` trigger types were removed because Webflow removed the events. `filter` is only accepted for `form_submission` and has a single field, `name`.
  - **Migration:** delete webhooks that use a `memberships_*` trigger (`pulumi state delete <URN>` if Webflow already dropped them) and replace `filter: { collectionIds }` with routing on the payload's `collectionId`.
- feat!(pagecontent): `localeId` is now required and must be a secondary locale - the API cannot edit primary-locale content. The `lastUpdated` output was removed, an empty `text` no longer clears a node, and a resource may update at most 1000 nodes. New resources use the ID `{pageId}/content/{localeId}`; existing `{pageId}/content` IDs keep working, and `pulumi import` must use the new form.
- feat!(getPage): `translatable` is now a string holding a secondary locale ID (it was a boolean). Webflow answers `400` for the primary locale and `403` when translation exclusions are disabled.
- feat!(registeredscript): `scriptVersion` is now required.
- feat!(registeredscript, inlinescript): `pulumi destroy` no longer calls the API - Webflow has no unregister endpoint - so the registration remains (a site holds at most 800 scripts). Changing `displayName` or `scriptVersion` registers a new script and leaves the previous one in place.
- feat!(redirect): `statusCode` is deprecated and ignored; Webflow redirects are always `301`.
- fix!(ecommercesettings): the `defaultCurrency` output is now optional (sites without e-commerce report no currency).
- fix!(asset): `Asset` resources created by an earlier release without `fileSource` never uploaded a file; they are re-created on the first `pulumi up`. Run `pulumi import` for assets whose IDs must be kept.
- fix!(config): explicit `webflow:apiToken` now takes precedence over `WEBFLOW_API_TOKEN`; the environment variable is only a fallback.
- The Go module now requires Go 1.26 (dependency upgrade to pulumi-go-provider v1.6 and pulumi/sdk v3.260).
- feat!(dotnet): the `Community.Pulumi.Webflow` NuGet package is now compiled for `net8.0` and `net10.0` (previously `net6.0` only). Projects on .NET 6 or 7, which are out of support, must move to .NET 8 or later. Building the SDK requires the .NET 10 SDK.

### Features

- feat: `GoogleTag` resource for the Google Tag Manager API (list/upsert/delete tag IDs on a site).
- feat: `PageSchemaMarkup` resource and `getPageSchemaMarkup` function for JSON-LD schema markup (beta API).
- feat: `PageMetadata` resource (title, slug, SEO and Open Graph, per locale) and `getPage` / `getPages` functions with `translatable` and locale support.
- feat: Analyze API (beta) functions: `getAnalyticsTraffic`, `getAnalyticsTopPages`, `getAnalyticsTopDimensions`, `getAnalyticsTopEvents`, `getAnalyticsTimeOnPage`. Requests go to `/v2/sites/{id}/analyze/reports/...` and fall back to the `/beta` path while Webflow still serves it.
- feat(site): publish a single page with `publishPageId`; `publishScope` output (`site` or `page`).
- feat(asset): `folderId` output; list filtering by folder.
- feat(collection): `displayName`, `singularName` and `slug` update in place (`PATCH`); only `siteId` replaces the collection.
- feat(collectionitem): `live` input publishes the item after create/update; `cmsLocaleId` is sent on reads and honoured on delete and unpublish; `lastPublished` is populated.
- feat(collectionfield): `metadata` input for Option and Reference fields; Option and Reference fields are read back correctly (options / referenced collection), so refresh and import reflect the Designer.
- feat(pagecontent): `localeId` input (a secondary locale).
- feat(webhook): the 14 documented trigger types, including `collection_item_published` and `comment_created`; `filter: { name }` for `form_submission`.
- feat(redirect): `destinationPath` updates in place.
- feat: input validation runs at Check time, so `pulumi preview` reports invalid trigger types, locales, status codes, field types and similar mistakes before any API call.
- feat: the provider binary embeds the IANA time zone database (`time/tzdata`), so timestamps parse on hosts and containers without `tzdata`.

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
- fix(auth): retries after 429 resend request bodies; transient 502/503/504 are retried only for idempotent requests (never `POST`, so a create is never duplicated); no whole-request timeout swallows long `Retry-After` waits; one shared connection pool honouring `HTTPS_PROXY`.
- fix: "not found" is detected from the HTTP status, never from error text; validation runs after the dry-run branch so previews with unknown inputs work; `pulumi preview` no longer shows fabricated IDs, timestamps or currencies for any resource - values Webflow assigns on create are shown as unknown outputs.
- fix(release): publish the Node SDK from `sdk/nodejs/bin` (previous releases shipped no JavaScript); publish steps can fail the workflow; SLSA attestation covers the archives; actions pinned to SHAs.
- fix(build): correct the `-X` version symbol, so Makefile builds report their version; `make test_all` runs the integration harness.
- fix(docs): examples compile again in all five languages (removed `shortName` input, `scriptVersion`, correct Go/C#/Java package paths, real version pins); install commands include `--server`; NuGet package name corrected; license aligned to MIT everywhere (LICENSE file, package metadata for every SDK, the Python package now ships the LICENSE text, and every example, script and tool source file carries the MIT SPDX header).

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
