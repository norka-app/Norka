package uilocale

import (
	"os"
	"path/filepath"
	"strings"
)

// FileName is stored next to config.toml so the Go tray can follow the UI language.
const FileName = "ui.locale"

// Normalize always returns Russian. The interface has one language.
func Normalize(raw string) string {
	_ = raw
	return "ru"
}

// DetectFromEnv always returns Russian.
func DetectFromEnv() string {
	return "ru"
}

// ReadFile returns the raw contents of ui.locale, or "" if missing.
func ReadFile(configDir string) string {
	if strings.TrimSpace(configDir) == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, FileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WriteFile persists the locale tag for the next process / tray refresh.
func WriteFile(configDir, locale string) error {
	dir := strings.TrimSpace(configDir)
	if dir == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tag := Normalize(locale)
	return os.WriteFile(filepath.Join(dir, FileName), []byte(tag+"\n"), 0o600)
}

// Resolve picks saved preference, otherwise OS locale.
func Resolve(configDir string) string {
	if s := ReadFile(configDir); s != "" {
		return Normalize(s)
	}
	return DetectFromEnv()
}
