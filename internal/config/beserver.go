package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RCON holds connection params resolved from beserver_x64*.cfg
type RCON struct {
	IP       string
	Password string
	Port     int
}

// LoadFromBeServerCfg resolves a file path (file or dir) and parses RCon settings.
// If ip is 0.0.0.0 or empty, it is normalized to 127.0.0.1 for Windows compatibility.
func LoadFromBeServerCfg(path string) (RCON, error) {
	cfgPath, err := resolveBeServerCfgPath(path)
	if err != nil {
		return RCON{}, err
	}

	rc, err := parseBeServerCfg(cfgPath)
	if err != nil {
		return RCON{}, err
	}

	if rc.IP == "" || rc.IP == "0.0.0.0" {
		rc.IP = "127.0.0.1"
	}

	return rc, nil
}

// resolveBeServerCfgPath returns a concrete cfg file path.
// If 'path' is a directory, it searches for beserver_x64*.cfg,
// preferring files with "active" in name, otherwise newest by mtime.
func resolveBeServerCfgPath(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if !st.IsDir() {
		return path, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	type candidate struct {
		mod      time.Time
		filename string
		active   bool
	}

	var cands []candidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		low := strings.ToLower(name)
		if !strings.HasPrefix(low, "beserver_x64") || !strings.HasSuffix(low, ".cfg") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		cands = append(cands, candidate{
			filename: filepath.Join(path, name),
			active:   strings.Contains(low, "active"),
			mod:      info.ModTime(),
		})
	}

	if len(cands) == 0 {
		return "", errors.New("no beserver_x64*.cfg found in directory")
	}

	// Active first, then newest by mtime
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].active != cands[j].active {
			return cands[i].active && !cands[j].active
		}

		return cands[i].mod.After(cands[j].mod)
	})

	return cands[0].filename, nil
}

// parseBeServerCfg parses RConPassword, RConPort, RConIP from cfg file.
// Ignores blank lines and lines starting with ';', '#', or '//' (after trim).
func parseBeServerCfg(path string) (RCON, error) {
	f, err := os.Open(path)
	if err != nil {
		return RCON{}, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var rc RCON
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		// Skip whole-line comments
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Tokenize by whitespace: "Key  Value"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.ToLower(fields[0])
		val := fields[1]

		switch key {
		case "rconpassword":
			rc.Password = val

		case "rconport":
			if p, err := strconv.Atoi(val); err == nil && p > 0 {
				rc.Port = p
			}

		case "rconip":
			rc.IP = val
		}
	}

	if err := sc.Err(); err != nil {
		return RCON{}, err
	}

	if rc.Password == "" {
		return RCON{}, errors.New("RConPassword not found in cfg")
	}

	if rc.Port == 0 {
		return RCON{}, errors.New("RConPort not found or invalid in cfg")
	}

	return rc, err
}
