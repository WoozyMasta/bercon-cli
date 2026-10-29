// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

const (
	// exitSuccess means the requested action completed successfully.
	exitSuccess = iota
	// exitRuntimeError means session setup or execution failed.
	exitRuntimeError
	// exitUsageError means CLI arguments or required options are invalid.
	exitUsageError
)

// exitCode names the process statuses returned by internal session handlers.
type exitCode int
