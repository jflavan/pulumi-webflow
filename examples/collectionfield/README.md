# CollectionField Resource Examples

This directory contains examples demonstrating how to create and manage fields for Webflow CMS collections using Pulumi.

## What You'll Learn

- Create various field types (PlainText, RichText, Number, DateTime, Switch, Email, Image, Phone, Color, Option, VideoLink)
- Configure Option fields with `metadata`
- Configure required vs. optional fields
- Add help text for content editors
- Read the slug Webflow generates from `displayName`

> **Deprecated inputs.** The Webflow API accepts neither `slug` nor `validations` when creating a
> field: the slug is always generated from `displayName`, and validation rules (min/max,
> maxLength, ...) can only be set in the Designer. Both inputs are deprecated and ignored by the
> provider; read the generated slug from the `slug` output.

## Available Languages

| Language   | Directory    | Entry Point    | Dependencies        |
|------------|--------------|----------------|---------------------|
| TypeScript | `typescript/`| `index.ts`     | `package.json`      |

## Quick Start

### TypeScript

```bash
cd typescript
npm install
pulumi stack init dev
pulumi config set webflow-collectionfield-example:collectionId your-collection-id --secret
pulumi up
```

## Prerequisites

Before running this example, you need:

1. **A Webflow Site**: Create a site in Webflow
2. **A Webflow Collection**: Create a collection in your site (Designer → CMS → Create Collection)
3. **Collection ID**: Get the collection ID from the Webflow API or dashboard (24-character hex string, e.g., `5f0c8c9e1c9d440000e8d8c3`)
4. **API Token**: Set `WEBFLOW_API_TOKEN` environment variable with your Webflow API token

### Finding Your Collection ID

Option 1 - Via Webflow API:
```bash
curl -H "Authorization: Bearer YOUR_API_TOKEN" \
     https://api.webflow.com/v2/sites/YOUR_SITE_ID/collections
```

Option 2 - From Collection URL in Webflow Designer:
The collection ID is in the URL when editing a collection: `https://webflow.com/design/your-site/cms/collections/COLLECTION_ID`

## Examples Included

### 1. Plain Text Field (Required)

A single-line text input. Best for titles, names, and short descriptions.

```typescript
type: "PlainText"
isRequired: true
```

### 2. Rich Text Field

Multi-line rich text editor for formatted content. Best for blog posts, articles, and long descriptions.

```typescript
type: "RichText"
isRequired: true
```

### 3. Number Field

Numeric input. Best for prices, quantities, ratings, or read times. Min/max and decimal-place
rules are configured in the Designer; the API does not accept them.

```typescript
type: "Number"
```

### 4. DateTime Field

Date and time picker. Best for publish dates, event dates, or deadlines.

```typescript
type: "DateTime"
isRequired: true
```

### 5. Switch Field (Boolean)

Toggle switch for true/false values. Best for feature flags or visibility controls.

```typescript
type: "Switch"
isRequired: false
```

### 6. Email Field

Email address input with built-in validation. Best for contact information.

```typescript
type: "Email"
```

### 7. Image Field

Single image reference. Best for cover images, thumbnails, or hero images.

```typescript
type: "Image"
```

### 8. Phone Field

Phone number input. Best for contact numbers.

```typescript
type: "Phone"
```

### 9. Color Field

Color picker. Best for theme colors or branding elements.

```typescript
type: "Color"
```

### 10. Generated Slug

Webflow always generates the field slug from `displayName`; read it from the `slug` output.

```typescript
displayName: "Short Description"
// field.slug resolves to "short-description"
```

### 11. Option Field with `metadata`

Dropdown fields declare their choices through the `metadata` input (required for `Option`,
`Reference` and `MultiReference` fields; create-only). The provider reads the options / referenced
collection back from the API, so `pulumi refresh` reflects changes made in the Designer.

```typescript
type: "Option"
metadata: {
  options: [{ name: "Draft" }, { name: "In Review" }, { name: "Published" }],
}
```

### 12. Video Link Field

Embeds a video from a URL. The type is `VideoLink` (the former `Video` name is no longer accepted).

```typescript
type: "VideoLink"
```

## Configuration

Each example requires the following configuration:

| Config Key        | Required | Description                              |
|-------------------|----------|------------------------------------------|
| `collectionId`    | Yes      | Your Webflow collection ID (24-char hex) |
| `environment`     | No       | Deployment environment (default: development) |

## Expected Output

After successful deployment, you'll see exports like:

```
Outputs:
    deployedCollectionId       : [secret]
    titleFieldId               : "abc123..."
    contentFieldId             : "def456..."
    readTimeFieldId            : "ghi789..."
    publishDateFieldId         : "jkl012..."
    featuredFieldId            : "mno345..."
    authorEmailFieldId         : "pqr678..."
    coverImageFieldId          : "stu901..."
    shortDescriptionFieldId    : "vwx234..."
    phoneFieldId               : "yza567..."
    accentColorFieldId         : "bcd890..."
    statusFieldId              : "efg123..."
    videoFieldId               : "hij456..."
    summary                    : "✅ Successfully created 12 collection fields:
                                   1. Article Title
                                   2. Article Content
                                   ..."
```

## Field Type Reference

The CollectionField resource supports the following field types:

| Type            | Description                                  | Type-specific configuration       |
|-----------------|----------------------------------------------|-----------------------------------|
| PlainText       | Single-line text input                       | -                                 |
| RichText        | Rich text editor with formatting             | -                                 |
| Number          | Numeric input                                | -                                 |
| DateTime        | Date and time picker                         | -                                 |
| Switch          | Boolean toggle (true/false)                  | -                                 |
| Email           | Email address with validation                | -                                 |
| Phone           | Phone number input                           | -                                 |
| Color           | Color picker                                 | -                                 |
| Image           | Single image reference                       | -                                 |
| MultiImage      | Multiple image references                    | -                                 |
| VideoLink       | Video embed link (YouTube, Vimeo, ...)        | -                                 |
| Link            | URL/link input                               | -                                 |
| File            | File upload                                  | -                                 |
| Option          | Dropdown/select field                        | `metadata: { options: [{ name }] }` (required) |
| Reference       | Reference to another collection item         | `metadata: { collectionId }` (required) |
| MultiReference  | Multiple references to collection items      | `metadata: { collectionId }` (required) |

`metadata` is the type-specific configuration for `Option`, `Reference` and `MultiReference` fields;
it is not accepted for other types and, like `type`, changing it replaces the field. The former
`Video` type name is now `VideoLink`; if you still have `Video` fields in state, run
`pulumi refresh` before `pulumi up` so the rename is not treated as a delete-before-replace.

## Important Notes

### Field Type Cannot Change

⚠️ **IMPORTANT**: The `type` field cannot be changed after creation. Changing it requires replacement (delete + recreate). This will result in data loss for existing collection items.

### Slug Generation

- Webflow generates the slug from `displayName`; the `slug` input is deprecated and ignored
- Read the generated value from the `slug` output; slugs are used in item `fieldData` keys
- Renaming a field (`displayName`) is an in-place update and does not change the slug

### Validations

- The API does not accept validation rules on create or update; the `validations` input is
  deprecated and ignored
- Configure min/max, character limits and similar rules in the Designer

### Field Editability

Some system fields may not be editable (`isEditable: false`). This is determined by Webflow and returned as a read-only output property.

## Cleanup

To remove all created fields:

```bash
pulumi destroy
pulumi stack rm dev
```

⚠️ **Warning**: Deleting collection fields will also delete all data in those fields for existing collection items. This action cannot be undone.

## Troubleshooting

### "Collection not found" Error

1. Verify your collection ID is correct (24-character hex string)
2. Ensure the collection exists in your Webflow site
3. Check API token has access to the site

### "Field already exists" Error

A field whose slug (generated from `displayName`) already exists. Either:
1. Import the existing field: `pulumi import webflow:index:CollectionField name collectionId/fieldId`
2. Use a different `displayName`
3. Delete the existing field in Webflow first

### "Invalid field type" Error

Ensure you're using one of the supported field types listed in the Field Type Reference above. Field types are case-sensitive.

### "Validation failed" Error

Check the type-specific configuration:
- Option fields: `metadata: { options: [...] }` is required
- Reference/MultiReference fields: `metadata: { collectionId: "..." }` is required
- Other types: `metadata` must be omitted
- `validations` and `slug` are ignored; remove them to silence the deprecation warning

## Related Resources

- [Collection Resource Examples](../collection/)
- [CollectionItem Resource Examples](../collectionitem/)
- [Main Examples Index](../README.md)
- [Webflow CMS Documentation](https://university.webflow.com/lesson/intro-to-the-cms)
- [Webflow Collection Fields API](https://developers.webflow.com/reference/collection-fields)

## Next Steps

After creating collection fields, you can:
1. Create collection items with the [CollectionItem resource](../collectionitem/)
2. Query and manage collections with the [Collection resource](../collection/)
3. Set up webhooks to track content changes with the [Webhook resource](../webhook/)
