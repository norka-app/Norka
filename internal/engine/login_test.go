package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/cli"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
)

func TestAutostartHiddenFlagResyncsLoginEntry(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the login entry is an XDG desktop file on linux")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	storage := testStorage(t)
	if _, err := storage.Update(func(cfg *conf.Config) error {
		cfg.AutoRun = true
		return cfg.Features.Set(features.AutostartHidden, true)
	}); err != nil {
		t.Fatal(err)
	}
	eng := New(Options{Storage: storage})
	eng.SyncAutoRun()

	path := filepath.Join(dir, "autostart", "norka.desktop")
	hidden, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hidden), cli.HiddenArg) {
		t.Fatalf("hidden flag did not write the tray argument:\n%s", hidden)
	}

	if _, err := storage.Update(func(cfg *conf.Config) error {
		return cfg.Features.Set(features.AutostartHidden, false)
	}); err != nil {
		t.Fatal(err)
	}
	eng.SyncAutoRun()

	visible, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(visible), cli.HiddenArg) {
		t.Fatalf("turning autostart_hidden off left the tray argument:\n%s", visible)
	}
	if string(visible) == string(hidden) {
		t.Fatal("turning autostart_hidden off did not rewrite the login entry")
	}
}

func testStorage(t *testing.T) *conf.Storage {
	t.Helper()
	storage, err := conf.NewStorage(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return storage
}
