package beparser

import (
	"strconv"
	"strings"
)

// Admin represents a connected RCon administrator parsed from the
// "Connected RCon admins:" section. Geolocation fields are optional and
// are filled by SetGeo/SetCountryCode if a GeoIP database is provided.
type Admin struct {
	IP        string  `json:"ip"`
	Country   string  `json:"country,omitempty"`
	City      string  `json:"city,omitempty"`
	Latitude  float64 `json:"lat,omitempty"`
	Longitude float64 `json:"lon,omitempty"`
	Port      uint16  `json:"port"`
	ID        byte    `json:"id"`
}

// Admins is a slice of Admin.
type Admins []Admin

const (
	adminsColID = iota
	adminsColIP
	adminsColsCount

	adminsHeaderSize  = 2
	adminsStartString = "Connected RCon admins:"
)

// NewAdmins returns an empty Admins slice.
func NewAdmins() *Admins {
	return &Admins{}
}

// Parse fills the Admins slice from the plaintext BattlEye response
// of the "admins" command.
func (a *Admins) Parse(data []byte) {
	lines := strings.Split(string(data), "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if line == "" {
			continue
		}

		if strings.Contains(line, adminsStartString) {
			i += adminsHeaderSize
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < adminsColsCount {
			continue
		}

		id, err := strconv.Atoi(parts[adminsColID])
		if err != nil {
			id = len(*a)
		}

		ip, port := parseAddress(parts[adminsColIP])

		admin := Admin{
			ID:   byte(id),
			IP:   ip,
			Port: port,
		}

		*a = append(*a, admin)
	}
}
