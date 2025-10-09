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

	"github.com/jessevdk/go-flags"
	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon-cli/internal/printer"
	"github.com/woozymasta/bercon-cli/internal/vars"
	"github.com/woozymasta/bercon-cli/pkg/bercon"
)

// CLI options
type Options struct {
	IP       string `short:"i" long:"ip"            description:"Server IPv4 address (default: 127.0.0.1)" env:"BERCON_ADDRESS"`
	Password string `short:"P" long:"password"      description:"Server RCON password" env:"BERCON_PASSWORD"`
	RCPath   string `short:"c" long:"config"        description:"Path to rc file (INI). If not set, standard locations are used" env:"BERCON_CONFIG"`
	Profile  string `short:"n" long:"profile"       description:"Profile name from rc file" env:"BERCON_PROFILE"`
	BeCfg    string `short:"r" long:"server-cfg"    description:"Path to beserver_x64.cfg file or directory to search beserver_x64*.cfg" env:"BERCON_SERVER_CFG"`
	GeoDB    string `short:"g" long:"geo-db"        description:"Path to Country GeoDB mmdb file" env:"BERCON_GEO_DB"`
	Format   string `short:"f" long:"format"        description:"Output format (default: table)" choice:"json" choice:"table" choice:"raw" choice:"md" choice:"html" env:"BERCON_FORMAT"`
	Port     int    `short:"p" long:"port"          description:"Server RCON port (default: 2305)" env:"BERCON_PORT"`
	Timeout  int    `short:"t" long:"timeout"       description:"Deadline and timeout in seconds (default: 3)" env:"BERCON_TIMEOUT"`
	Buffer   uint16 `short:"b" long:"buffer-size"   description:"Buffer size for RCON connection (default: 1024)" env:"BERCON_BUFFER_SIZE"`
	JSON     bool   `short:"j" long:"json"          description:"Print result in JSON format (deprecated, use --format=json)" env:"BERCON_JSON_OUTPUT"`
	ListRC   bool   `short:"l" long:"list-profiles" description:"List profiles from rc file and exit"`
	Example  bool   `short:"e" long:"example"       description:"Print example rc (INI) config and exit"`
	Help     bool   `short:"h" long:"help"          description:"Show version, commit, and build time"`
	Version  bool   `short:"v" long:"version"       description:"Prints this help message"`
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

	if opts.Help {
		p.WriteHelp(os.Stdout)
		return
	}

	if opts.Version {
		vars.Print()
		return
	}

	if opts.Example {
		printExampleRC(os.Stdout)
		return
	}

	if opts.ListRC {
		base := &config.RC{
			Format:     opts.Format,
			TimeoutSec: opts.Timeout,
			BufferSize: opts.Buffer,
		}
		if err := config.PrintProfiles(opts.RCPath, base, os.Stdout); err != nil {
			fatalf("rc: %v", err)
		}
		return
	}

	if rc, ok, err := config.LoadRC(opts.RCPath, opts.Profile); err != nil {
		fatalf("rc: %v", err)
	} else if ok {
		// apply globals/profile into opts (lowest priority vs CLI/env except -r semantics)
		if opts.Format == "" && rc.Format != "" {
			opts.Format = rc.Format
		}
		if opts.GeoDB == "" && rc.GeoDB != "" {
			opts.GeoDB = rc.GeoDB
		}
		if opts.Timeout == 0 && rc.TimeoutSec > 0 {
			opts.Timeout = rc.TimeoutSec
		}
		if opts.Buffer == 0 && rc.BufferSize > 0 {
			opts.Buffer = rc.BufferSize
		}

		// connection parameters from rc (will be overridden by -r below if both set)
		if opts.IP == "" && rc.IP != "" {
			opts.IP = rc.IP
		}
		if opts.Port == 0 && rc.Port != 0 {
			opts.Port = rc.Port
		}
		if opts.Password == "" && rc.Password != "" {
			opts.Password = rc.Password
		}

		// if profile provided server_cfg – treat as BeCfg input
		if opts.BeCfg == "" && rc.ServerCfg != "" {
			opts.BeCfg = rc.ServerCfg
		}
	}

	if opts.BeCfg != "" {
		rc, err := config.LoadFromBeServerCfg(opts.BeCfg)
		if err != nil {
			fatalf("beserver cfg: %v\n", err)
		}

		opts.IP = rc.IP
		opts.Port = rc.Port
		opts.Password = rc.Password
	}

	// defaults
	if opts.IP == "" {
		opts.IP = "127.0.0.1"
	}
	if opts.Port == 0 {
		opts.Port = 2305
	}
	if opts.Timeout == 0 {
		opts.Timeout = 3
	}
	if opts.Buffer == 0 {
		opts.Buffer = 1024
	}
	if opts.Format == "" {
		opts.Format = "table"
	}

	if opts.Password == "" {
		fatalf("RCON password must be specified")
	}

	if len(args) < 1 {
		fatalf("Command must be provided")
	}

	// map legacy --json to --format=json
	format := printer.FormatFromString(opts.Format)
	if opts.JSON {
		format = printer.FormatJSON
	}

	addr := fmt.Sprintf("%s:%d", opts.IP, opts.Port)
	conn, err := bercon.Open(addr, opts.Password)
	if err != nil {
		fatalf("error opening connection: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fatalf("cant close connection: %v", err)
		}
	}()

	// setup bercon params
	conn.SetDeadlineTimeout(opts.Timeout)
	conn.SetBufferSize(opts.Buffer)

	// execute command
	for i, cmd := range args {
		data, err := conn.Send(cmd)
		if err != nil {
			fatalf("error in command %d '%s': %v", i, cmd, err)
		}
		if err := printer.ParseAndPrintData(os.Stdout, data, cmd, opts.GeoDB, format); err != nil {
			fatalf("cant print response data: %v", err)
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
