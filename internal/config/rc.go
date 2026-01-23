package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/ini.v1"
)

// RC holds effective runtime options from rc/config.
type RC struct {
	IP         string
	Password   string
	ServerCfg  string
	GeoDB      string
	Format     string
	Port       int
	TimeoutSec int
	BufferSize uint16
}

// LoadRC loads rc (globals + optional profile) via RCFile.
// Kept for backward compatibility.
func LoadRC(explicitPath, profile string) (RC, bool, error) {
	f, ok, err := LoadRCFile(explicitPath)
	if err != nil || !ok {
		return RC{}, ok, err
	}

	rc, err := f.Effective(profile)
	return rc, true, err
}

func readSectionInto(dst *RC, s *ini.Section) {
	// all keys are optional
	if k := s.Key("ip"); k != nil {
		dst.IP = k.String()
	}
	if k := s.Key("port"); k != nil {
		dst.Port, _ = k.Int()
	}
	if k := s.Key("password"); k != nil {
		dst.Password = k.String()
	}
	if k := s.Key("server_cfg"); k != nil {
		dst.ServerCfg = k.String()
	}
	if k := s.Key("geo_db"); k != nil {
		dst.GeoDB = k.String()
	}
	if k := s.Key("format"); k != nil {
		dst.Format = k.String()
	}
	if k := s.Key("timeout"); k != nil {
		if v, _ := k.Int(); v > 0 {
			dst.TimeoutSec = v
		}
	}
	if k := s.Key("buffer_size"); k != nil {
		if v, _ := k.Int(); v > 0 && v <= 65535 {
			dst.BufferSize = uint16(v)
		}
	}
}

func mergeRC(base, over RC) RC {
	// connection
	if over.IP != "" {
		base.IP = over.IP
	}
	if over.Port != 0 {
		base.Port = over.Port
	}
	if over.Password != "" {
		base.Password = over.Password
	}
	if over.ServerCfg != "" {
		base.ServerCfg = over.ServerCfg
	}
	// misc
	if over.GeoDB != "" {
		base.GeoDB = over.GeoDB
	}
	if over.Format != "" {
		base.Format = strings.ToLower(over.Format)
	}
	if over.TimeoutSec != 0 {
		base.TimeoutSec = over.TimeoutSec
	}
	if over.BufferSize != 0 {
		base.BufferSize = over.BufferSize
	}

	return base
}

func resolveRCPath(explicit string) (string, bool) {
	if explicit != "" {
		if fileExists(explicit) {
			return explicit, true
		}

		return "", false
	}

	home, _ := os.UserHomeDir()
	var candidates []string

	candidates = append(candidates,
		filepath.Join(home, ".config", "bercon-cli", "config.ini"),
		filepath.Join(home, ".bercon-cli.ini"),
	)

	switch runtime.GOOS {
	case "windows":
		if app := os.Getenv("APPDATA"); app != "" {
			candidates = append(candidates,
				filepath.Join(app, "bercon-cli", "config.ini"))
		}

	case "darwin":
		candidates = append(candidates,
			filepath.Join(home, "Library", "Application Support", "bercon-cli", "config.ini"))
	}

	for _, p := range candidates {
		if fileExists(p) {
			return p, true
		}
	}

	return "", false
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
