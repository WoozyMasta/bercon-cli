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

BattlEye RCon CLI.

Application Options:
  -i, --ip=          Server IPv4 address (default: 127.0.0.1) [$BERCON_ADDRESS]
  -P, --password=    Server RCON password [$BERCON_PASSWORD]
  -g, --geo-db=      Path to Country GeoDB mmdb file [$BERCON_GEO_DB]
  -p, --port=        Server RCON port (default: 2305) [$BERCON_PORT]
  -t, --timeout=     Deadline and timeout in seconds (default: 3) [$BERCON_TIMEOUT]
  -b, --buffer-size= Buffer size for RCON connection (default: 1024) [$BERCON_BUFFER_SIZE]
  -j, --json         Print result in JSON format [$BERCON_JSON_OUTPUT]
  -h, --help         Show version, commit, and build time
  -v, --version      Prints this help message
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
Players on server:
[#]  [IP Address]:[Port]    [Ping]  [GUID]                            [Name]                  [Country]
---------------------------------------------------------------------------------------------------------
0    175.78.137.224:46534   33      20501A3C348F41D8B7AC3F4D1BB2B11C  Avtonom Fedenko         CN       
1    162.47.104.77:45539    298     A3333BB4AFBC64F07F1FA0C6C09E6746  Svitlogor Zelinka       US       
2    99.245.38.37:31924     156     8DA159D526C95D590303BF5DE422D044  Budislav Dovgalyuk      CA       
3    181.238.97.213:37703   285     DA55E95D18536F77A14C0EC70562CB20  Sergiy Filevich         AR       
4    213.242.6.7:29653      274     090B1EAD1075519FC30942580067EB48  Vernislav Moyseienko    RU       
5    14.186.90.206:48687    16      2E1589F4CF2E3EF553A4DA9F6C2ADB4C  Radimir Sosnovskiy      VN       
6    241.66.187.25:40056    198     D5D648188992BB7B4994451E70F71558  Sobislav Peleshchishin  XX       
7    5.252.240.44:48936     227     256D87ED2B7D0ADB664B372C297E1B4D  Virodan Bogovin         IT       
8    172.148.115.119:32793  141     799B37118AC27D5C345092069DAFE8B2  Gostomisl Yaskevich     GB       
9    39.127.252.69:44989    106     CFBAC3F0F22C492FA238D9ED159F3E6C  Vodogray Zhigalko       KR       
10   125.202.166.119:31839  277     ADD6FEB25F352F0F6C01F0731E49EF43  Toligniv Doshchenko     JP       
(11 players in total)
PASS
```

```json
[
  {
    "ip": "175.78.137.224",
    "guid": "20501A3C348F41D8B7AC3F4D1BB2B11C",
    "name": "Avtonom Fedenko",
    "country": "CN",
    "port": 46534,
    "ping": 33,
    "id": 0,
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

## Support me 💖

If you enjoy my projects and want to support further development,
feel free to donate! Every contribution helps to keep the work going.
Thank you!

### Crypto Donations

<!-- cSpell:disable -->
* **BTC**: `1Jb6vZAMVLQ9wwkyZfx2XgL5cjPfJ8UU3c`
* **USDT (TRC20)**: `TN99xawQTZKraRyvPAwMT4UfoS57hdH8Kz`
* **TON**: `UQBB5D7cL5EW3rHM_44rur9RDMz_fvg222R4dFiCAzBO_ptH`
<!-- cSpell:enable -->

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
