package printer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/woozymasta/bercon-cli/pkg/beparser"
)

// ParseAndPrintData parses a BattlEye response for the given command and
// writes it to w in the selected format. If geoDB is provided, enriched
// parsing is used (Country/City/coordinates where available).
func ParseAndPrintData(w io.Writer, data []byte, cmd, geoDB string, format Format) error {
	// no GeoIP enrichment
	if geoDB == "" {
		switch format {
		case FormatJSON:
			return writeJSON(w, beparser.Parse(data, cmd))

		case FormatPlain:
			writePlain(w, data)
			return nil

		default:
			return renderParsed(w, beparser.Parse(data, cmd), false, format)
		}
	}

	// with GeoIP
	parsed, err := beparser.ParseWithGeoDB(data, cmd, geoDB)
	if err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	switch format {
	case FormatJSON:
		return writeJSON(w, parsed)

	case FormatPlain:
		writePlain(w, data)
		return nil

	default:
		return renderParsed(w, parsed, true, format)
	}
}

func writePlain(w io.Writer, data []byte) {
	if len(data) == 0 {
		_, _ = fmt.Fprintln(w, "OK")
		return
	}

	_, _ = fmt.Fprintln(w, string(data))
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(v)
}

// renderParsed prints typed structures as pretty tables (when possible).
// If unknown type is passed, it falls back to plain/JSON depending on format.
func renderParsed(w io.Writer, v any, withGeo bool, format Format) error {
	switch x := v.(type) {
	case *beparser.Players:
		return renderPlayersTable(w, *x, withGeo, format)

	case *beparser.Admins:
		return renderAdminsTable(w, *x, withGeo, format)

	case *beparser.Bans:
		return renderBansTable(w, *x, withGeo, format)

	case *beparser.Messages:
		return renderFreeText(w, x.Msg, format)

	default:
		if b, ok := v.([]byte); ok {
			return renderFreeText(w, []string{string(b)}, format)
		}

		return writeJSON(w, v)
	}
}

func renderFreeText(w io.Writer, lines []string, format Format) error {
	switch format {
	case FormatMarkdown:
		if _, err := fmt.Fprintln(w, "```"); err != nil {
			return err
		}

		for _, ln := range lines {
			if _, err := fmt.Fprintln(w, ln); err != nil {
				return err
			}
		}

		_, err := fmt.Fprintln(w, "```")

		return err

	case FormatHTML:
		if _, err := fmt.Fprintln(w, "<pre>"); err != nil {
			return err
		}

		for _, ln := range lines {
			if _, err := fmt.Fprintln(w, ln); err != nil {
				return err
			}
		}

		_, err := fmt.Fprintln(w, "</pre>")
		return err

	default: // table/plain fallback
		for _, ln := range lines {
			if _, err := fmt.Fprintln(w, ln); err != nil {
				return err
			}
		}

		return nil
	}
}

// DefaultWriter returns stdout; exported for callers that don't have their own writer.
func DefaultWriter() io.Writer {
	return os.Stdout
}
