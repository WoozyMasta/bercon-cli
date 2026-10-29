// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package main

import (
	"os"

	"github.com/woozymasta/bercon-cli/internal/app"
)

// main passes process streams to the testable application entry point.
func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
