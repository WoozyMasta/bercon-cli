// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang/v2"
	"github.com/woozymasta/bercon"
	"github.com/woozymasta/bercon-cli/internal/output"
	"golang.org/x/term"
)

// shellLineReader supplies complete command lines to the interactive session.
type shellLineReader interface {
	ReadLine() (string, error)
}

// scannerShellReader adapts redirected or piped input to shellLineReader.
type scannerShellReader struct {
	scanner *bufio.Scanner // scanner reads newline-delimited commands from standard input.
}

// terminalShellReader uses terminal editing and terminal-native EOF handling.
type terminalShellReader struct {
	terminal *term.Terminal // terminal provides command history and Ctrl+D handling.
}

// terminalReadWriter joins the separate process input and output streams for x/term.
type terminalReadWriter struct {
	reader io.Reader // reader supplies keyboard input to the terminal editor.
	writer io.Writer // writer displays edited input and asynchronous server messages.
}

// batch executes commands in order and never retries an application command.
func batch(
	ctx context.Context,
	commands []string,
	stdout, stderr io.Writer,
	client rconClient,
	broker *messageBroker,
	opts options,
	geo *geoip2.Reader,
) exitCode {
	for loop := 0; opts.Exec.Repeat < 0 || loop < opts.Exec.Repeat; loop++ {
		for index, command := range commands {
			if err := execute(ctx, command, stdout, stderr, client, broker, opts, geo); err != nil {
				writeDiagnostic(stderr, "%v\n", err)
				return exitRuntimeError
			}

			if index+1 < len(commands) && opts.Exec.CmdSleep > 0 &&
				!pause(ctx, time.Duration(opts.Exec.CmdSleep)) {
				return exitSuccess
			}
		}

		if (opts.Exec.Repeat < 0 || loop+1 < opts.Exec.Repeat) && opts.Exec.LoopSleep > 0 &&
			!pause(ctx, time.Duration(opts.Exec.LoopSleep)) {
			return exitSuccess
		}
	}

	return exitSuccess
}

// execute arbitrates direct response bytes against an already-installed async matcher.
func execute(
	ctx context.Context,
	command string,
	stdout, stderr io.Writer,
	client rconClient,
	broker *messageBroker,
	opts options,
	geo *geoip2.Reader,
) error {
	waiter := broker.match(command)
	asyncResponseUsed := false
	if waiter != nil {
		defer func() { broker.clear(waiter, !asyncResponseUsed) }()
	}

	renderAsync := func(message bercon.Message) error {
		asyncResponseUsed = true
		return renderMatched(stdout, stderr, message.Data, command, opts.Output.Format, geo)
	}

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.Connection.Timeout))
	defer cancel()
	data, commandErr := client.Command(callCtx, command)

	direct := output.Parse(data, command)
	if direct.Known {
		if _, err := output.RenderLocalized(
			stdout, data, command, opts.Output.Format, geo, appText,
		); err != nil {
			return err
		}

		if direct.Err != nil {
			writeDiagnostic(stderr, appText(
				"app.warning.response_malformed",
				"Warning: malformed structured response: %v",
			)+"\n", direct.Err)
			if opts.Output.Format != output.Raw {
				return appError(
					"app.error.response_malformed",
					"Structured response was malformed; raw response printed",
				)
			}
		}
		return commandErr
	}

	// The reader stores a matching event while Command waits,
	// so no second goroutine needs access to the transport channel and fast events are not lost.
	if waiter != nil {
		select {
		case message := <-waiter:
			return renderAsync(message)

		default:
		}

		select {
		case message := <-waiter:
			return renderAsync(message)

		case <-callCtx.Done():
			// A timer and the matching event can become ready together.
			// Prefer the event if it was already routed before the deadline fired.
			select {
			case message := <-waiter:
				return renderAsync(message)

			default:
			}

		case <-client.Done():
			select {
			case message := <-waiter:
				return renderAsync(message)

			default:
			}
		}
	}

	if commandErr != nil {
		if len(data) > 0 {
			if err := output.WriteRaw(stdout, data); err != nil {
				return err
			}
		}
		return appWrap("app.error.command", "RCon command failed", commandErr)
	}

	if waiter != nil {
		if err := output.WriteRaw(stdout, data); err != nil {
			return err
		}
		if opts.Output.Format == output.Raw {
			return nil
		}

		writeDiagnostic(stderr, "%s\n", appText(
			"app.warning.response_unrecognized",
			"Warning: structured response was not recognized",
		))
		return appError(
			"app.error.response_unrecognized",
			"Structured response was not recognized",
		)
	}

	_, err := output.RenderLocalized(stdout, data, command, opts.Output.Format, geo, appText)
	return err
}

// renderMatched presents the recognized async payload and reports partial rows.
func renderMatched(stdout, stderr io.Writer, data []byte, command string, format output.Format,
	geo *geoip2.Reader,
) error {
	parsed, err := output.RenderLocalized(stdout, data, command, format, geo, appText)
	if err != nil {
		return err
	}

	if parsed.Err != nil {
		writeDiagnostic(stderr, appText(
			"app.warning.response_malformed",
			"Warning: malformed structured response: %v",
		)+"\n", parsed.Err)
		if format != output.Raw {
			return appError(
				"app.error.response_malformed",
				"Structured response was malformed; raw response printed",
			)
		}
	}

	return nil
}

// interactive reads one command per line and handles local exit controls.
func interactive(ctx context.Context, input shellLineReader, stdout, stderr io.Writer,
	client rconClient, broker *messageBroker, opts options,
	geo *geoip2.Reader,
) exitCode {
	for {
		line, err := input.ReadLine()
		if errors.Is(err, io.EOF) {
			return exitSuccess
		}
		if err != nil {
			writeDiagnostic(stderr, "%s: %v\n", appText("app.error.read_commands", "read commands"), err)
			return exitRuntimeError
		}

		command := strings.TrimSpace(line)
		if command == "" {
			continue
		}
		switch command {
		case "\x04":
			return exitSuccess

		case "exit", "quit":
			if !opts.Shell.NoBuiltins {
				return exitSuccess
			}

		case "\\exit":
			command = "exit"

		case "\\quit":
			command = "quit"
		}

		if err := execute(ctx, command, stdout, stderr, client, broker, opts, geo); err != nil {
			writeDiagnostic(stderr, appText("app.error.command_detail", "Command %q: %v")+"\n", command, err)
		}
	}
}

// ReadLine returns the next complete line from redirected or piped input.
func (r scannerShellReader) ReadLine() (string, error) {
	if r.scanner.Scan() {
		return r.scanner.Text(), nil
	}
	if err := r.scanner.Err(); err != nil {
		return "", err
	}

	return "", io.EOF
}

// ReadLine returns one edited terminal line; x/term reports Ctrl+D as io.EOF.
func (r terminalShellReader) ReadLine() (string, error) {
	line, err := r.terminal.ReadLine()
	if err == term.ErrPasteIndicator {
		return line, nil
	}

	return line, err
}

// Read supplies keyboard input to the terminal editor.
func (rw terminalReadWriter) Read(data []byte) (int, error) {
	return rw.reader.Read(data)
}

// Write displays terminal editor output.
func (rw terminalReadWriter) Write(data []byte) (int, error) {
	return rw.writer.Write(data)
}

// newScannerShellReader configures line scanning for non-terminal input.
func newScannerShellReader(input io.Reader) scannerShellReader {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)

	return scannerShellReader{scanner: scanner}
}

// configureShellInput selects terminal editing for TTYs and scanning for other input streams.
func configureShellInput(input io.Reader, output io.Writer) (shellLineReader, io.Writer, func() error, error) {
	inputFile, inputIsFile := input.(*os.File)
	outputFile, outputIsFile := output.(*os.File)
	if !inputIsFile ||
		!outputIsFile ||
		!term.IsTerminal(int(inputFile.Fd())) ||
		!term.IsTerminal(int(outputFile.Fd())) {
		return newScannerShellReader(input), output, nil, nil
	}

	fd := int(inputFile.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("enable raw terminal input: %w", err)
	}

	terminal := term.NewTerminal(terminalReadWriter{reader: input, writer: output}, "")
	if width, height, err := term.GetSize(int(outputFile.Fd())); err == nil {
		_ = terminal.SetSize(width, height)
	}

	restore := func() error {
		return term.Restore(fd, state)
	}

	return terminalShellReader{terminal: terminal}, terminal, restore, nil
}

// watch prints asynchronous server messages until cancellation or transport failure.
func watch(ctx context.Context, client rconClient, stderr io.Writer) exitCode {
	for {
		select {
		case <-ctx.Done():
			return exitSuccess

		case <-client.Done():
			if err := client.Err(); err != nil {
				writeDiagnostic(stderr, "%s: %v\n", appText("app.error.connection", "connection"), err)
				return exitRuntimeError
			}

			return exitSuccess
		}
	}
}

// pause makes repeated command delays interruptible by process cancellation.
func pause(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true

	case <-ctx.Done():
		return false
	}
}
