package update

import (
	"os"
	"path/filepath"
	"strings"
)

// SkipFileName is stored next to config.toml.
const SkipFileName = "update.skip"

// ReadSkip returns the version the user chose to ignore, or "".
func ReadSkip(configDir string) string {
	dir := strings.TrimSpace(configDir)
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, SkipFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WriteSkip remembers a version so later automatic checks stay quiet.
func WriteSkip(configDir, version string) error {
	dir := strings.TrimSpace(configDir)
	if dir == "" {
		return os.ErrInvalid
	}
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	if version == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, SkipFileName), []byte(version+"\n"), 0o600)
}
