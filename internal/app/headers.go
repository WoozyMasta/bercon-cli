// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/flags"
)

// documentationMarkdownWidth keeps generated Markdown readable in source form.
const documentationMarkdownWidth = 80

// docsPathGroup describes default config locations for one operating system family.
type docsPathGroup struct {
	name    string
	entries []string
}

// applicationHelpHeader builds the localized overview shown by --help with local config paths.
func applicationHelpHeader() string {
	return strings.Join([]string{
		appText("app.header", "Command-line client for servers using BattlEye RCon."), "",
		appText("app.help.profiles", "Profiles and configuration:"),
		appText("app.help.profiles_desc", "Use [globals] for shared values and [profile.NAME] for each server."),
		appText("app.help.config_paths", "When --config/-c is omitted, search these paths in order:"),
		applicationConfigPaths(), "",
		appText("app.help.priority", "Parameter priority, lowest to highest:"),
		appText("app.help.priority_order", "defaults < .env < environment < config < CLI"), "", "",
	}, "\n")
}

// applicationDocsHeader builds a localized, portable overview formatted for one documentation type.
func applicationDocsHeader(format flags.DocFormat) string {
	switch format {
	case flags.DocFormatMarkdown:
		return applicationMarkdownDocsHeader()

	case flags.DocFormatHTML:
		return applicationHTMLDocsHeader()

	case flags.DocFormatMan:
		return ""

	default:
		return ""
	}
}

// applicationMarkdownDocsHeader formats portable paths and translated priority as Markdown lists.
func applicationMarkdownDocsHeader() string {
	lines := []string{
		appText("app.header", "Command-line client for servers using BattlEye RCon."), "",
		"## " + appText("app.docs.profiles", "Server profiles"), "",
	}
	lines = append(lines, wrapMarkdownText(docsInline(flags.DocFormatMarkdown,
		appText("app.help.profiles_desc", "Use [globals] for shared values and [profile.NAME] for each server.")))...)
	lines = append(lines, "")
	lines = append(lines, wrapMarkdownText(docsInline(flags.DocFormatMarkdown,
		appText("app.help.config_paths", "When --config/-c is omitted, search these paths in order:")))...)
	lines = append(lines, "")

	for _, group := range applicationDocsPathGroups() {
		lines = append(lines, "### "+group.name, "")
		for index, entry := range group.entries {
			lines = append(lines, wrapMarkdownText(
				fmt.Sprintf("%d. %s", index+1, docsInline(flags.DocFormatMarkdown, entry)),
			)...)
		}
		lines = append(lines, "")
	}

	lines = append(lines,
		"## "+appText("app.docs.priority", "Configuration priority"), "",
		appText("app.help.priority", "Parameter priority, lowest to highest:"), "",
	)
	for index, item := range applicationPriorityItems() {
		lines = append(lines, wrapMarkdownText(
			fmt.Sprintf("%d. %s", index+1, docsInline(flags.DocFormatMarkdown, item)),
		)...)
	}

	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// wrapMarkdownText wraps prose at the configured width and aligns ordered-list continuations.
func wrapMarkdownText(text string) []string {
	firstPrefix := ""
	content := text
	if prefix, rest, ok := strings.Cut(text, " "); ok && strings.HasSuffix(prefix, ".") {
		if _, err := strconv.Atoi(strings.TrimSuffix(prefix, ".")); err == nil {
			firstPrefix = prefix + " "
			content = rest
		}
	}

	continuationPrefix := strings.Repeat(" ", utf8.RuneCountInString(firstPrefix))
	words := markdownWords(content)
	lines := make([]string, 0, len(words))
	line := firstPrefix
	for _, word := range words {
		separator := " "
		if line == firstPrefix || line == continuationPrefix {
			separator = ""
		}

		if line != "" && line != firstPrefix && line != continuationPrefix && markdownSentenceBoundary(line, word) {
			lines = append(lines, line)
			line = continuationPrefix + word

			continue
		}

		if line != "" && utf8.RuneCountInString(line+separator+word) > documentationMarkdownWidth {
			lines = append(lines, line)
			line = continuationPrefix + word
			continue
		}

		line += separator + word
	}
	if line != "" {
		lines = append(lines, line)
	}

	return lines
}

// markdownSentenceBoundary starts a new source line between complete sentences.
func markdownSentenceBoundary(line, nextWord string) bool {
	lineRunes := []rune(strings.TrimSpace(line))
	nextRunes := []rune(nextWord)
	if len(lineRunes) == 0 || len(nextRunes) == 0 {
		return false
	}

	last := lineRunes[len(lineRunes)-1]
	first := nextRunes[0]
	return (last == '.' || last == '!' || last == '?') && unicode.IsUpper(first)
}

// markdownWords keeps spaces inside inline-code spans together while splitting prose.
func markdownWords(text string) []string {
	var words []string
	var word strings.Builder
	inCode := false

	for _, char := range text {
		if char == '`' {
			inCode = !inCode
		}
		if char == ' ' && !inCode {
			if word.Len() != 0 {
				words = append(words, word.String())
				word.Reset()
			}

			continue
		}

		word.WriteRune(char)
	}
	if word.Len() != 0 {
		words = append(words, word.String())
	}

	return words
}

// applicationHTMLDocsHeader uses escaped localized text and semantic HTML markup.
func applicationHTMLDocsHeader() string {
	var lines []string
	lines = append(lines,
		"<p>"+docsInline(flags.DocFormatHTML,
			appText("app.header", "Command-line client for servers using BattlEye RCon."))+"</p>",
		"<h2>"+html.EscapeString(appText("app.docs.profiles", "Server profiles"))+"</h2>",
		"<p>"+docsInline(flags.DocFormatHTML,
			appText("app.help.profiles_desc", "Use [globals] for shared values and [profile.NAME] for each server."))+"</p>",
		"<p>"+docsInline(flags.DocFormatHTML,
			appText("app.help.config_paths", "When --config/-c is omitted, search these paths in order:"))+"</p>",
	)
	for _, group := range applicationDocsPathGroups() {
		lines = append(lines, "<h3>"+html.EscapeString(group.name)+"</h3>", "<ol>")
		for _, entry := range group.entries {
			lines = append(lines, "  <li>"+docsInline(flags.DocFormatHTML, entry)+"</li>")
		}
		lines = append(lines, "</ol>")
	}

	lines = append(lines,
		"<h2>"+html.EscapeString(appText("app.docs.priority", "Configuration priority"))+"</h2>",
		"<p>"+html.EscapeString(appText("app.help.priority", "Parameter priority, lowest to highest:"))+"</p>",
		"<ol>",
	)
	for _, item := range applicationPriorityItems() {
		lines = append(lines, "  <li>"+docsInline(flags.DocFormatHTML, item)+"</li>")
	}
	lines = append(lines, "</ol>")

	return strings.Join(lines, "\n")
}

// applicationDocsPathGroups returns OS-specific paths without embedding the current user's home directory.
func applicationDocsPathGroups() []docsPathGroup {
	xdgPaths := func(root, home, separator string) string {
		return fmt.Sprintf(
			appText("app.docs.path.if_set_else", "If %s is set, use %s; otherwise, use %s."),
			"`"+root+"`",
			"`"+root+separator+"bercon-cli"+separator+"config.ini`",
			"`"+home+separator+"bercon-cli"+separator+"config.ini`",
		)
	}

	return []docsPathGroup{
		{
			name: "Linux / BSD",
			entries: []string{
				xdgPaths("$XDG_CONFIG_HOME", "~/.config", "/"),
				"`~/.bercon-cli.ini`",
			},
		},
		{
			name: "macOS",
			entries: []string{
				"`~/Library/Application Support/bercon-cli/config.ini`",
				xdgPaths("$XDG_CONFIG_HOME", "~/.config", "/"),
				"`~/.bercon-cli.ini`",
			},
		},
		{
			name: "Windows",
			entries: []string{
				fmt.Sprintf(
					appText("app.docs.path.if_set", "If %s is set, use %s"),
					"`%APPDATA%`", "`%APPDATA%\\bercon-cli\\config.ini`",
				),
				xdgPaths("%XDG_CONFIG_HOME%", "%USERPROFILE%\\.config", "\\"),
				"`%USERPROFILE%\\.bercon-cli.ini`",
			},
		},
	}
}

// applicationPriorityItems removes translated numbering so each documentation format can render its own list.
func applicationPriorityItems() []string {
	lines := strings.Split(appText("app.help.priority_order", ""), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimPrefix(line, fmt.Sprintf("%d. ", index+1))
	}

	return lines
}

// applicationConfigPaths formats only the host-specific locations shown by interactive help.
func applicationConfigPaths() string {
	paths := config.DefaultPaths()
	lines := make([]string, len(paths))
	for index, path := range paths {
		lines[index] = fmt.Sprintf("%d. %s", index+1, path)
	}

	return strings.Join(lines, "\n")
}

// docsInline applies format-safe inline code markup to localized documentation text.
func docsInline(format flags.DocFormat, text string) string {
	text = strings.NewReplacer(
		"[profile.NAME]", "`[profile.NAME]`",
		"[globals]", "`[globals]`",
		"--profile NAME", "`--profile NAME`",
		"--config PATH", "`--config PATH`",
		"--config/-c", "`--config/-c`",
		".env", "`.env`",
	).Replace(text)
	if format == flags.DocFormatMarkdown {
		return text
	}

	var output strings.Builder
	parts := strings.Split(text, "`")
	for index, part := range parts {
		if index%2 == 0 {
			output.WriteString(html.EscapeString(part))

			continue
		}

		output.WriteString("<code>")
		output.WriteString(html.EscapeString(part))
		output.WriteString("</code>")
	}

	return output.String()
}
