// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Helpers for implementing infer.CustomCheck.
//
// Check runs during `pulumi preview` with the raw property map, in which values that depend
// on other resources' outputs are still "computed" (unknown). Validation must therefore
// only look at known values: an unknown value is skipped and validated again in Create or
// Update once it is resolved. These helpers make that pattern short.

// checkFailure builds a CheckFailure for a property.
func checkFailure(property string, err error) p.CheckFailure {
	return p.CheckFailure{Property: property, Reason: err.Error()}
}

// knownString returns the string value of key in inputs and whether it is known.
// Missing, null, computed, or non-string values report known=false.
func knownString(inputs property.Map, key string) (value string, known bool) {
	v, ok := inputs.GetOk(key)
	if !ok || v.IsNull() || v.IsComputed() || v.HasComputed() || !v.IsString() {
		return "", false
	}
	return v.AsString(), true
}

// knownNumber returns the numeric value of key in inputs and whether it is known.
func knownNumber(inputs property.Map, key string) (value float64, known bool) {
	v, ok := inputs.GetOk(key)
	if !ok || v.IsNull() || v.IsComputed() || v.HasComputed() || !v.IsNumber() {
		return 0, false
	}
	return v.AsNumber(), true
}

// isKnown reports whether key is present in inputs and fully resolved.
func isKnown(inputs property.Map, key string) bool {
	v, ok := inputs.GetOk(key)
	return ok && !v.IsNull() && !v.IsComputed() && !v.HasComputed()
}

// stringValidator validates one known string input.
type stringValidator struct {
	property string
	validate func(string) error
}

// checkStrings decodes inputs with DefaultCheck and applies each validator to its property
// when that property is known. Validators for unknown properties are skipped. It returns the
// decoded inputs and the accumulated failures so callers can add resource-specific checks.
func checkStrings[I any](
	ctx context.Context, inputs property.Map, validators ...stringValidator,
) (I, []p.CheckFailure, error) {
	decoded, failures, err := infer.DefaultCheck[I](ctx, inputs)
	if err != nil {
		return decoded, failures, err
	}
	for _, sv := range validators {
		value, known := knownString(inputs, sv.property)
		if !known {
			continue
		}
		if verr := sv.validate(value); verr != nil {
			failures = append(failures, checkFailure(sv.property, verr))
		}
	}
	return decoded, failures, nil
}
