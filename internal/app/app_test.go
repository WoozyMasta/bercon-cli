package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woozymasta/bercon-cli/internal/config"
)

// TestConfigurationPrecedence loads all sources through the production invocation flow.
func TestConfigurationPrecedence(t *testing.T) {
	dir := t.TempDir()
	rcPath := filepath.Join(dir, "config.ini")
	serverConfigPath := filepath.Join(dir, "beserver.cfg")
	dotEnvPath := filepath.Join(dir, ".env")

	if err := os.WriteFile(serverConfigPath, []byte(
		"RConPassword battleye-secret\nRConPort 23030\nRConIP 192.0.2.30\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rcContents := "[globals]\nip = 192.0.2.10\nport = 21010\n" +
		"password = global-secret\ntimeout = 9\nformat = csv\n\n" +
		"[profile.remote]\nip = 192.0.2.20\nport = 22020\n" +
		"password = profile-secret\nformat = table\nserver_cfg = " + serverConfigPath + "\n"
	if err := os.WriteFile(rcPath, []byte(rcContents), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(dotEnvPath, []byte(
		"BERCON_ADDRESS=192.0.2.40\nBERCON_PORT=24040\n"+
			"BERCON_PASSWORD=dotenv-secret\nBERCON_TIMEOUT=5s\n"+
			"BERCON_KEEPALIVE=7s\nBERCON_FORMAT=md\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	processEnv := map[string]string{
		"BERCON_ADDRESS":   "192.0.2.50",
		"BERCON_PORT":      "25050",
		"BERCON_PASSWORD":  "process-secret",
		"BERCON_TIMEOUT":   "6s",
		"BERCON_KEEPALIVE": "8s",
		"BERCON_FORMAT":    "json",
		"BERCON_ATTEMPTS":  "",
	}
	for name := range processEnv {
		clearEnv(t, name)
	}
	for name, value := range processEnv {
		if value != "" {
			withEnv(t, name, value)
		}
	}

	tests := []struct {
		name          string
		args          []string
		wantIP        string
		wantPort      int
		wantPassword  string
		wantTimeout   time.Duration
		wantKeepalive time.Duration
		wantFormat    string
	}{
		{
			name:          "highest configured source wins at each field",
			wantIP:        "192.0.2.30",
			wantPort:      23030,
			wantPassword:  "battleye-secret",
			wantTimeout:   9 * time.Second,
			wantKeepalive: 8 * time.Second,
			wantFormat:    "table",
		},
		{
			name:          "env override changes only dotenv and process environment",
			args:          []string{"--env-override"},
			wantIP:        "192.0.2.30",
			wantPort:      23030,
			wantPassword:  "battleye-secret",
			wantTimeout:   9 * time.Second,
			wantKeepalive: 7 * time.Second,
			wantFormat:    "table",
		},
		{
			name: "CLI overrides every loaded source",
			args: []string{
				"--ip=192.0.2.99",
				"--port=23999",
				"--password=cli-secret",
				"--timeout=11s",
				"--keepalive=10s",
				"--format=json",
			},
			wantIP:        "192.0.2.99",
			wantPort:      23999,
			wantPassword:  "cli-secret",
			wantTimeout:   11 * time.Second,
			wantKeepalive: 10 * time.Second,
			wantFormat:    "json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := []string{
				"--config=" + rcPath,
				"--profile=remote",
				"--env-file=" + dotEnvPath,
			}
			args = append(args, test.args...)
			args = append(args, "players")

			var stderr strings.Builder
			inv, status := parseInvocation(args, &stderr)
			if status != exitSuccess || inv == nil {
				t.Fatalf("parseInvocation() = (%v, %v), stderr=%q", inv, status, stderr.String())
			}

			got := inv.opts.Connection
			if got.IP != test.wantIP ||
				got.Port != test.wantPort ||
				got.Password != test.wantPassword ||
				time.Duration(got.Timeout) != test.wantTimeout ||
				time.Duration(got.Keepalive) != test.wantKeepalive ||
				string(inv.opts.Output.Format) != test.wantFormat ||
				got.Attempts != 1 {
				t.Fatalf("effective options = %#v, format=%q; want %s:%d, timeout=%s, keepalive=%s, format=%s, attempts=1",
					got, inv.opts.Output.Format, test.wantIP, test.wantPort, test.wantTimeout, test.wantKeepalive, test.wantFormat)
			}
			if len(inv.commands) != 1 || inv.commands[0] != "players" {
				t.Fatalf("commands = %#v, want [players]", inv.commands)
			}
		})
	}
}

// TestConfigurationSelectorsLoadFromEnvironment checks selector values before RC defaults are loaded.
func TestConfigurationSelectorsLoadFromEnvironment(t *testing.T) {
	dir := t.TempDir()
	rcPath := filepath.Join(dir, "servers.ini")
	dotEnvPath := filepath.Join(dir, ".env")
	contents := "[profile.remote]\nip = 192.0.2.70\nport = 23700\npassword = profile-secret\n"
	if err := os.WriteFile(rcPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dotEnvPath, []byte(
		"BERCON_CONFIG="+rcPath+"\nBERCON_PROFILE=remote\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		set  map[string]string
	}{
		{
			name: "process environment",
			args: []string{"--no-env", "players"},
			set: map[string]string{
				"BERCON_CONFIG":  rcPath,
				"BERCON_PROFILE": "remote",
			},
		},
		{
			name: "dotenv file",
			args: []string{"--env-file=" + dotEnvPath, "players"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearEnv(t, "BERCON_CONFIG", "BERCON_PROFILE", "BERCON_PASSWORD")
			for name, value := range test.set {
				withEnv(t, name, value)
			}

			var stderr strings.Builder
			inv, status := parseInvocation(test.args, &stderr)
			if status != exitSuccess || inv == nil {
				t.Fatalf("parseInvocation() = (%v, %v), stderr=%q", inv, status, stderr.String())
			}

			got := inv.opts.Connection
			if got.IP != "192.0.2.70" || got.Port != 23700 || got.Password != "profile-secret" {
				t.Fatalf("effective connection = %#v; want selected profile values", got)
			}
			if inv.opts.Profile.Profile != "remote" {
				t.Fatalf("selected profile = %q, want remote", inv.opts.Profile.Profile)
			}
		})
	}
}

// TestDurationFlagsKeepLegacyUnitsAndAcceptGoDurations protects existing CLI values and adds suffix support.
func TestDurationFlagsKeepLegacyUnitsAndAcceptGoDurations(t *testing.T) {
	clearEnv(t, "BERCON_TIMEOUT", "BERCON_KEEPALIVE", "BERCON_SLEEP_CMD", "BERCON_SLEEP_LOOP")
	tests := []struct {
		name          string
		values        []string
		wantTimeout   time.Duration
		wantKeepalive time.Duration
		wantCmdSleep  time.Duration
		wantLoopSleep time.Duration
	}{
		{
			name: "unitless values retain their previous units",
			values: []string{
				"--timeout=3", "--keepalive=4", "--cmd-sleep=1", "--loop-sleep=5",
			},
			wantTimeout:   3 * time.Second,
			wantKeepalive: 4 * time.Second,
			wantCmdSleep:  time.Millisecond,
			wantLoopSleep: 5 * time.Second,
		},
		{
			name: "Go duration suffixes are accepted",
			values: []string{
				"--timeout=250ms", "--keepalive=1500ms", "--cmd-sleep=250ms", "--loop-sleep=2m",
			},
			wantTimeout:   250 * time.Millisecond,
			wantKeepalive: 1500 * time.Millisecond,
			wantCmdSleep:  250 * time.Millisecond,
			wantLoopSleep: 2 * time.Minute,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"--no-env", "--password=secret"}, test.values...)
			args = append(args, "players")

			var stderr strings.Builder
			inv, status := parseInvocation(args, &stderr)
			if status != exitSuccess || inv == nil {
				t.Fatalf("parseInvocation() = (%v, %v), stderr=%q", inv, status, stderr.String())
			}

			got := inv.opts
			if time.Duration(got.Connection.Timeout) != test.wantTimeout ||
				time.Duration(got.Connection.Keepalive) != test.wantKeepalive ||
				time.Duration(got.Exec.CmdSleep) != test.wantCmdSleep ||
				time.Duration(got.Exec.LoopSleep) != test.wantLoopSleep {
				t.Fatalf("durations = timeout %s, keepalive %s, command delay %s, loop delay %s",
					got.Connection.Timeout, got.Connection.Keepalive,
					got.Exec.CmdSleep, got.Exec.LoopSleep)
			}
		})
	}
}

// TestGeoDBURLCanBeExplicitlyDisabled preserves local database use without updates.
func TestGeoDBURLCanBeExplicitlyDisabled(t *testing.T) {
	clearEnv(t, "BERCON_GEO_DB", "BERCON_GEO_DB_URL", "BERCON_GEO_DB_URL_AUTH")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.ini")
	contents := "[globals]\ngeo_db = " + filepath.Join(dir, "GeoLite2.mmdb") +
		"\ngeo_db_url = https://example.test/database.mmdb\n"
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr strings.Builder
	inv, status := parseInvocation([]string{
		"--no-env",
		"--config=" + configPath,
		"--geo-db-url=",
		"players",
	}, &stderr)
	if status != exitSuccess || inv == nil {
		t.Fatalf("parseInvocation() = (%v, %v), stderr=%q", inv, status, stderr.String())
	}
	if inv.opts.Geo.URL != "" || inv.opts.Geo.DB != filepath.Join(dir, "GeoLite2.mmdb") {
		t.Fatalf("Geo options = %#v, want updates disabled and database path preserved", inv.opts.Geo)
	}
}

// TestSessionCommandsSelectExecutionFlow verifies both explicit commands and the root exec shorthand.
func TestSessionCommandsSelectExecutionFlow(t *testing.T) {
	clearEnv(t, "BERCON_PASSWORD", "BERCON_REPEAT")
	tests := []struct {
		name                string
		args                []string
		wantMode            sessionMode
		wantCommand         []string
		wantRepeat          int
		wantWatchTimestamps bool
		wantShellNoBuiltins bool
	}{
		{
			name:        "root invocation defaults to exec",
			args:        []string{"--no-env", "--password=secret", "players"},
			wantMode:    modeExec,
			wantCommand: []string{"players"},
			wantRepeat:  1,
		},
		{
			name:        "root invocation accepts exec options",
			args:        []string{"--no-env", "--password=secret", "--repeat=2", "players"},
			wantMode:    modeExec,
			wantCommand: []string{"players"},
			wantRepeat:  2,
		},
		{
			name:        "explicit exec accepts command options",
			args:        []string{"--no-env", "exec", "--password=secret", "--repeat=3", "players"},
			wantMode:    modeExec,
			wantCommand: []string{"players"},
			wantRepeat:  3,
		},
		{
			name:        "exec can send a command sharing a session command name",
			args:        []string{"--no-env", "exec", "--password=secret", "--", "watch"},
			wantMode:    modeExec,
			wantCommand: []string{"watch"},
			wantRepeat:  1,
		},
		{
			name:                "watch selects message listener",
			args:                []string{"--no-env", "watch", "--password=secret", "--timestamps"},
			wantMode:            modeWatch,
			wantRepeat:          1,
			wantWatchTimestamps: true,
		},
		{
			name:                "shell selects stdin session",
			args:                []string{"--no-env", "shell", "--password=secret", "--no-builtins"},
			wantMode:            modeShell,
			wantRepeat:          1,
			wantShellNoBuiltins: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr strings.Builder
			inv, status := parseInvocation(test.args, &stderr)
			if status != exitSuccess || inv == nil {
				t.Fatalf("parseInvocation() = (%v, %v), stderr=%q", inv, status, stderr.String())
			}
			if inv.mode != test.wantMode || strings.Join(inv.commands, " ") != strings.Join(test.wantCommand, " ") {
				t.Fatalf("selected flow = (%q, %#v), want (%q, %#v)",
					inv.mode, inv.commands, test.wantMode, test.wantCommand)
			}
			if inv.opts.Exec.Repeat != test.wantRepeat {
				t.Fatalf("repeat = %d, want %d", inv.opts.Exec.Repeat, test.wantRepeat)
			}
			if inv.opts.Watch.Timestamps != test.wantWatchTimestamps {
				t.Fatalf("watch timestamps = %t, want %t",
					inv.opts.Watch.Timestamps, test.wantWatchTimestamps)
			}
			if inv.opts.Shell.NoBuiltins != test.wantShellNoBuiltins {
				t.Fatalf("shell no-builtins = %t, want %t",
					inv.opts.Shell.NoBuiltins, test.wantShellNoBuiltins)
			}
		})
	}
}

// TestExecLoadsProfileSelectedAfterCommand checks bootstrap and full parsing share command placement.
func TestExecLoadsProfileSelectedAfterCommand(t *testing.T) {
	clearEnv(t,
		"BERCON_ADDRESS", "BERCON_PORT", "BERCON_PASSWORD", "BERCON_PROFILE", "BERCON_CONFIG",
		"BERCON_TIMEOUT", "BERCON_ATTEMPTS", "BERCON_KEEPALIVE",
	)
	configPath := filepath.Join(t.TempDir(), "config.ini")
	contents := "[profile.remote]\nip = 192.0.2.70\nport = 23700\npassword = profile-secret\n"
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr strings.Builder
	args := []string{
		"exec", "--no-env", "--config=" + configPath, "--profile=remote", "players",
	}
	inv, status := parseInvocation(args, &stderr)
	if status != exitSuccess || inv == nil {
		t.Fatalf("parseInvocation() = (%v, %v), stderr=%q", inv, status, stderr.String())
	}
	if inv.opts.Connection.IP != "192.0.2.70" || inv.opts.Connection.Port != 23700 ||
		inv.opts.Connection.Password != "profile-secret" || len(inv.commands) != 1 || inv.commands[0] != "players" {
		t.Fatalf("effective connection=%#v, command=%#v; want selected profile and players",
			inv.opts.Connection, inv.commands)
	}
}

// TestRunRejectsInvalidSessionCommands checks cross-option rules through the production CLI flow.
func TestRunRejectsInvalidSessionCommands(t *testing.T) {
	clearEnv(t,
		"BERCON_ADDRESS", "BERCON_PORT", "BERCON_PASSWORD", "BERCON_PROFILE", "BERCON_CONFIG",
		"BERCON_TIMEOUT", "BERCON_ATTEMPTS", "BERCON_KEEPALIVE", "BERCON_REPEAT",
	)
	configPath := filepath.Join(t.TempDir(), "config.ini")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "password required for session",
			args: []string{"players"},
		},
		{
			name: "repeat cannot be zero",
			args: []string{"--password=secret", "exec", "--repeat=0", "players"},
		},
		{
			name: "command required in exec mode",
			args: []string{"--password=secret"},
		},
		{
			name: "watch rejects positional commands",
			args: []string{"--password=secret", "watch", "players"},
		},
		{
			name: "shell rejects positional commands",
			args: []string{"--password=secret", "shell", "players"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"--no-env", "--config=" + configPath}
			args = append(args, test.args...)
			var stdout, stderr strings.Builder
			if status := Run(args, strings.NewReader(""), &stdout, &stderr); status != int(exitUsageError) {
				t.Fatalf("Run() status=%d, want %d; stdout=%q stderr=%q",
					status, exitUsageError, stdout.String(), stderr.String())
			}
		})
	}
}

// TestExampleConfigCanBeLoaded verifies the generated profile survives the normal config loader.
func TestExampleConfigCanBeLoaded(t *testing.T) {
	var stdout, stderr strings.Builder
	if status := Run([]string{"--no-env", "--example"}, strings.NewReader(""), &stdout, &stderr); status != 0 {
		t.Fatalf("Run() status = %d, stderr = %q", status, stderr.String())
	}

	path := filepath.Join(t.TempDir(), "example.ini")
	if err := os.WriteFile(path, []byte(stdout.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	file, found, err := config.Load(path)
	if err != nil || !found {
		t.Fatalf("config.Load() found=%t err=%v", found, err)
	}
	profile, err := file.Effective("local")
	if err != nil {
		t.Fatal(err)
	}
	if profile.IP != "127.0.0.1" || profile.Port != 2305 ||
		profile.Password != "replace-me" || profile.ServerCfg == "" {
		t.Fatalf("loaded example profile = %#v", profile)
	}
}

// TestProfileListingHidesCredentials protects the profile command's output contract.
func TestProfileListingHidesCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.ini")
	contents := "[profile.local]\nip = 192.0.2.10\nport = 2310\n" +
		"password = do-not-print-this\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	args := []string{"--no-env", "--config=" + path, "--list-profiles"}
	if status := Run(args, strings.NewReader(""), &stdout, &stderr); status != 0 {
		t.Fatalf("Run() status=%d stderr=%q", status, stderr.String())
	}
	profileOutput := strings.ToLower(stdout.String())
	if !strings.Contains(profileOutput, "local") || !strings.Contains(profileOutput, "192.0.2.10") {
		t.Fatalf("profile listing omitted the configured profile or address: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "do-not-print-this") {
		t.Fatalf("profile output leaked data or omitted endpoint: %q", stdout.String())
	}
}

// withEnv restores process-global environment state after one test.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	previous, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// clearEnv removes process-global values for an isolated precedence test.
func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		previous, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, previous)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}
