package beparser

import (
	"strconv"
	"strings"
)

// Player represents a single player.
type Player struct {
	ID      byte   `json:"id"`
	IP      string `json:"ip"`
	Port    uint16 `json:"port"`
	Ping    uint16 `json:"ping"`
	GUID    string `json:"guid"`
	Valid   bool   `json:"valid"`
	Name    string `json:"name"`
	Lobby   bool   `json:"lobby"`
	Country string `json:"country,omitempty"`
}
type Players []Player

const (
	playersColID = iota
	playersColIP
	playersColPing
	playersColGuid
	playersColName
	playersColsCount

	playersHeaderSize  = 2
	playersStartString = "Players on server:"
	playersTotal       = "players in total"

	playerOK    = "(OK)"
	playerLobby = " (Lobby)"
	defaultPing = 0
)

func NewPlayers() *Players {
	return &Players{}
}

// ParsePlayers parses the player section of the input.
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

		id, err := strconv.Atoi(parts[playersColID])
		if err != nil {
			id = len(*p)
		}

		ip, port := parseAddress(parts[playersColIP])

		ping, err := strconv.Atoi(parts[playersColPing])
		if err != nil {
			ping = defaultPing
		}

		var guid string
		var valid bool
		if len(parts[playersColGuid]) >= hashBytesGUID {
			guid = strings.TrimSpace(parts[playersColGuid][:hashBytesGUID])
			valid = parts[playersColGuid][hashBytesGUID:] == playerOK
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
			Ping:  uint16(ping),
			GUID:  guid,
			Valid: valid,
			Name:  strings.TrimSpace(name),
			Lobby: inLobby,
		}

		*p = append(*p, player)
	}
}
