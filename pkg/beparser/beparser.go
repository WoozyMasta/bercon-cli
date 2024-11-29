package beparser

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
