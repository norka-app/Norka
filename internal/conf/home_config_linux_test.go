//go:build linux

package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeConfigDir_UsesXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	got := homeConfigDir(home)
	want := filepath.Join(xdg, "norka")
	if got != want {
		t.Fatalf("homeConfigDir() = %q, want %q", got, want)
	}
}

func TestHomeConfigDir_DefaultsToDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")

	got := homeConfigDir(home)
	want := filepath.Join(home, ".config", "norka")
	if got != want {
		t.Fatalf("homeConfigDir() = %q, want %q", got, want)
	}
}

func TestHomeConfigDir_LegacyDotNorkaWhenXDGMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	legacy := filepath.Join(home, appConfigDirName)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, defaultConfigPath), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := homeConfigDir(home)
	if got != legacy {
		t.Fatalf("homeConfigDir() = %q, want legacy %q", got, legacy)
	}
}

func TestHomeConfigDir_XDGWinsOverLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	legacy := filepath.Join(home, appConfigDirName)
	xdg := filepath.Join(home, ".config", "norka")
	for _, dir := range []string{legacy, xdg} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, defaultConfigPath), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := homeConfigDir(home)
	if got != xdg {
		t.Fatalf("homeConfigDir() = %q, want xdg %q", got, xdg)
	}
}
