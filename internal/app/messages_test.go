package app

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/woozymasta/bercon"
	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon-cli/internal/output"
	"golang.org/x/term"
)

const rvPlayersResponse = "Players on server:\n Id IP Address Ping GUID Name\n" +
	"1 127.0.0.1:2302 42 0123456789abcdef0123456789abcdef(OK) RV Player\n" +
	"(1 players in total)\n"

const efPlayersResponse = "Players on server: [Player#] ; [Player UID] ; [Player Name]\n" +
	"1 ; 8e0a172c-9b12-4f40-85c1-7218dd42a612 ; EF Player\n" +
	"(1 players in total)\n"

// TestCommandUsesAsyncResponse covers an acknowledgement followed by its structured event.
func TestCommandUsesAsyncResponse(t *testing.T) {
	var client *fakeClient
	client = newFakeClient(func(_ context.Context, _ string) ([]byte, error) {
		client.messages <- bercon.Message{Data: []byte("unrelated event")}
		client.messages <- bercon.Message{Data: []byte(efPlayersResponse)}
		return []byte("OK"), nil
	})

	broker := startBroker(t, client)
	var stdout, stderr strings.Builder
	if err := execute(context.Background(), "players", &stdout, &stderr,
		client, broker, testOptions("json"), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "EF Player") {
		t.Fatalf("stdout did not contain async response: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), "EF Player") {
		t.Fatalf("matched response was duplicated as a server message: %s", stderr.String())
	}

	select {
	case message := <-broker.events:
		if string(message.Data) != "unrelated event" {
			t.Fatalf("event = %q, want unrelated event", message.Data)
		}

	case <-time.After(time.Second):
		t.Fatal("unrelated event was not preserved")
	}
}

// TestExecSuppressesUnmatchedEvents keeps login notices out of command output while routing replies.
func TestExecSuppressesUnmatchedEvents(t *testing.T) {
	var client *fakeClient
	client = newFakeClient(func(context.Context, string) ([]byte, error) {
		client.messages <- bercon.Message{Data: []byte("RCon admin logged in")}
		client.messages <- bercon.Message{Data: []byte(efPlayersResponse)}
		return []byte("OK"), nil
	})

	broker := newMessageBroker(client, false)
	go broker.run()
	t.Cleanup(broker.close)

	var stdout, stderr strings.Builder
	if err := execute(context.Background(), "players", &stdout, &stderr,
		client, broker, testOptions("json"), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "EF Player") {
		t.Fatalf("stdout did not contain the structured command response: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), "RCon admin logged in") {
		t.Fatalf("unmatched server event leaked into exec diagnostics: %s", stderr.String())
	}
	if len(broker.events) != 0 {
		t.Fatalf("exec queued %d unmatched server events", len(broker.events))
	}
}

// TestCommandPrefersDirectResponse keeps a matching concurrent event available to consumers.
func TestCommandPrefersDirectResponse(t *testing.T) {
	var client *fakeClient
	client = newFakeClient(func(context.Context, string) ([]byte, error) {
		client.messages <- bercon.Message{Data: []byte(efPlayersResponse)}
		return []byte(rvPlayersResponse), nil
	})

	assertExecuteResponse(t, client, "players", "RV Player", "EF Player")
}

// TestUnknownCommandPreservesEvents ensures arbitrary messages are not treated as command output.
func TestUnknownCommandPreservesEvents(t *testing.T) {
	const event = "server maintenance notice"
	var client *fakeClient
	client = newFakeClient(func(context.Context, string) ([]byte, error) {
		client.messages <- bercon.Message{Data: []byte(event)}
		return []byte("OK"), nil
	})

	broker := startBroker(t, client)
	var stdout, stderr strings.Builder
	if err := execute(context.Background(), "custom-command", &stdout, &stderr,
		client, broker, testOptions("json"), nil); err != nil {
		t.Fatal(err)
	}

	select {
	case message := <-broker.events:
		if string(message.Data) != event {
			t.Fatalf("event = %q, want %q", message.Data, event)
		}

	case <-time.After(time.Second):
		t.Fatal("unknown command consumed or lost an unrelated event")
	}
}

// TestBatchRepeatsCommandsInOrder covers the CLI's command scheduling loop without real delays.
func TestBatchRepeatsCommandsInOrder(t *testing.T) {
	var commands []string
	client := newFakeClient(func(_ context.Context, command string) ([]byte, error) {
		commands = append(commands, command)
		return []byte("OK"), nil
	})
	broker := startBroker(t, client)
	opts := testOptions("raw")
	opts.Exec.Repeat = 3

	status := batch(context.Background(), []string{"first", "second"},
		&strings.Builder{}, &strings.Builder{}, client, broker, opts, nil)
	if status != exitSuccess {
		t.Fatalf("batch() status = %d, want success", status)
	}

	want := []string{"first", "second", "first", "second", "first", "second"}
	if !slices.Equal(commands, want) {
		t.Fatalf("commands = %v, want %v", commands, want)
	}
}

// TestCommandFailurePreservesResponse keeps partial server output when transport also reports failure.
func TestCommandFailurePreservesResponse(t *testing.T) {
	wantErr := errors.New("response deadline exceeded")
	client := newFakeClient(func(context.Context, string) ([]byte, error) {
		return []byte("partial response"), wantErr
	})

	broker := startBroker(t, client)
	var stdout, stderr strings.Builder
	err := execute(context.Background(), "custom-command", &stdout, &stderr,
		client, broker, testOptions("raw"), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("execute() error = %v, want wrapped transport error", err)
	}
	if !strings.Contains(stdout.String(), "partial response") {
		t.Fatalf("partial response was lost: stdout=%q", stdout.String())
	}
}

// TestMalformedAsyncResponseFallsBackToRaw preserves evidence for malformed structured output.
func TestMalformedAsyncResponseFallsBackToRaw(t *testing.T) {
	malformed := "Players on server:\n Id IP Address Ping GUID Name\n" +
		"1 invalid-address nope broken Broken row\n"
	var client *fakeClient
	client = newFakeClient(func(context.Context, string) ([]byte, error) {
		client.messages <- bercon.Message{Data: []byte(malformed)}
		return []byte("OK"), nil
	})

	broker := startBroker(t, client)
	var stdout, stderr strings.Builder
	err := execute(
		context.Background(),
		"players",
		&stdout,
		&stderr,
		client,
		broker,
		testOptions("json"),
		nil,
	)
	wantError := appText(
		"app.error.response_malformed",
		"Structured response was malformed; raw response printed",
	)
	if err == nil || !strings.Contains(err.Error(), wantError) {
		t.Fatalf("execute() error = %v, want malformed response error", err)
	}
	if strings.Contains(err.Error(), "%!(EXTRA") {
		t.Fatalf("execute() error contains a formatting artifact: %v", err)
	}
	warningPrefix := strings.Split(appText(
		"app.warning.response_malformed",
		"Warning: malformed structured response: %v",
	), "%v")[0]
	if !strings.Contains(stdout.String(), "invalid-address") ||
		!strings.Contains(stderr.String(), warningPrefix) {
		t.Fatalf("raw fallback missing; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// TestInteractiveEscapedExitCommands sends escaped local commands to the server.
func TestInteractiveEscapedExitCommands(t *testing.T) {
	var commands []string
	client := newFakeClient(func(_ context.Context, command string) ([]byte, error) {
		commands = append(commands, command)
		return []byte("OK"), nil
	})

	broker := startBroker(t, client)
	var stdout, stderr strings.Builder
	input := newScannerShellReader(strings.NewReader("\\exit\n\\quit\nexit\nversion\n"))
	status := interactive(context.Background(), input, &stdout, &stderr,
		client, broker, testOptions("raw"), nil)
	if status != 0 {
		t.Fatalf("interactive() status = %d; stderr = %q", status, stderr.String())
	}
	if strings.Join(commands, ",") != "exit,quit" {
		t.Fatalf("commands sent = %q, want escaped commands only", commands)
	}
}

// TestInteractiveControlDExitsLocally keeps the EOF control out of RCon commands.
func TestInteractiveControlDExitsLocally(t *testing.T) {
	var commands []string
	client := newFakeClient(func(_ context.Context, command string) ([]byte, error) {
		commands = append(commands, command)
		return []byte("OK"), nil
	})

	broker := startBroker(t, client)
	terminal := term.NewTerminal(terminalReadWriter{
		reader: strings.NewReader("\x04"),
		writer: &strings.Builder{},
	}, "")
	input := terminalShellReader{terminal: terminal}
	var stdout, stderr strings.Builder
	status := interactive(context.Background(), input, &stdout, &stderr,
		client, broker, testOptions("raw"), nil)
	if status != exitSuccess {
		t.Fatalf("interactive() status = %d; stderr = %q", status, stderr.String())
	}
	if len(commands) != 0 {
		t.Fatalf("Ctrl+D sent commands = %q, want none", commands)
	}
}

// TestTerminalShellReaderHandlesEditedInputAndCtrlD checks terminal-native line and EOF behavior.
func TestTerminalShellReaderHandlesEditedInputAndCtrlD(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "command line", input: "players\r", want: "players"},
		{name: "Ctrl+D", input: "\x04", wantErr: io.EOF},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			terminal := term.NewTerminal(terminalReadWriter{
				reader: strings.NewReader(test.input), writer: &strings.Builder{},
			}, "")
			reader := terminalShellReader{terminal: terminal}
			line, err := reader.ReadLine()
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ReadLine() error = %v, want %v", err, test.wantErr)
			}
			if line != test.want {
				t.Fatalf("ReadLine() = %q, want %q", line, test.want)
			}
		})
	}
}

// TestInteractiveCanPassBuiltinNamesToTheServer disables local exit handling.
func TestInteractiveCanPassBuiltinNamesToTheServer(t *testing.T) {
	var commands []string
	client := newFakeClient(func(_ context.Context, command string) ([]byte, error) {
		commands = append(commands, command)
		return []byte("OK"), nil
	})

	broker := startBroker(t, client)
	var stdout, stderr strings.Builder
	opts := testOptions("raw")
	opts.Shell.NoBuiltins = true
	status := interactive(context.Background(), newScannerShellReader(strings.NewReader("exit\nquit\n")),
		&stdout, &stderr, client, broker, opts, nil)
	if status != exitSuccess {
		t.Fatalf("interactive() status = %d; stderr = %q", status, stderr.String())
	}
	if !slices.Equal(commands, []string{"exit", "quit"}) {
		t.Fatalf("commands sent = %v, want [exit quit]", commands)
	}
}

// TestStreamMessagesTimestampMode checks timestamp formatting and the unchanged raw mode.
func TestStreamMessagesTimestampMode(t *testing.T) {
	for _, timestamps := range []bool{false, true} {
		events := make(chan bercon.Message, 1)
		events <- bercon.Message{Data: []byte("RCon admin logged in")}
		close(events)

		var stdout strings.Builder
		streamMessages(context.Background(), events, &stdout, timestamps)
		got := stdout.String()
		if !timestamps {
			if got != "RCon admin logged in\n" {
				t.Fatalf("streamMessages() = %q, want raw event", got)
			}

			continue
		}

		separator := strings.Index(got, "] ")
		if !strings.HasPrefix(got, "[") || separator < 0 ||
			got[separator+2:] != "RCon admin logged in\n" {
			t.Fatalf("streamMessages() = %q, want timestamp and event", got)
		}
		timestamp, err := time.Parse(time.RFC3339, got[1:separator])
		if err != nil {
			t.Fatalf("timestamp %q is not RFC 3339: %v", got[1:separator], err)
		}
		_, gotOffset := timestamp.Zone()
		_, localOffset := timestamp.In(time.Local).Zone()
		if gotOffset != localOffset {
			t.Fatalf("timestamp offset = %d, want local offset %d", gotOffset, localOffset)
		}
	}
}

// TestWatchReportsConnectionFailure distinguishes cancellation from transport failure.
func TestWatchReportsConnectionFailure(t *testing.T) {
	client := newFakeClient(nil)
	client.err = errors.New("connection lost")
	close(client.done)
	var stderr strings.Builder
	if status := watch(context.Background(), client, &stderr); status != 1 {
		t.Fatalf("watch() status = %d, want 1", status)
	}
	if !strings.Contains(stderr.String(), "connection lost") {
		t.Fatalf("watch() omitted transport error: %q", stderr.String())
	}
}

// fakeClient supplies only the transport behavior used by command routing tests.
type fakeClient struct {
	// command returns the direct response and may queue server messages.
	command func(context.Context, string) ([]byte, error)
	// messages models the asynchronous transport stream.
	messages chan bercon.Message
	// done stays open unless a test needs to simulate transport shutdown.
	done chan struct{}
	// err is returned after done closes to model an ended transport session.
	err error
}

// newFakeClient creates a transport double with a bounded message queue.
func newFakeClient(command func(context.Context, string) ([]byte, error)) *fakeClient {
	return &fakeClient{
		command: command, messages: make(chan bercon.Message, 8), done: make(chan struct{}),
	}
}

// Command returns the configured direct response for one command.
func (c *fakeClient) Command(ctx context.Context, command string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.command(ctx, command)
}

// Messages exposes the fake asynchronous message stream.
func (c *fakeClient) Messages() <-chan bercon.Message {
	return c.messages
}

// DroppedMessages reports no transport-side losses in the fake client.
func (c *fakeClient) DroppedMessages() uint64 {
	return 0
}

// Done reports when the fake transport has ended.
func (c *fakeClient) Done() <-chan struct{} {
	return c.done
}

// Err reports no transport failure for the fake client.
func (c *fakeClient) Err() error {
	return c.err
}

// startBroker starts message routing and registers cleanup before tests use it.
func startBroker(t *testing.T, client rconClient) *messageBroker {
	t.Helper()
	broker := newMessageBroker(client, true)
	go broker.run()
	t.Cleanup(broker.close)
	return broker
}

// assertExecuteResponse checks command output and any event left for stream consumers.
func assertExecuteResponse(t *testing.T, client *fakeClient, command, want, event string) {
	t.Helper()
	broker := startBroker(t, client)
	var stdout, stderr strings.Builder
	err := execute(context.Background(), command, &stdout, &stderr,
		client, broker, testOptions("json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout did not contain %q: %s", want, stdout.String())
	}
	if event == "" {
		return
	}

	select {
	case message := <-broker.events:
		if !strings.Contains(string(message.Data), event) {
			t.Fatalf("preserved event %q does not contain %q", message.Data, event)
		}
	case <-time.After(time.Second):
		t.Fatal("matching concurrent event was not preserved")
	}
}

// testOptions provides the valid connection settings shared by command tests.
func testOptions(format string) options {
	return options{
		Connection: connectionOptions{Timeout: config.Duration(2 * time.Second)},
		Output:     outputOptions{Format: output.Format(format)},
	}
}
