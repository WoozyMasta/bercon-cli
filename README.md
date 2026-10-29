# bercon-cli

<!-- markdownlint-disable MD033 -->

<p align="center">
  <img src="./bercon.png" alt="bercon-cli logo" width="70%">
</p>

<!-- markdownlint-enable MD033 -->

Run one-off BattlEye RCON commands, automate routine tasks,
and follow live server activity from your terminal.

## Description

Send commands supported by the server,
receive command replies and live server messages,
and choose how responses are formatted: tables,
JSON, CSV, Markdown, HTML, or raw text.

Works with game servers that expose
the [BattlEye RCON protocol][BERConProtocol].
Built-in parsers recognize response formats from
DayZ, Arma 2, Arma 3, and Arma Reforger;
available commands are defined by each server.

## Installation

Choose a release for your operating system and architecture:

Architecture | macOS           | Linux           | Windows
------------ | --------------- | --------------- | -----------------
**AMD64**    | [macOS amd64][] | [Linux amd64][] | [Windows amd64][]
**ARM64**    | [macOS arm64][] | [Linux arm64][] | [Windows arm64][]

<!-- markdownlint-disable MD033 -->

<details>
  <summary>Download the latest release with curl in Bash:</summary>

```shell
case "$(uname -s)" in
  Linux*) OS=linux;;
  Darwin*) OS=darwin;;
  MINGW*|MSYS*|CYGWIN*) OS=windows; EXT=.exe;;
  *) exit 1;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64;;
  aarch64|arm64) ARCH=arm64;;
  *) exit 1;;
esac

BIN="./bercon-cli$EXT"
REPO='https://github.com/WoozyMasta/bercon-cli'

curl -#SfLo "$BIN" "$REPO/releases/latest/download/bercon-cli-$OS-$ARCH$EXT"

chmod +x "$BIN"
"$BIN" -h && "$BIN" -v

# Optionally move the binary to some PATH dir:
mv "$BIN" /usr/local/bin/
```

</details>

<!-- markdownlint-enable MD033 -->

## Go library

The CLI uses [bercon][] for its BattlEye RCON client and game response parsers.

> [!IMPORTANT]
> If your Go project previously imported packages from this repository,
> use [`github.com/woozymasta/bercon`][bercon] for the library API.

## Usage

Run `bercon-cli --help` for an overview of commands and options.
The complete reference is in [CLI.md][CLI].

Generate documents with the built-in `docs` command:

```shell
# Generate Markdown, HTML, and man-page references.
bercon-cli docs md --toc --template=table bercon.md
bercon-cli docs html --toc --template=styled bercon.html
bercon-cli docs man bercon.man
```

## Quick start

### Send a command

Pass connection settings and a command supported by the server:

```shell
bercon-cli exec --ip 192.0.2.10 --port 2305 --password '<RCON_PASSWORD>' players
```

## Configuration and profiles

Settings can be provided in several places.
If the same setting comes from multiple sources,
the value from the source listed later takes precedence:

1. Built-in default values
2. Variables loaded from the `.env` file
3. Environment variables set in your shell or service
4. Shared settings in the INI configuration file (`[globals]` section)
5. Settings in the selected server profile in that INI file
6. Connection settings from the BattlEye config
7. Command-line flags

### Environment variables and `.env` file

The CLI loads `.env` from the current directory by default.
Use `--env-file` to choose another file or `--no-env` to disable loading it.
Variables already set in your shell or service take precedence over `.env`;
`--env-override` reverses the order between those two sources.
See [example.env][env] for the available variable names.

### INI configuration and profiles

An INI configuration file can define settings shared by all profiles
in the `[globals]` section and per-server settings in `[profile.NAME]` sections.
Choose a profile with `--profile NAME`.

Pass `--config PATH` to select an INI file.
Otherwise, it looks for the default INI file in the standard
user configuration directory for the current operating system:

* **Linux/BSD:** `$XDG_CONFIG_HOME/bercon-cli/config.ini` or
  `~/.config/bercon-cli/config.ini`.
* **macOS:** `~/Library/Application Support/bercon-cli/config.ini`.
* **Windows:** `%APPDATA%\bercon-cli\config.ini`.

The [CLI reference][CLI] lists the full search order
and environment-variable fallbacks.

Example INI config file:

```ini
[globals]
port = 2305
timeout = 3
format = table

[profile.dayz-local]
ip = 192.0.2.10
password = replace-me
geo_db = /srv/geoip/GeoLite2-City.mmdb

[profile.arma3-eu]
server_cfg = /srv/arma3/battleye
timeout = 5s
```

```shell
# Run a command using the selected profile.
bercon-cli exec --profile dayz-local players
# Select a config file explicitly and run another profile.
bercon-cli exec --config ./servers.ini --profile arma3-eu players
# List profiles from the default config file.
bercon-cli --list-profiles
# Print a sample INI configuration.
bercon-cli --example
```

### BattlEye server configuration

Provide a BattlEye config file or directory with `--server-cfg`
or the `server_cfg` setting in a profile.
The CLI reads `RConIP`, `RConPort`, and `RConPassword` from the file;
for a directory, it selects the active or latest matching `beserver_x64*.cfg`.
These connection settings override values from the profile and environment.
Command-line flags take precedence over them.

### Duration values

Duration options accept Go-style values such as `3s` and `250ms`
in CLI flags, environment variables, `.env`, and INI settings.
Values without a unit use the option's default unit:
seconds for timeout, keepalive, and loop sleep;
milliseconds for command sleep.

For example, `--timeout 3` means 3 seconds,
while `--cmd-sleep 3` means 3 milliseconds.

## Typical workflows

### Send a command batch

Use `--` to mark the end of CLI options and pass multiple server commands.
Quote commands that contain spaces or shell-significant characters:

```shell
bercon-cli exec --profile dayz-local -- \
  '#lock' 'say -1 Server restart in 5 minutes'
```

### Repeat a batch

`--repeat` repeats the whole batch.
`--cmd-sleep` sets the delay between commands in a batch;
`--loop-sleep` sets the delay between batches.

```shell
bercon-cli exec --profile dayz-local --repeat 3 \
  --cmd-sleep 250ms --loop-sleep 30s -- \
  players 'say -1 Server status check'
```

### Start an interactive session

Use `shell` to enter commands interactively and see each server response:

```shell
bercon-cli shell --profile dayz-local
```

<!-- markdownlint-disable MD033 -->

Press <kbd>Ctrl</kbd>+<kbd>D</kbd>
or <kbd>Ctrl</kbd>+<kbd>C</kbd> to close the session.
By default, `exit` and `quit` also work as built-in commands;
`--no-builtins` disables them.

<!-- markdownlint-enable MD033 -->

### Follow server messages

<!-- markdownlint-disable MD033 -->

Use `watch` to follow server messages.
It runs until you stop it with <kbd>Ctrl</kbd>+<kbd>C</kbd> ;
`--timestamps` prefixes each message with the local time:

<!-- markdownlint-enable MD033 -->

```shell
bercon-cli watch --profile dayz-local --timestamps >> dayz-watch.log
```

### Process structured output

Select JSON output when piping recognized server responses to another tool:

```shell
bercon-cli exec --profile dayz-local --format json players | jq .
```

## GeoIP

GeoIP optionally adds location fields
to parsed server responses that contain an IP address.
A country database provides country codes;
a city database can also provide city names and coordinates.
Responses without an IP field cannot be enriched.
Set `--geo-db` to use a local database.
Add `--geo-db-url` to download it when missing;
once the database exists, update checks run at most once per day.

```shell
# Use an existing database.
bercon-cli exec --profile dayz-local \
  --geo-db ./GeoLite2-City.mmdb players
```

For regular use, get a download URL from MaxMind.
This public mirror can be used to try the download option:

```shell
# Download the database if needed and enable daily update checks.
bercon-cli exec --profile dayz-local \
  --geo-db ./GeoLite2-City.mmdb \
  --geo-db-url https://git.io/GeoLite2-City.mmdb players
```

For MaxMind downloads, set `BERCON_GEO_DB_URL_AUTH` to `ACCOUNT_ID:LICENSE_KEY`
in the environment or a `.env` file excluded from version control.

An empty `--geo-db-url=` disables update checks but keeps local lookups enabled.
An empty `--geo-db` disables GeoIP.

## 👉 [Support Me](https://gist.github.com/WoozyMasta/7b0cabb538236b7307002c1fbc2d94ea)

Your support is greatly appreciated!

<!-- Links -->
[CLI]: ./CLI.md
[bercon]: https://github.com/woozymasta/bercon
[env]: ./example.env
[BERConProtocol]: https://www.battleye.com/downloads/BERConProtocol.txt
[macOS arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-darwin-arm64 "macOS arm64 file"
[macOS amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-darwin-amd64 "macOS amd64 file"
[Linux amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-amd64 "Linux amd64 file"
[Linux arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-linux-arm64 "Linux arm64 file"
[Windows amd64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-windows-amd64.exe "Windows amd64 file"
[Windows arm64]: https://github.com/WoozyMasta/bercon-cli/releases/latest/download/bercon-cli-windows-arm64.exe "Windows arm64 file"
