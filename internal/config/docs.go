/*
Package config provides helpers for loading and merging configuration sources
used by bercon-cli.

It supports:
  - Loading BattlEye beserver_x64*.cfg files to extract RCON connection
    parameters (RConIP, RConPort, RConPassword).
  - Parsing and merging INI-based RC configuration with [globals] and [profile.*] sections.
  - Resolving RC config file locations automatically based on OS conventions
    (e.g. ~/.config/bercon-cli/config.ini, %APPDATA%\bercon-cli\config.ini, etc).
  - Listing available profiles and printing them in a table-friendly format.

When multiple sources are provided, the precedence is:
CLI > Environment > RC file > beserver_x64*.cfg (for connection parameters only).

This allows bercon-cli to seamlessly manage per-server RCON settings and
environment-agnostic defaults.
*/
package config
