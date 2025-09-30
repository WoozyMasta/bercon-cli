/*
Package bercon implements a BattlEye RCON client: it opens a UDP
session, performs login, sends commands, assembles (multi-part)
responses, keeps the session alive, and exposes incoming server
messages via a channel.

Key features

  - Single-writer event loop: all UDP writes and state mutations are
    serialized in the manager goroutine to avoid races.
  - Multipart assembly: long command responses are reassembled in order
    (with strict page/sequence checks).
  - Keepalive: optional periodic “empty command” pings to keep the
    RCON session alive (BattlEye typically disconnects on long idle).
  - Backpressure & deadlines: Send() waits for a free sequence or fails
    fast with ErrBufferFull / ErrTimeout; reads are deadline-bound too.
  - Typed errors: common protocol/transport problems have stable
    error values (see variables in errors.go).

# Concurrency model

A single Connection owns:
  - A UDP reader loop that parses packets into typed structs.
  - A manager loop that:
    – assigns free sequence numbers,
    – writes packets,
    – assembles multi-part responses,
    – acks message packets,
    – emits PacketEvent values to c.Messages.
  - Send(command) is safe to call from multiple goroutines; responses
    are routed back to the caller.

Limits & sizes

  - MaxCommandBodySize is the BattlEye protocol limit for a *client*
    command body (no multipart from the client side).
  - Buffer size controls the single UDP read buffer (header + body).

# Timeouts

Use SetDeadlineTimeout to cap request/response round trips. Use
SetKeepaliveTimeout(<45s) to keep a session alive (StartKeepAlive to
enable). Use SetMicroSleepTimeout to trade CPU vs latency while waiting
for a free sequence number under load.

Quick start

	package main

	import (
		"fmt"
		"time"

		"github.com/woozymasta/bercon-cli/pkg/bercon"
	)

	func main() {
		conn, err := bercon.Open("127.0.0.1:2302", "MyRconPassword")
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		// Optional tuning
		conn.SetDeadlineTimeout(5)  // seconds
		conn.SetKeepaliveTimeout(10)
		conn.StartKeepAlive()

		// Listen for server messages/login notifications
		go func() {
			for ev := range conn.Messages {
				fmt.Printf("[msg seq=%d at %s] %s\n",
					ev.Seq, ev.Time.Format(time.Stamp), string(ev.Data))
			}
		}()

		// Send command and print response
		resp, err := conn.Send("players")
		if err != nil {
			panic(err)
		}
		fmt.Println(string(resp))
	}

See also

  - Connection for per-connection controls.
  - PacketEvent for streaming server-side messages.
  - Error variables in errors.go for consistent error handling.
*/
package bercon
