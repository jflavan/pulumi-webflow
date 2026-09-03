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
	"strings"
)

// CollectionFieldResponse represents the Webflow API response for a collection field.
// This struct matches the Webflow API v2 response format for collection fields.
type CollectionFieldResponse struct {
	ID          string                 `json:"id"`                    // Webflow-assigned field ID (read-only)
	IsEditable  bool                   `json:"isEditable"`            // Whether the field can be edited (read-only)
	IsRequired  bool                   `json:"isRequired"`            // Whether the field is required
	Type        string                 `json:"type"`                  // Field type (PlainText, RichText, Image, etc.)
	Slug        string                 `json:"slug"`                  // URL-friendly slug for the field
	DisplayName string                 `json:"displayName"`           // Human-readable name of the field
	HelpText    string                 `json:"helpText,omitempty"`    // Optional help text for the field
	Validations map[string]interface{} `json:"validations,omitempty"` // Type-specific validations
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Option choices / referenced collection
}

// CollectionFieldCreateRequest represents the request body for POST /v2/collections/{id}/fields.
// The Create Field endpoint accepts type, displayName, id, isEditable, isRequired, helpText and
// metadata. It does not accept a slug (Webflow generates one from displayName) and
// "field validation is currently not available through the API", so neither is sent.
type CollectionFieldCreateRequest struct {
	Type        string                 `json:"type"`                 // Field type (required)
	DisplayName string                 `json:"displayName"`          // Human-readable name (required)
	IsRequired  *bool                  `json:"isRequired,omitempty"` // Whether the field is required
	HelpText    string                 `json:"helpText,omitempty"`   // Optional help text
	Metadata    map[string]interface{} `json:"metadata,omitempty"`   // Required for Option/Reference/MultiReference
}

// CollectionFieldUpdateRequest represents the request body for PATCH /v2/collections/{id}/fields/{fid}.
// The Webflow API only allows isRequired, displayName and helpText to change after creation.
// IsRequired is a pointer with omitempty so that an explicit false is still sent.
type CollectionFieldUpdateRequest struct {
	IsRequired  *bool  `json:"isRequired,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	HelpText    string `json:"helpText"`
}

// Valid field types for Webflow collection fields (Webflow Data API v2 "Create Field" enum).
const (
	FieldTypeColor          = "Color"
	FieldTypeDateTime       = "DateTime"
	FieldTypeEmail          = "Email"
	FieldTypeFile           = "File"
	FieldTypeImage          = "Image"
	FieldTypeLink           = "Link"
	FieldTypeMultiImage     = "MultiImage"
	FieldTypeNumber         = "Number"
	FieldTypePhone          = "Phone"
	FieldTypePlainText      = "PlainText"
	FieldTypeRichText       = "RichText"
	FieldTypeSwitch         = "Switch"
	FieldTypeVideoLink      = "VideoLink"
	FieldTypeOption         = "Option"
	FieldTypeMultiReference = "MultiReference"
	FieldTypeReference      = "Reference"
)

// ValidFieldTypes is a map of all valid field types for validation.
var ValidFieldTypes = map[string]bool{
	FieldTypeColor:          true,
	FieldTypeDateTime:       true,
	FieldTypeEmail:          true,
	FieldTypeFile:           true,
	FieldTypeImage:          true,
	FieldTypeLink:           true,
	FieldTypeMultiImage:     true,
	FieldTypeNumber:         true,
	FieldTypePhone:          true,
	FieldTypePlainText:      true,
	FieldTypeRichText:       true,
	FieldTypeSwitch:         true,
	FieldTypeVideoLink:      true,
	FieldTypeOption:         true,
	FieldTypeMultiReference: true,
	FieldTypeReference:      true,
}

// supportedFieldTypeList is the human-readable enum used in error messages and docs.
const supportedFieldTypeList = "Color, DateTime, Email, File, Image, Link, MultiImage, Number, Phone, " +
	"PlainText, RichText, Switch, VideoLink, Option, MultiReference, Reference"

// ValidateFieldType validates that a field type is one of the supported types.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateFieldType(fieldType string) error {
	if fieldType == "" {
		return errors.New("type is required but was not provided. " +
			"Please provide a valid field type (e.g., 'PlainText', 'RichText', 'Image'). " +
			"Supported types: " + supportedFieldTypeList)
	}
	if !ValidFieldTypes[fieldType] {
		return fmt.Errorf("type has invalid value: got '%s'. "+
			"Supported types: %s. "+
			"Please use one of the supported field types", fieldType, supportedFieldTypeList)
	}
	return nil
}

// ValidateFieldDisplayName validates that displayName is non-empty and reasonable length.
// Returns actionable error messages that explain what's wrong and how to fix it.
func ValidateFieldDisplayName(displayName string) error {
	if displayName == "" {
		return errors.New("displayName is required but was not provided. " +
			"Please provide a name for your field (e.g., 'Title', 'Description', 'Author'). " +
			"The display name is shown in the Webflow CMS interface")
	}
	if len(displayName) > 255 {
		return fmt.Errorf("displayName is too long: '%s' exceeds maximum length of 255 characters. "+
			"Please use a shorter, more concise name for your field", displayName)
	}
	return nil
}

// ValidateFieldMetadata checks that the metadata required by the field type is present.
// Option fields need metadata.options (a list of {name} objects); Reference and
// MultiReference fields need metadata.collectionId. Other types accept no metadata.
func ValidateFieldMetadata(fieldType string, metadata map[string]interface{}) error {
	switch fieldType {
	case FieldTypeOption:
		options, ok := metadata["options"]
		if !ok {
			return errors.New("metadata.options is required for Option fields. " +
				"Provide the choices as a list of objects with a 'name' key, " +
				"e.g., metadata: {options: [{name: 'Draft'}, {name: 'Published'}]}")
		}
		list, ok := options.([]interface{})
		if !ok || len(list) == 0 {
			return errors.New("metadata.options must be a non-empty list of {name: string} objects for Option fields")
		}
		for i, opt := range list {
			m, ok := opt.(map[string]interface{})
			if !ok {
				return fmt.Errorf("metadata.options[%d] must be an object with a 'name' key", i)
			}
			if name, ok := m["name"].(string); !ok || name == "" {
				return fmt.Errorf("metadata.options[%d].name is required and must be a non-empty string", i)
			}
		}
	case FieldTypeReference, FieldTypeMultiReference:
		id, ok := metadata["collectionId"].(string)
		if !ok || id == "" {
			return fmt.Errorf("metadata.collectionId is required for %s fields. "+
				"Provide the ID of the collection the field references, "+
				"e.g., metadata: {collectionId: '5f0c8c9e1c9d440000e8d8c3'}", fieldType)
		}
		if err := ValidateCollectionID(id); err != nil {
			return fmt.Errorf("metadata.collectionId is invalid: %w", err)
		}
	default:
		if len(metadata) > 0 {
			return fmt.Errorf("metadata is only supported for Option, Reference and MultiReference fields, "+
				"but was provided for a %s field. Remove the metadata input or change the field type", fieldType)
		}
	}
	return nil
}

// ValidateFieldID validates a Webflow field ID before it is used in an API path.
func ValidateFieldID(fieldID string) error {
	return validatePathID("fieldId", fieldID)
}

// GenerateCollectionFieldResourceID generates a Pulumi resource ID for a CollectionField resource.
// Format: {collectionID}/fields/{fieldID}
func GenerateCollectionFieldResourceID(collectionID, fieldID string) string {
	return fmt.Sprintf("%s/fields/%s", collectionID, fieldID)
}

// ExtractIDsFromCollectionFieldResourceID extracts the collectionID and fieldID from a CollectionField resource ID.
// Expected format: {collectionID}/fields/{fieldID}
func ExtractIDsFromCollectionFieldResourceID(resourceID string) (collectionID, fieldID string, err error) {
	if resourceID == "" {
		return "", "", errors.New("resourceId cannot be empty")
	}

	parts := strings.Split(resourceID, "/")
	if len(parts) < 3 || parts[1] != "fields" {
		return "", "", fmt.Errorf(
			"invalid resource ID format: expected {collectionId}/fields/{fieldId}, got: %s",
			resourceID,
		)
	}

	collectionID = parts[0]
	fieldID = strings.Join(parts[2:], "/") // Handle fieldID that might contain slashes

	return collectionID, fieldID, nil
}

// GetCollectionField retrieves a single collection field by ID.
// Webflow has no GET endpoint for an individual field, so the collection is fetched
// (GET /v2/collections/{collection_id}) and filtered. A missing collection or a field that
// is not part of it both yield an error satisfying IsNotFound.
func GetCollectionField(
	ctx context.Context, client *http.Client, collectionID, fieldID string,
) (*CollectionFieldResponse, error) {
	collection, err := GetCollection(ctx, client, collectionID)
	if err != nil {
		return nil, err
	}
	for i := range collection.Fields {
		if collection.Fields[i].ID == fieldID {
			return &collection.Fields[i], nil
		}
	}
	return nil, fmt.Errorf("field %s does not exist in collection %s: %w", fieldID, collectionID, ErrNotFound)
}

// PostCollectionField creates a new field for a Webflow collection.
// It calls POST /v2/collections/{collection_id}/fields.
func PostCollectionField(
	ctx context.Context, client *http.Client, collectionID string, body CollectionFieldCreateRequest,
) (*CollectionFieldResponse, error) {
	var field CollectionFieldResponse
	if _, err := doRequest(ctx, client, http.MethodPost,
		apiURL("/v2/collections/%s/fields", collectionID), body, &field,
		http.StatusOK, http.StatusCreated, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &field, nil
}

// metadataFromValidations reconstructs the create-time metadata of a field from the
// validations the Get Collection endpoint reports. Fields are read back without a metadata
// object: Option choices come back as validations.options ([{name, id}]) and referenced
// collections as validations.collectionId. For other field types nil is returned.
//
// With includeOptionIDs false the option entries carry only their name, which is the shape
// users write in their programs; with true the Webflow-assigned option IDs are kept.
func metadataFromValidations(
	fieldType string, validations map[string]interface{}, includeOptionIDs bool,
) map[string]interface{} {
	switch fieldType {
	case FieldTypeOption:
		raw, ok := validations["options"].([]interface{})
		if !ok {
			return nil
		}
		options := make([]interface{}, 0, len(raw))
		for _, entry := range raw {
			option, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			normalized := map[string]interface{}{"name": option["name"]}
			if id, ok := option["id"]; ok && includeOptionIDs {
				normalized["id"] = id
			}
			options = append(options, normalized)
		}
		return map[string]interface{}{"options": options}
	case FieldTypeReference, FieldTypeMultiReference:
		id, ok := validations["collectionId"].(string)
		if !ok || id == "" {
			return nil
		}
		return map[string]interface{}{"collectionId": id}
	default:
		return nil
	}
}

// PatchCollectionField updates the mutable properties of an existing field.
// It calls PATCH /v2/collections/{collection_id}/fields/{field_id}; only isRequired,
// displayName and helpText can be changed (type and metadata are create-only).
func PatchCollectionField(
	ctx context.Context, client *http.Client, collectionID, fieldID string, body CollectionFieldUpdateRequest,
) (*CollectionFieldResponse, error) {
	var field CollectionFieldResponse
	if _, err := doRequest(ctx, client, http.MethodPatch,
		apiURL("/v2/collections/%s/fields/%s", collectionID, fieldID), body, &field,
		http.StatusOK, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &field, nil
}

// DeleteCollectionField removes a field from a Webflow collection.
// It calls DELETE /v2/collections/{collection_id}/fields/{field_id}; a 404 is treated as success.
func DeleteCollectionField(ctx context.Context, client *http.Client, collectionID, fieldID string) error {
	return doDelete(ctx, client, apiURL("/v2/collections/%s/fields/%s", collectionID, fieldID), nil)
}

// subsetEqual reports whether every value in want is present and equal in have.
// Maps are compared key by key (extra keys in have are ignored), slices element by element
// (same length, each element subsetEqual), and numbers by value regardless of Go numeric type.
// It is used to compare user-supplied metadata against what the API reports, because the
// API decorates those objects with server-side keys (option IDs).
func subsetEqual(want, have interface{}) bool {
	switch w := want.(type) {
	case map[string]interface{}:
		h, ok := have.(map[string]interface{})
		if !ok {
			return false
		}
		for k, wv := range w {
			hv, ok := h[k]
			if !ok || !subsetEqual(wv, hv) {
				return false
			}
		}
		return true
	case []interface{}:
		h, ok := have.([]interface{})
		if !ok || len(h) != len(w) {
			return false
		}
		for i := range w {
			if !subsetEqual(w[i], h[i]) {
				return false
			}
		}
		return true
	default:
		if wf, ok := toFloat(want); ok {
			hf, ok := toFloat(have)
			return ok && wf == hf
		}
		return reflect.DeepEqual(want, have)
	}
}

// toFloat converts any Go numeric type to float64.
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
