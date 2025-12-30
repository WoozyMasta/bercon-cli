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

|  Arch/OS  |      MacOS      |      Linux      |      Windows      |
| --------- | --------------- | --------------- | ----------------- |
| **AMD64** | [MacOS amd64][] | [Linux amd64][] | [Windows amd64][] |
| **ARM64** | [MacOS arm64][] | [Linux arm64][] | [Windows arm64][] |

For Linux you can also use the command

```bash
curl -#SfLo /usr/bin/bercon-cli \
  https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-amd64
chmod +x /usr/bin/bercon-cli
bercon-cli -h && bercon-cli -v
```

## Parameters

This help information is available when running the program
with the `--help` flag

```bash
bercon-cli --help
```

```txt
Usage:
  bercon-cli [OPTIONS] command [command, command, ...]

BattlEye RCon CLI — command-line tool for interacting with BattlEye RCON servers (used by DayZ, Arma 2/3, etc).
It allows executing server commands, reading responses, and formatting results in table, JSON, Markdown, or HTML.

Configuration can be provided via:
- CLI flags
- Environment variables
- RC config file (INI) with globals and profiles
- beserver_x64*.cfg — to auto-load RConIP, RConPort and RConPassword

Application Options:
  -i, --ip=                             Server IPv4 address (default: 127.0.0.1) [$BERCON_ADDRESS]
  -P, --password=                       Server RCON password [$BERCON_PASSWORD]
  -c, --config=                         Path to rc file (INI). If not set, standard locations are used [$BERCON_CONFIG]
  -n, --profile=                        Profile name from rc file [$BERCON_PROFILE]
  -r, --server-cfg=                     Path to beserver_x64.cfg file or directory to search beserver_x64*.cfg [$BERCON_SERVER_CFG]
  -g, --geo-db=                         Path to Country GeoDB mmdb file [$BERCON_GEO_DB]
  -f, --format=[json|table|raw|md|html] Output format (default: table) [$BERCON_FORMAT]
  -p, --port=                           Server RCON port (default: 2305) [$BERCON_PORT]
  -t, --timeout=                        Deadline and timeout in seconds (default: 3) [$BERCON_TIMEOUT]
  -b, --buffer-size=                    Buffer size for RCON connection (default: 1024) [$BERCON_BUFFER_SIZE]
  -j, --json                            Print result in JSON format (deprecated, use --format=json) [$BERCON_JSON_OUTPUT]
  -l, --list-profiles                   List profiles from rc file and exit
  -e, --example                         Print example rc (INI) config and exit
  -h, --help                            Show version, commit, and build time
  -v, --version                         Prints this help message
```

You can use environment variables, they are specified in the help in
square brackets `[]`, and are also listed in the file
[example.env](example.env)

RCON connection parameters (`RConIP`, `RConPort`, `RConPassword`)
from the `beserver_x64*.cfg` file used by BattlEye.  
To do this, specify the file or directory path with the `--beserver-cfg`
(`-r`) option or the environment variable `BERCON_SERVER_CFG`.
If a directory is specified, the tool will automatically find the active
or latest config file (e.g. `beserver_x64_active_*.cfg`).

## Usage Examples

You can use arguments, variables, or a combination of both

```bash
bercon-cli -p 2306 -P myPass players
BERCON_PASSWORD=myPass BERCON_PORT=2306 bercon-cli players
BERCON_PASSWORD=myPass bercon-cli -p 2306 players
```

The argument value has the highest priority over the environment variable,
but the BattlEye config overrides both.

```bash
# password from arg will be used
BERCON_PASSWORD='strong' bercon-cli --password 'pas$$word' players
# password from config will be used
BERCON_PASSWORD='strong' bercon-cli -r beserver_x64.cfg players
# password from config will be used
BERCON_PASSWORD='strong' bercon-cli -P 'pas$$word' -r beserver_x64.cfg players
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

## Profiles config (INI)

`bercon-cli` supports reusable configuration profiles stored in an INI file,
allowing you to define multiple RCON connections and global defaults.

The tool automatically looks for an RC config file in platform-specific
locations if `--config` (`-c`) is not provided:

* **Linux/BSD:**
  `~/.config/bercon-cli/config.ini`,
  `~/.bercon-cli.ini`
* **macOS:**
  `~/Library/Application Support/bercon-cli/config.ini`,
  plus Linux paths
* **Windows:** `%APPDATA%\bercon-cli\config.ini`,
  `%USERPROFILE%\.config\bercon-cli\config.ini`,
  `%USERPROFILE%\.bercon-cli.ini`

Configuration priority (lowest -> highest):

```txt
RC [globals] -> RC [profile.*] -> Environment -> CLI flags -> beserver_x64*.cfg
```

If a profile specifies `server_cfg`, it overrides `ip`, `port`, and `password`
with values parsed from the active `beserver_x64*.cfg`.

### Example `config.ini`

```ini
[globals]
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

[profile.arma3-test]
server_cfg = C:\Games\Arma3Server\battleye
timeout = 5
```

### Usage examples

```bash
# Use a profile from the default config locations
bercon-cli --profile dayz-local players
# Explicitly specify a config file
bercon-cli --config ~/.config/bercon-cli/config.ini -n arma3-test players
# List all profiles with resolved IP/Port
bercon-cli --list-profiles
# Print example config
bercon-cli --example
```

## Geo IP

If you specify the path to the GeoIP city database in `mmdb` format,
the `Country` column with the short country code or the `country` key
in json format will be added to the output.  
GeoIP processing supported for **Players**, **IP Bans** and **Admins**.

```bash
# pass as flag
bercon-cli -p 2306 -P myPass -g /path/to/GeoLite2-Country.mmdb players
# or as variable
BERCON_GEO_DB=/pat/to/GeoLite2-Country.mmdb BERCON_PASSWORD='myPass' bercon-cli -p 2306 players
```

> [!TIP]  
> An empty value in `--geo-db`, `-g` or `BERCON_GEO_DB`
> will disable geoip processing.

Below are examples of responses:

```txt
╭───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Players on server (17 in total)                                                                                                                                       │
├────┬─────────────────┬───────┬──────┬──────────────────────────────────┬────────────────────────┬───────┬───────┬─────────┬──────────────────┬───────────┬────────────┤
│  # │ IP              │  Port │ Ping │ GUID                             │ Name                   │ Valid │ Lobby │ Country │ City             │ Lat       │ Lon        │
├────┼─────────────────┼───────┼──────┼──────────────────────────────────┼────────────────────────┼───────┼───────┼─────────┼──────────────────┼───────────┼────────────┤
│  0 │ 175.78.137.224  │ 46534 │   33 │ 20501A3C348F41D8B7AC3F4D1BB2B11C │ Avtonom Fedenko        │ true  │ false │ CN      │                  │ 34.77320  │ 113.72200  │
│  1 │ 99.245.38.37    │ 31924 │  156 │ 8DA159D526C95D590303BF5DE422D044 │ Budislav Dovgalyuk     │ true  │ false │ CA      │ Mississauga      │ 43.56390  │ -79.71720  │
│  2 │ 213.242.6.7     │ 29653 │  274 │ 090B1EAD1075519FC30942580067EB48 │ Vernislav Moyseienko   │ true  │ true  │ RU      │ Teykovo          │ 56.85650  │ 40.53460   │
│  3 │ 14.186.90.206   │ 48687 │   16 │ 2E1589F4CF2E3EF553A4DA9F6C2ADB4C │ Radimir Sosnovskiy     │ true  │ false │ VN      │ Ho Chi Minh City │ 10.82200  │ 106.62570  │
│  4 │ 5.64.182.98     │ 33898 │  272 │ EE77B327B8ADB31B15F058FD9DAA13BD │ Ladolyub Semchuk       │ true  │ false │ GB      │ Plymouth         │ 50.39070  │ -4.06020   │
│  5 │ 109.137.98.204  │ 49085 │  185 │ BF14CC820DAA9BB3293DE24FBE75E7F8 │ Osemrit Fesun          │ true  │ false │ BE      │ Antwerp          │ 51.21920  │ 4.39170    │
│  6 │ 135.18.149.138  │ 23544 │  282 │ 8A9A96D2FA5466CB8BAA5951711FE028 │ Ulichan Venislavskiy   │ true  │ false │ US      │ Chicago          │ 41.88350  │ -87.63050  │
│  7 │ 42.59.192.137   │ 49326 │  243 │ 3C32AC2FF50AE08EF000D7E88CDE0C47 │ Snovid Poloviy         │ true  │ false │ CN      │                  │ 34.77320  │ 113.72200  │
│  8 │ 213.229.129.71  │ 25891 │  235 │ 062E3F4183B90DA6534AA738FB4CE501 │ Hristofor Kirpan       │ true  │ false │ ES      │ Madrid           │ 40.41530  │ -3.69400   │
│  9 │ 136.152.138.124 │ 21192 │  148 │ 33C81E25895ADCE458AA22F2A55D668A │ Hvalimir Chamata       │ true  │ false │ US      │ Berkeley         │ 37.87360  │ -122.25700 │
│ 10 │ 76.4.61.28      │ 29028 │   94 │ D5F7BC1D4D113C180AE2A6A18C3E40CF │ Solomon Haieckiy       │ false │ false │ US      │                  │ 37.75100  │ -97.82200  │
│ 11 │ 132.181.34.254  │ 50362 │  120 │ 4F069086BDAB40183121F0CA2F6F7E34 │ Gorun Yashchenko       │ true  │ false │ NZ      │ Christchurch     │ -43.52340 │ 172.59900  │
│ 12 │ 176.247.54.236  │ 46487 │   23 │ 38DF0773821E8D0A1BDAB620981302E8 │ Yarosvit Tobilevich    │ true  │ false │ IT      │ Cagliari         │ 39.23020  │ 9.12100    │
│ 13 │ 5.252.240.44    │ 48936 │  227 │ 256D87ED2B7D0ADB664B372C297E1B4D │ Virodan Bogovin        │ true  │ false │ IT      │ Melegnano        │ 45.35690  │ 9.32670    │
│ 14 │ 2.153.187.194   │ 42007 │  181 │ 796CAA8F77E01D48B2D4E91F156E4387 │ Naslav Mazurok         │ true  │ false │ ES      │ Madrid           │ 40.34820  │ -3.69890   │
│ 15 │ 39.127.252.69   │ 44989 │  106 │ CFBAC3F0F22C492FA238D9ED159F3E6C │ Vodogray Zhigalko      │ true  │ false │ KR      │ Yongin-si        │ 37.27280  │ 127.11180  │
│ 16 │ 125.202.166.119 │ 31839 │  277 │ ADD6FEB25F352F0F6C01F0731E49EF43 │ Toligniv Doshchenko    │ true  │ false │ JP      │ Hachinohe        │ 40.50260  │ 141.48540  │
╰────┴─────────────────┴───────┴──────┴──────────────────────────────────┴────────────────────────┴───────┴───────┴─────────┴──────────────────┴───────────┴────────────╯
```

```json
[
  {
    "ip": "213.229.129.71",
    "guid": "062E3F4183B90DA6534AA738FB4CE501",
    "name": "Hristofor Kirpan",
    "country": "ES",
    "city": "Madrid",
    "lat": 40.41530,
    "lon": -3.69400,
    "port": 25891,
    "ping": 33,
    "id": 8,
    "valid": true,
    "lobby": false
  }
]
```

> [!TIP]  
> `XX` country code is used for local addresses and other cases when it
> was not possible to get data from the GeoIP DB

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
    bercon-cli -t 1 -- "$@";
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
    ((timer-=step)) || :
    dayz-all-rcon "say -1 Restart server after $timer seconds"
  done

  dayz-all-rcon 'kick -1'
  sleep "$step"
  dayz-all-rcon '#shutdown'
}

# restart all servers after 120 (default) seconds
dayz-all-restart
# restart all servers after 360 seconds and send messages every 20 seconds
dayz-all-restart 360 20
```

## 👉 [Support Me](https://gist.github.com/WoozyMasta/7b0cabb538236b7307002c1fbc2d94ea)

Your support is greatly appreciated!

<!-- Links -->
[logo]: assets/bercon.png
[BattlEye]: https://www.battleye.com/ "BattlEye – The Anti-Cheat Gold Standard"
[BERConProtocol]: pkg/bercon/spec/bercon-protocol.md "BattlEye RCON Protocol Specification"
[MacOS arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-darwin-arm64 "MacOS arm64 file"
[MacOS amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-darwin-amd64 "MacOS amd64 file"
[Linux amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-amd64 "Linux amd64 file"
[Linux arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-arm64 "Linux arm64 file"
[Windows amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-windows-amd64.exe "Windows amd64 file"
[Windows arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-windows-arm64.exe "Windows arm64 file"
