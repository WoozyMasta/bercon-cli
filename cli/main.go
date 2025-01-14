package main

import (
	"fmt"
	"os"
	"path/filepath"

	"internal/vars"

	"github.com/jessevdk/go-flags"
	"github.com/woozymasta/bercon-cli/internal/printer"
	"github.com/woozymasta/bercon-cli/pkg/bercon"
)

var (
	Version   string // Version of application (git tag)
	Commit    string // Current git commit
	BuildTime string // Time of start build app
	URL       string // URL to repository
)

// CLI options
type Options struct {
	// server IPv4 address
	IP string `short:"i" long:"ip" description:"Server IPv4 address" default:"127.0.0.1" env:"BERCON_ADDRESS"`
	// server RCON password
	Password string `short:"P" long:"password" description:"Server RCON password" env:"BERCON_PASSWORD"`
	// path to Country GeoDB mmdb file
	GeoDB string `short:"g" long:"geo-db" description:"Path to Country GeoDB mmdb file" env:"BERCON_GEO_DB"`
	// server RCON port
	Port int `short:"p" long:"port" description:"Server RCON port" default:"2305" env:"BERCON_PORT"`
	// deadline and timeout in seconds
	Timeout int `short:"t" long:"timeout" description:"Deadline and timeout in seconds" default:"3" env:"BERCON_TIMEOUT"`
	// buffer size for RCON connection
	Buffer uint16 `short:"b" long:"buffer-size" description:"Buffer size for RCON connection" default:"1024" env:"BERCON_BUFFER_SIZE"`
	// print result in JSON format
	JSON bool `short:"j" long:"json" description:"Print result in JSON format" env:"BERCON_JSON_OUTPUT"`
	// server IPv4 address
	Help bool `short:"h" long:"help" description:"Show version, commit, and build time"`
	// server IPv4 address
	Version bool `short:"v" long:"version" description:"Prints this help message"`
}

func main() {
	opts := &Options{}
	p := flags.NewParser(opts, flags.PassDoubleDash|flags.PrintErrors|flags.PassAfterNonOption)

	p.Usage = "[OPTIONS] command [command, command, ...]"
	p.LongDescription = "BattlEye RCon CLI."
	p.Command.Name = filepath.Base(p.Command.Name)

	args, err := p.Parse()
	if err != nil {
		os.Exit(0)
	}

	if opts.Help {
		p.WriteHelp(os.Stdout)
		os.Exit(0)
	}
	if opts.Version {
		printVersion()
	}
	if len(opts.Password) == 0 {
		fatal("required flag '-P, --password' was not specified")
	}
	if len(args) < 1 {
		fatal("Command must be provided")
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

		if err := printer.ParseAndPrintData(data, cmd, opts.GeoDB, opts.JSON); err != nil {
			fatalf("cant print response data: %v", err)
		}
	}
}

func printVersion() {
	fmt.Printf(`
file:     %s
version:  %s
commit:   %s
built:    %s
project:  %s
`, os.Args[0], vars.Version, vars.Commit, vars.BuildTime, vars.URL)
	os.Exit(0)
}

func fatal(a ...any) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(1)
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}
