// Command-line entrypoint for bercon-cli: parses flags, runs RCON commands,
// and prints results in the selected output format.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon-cli/internal/printer"
	"github.com/woozymasta/bercon-cli/internal/vars"
	"github.com/woozymasta/bercon-cli/pkg/bercon"
)

// CLI options
type Options struct {
	IP       string `short:"i" long:"ip"          description:"Server IPv4 address" default:"127.0.0.1" env:"BERCON_ADDRESS"`
	Password string `short:"P" long:"password"    description:"Server RCON password" env:"BERCON_PASSWORD"`
	BeCfg    string `short:"r" long:"server-cfg"  description:"Path to beserver_x64.cfg file or directory to search beserver_x64*.cfg" env:"BERCON_SERVER_CFG"`
	GeoDB    string `short:"g" long:"geo-db"      description:"Path to Country GeoDB mmdb file" env:"BERCON_GEO_DB"`
	Format   string `short:"f" long:"format"      description:"Output format" choice:"json" choice:"table" choice:"raw" choice:"md" choice:"html" default:"table" env:"BERCON_FORMAT"`
	Port     int    `short:"p" long:"port"        description:"Server RCON port" default:"2305" env:"BERCON_PORT"`
	Timeout  int    `short:"t" long:"timeout"     description:"Deadline and timeout in seconds" default:"3" env:"BERCON_TIMEOUT"`
	Buffer   uint16 `short:"b" long:"buffer-size" description:"Buffer size for RCON connection" default:"1024" env:"BERCON_BUFFER_SIZE"`
	JSON     bool   `short:"j" long:"json"        description:"Print result in JSON format (deprecated, use --format=json)" env:"BERCON_JSON_OUTPUT"`
	Help     bool   `short:"h" long:"help"        description:"Show version, commit, and build time"`
	Version  bool   `short:"v" long:"version"     description:"Prints this help message"`
}

func main() {
	opts := &Options{}
	p := flags.NewParser(opts, flags.PassDoubleDash|flags.PrintErrors|flags.PassAfterNonOption)
	p.Usage = "[OPTIONS] command [command, ...]"
	p.LongDescription = "BattlEye RCon CLI."
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

	if opts.BeCfg != "" {
		rc, err := config.LoadFromBeServerCfg(opts.BeCfg)
		if err != nil {
			fatalf("beserver cfg: %v\n", err)
		}

		opts.IP = rc.IP
		opts.Port = rc.Port
		opts.Password = rc.Password
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
