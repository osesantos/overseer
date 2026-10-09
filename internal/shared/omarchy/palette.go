// Package omarchy reads the colour palette of the Omarchy theme active on this machine.
package omarchy

import (
	"os"
	"path/filepath"
	"strings"
)

// Dir is the directory Omarchy swaps the active theme into; it honours $XDG_STATE_HOME.
func Dir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "omarchy", "current")
}

// Palette reads dir/theme/colors.toml as flat key/value pairs; false when it is absent or empty.
func Palette(dir string) (map[string]string, bool) {
	if dir == "" {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(dir, "theme", "colors.toml"))
	if err != nil {
		return nil, false
	}
	p := make(map[string]string)
	for line := range strings.SplitSeq(string(raw), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		p[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	return p, len(p) > 0
}
