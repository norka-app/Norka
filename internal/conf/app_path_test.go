package conf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteAndReadAppPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := WriteAppPath(configPath); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAppPath(configPath)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		exe = resolved
	}
	want, err := filepath.Abs(exe)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("path %q want %q", got, want)
	}
	info, err := os.Stat(AppPathFile(configPath))
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not report the ACL through FileMode; Go shows 0666.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
}

func TestReadAppPathMissingAndStale(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if _, err := ReadAppPath(configPath); !os.IsNotExist(err) {
		t.Fatalf("missing: %v", err)
	}
	stale := filepath.Join(dir, "gone")
	if err := writeAppPath(configPath, stale); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAppPath(configPath); err == nil || !strings.Contains(err.Error(), stale) {
		t.Fatalf("stale: %v", err)
	}
	if err := os.WriteFile(AppPathFile(configPath), []byte("relative\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAppPath(configPath); err == nil {
		t.Fatal("relative path was accepted")
	}
}
