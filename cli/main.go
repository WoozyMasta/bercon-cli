package main

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/woozymasta/bercon-cli/pkg/beparser"
	"github.com/woozymasta/bercon-cli/pkg/bercon"
	"github.com/woozymasta/bercon-cli/pkg/config"
)

var logFormatter *log.TextFormatter

func main() {
	app := &cli.App{
		Name:      "bercon-cli",
		Usage:     "BattlEye RCon CLI",
		UsageText: "bercon-cli [options] command [command, command, ...]",
		Flags:     getFlags(),
		Action:    runApp,
		Writer:    os.Stderr,
		CustomAppHelpTemplate: `NAME:
   {{.Name}} - {{.Usage}} {{.Version}}

USAGE:
   {{.UsageText}}

OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}
`,
	}

	logFormatter = prepareLogging()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// get flags passed in cli
func getFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "ip",
			Value:   "127.0.0.1",
			Usage:   "server IPv4 address",
			Aliases: []string{"i"},
			EnvVars: []string{"BERCON_ADDRESS"},
		},
		&cli.IntFlag{
			Name:    "port",
			Value:   2305,
			Usage:   "server RCON port",
			Aliases: []string{"p"},
			EnvVars: []string{"BERCON_PORT"},
			Action: func(ctx *cli.Context, v int) error {
				if v >= 65536 {
					return fmt.Errorf("flag port value %v out of range [0-65535]", v)
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:     "password",
			Usage:    "server RCON password",
			Aliases:  []string{"P"},
			EnvVars:  []string{"BERCON_PASSWORD"},
			FilePath: ".env",
		},
		&cli.BoolFlag{
			Name:    "json",
			Usage:   "print result in JSON format",
			Aliases: []string{"j"},
			EnvVars: []string{"BERCON_JSON_OUTPUT"},
		},
		&cli.IntFlag{
			Name:    "timeout",
			Value:   bercon.DefaultDeadlineTimeout,
			Usage:   "deadline and timeout in seconds",
			Aliases: []string{"t"},
			EnvVars: []string{"BERCON_TIMEOUT"},
		},
		&cli.IntFlag{
			Name:    "buffer-size",
			Value:   bercon.DefaultBufferSize,
			Usage:   "buffer size for RCON connection",
			Aliases: []string{"b"},
			EnvVars: []string{"BERCON_BUFFER_SIZE"},
		},
		&cli.StringFlag{
			Name:    "log-level",
			Value:   "error",
			Usage:   "log level (trace, debug, info, warn, error)",
			Aliases: []string{"l"},
			EnvVars: []string{"BERCON_LOG_LEVEL"},
		},
		&cli.BoolFlag{
			Name:               "version",
			Aliases:            []string{"v"},
			Usage:              "print version",
			DisableDefaultText: true,
		},
	}
}

// run application with curent context
func runApp(cCtx *cli.Context) error {
	setupLogging(cCtx.String("loglevel"), logFormatter)

	args := cCtx.Args()

	if cCtx.Bool("version") {
		fmt.Printf("%s\n\nversion\t%s\ncommit\t%s\nbuilt\t%s\n", cCtx.App.Name, config.Version, config.Commit, config.BuildTime)
		os.Exit(0)
	}

	if args.Len() == 0 {
		_ = cli.ShowAppHelp(cCtx)
		return fmt.Errorf("no command passed")
	}

	// connect to RCON server
	ip := cCtx.String("ip")
	port := cCtx.Int("port")
	password := cCtx.String("password")

	if password == "" {
		return fmt.Errorf("no password passed")
	}

	conn, err := bercon.Open(fmt.Sprintf("%s:%d", ip, port), password)
	if err != nil {
		return fmt.Errorf("error opening connection: %v", err)
	}
	defer conn.Close()

	// setup bercon params
	conn.SetDeadlineTimeout(cCtx.Int("timeout"))
	buffersize := cCtx.Int("buffersize")
	if buffersize < 1024 {
		log.Warnf("Buffer sizes less than 1024 may be unstable")
	}
	conn.SetBufferSize(buffersize)

	// execute command
	for i, cmd := range args.Slice() {
		if err := executeCommand(conn, cmd, i, cCtx.Bool("json")); err != nil {
			return err
		}
	}

	return nil
}

// execute command on server and print response
func executeCommand(conn *bercon.Connection, cmd string, index int, printJSON bool) error {
	data, err := conn.Send(cmd)
	if err != nil {
		return fmt.Errorf("error in command %d '%s': %v", index, cmd, err)
	}

	if printJSON {
		return printJSONResponse(data, cmd)
	}

	printResponse(data)
	return nil
}

// print plain text response
func printResponse(data []byte) {
	if len(data) == 0 {
		fmt.Println("OK")
		return
	}

	fmt.Println(string(data))
}

// parse and print data as json
func printJSONResponse(data []byte, cmd string) error {
	jsonData, err := json.MarshalIndent(beparser.Parse(data, cmd), "", "  ")
	if err != nil {
		return fmt.Errorf("error converting data to JSON: %v", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

// initialize logging
func prepareLogging() *log.TextFormatter {
	formatter := log.TextFormatter{
		ForceColors:            true,
		DisableQuote:           false,
		DisableTimestamp:       true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	}

	log.SetFormatter(&formatter)
	log.SetLevel(log.InfoLevel)
	log.SetOutput(os.Stderr)

	return &formatter
}

// setup log level
func setupLogging(level string, formatter *log.TextFormatter) {
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		log.Errorf("Undefined log level %s, fallback to error level", level)
		logLevel = log.ErrorLevel
	}

	log.SetLevel(logLevel)

	if logLevel == log.DebugLevel {
		formatter.DisableTimestamp = false
		log.SetFormatter(formatter)
	}

	if logLevel == log.TraceLevel {
		formatter.DisableTimestamp = false
		formatter.FullTimestamp = true
		log.SetFormatter(formatter)
		log.SetReportCaller(true)
	}

	log.Debugf("Logger setup with level %s", level)
}
