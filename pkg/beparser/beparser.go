// Package beparser parses BattlEye RCON command responses ("players",
// "admins", "bans", and generic messages). It also supports enriching
// results with GeoIP (Country/City/coordinates) when a GeoLite2/GeoIP2
// database is provided.
package beparser

import (
	"github.com/oschwald/geoip2-golang"
)

const (
	defaultInvalidGUID = "00000000000000000000000000000000"
	hashBytesGUID      = 32
)

// Parse returns a typed structure for the given command:
//   - "players" -> *Players
//   - "admins"  -> *Admins
//   - "bans"    -> *Bans
//   - any other -> *Messages
func Parse(data []byte, cmd string) any {
	switch cmd {
	case "players":
		x := NewPlayers()
		x.Parse(data)
		return x

	case "bans":
		x := NewBans()
		x.Parse(data)
		return x

	case "admins":
		x := NewAdmins()
		x.Parse(data)
		return x

	default:
		x := NewMessage()
		x.Parse(data)
		return x
	}
}

// ParseWithGeo behaves like Parse and additionally enriches parsed
// entities with geolocation using the provided geoip2.Reader. If reader
// is nil, it falls back to Parse without geo enrichment.
func ParseWithGeo(data []byte, cmd string, geoReader *geoip2.Reader) (any, error) {
	if geoReader == nil {
		return Parse(data, cmd), nil
	}

	switch cmd {
	case "players":
		x := NewPlayers()
		x.Parse(data)
		x.SetGeo(geoReader)
		return x, nil

	case "bans":
		x := NewBans()
		x.Parse(data)
		x.SetGeo(geoReader)
		return x, nil

	case "admins":
		x := NewAdmins()
		x.Parse(data)
		x.SetGeo(geoReader)
		return x, nil

	default:
		m := NewMessage()
		m.Parse(data)
		return m, nil
	}
}

// ParseWithGeoDB opens the given GeoIP2/GeoLite2 database file and calls
// ParseWithGeo. The file handle is closed before the function returns.
func ParseWithGeoDB(data []byte, cmd string, geoDB string) (any, error) {
	reader, err := geoip2.Open(geoDB)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	return ParseWithGeo(data, cmd, reader)
}
