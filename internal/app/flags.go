// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon-cli/internal/output"
	"github.com/woozymasta/flags"
)

// parserOptions enables the CLI's built-in help, version, completion, docs, and dotenv behavior.
const parserOptions flags.Options = flags.HelpFlag |
	flags.HelpCommand |
	flags.VersionFlag |
	flags.VersionCommand |
	flags.CompletionCommand |
	flags.DocsCommand |
	flags.PrintErrors |
	flags.PrintHelpOnInputErrors |
	flags.PassDoubleDash |
	flags.PassAfterNonOption |
	flags.AllowBoolValues |
	flags.ConfiguredValues |
	flags.DetectShellFlagStyle |
	flags.DetectShellEnvStyle |
	flags.EnvProvisioning |
	flags.AutoShowChoiceListInHelp |
	flags.StrictPositionalArgs |
	flags.ShowRepeatableInHelp |
	flags.KeepDescriptionWhitespace |
	flags.DotEnv |
	flags.DotEnvFlags

// options groups CLI flags by the part of the application they configure.
type options struct { // betteralign:ignore
	Connection connectionOptions `ini-group:"connection" group:"Connection"         group-i18n:"group.connection"`
	Output     outputOptions     `ini-group:"output"     group:"Output"             group-i18n:"group.output"`
	Profile    profileOptions    `ini-group:"profile"    group:"Profile and config" group-i18n:"group.profile"`
	Geo        geoOptions        `ini-group:"geo"        group:"GeoIP"              group-i18n:"group.geo" namespace:"geo" env-namespace:"GEO"`
	Exec       execOptions       `ini-group:"exec" command:"exec" description-i18n:"command.exec" default-command:"true"`
	Watch      watchOptions      `ini-group:"watch" command:"watch" description-i18n:"command.watch"`
	Shell      shellOptions      `ini-group:"shell" command:"shell" description-i18n:"command.shell"`
}

// dur keeps the configuration package's seconds-based duration concise in option declarations.
type dur = config.Duration

// msDur keeps the configuration package's millisecond-based duration concise in option declarations.
type msDur = config.MillisecondDuration

// connectionOptions contains endpoint and RCon session settings.
type connectionOptions struct {
	IP        string `short:"i" long:"ip"          description-i18n:"opt.ip"        default:"127.0.0.1" env:"ADDRESS"`
	Password  string `short:"P" long:"password"    description-i18n:"opt.password" secret:"true"`
	Port      int    `short:"p" long:"port"        description-i18n:"opt.port"      default:"2305"      validate-min:"1" validate-max:"65535"`
	Timeout   dur    `short:"t" long:"timeout"     description-i18n:"opt.timeout"   default:"3s"        validate-min:"1"`
	Attempts  int    `short:"a" long:"attempts"    description-i18n:"opt.attempts"  default:"1"         validate-min:"1"`
	Keepalive dur    `short:"k" long:"keepalive"   description-i18n:"opt.keepalive" default:"30s"       validate-min:"0"`
	Buffer    uint16 `short:"b" long:"buffer-size" description-i18n:"opt.buffer" deprecated:"ignored"`
}

// profileOptions contains profile selection and configuration-file flags.
type profileOptions struct {
	Profile   string `short:"n" long:"profile"       description-i18n:"opt.profile"`
	RCPath    string `short:"c" long:"config"        description-i18n:"opt.config"`
	ServerCfg string `short:"r" long:"server-cfg"    description-i18n:"opt.server_cfg"    auto-env:"false"`
	ListRC    bool   `short:"l" long:"list-profiles" description-i18n:"opt.list_profiles" auto-env:"false"`
	Example   bool   `short:"e" long:"example"       description-i18n:"opt.example"       auto-env:"false"`
}

// geoOptions contains optional GeoIP database and update settings.
type geoOptions struct {
	DB      string `long:"db"          description-i18n:"opt.geo_db"          short:"g"`
	URL     string `long:"db-url"      description-i18n:"opt.geo_db_url"      short:"G"`
	URLAuth string `long:"db-url-auth" description-i18n:"opt.geo_db_url_auth" secret:"true"`
}

// execOptions controls repetition and pacing for commands sent by the exec command.
type execOptions struct {
	Repeat    int   `short:"x" default:"1"   long:"repeat"     validate-min:"-1" description-i18n:"opt.repeat"`
	CmdSleep  msDur `short:"s" default:"1ms" long:"cmd-sleep"  validate-min:"0"  description-i18n:"opt.cmd_sleep"  env:"SLEEP_CMD"`
	LoopSleep dur   `short:"S" default:"5s"  long:"loop-sleep" validate-min:"0"  description-i18n:"opt.loop_sleep" env:"SLEEP_LOOP"`
}

// watchOptions controls how asynchronous server messages are displayed.
type watchOptions struct {
	// Timestamps prefixes each message with its local receive time.
	Timestamps bool `long:"timestamps" description-i18n:"opt.watch_timestamps"`
}

// shellOptions controls interpretation of commands entered in the interactive shell.
type shellOptions struct {
	// NoBuiltins sends CLI built-in commands to the server instead of handling them locally.
	NoBuiltins bool `long:"no-builtins" description-i18n:"opt.shell_no_builtins"`
}

// outputOptions contains response-format selection flags.
type outputOptions struct {
	Format output.Format `short:"f" long:"format" default:"table" description-i18n:"opt.format" choices:"table;json;raw;md;html;csv"`
}

// newParser binds option tags and the CLI's built-in commands.
func newParser(opts *options) (*flags.Parser, error) {
	parser := flags.NewParser(opts, parserOptions)
	parser.Name = filepath.Base(os.Args[0])
	parser.SubcommandsOptional = true
	parser.SetEnvPrefix("BERCON")
	parser.NamespaceDelimiter = "-"
	parser.SetI18n(i18nConfig())

	parser.SetVersionInfo(flags.VersionInfo{
		Version:      version,
		Revision:     commit,
		RevisionTime: buildTime,
		URL:          repositoryURL,
	})

	execCommand, err := parser.CommandFor(&opts.Exec)
	if err != nil {
		return nil, appWrap("app.error.examples", "find exec command", err)
	}
	if err := execCommand.SetExamples(
		flags.Example().
			Describe(appText("app.example.direct", "Connect directly to a server")).
			Option(&opts.Connection.IP, "192.0.2.10").
			Option(&opts.Connection.Port, "2305").
			Option(&opts.Connection.Password, "<RCON_PASSWORD>").
			Arg("players"),

		flags.Example().
			Describe(appText("app.example.profile", "Run a command against a saved profile")).
			Option(&opts.Profile.Profile, "dayz-local").
			Arg("players"),

		flags.Example().
			Describe(appText("app.example.command", "Pass a command with spaces to the server")).
			Option(&opts.Profile.Profile, "arma3-eu").
			Raw("--").
			Arg("say -1 Server restart in 5 minutes"),
	); err != nil {
		return nil, appWrap("app.error.examples", "configure command examples", err)
	}

	shellCommand, err := parser.CommandFor(&opts.Shell)
	if err != nil {
		return nil, appWrap("app.error.examples", "find shell command", err)
	}
	if err := shellCommand.SetExamples(
		flags.Example().Option(&opts.Profile.Profile, "dayz-local"),
	); err != nil {
		return nil, appWrap("app.error.examples", "configure shell examples", err)
	}

	watchCommand, err := parser.CommandFor(&opts.Watch)
	if err != nil {
		return nil, appWrap("app.error.examples", "find watch command", err)
	}
	if err := watchCommand.SetExamples(
		flags.Example().
			Option(&opts.Profile.Profile, "dayz-local").
			Option(&opts.Watch.Timestamps).
			Raw(">> dayz-watch.log"),
	); err != nil {
		return nil, appWrap("app.error.examples", "configure watch examples", err)
	}

	parser.SetHelpHeader(applicationHelpHeader())
	for _, format := range []flags.DocFormat{
		flags.DocFormatMarkdown,
		flags.DocFormatHTML,
		flags.DocFormatMan,
	} {
		if err := parser.SetDocHeader(format, applicationDocsHeader(format)); err != nil {
			return nil, appWrap("app.error.docs_header", "configure documentation header", err)
		}
	}

	return parser, nil
}

// validate checks rules that depend on multiple options or command selection.
func (opts options) validate(mode sessionMode, commands []string, localizer *flags.Localizer) error {
	if opts.Connection.Password == "" {
		return errors.New(localizer.Localize("app.password", "RCon password must be specified", nil))
	}

	if mode == modeExec && opts.Exec.Repeat == 0 {
		return errors.New(localizer.Localize("app.repeat", "repeat must be positive or -1", nil))
	}

	if mode != modeExec && len(commands) > 0 {
		return errors.New(localizer.Localize(
			"app.command.mode", "positional commands can only be used with exec", nil,
		))
	}

	if len(commands) == 0 && mode == modeExec {
		return errors.New(localizer.Localize("app.command", "a server command is required", nil))
	}

	return nil
}
