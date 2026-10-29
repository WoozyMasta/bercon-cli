// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import "time"

var (
	// version is the application version injected by the linker.
	version = "dev"

	// commit is the source revision injected by the linker.
	commit = "unknown"

	// buildTime is the parsed UTC build timestamp used by the version command.
	buildTime = time.Unix(0, 0)

	// buildTimeOverride receives the linker timestamp before init parses it.
	buildTimeOverride string

	// repositoryURL is the project URL shown by the version command.
	repositoryURL = "https://github.com/woozymasta/bercon-cli"
)

// init parses the linker-provided build timestamp when it uses RFC3339.
func init() {
	if buildTimeOverride == "" {
		return
	}

	parsed, err := time.Parse(time.RFC3339, buildTimeOverride)
	if err == nil {
		buildTime = parsed.UTC()
	}
}
