package beparser

import (
	"strconv"
	"strings"

	"github.com/woozymasta/dzid"
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
	*p = (*p)[:0]

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Catch footer
		if isPlayersFooter(line) {
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

		// ID must be the first token, otherwise it's garbage line
		id64, err := strconv.ParseUint(parts[playersColID], 10, 8)
		if err != nil {
			continue
		}

		guid := defaultInvalidGUID
		beOK := false
		guidIdx := -1

		for k := 1; k < len(parts); k++ {
			tok := parts[k]
			ok := false

			if strings.HasSuffix(tok, playerOK) {
				ok = true
				tok = strings.TrimSuffix(tok, playerOK)
			}

			if len(tok) < hashBytesGUID {
				continue
			}

			candidate := dzid.NormalizeBattlEye(tok[:hashBytesGUID])
			if dzid.IsBattlEye(candidate) && candidate != defaultInvalidGUID {
				guidIdx = k
				guid = candidate
				beOK = ok
				break
			}
		}

		ip, port := parseAddress(parts[playersColIP])

		ping, err := strconv.ParseUint(parts[playersColPing], 10, 16)
		if err != nil {
			ping = defaultPing
		}

		var name string
		if guidIdx != -1 && guidIdx+1 < len(parts) {
			name = strings.Join(parts[guidIdx+1:], " ") // Best case
		} else if len(parts) >= playersColName+1 {
			name = strings.Join(parts[playersColName:], " ") // Fallback
		} else {
			name = parts[len(parts)-1] // Last resort
		}
		name = strings.TrimSpace(name)

		inLobby := false
		if strings.HasSuffix(name, playerLobby) {
			inLobby = true
			name = strings.TrimSpace(strings.TrimSuffix(name, playerLobby))
		} else if strings.HasSuffix(name, "(Lobby)") {
			inLobby = true
			name = strings.TrimSpace(strings.TrimSuffix(name, "(Lobby)"))
		}

		player := Player{
			ID:    byte(id64),
			IP:    ip,
			Port:  port,
			Ping:  uint16(ping),
			GUID:  guid,
			Valid: beOK,
			Name:  name,
			Lobby: inLobby,
		}

		*p = append(*p, player)
	}
}

func isPlayersFooter(line string) bool {
	line = strings.TrimSpace(line)

	if len(line) < 3 {
		return false
	}

	if line[0] != '(' || line[len(line)-1] != ')' {
		return false
	}

	if !strings.Contains(line, playersTotal) {
		return false
	}

	j := 1
	for j < len(line) && line[j] >= '0' && line[j] <= '9' {
		j++
	}

	return j > 1 && j < len(line) && line[j] == ' '
}
