package loginstart

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdUnitAndEnable(t *testing.T) {
	dir := t.TempDir()
	exe := "/usr/local/bin/norkad"
	var calls []string
	method, err := Enable(Spec{
		GOOS: "linux",
		Exe:  exe,
		Layout: Layout{
			SystemdDir:   filepath.Join(dir, "systemd"),
			AutostartDir: filepath.Join(dir, "autostart"),
		},
		Run: func(name string, args ...string) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != "systemd" {
		t.Fatalf("method %s", method)
	}
	data, err := os.ReadFile(filepath.Join(dir, "systemd", SystemdUnitName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "ExecStart="+exe) || !strings.Contains(text, "WantedBy=default.target") {
		t.Fatalf("unit:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(dir, "autostart", DesktopFileName)); !os.IsNotExist(err) {
		t.Fatal("desktop fallback should not be written when systemd works")
	}
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "systemctl --user enable "+SystemdUnitName) {
		t.Fatalf("enable was not called: %s", joined)
	}
	on, err := Enabled(Spec{GOOS: "linux", Layout: Layout{SystemdDir: filepath.Join(dir, "systemd"), AutostartDir: filepath.Join(dir, "autostart")}})
	if err != nil || !on {
		t.Fatalf("enabled %v %v", on, err)
	}
}

func TestLinuxFallsBackToDesktop(t *testing.T) {
	dir := t.TempDir()
	exe := "/home/norka/bin/norkad"
	method, err := Enable(Spec{
		GOOS: "linux",
		Exe:  exe,
		Layout: Layout{
			SystemdDir:   filepath.Join(dir, "systemd"),
			AutostartDir: filepath.Join(dir, "autostart"),
		},
		Run: func(string, ...string) error { return errors.New("systemd --user is unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != "xdg" {
		t.Fatalf("method %s", method)
	}
	data, err := os.ReadFile(filepath.Join(dir, "autostart", DesktopFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Exec="+exe) || !strings.Contains(string(data), "X-GNOME-Autostart-enabled=true") {
		t.Fatalf("desktop:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "systemd", SystemdUnitName)); !os.IsNotExist(err) {
		t.Fatal("systemd unit should not remain when falling back")
	}
}

func TestLaunchAgentFile(t *testing.T) {
	dir := t.TempDir()
	exe := "/Applications/Norka.app/Contents/MacOS/norkad"
	var calls []string
	method, err := Enable(Spec{
		GOOS:   "darwin",
		Exe:    exe,
		Layout: Layout{LaunchAgentsDir: dir},
		Run: func(name string, args ...string) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != "launchagent" {
		t.Fatalf("method %s", method)
	}
	data, err := os.ReadFile(filepath.Join(dir, LaunchAgentFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "<string>"+exe+"</string>") || !strings.Contains(text, "<key>RunAtLoad</key>") || !strings.Contains(text, LaunchAgentLabel) {
		t.Fatalf("plist:\n%s", text)
	}
	if len(calls) != 1 || !strings.Contains(calls[0], "launchctl bootstrap") {
		t.Fatalf("launchctl calls: %v", calls)
	}
	if err := Disable(Spec{GOOS: "darwin", Layout: Layout{LaunchAgentsDir: dir}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, LaunchAgentFile)); !os.IsNotExist(err) {
		t.Fatal("plist should be removed")
	}
}

func TestWindowsRunValue(t *testing.T) {
	exe := `C:\Program Files\Norka\norkad.exe`
	if WindowsRunValue(exe) != `"C:\Program Files\Norka\norkad.exe"` {
		t.Fatalf("quoted %s", WindowsRunValue(exe))
	}
	reg := &memRegistry{}
	spec := Spec{GOOS: "windows", Exe: exe, Registry: reg}
	method, err := Enable(spec)
	if err != nil {
		t.Fatal(err)
	}
	if method != "registry" {
		t.Fatalf("method %s", method)
	}
	got, ok, err := reg.GetString(WindowsRunKey, WindowsValueName)
	if err != nil || !ok || got != WindowsRunValue(exe) {
		t.Fatalf("registry %q ok=%v err=%v", got, ok, err)
	}
	if gotKey := reg.lastKey; gotKey != WindowsRunKey {
		t.Fatalf("key %s", gotKey)
	}
	on, err := Enabled(spec)
	if err != nil || !on {
		t.Fatalf("enabled %v %v", on, err)
	}
	if err := Disable(spec); err != nil {
		t.Fatal(err)
	}
	on, err = Enabled(spec)
	if err != nil || on {
		t.Fatalf("still enabled %v %v", on, err)
	}
}

func TestQuotedSystemdPath(t *testing.T) {
	text := SystemdUnit(`/opt/Norka App/norkad`)
	if !strings.Contains(text, `ExecStart="/opt/Norka App/norkad"`) {
		t.Fatalf("unit did not quote the path:\n%s", text)
	}
}

type memRegistry struct {
	values  map[string]string
	lastKey string
}

func (m *memRegistry) SetString(key, name, value string) error {
	m.lastKey = key
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key+"\x00"+name] = value
	return nil
}

func (m *memRegistry) GetString(key, name string) (string, bool, error) {
	if m.values == nil {
		return "", false, nil
	}
	value, ok := m.values[key+"\x00"+name]
	return value, ok, nil
}

func (m *memRegistry) DeleteValue(key, name string) error {
	if m.values == nil {
		return nil
	}
	delete(m.values, key+"\x00"+name)
	return nil
}
