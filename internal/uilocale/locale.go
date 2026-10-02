package uilocale

import (
	"os"
	"path/filepath"
	"strings"
)

// FileName is stored next to config.toml so a relocated config folder keeps
// the language preference. The source of truth is config.toml (language).
const FileName = "ui.locale"

const (
	// PrefAuto follows the system locale.
	PrefAuto = "auto"
	// PrefRU is Russian.
	PrefRU = "ru"
	// PrefEN is English.
	PrefEN = "en"
)

// NormalizePreference maps a stored or UI value to auto, ru, or en.
// Empty and unknown values are auto. Tags that start with ru or en are explicit.
func NormalizePreference(raw string) string {
	text := strings.ToLower(strings.TrimSpace(raw))
	text = strings.ReplaceAll(text, "_", "-")
	if i := strings.IndexAny(text, ".@"); i >= 0 {
		text = text[:i]
	}
	switch text {
	case "", PrefAuto:
		return PrefAuto
	case PrefRU, "ru-ru":
		return PrefRU
	case PrefEN, "en-us", "en-gb":
		return PrefEN
	}
	if strings.HasPrefix(text, "ru") {
		return PrefRU
	}
	if strings.HasPrefix(text, "en") {
		return PrefEN
	}
	return PrefAuto
}

// Normalize maps a locale tag to the UI language actually shown: ru or en.
// auto and empty follow the system locale. Anything else that is not Russian is English.
func Normalize(raw string) string {
	switch NormalizePreference(raw) {
	case PrefRU:
		return PrefRU
	case PrefEN:
		return PrefEN
	default:
		return DetectFromEnv()
	}
}

// Effective resolves a stored preference (auto, ru, en, or empty) to ru or en.
func Effective(preference string) string {
	return Normalize(preference)
}

// DetectFromEnv returns ru when the system locale starts with ru, otherwise en.
// Empty, C, and POSIX locale values are skipped so the next source can decide.
func DetectFromEnv() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if tag := classifyLocale(os.Getenv(key)); tag != "" {
			return tag
		}
	}
	if tag := classifyLocale(platformLocale()); tag != "" {
		return tag
	}
	return PrefEN
}

// classifyLocale returns "ru", "en", or "" when the value should be ignored.
func classifyLocale(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	text = strings.ToLower(text)
	if i := strings.IndexAny(text, ".@"); i >= 0 {
		text = text[:i]
	}
	text = strings.ReplaceAll(text, "-", "_")
	switch text {
	case "c", "posix":
		return ""
	}
	if strings.HasPrefix(text, "ru") {
		return PrefRU
	}
	return PrefEN
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

// WriteFile persists the language preference (auto, ru, or en) for the next process.
func WriteFile(configDir, preference string) error {
	dir := strings.TrimSpace(configDir)
	if dir == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tag := NormalizePreference(preference)
	return os.WriteFile(filepath.Join(dir, FileName), []byte(tag+"\n"), 0o600)
}

// Resolve picks the preference saved in ui.locale, otherwise the OS locale.
func Resolve(configDir string) string {
	if s := ReadFile(configDir); s != "" {
		return Effective(s)
	}
	return DetectFromEnv()
}
