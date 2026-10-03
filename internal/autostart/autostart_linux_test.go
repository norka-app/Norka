//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/cli"
)

func TestDesktopEnabled(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"plain", "Type=Application\nX-GNOME-Autostart-enabled=true\n", true},
		{"gnome off", "X-GNOME-Autostart-enabled=false\n", false},
		{"hidden", "Hidden=true\n", false},
		{"comment ignored", "# Hidden=true\nType=Application\n", true},
	}
	for _, tc := range cases {
		if got := desktopEnabled(tc.content); got != tc.want {
			t.Fatalf("%s: desktopEnabled() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestQuoteDesktopExec(t *testing.T) {
	if got := quoteDesktopExec("/usr/bin/norka"); got != "/usr/bin/norka" {
		t.Fatalf("plain path = %q", got)
	}
	got := quoteDesktopExec(`/home/user/My Apps/norka`)
	if got != `"/home/user/My Apps/norka"` {
		t.Fatalf("spaced path = %q", got)
	}
}

func TestEnableWritesXDGAutostart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	enabled, err := IsEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("expected autostart to be off before Enable")
	}
	if err := Enable(true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "autostart", appName+".desktop")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "Name=Norka") || !strings.Contains(text, "X-GNOME-Autostart-enabled=true") {
		t.Fatalf("desktop file missing fields:\n%s", text)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Exec="+linuxDesktopExec(exe, true)) {
		t.Fatalf("desktop file missing hidden Exec:\n%s", text)
	}
	enabled, err = IsEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("expected autostart to be on after Enable")
	}
	if err := Disable(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("desktop file should be removed, stat err = %v", err)
	}
}

func TestSyncRewritesLegacyDesktopEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "autostart", appName+".desktop")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := desktopFile(exe, false)
	if strings.Contains(legacy, cli.HiddenArg) {
		t.Fatal("legacy desktop file must open the window")
	}
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Sync(true, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Exec="+linuxDesktopExec(exe, true)) {
		t.Fatalf("legacy entry was not rewritten:\n%s", data)
	}

	marked := string(data) + "# keep\n"
	if err := os.WriteFile(path, []byte(marked), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Sync(true, true); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != marked {
		t.Fatalf("matching entry was rewritten:\n%s", again)
	}

	if err := Sync(true, false); err != nil {
		t.Fatal(err)
	}
	visible, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(visible), cli.HiddenArg) {
		t.Fatalf("flag off still starts in the tray:\n%s", visible)
	}
	if !strings.Contains(string(visible), "Exec="+linuxDesktopExec(exe, false)) {
		t.Fatalf("visible exec:\n%s", visible)
	}

	if err := Sync(false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("disabled autostart should remove the file, stat err = %v", err)
	}
}
