/*
Package bercon provides a BattlEye RCON connection handling (send commands, receive responses, keep alive).

Usage:

	package main

	import (
		"fmt"
		"time"

		"github.com/woozymasta/bercon-cli/pkg/bercon"
	)

	func main() {
		// Open RCON connection
		conn, err := bercon.Open("127.0.0.1:2302", "MyRconPassword")
		if err != nil {
			fmt.Println("Failed to open connection:", err)
			return
		}
		defer conn.Close()

		// Start keepalive routine
		conn.StartKeepAlive()

		// Listen for loginPacket or messagePacket in a separate goroutine
		// (events will appear in conn.Messages channel)
		go func() {
			// This loop will receive packets such as messages from the server
			for event := range conn.Messages {
				// Handle PacketEvent
				// Example: log or print to console
				fmt.Printf("[EVENT] seq=%d time=%s data=%s\n",
					event.Seq, event.Time.Format(time.Stamp), string(event.Data))
			}
		}()

		// Now we can send a command and wait for a response
		resp, err := conn.Send("players")
		if err != nil {
			fmt.Println("Failed to send command:", err)
			return
		}
		// Print out the response from 'players' command
		fmt.Printf("Command 'players' response: %s\n", string(resp))

		// ... do more stuff, send more commands, etc.

		// Close connection when done
		fmt.Println("Closing connection...")
	}
*/
package bercon
