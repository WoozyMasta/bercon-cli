# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.4.3][] - 2025-12-10

### Changed

* Fixed issue where `Send()` would block indefinitely if the connection
  manager loop crashed or became unresponsive.
* Fixed race conditions when calling `Close()` multiple times.
* Decoupled message handling into a dedicated `dispatchLoop`.
* Internal request channels are now buffered to reduce latency
  during high-load command sending.
* Improved `IsAlive()` accuracy. It now tracks the last received packet
  time and detects dropped connections without extra network overhead.

[0.4.3]: https://github.com/WoozyMasta/bercon-cli/compare/v0.4.2...v0.4.3

## [0.4.2][] - 2025-10-09

### Changed

* Fixed configuration priority

[0.4.2]: https://github.com/WoozyMasta/bercon-cli/compare/v0.4.1...v0.4.2

## [0.4.1][] - 2025-10-08

### Added

* RC file support `--config` (`-c`) flag to load INI config with
  `[globals]` and `[profile.*]` sections.
* Profile selection `--profile` (`-n`) flag to select a named profile
  from RC file.
* Profile listing `--list-profiles` (`-l`) flag to show available
  profiles in a formatted table.
* Config discovery auto-detected in platform-specific paths:
  * Linux and others: `~/.config/bercon-cli/config.ini`, `~/.bercon-cli.ini`
  * macOS: `~/Library/Application Support/bercon-cli/config.ini`
  * Windows: `%APPDATA%\bercon-cli\config.ini`
* `--server-cfg` (`-r`) option to auto-load settings from `beserver_x64*.cfg`.
  Supports both file and directory paths,
  automatically picks active or latest config.
* `--example` (`-e`) flag to print an example RC file.

### Changed

* CLI now merges configuration in layered order:
  `CLI > Env > RC file (globals/profile) > beserver_x64*.cfg`.
* Reworked table rendering for Players and Admins:
  * Split combined `IP:Port` into separate **IP** and **Port** columns.
  * Added missing **Valid** and **Lobby** columns for consistency.

[0.4.1]: https://github.com/WoozyMasta/bercon-cli/compare/v0.4.0...v0.4.1

## [0.4.0][] - 2025-09-30

### Added

* CLI: new `--format` flag (`table`, `json`, `plain`, `md`, `html`).
* printer: pretty tables via go-pretty
* printer: Markdown/HTML rendering
* beparser: geo enrichment: `country`, `city`, `lat`, `lon`.
* bercon: duration-based setters and getters (`SetKeepalive`, `Keepalive`,
  `SetDeadline`, `Deadline`, `SetMicroSleep`, `MicroSleep`).
* Makefile (release matrix, winres patch, SBOM)

### Changed

* bercon: rewritten manager/reader loops for robustness
* bercon: strict multipart assembly
* bercon: protocol checks
* bercon: normalized errors
* bercon: enforced CRC/header validation
* bercon: max command body limit
* printer: unified rendering via `ParseAndPrintData(w, ...)`
* printer: table captions with totals.

### Migration Notes

* JSON consumers should be aware of new fields `city`, `lat`, `lon`
  (additive, non-breaking).
* CLI `--json` remains supported for backward compatibility;
  prefer `--format=json`.

[0.4.0]: https://github.com/WoozyMasta/bercon-cli/compare/v0.3.1...v0.4.0

## [0.3.1][] - 2025-01-28

### Changed

* listener now uses a waiting group and checks for connectivity in all received
  packets to prevent race conditions when reading from a message channel

[0.3.1]: https://github.com/WoozyMasta/bercon-cli/compare/v0.3.0...v0.3.1

## [0.3.0][] - 2025-01-14

Refactoring and Simplification

### Added

* new `Messages` channel in `Connection` for receiving server messages
  (authorization status, server notifications) sent by the server not
  in response to direct commands
* 32x32 and 64x64 winres icons for cli
* `.golangci.yml` config and fix linting issues
* more detailed comments in accordance with godoc

### Changed

* cli args parse now with `jessevdk/go-flags`
* removed logging from `bercon` and `bercon-cli`
* dependencies related to cli have been moved to internal packages

[0.3.0]: https://github.com/WoozyMasta/bercon-cli/compare/v0.2.0...v0.3.0

## [0.2.0][] - 2024-12-11

### Added

* Output of short country code based on GeoIP data in plain or JSON
  response format if path to `mmdb` GeoIP city database is specified
  in `--geo-db` flag, `-g` or `BERCON_GEO_DB` variable
* `ParseWithGeo` and `ParseWithGeoDB` functions in **beparser** for simple
  use with geo data
* **beprinter** package for simple response data printing
* Bill of materials for cli and binaries
* CI stage to check the alignment of go structure fields

### Changed

* Aligned fields for all go structures
* Update dependencies

[0.2.0]: https://github.com/WoozyMasta/bercon-cli/compare/v0.1.1...v0.2.0

## [0.1.1][] - 2024-12-08

### Added

* Windows manifest and icon for binary exe
* Scan release binaries on VirusTotal

### Changed

* Disabled UPX packer for Windows binaries to prevent false
  positives from some antivirus

[0.1.1]: https://github.com/WoozyMasta/bercon-cli/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2024-11-28

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/bercon-cli/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
