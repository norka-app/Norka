//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err := Enable(); err != nil {
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
	if !strings.Contains(text, "Exec=") {
		t.Fatalf("desktop file missing Exec:\n%s", text)
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
