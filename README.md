# bercon-cli

![logo][]

`./bercon-cli` is the command line interface and go package for the
**BattlEye RCON** protocol

## Description

`bercon` provides a convenient way to interact with the server using the
BattlEye RCON (Remote Console) protocol.
This tool allows you to execute various commands,
control the server, and track responses from the server.

It is suitable for such servers as Arma2, Arma3, DayZ, etc. using the
protocol [BERConProtocol][], with a full list of games you can
check out the full list of games on the [BattlEye][] website

## Installation

You can download the latest version of the programme by following the links:

* [MacOS arm64][]
* [MacOS amd64][]
* [Linux i386][]
* [Linux amd64][]
* [Linux arm][]
* [Linux arm64][]
* [Windows i386][]
* [Windows amd64][]
* [Windows arm64][]

For Linux you can also use the command

```bash
curl -#SfLo /usr/bin/bercon-cli \
  https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-amd64
chmod +x /usr/bin/bercon
bercon-cli -h && bercon-cli -v
```

## Parameters

This help information is available when running the program
with the `--help` flag

```bash
bercon-cli --help
```

```txt
NAME:
   bercon-cli - BattlEye RCon CLI

USAGE:
   bercon-cli [options] command [command, command, ...]

OPTIONS:
   --ip value, -i value           server IPv4 address (default: "127.0.0.1") [$BERCON_ADDRESS]
   --port value, -p value         server RCON port (default: 2305) [$BERCON_PORT]
   --password value, -P value     server RCON password [$BERCON_PASSWORD]
   --json, -j                     print result in JSON format (default: false) [$BERCON_JSON_OUTPUT]
   --timeout value, -t value      deadline and timeout in seconds (default: 5) [$BERCON_TIMEOUT]
   --buffer-size value, -b value  buffer size for RCON connection (default: 1024) [$BERCON_BUFFER_SIZE]
   --log-level value, -l value    log level (trace, debug, info, warn, error) (default: "error") [$BERCON_LOG_LEVEL]
   --version, -v                  print version (default: false)
   --help, -h                     show help
```

You can also use environment variables, they are specified in the help in
square brackets `[]`, and are also listed in the file
[example.env](example.env)

## Usage Examples

You can use arguments, variables, or a combination of both

```bash
bercon-cli -p 2306 -P myPass players
BERCON_PASSWORD=myPass BERCON_PORT=2306 bercon-cli players
BERCON_PASSWORD=myPass bercon-cli -p 2306 players
```

The argument value has the highest priority over the environment variable

```bash
# pas$$word will be used
BERCON_PASSWORD='strong' bercon-cli --password 'pas$$word' players
```

You can pass multiple commands within a single context.
If a command consists of multiple words separated by spaces,
or contains `#` or `-` characters, you must enclose them in quotes.
You can also explicitly separate commands after flags using `--`.

```bash
bercon-cli --ip 192.168.0.10 --port 2306 --password 'pas$$word' -- '#unlock'
bercon-cli -t 1 -i 192.168.0.10 -p 2306 -P 'pas$$word' '#shutdown'
bercon-cli -i 192.168.0.10 -p 2306 -P 'pas$$word' -- '#lock' 'say -1 server restart in 5 min'
```

You can use json output for further processing

```bash
bercon-cli -p 2306 -P myPass -j players | jq -er .
```

## More useful bash examples

You can also use variables to store parameters for
different servers in different files

```bash
# in the ~/.server-1.env file
BERCON_IP=192.168.0.10
BERCON_PORT=2306
BERCON_PASSWORD='pas$$word'.

# read the file and execute the command
. .server-1.env && bercon-cli players
```

An example function that will allow you to execute commands on several of your
DayZ servers at the same time

> [!TIP]  
> Functions can be placed in `~/.bashrc` for quick access to them

```bash
export DAYZ_SERVERS_COUNT=5

dayz-all-rcon() {
  for i in $(seq 1 "$DAYZ_SERVERS_COUNT"); do
    printf '[$s] ' "Server-$i"
    . "~/.server-$i.env".
    bercon-cli -t 1 -- $@;
    echo
  done
}

# show players on all servers
dayz-all-rcon players
```

This example will allow you to conveniently perform a delayed restart on all
DayZ servers at the same time, notifying players that a restart is imminent

> [!TIP]  
> This example recycles the function from the previous example

```bash
dayz-all-restart() {
  local timer="${1:-120}" step="${2:-10}"
  dayz-all-rcon \
    '#lock' \
    "say -1 Server locked for new connection, restart after $timer seconds"
  for i in $(seq "$timer" "-$step" 0); do
    sleep "$step"
    dayz-all-rcon "say -1 Restart server after $timer seconds"
  done
  dayz-all-rcon '#shutdown'
}

# restart all servers after 120 (default) seconds
dayz-all-restart
# restart all servers after 360 seconds
dayz-all-restart 360
```

## Support me 💖

If you enjoy my projects and want to support further development,
feel free to donate! Every contribution helps to keep the work going.
Thank you!

### Crypto Donations

* **BTC**: `1Jb6vZAMVLQ9wwkyZfx2XgL5cjPfJ8UU3c`
* **USDT (TRC20)**: `TN99xawQTZKraRyvPAwMT4UfoS57hdH8Kz`
* **TON**: `UQBB5D7cL5EW3rHM_44rur9RDMz_fvg222R4dFiCAzBO_ptH`

Your support is greatly appreciated!

<!-- Links -->
[logo]: assets/bercon.png
[BattlEye]: https://www.battleye.com/ "BattlEye – The Anti-Cheat Gold Standard"
[BERConProtocol]: pkg/bercon/spec/bercon-protocol.md "BattlEye RCON Protocol Specification"
[MacOS arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-darwin-arm64 "MacOS arm64 file"
[MacOS amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-darwin-amd64 "MacOS amd64 file"
[Linux i386]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-386 "Linux i386 file"
[Linux amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-amd64 "Linux amd64 file"
[Linux arm]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-arm "Linux arm file"
[Linux arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-arm64 "Linux arm64 file"
[Windows i386]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-windows-386.exe "Windows i386 file"
[Windows amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-windows-amd64.exe "Windows amd64 file"
[Windows arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-windows-arm64.exe "Windows arm64 file"
