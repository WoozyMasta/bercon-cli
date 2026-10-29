// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli
package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/oschwald/geoip2-golang/v2"
	"github.com/woozymasta/bercon/parser"
)

// TestParseSelectsRVAndEF checks the engine-format adapters and stable fields.
func TestParseSelectsRVAndEF(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		field string
		want  string
	}{
		{
			name: "rv",
			data: "Players on server:\n Id IP Address Ping GUID Name\n" +
				"1 127.0.0.1:2302 42 0123456789abcdef0123456789abcdef(OK) Example Player\n" +
				"(1 players in total)\n",
			field: "guid",
			want:  "0123456789abcdef0123456789abcdef",
		},
		{
			name: "ef",
			data: "Players on server: [Player#] ; [Player UID] ; [Player Name]\n" +
				"1 ; 8e0a172c-9b12-4f40-85c1-7218dd42a612 ; Example Player\n",
			field: "identity_id",
			want:  "8e0a172c-9b12-4f40-85c1-7218dd42a612",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			result, err := Render(&out, []byte(test.data), "players", JSON, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Known || result.Err != nil {
				t.Fatalf("parse result = %#v", result)
			}
			var rows []map[string]any
			if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
				t.Fatalf("decode rendered response: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("rendered rows = %#v", rows)
			}
			if got := rows[0][test.field]; got != test.want {
				t.Fatalf("rendered %s = %#v, want %q", test.field, got, test.want)
			}
		})
	}
}

// TestGeoIPRequiresIPAddress keeps rows without an IP out of GeoIP enrichment.
func TestGeoIPRequiresIPAddress(t *testing.T) {
	data := "Players on server: [Player#] ; [Player UID] ; [Player Name]\n" +
		"1 ; 8e0a172c-9b12-4f40-85c1-7218dd42a612 ; Example Player\n"
	var database geoip2.Reader
	var output bytes.Buffer

	if _, err := Render(&output, []byte(data), "players", Table, &database); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(output.String(), "Country") || strings.Contains(output.String(), "XX") {
		t.Fatalf("table contains GeoIP data without an IP address: %s", output.String())
	}
}

// TestRVTableRendersUnknownGeoWithLegacyFallback keeps missing GeoIP values readable.
func TestRVTableRendersUnknownGeoWithLegacyFallback(t *testing.T) {
	parsed := Parsed{
		Known:  true,
		family: familyRVPlayers,
		Value: []map[string]any{{
			"id": 0, "ip": "127.0.0.1", "port": 2302, "ping": 42,
			"guid": "0123456789abcdef0123456789abcdef", "name": "Example",
			"valid": true, "lobby": false, "country": "XX", "city": "",
			"lat": nil, "lon": nil,
		}},
	}
	var out bytes.Buffer
	if err := renderRows(&out, parsed, Table, true, nil); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "XX") {
		t.Fatalf("unknown country fallback missing from table: %s", out.String())
	}
	if strings.Contains(out.String(), "<nil>") {
		t.Fatalf("nil GeoIP coordinates leaked into table: %s", out.String())
	}
}

// TestRVBanTableRendersGUIDAndIPSections keeps the complete mixed ban response visible.
func TestRVBanTableRendersGUIDAndIPSections(t *testing.T) {
	parsed := Parsed{
		Known:  true,
		family: familyRVBans,
		Value: map[string]any{
			"guid_bans": []map[string]any{{
				"id": 0, "guid": "0123456789abcdef0123456789abcdef",
				"minutes": -1, "reason": "GUID test ban",
			}},
			"ip_bans": []map[string]any{{
				"id": 1, "ip": "192.0.2.1", "minutes": 15, "reason": "IP test ban",
			}},
		},
	}
	var out bytes.Buffer
	if err := renderRows(&out, parsed, Table, false, nil); err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{"GUID test ban", "IP test ban", "192.0.2.1"} {
		if !strings.Contains(out.String(), value) {
			t.Errorf("rendered bans omitted %q: %s", value, out.String())
		}
	}
}

// TestEmptyRVBansKeepBothSchemasVisible prevents recognized empty ban lists from rendering as blank output.
func TestEmptyRVBansKeepBothSchemasVisible(t *testing.T) {
	data := []byte("GUID Bans:\n[#] [GUID] [Minutes left] [Reason]\n" +
		"----------------------------------------\n\n" +
		"IP Bans:\n[#] [IP Address] [Minutes left] [Reason]\n" +
		"----------------------------------------------\n")
	var out bytes.Buffer
	parsed, err := Render(&out, data, "bans", Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Known || parsed.Err != nil {
		t.Fatalf("empty RV bans parse = %#v", parsed)
	}

	for _, header := range []string{"GUID Bans", "IP Bans", "GUID", "IP", "Minutes Left", "Reason"} {
		if !strings.Contains(out.String(), header) {
			t.Errorf("empty RV bans table is missing %q: %s", header, out.String())
		}
	}
}

// TestMalformedRowsUseRawFallback keeps partial parser results visible to operators.
func TestMalformedRowsUseRawFallback(t *testing.T) {
	data := []byte("Players on server:\n Id IP Address Ping GUID Name\n" +
		"1 invalid-address nope broken Example Player\n")
	var out bytes.Buffer
	parsed, err := Render(&out, data, "players", JSON, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Known || !errors.Is(parsed.Err, parser.ErrMalformedRows) {
		t.Fatalf("parse status = %#v", parsed)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("fallback output = %q, want original payload", out.String())
	}
}

// TestRawHTMLIsEscaped prevents server payloads from becoming active markup.
func TestRawHTMLIsEscaped(t *testing.T) {
	var out bytes.Buffer
	_, err := Render(&out, []byte("<script>alert(1)</script>"), "say", HTML, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<script>") || !strings.Contains(out.String(), "&lt;script&gt;") {
		t.Fatalf("HTML output is not escaped: %q", out.String())
	}
}

// TestEFEmptyBansIsStructured verifies textual and zero-byte empty-list responses remain typed.
func TestEFEmptyBansIsStructured(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "empty marker", data: []byte("Server has no bans to list.")},
		{name: "empty payload"},
		{name: "success response", data: []byte("OK")},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			parsed, err := Render(&out, test.data, "bans", JSON, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !parsed.Known || parsed.Err != nil {
				t.Fatalf("empty EF bans parse = %#v", parsed)
			}

			var rows []map[string]any
			if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
				t.Fatalf("decode empty EF bans JSON: %v", err)
			}
			if len(rows) != 0 {
				t.Fatalf("empty EF bans JSON = %q", out.String())
			}
		})
	}
}

// TestEmptyEFListsKeepTheirColumns preserves useful table schemas for empty server lists.
func TestEmptyEFListsKeepTheirColumns(t *testing.T) {
	tests := []struct {
		name    string
		command string
		data    string
		columns []string
	}{
		{
			name:    "players",
			command: "players",
			data:    "Players on server: [Player#] ; [Player UID] ; [Player Name]\n",
			columns: []string{"Identity ID", "Name"},
		},
		{
			name:    "bans",
			command: "bans",
			data:    "OK",
			columns: []string{"Identity ID", "Banned name"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			parsed, err := Render(&out, []byte(test.data), test.command, Table, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !parsed.Known || parsed.Err != nil {
				t.Fatalf("empty Enfusion list parse = %#v", parsed)
			}
			for _, column := range test.columns {
				if !strings.Contains(strings.ToLower(out.String()), strings.ToLower(column)) {
					t.Errorf("empty %s table is missing %q: %s", test.command, column, out.String())
				}
			}
			if strings.Contains(out.String(), "No entries") {
				t.Fatalf("empty %s table lost its schema: %s", test.command, out.String())
			}
		})
	}
}

// TestCSVPreservesPlayerFields keeps parsed values usable by downstream tools.
func TestCSVPreservesPlayerFields(t *testing.T) {
	data := []byte("Players on server:\n Id IP Address Ping GUID Name\n" +
		"1 127.0.0.1:2302 42 0123456789abcdef0123456789abcdef(OK) Example Player\n" +
		"(1 players in total)\n")
	var out bytes.Buffer
	if _, err := Render(&out, data, "players", CSV, nil); err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(&out).ReadAll()
	if err != nil {
		t.Fatalf("decode CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("CSV record count = %d, want header and one player", len(records))
	}

	columns := make(map[string]int, len(records[0]))
	for index, name := range records[0] {
		columns[name] = index
	}
	for name, want := range map[string]string{
		"guid": "0123456789abcdef0123456789abcdef",
		"name": "Example Player",
	} {
		index, ok := columns[name]
		if !ok || records[1][index] != want {
			t.Errorf("CSV %s = %v, want %q", name, records[1], want)
		}
	}
}
