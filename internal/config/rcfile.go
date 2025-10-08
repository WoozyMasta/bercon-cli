package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-ini/ini"
)

// RCFile represents parsed rc file with globals and profiles.
type RCFile struct {
	Profiles map[string]RC
	Path     string
	Globals  RC
}

// Effective returns merged RC for given profile (globals overridden by profile).
// If profile is empty, returns just globals. Returns error if profile not found.
func (f *RCFile) Effective(profile string) (RC, error) {
	if profile == "" {
		return f.Globals, nil
	}

	pr, ok := f.Profiles[profile]
	if !ok {
		return RC{}, fmt.Errorf("profile not found: %s", profile)
	}
	rc := mergeRC(f.Globals, pr)

	return rc, nil
}

// ProfileNames returns sorted profile names.
func (f *RCFile) ProfileNames() []string {
	names := make([]string, 0, len(f.Profiles))
	for k := range f.Profiles {
		names = append(names, k)
	}
	sort.Strings(names)

	return names
}

// LoadRCFile loads and parses the rc ini once. Returns (nil,false,nil) if not found.
func LoadRCFile(explicitPath string) (*RCFile, bool, error) {
	path, ok := resolveRCPath(explicitPath)
	if !ok {
		return nil, false, nil
	}

	cfg, err := ini.Load(path)
	if err != nil {
		return nil, false, err
	}

	f := &RCFile{
		Path:     path,
		Profiles: make(map[string]RC),
	}
	// read globals
	readSectionInto(&f.Globals, cfg.Section("globals"))

	// read profiles
	for _, sec := range cfg.Sections() {
		if !strings.HasPrefix(sec.Name(), "profile.") {
			continue
		}
		name := strings.TrimPrefix(sec.Name(), "profile.")
		var pr RC
		readSectionInto(&pr, sec)
		f.Profiles[name] = pr
	}

	return f, true, nil
}
