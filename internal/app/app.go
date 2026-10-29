// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/oschwald/geoip2-golang/v2"
	"github.com/woozymasta/bercon"
	"github.com/woozymasta/flags"
)

// sessionMode selects the work performed after a successful RCon login.
type sessionMode string

const (
	// modeExec sends the positional commands to the server.
	modeExec sessionMode = "exec"
	// modeWatch reads asynchronous messages from the server.
	modeWatch sessionMode = "watch"
	// modeShell reads RCon commands from standard input.
	modeShell sessionMode = "shell"
)

// invocation holds the parsed command and configuration needed for one run.
type invocation struct {
	bootstrapErr error          // bootstrapErr is deferred until built-in help and the example action run.
	opts         options        // opts is the authoritative CLI configuration after parsing.
	mode         sessionMode    // mode is the selected session command.
	commands     []string       // commands are the positional RCon commands passed to the client.
	bootstrap    bootstrapState // bootstrap contains values loaded before the full parser applies defaults.
}

// Run parses arguments, opens one RCon client and returns the process status.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	inv, status := parseInvocation(args, stderr)
	if inv == nil {
		return int(status)
	}

	if inv.opts.Profile.Example {
		if err := printExample(stdout); err != nil {
			writeDiagnostic(stderr, "%s: %v\n", appText("app.error.write_example", "write example"), err)
			return int(exitRuntimeError)
		}

		return int(exitSuccess)
	}

	if inv.bootstrapErr != nil {
		writeDiagnostic(stderr, "%v\n", inv.bootstrapErr)
		return int(exitUsageError)
	}

	if inv.opts.Profile.ListRC {
		if err := printProfiles(stdout, inv.bootstrap.file, inv.bootstrap.found); err != nil {
			writeDiagnostic(stderr, "%s: %v\n", appText("app.error.write_profiles", "write profiles"), err)
			return int(exitRuntimeError)
		}

		return int(exitSuccess)
	}

	if err := inv.opts.validate(inv.mode, inv.commands, flags.NewLocalizer(i18nConfig())); err != nil {
		writeDiagnostic(stderr, "%v\n", err)
		return int(exitUsageError)
	}

	return int(runSession(inv.mode, inv.commands, stdin, stdout, stderr, inv.opts))
}

// parseInvocation resolves configuration, applies parser defaults, and parses arguments.
func parseInvocation(args []string, stderr io.Writer) (*invocation, exitCode) {
	boot, bootstrapErr := bootstrap(args)
	var opts options
	// Preload the selected server before parsing;
	// ConfiguredValues lets CLI flags override it
	// while ambient environment values fill only missing fields.
	if bootstrapErr == nil {
		applyConfigValues(&opts, boot.rc, boot.server)
	}

	parser, err := newParser(&opts)
	if err != nil {
		writeDiagnostic(stderr, "%v\n", err)
		return nil, exitUsageError
	}

	commands, err := parser.ParseArgs(args)
	if err != nil {
		var parseErr *flags.Error
		if errors.As(err, &parseErr) &&
			(parseErr.Type == flags.ErrHelp || parseErr.Type == flags.ErrVersion) {
			return nil, exitSuccess
		}

		if bootstrapErr != nil {
			writeDiagnostic(stderr, "%v\n", bootstrapErr)
		}
		return nil, exitUsageError
	}

	mode, selected := selectedSessionMode(parser.Active)
	if !selected {
		// Built-in help, completion, docs, and version commands have already written their output.
		return nil, exitSuccess
	}

	return &invocation{
		bootstrap:    boot,
		bootstrapErr: bootstrapErr,
		opts:         opts,
		mode:         mode,
		commands:     commands,
	}, exitSuccess
}

// selectedSessionMode maps an active application command to its session behavior.
func selectedSessionMode(command *flags.Command) (sessionMode, bool) {
	if command == nil {
		return modeExec, false
	}

	switch command.Name {
	case string(modeExec):
		return modeExec, true

	case string(modeWatch):
		return modeWatch, true

	case string(modeShell):
		return modeShell, true

	default:
		return modeExec, false
	}
}

// runSession owns the connected client, message broker, GeoIP reader, and selected command flow.
func runSession(mode sessionMode, commands []string, stdin io.Reader, stdout, stderr io.Writer, opts options) exitCode {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	clientOptions := bercon.DefaultOptions()
	clientOptions.LoginTimeout = time.Duration(opts.Connection.Timeout)
	clientOptions.ResponseTimeout = time.Duration(opts.Connection.Timeout)
	clientOptions.LoginAttempts = opts.Connection.Attempts
	clientOptions.KeepAliveInterval = time.Duration(opts.Connection.Keepalive)

	// Dial applies LoginTimeout to each attempt; ctx only needs to carry process shutdown.
	client, err := bercon.Dial(
		ctx,
		net.JoinHostPort(opts.Connection.IP, strconv.Itoa(opts.Connection.Port)),
		opts.Connection.Password,
		&clientOptions,
	)
	if err != nil {
		writeDiagnostic(stderr, "%s: %v\n", appText("app.error.connect", "connect"), err)
		return exitRuntimeError
	}

	defer func() { _ = client.Close() }()
	var geo *geoip2.Reader
	if opts.Geo.DB != "" {
		if opts.Geo.URL != "" {
			localizer := flags.NewLocalizer(i18nConfig())
			geoProgress := geoDBProgressOptions{
				output:   stderr,
				terminal: flags.DetectWriterTTY(stderr),
				startedMessage: localizer.Localize(
					"geo.progress.started", "Downloading GeoIP database", nil,
				),
				completedMessage: localizer.Localize(
					"geo.progress.completed", "GeoIP database downloaded in %s", nil,
				),
				failedMessage: localizer.Localize(
					"geo.progress.failed", "GeoIP database download failed after %s", nil,
				),
			}
			if err := updateGeoDB(
				opts.Geo.DB,
				opts.Geo.URL,
				opts.Geo.URLAuth,
				time.Now(),
				geoProgress,
			); err != nil {
				if _, statErr := os.Stat(opts.Geo.DB); statErr != nil {
					writeDiagnostic(stderr, "%s: %v\n",
						appText("app.error.geo_update", "GeoIP database update"), err)
					return exitRuntimeError
				}

				writeDiagnostic(stderr, "%s: %v\n",
					appText("app.error.geo_update_skipped", "GeoIP database update skipped"), err)
			}
		}

		geo, err = geoip2.Open(opts.Geo.DB)
		if err != nil {
			writeDiagnostic(stderr, "%s: %v\n", appText("app.error.geo_open", "GeoIP database"), err)
			return exitRuntimeError
		}

		defer func() { _ = geo.Close() }()
	}

	var shellInput shellLineReader
	sessionOutput := stdout
	var restoreTerminal func() error
	if mode == modeShell {
		shellInput, sessionOutput, restoreTerminal, err = configureShellInput(stdin, stdout)
		if err != nil {
			writeDiagnostic(stderr, "%s: %v\n", appText("app.error.terminal", "interactive terminal"), err)
			return exitRuntimeError
		}

		if restoreTerminal != nil {
			defer func() {
				if err := restoreTerminal(); err != nil {
					writeDiagnostic(stderr, "%s: %v\n",
						appText("app.error.terminal", "interactive terminal"), err)
				}
			}()
		}
	}

	forwardEvents := mode == modeWatch || mode == modeShell
	broker := newMessageBroker(client, forwardEvents)
	go broker.run()
	defer broker.close()

	safeStdout := &synchronizedWriter{w: sessionOutput}
	safeStderr := &synchronizedWriter{w: stderr}
	var streamDone chan struct{}
	if forwardEvents {
		streamDone = make(chan struct{})
		go func() {
			defer close(streamDone)
			streamMessages(ctx, broker.events, safeStdout, mode == modeWatch && opts.Watch.Timestamps)
		}()
		go reportDroppedMessages(ctx, broker, safeStderr)
	}

	switch mode {
	case modeWatch:
		return watch(ctx, client, safeStderr)

	case modeShell:
		defer func() {
			stop()
			<-streamDone
		}()
		return interactive(ctx, shellInput, safeStdout, safeStderr, client, broker, opts, geo)
	}

	return batch(ctx, commands, safeStdout, safeStderr, client, broker, opts, geo)
}
