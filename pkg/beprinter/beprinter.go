package beprinter

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/woozymasta/bercon-cli/pkg/beparser"
)

// parse data for cmd and print as table
func ParseAndPrintData(data []byte, cmd, geoDB string, json bool) {
	// print if not geo enabled
	if geoDB == "" {
		if json {
			printJSON(beparser.Parse(data, cmd))
		} else {
			printPlain(data)
		}

		return
	}

	// parse data with geo
	parsedData, err := beparser.ParseWithGeoDB(data, cmd, geoDB)
	if err != nil {
		log.Fatalf("Parse response: %v", err)
	}

	// print json response
	if json {
		printJSON(parsedData)
		return
	}

	// print table response with geo
	if geoDB != "" {
		switch pData := parsedData.(type) {
		case *beparser.Players:
			PrintPlayers(*pData)
		case *beparser.Bans:
			PrintBans(*pData)
		case *beparser.Admins:
			PrintAdmins(*pData)

		default:
			printPlain(data)
		}
	}
}

func printPlain(data []byte) {
	if len(data) == 0 {
		fmt.Println("OK")
		return
	}

	fmt.Println(string(data))
}

// parse and print data as json
func printJSON(data any) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal("error converting data to JSON")
	}

	fmt.Println(string(jsonData))
}
