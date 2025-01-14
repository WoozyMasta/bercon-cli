package printer

import (
	"encoding/json"
	"fmt"

	"github.com/woozymasta/bercon-cli/pkg/beparser"
)

// parse data for cmd and print as table
func ParseAndPrintData(data []byte, cmd, geoDB string, json bool) error {
	// print if not geo enabled
	if geoDB == "" {
		if json {
			return printJSON(beparser.Parse(data, cmd))
		}

		printPlain(data)
		return nil
	}

	// parse data with geo
	parsedData, err := beparser.ParseWithGeoDB(data, cmd, geoDB)
	if err != nil {
		return fmt.Errorf("parse response: %v", err)
	}

	// print json response
	if json {
		return printJSON(parsedData)
	}

	// print table response with geo
	if geoDB != "" {
		switch pData := parsedData.(type) {
		case *beparser.Players:
			return PrintPlayers(*pData)
		case *beparser.Bans:
			return PrintBans(*pData)
		case *beparser.Admins:
			return PrintAdmins(*pData)

		default:
			printPlain(data)
		}
	}

	return nil
}

func printPlain(data []byte) {
	if len(data) == 0 {
		fmt.Println("OK")
		return
	}

	fmt.Println(string(data))
}

// parse and print data as json
func printJSON(data any) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error converting data to JSON: %v", err)
	}

	fmt.Println(string(jsonData))

	return nil
}
