package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/engine"
	"github.com/norka-app/Norka/internal/features"
)

func TestSaveDiagnosticsStaysOffWhenFlagDisabled(t *testing.T) {
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		return cfg.Features.Set(features.Diagnostics, false)
	}); err != nil {
		t.Fatal(err)
	}
	app := &App{engine: engine.New(engine.Options{Storage: storage})}
	if _, err := app.SaveDiagnostics(true, "dark"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled flag returned %v", err)
	}
}

func TestDiagnosticsFolderAllowlist(t *testing.T) {
	app := &App{}
	if app.knownDiagnostics("/tmp/not-ours.zip") {
		t.Fatal("an arbitrary path was treated as a diagnostics archive")
	}
	const saved = "/tmp/norka-diagnostics-20261003-1607.zip"
	app.rememberDiagnostics(saved)
	if !app.knownDiagnostics(saved) {
		t.Fatal("the archive this session wrote was rejected")
	}
	if app.knownDiagnostics(saved + ".other") {
		t.Fatal("a neighbour of the archive was allowed")
	}
}
