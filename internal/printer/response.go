package printer

import (
	"fmt"
	"strconv"

	"github.com/woozymasta/bercon-cli/pkg/beparser"
)

// print "Players" response table with geo data
func PrintPlayers(players beparser.Players) error {
	fmt.Println("Players on server:")
	table := NewTablePrinter([]string{"[#]", "[IP Address]:[Port]", "[Ping]", "[GUID]", "[Name]", "[Country]"})

	for _, player := range players {
		if err := table.AddRow([]string{
			strconv.Itoa(int(player.ID)),
			fmt.Sprintf("%s:%d", player.IP, player.Port),
			strconv.Itoa(int(player.Ping)),
			player.GUID,
			player.Name,
			player.Country,
		}); err != nil {
			return err
		}
	}

	table.Print()
	fmt.Printf("(%d players in total)\n\n", len(players))

	return nil
}

// print "Admins" response table with geo data
func PrintAdmins(admins beparser.Admins) error {
	fmt.Println("Connected RCon admins:")
	table := NewTablePrinter([]string{"[#]", "[IP Address]:[Port]", "[Country]"})

	for _, admin := range admins {
		if err := table.AddRow([]string{
			strconv.Itoa(int(admin.ID)),
			fmt.Sprintf("%s:%d", admin.IP, admin.Port),
			admin.Country,
		}); err != nil {
			return err
		}
	}
	table.Print()
	fmt.Println()

	return nil
}

// print "Bans" response table with geo data
func PrintBans(bans beparser.Bans) error {
	if err := PrintBansGUID(bans.GUIDBans); err != nil {
		return err
	}
	if err := PrintBansIP(bans.IPBans); err != nil {
		return err
	}

	return nil
}

// print "GUID Bans" part of "Bans" table
func PrintBansGUID(bans beparser.BansGUID) error {
	fmt.Println("GUID Bans:")

	table := NewTablePrinter([]string{"[#]", "[GUID]", "[Minutes left]", "[Reason]"})
	for _, ban := range bans {
		if err := table.AddRow([]string{
			strconv.Itoa(ban.ID),
			ban.GUID,
			minutesLeft(ban.MinutesLeft),
			ban.Reason,
		}); err != nil {
			return err
		}
	}

	table.Print()
	fmt.Println()

	return nil
}

// print "IP Bans" response part of "Bans" table with geo data
func PrintBansIP(bans beparser.BansIP) error {
	fmt.Println("IP Bans:")

	table := NewTablePrinter([]string{"[#]", "[IP Address]", "[Minutes left]", "[Reason]", "[Country]"})
	for _, ban := range bans {
		if err := table.AddRow([]string{
			strconv.Itoa(ban.ID),
			ban.IP,
			minutesLeft(ban.MinutesLeft),
			ban.Reason,
			ban.Country,
		}); err != nil {
			return err
		}
	}

	table.Print()
	fmt.Println()

	return nil
}

// get Minutes left string
func minutesLeft(minutes int) string {
	if minutes < 0 {
		return "perm"
	}

	return strconv.Itoa(minutes)
}
