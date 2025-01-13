package tableprinter

import (
	"os"
	"path/filepath"
	"testing"
)

// LoadTestData reads test data from a specified text file.
func LoadTestData(filename string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join("test_data", filename))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// TestParsePlayers tests the ParsePlayers function.
func TestParsePlayers(t *testing.T) {
	input, err := LoadTestData("players.txt")
	if err != nil {
		t.Fatalf("Failed to load players test data: %v", err)
	}

	ParseAndPrintData(input, "players", "GeoLite2-Country.mmdb", true)
}
