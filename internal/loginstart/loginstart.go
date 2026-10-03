// Package loginstart registers norkad to launch at login.
// It never asks for administrator rights.
// Linux prefers a systemd --user unit and falls back to an XDG autostart file.
// macOS writes a LaunchAgent plist. Windows writes HKCU Run, not a service.
package loginstart

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// SystemdUnitName is the user unit file name.
	SystemdUnitName = "norkad.service"
	// DesktopFileName does not clash with the window's norka.desktop.
	DesktopFileName = "norkad-background.desktop"
	// LaunchAgentLabel is the launchd label.
	LaunchAgentLabel = "app.norka.norkad"
	// LaunchAgentFile is the plist name.
	LaunchAgentFile = LaunchAgentLabel + ".plist"
	// WindowsValueName is the HKCU Run value.
	WindowsValueName = "norkad"
	// WindowsRunKey is the current-user Run key. It is not HKLM and not a service.
	WindowsRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
)

// Layout is where login files are written. Tests pass a temp directory.
type Layout struct {
	SystemdDir      string
	AutostartDir    string
	LaunchAgentsDir string
}

// Registry is the HKCU writer. Tests pass a fake. Windows production uses the real key.
type Registry interface {
	SetString(key, name, value string) error
	GetString(key, name string) (string, bool, error)
	DeleteValue(key, name string) error
}

// Spec is one enable or disable. Run is systemctl or launchctl. Nil skips the command.
type Spec struct {
	GOOS     string
	Exe      string
	Layout   Layout
	Registry Registry
	Run      func(name string, args ...string) error
}

// SystemdUnit is a user unit that starts norkad at login. It does not restart in a loop.
func SystemdUnit(exe string) string {
	return "[Unit]\nDescription=Norka background tunnels\nAfter=default.target\n\n[Service]\nType=simple\nExecStart=" + quoteExec(exe) + "\nRestart=on-failure\nRestartSec=3\n\n[Install]\nWantedBy=default.target\n"
}

// DesktopFile is the XDG autostart entry used when systemd --user is unavailable.
func DesktopFile(exe string) string {
	return "[Desktop Entry]\nType=Application\nVersion=1.0\nName=Norka background\nComment=Keep Norka tunnels running without the window\nExec=" + quoteExec(exe) + "\nTerminal=false\nNoDisplay=true\nX-GNOME-Autostart-enabled=true\n"
}

// LaunchAgent is a per-user LaunchAgent plist. RunAtLoad starts norkad at login.
func LaunchAgent(exe string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>` + LaunchAgentLabel + `</string>
  <key>ProgramArguments</key>
  <array>
    <string>` + xmlEscape(exe) + `</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
</dict>
</plist>
`
}

// WindowsRunValue is the HKCU Run string. The executable is quoted.
func WindowsRunValue(exe string) string {
	return strconvQuote(strings.TrimSpace(exe))
}

func quoteExec(exe string) string {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return "norkad"
	}
	if strings.ContainsAny(exe, " \t\"\\$`") {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(exe)
		return `"` + escaped + `"`
	}
	return exe
}

func xmlEscape(value string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(value)
}

func strconvQuote(value string) string {
	if value == "" {
		return `""`
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

// Enable writes the login entry for spec.GOOS.
// On Linux a working systemctl --user selects the unit; otherwise the desktop file.
func Enable(spec Spec) (string, error) {
	exe := strings.TrimSpace(spec.Exe)
	if exe == "" {
		return "", fmt.Errorf("norkad path is empty")
	}
	switch spec.GOOS {
	case "linux":
		return enableLinux(spec, exe)
	case "darwin":
		return enableDarwin(spec, exe)
	case "windows":
		return enableWindows(spec, exe)
	default:
		return "", fmt.Errorf("login start is not supported on %s", spec.GOOS)
	}
}

// Disable removes the login entry. It does not stop a daemon the window started.
func Disable(spec Spec) error {
	switch spec.GOOS {
	case "linux":
		return disableLinux(spec)
	case "darwin":
		return disableDarwin(spec)
	case "windows":
		return disableWindows(spec)
	default:
		return nil
	}
}

// Enabled reports whether a login entry is present.
func Enabled(spec Spec) (bool, error) {
	switch spec.GOOS {
	case "linux":
		return enabledLinux(spec)
	case "darwin":
		return fileExists(filepath.Join(spec.Layout.LaunchAgentsDir, LaunchAgentFile)), nil
	case "windows":
		if spec.Registry == nil {
			return false, fmt.Errorf("registry is not available")
		}
		_, ok, err := spec.Registry.GetString(WindowsRunKey, WindowsValueName)
		return ok, err
	default:
		return false, nil
	}
}

func enableLinux(spec Spec, exe string) (string, error) {
	if systemdUser(spec) {
		path := filepath.Join(spec.Layout.SystemdDir, SystemdUnitName)
		if err := writeFile(path, SystemdUnit(exe)); err != nil {
			return "", err
		}
		_ = os.Remove(filepath.Join(spec.Layout.AutostartDir, DesktopFileName))
		if spec.Run != nil {
			_ = spec.Run("systemctl", "--user", "daemon-reload")
			if err := spec.Run("systemctl", "--user", "enable", SystemdUnitName); err != nil {
				return "", err
			}
		}
		return "systemd", nil
	}
	path := filepath.Join(spec.Layout.AutostartDir, DesktopFileName)
	if err := writeFile(path, DesktopFile(exe)); err != nil {
		return "", err
	}
	_ = os.Remove(filepath.Join(spec.Layout.SystemdDir, SystemdUnitName))
	return "xdg", nil
}

func disableLinux(spec Spec) error {
	if systemdUser(spec) && spec.Run != nil {
		_ = spec.Run("systemctl", "--user", "disable", SystemdUnitName)
		_ = spec.Run("systemctl", "--user", "daemon-reload")
	}
	_ = os.Remove(filepath.Join(spec.Layout.SystemdDir, SystemdUnitName))
	_ = os.Remove(filepath.Join(spec.Layout.AutostartDir, DesktopFileName))
	return nil
}

func enabledLinux(spec Spec) (bool, error) {
	if fileExists(filepath.Join(spec.Layout.SystemdDir, SystemdUnitName)) {
		return true, nil
	}
	return fileExists(filepath.Join(spec.Layout.AutostartDir, DesktopFileName)), nil
}

func systemdUser(spec Spec) bool {
	if spec.Run == nil {
		return false
	}
	return spec.Run("systemctl", "--user", "show-environment") == nil
}

func enableDarwin(spec Spec, exe string) (string, error) {
	path := filepath.Join(spec.Layout.LaunchAgentsDir, LaunchAgentFile)
	if err := writeFile(path, LaunchAgent(exe)); err != nil {
		return "", err
	}
	if spec.Run != nil {
		_ = spec.Run("launchctl", "bootstrap", "gui/"+currentUID(), path)
	}
	return "launchagent", nil
}

func disableDarwin(spec Spec) error {
	// Removing the plist unregisters the next login. A norkad the window
	// already started is a different process and is left running; the window
	// offers to stop it separately. bootout is not used here because it would
	// stop a LaunchAgent job before that offer.
	_ = spec
	err := os.Remove(filepath.Join(spec.Layout.LaunchAgentsDir, LaunchAgentFile))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func enableWindows(spec Spec, exe string) (string, error) {
	if spec.Registry == nil {
		return "", fmt.Errorf("registry is not available")
	}
	if err := spec.Registry.SetString(WindowsRunKey, WindowsValueName, WindowsRunValue(exe)); err != nil {
		return "", err
	}
	return "registry", nil
}

func disableWindows(spec Spec) error {
	if spec.Registry == nil {
		return fmt.Errorf("registry is not available")
	}
	return spec.Registry.DeleteValue(WindowsRunKey, WindowsValueName)
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func currentUID() string {
	return fmt.Sprint(os.Getuid())
}

// CurrentSpec is the login entry for this machine. exe is the norkad path.
func CurrentSpec(exe string, reg Registry, run func(string, ...string) error) Spec {
	return Spec{
		GOOS:     runtime.GOOS,
		Exe:      exe,
		Layout:   DefaultLayout(),
		Registry: reg,
		Run:      run,
	}
}

// DefaultLayout uses the current user's config directories.
func DefaultLayout() Layout {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	config := filepath.Join(home, ".config")
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		config = dir
	}
	return Layout{
		SystemdDir:      filepath.Join(config, "systemd", "user"),
		AutostartDir:    filepath.Join(config, "autostart"),
		LaunchAgentsDir: filepath.Join(home, "Library", "LaunchAgents"),
	}
}
