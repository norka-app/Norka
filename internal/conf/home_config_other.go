//go:build !linux

package conf

import (
	"path/filepath"
	"strings"
)

// homeConfigDir is ~/.norka on Windows and macOS.
func homeConfigDir(homeDir string) string {
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, appConfigDirName)
}
