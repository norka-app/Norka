package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
)

func TestMain(m *testing.M) {
	if os.Getenv("NORKAD_TEST_RUN") == "1" {
		os.Exit(run(os.Args[1:], realEnv()))
	}
	os.Exit(m.Run())
}

func TestRunExitsWhenBackgroundOff(t *testing.T) {
	for _, name := range []string{"missing", "explicit-false"} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, func(cfg *conf.Config) {
				if name != "explicit-false" {
					return
				}
				if err := cfg.Features.Set(features.BackgroundMode, false); err != nil {
					t.Fatal(err)
				}
			})
			var stderr bytes.Buffer
			code := run([]string{"--config", path}, env{
				Stderr: &stderr,
				Wait:   func() { t.Fatal("norkad started while background mode is off") },
			})
			if code != exitBackgroundOff {
				t.Fatalf("exit %d, stderr %s", code, stderr.String())
			}
			text := stderr.String()
			if !strings.Contains(text, "background mode is off") || !strings.Contains(text, "Фоновый режим") || !strings.Contains(text, "--force") {
				t.Fatalf("stderr %s", text)
			}
		})
	}
}

func TestRunStartsWhenAllowed(t *testing.T) {
	isolateHome(t)
	cases := []struct {
		name  string
		on    bool
		force bool
	}{
		{name: "explicit-on", on: true},
		{name: "force", on: false, force: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, func(cfg *conf.Config) {
				quietDaemonFeatures(t, cfg, tc.on)
			})
			args := []string{"--config", path, "--foreground"}
			if tc.force {
				args = append(args, "--force")
			}
			var stderr bytes.Buffer
			// StartAutoStart returns before its goroutine finishes. Wait until
			// that goroutine is done so TempDir cleanup does not race it.
			code := run(args, env{Stderr: &stderr, Wait: func() { time.Sleep(200 * time.Millisecond) }})
			if code != exitOK {
				t.Fatalf("exit %d, stderr %s", code, stderr.String())
			}
		})
	}
}

func TestVersionAndUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, env{Stdout: &stdout, Stderr: &stderr, Version: "1.2.3"}); code != exitOK || strings.TrimSpace(stdout.String()) != "norkad 1.2.3" {
		t.Fatalf("version code %d out %q err %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--help"}, env{Stdout: &stdout, Stderr: &stderr}); code != exitOK || !strings.Contains(stdout.String(), "--foreground") {
		t.Fatalf("help code %d out %q", code, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--nope"}, env{Stdout: &stdout, Stderr: &stderr}); code != exitUsage || !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("usage code %d err %q", code, stderr.String())
	}
}

func writeConfig(t *testing.T, mutate func(*conf.Config)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		if mutate != nil {
			mutate(cfg)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return path
}

func quietDaemonFeatures(t *testing.T, cfg *conf.Config, background bool) {
	t.Helper()
	for _, item := range []struct {
		id      features.ID
		enabled bool
	}{
		{features.BackgroundMode, background},
		{features.WakeReconnect, false},
		{features.Automation, false},
	} {
		if err := cfg.Features.Set(item.id, item.enabled); err != nil {
			t.Fatal(err)
		}
	}
}

func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
}
