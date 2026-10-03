package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/engine"
	"github.com/norka-app/Norka/internal/features"
)

func TestExitWhenOwnedByOther(t *testing.T) {
	path := writeConfig(t, func(cfg *conf.Config) {
		if err := cfg.Features.Set(features.BackgroundMode, true); err != nil {
			t.Fatal(err)
		}
	})
	owner, err := engine.Acquire(path, engine.KindGUI, "gui-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Release() })

	var out bytes.Buffer
	cmd := exec.Command(os.Args[0], "--config", path, "--foreground")
	cmd.Env = daemonEnv(t)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitOwned {
		t.Fatalf("err %v output %s", err, out.String())
	}
	text := out.String()
	wantPID := fmt.Sprintf("pid %d", owner.Meta().PID)
	if !strings.Contains(text, wantPID) || !strings.Contains(text, "(gui)") {
		t.Fatalf("output %s", text)
	}
}

func daemonEnv(t *testing.T) []string {
	t.Helper()
	home := t.TempDir()
	return append(os.Environ(),
		"NORKAD_TEST_RUN=1",
		"HOME="+home,
		"USERPROFILE="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "xdg"),
	)
}
