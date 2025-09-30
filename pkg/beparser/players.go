package beparser

import (
	"strconv"
	"strings"
)

// Player represents a single player entry parsed from the "Players on server:"
// section. Geolocation fields are optional and are filled by SetGeo/SetCountryCode.
type Player struct {
	IP        string  `json:"ip"`
	GUID      string  `json:"guid"`
	Name      string  `json:"name"`
	Country   string  `json:"country,omitempty"`
	City      string  `json:"city,omitempty"`
	Latitude  float64 `json:"lat,omitempty"`
	Longitude float64 `json:"lon,omitempty"`
	Port      uint16  `json:"port"`
	Ping      uint16  `json:"ping"`
	ID        byte    `json:"id"`
	Valid     bool    `json:"valid"`
	Lobby     bool    `json:"lobby"`
}

// Players is a slice of Player.
type Players []Player

const (
	playersColID = iota
	playersColIP
	playersColPing
	playersColGUID
	playersColName
	playersColsCount

	playersHeaderSize  = 2
	playersStartString = "Players on server:"
	playersTotal       = "players in total"

	playerOK    = "(OK)"
	playerLobby = " (Lobby)"
	defaultPing = 0
)

// NewPlayers returns an empty Players slice.
func NewPlayers() *Players {
	return &Players{}
}

// Parse fills the Players slice from the plaintext BattlEye response
// of the "players" command.
func (p *Players) Parse(data []byte) {
	lines := strings.Split(string(data), "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.Contains(line, playersTotal) {
			break
		}

		if line == "" {
			continue
		}

		if strings.Contains(line, playersStartString) {
			i += playersHeaderSize
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < playersColsCount {
			continue
		}

		id, err := strconv.ParseUint(parts[playersColID], 10, 8)
		if err != nil {
			if len(*p) > 255 {
				id = 255
			} else {
				id = uint64(len(*p))
			}
		}

		ip, port := parseAddress(parts[playersColIP])

		ping, err := strconv.ParseUint(parts[playersColPing], 10, 16)
		if err != nil {
			ping = defaultPing
		}

		var guid string
		var valid bool
		if len(parts[playersColGUID]) >= hashBytesGUID {
			guid = strings.TrimSpace(parts[playersColGUID][:hashBytesGUID])
			valid = parts[playersColGUID][hashBytesGUID:] == playerOK
		} else {
			guid = defaultInvalidGUID
		}

		name := strings.Join(parts[playersColName:], " ")
		var inLobby bool
		if len(name) > len(playerLobby) {
			inLobby = name[len(name)-len(playerLobby):] == playerLobby
			if inLobby {
				name = name[:len(name)-len(playerLobby)]
			}
		}

		player := Player{
			ID:    byte(id),
			IP:    ip,
			Port:  port,
			Ping:  uint16(ping), // #nosec G115
			GUID:  guid,
			Valid: valid,
			Name:  strings.TrimSpace(name),
			Lobby: inLobby,
		}

		*p = append(*p, player)
	}
}
