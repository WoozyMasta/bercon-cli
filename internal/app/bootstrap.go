// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"github.com/woozymasta/bercon-cli/internal/config"
	"github.com/woozymasta/bercon/beconfig"
	"github.com/woozymasta/flags"
)

// bootstrapOptions contains only values needed before RC defaults can be loaded.
type bootstrapOptions struct {
	Exec  bootstrapSelectors `command:"exec"  ini-group:"exec" default-command:"true"`
	Watch bootstrapSelectors `command:"watch" ini-group:"watch"`
	Shell bootstrapSelectors `command:"shell" ini-group:"shell"`
}

// bootstrapSelectors contains only flags needed before loading profile values.
type bootstrapSelectors struct {
	RCPath    string `long:"config"     short:"c"`
	Profile   string `long:"profile"    short:"n"`
	ServerCfg string `long:"server-cfg" short:"r"`
}

// bootstrapState holds resolved configuration used to seed the full parser.
type bootstrapState struct {
	file   *config.RCFile   // file supplies profile names to the local profile-list action.
	server *beconfig.Config // server contains stronger connection defaults from the BattlEye config.
	rc     config.RC        // rc contains defaults after merging global and selected-profile values.
	found  bool             // found reports whether an RC file was loaded.
}

// bootstrap reads only the selectors needed to load RC and BattlEye configuration.
func bootstrap(args []string) (bootstrapState, error) {
	var opts bootstrapOptions
	parser := flags.NewParser(
		&opts,
		flags.IgnoreUnknown|
			flags.PassAfterNonOption|
			flags.PassDoubleDash|
			flags.EnvProvisioning|
			flags.DotEnv|
			flags.DotEnvFlags,
	)
	parser.SetEnvPrefix("BERCON")
	if _, err := parser.ParseArgs(args); err != nil {
		return bootstrapState{}, appWrap("app.config.selectors", "read config selectors", err)
	}
	selectors := bootstrapSelectorsFor(opts, parser.Active)

	file, found, err := config.Load(selectors.RCPath)
	if err != nil {
		return bootstrapState{}, appWrap("app.config.rc", "RC configuration", err)
	}

	var rc config.RC
	if found {
		rc, err = file.Effective(selectors.Profile)
		if err != nil {
			return bootstrapState{}, appWrap("app.config.profile", "RC profile", err)
		}
	}

	serverPath := rc.ServerCfg
	if selectors.ServerCfg != "" {
		serverPath = selectors.ServerCfg
	}
	if serverPath == "" {
		return bootstrapState{file: file, rc: rc, found: found}, nil
	}

	resolved, err := beconfig.Resolve(serverPath)
	if err != nil {
		return bootstrapState{}, appWrap("app.config.battlEye", "BattlEye config", err)
	}

	serverConfig, err := beconfig.Load(resolved)
	if err != nil {
		return bootstrapState{}, appWrap("app.config.battlEye", "BattlEye config", err)
	}

	return bootstrapState{file: file, rc: rc, server: &serverConfig, found: found}, nil
}

// bootstrapSelectorsFor returns the selector values from the active command scope.
func bootstrapSelectorsFor(opts bootstrapOptions, command *flags.Command) bootstrapSelectors {
	if command == nil {
		return opts.Exec
	}

	switch command.Name {
	case "watch":
		return opts.Watch

	case "shell":
		return opts.Shell

	default:
		return opts.Exec
	}
}
