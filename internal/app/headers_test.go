// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"strings"
	"testing"

	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/flags"
)

// TestGeneratedDocsFormatsPortablePathsAndKeepsManSectionsClean checks docs output by format.
func TestGeneratedDocsFormatsPortablePathsAndKeepsManSectionsClean(t *testing.T) {
	tests := []struct {
		name     string
		format   flags.DocFormat
		template string
		markup   string
		inline   string
	}{
		{
			name:     "markdown",
			format:   flags.DocFormatMarkdown,
			template: flags.DocTemplateMarkdownTable,
			markup:   "### Linux / BSD",
			inline:   "`[globals]`",
		},
		{
			name:     "html",
			format:   flags.DocFormatHTML,
			template: flags.DocTemplateHTMLDefault,
			markup:   "<ol>",
			inline:   "<code>[globals]</code>",
		},
		{
			name:     "man",
			format:   flags.DocFormatMan,
			template: flags.DocTemplateManDefault,
			markup:   ".TH ",
			inline:   ".SH ",
		},
	}

	parser, err := newParser(&options{})
	if err != nil {
		t.Fatalf("newParser() error = %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output strings.Builder
			if err := parser.WriteDoc(
				&output,
				test.format,
				flags.WithBuiltinTemplate(test.template),
			); err != nil {
				t.Fatalf("WriteDoc() error = %v", err)
			}

			text := output.String()
			expected := []string{test.markup, test.inline}
			if test.format != flags.DocFormatMan {
				expected = append(expected,
					"$XDG_CONFIG_HOME/bercon-cli/config.ini",
					"~/Library/Application Support/bercon-cli/config.ini",
					"%XDG_CONFIG_HOME%\\bercon-cli\\config.ini",
					"%APPDATA%",
					"%USERPROFILE%",
					appText("app.docs.priority", "Configuration priority"),
				)
			}

			for _, item := range expected {
				if !strings.Contains(text, item) {
					t.Errorf("generated documentation is missing %q", item)
				}
			}
			if test.format == flags.DocFormatMan &&
				(strings.Index(text, test.markup) > strings.Index(text, test.inline) ||
					strings.Contains(text, "%APPDATA%") || strings.Contains(text, "%USERPROFILE%") ||
					strings.Contains(text, appText("app.docs.profiles", "Server profiles")) ||
					strings.Contains(text, appText("app.docs.priority", "Configuration priority"))) {
				t.Errorf("man page has a nonstandard header or a Windows-specific path:\n%s", text)
			}

			for _, path := range config.DefaultPaths() {
				if path != "" && strings.Contains(text, path) {
					t.Errorf("generated documentation contains a machine-specific path %q", path)
				}
			}
		})
	}
}
