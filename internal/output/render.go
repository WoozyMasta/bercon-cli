// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/netip"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/oschwald/geoip2-golang/v2"
	"github.com/woozymasta/bercon/parser"
	"github.com/woozymasta/bercon/parser/ef"
	"github.com/woozymasta/bercon/parser/rv"
)

// Format selects a human-readable or machine-readable response representation.
type Format string

const (
	Table    Format = "table" // Table renders parsed rows in a terminal table.
	JSON     Format = "json"  // JSON emits stable JSON values.
	Raw      Format = "raw"   // Raw prints the server response without parsing.
	Markdown Format = "md"    // Markdown renders parsed rows as Markdown.
	HTML     Format = "html"  // HTML renders parsed rows as an HTML table or escaped text.
	CSV      Format = "csv"   // CSV emits records for row responses and a response column for text.
)

// responseFamily identifies the parsed schema used by compatibility renderers.
type responseFamily uint8

const (
	familyUnknown   responseFamily = iota // familyUnknown uses the generic row renderer.
	familyRVPlayers                       // familyRVPlayers uses the established Real Virtuality player layout.
	familyRVAdmins                        // familyRVAdmins uses the established Real Virtuality administrator layout.
	familyRVBans                          // familyRVBans uses the established Real Virtuality ban layout.
	familyEFPlayers                       // familyEFPlayers preserves the Enfusion player columns for empty results.
	familyEFBans                          // familyEFBans preserves the Enfusion ban columns for empty results.
)

// Parsed describes recognition and row validation for one response.
type Parsed struct {
	Value  any            // Value contains the typed response or a generic message value.
	Err    error          // Err reports malformed rows while retaining any partial result.
	Known  bool           // Known reports whether a parser recognized the response schema.
	family responseFamily // family selects compatibility presentation for recognized response formats.
}

// Parse recognizes known response families without game-name switches.
func Parse(data []byte, command string) Parsed {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(command)))
	if len(fields) == 0 {
		return Parsed{Value: map[string]any{"msg": messageLines(data)}}
	}

	switch fields[0] {
	case "players", "#players":
		if rows, err := rv.ParsePlayers(data); !errors.Is(err, parser.ErrUnrecognized) {
			return Parsed{Value: rvPlayers(rows), Err: err, Known: true, family: familyRVPlayers}
		}
		if rows, err := ef.ParsePlayers(data); !errors.Is(err, parser.ErrUnrecognized) {
			return Parsed{Value: efPlayers(rows), Err: err, Known: true, family: familyEFPlayers}
		}

	case "admins":
		if rows, err := rv.ParseAdmins(data); !errors.Is(err, parser.ErrUnrecognized) {
			return Parsed{Value: rvAdmins(rows), Err: err, Known: true, family: familyRVAdmins}
		}

	case "bans":
		// Reforger returns OK when its ban list is empty;
		// some server versions return an empty payload instead.
		// Both are successful empty results.
		response := strings.TrimSpace(string(data))
		if response == "" || strings.EqualFold(response, "OK") {
			return Parsed{Value: []map[string]any{}, Known: true, family: familyEFBans}
		}

		if rows, err := rv.ParseBans(data); !errors.Is(err, parser.ErrUnrecognized) {
			return Parsed{Value: rvBans(rows), Err: err, Known: true, family: familyRVBans}
		}
		if rows, err := ef.ParseBans(data); !errors.Is(err, parser.ErrUnrecognized) {
			return Parsed{Value: efBans(rows), Err: err, Known: true, family: familyEFBans}
		}
	}

	return Parsed{Value: map[string]any{"msg": messageLines(data)}}
}

// Render writes the selected representation and returns parser status separately.
func Render(w io.Writer, data []byte, command string, format Format, geo *geoip2.Reader) (Parsed, error) {
	return RenderLocalized(w, data, command, format, geo, nil)
}

// RenderLocalized renders a response with localized headings and user-facing output errors.
func RenderLocalized(
	w io.Writer,
	data []byte,
	command string,
	format Format,
	geo *geoip2.Reader,
	localize func(key, fallback string) string,
) (Parsed, error) {
	parsed := Parse(data, command)
	if geo != nil && parsed.Known {
		enrich(parsed.Value, geo)
	}

	if format == Raw {
		return parsed, WriteRaw(w, data)
	}
	if !parsed.Known && format != JSON {
		return parsed, renderText(w, data, format)
	}
	if parsed.Err != nil && !errors.Is(parsed.Err, parser.ErrUnrecognized) {
		return parsed, WriteRaw(w, data)
	}

	switch format {
	case JSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return parsed, encoder.Encode(parsed.Value)

	case CSV:
		return parsed, writeCSV(w, parsed.Value)

	case Table, Markdown, HTML:
		return parsed, renderRows(w, parsed, format, geo != nil, localize)

	default:
		message := outputText(localize, "output.error.format", "Unsupported output format %q")
		return parsed, fmt.Errorf(message, format)
	}
}

// WriteRaw prints empty responses as OK and appends a newline when needed.
func WriteRaw(w io.Writer, data []byte) error {
	if len(data) == 0 {
		_, err := fmt.Fprintln(w, "OK")
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if data[len(data)-1] != '\n' {
		_, err := io.WriteString(w, "\n")
		return err
	}

	return nil
}

// rvPlayers keeps the original CLI JSON field names for Real Virtuality rows.
func rvPlayers(rows []rv.Player) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		ip, port := endpoint(row.Address, row.AddressText)
		out = append(out, map[string]any{
			"id":    max(row.ID, 0),
			"ip":    ip,
			"port":  port,
			"ping":  row.Ping,
			"guid":  row.GUID,
			"name":  row.Name,
			"valid": row.Verified,
			"lobby": row.Lobby,
		})
	}

	return out
}

// rvAdmins keeps the original CLI JSON field names for administrator rows.
func rvAdmins(rows []rv.Admin) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		ip, port := endpoint(row.Address, row.AddressText)
		out = append(out, map[string]any{
			"id":   max(row.ID, 0),
			"ip":   ip,
			"port": port,
		})
	}

	return out
}

// rvBans keeps the original CLI JSON object shape for both ban sections.
func rvBans(bans rv.Bans) map[string]any {
	guid := make([]map[string]any, 0, len(bans.GUID))
	for _, row := range bans.GUID {
		guid = append(guid, map[string]any{
			"guid":    row.Subject,
			"reason":  row.Reason,
			"id":      row.ID,
			"minutes": row.Minutes,
			"valid":   row.SubjectValid && row.IDValid && row.MinutesValid,
		})
	}

	ip := make([]map[string]any, 0, len(bans.IP))
	for _, row := range bans.IP {
		ip = append(ip, map[string]any{
			"ip":      row.Subject,
			"reason":  row.Reason,
			"id":      row.ID,
			"minutes": row.Minutes,
			"valid":   row.SubjectValid && row.IDValid && row.MinutesValid,
		})
	}

	return map[string]any{"guid_bans": guid, "ip_bans": ip}
}

// efPlayers exposes Enfusion fields without translating them into RV semantics.
func efPlayers(rows []ef.Player) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":                row.ID,
			"identity_id":       row.IdentityID,
			"identity_id_valid": row.IdentityIDValid,
			"name":              row.Name,
		})
	}

	return out
}

// efBans exposes Enfusion identity and display-name fields.
func efBans(rows []ef.Ban) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"identity_id":       row.IdentityID,
			"banned_name":       row.BannedName,
			"identity_id_valid": row.IdentityIDValid,
		})
	}

	return out
}

// endpoint returns a parsed endpoint or the original malformed value.
func endpoint(address netip.AddrPort, original string) (string, uint16) {
	if address.IsValid() {
		return address.Addr().String(), address.Port()
	}

	return original, 0
}

// enrich adds GeoIP metadata only to Real Virtuality records with IP fields.
func enrich(value any, database *geoip2.Reader) {
	switch rows := value.(type) {
	case []map[string]any:
		for _, row := range rows {
			enrichRecord(row, database)
		}

	case map[string]any:
		if bans, ok := rows["ip_bans"].([]map[string]any); ok {
			for _, row := range bans {
				enrichRecord(row, database)
			}
		}
	}
}

// enrichRecord adds GeoIP data and preserves the legacy values for unknown locations.
func enrichRecord(row map[string]any, database *geoip2.Reader) {
	ipText, ok := row["ip"].(string)
	if !ok {
		return
	}

	ip, err := netip.ParseAddr(ipText)
	if err != nil {
		return
	}

	row["country"] = "XX"

	record, err := database.City(ip)
	if err == nil && record != nil {
		if record.Country.ISOCode != "" {
			row["country"] = record.Country.ISOCode
		}
		if record.City.Names.English != "" {
			row["city"] = record.City.Names.English
		}
		if latitude := record.Location.Latitude; latitude != nil && *latitude != 0 {
			row["lat"] = *latitude
		}
		if longitude := record.Location.Longitude; longitude != nil && *longitude != 0 {
			row["lon"] = *longitude
		}

		return
	}

	country, err := database.Country(ip)
	if err == nil && country != nil && country.Country.ISOCode != "" {
		row["country"] = country.Country.ISOCode
	}
}

// messageLines retains the historical generic JSON field while representing empty output.
func messageLines(data []byte) []string {
	if len(data) == 0 {
		return []string{"OK"}
	}

	return strings.Split(string(data), "\n")
}

// renderText escapes HTML and fences Markdown while raw output stays byte-oriented.
func renderText(w io.Writer, data []byte, format Format) error {
	value := string(data)
	switch format {
	case HTML:
		_, err := fmt.Fprintf(w, "<pre>%s</pre>\n", html.EscapeString(value))
		return err

	case Markdown:
		fence := strings.Repeat(string(rune(96)), 3)
		_, err := fmt.Fprintf(w, "%s\n%s\n%s\n", fence, strings.TrimSuffix(value, "\n"), fence)
		return err

	case CSV:
		return writeCSV(w, map[string]any{"response": value})

	default:
		return WriteRaw(w, data)
	}
}

// renderRows prints record arrays as stable tables and other values as text.
func renderRows(
	w io.Writer,
	parsed Parsed,
	format Format,
	withGeo bool,
	localize func(key, fallback string) string,
) error {
	if parsed.family != familyUnknown {
		switch parsed.family {
		case familyRVPlayers, familyRVAdmins, familyRVBans:
			return renderRVRows(w, parsed, format, withGeo, localize)

		case familyEFPlayers, familyEFBans:
			rows, _ := parsed.Value.([]map[string]any)
			if len(rows) == 0 {
				return renderEFEmptyRows(w, parsed, format, localize)
			}

		default:
			message := outputText(localize, "output.error.response_family", "Unsupported response family %d")
			return fmt.Errorf(message, parsed.family)
		}
	}

	encoded, err := json.Marshal(parsed.Value)
	if err != nil {
		return err
	}

	var rows []map[string]any
	if err := json.Unmarshal(encoded, &rows); err != nil {
		return renderText(w, encoded, format)
	}

	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, outputText(localize, "output.empty", "No entries"))
		return err
	}

	keys := recordKeys(rows[0])
	writer := table.NewWriter()
	writer.SetStyle(table.Style{
		Name:    "Compact",
		Box:     table.StyleBoxRounded,
		Color:   table.ColorOptions{Header: text.Colors{text.Bold}},
		Options: table.Options{DrawBorder: true, SeparateColumns: true, SeparateHeader: true},
	})
	header := make(table.Row, len(keys))
	for i, key := range keys {
		header[i] = outputText(localize, "output.column."+key, key)
	}

	writer.AppendHeader(header)
	for _, row := range rows {
		values := make(table.Row, len(keys))
		for i, key := range keys {
			values[i] = row[key]
		}
		writer.AppendRow(values)
	}

	switch format {
	case Markdown:
		_, err = io.WriteString(w, writer.RenderMarkdown()+"\n")

	case HTML:
		_, err = io.WriteString(w, writer.RenderHTML()+"\n")

	default:
		_, err = io.WriteString(w, writer.Render()+"\n")
	}

	return err
}

// renderEFEmptyRows keeps the known Enfusion schema visible when there are no rows.
func renderEFEmptyRows(
	w io.Writer,
	parsed Parsed,
	format Format,
	localize func(key, fallback string) string,
) error {
	headers := []string{"#", "Identity ID", "Identity ID valid", "Name"}
	if parsed.family == familyEFBans {
		headers = []string{"Identity ID", "Banned name", "Identity ID valid"}
	}

	return renderTable(w, nil, headers, "", format, localize)
}

// renderRVRows preserves the historical RV column names, order, and section titles.
func renderRVRows(
	w io.Writer,
	parsed Parsed,
	format Format,
	withGeo bool,
	localize func(key, fallback string) string,
) error {
	var rows []map[string]any
	var headers []string
	titleKey := ""
	titleFallback := ""

	switch parsed.family {
	case familyRVPlayers:
		rows, _ = parsed.Value.([]map[string]any)
		headers = []string{"#", "IP", "Port", "Ping", "GUID", "Name", "Valid", "Lobby"}
		titleKey = "output.title.players"
		titleFallback = "Players on server (%d in total)"

	case familyRVAdmins:
		rows, _ = parsed.Value.([]map[string]any)
		headers = []string{"#", "IP", "Port", "Country"}
		titleKey = "output.title.admins"
		titleFallback = "Connected RCon admins"

	case familyRVBans:
		return renderRVBans(w, parsed, format, withGeo, localize)

	default:
		message := outputText(localize, "output.error.response_family", "Unsupported RV response family %d")
		return fmt.Errorf(message, parsed.family)
	}

	if withGeo {
		switch parsed.family {
		case familyRVPlayers:
			headers = append(headers, "Country", "City", "Lat", "Lon")

		case familyRVAdmins:
			headers = append(headers, "City", "Lat", "Lon")
		}
	}

	title := outputText(localize, titleKey, titleFallback)
	if titleKey == "output.title.players" {
		title = fmt.Sprintf(title, len(rows))
	}

	return renderTable(w, rows, headers, title, format, localize)
}

// renderRVBans renders both independent sections returned by the RV bans command.
func renderRVBans(
	w io.Writer,
	parsed Parsed,
	format Format,
	withGeo bool,
	localize func(key, fallback string) string,
) error {
	value, _ := parsed.Value.(map[string]any)
	guid, _ := value["guid_bans"].([]map[string]any)
	ip, _ := value["ip_bans"].([]map[string]any)
	if len(guid) == 0 && len(ip) == 0 {
		// Keep both recognized ban schemas visible so an empty server list is not silent.
		if err := renderRVBanTable(w, nil, true, withGeo, format, localize); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}

		return renderRVBanTable(w, nil, false, withGeo, format, localize)
	}

	if len(guid) > 0 {
		if err := renderRVBanTable(w, guid, true, withGeo, format, localize); err != nil {
			return err
		}
	}

	if len(ip) > 0 {
		if len(guid) > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}

		return renderRVBanTable(w, ip, false, withGeo, format, localize)
	}

	return nil
}

// renderRVBanTable applies the distinct row schema for one ban section.
func renderRVBanTable(
	w io.Writer,
	rows []map[string]any,
	isGUID bool,
	withGeo bool,
	format Format,
	localize func(key, fallback string) string,
) error {
	header := []string{"#", "GUID", "Minutes left", "Reason"}
	titleKey := "output.title.guid_bans"
	titleFallback := "GUID Bans"
	if !isGUID {
		header = []string{"#", "IP", "Minutes left", "Reason"}
		titleKey = "output.title.ip_bans"
		titleFallback = "IP Bans"

		if withGeo {
			header = append(header, "Country", "City", "Lat", "Lon")
		}
	}

	return renderTable(
		w,
		rows,
		header,
		outputText(localize, titleKey, titleFallback),
		format,
		localize,
	)
}

// renderTable writes rows with a fixed schema so empty and populated outputs align.
func renderTable(
	w io.Writer,
	rows []map[string]any,
	headers []string,
	title string,
	format Format,
	localize func(key, fallback string) string,
) error {
	writer := table.NewWriter()
	writer.SetStyle(table.Style{
		Name:    "CompactLight",
		Box:     table.StyleBoxRounded,
		Color:   table.ColorOptions{Header: text.Colors{text.Bold}},
		Format:  table.FormatOptions{Header: text.FormatTitle},
		Options: table.Options{DrawBorder: true, SeparateColumns: true, SeparateHeader: true},
	})

	writer.SetTitle(title)
	header := make(table.Row, len(headers))
	for index, value := range headers {
		key := outputHeaderKey(value)
		header[index] = outputText(localize, "output.column."+key, value)
	}

	writer.AppendHeader(header)
	for _, row := range rows {
		values := make(table.Row, len(headers))
		for index, key := range headers {
			values[index] = rvTableValue(key, row)
		}
		writer.AppendRow(values)
	}

	return writeTable(w, writer, format)
}

// outputHeaderKey maps presentation labels to stable translation keys.
func outputHeaderKey(header string) string {
	switch header {
	case "#":
		return "id"

	case "Minutes left":
		return "minutes"

	case "Identity ID":
		return "identity_id"

	case "Identity ID valid":
		return "identity_id_valid"

	case "Banned name":
		return "banned_name"

	default:
		return strings.ToLower(header)
	}
}

// outputText applies a translation when supplied and otherwise keeps the fallback text.
func outputText(localize func(key, fallback string) string, key, fallback string) string {
	if localize == nil {
		return fallback
	}

	return localize(key, fallback)
}

// rvTableValue maps the compatibility headings to the stable JSON field names.
func rvTableValue(header string, row map[string]any) any {
	key := ""
	switch header {
	case "#":
		key = "id"
	case "IP":
		key = "ip"
	case "Port":
		key = "port"
	case "Ping":
		key = "ping"
	case "GUID":
		key = "guid"
	case "Name":
		key = "name"
	case "Valid":
		key = "valid"
	case "Lobby":
		key = "lobby"
	case "Country":
		key = "country"
	case "City":
		key = "city"
	case "Lat":
		key = "lat"
	case "Lon":
		key = "lon"
	case "Reason":
		key = "reason"
	case "Minutes left":
		key = "minutes"
	}

	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}

	if header == "Minutes left" {
		minutes, ok := value.(int)
		if ok && minutes < 0 {
			return "perm"
		}
	}

	if header == "Lat" || header == "Lon" {
		coordinate, ok := value.(float64)
		if ok {
			if coordinate == 0 {
				return ""
			}

			return fmt.Sprintf("%.5f", coordinate)
		}
	}

	return value
}

// writeTable selects the requested textual table renderer.
func writeTable(w io.Writer, writer table.Writer, format Format) error {
	var rendered string
	switch format {
	case Markdown:
		rendered = writer.RenderMarkdown()

	case HTML:
		rendered = writer.RenderHTML()

	default:
		rendered = writer.Render()
	}

	if !strings.HasSuffix(rendered, "\n") {
		rendered += "\n"
	}

	_, err := io.WriteString(w, rendered)
	return err
}

// writeCSV writes nested data as one stable JSON response column.
func writeCSV(w io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var rows []map[string]any
	if err := json.Unmarshal(encoded, &rows); err != nil || len(rows) == 0 {
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"response"}); err != nil {
			return err
		}
		if err := writer.Write([]string{string(encoded)}); err != nil {
			return err
		}

		writer.Flush()
		return writer.Error()
	}

	keys := recordKeys(rows[0])
	writer := csv.NewWriter(w)
	if err := writer.Write(keys); err != nil {
		return err
	}

	for _, row := range rows {
		record := make([]string, len(keys))
		for i, key := range keys {
			record[i] = fmt.Sprint(row[key])
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

// recordKeys returns sorted keys for stable table and CSV output.
func recordKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}
