package beprinter

import (
	"fmt"
	"strconv"

	"github.com/woozymasta/bercon-cli/pkg/beparser"
)

// print "Players" response table with geo data
func PrintPlayers(players beparser.Players) {
	fmt.Println("Players on server:")
	printer := NewTablePrinter([]string{"[#]", "[IP Address]:[Port]", "[Ping]", "[GUID]", "[Name]", "[Country]"})

	for _, player := range players {
		printer.AddRow([]string{
			strconv.Itoa(int(player.ID)),
			fmt.Sprintf("%s:%d", player.IP, player.Port),
			strconv.Itoa(int(player.Ping)),
			player.GUID,
			player.Name,
			player.Country,
		})
	}

	printer.Print()
	fmt.Printf("(%d players in total)\n", len(players))
}

// print "Admins" response table with geo data
func PrintAdmins(admins beparser.Admins) {
	fmt.Println("Connected RCon admins:")
	printer := NewTablePrinter([]string{"[#]", "[IP Address]:[Port]", "[Country]"})

	for _, admin := range admins {
		printer.AddRow([]string{
			strconv.Itoa(int(admin.ID)),
			fmt.Sprintf("%s:%d", admin.IP, admin.Port),
			admin.Country,
		})
	}
	printer.Print()
}

// print "Bans" response table with geo data
func PrintBans(bans beparser.Bans) {
	PrintBansGUID(bans.GUIDBans)
	fmt.Println()
	PrintBansIP(bans.IPBans)
}

// print "GUID Bans" part of "Bans" table
func PrintBansGUID(bans beparser.BansGUID) {
	fmt.Println("GUID Bans:")

	printer := NewTablePrinter([]string{"[#]", "[GUID]", "[Minutes left]", "[Reason]"})
	for _, ban := range bans {
		printer.AddRow([]string{
			strconv.Itoa(ban.ID),
			ban.GUID,
			minutesLeft(ban.MinutesLeft),
			ban.Reason,
		})
	}

	printer.Print()
}

// print "IP Bans" response part of "Bans" table with geo data
func PrintBansIP(bans beparser.BansIP) {
	fmt.Println("IP Bans:")

	printer := NewTablePrinter([]string{"[#]", "[IP Address]", "[Minutes left]", "[Reason]", "[Country]"})
	for _, ban := range bans {
		printer.AddRow([]string{
			strconv.Itoa(ban.ID),
			ban.IP,
			minutesLeft(ban.MinutesLeft),
			ban.Reason,
			ban.Country,
		})
	}

	printer.Print()
}

// get Minutes left string
func minutesLeft(minutes int) string {
	if minutes < 0 {
		return "perm"
	}

	return strconv.Itoa(minutes)
}
