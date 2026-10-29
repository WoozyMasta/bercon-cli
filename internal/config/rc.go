// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RC stores values from the [globals] section or one [profile.NAME] section.
type RC struct {
	IP           string   // IP overrides the default RCon host.
	Password     string   // Password provides the profile's RCon credential.
	ServerCfg    string   // ServerCfg selects a BattlEye config whose values become defaults.
	GeoDB        string   // GeoDB selects the optional GeoIP database.
	GeoDBURL     string   // GeoDBURL enables updates from an HTTPS database URL when set.
	GeoDBURLAuth string   // GeoDBURLAuth stores HTTP Basic Auth credentials for the GeoIP URL.
	Format       string   // Format selects the default command output format.
	Port         int      // Port overrides the default RCon port; zero means unset.
	Timeout      Duration // Timeout overrides the operation timeout; zero means unset.
	BufferSize   uint16   // BufferSize accepts the old setting but is intentionally unused.
}

// RCFile contains global defaults and named server profiles.
type RCFile struct {
	Profiles map[string]RC // Profiles stores individually named connection defaults.
	Path     string        // Path is the file read by Load.
	Globals  RC            // Globals stores values applied before the selected profile.
}

// Load reads an explicit file or discovers the standard per-user config file.
// A missing implicit config is not an error; a missing explicit path is.
func Load(path string) (*RCFile, bool, error) {
	explicit := path != ""
	if !explicit {
		path = defaultPath()
		if path == "" {
			return nil, false, nil
		}
	}

	file, err := os.Open(path)
	if err != nil {
		if !explicit && errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("open rc file %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	result := &RCFile{Path: path, Profiles: make(map[string]RC)}
	section := ""
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if lineNumber == 1 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = normalizeSection(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, false, fmt.Errorf("rc file %q line %d: expected key=value", path, lineNumber)
		}

		values[section+"\x00"+strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf("read rc file %q: %w", path, err)
	}

	for compound, value := range values {
		section, key, _ := strings.Cut(compound, "\x00")
		if section == "globals" {
			if err := assign(&result.Globals, key, value); err != nil {
				return nil, false, fmt.Errorf("rc globals %s: %w", key, err)
			}

			continue
		}

		name, ok := strings.CutPrefix(section, "profile.")
		if !ok || name == "" {
			continue
		}

		profile := result.Profiles[name]
		if err := assign(&profile, key, value); err != nil {
			return nil, false, fmt.Errorf("rc profile %q %s: %w", name, key, err)
		}

		result.Profiles[name] = profile
	}

	return result, true, nil
}

// DefaultPaths returns the implicit RC config paths in search order.
func DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	var candidates []string
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			candidates = append(candidates, filepath.Join(appData, "bercon-cli", "config.ini"))
		}
	}

	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			filepath.Join(home, "Library", "Application Support", "bercon-cli", "config.ini"))
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "bercon-cli", "config.ini"))
	} else {
		candidates = append(candidates, filepath.Join(home, ".config", "bercon-cli", "config.ini"))
	}

	return append(candidates, filepath.Join(home, ".bercon-cli.ini"))
}

// normalizeSection folds reserved names but preserves profile names exactly.
func normalizeSection(section string) string {
	if strings.EqualFold(section, "globals") {
		return "globals"
	}

	name, ok := strings.CutPrefix(strings.ToLower(section), "profile.")
	if !ok || name == "" {
		return ""
	}

	_, originalName, _ := strings.Cut(section, ".")
	return "profile." + originalName
}

// Effective merges profile values over globals and reports unknown profiles.
func (f *RCFile) Effective(name string) (RC, error) {
	if name == "" {
		return f.Globals, nil
	}

	profile, ok := f.Profiles[name]
	if !ok {
		return RC{}, fmt.Errorf("profile %q not found", name)
	}

	return merge(f.Globals, profile), nil
}

// ProfileNames returns names in stable alphabetical order for scriptable output.
func (f *RCFile) ProfileNames() []string {
	names := make([]string, 0, len(f.Profiles))
	for name := range f.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// assign validates one known INI value without silently accepting bad numbers.
func assign(rc *RC, key, value string) error {
	switch key {
	case "ip":
		rc.IP = value

	case "password":
		rc.Password = value

	case "server_cfg":
		rc.ServerCfg = value

	case "geo_db":
		rc.GeoDB = value

	case "geo_db_url":
		rc.GeoDBURL = value

	case "geo_db_url_auth":
		rc.GeoDBURLAuth = value

	case "format":
		rc.Format = strings.ToLower(value)

	case "port":
		return parseInt(value, &rc.Port, 1, 65535)

	case "timeout":
		var timeout Duration
		if err := timeout.UnmarshalText([]byte(value)); err != nil {
			return err
		}

		duration := time.Duration(timeout)
		if duration < time.Second || duration > 24*time.Hour {
			return errors.New("expected a duration from 1s to 24h")
		}

		rc.Timeout = timeout

	case "buffer_size":
		var size int
		if err := parseInt(value, &size, 1, 65535); err != nil {
			return err
		}

		// parseInt enforces uint16's complete nonzero range before conversion.
		rc.BufferSize = uint16(size) // #nosec G115 -- range is bounded above
	}

	return nil
}

// parseInt applies numeric bounds shared by RC integer settings.
func parseInt(value string, target *int, minimum, maximum int) error {
	number, err := strconv.Atoi(value)
	if err != nil || number < minimum || number > maximum {
		return fmt.Errorf("expected an integer from %d to %d", minimum, maximum)
	}
	*target = number

	return nil
}

// merge lets a profile override only values it explicitly sets.
func merge(base, override RC) RC {
	if override.IP != "" {
		base.IP = override.IP
	}
	if override.Password != "" {
		base.Password = override.Password
	}
	if override.ServerCfg != "" {
		base.ServerCfg = override.ServerCfg
	}
	if override.GeoDB != "" {
		base.GeoDB = override.GeoDB
	}
	if override.GeoDBURL != "" {
		base.GeoDBURL = override.GeoDBURL
	}
	if override.GeoDBURLAuth != "" {
		base.GeoDBURLAuth = override.GeoDBURLAuth
	}
	if override.Format != "" {
		base.Format = override.Format
	}
	if override.Port != 0 {
		base.Port = override.Port
	}
	if override.Timeout != 0 {
		base.Timeout = override.Timeout
	}
	if override.BufferSize != 0 {
		base.BufferSize = override.BufferSize
	}
	return base
}

// defaultPath returns the standard location for the current operating system.
func defaultPath() string {
	for _, candidate := range DefaultPaths() {
		// User-configured roots are intentionally probed during default discovery.
		//nolint:gosec // This stats only candidate config paths from the OS environment.
		if _, err := os.Stat(candidate); err == nil || !errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}

	return ""
}
