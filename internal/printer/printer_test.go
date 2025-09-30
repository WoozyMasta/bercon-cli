package printer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func loadData(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("test_data", name))
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}

	return b
}

func geoPath() string {
	if p := os.Getenv("BERCON_GEO_DB"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	for _, c := range []string{"GeoLite2-City.mmdb", "GeoLite2-Country.mmdb"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return ""
}

func TestPrinter_AllExamples(t *testing.T) {
	geo := geoPath()
	if geo == "" {
		t.Log("geo disabled: mmdb not found")
	}

	cases := []struct {
		cmd  string
		file string
	}{
		{"players", "players.txt"},
		{"admins", "admins.txt"},
		{"bans", "bans.txt"},
		{"say", "message.txt"}, // fallback generic
	}

	formats := []struct {
		name   string
		format Format
	}{
		{"table", FormatTable},
		{"json", FormatJSON},
		{"markdown", FormatMarkdown},
		{"html", FormatHTML},
		{"plain", FormatPlain},
	}

	for _, c := range cases {
		data := loadData(t, c.file)

		for _, f := range formats {
			t.Run(c.cmd+"_"+f.name, func(t *testing.T) {
				var buf bytes.Buffer
				if err := ParseAndPrintData(&buf, data, c.cmd, geo, f.format); err != nil {
					t.Fatalf("ParseAndPrintData: %v", err)
				}
				t.Logf("\n=== %s / %s ===\n%s", c.cmd, f.name, buf.String())
			})
		}
	}
}
