package config

import (
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// PrintProfiles prints all available profiles in a table format.
// If rc is not found, it prints "no profiles found" to the writer.
func PrintProfiles(explicitPath string, base *RC, w io.Writer) error {
	f, ok, err := LoadRCFile(explicitPath)
	if err != nil {
		return err
	}

	if !ok || len(f.Profiles) == 0 {
		_, _ = fmt.Fprintln(w, "no profiles found")
		return nil
	}

	type row struct {
		Name, IP, Format, CfgSrc string
		Buffer                   uint16
		Timeout, Port            int
	}
	var rows []row

	for _, name := range f.ProfileNames() {
		// globals + profile
		rc, err := f.Effective(name)
		if err != nil {
			continue
		}

		// optionally fill missing misc fields from CLI/ENV defaults
		if base != nil {
			if rc.Format == "" && base.Format != "" {
				rc.Format = base.Format
			}
			if rc.TimeoutSec == 0 && base.TimeoutSec > 0 {
				rc.TimeoutSec = base.TimeoutSec
			}
			if rc.BufferSize == 0 && base.BufferSize > 0 {
				rc.BufferSize = base.BufferSize
			}
			if rc.GeoDB == "" && base.GeoDB != "" {
				rc.GeoDB = base.GeoDB
			}
		}

		ip := rc.IP
		port := rc.Port
		cfgsrc := rc.ServerCfg

		// server_cfg resolves ip/port from beserver_x64*.cfg
		if rc.ServerCfg != "" {
			if r, err := LoadFromBeServerCfg(rc.ServerCfg); err == nil {
				ip, port = r.IP, r.Port
			}
		}

		if ip == "" || ip == "0.0.0.0" {
			ip = "127.0.0.1"
		}

		rows = append(rows, row{
			Name:    name,
			IP:      ip,
			Port:    port,
			Buffer:  rc.BufferSize,
			Timeout: rc.TimeoutSec,
			Format:  rc.Format,
			CfgSrc:  cfgsrc,
		})
	}

	if len(rows) == 0 {
		_, _ = fmt.Fprintln(w, "no profiles found")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.Style{
		Name: "CompactLight",
		Box:  table.StyleBoxRounded,
		Color: table.ColorOptions{
			Header: text.Colors{text.Bold},
		},
		Format: table.FormatOptions{
			Header: text.FormatTitle,
		},
		Options: table.Options{
			SeparateColumns: true,
			SeparateHeader:  true,
			DrawBorder:      true,
		},
	})

	path, _ := resolveRCPath(explicitPath)
	t.SetTitle("Loaded from rc file: %s", path)
	t.AppendHeader(table.Row{"Profile", "IP", "Port", "Buffer", "Timeout", "Format", "Config Source"})
	for _, r := range rows {
		t.AppendRow(table.Row{r.Name, r.IP, r.Port, r.Buffer, r.Timeout, r.Format, r.CfgSrc})
	}
	t.Render()

	return nil
}
