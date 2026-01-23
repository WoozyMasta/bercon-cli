package beparser

import (
	"strconv"
	"strings"
)

// Bans aggregates GUID and IP bans as parsed from the "bans" command output.
type Bans struct {
	GUIDBans BansGUID `json:"guid_bans"`
	IPBans   BansIP   `json:"ip_bans"`
}

// BanGUID represents a single GUID ban entry.
type BanGUID struct {
	GUID        string `json:"guid"`
	Reason      string `json:"reason"`
	ID          int    `json:"id"`
	MinutesLeft int    `json:"minutes"`
	Valid       bool   `json:"valid"`
}

// BansGUID is a slice of BanGUID.
type BansGUID []BanGUID

// BanIP represents a single IP ban entry. Geolocation fields are optional
// and are filled by SetGeo/SetCountryCode if a GeoIP database is provided.
type BanIP struct {
	IP          string  `json:"ip"`
	Reason      string  `json:"reason"`
	Country     string  `json:"country,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"lat,omitempty"`
	Longitude   float64 `json:"lon,omitempty"`
	ID          int     `json:"id"`
	MinutesLeft int     `json:"minutes"`
	Valid       bool    `json:"valid"`
}

// BansIP is a slice of BanIP.
type BansIP []BanIP

const (
	bansColID = iota
	bansColWho
	bansColTime
	bansColReason
	bansColsCount = 3 // Reason column is optional

	bansHeaderSize      = 2
	bansGUIDStartString = "GUID Bans:"
	bansIPStartString   = "IP Bans:"
)

// NewBans returns an empty Bans struct.
func NewBans() *Bans {
	return &Bans{}
}

// Parse populates the Bans struct (GUID and IP sections) from the plaintext
// BattlEye response of the "bans" command.
func (b *Bans) Parse(data []byte) {
	*b = Bans{}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.Contains(line, bansGUIDStartString) {
			guidBan := NewBansGUID()
			guidBan.Parse(lines[i+bansHeaderSize+1:])
			b.GUIDBans = *guidBan
			i += len(b.GUIDBans) + bansHeaderSize
			continue
		}

		if strings.Contains(line, bansIPStartString) {
			ipBan := NewBansIP()
			ipBan.Parse(lines[i+bansHeaderSize+1:], len(b.GUIDBans))
			b.IPBans = *ipBan
			break
		}
	}
}

// NewBansGUID returns an empty BansGUID slice.
func NewBansGUID() *BansGUID {
	return &BansGUID{}
}

// Parse fills the BansGUID slice from the GUID bans section of the response.
func (b *BansGUID) Parse(lines []string) {
	*b = BansGUID{}

	if len(lines) == 0 {
		return
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.Contains(line, bansIPStartString) {
			break
		}

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < bansColsCount {
			continue
		}

		id, err := strconv.Atoi(parts[bansColID])
		if err != nil {
			id = len(*b)
		}

		valid := true
		guid := parts[bansColWho]
		if len(parts[bansColWho]) != hashBytesGUID {
			guid = defaultInvalidGUID
			valid = false
		}

		time := getMinutes(parts[bansColTime])
		if time <= 0 && time != -1 {
			valid = false
		}

		reason := strings.Join(parts[bansColReason:], " ")

		ban := BanGUID{
			ID:          id,
			GUID:        guid,
			MinutesLeft: time,
			Reason:      reason,
			Valid:       valid,
		}

		*b = append(*b, ban)
	}
}

// NewBansIP returns an empty BansIP slice.
func NewBansIP() *BansIP {
	return &BansIP{}
}

// Parse fills the BansIP slice from the IP bans section of the response.
// guidCount is used to keep IDs contiguous across GUID and IP sections.
func (b *BansIP) Parse(lines []string, guidCount int) {
	*b = BansIP{}

	if len(lines) == 0 {
		return
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < bansColsCount {
			continue
		}

		id, err := strconv.Atoi(parts[bansColID])
		if err != nil {
			id = len(*b) + guidCount
		}

		valid := true
		ip := parts[bansColWho]
		if !isValidIPv4(ip) {
			ip = "invalid"
			valid = false
		}

		time := getMinutes(parts[bansColTime])
		if time <= 0 && time != -1 {
			valid = false
		}

		reason := strings.Join(parts[bansColReason:], " ")

		ban := BanIP{
			ID:          id,
			IP:          ip,
			MinutesLeft: time,
			Reason:      reason,
			Valid:       valid,
		}

		*b = append(*b, ban)
	}
}
