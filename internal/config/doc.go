// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

/*
Package config reads bercon-cli's INI defaults and named connection profiles.
It merges profile values over globals and leaves final precedence to the flags parser,
so environment variables and explicit arguments remain authoritative.
Timeout values use [Duration], which accepts Go duration strings
and keeps bare integers in seconds for existing profiles.
Profile listings expose endpoints but never credentials.
*/
package config
