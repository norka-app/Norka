//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const appDisplayName = "Norka"

// autostartDir is $XDG_CONFIG_HOME/autostart, or ~/.config/autostart.
func autostartDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "autostart"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("home directory is unavailable")
	}
	return filepath.Join(home, ".config", "autostart"), nil
}

func desktopPath() (string, error) {
	dir, err := autostartDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName+".desktop"), nil
}

func IsEnabled() (bool, error) {
	path, err := desktopPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return desktopEnabled(string(data)), nil
}

func Enable() error {
	path, err := desktopPath()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := desktopFile(exe)
	return os.WriteFile(path, []byte(content), 0o644)
}

func Disable() error {
	path, err := desktopPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func desktopFile(exe string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Version=1.0\n" +
		"Name=" + appDisplayName + "\n" +
		"Comment=SSH tunnel manager\n" +
		"Exec=" + quoteDesktopExec(exe) + "\n" +
		"Icon=norka\n" +
		"Terminal=false\n" +
		"Categories=Network;\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

func quoteDesktopExec(exe string) string {
	if exe == "" {
		return "norka"
	}
	if strings.ContainsAny(exe, " \t\"\\$`") {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(exe)
		return `"` + escaped + `"`
	}
	return exe
}

// desktopEnabled reports whether an autostart entry should launch the app.
// A missing file is handled by the caller. Hidden=true and
// X-GNOME-Autostart-enabled=false both mean "off".
func desktopEnabled(content string) bool {
	hidden := false
	gnome := true
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch strings.ToLower(key) {
		case "hidden":
			hidden = strings.EqualFold(value, "true")
		case "x-gnome-autostart-enabled":
			gnome = !strings.EqualFold(value, "false")
		}
	}
	return !hidden && gnome
}
