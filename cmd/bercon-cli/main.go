// Command-line entrypoint for bercon-cli: parses flags, runs RCON commands,
// and prints results in the selected output format.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon-cli/internal/printer"
	"github.com/woozymasta/bercon-cli/internal/vars"
	"github.com/woozymasta/bercon-cli/pkg/bercon"
)

type ConnectionOptions struct {
	// betteralign:ignore

	IP            string `short:"i" long:"ip"          env:"ADDRESS"     default:"127.0.0.1"  description:"Server IPv4 address"`
	Port          int    `short:"p" long:"port"        env:"PORT"        default:"2305"       description:"Server RCON port"`
	Password      string `short:"P" long:"password"    env:"PASSWORD"                         description:"Server RCON password"`
	Profile       string `short:"n" long:"profile"     env:"PROFILE"                          description:"Profile name from rc file"`
	Timeout       int    `short:"t" long:"timeout"     env:"TIMEOUT"     default:"3"          description:"Deadline and timeout in seconds"`
	Buffer        uint16 `short:"b" long:"buffer-size" env:"BUFFER_SIZE" default:"1024"       description:"Buffer size for RCON connection"`
	LoginAttempts int    `short:"a" long:"attempts"    env:"ATTEMPTS"    default:"1"          description:"Number of login attempts"`
}

type RepeatOptions struct {
	CmdSleep    int `short:"s" long:"cmd-sleep"  env:"SLEEP_CMD"   default:"1"          description:"Sleep time in milliseconds after each command"`
	LoopSleep   int `short:"S" long:"loop-sleep" env:"SLEEP_LOOP"  default:"5"          description:"Sleep time in seconds after each loop"`
	Keepalive   int `short:"k" long:"keepalive"  env:"KEEPALIVE"   default:"30"         description:"Keepalive interval in seconds"`
	RepeatCount int `short:"x" long:"repeat"     env:"REPEAT"      default:"1"          description:"Repeat command N times (-1 for infinite)"`
}

type ResourceOptions struct {
	RCPath string `short:"c" long:"config"  env:"CONFIG"  description:"Path to rc file (INI). If not set, standard locations are used"`
	BeCfg  string `short:"r" long:"server-cfg" env:"SERVER_CFG" description:"Path to beserver_x64.cfg file or directory to search"`
	GeoDB  string `short:"g" long:"geo-db"     env:"GEO_DB"     description:"Path to Country GeoDB mmdb file"`
}

type OutputOptions struct {
	Format string `short:"f" long:"format" env:"FORMAT" default:"table" choice:"json" choice:"table" choice:"raw" choice:"md" choice:"html" description:"Output format"`
	JSON   bool   `short:"j" long:"json"   env:"JSON"   description:"Print result in JSON format (deprecated, use --format=json)"`
}

type UtilityOptions struct {
	ListRC  bool `short:"l" long:"list-profiles" description:"List profiles from rc file and exit"`
	Example bool `short:"e" long:"example"       description:"Print example rc (INI) config and exit"`
}

type InfoOptions struct {
	Version bool `short:"v" long:"version" description:"Show version, commit, and build time"`
	Help    bool `short:"h" long:"help"    description:"Show this help message"`
}

type Options struct {
	// betteralign:ignore

	Conn      ConnectionOptions `group:"Connection Settings" env-namespace:"BERCON"`
	Repeat    RepeatOptions     `group:"Repeat Settings" env-namespace:"BERCON"`
	Resources ResourceOptions   `group:"File Resources" env-namespace:"BERCON"`
	Output    OutputOptions     `group:"Output Formatting" env-namespace:"BERCON"`
	Utility   UtilityOptions    `group:"Utility Commands" env-namespace:"BERCON"`
	Info      InfoOptions       `group:"Informational" env-namespace:"BERCON"`
}

func main() {
	opts := &Options{}
	p := flags.NewParser(opts, flags.PassDoubleDash|flags.PrintErrors|flags.PassAfterNonOption)
	p.Usage = "[OPTIONS] command [command, ...]"
	p.LongDescription = longDescription()
	p.Name = filepath.Base(p.Name)

	args, err := p.Parse()
	if err != nil {
		os.Exit(0)
	}

	if opts.Info.Help {
		p.WriteHelp(os.Stdout)
		return
	}

	if opts.Info.Version {
		vars.Print()
		return
	}

	if opts.Utility.Example {
		printExampleRC(os.Stdout)
		return
	}

	if opts.Utility.ListRC {
		base := &config.RC{
			Format:     opts.Output.Format,
			TimeoutSec: opts.Conn.Timeout,
			BufferSize: opts.Conn.Buffer,
		}
		if err := config.PrintProfiles(opts.Resources.RCPath, base, os.Stdout); err != nil {
			fatalf("rc: %v", err)
		}
		return
	}

	if rc, ok, err := config.LoadRC(opts.Resources.RCPath, opts.Conn.Profile); err != nil {
		fatalf("rc: %v", err)
	} else if ok {
		// apply globals/profile into opts (lowest priority vs CLI/env except -r semantics)
		if opts.Output.Format == "" && rc.Format != "" {
			opts.Output.Format = rc.Format
		}
		if opts.Resources.GeoDB == "" && rc.GeoDB != "" {
			opts.Resources.GeoDB = rc.GeoDB
		}
		if opts.Conn.Timeout == 0 && rc.TimeoutSec > 0 {
			opts.Conn.Timeout = rc.TimeoutSec
		}
		if opts.Conn.Buffer == 0 && rc.BufferSize > 0 {
			opts.Conn.Buffer = rc.BufferSize
		}

		// connection parameters from rc (will be overridden by -r below if both set)
		if opts.Conn.IP == "" && rc.IP != "" {
			opts.Conn.IP = rc.IP
		}
		if opts.Conn.Port == 0 && rc.Port != 0 {
			opts.Conn.Port = rc.Port
		}
		if opts.Conn.Password == "" && rc.Password != "" {
			opts.Conn.Password = rc.Password
		}

		// if profile provided server_cfg – treat as BeCfg input
		if opts.Resources.BeCfg == "" && rc.ServerCfg != "" {
			opts.Resources.BeCfg = rc.ServerCfg
		}
	}

	if opts.Resources.BeCfg != "" {
		rc, err := config.LoadFromBeServerCfg(opts.Resources.BeCfg)
		if err != nil {
			fatalf("beserver cfg: %v\n", err)
		}

		opts.Conn.IP = rc.IP
		opts.Conn.Port = rc.Port
		opts.Conn.Password = rc.Password
	}

	// defaults
	if opts.Conn.IP == "" {
		opts.Conn.IP = "127.0.0.1"
	}
	if opts.Conn.Port == 0 {
		opts.Conn.Port = 2305
	}

	if opts.Conn.Password == "" {
		fatalf("RCON password must be specified")
	}

	if len(args) < 1 {
		fatalf("Command must be provided")
	}

	if opts.Repeat.RepeatCount == 0 {
		fatalf("Repeat must be >= 1 or -1 for infinite")
	}

	// map legacy --json to --format=json
	format := printer.FormatFromString(opts.Output.Format)
	if opts.Output.JSON {
		format = printer.FormatJSON
	}

	addr := fmt.Sprintf("%s:%d", opts.Conn.IP, opts.Conn.Port)
	conn, err := bercon.Open(addr, opts.Conn.Password)
	if err != nil {
		fatalf("error opening connection: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fatalf("cant close connection: %v", err)
		}
	}()

	// setup bercon params
	conn.SetDeadlineTimeout(opts.Conn.Timeout)
	conn.SetBufferSize(opts.Conn.Buffer)
	conn.SetLoginAttempts(opts.Conn.LoginAttempts)

	gap := time.Duration(opts.Repeat.LoopSleep) * time.Second
	if len(args) > 1 {
		gap = max(gap, time.Duration(opts.Repeat.CmdSleep)*time.Millisecond)
	}

	// keepalive only for long sessions
	if (opts.Repeat.RepeatCount < 0 || opts.Repeat.RepeatCount > 1 || len(args) > 1) &&
		gap >= bercon.MaxKeepaliveTimeout*time.Second {
		conn.SetKeepaliveTimeout(opts.Repeat.Keepalive)
		conn.StartKeepAlive()
	}

	runOnce := func() {
		for idx, cmd := range args {
			data, err := conn.Send(cmd)
			if err != nil {
				fatalf("error in command %d '%s': %v", idx, cmd, err)
			}
			if err := printer.ParseAndPrintData(os.Stdout, data, cmd, opts.Resources.GeoDB, format); err != nil {
				fatalf("cant print response data: %v", err)
			}

			if idx < len(args)-1 && opts.Repeat.CmdSleep > 0 {
				time.Sleep(time.Duration(opts.Repeat.CmdSleep) * time.Millisecond)
			}
		}
	}

	for loop := 0; opts.Repeat.RepeatCount < 0 || loop < opts.Repeat.RepeatCount; loop++ {
		runOnce()

		// sleep only between loops
		if opts.Repeat.LoopSleep > 0 && (opts.Repeat.RepeatCount < 0 || loop < opts.Repeat.RepeatCount-1) {
			time.Sleep(time.Duration(opts.Repeat.LoopSleep) * time.Second)
		}
	}
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func longDescription() string {
	var rcPaths []string
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			rcPaths = append(rcPaths, strings.TrimSpace(filepath.Join(appData, "bercon-cli", "config.ini")))
		}
	case "darwin":
		rcPaths = append(rcPaths,
			strings.TrimSpace(filepath.Join(home, "Library", "Application Support", "bercon-cli", "config.ini")))
	}

	rcPaths = append(rcPaths,
		strings.TrimSpace(filepath.Join(home, ".config", "bercon-cli", "config.ini")),
		strings.TrimSpace(filepath.Join(home, ".bercon-cli.ini")))

	text := fmt.Sprintf(`BattlEye RCon CLI — command-line tool for interacting with BattlEye RCON servers (used by DayZ, Arma 2/3, etc).
It allows executing server commands, reading responses, and formatting results in table, JSON, Markdown, or HTML.

Configuration can be provided via:
- CLI flags
- Environment variables
- RC config file (INI) with globals and profiles
- beserver_x64*.cfg — to auto-load RConIP, RConPort and RConPassword

When the RC file is not explicitly specified with --config/-c,
bercon-cli automatically looks for it in the following locations for %s:
- %s
`, runtime.GOOS, strings.Join(rcPaths, "\n- "))

	return text
}

func printExampleRC(w io.Writer) {
	_, _ = fmt.Fprintln(w, `# Example bercon-cli config file (INI)
# Lines starting with '#' are comments.

[globals]
# Default settings applied to all profiles (unless overridden)
ip = 127.0.0.1
port = 2305
password = MyDefaultPass
geo_db = /srv/geoip/GeoLite2-Country.mmdb
format = table
timeout = 3
buffer_size = 1024

[profile.dayz-local]
# Load BattlEye RCon params automatically from beserver_x64*.cfg
server_cfg = /home/dayz/server/battleye
format = json

[profile.dayz-eu]
ip = 192.168.1.55
port = 2310
password = strongPass
geo_db = /data/geo/GeoLite2.mmdb

[profile.arma3-test]
server_cfg = C:\Games\Arma3Server\battleye
timeout = 5`)
}
