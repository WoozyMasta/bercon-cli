package beparser

import (
	"strconv"
	"strings"
)

// Admin represents a connected RCon admin.
type Admin struct {
	IP      string `json:"ip"`
	Country string `json:"country,omitempty"`
	Port    uint16 `json:"port"`
	ID      byte   `json:"id"`
}

// Admins represents a []Admin list.
type Admins []Admin

const (
	adminsColID = iota
	adminsColIP
	adminsColsCount

	adminsHeaderSize  = 2
	adminsStartString = "Connected RCon admins:"
)

// Create new Admins
func NewAdmins() *Admins {
	return &Admins{}
}

// ParseAdmins parses the admin section of the input.
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
