// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

package provider

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestEcommerceSettingsCheck(t *testing.T) {
	r := &EcommerceSettings{}

	bad, err := r.Check(context.Background(), infer.CheckRequest{
		NewInputs: property.NewMap(map[string]property.Value{"siteId": property.New("nope")}),
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(bad.Failures) != 1 || bad.Failures[0].Property != "siteId" {
		t.Fatalf("expected one siteId failure, got %+v", bad.Failures)
	}

	unknown, err := r.Check(context.Background(), infer.CheckRequest{
		NewInputs: property.NewMap(map[string]property.Value{"siteId": property.New(property.Computed)}),
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(unknown.Failures) != 0 {
		t.Fatalf("unknown siteId must not fail Check, got %+v", unknown.Failures)
	}

	good, err := r.Check(context.Background(), infer.CheckRequest{
		NewInputs: property.NewMap(map[string]property.Value{"siteId": property.New("5f0c8c9e1c9d440000e8d8c3")}),
	})
	if err != nil || len(good.Failures) != 0 {
		t.Fatalf("valid siteId must pass, err=%v failures=%+v", err, good.Failures)
	}
}
