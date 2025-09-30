// Package printer provides utilities for formatting and printing
// BattlEye RCon responses in various output formats (table, JSON, raw, etc.).
package printer

import "strings"

// Format is an output format selector for printer.
type Format int

const (
	// FormatTable prints responses in human-friendly table format.
	FormatTable Format = iota
	// FormatJSON prints responses as JSON.
	FormatJSON
	// FormatPlain prints raw responses as plain text.
	FormatPlain
	// FormatMarkdown prints responses as Markdown tables or code blocks.
	FormatMarkdown
	// FormatHTML prints responses as HTML tables or pre blocks.
	FormatHTML
)

// FormatFromString converts a string like "json", "table", "raw" into a Format constant.
// Unknown values default to FormatTable.
func FormatFromString(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON

	case "plain", "text", "raw":
		return FormatPlain

	case "md", "markdown":
		return FormatMarkdown

	case "html", "htm":
		return FormatHTML

	default:
		return FormatTable // "table" or any
	}
}
