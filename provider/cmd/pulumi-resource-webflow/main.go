// Copyright 2025, Justin Detmar.
// SPDX-License-Identifier: MIT
//
// This is an unofficial, community-maintained Pulumi provider for Webflow.
// Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.

// Package main runs the provider's gRPC server.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/JDetmar/pulumi-webflow/provider"
)

// Serve the provider against Pulumi's Provider protocol.
func main() {
	err := provider.Provider().Run(context.Background(), provider.Name, provider.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
