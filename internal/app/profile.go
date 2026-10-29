// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"fmt"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon-cli/internal/output"
)

// printExample writes the established globals and profile layout.
func printExample(w io.Writer) error {
	_, err := io.WriteString(w, "[globals]\nip = 127.0.0.1\nport = 2305\n"+
		"format = table\ntimeout = 3\n\n[profile.local]\n"+
		"password = replace-me\nserver_cfg = /srv/dayz/battleye\n")
	return err
}

// printProfiles lists endpoints without exposing stored passwords.
func printProfiles(w io.Writer, file *config.RCFile, found bool) error {
	if !found || len(file.Profiles) == 0 {
		_, err := fmt.Fprintln(w, appText("profile.empty", "No profiles found"))
		return err
	}

	writer := table.NewWriter()
	writer.SetStyle(table.Style{
		Name:  "CompactLight",
		Box:   table.StyleBoxRounded,
		Color: table.ColorOptions{Header: text.Colors{text.Bold}},
		Format: table.FormatOptions{
			Header: text.FormatTitle,
		},
		Options: table.Options{
			DrawBorder:      true,
			SeparateColumns: true,
			SeparateHeader:  true,
		},
	})
	writer.SetTitle(appText("profile.title", "Loaded from RC file: %s"), file.Path)
	writer.AppendHeader(table.Row{
		appText("profile.column.name", "Profile"),
		appText("profile.column.ip", "IP"),
		appText("profile.column.port", "Port"),
		appText("profile.column.buffer", "Buffer"),
		appText("profile.column.timeout", "Timeout"),
		appText("profile.column.format", "Format"),
		appText("profile.column.config", "Config source"),
	})

	for _, name := range file.ProfileNames() {
		rc, _ := file.Effective(name)
		ip := rc.IP
		if ip == "" || ip == "0.0.0.0" {
			ip = "127.0.0.1"
		}

		format := rc.Format
		if format == "" {
			format = string(output.Table)
		}

		buffer := rc.BufferSize
		if buffer == 0 {
			buffer = 1024
		}

		timeout := rc.Timeout
		if timeout == 0 {
			timeout = config.Duration(3 * time.Second)
		}

		writer.AppendRow(table.Row{
			name, ip, rc.Port, buffer, timeout.String(), format, rc.ServerCfg,
		})
	}

	_, err := io.WriteString(w, writer.Render()+"\n")
	return err
}
