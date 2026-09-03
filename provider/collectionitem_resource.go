// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// CollectionItemResource is the resource controller for managing Webflow CMS collection items.
// It implements the infer.CustomResource interface for full CRUD operations.
type CollectionItemResource struct{}

// CollectionItemArgs defines the input properties for the CollectionItem resource.
type CollectionItemArgs struct {
	// CollectionID is the Webflow collection ID (24-character lowercase hexadecimal string).
	// Example: "5f0c8c9e1c9d440000e8d8c3"
	CollectionID string `pulumi:"collectionId"`
	// FieldData is a map of field slugs to values for the collection item.
	// The field slugs must match the fields defined in the collection schema.
	// Example: {"name": "My Blog Post", "slug": "my-blog-post", "content": "Post content..."}
	FieldData map[string]interface{} `pulumi:"fieldData"`
	// IsArchived indicates whether the item is archived (optional; nil means "don't manage").
	IsArchived *bool `pulumi:"isArchived,optional"`
	// IsDraft indicates whether the item is a draft (optional; nil means "don't manage").
	IsDraft *bool `pulumi:"isDraft,optional"`
	// CmsLocaleID is the locale ID for localized sites (optional).
	CmsLocaleID string `pulumi:"cmsLocaleId,optional"`
	// Live, when true, publishes the item to the live site after every create and update.
	Live bool `pulumi:"live,optional"`
}

// CollectionItemState defines the output properties for the CollectionItem resource.
// It embeds CollectionItemArgs to include input properties in the output.
type CollectionItemState struct {
	CollectionItemArgs
	// ItemID is the Webflow-assigned item ID (read-only).
	ItemID string `pulumi:"itemId,optional"`
	// LastPublished is the timestamp when the item was last published (read-only).
	LastPublished string `pulumi:"lastPublished,optional"`
	// LastUpdated is the timestamp when the item was last updated (read-only).
	LastUpdated string `pulumi:"lastUpdated,optional"`
	// CreatedOn is the timestamp when the item was created (read-only).
	CreatedOn string `pulumi:"createdOn,optional"`
}

// Annotate adds descriptions and constraints to the CollectionItem resource.
func (c *CollectionItemResource) Annotate(a infer.Annotator) {
	a.SetToken("index", "CollectionItem")
	a.Describe(c, "Manages CMS collection items for a Webflow collection. "+
		"Collection items represent individual content entries (blog posts, products, etc.) "+
		"within a CMS collection. Each item has dynamic field data based on the collection schema. "+
		"Items are created and updated in staging; set live=true to publish them to the live site "+
		"as part of every create and update.")
}

// Annotate adds descriptions to the CollectionItemArgs fields.
func (args *CollectionItemArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.CollectionID,
		"The Webflow collection ID (24-character lowercase hexadecimal string, "+
			"e.g., '5f0c8c9e1c9d440000e8d8c3'). "+
			"You can find collection IDs via the Webflow API or dashboard. "+
			"This field will be validated before making any API calls.")

	a.Describe(&args.FieldData,
		"A map of field slugs to values for the collection item. "+
			"The field slugs must match the fields defined in the collection schema. "+
			"Common fields include 'name' (required), 'slug' (required, URL-friendly), "+
			"and any custom fields you've added to the collection. "+
			"Only the fields listed here are managed; other fields of the item are left untouched. "+
			"Example: {\"name\": \"My Blog Post\", \"slug\": \"my-blog-post\", \"content\": \"Post content...\"}")

	a.Describe(&args.IsArchived,
		"Whether the item is archived (optional). "+
			"Archived items are not visible on the published site but remain in the CMS. "+
			"When omitted, the archived state is not managed and never causes a diff.")

	a.Describe(&args.IsDraft,
		"Whether the item is a draft (optional; Webflow defaults new items to true). "+
			"Setting isDraft to false stages the item to go out with the next site publish - "+
			"it does not publish the item by itself. Use live=true to publish the item immediately. "+
			"When omitted, the draft state is not managed and never causes a diff.")

	a.Describe(&args.CmsLocaleID,
		"The CMS locale ID for localized sites (optional). "+
			"Only required if your site uses Webflow's localization features; "+
			"it is sent with every request for this item, including reads. "+
			"Leave empty for non-localized sites.")

	a.Describe(&args.Live,
		"Publish the item to the live site after every create and update (optional, defaults to false). "+
			"When true the provider calls the Webflow publish-items endpoint after writing the item and "+
			"reads the item back from the live endpoint, so lastPublished reflects the live copy. "+
			"Setting this back to false stops publishing future changes but does not unpublish the item.")
}

// Annotate adds descriptions to the CollectionItemState fields.
func (state *CollectionItemState) Annotate(a infer.Annotator) {
	a.Describe(&state.ItemID,
		"The Webflow-assigned item ID (read-only). "+
			"This is automatically set by Webflow when the item is created.")

	a.Describe(&state.LastPublished,
		"The timestamp when the item was last published (RFC3339 format, read-only). "+
			"This is automatically updated by Webflow when the item is published.")

	a.Describe(&state.LastUpdated,
		"The timestamp when the item was last updated (RFC3339 format, read-only). "+
			"This is automatically updated by Webflow whenever the item is modified.")

	a.Describe(&state.CreatedOn,
		"The timestamp when the item was created (RFC3339 format, read-only). "+
			"This is automatically set by Webflow and is immutable.")
}

// fieldDataEqual compares two fieldData maps, treating nil and empty as equal.
func fieldDataEqual(a, b map[string]interface{}) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// optionalBoolChanged reports whether an optional boolean input differs from state.
// A nil input means "don't manage this flag" and never produces a diff, so a value that
// Read populated from the API is not compared against an omitted input.
func optionalBoolChanged(state, input *bool) bool {
	if input == nil {
		return false
	}
	return state == nil || *state != *input
}

// projectFieldData narrows the fieldData reported by the API to the keys the user manages.
// The API returns every field of the collection (including ones the program never set), so
// comparing the full object against the inputs would produce a permanent diff. When wanted
// is empty (import), the full API object is returned.
func projectFieldData(apiFieldData, wanted map[string]interface{}) map[string]interface{} {
	if len(wanted) == 0 {
		return apiFieldData
	}
	projected := make(map[string]interface{}, len(wanted))
	for key := range wanted {
		if value, ok := apiFieldData[key]; ok {
			projected[key] = value
		}
	}
	return projected
}

// collectionItemValidators are the per-property string checks shared by Check and the
// apply-time validation.
var collectionItemValidators = []stringValidator{
	{property: "collectionId", validate: ValidateCollectionID},
	{property: "cmsLocaleId", validate: ValidateCmsLocaleID},
}

// checkFieldDataKeys reports the required fieldData keys (name, slug) that are missing or
// known to be empty. Values that are still unknown are accepted: presence is all that can
// be checked at preview time.
func checkFieldDataKeys(fieldData property.Map) []p.CheckFailure {
	var failures []p.CheckFailure
	for _, key := range []string{"name", "slug"} {
		v, ok := fieldData.GetOk(key)
		if !ok || v.IsNull() {
			failures = append(failures, checkFailure("fieldData", fmt.Errorf(
				"fieldData.%s is required by the Webflow API. "+
					"Provide it alongside the other fields, e.g. {\"name\": \"My Item\", \"slug\": \"my-item\"}", key)))
			continue
		}
		if v.IsString() && v.AsString() == "" {
			failures = append(failures, checkFailure("fieldData", fmt.Errorf(
				"fieldData.%s must be a non-empty string", key)))
		}
	}
	return failures
}

// Check validates the known inputs at preview time. Unknown (computed) values - a
// collectionId that comes from a Collection resource, or a fieldData value that comes from
// another resource - are skipped and validated again in Create/Update once resolved.
func (c *CollectionItemResource) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[CollectionItemArgs], error) {
	inputs, failures, err := checkStrings[CollectionItemArgs](ctx, req.NewInputs, collectionItemValidators...)
	if err != nil {
		return infer.CheckResponse[CollectionItemArgs]{Inputs: inputs, Failures: failures}, err
	}
	// The set of keys in fieldData is known even when some of the values are not, so the
	// required keys can be checked as long as the map itself is not unknown.
	if v, ok := req.NewInputs.GetOk("fieldData"); ok && v.IsMap() {
		failures = append(failures, checkFieldDataKeys(v.AsMap())...)
	}
	return infer.CheckResponse[CollectionItemArgs]{Inputs: inputs, Failures: failures}, nil
}

// validateCollectionItemArgs validates fully-resolved inputs at apply time.
func validateCollectionItemArgs(args CollectionItemArgs) error {
	if err := ValidateCollectionID(args.CollectionID); err != nil {
		return fmt.Errorf("validation failed for CollectionItem resource: %w", err)
	}
	if err := ValidateFieldData(args.FieldData); err != nil {
		return fmt.Errorf("validation failed for CollectionItem resource: %w", err)
	}
	if err := ValidateCmsLocaleID(args.CmsLocaleID); err != nil {
		return fmt.Errorf("validation failed for CollectionItem resource: %w", err)
	}
	return nil
}

// Diff determines what changes need to be made to the collection item resource.
// A collectionId change requires replacement: the item is created in the new collection
// before the old one is deleted (different collections cannot conflict on slug).
// Everything else is updated in place.
func (c *CollectionItemResource) Diff(
	ctx context.Context, req infer.DiffRequest[CollectionItemArgs, CollectionItemState],
) (infer.DiffResponse, error) {
	diff := infer.DiffResponse{}
	detailedDiff := map[string]p.PropertyDiff{}

	update := func(property string) {
		detailedDiff[property] = p.PropertyDiff{Kind: p.Update}
		diff.HasChanges = true
	}

	if req.State.CollectionID != req.Inputs.CollectionID {
		detailedDiff["collectionId"] = p.PropertyDiff{Kind: p.UpdateReplace}
		diff.HasChanges = true
	}

	if !fieldDataEqual(req.State.FieldData, req.Inputs.FieldData) {
		update("fieldData")
	}
	if optionalBoolChanged(req.State.IsArchived, req.Inputs.IsArchived) {
		update("isArchived")
	}
	if optionalBoolChanged(req.State.IsDraft, req.Inputs.IsDraft) {
		update("isDraft")
	}

	// An empty cmsLocaleId input means "don't care", not "remove it": the API reports a
	// locale for every item on localized sites even when the program never set one.
	if req.Inputs.CmsLocaleID != "" && req.State.CmsLocaleID != req.Inputs.CmsLocaleID {
		update("cmsLocaleId")
	}

	if req.State.Live != req.Inputs.Live {
		update("live")
	}

	if len(detailedDiff) > 0 {
		diff.DetailedDiff = detailedDiff
	}
	return diff, nil
}

// buildCollectionItemState assembles the resource state from the inputs and the item the
// API returned. FieldData keeps the user's view (see projectFieldData); flags and
// timestamps come from the API.
func buildCollectionItemState(args CollectionItemArgs, item *CollectionItem) CollectionItemState {
	state := CollectionItemState{
		CollectionItemArgs: args,
		ItemID:             item.ID,
		CreatedOn:          item.CreatedOn,
		LastUpdated:        item.LastUpdated,
		LastPublished:      item.LastPublished,
	}
	isArchived, isDraft := item.IsArchived, item.IsDraft
	state.IsArchived = &isArchived
	state.IsDraft = &isDraft
	if item.CmsLocaleID != "" {
		state.CmsLocaleID = item.CmsLocaleID
	}
	return state
}

// publishAndRefresh publishes item and, when possible, re-reads the live copy so that
// lastPublished and the other server-managed fields reflect the published state.
// The returned item is the refreshed copy, or the original when the live read is not yet available.
func publishAndRefresh(
	ctx context.Context, client *http.Client, collectionID string, item *CollectionItem, cmsLocaleID string,
) (*CollectionItem, error) {
	if _, err := PublishCollectionItems(ctx, client, collectionID, []string{item.ID}, cmsLocaleID); err != nil {
		return nil, fmt.Errorf("failed to publish collection item %s: %w", item.ID, err)
	}
	live, err := GetCollectionItem(ctx, client, collectionID, item.ID, cmsLocaleID, true)
	if err != nil {
		if IsNotFound(err) {
			// Publishing is asynchronous; the live copy may not be readable yet.
			NewLogContext(ctx).WithField("itemId", item.ID).
				Debug("Live item not yet available after publish; keeping staged response")
			return item, nil
		}
		return nil, fmt.Errorf("failed to read published collection item %s: %w", item.ID, err)
	}
	return live, nil
}

// Create creates a new collection item in the Webflow collection.
func (c *CollectionItemResource) Create(
	ctx context.Context, req infer.CreateRequest[CollectionItemArgs],
) (infer.CreateResponse[CollectionItemState], error) {
	// During preview, return the expected state without validating or calling the API:
	// inputs that come from other resources (e.g. collectionId) are unknown at this point.
	// The ID is left empty so dependents see it as unknown, and server-assigned outputs
	// (itemId, timestamps) are left empty rather than fabricated.
	if req.DryRun {
		return infer.CreateResponse[CollectionItemState]{
			Output: CollectionItemState{CollectionItemArgs: req.Inputs},
		}, nil
	}

	if err := validateCollectionItemArgs(req.Inputs); err != nil {
		return infer.CreateResponse[CollectionItemState]{}, err
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.CreateResponse[CollectionItemState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	item, err := PostCollectionItem(ctx, client, req.Inputs.CollectionID, CollectionItemRequest{
		FieldData:   req.Inputs.FieldData,
		IsArchived:  req.Inputs.IsArchived,
		IsDraft:     req.Inputs.IsDraft,
		CmsLocaleID: req.Inputs.CmsLocaleID,
	})
	if err != nil {
		return infer.CreateResponse[CollectionItemState]{}, fmt.Errorf("failed to create collection item: %w", err)
	}
	if item.ID == "" {
		return infer.CreateResponse[CollectionItemState]{}, errors.New(
			"webflow API returned empty item ID - " +
				"this is unexpected and may indicate an API issue")
	}

	if req.Inputs.Live {
		stagedID := item.ID
		item, err = publishAndRefresh(ctx, client, req.Inputs.CollectionID, item, req.Inputs.CmsLocaleID)
		if err != nil {
			return infer.CreateResponse[CollectionItemState]{}, fmt.Errorf(
				"%w. The staged item %s was created; import it with ID %q or delete it in the Webflow CMS",
				err, stagedID, GenerateCollectionItemResourceID(req.Inputs.CollectionID, stagedID))
		}
	}

	return infer.CreateResponse[CollectionItemState]{
		ID:     GenerateCollectionItemResourceID(req.Inputs.CollectionID, item.ID),
		Output: buildCollectionItemState(req.Inputs, item),
	}, nil
}

// Read retrieves the current state of a collection item from Webflow.
// Used for drift detection and import operations.
func (c *CollectionItemResource) Read(
	ctx context.Context, req infer.ReadRequest[CollectionItemArgs, CollectionItemState],
) (infer.ReadResponse[CollectionItemArgs, CollectionItemState], error) {
	collectionID, itemID, err := ExtractIDsFromCollectionItemResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateItemID(itemID); err != nil {
		return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{}, fmt.Errorf(
			"failed to create HTTP client: %w", err)
	}

	// Live items are read from the live endpoint; if the item exists but has not been
	// published yet (or was unpublished), fall back to the staged copy.
	var item *CollectionItem
	if req.Inputs.Live {
		item, err = GetCollectionItem(ctx, client, collectionID, itemID, req.Inputs.CmsLocaleID, true)
		if err != nil && !IsNotFound(err) {
			return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{}, fmt.Errorf(
				"failed to read live collection item: %w", err)
		}
	}
	if item == nil {
		item, err = GetCollectionItem(ctx, client, collectionID, itemID, req.Inputs.CmsLocaleID, false)
		if err != nil {
			if IsNotFound(err) {
				// Item no longer exists - return empty ID to signal deletion
				return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{}, nil
			}
			return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{}, fmt.Errorf(
				"failed to read collection item: %w", err)
		}
	}

	// Inputs the user omitted stay omitted: isArchived/isDraft/cmsLocaleId are "don't care"
	// when absent. Explicit inputs are refreshed from the API so drift is visible.
	currentInputs := CollectionItemArgs{
		CollectionID: collectionID,
		FieldData:    projectFieldData(item.FieldData, req.Inputs.FieldData),
		IsArchived:   req.Inputs.IsArchived,
		IsDraft:      req.Inputs.IsDraft,
		CmsLocaleID:  req.Inputs.CmsLocaleID,
		Live:         req.Inputs.Live,
	}
	if req.Inputs.IsArchived != nil {
		isArchived := item.IsArchived
		currentInputs.IsArchived = &isArchived
	}
	if req.Inputs.IsDraft != nil {
		isDraft := item.IsDraft
		currentInputs.IsDraft = &isDraft
	}
	if req.Inputs.CmsLocaleID != "" && item.CmsLocaleID != "" {
		currentInputs.CmsLocaleID = item.CmsLocaleID
	}

	return infer.ReadResponse[CollectionItemArgs, CollectionItemState]{
		ID:     req.ID,
		Inputs: currentInputs,
		State:  buildCollectionItemState(currentInputs, item),
	}, nil
}

// Update modifies an existing collection item.
func (c *CollectionItemResource) Update(
	ctx context.Context, req infer.UpdateRequest[CollectionItemArgs, CollectionItemState],
) (infer.UpdateResponse[CollectionItemState], error) {
	// During preview, return the expected state without making API calls.
	// Server-managed timestamps are carried over rather than fabricated.
	if req.DryRun {
		return infer.UpdateResponse[CollectionItemState]{
			Output: CollectionItemState{
				CollectionItemArgs: req.Inputs,
				ItemID:             req.State.ItemID,
				CreatedOn:          req.State.CreatedOn,
				LastUpdated:        req.State.LastUpdated,
				LastPublished:      req.State.LastPublished,
			},
		}, nil
	}

	if err := validateCollectionItemArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[CollectionItemState]{}, err
	}

	collectionID, itemID, err := ExtractIDsFromCollectionItemResourceID(req.ID)
	if err != nil {
		return infer.UpdateResponse[CollectionItemState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.UpdateResponse[CollectionItemState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateItemID(itemID); err != nil {
		return infer.UpdateResponse[CollectionItemState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.UpdateResponse[CollectionItemState]{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	item, err := PatchCollectionItem(ctx, client, collectionID, itemID, CollectionItemRequest{
		FieldData:   prepareFieldDataForPatch(req.State.FieldData, req.Inputs.FieldData),
		IsArchived:  req.Inputs.IsArchived,
		IsDraft:     req.Inputs.IsDraft,
		CmsLocaleID: req.Inputs.CmsLocaleID,
	})
	if err != nil {
		return infer.UpdateResponse[CollectionItemState]{}, fmt.Errorf("failed to update collection item: %w", err)
	}

	if req.Inputs.Live {
		item, err = publishAndRefresh(ctx, client, collectionID, item, req.Inputs.CmsLocaleID)
		if err != nil {
			return infer.UpdateResponse[CollectionItemState]{}, err
		}
	}

	state := buildCollectionItemState(req.Inputs, item)
	if state.ItemID == "" {
		state.ItemID = req.State.ItemID
	}
	if state.CreatedOn == "" {
		state.CreatedOn = req.State.CreatedOn
	}
	if state.LastPublished == "" {
		state.LastPublished = req.State.LastPublished
	}

	return infer.UpdateResponse[CollectionItemState]{Output: state}, nil
}

// Delete removes a collection item from the Webflow collection.
// Items that were published through live=true are first removed from the live site,
// then the staged copy is deleted. Both calls carry the item's cmsLocaleId (when set):
// without it Webflow only deletes the item in the primary locale.
func (c *CollectionItemResource) Delete(
	ctx context.Context, req infer.DeleteRequest[CollectionItemState],
) (infer.DeleteResponse, error) {
	collectionID, itemID, err := ExtractIDsFromCollectionItemResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateCollectionID(collectionID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	if err := ValidateItemID(itemID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	client, err := GetHTTPClient(ctx, currentProviderVersion())
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// The locale is sent as-is (URL-encoded): it was recorded from the inputs or the API.
	cmsLocaleID := req.State.CmsLocaleID

	// Both calls treat 404 as success so deletes are idempotent
	if req.State.Live {
		if err := DeleteCollectionItem(ctx, client, collectionID, itemID, cmsLocaleID, true); err != nil {
			return infer.DeleteResponse{}, fmt.Errorf("failed to unpublish collection item: %w", err)
		}
	}
	if err := DeleteCollectionItem(ctx, client, collectionID, itemID, cmsLocaleID, false); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete collection item: %w", err)
	}

	return infer.DeleteResponse{}, nil
}
