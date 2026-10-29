// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"fmt"
	"io"
)

// writeDiagnostic keeps the primary CLI status if its diagnostic stream fails.
func writeDiagnostic(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
