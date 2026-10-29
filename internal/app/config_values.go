// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon-cli/internal/output"
	"github.com/woozymasta/bercon/beconfig"
)

// applyConfigValues seeds options before parsing so ConfiguredValues preserves the selected server.
func applyConfigValues(opts *options, rc config.RC, serverConfig *beconfig.Config) {
	if rc.IP != "" {
		opts.Connection.IP = rc.IP
	}
	if rc.Port != 0 {
		opts.Connection.Port = rc.Port
	}
	if rc.Password != "" {
		opts.Connection.Password = rc.Password
	}
	if rc.Timeout > 0 {
		opts.Connection.Timeout = rc.Timeout
	}
	if rc.GeoDB != "" {
		opts.Geo.DB = rc.GeoDB
	}
	if rc.GeoDBURL != "" {
		opts.Geo.URL = rc.GeoDBURL
	}
	if rc.GeoDBURLAuth != "" {
		opts.Geo.URLAuth = rc.GeoDBURLAuth
	}
	if rc.Format != "" {
		opts.Output.Format = output.Format(rc.Format)
	}
	if rc.ServerCfg != "" {
		opts.Profile.ServerCfg = rc.ServerCfg
	}

	if serverConfig != nil {
		address := serverConfig.LocalAddrPort()
		if address.Addr().IsValid() {
			opts.Connection.IP = address.Addr().String()
		}
		if address.Port() != 0 {
			opts.Connection.Port = int(address.Port())
		}
		if serverConfig.RConPassword != "" {
			opts.Connection.Password = serverConfig.RConPassword
		}
	}
}
