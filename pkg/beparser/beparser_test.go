package beparser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/oschwald/geoip2-golang"
)

// loadTestData reads test data from a specified text file.
func loadTestData(filename string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join("test_data", filename))
	if err != nil {
		return nil, err
	}

	return data, nil
}

// printJSON prints the given data in a JSON format.
func printJSON(title string, data interface{}) {
	if os.Getenv("PRINT_JSON") == "0" {
		return
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error converting %s to JSON: %v", title, err)
		return
	}

	fmt.Printf("%s:\n%s\n", title, string(jsonData))
}

// loadGeoOrSkip tries to open GeoIP DB (path from BERCON_GEO_DB env or default files).
// Skips the test if DB not found.
func loadGeoOrSkip(t *testing.T) *geoip2.Reader {
	t.Helper()

	// env has priority
	if p := os.Getenv("BERCON_GEO_DB"); p != "" {
		db, err := geoip2.Open(p)
		if err == nil {
			t.Cleanup(func() { _ = db.Close() })
			return db
		}
		t.Skipf("skip: failed to open %s (%v)", p, err)
	}

	// common candidates
	for _, c := range []string{"GeoLite2-City.mmdb", "GeoLite2-Country.mmdb"} {
		if _, err := os.Stat(c); err == nil {
			db, err := geoip2.Open(c)
			if err == nil {
				t.Cleanup(func() { _ = db.Close() })
				return db
			}
			t.Skipf("skip: failed to open %s (%v)", c, err)
		}
	}

	t.Skip("skip: no GeoLite2 mmdb found (set BERCON_GEO_DB to override)")
	return nil
}

func TestPlayersParse_PartialFields(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Player
	}{
		{
			name: "ok+lobby",
			line: "0  127.0.0.1:4444      1      48032258807176771690632755883357(OK) Player (Lobby)",
			want: Player{
				ID: 0, IP: "127.0.0.1", Port: 4444, Ping: 1,
				GUID:  "48032258807176771690632755883357",
				Valid: true, Name: "Player", Lobby: true,
			},
		},
		{
			name: "no-ok",
			line: "1  8.8.8.8:65263       560    48032258807176771690632755883357 Nagibator 3000",
			want: Player{
				ID: 1, IP: "8.8.8.8", Port: 65263, Ping: 560,
				GUID:  "48032258807176771690632755883357",
				Valid: false, Name: "Nagibator 3000", Lobby: false,
			},
		},
		{
			name: "missing-ip-placeholder",
			// use "-" to keep token position; parseAddress("-") => invalid,0
			line: "2  -                 10     48032258807176771690632755883357(OK) Name",
			want: Player{
				ID: 2, IP: "invalid", Port: 0, Ping: 10,
				GUID:  "48032258807176771690632755883357",
				Valid: true, Name: "Name", Lobby: false,
			},
		},
		{
			name: "missing-ping-placeholder",
			// use "-" to keep token position; ParseUint("-") fails => default 0
			line: "3  8.8.8.8:1          -      48032258807176771690632755883357(OK) Name",
			want: Player{
				ID: 3, IP: "8.8.8.8", Port: 1, Ping: 0,
				GUID:  "48032258807176771690632755883357",
				Valid: true, Name: "Name", Lobby: false,
			},
		},
		{
			name: "lobby-without-space",
			line: "4  8.8.8.8:1      10   48032258807176771690632755883357(OK) John(Lobby)",
			want: Player{
				ID: 4, IP: "8.8.8.8", Port: 1, Ping: 10,
				GUID:  "48032258807176771690632755883357",
				Valid: true, Name: "John", Lobby: true,
			},
		},
	}

	wrap := func(line string) []byte {
		return []byte(
			"Players on server:\n" +
				"[#] [IP Address]:[Port] [Ping] [GUID] [Name]\n" +
				"--------------------------------------------------\n" +
				line + "\n" +
				"(1 players in total)\n",
		)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var players Players
			players.Parse(wrap(tc.line))

			if len(players) != 1 {
				t.Fatalf("expected 1 player, got %d", len(players))
			}

			got := players[0]
			if got.ID != tc.want.ID ||
				got.IP != tc.want.IP ||
				got.Port != tc.want.Port ||
				got.Ping != tc.want.Ping ||
				got.GUID != tc.want.GUID ||
				got.Valid != tc.want.Valid ||
				got.Name != tc.want.Name ||
				got.Lobby != tc.want.Lobby {
				t.Fatalf("got=%+v want=%+v", got, tc.want)
			}
		})
	}
}

// TestParseAdmins tests the ParseAdmins function.
func TestParseAdmins(t *testing.T) {
	input, err := loadTestData("admins.txt")
	if err != nil {
		t.Fatalf("Failed to load admins test data: %v", err)
	}

	admins := Admins{}
	admins.Parse(input)

	if len(admins) != 3 {
		t.Errorf("Expected 1 admin, got %d", len(admins))
	}
	if admins[0].ID != 0 {
		t.Errorf("Expected for 1 admin ID 0, but '%d'", admins[0].ID)
	}
	if admins[2].ID != 2 {
		t.Errorf("Expected for 3 admin ID 0, but '%d'", admins[2].ID)
	}
	if admins[0].IP != "127.0.0.1" || admins[0].Port != 62676 {
		t.Errorf("Expected admin IP to be '127.0.0.1:62676', got '%s:%d'", admins[0].IP, admins[0].Port)
	}
	if admins[1].IP != "10.0.0.90" || admins[1].Port != 1 {
		t.Errorf("Expected admin IP to be '10.0.0.90:1', got '%s:%d'", admins[1].IP, admins[1].Port)
	}
	if admins[2].IP != "8.8.8.8" || admins[2].Port != 0 {
		t.Errorf("Expected admin IP to be '8.8.8.8:1', got '%s:%d'", admins[2].IP, admins[2].Port)
	}

	geoDB := loadGeoOrSkip(t)
	admins.SetGeo(geoDB)
	if admins[1].Country != "XX" {
		t.Errorf("Expected for 1 admin 'XX' country but got '%s'", admins[1].Country)
	}

	printJSON("Admins", admins)
}

// TestParseBans tests the ParseBans function.
func TestParseBans(t *testing.T) {
	input, err := loadTestData("bans.txt")
	if err != nil {
		t.Fatalf("Failed to load bans test data: %v", err)
	}

	bans := Bans{}
	bans.Parse(input)

	if len(bans.GUIDBans) != 3 {
		t.Errorf("Expected 3 GUID bans, got %d", len(bans.GUIDBans))
	}
	if bans.GUIDBans[0].GUID != "11111111111122222222222223333333" {
		t.Errorf("Expected first GUID ban to be '11111111111122222222222223333333', got '%s'", bans.GUIDBans[0].GUID)
	}
	if len(bans.IPBans) != 3 {
		t.Errorf("Expected 3 IP ban, got %d", len(bans.IPBans))
	}
	if bans.IPBans[0].IP != "127.0.0.1" {
		t.Errorf("Expected first IP ban to be '127.0.0.1', got '%s'", bans.IPBans[0].IP)
	}

	if !bans.GUIDBans[0].Valid {
		t.Errorf("Expected 1 IP ban to be valid, got '%t'", bans.GUIDBans[0].Valid)
	}
	if bans.GUIDBans[1].Valid {
		t.Errorf("Expected 2 IP ban to be invalid, got '%t'", bans.GUIDBans[1].Valid)
	}
	if !bans.IPBans[0].Valid {
		t.Errorf("Expected 1 IP ban to be valid, got '%t'", bans.IPBans[0].Valid)
	}
	if bans.IPBans[2].Valid {
		t.Errorf("Expected 3 IP ban to be invalid, got '%t'", bans.IPBans[0].Valid)
	}

	geoDB := loadGeoOrSkip(t)
	bans.SetGeo(geoDB)
	if bans.IPBans[1].Country != "US" {
		t.Errorf("Expected for 1 admin 'US' country but got '%s'", bans.IPBans[1].Country)
	}

	printJSON("Bans", bans)
}

func Test_parseAddress(t *testing.T) {
	cases := []struct {
		in   string
		ip   string
		port uint16
	}{
		{"127.0.0.1:1234", "127.0.0.1", 1234},
		{" 8.8.8.8:53 ", "8.8.8.8", 53},
		{"bad:port", "invalid", 0},
		{"10.0.0.1", "10.0.0.1", 0},
		{"nope", "invalid", 0},
	}

	for _, c := range cases {
		ip, port := parseAddress(c.in)
		if ip != c.ip || port != c.port {
			t.Errorf("parseAddress(%q) = %q,%d; want %q,%d",
				c.in, ip, port, c.ip, c.port)
		}
	}
}

func Test_getMinutes(t *testing.T) {
	cases := map[string]int{
		"perm": -1, "-": 0, "0": 0, "15": 15, "x": 0,
	}

	for in, want := range cases {
		if got := getMinutes(in); got != want {
			t.Errorf("getMinutes(%q)=%d; want %d", in, got, want)
		}
	}
}
