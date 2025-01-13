package beparser

import (
	"fmt"

	"github.com/oschwald/geoip2-golang"
)

const (
	defaultInvalidGUID = "00000000000000000000000000000000"
	hashBytesGUID      = 32
)

// Return struct by passed command:
//   - `players` -> []Player
//   - `admins`  -> []Admin
//   - `bans`    -> []Bans
//   - others    -> []string
func Parse(data []byte, cmd string) any {
	switch cmd {
	case "players":
		players := NewPlayers()
		players.Parse(data)
		return players
	case "bans":
		bans := NewBans()
		bans.Parse(data)
		return bans
	case "admins":
		admins := NewAdmins()
		admins.Parse(data)
		return admins
	default:
		message := NewMessage()
		message.Parse(data)
		return message
	}
}

// Return struct by passed command like Parse with country from IP fields by geoip2.Reader
func ParseWithGeo(data []byte, cmd string, geoReader *geoip2.Reader) (any, error) {
	// fallback if geo reader is nil
	if geoReader == (*geoip2.Reader)(nil) {
		return Parse(data, cmd), fmt.Errorf("")
	}

	switch cmd {
	case "players":
		players := NewPlayers()
		players.Parse(data)
		players.SetCountryCode(geoReader)
		return players, nil
	case "bans":
		bans := NewBans()
		bans.Parse(data)
		bans.SetCountryCode(geoReader)
		return bans, nil
	case "admins":
		admins := NewAdmins()
		admins.Parse(data)
		admins.SetCountryCode(geoReader)
		return admins, nil
	default:
		message := NewMessage()
		message.Parse(data)
		return message, nil
	}
}

// Return struct by passed command like Parse with country from IP fields by geoDB file path
func ParseWithGeoDB(data []byte, cmd string, geoDB string) (any, error) {
	reader, err := geoip2.Open(geoDB)
	if err != nil {
		return nil, err
	}

	defer func(reader *geoip2.Reader) {
		if err := reader.Close(); err != nil {
			panic("Cant close GeoDB")
		}
	}(reader)

	return ParseWithGeo(data, cmd, reader)
}
