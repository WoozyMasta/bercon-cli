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

[0.2.0]: https://github.com/WoozyMasta/bercon-go/releases/tag/v0.2.0

## [0.1.1][] - 2024-12-08

### Added

* Windows manifest and icon for binary exe
* Scan release binaries on VirusTotal

### Changed

* Disabled UPX packer for Windows binaries to prevent false
  positives from some antivirus

[0.1.1]: https://github.com/WoozyMasta/bercon-go/releases/tag/v0.1.1

## [0.1.0][] - 2024-11-28

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/bercon-go/releases/tag/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
