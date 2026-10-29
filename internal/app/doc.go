// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

/*
Package app assembles configuration, RCon sessions,
response routing, and output modes for bercon-cli.
A run resolves defaults in precedence order, owns one client,
and gives the message broker the sole transport-message reader.
Recognized replies can satisfy structured commands;
watch and shell modes expose other server messages, while exec ignores them.
*/
package app
