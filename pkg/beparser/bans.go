package beparser

import (
	"strconv"
	"strings"
)

// Bans represents a structure for all types of bans.
type Bans struct {
	GUIDBans BansGUID `json:"guid_bans"`
	IPBans   BansIP   `json:"ip_bans"`
}

// GUIDBan represents a GUID ban entry.
type BanGUID struct {
	ID          int    `json:"id"`
	GUID        string `json:"guid"`
	MinutesLeft int    `json:"minutes"`
	Reason      string `json:"reason"`
	Valid       bool   `json:"valid"`
}
type BansGUID []BanGUID

// IPBan represents an IP ban entry.
type BanIP struct {
	ID          int    `json:"id"`
	IP          string `json:"ip"`
	MinutesLeft int    `json:"minutes"`
	Reason      string `json:"reason"`
	Valid       bool   `json:"valid"`
	Country     string `json:"country,omitempty"`
}
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

func NewBans() *Bans {
	return &Bans{}
}

// ParseBans parses the ban section of the input.
func (b *Bans) Parse(data []byte) {
	lines := strings.Split(string(data), "\n")

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

func NewBansGUID() *BansGUID {
	return &BansGUID{}
}

func (b *BansGUID) Parse(lines []string) {
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

func NewBansIP() *BansIP {
	return &BansIP{}
}

func (b *BansIP) Parse(lines []string, guidCount int) {
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
