package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadAndMergeProfile covers case-insensitive keys and profile overrides.
func TestLoadAndMergeProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ini")
	content := "\uFEFF[globals]\nIP = 127.0.0.1\nport = 2305\ntimeout = 3\nformat = json\n" +
		"password = global-secret\n\n[profile.EU]\nip = 192.0.2.10\nport = 2310\ntimeout = 5\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	file, found, err := Load(path)
	if err != nil || !found {
		t.Fatalf("Load() found=%t err=%v", found, err)
	}
	got, err := file.Effective("EU")
	if err != nil {
		t.Fatal(err)
	}
	if got.IP != "192.0.2.10" || got.Port != 2310 || got.Format != "json" ||
		got.Password != "global-secret" || time.Duration(got.Timeout) != 5*time.Second {
		t.Fatalf("effective profile = %#v", got)
	}
}

// TestLoadParsesDurationSuffixes keeps Go duration values valid in RC profiles.
func TestLoadParsesDurationSuffixes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ini")
	if err := os.WriteFile(path, []byte("[profile.test]\ntimeout = 1500ms\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	file, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := file.Effective("test")
	if err != nil {
		t.Fatal(err)
	}
	if time.Duration(got.Timeout) != 1500*time.Millisecond {
		t.Fatalf("profile timeout = %s, want 1.5s", got.Timeout)
	}
}

// TestProfileNamesRemainCaseSensitive prevents a profile name from being rewritten.
func TestProfileNamesRemainCaseSensitive(t *testing.T) {
	file := &RCFile{Profiles: map[string]RC{"EU": {IP: "192.0.2.10"}}}
	if _, err := file.Effective("eu"); err == nil {
		t.Fatal("profile lookup unexpectedly ignored case")
	}
}

// TestLoadRejectsInvalidNumericValues prevents malformed config from becoming zero defaults.
func TestLoadRejectsInvalidNumericValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ini")
	if err := os.WriteFile(path, []byte("[globals]\nport = 70000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil {
		t.Fatal("invalid port was accepted")
	}
}

// TestEffectiveRejectsUnknownProfile keeps misspelled names from silently using globals.
func TestEffectiveRejectsUnknownProfile(t *testing.T) {
	file := &RCFile{Profiles: map[string]RC{"known": {IP: "127.0.0.1"}}}
	if _, err := file.Effective("missing"); err == nil {
		t.Fatal("missing profile was accepted")
	}
}

// TestGeoDBUpdateSettingsLoadAndMerge keeps source and auth scoped to the selected profile.
func TestGeoDBUpdateSettingsLoadAndMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ini")
	content := "[globals]\ngeo_db_url = https://example.test/global.mmdb\n" +
		"geo_db_url_auth = global-account:global-license\n\n" +
		"[profile.test]\ngeo_db_url = https://example.test/profile.mmdb\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	file, found, err := Load(path)
	if err != nil || !found {
		t.Fatalf("Load() found=%t err=%v", found, err)
	}
	got, err := file.Effective("test")
	if err != nil {
		t.Fatal(err)
	}
	if got.GeoDBURL != "https://example.test/profile.mmdb" ||
		got.GeoDBURLAuth != "global-account:global-license" {
		t.Fatalf("effective GeoIP update settings = %#v", got)
	}
}
