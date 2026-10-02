//go:build linux

package conf

import (
	"os"
	"path/filepath"
	"strings"
)

// homeConfigDir follows the XDG Base Directory spec:
// $XDG_CONFIG_HOME/norka, or ~/.config/norka when the variable is unset.
// If that directory has no config yet but ~/.norka does, the legacy path is
// kept so an existing file is not abandoned.
func homeConfigDir(homeDir string) string {
	homeDir = strings.TrimSpace(homeDir)
	xdg := xdgConfigDir(homeDir)
	if homeDir == "" {
		return xdg
	}
	legacy := filepath.Join(homeDir, appConfigDirName)
	if configAnchorExists(xdg) {
		return xdg
	}
	if configAnchorExists(legacy) {
		return legacy
	}
	return xdg
}

func xdgConfigDir(homeDir string) string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "norka")
	}
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, ".config", "norka")
}

func configAnchorExists(dir string) bool {
	if dir == "" {
		return false
	}
	if regularFileExists(filepath.Join(dir, defaultConfigPath)) {
		return true
	}
	return regularFileExists(filepath.Join(dir, ConfigRootFileName))
}

func regularFileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
