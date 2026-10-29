// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

/*
Package output recognizes the RV and EF response schemas
selected by a command, adapts their typed rows to CLI-compatible values,
and renders raw, table, JSON, Markdown, HTML, or CSV output.
Unknown and malformed payloads remain visible
as raw server text instead of becoming empty structured data.
*/
package output
