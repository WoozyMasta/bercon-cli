package printer

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/woozymasta/bercon-cli/pkg/beparser"
)

// Table style (compact, fits terminals well)
func baseTable() table.Writer {
	t := table.NewWriter()
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

	return t
}

func renderPlayersTable(w io.Writer, players beparser.Players, withGeo bool, format Format) error {
	t := baseTable()
	if withGeo {
		t.AppendHeader(table.Row{"#", "IP:Port", "Ping", "GUID", "Name", "Country", "City", "Lat", "Lon"})
	} else {
		t.AppendHeader(table.Row{"#", "IP:Port", "Ping", "GUID", "Name"})
	}

	for _, p := range players {
		if withGeo {
			t.AppendRow(table.Row{
				p.ID,
				fmt.Sprintf("%s:%d", p.IP, p.Port),
				p.Ping,
				p.GUID,
				p.Name,
				p.Country,
				p.City,
				fmtCoord(p.Latitude),
				fmtCoord(p.Longitude),
			})
		} else {
			t.AppendRow(table.Row{
				p.ID,
				fmt.Sprintf("%s:%d", p.IP, p.Port),
				p.Ping,
				p.GUID,
				p.Name,
			})
		}
	}

	t.SetTitle("Players on server (%d in total)", len(players))
	t.Render()

	return renderTableWithFormat(w, t, format)
}

func renderAdminsTable(w io.Writer, admins beparser.Admins, withGeo bool, format Format) error {
	t := baseTable()
	if withGeo {
		t.AppendHeader(table.Row{"#", "IP:Port", "Country", "City", "Lat", "Lon"})
	} else {
		t.AppendHeader(table.Row{"#", "IP:Port", "Country"})
	}

	for _, a := range admins {
		if withGeo {
			t.AppendRow(table.Row{
				a.ID,
				fmt.Sprintf("%s:%d", a.IP, a.Port),
				a.Country,
				a.City,
				fmtCoord(a.Latitude),
				fmtCoord(a.Longitude),
			})
		} else {
			t.AppendRow(table.Row{
				a.ID,
				fmt.Sprintf("%s:%d", a.IP, a.Port),
				a.Country,
			})
		}
	}

	t.SetTitle("Connected RCon admins")
	t.Render()

	return renderTableWithFormat(w, t, format)
}

func renderBansTable(w io.Writer, bans beparser.Bans, withGeo bool, format Format) error {
	if len(bans.GUIDBans) > 0 {
		t := baseTable()
		t.SetTitle("GUID Bans")
		t.AppendHeader(table.Row{"#", "GUID", "Minutes left", "Reason"})

		for _, b := range bans.GUIDBans {
			t.AppendRow(table.Row{b.ID, b.GUID, minutesLeft(b.MinutesLeft), b.Reason})
		}

		t.Render()

		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}

		return renderTableWithFormat(w, t, format)
	}

	if len(bans.IPBans) > 0 {
		t := baseTable()
		t.SetTitle("IP Bans")

		if withGeo {
			t.AppendHeader(table.Row{"#", "IP", "Minutes left", "Reason", "Country", "City", "Lat", "Lon"})
			for _, b := range bans.IPBans {
				t.AppendRow(table.Row{
					b.ID, b.IP, minutesLeft(b.MinutesLeft), b.Reason,
					b.Country, b.City, fmtCoord(b.Latitude), fmtCoord(b.Longitude),
				})
			}
		} else {
			t.AppendHeader(table.Row{"#", "IP", "Minutes left", "Reason"})
			for _, b := range bans.IPBans {
				t.AppendRow(table.Row{b.ID, b.IP, minutesLeft(b.MinutesLeft), b.Reason})
			}
		}

		t.Render()
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}

		return renderTableWithFormat(w, t, format)
	}

	return nil
}

func minutesLeft(m int) string {
	if m < 0 {
		return "perm"
	}

	return strconv.Itoa(m)
}

func fmtCoord(v float64) string {
	if v == 0 {
		return ""
	}

	return fmt.Sprintf("%.5f", v)
}

func renderTableWithFormat(w io.Writer, t table.Writer, format Format) error {
	switch format {
	case FormatMarkdown:
		if md, ok := any(t).(interface{ RenderMarkdown() string }); ok {
			return writeBlock(w, md.RenderMarkdown())
		}

		return writeBlock(w, t.Render())

	case FormatHTML:
		if html, ok := any(t).(interface{ RenderHTML() string }); ok {
			return writeBlock(w, html.RenderHTML())
		}

		return writeBlock(w, t.Render())

	default:
		return writeBlock(w, t.Render())
	}
}

func writeBlock(w io.Writer, s string) error {
	if _, err := io.WriteString(w, s); err != nil {
		return err
	}

	if !strings.HasSuffix(s, "\n") {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}

	return nil
}
