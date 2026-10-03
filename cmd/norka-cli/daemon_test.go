package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/engine"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/notify"
)

func TestV2CommandsAgainstInProcessOwner(t *testing.T) {
	t.Run("daemon", func(t *testing.T) {
		eng, path := listenOwner(t, engine.KindDaemon)
		var stdout bytes.Buffer
		e := liveEnv(t, path, &stdout, &bytes.Buffer{})
		if code := run([]string{"status", "--json"}, e); code != 0 {
			t.Fatalf("status %d %s", code, stdout.String())
		}
		if !strings.Contains(stdout.String(), `"owner": "daemon"`) || !strings.Contains(stdout.String(), `"protocol_version": 2`) {
			t.Fatalf("stdout %s", stdout.String())
		}
		stdout.Reset()
		if code := run([]string{"daemon", "handover"}, e); code != 0 {
			t.Fatalf("handover %d %s", code, stdout.String())
		}
		if !strings.Contains(stdout.String(), "engine lock is released") {
			t.Fatalf("stdout %s", stdout.String())
		}
		select {
		case <-eng.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("handover did not ask the owner to leave")
		}
		next, err := engine.Acquire(path, engine.KindGUI, "next")
		if err != nil {
			t.Fatalf("Acquire after handover: %v", err)
		}
		_ = next.Release()
	})

	t.Run("gui ignores stop", func(t *testing.T) {
		eng, path := listenOwner(t, engine.KindGUI)
		var stdout, stderr bytes.Buffer
		e := liveEnv(t, path, &stdout, &stderr)
		if code := run([]string{"daemon", "stop"}, e); code == 0 || !strings.Contains(stderr.String(), "--force") {
			t.Fatalf("code %d stderr %q", code, stderr.String())
		}
		select {
		case <-eng.Done():
			t.Fatal("the window stopped without --force")
		default:
		}
		stdout.Reset()
		stderr.Reset()
		if code := run([]string{"daemon", "stop", "--force"}, e); code != 0 {
			t.Fatalf("force %d stderr %q", code, stderr.String())
		}
		select {
		case <-eng.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("--force did not stop the window")
		}
	})
}

func TestV2CommandsAgainstNorkad(t *testing.T) {
	bin := buildNorkad(t)

	t.Run("status and stop", func(t *testing.T) {
		path, runtimeDir := daemonConfig(t)
		cmd, output, exited := startNorkad(t, bin, path, runtimeDir)
		t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
		waitDaemon(t, path, exited, output)

		var stdout, stderr bytes.Buffer
		e := liveEnv(t, path, &stdout, &stderr)
		legacyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		token, err := ipc.ReadToken(ipc.TokenPath(path))
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		address, err := ipc.Address(path)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		legacy, err := ipc.Call(legacyCtx, address, token, ipc.Request{Op: ipc.OpStatus})
		cancel()
		if err != nil || !legacy.OK || legacy.V != ipc.ProtocolVersion {
			t.Fatalf("v1 status %+v err %v", legacy, err)
		}

		if code := run([]string{"status", "--json"}, e); code != 0 {
			t.Fatalf("status %d stdout %s stderr %s\n%s", code, stdout.String(), stderr.String(), output.String())
		}
		if !strings.Contains(stdout.String(), `"owner": "daemon"`) || !strings.Contains(stdout.String(), `"pid":`) {
			t.Fatalf("stdout %s", stdout.String())
		}
		stdout.Reset()
		if code := run([]string{"daemon", "stop"}, e); code != 0 {
			t.Fatalf("stop %d stdout %s stderr %s\n%s", code, stdout.String(), stderr.String(), output.String())
		}
		waitExit(t, cmd, exited, output, 0)
		if _, err := engine.Acquire(path, engine.KindGUI, "after"); err != nil {
			t.Fatalf("lock after stop: %v", err)
		}
	})

	t.Run("handover", func(t *testing.T) {
		path, runtimeDir := daemonConfig(t)
		cmd, output, exited := startNorkad(t, bin, path, runtimeDir)
		t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
		waitDaemon(t, path, exited, output)

		var stdout, stderr bytes.Buffer
		e := liveEnv(t, path, &stdout, &stderr)
		if code := run([]string{"daemon", "handover"}, e); code != 0 {
			t.Fatalf("handover %d stdout %s stderr %s\n%s", code, stdout.String(), stderr.String(), output.String())
		}
		waitExit(t, cmd, exited, output, 0)
		owner, err := engine.Acquire(path, engine.KindGUI, "after")
		if err != nil {
			t.Fatalf("Acquire after handover: %v\n%s", err, output.String())
		}
		_ = owner.Release()
	})
}

func listenOwner(t *testing.T, kind engine.Kind) (*engine.Engine, string) {
	t.Helper()
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	path := filepath.Join(t.TempDir(), "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		if err := cfg.Features.Set(features.Automation, true); err != nil {
			return err
		}
		if err := cfg.Features.Set(features.WakeReconnect, false); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	eng := engine.New(engine.Options{
		Storage:    storage,
		Watch:      func(context.Context) (<-chan netwatch.Event, error) { return nil, context.Canceled },
		SyncLogin:  func(bool, bool) error { return nil },
		SyncScheme: func(bool) error { return nil },
		Poster:     func(notify.PosterConfig) notify.Poster { return discardPoster{} },
	})
	eng.SetVersion("test")
	if _, err := eng.Acquire(kind); err != nil {
		t.Fatal(err)
	}
	eng.Start()
	t.Cleanup(eng.Shutdown)
	waitOwner(t, path)
	return eng, path
}

func liveEnv(t *testing.T, path string, stdout, stderr *bytes.Buffer) env {
	t.Helper()
	return env{
		Stdout:  stdout,
		Stderr:  stderr,
		Version: "test",
		Load: func() (*conf.Config, string, error) {
			storage, err := conf.NewStorage(path)
			if err != nil {
				return nil, path, err
			}
			cfg, err := storage.Load()
			return cfg, path, err
		},
		Address:     ipc.Address,
		TokenPath:   ipc.TokenPath,
		ReadToken:   ipc.ReadToken,
		Call:        ipc.Call,
		ReadAppPath: func(string) (string, error) { return "", os.ErrNotExist },
		Launch: func(string) error {
			t.Fatal("daemon commands must not launch the app")
			return nil
		},
		Wait: func(context.Context, string, string) error { return nil },
	}
}

func waitOwner(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		token, err := ipc.ReadToken(ipc.TokenPath(path))
		if err == nil {
			address, addrErr := ipc.Address(path)
			if addrErr != nil {
				t.Fatal(addrErr)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, err = ipc.Call(ctx, address, token, ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpHello})
			cancel()
			if err == nil {
				return
			}
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("owner ipc: %v", last)
}

type discardPoster struct{}

func (discardPoster) Post(notify.Notice) error { return nil }

func daemonConfig(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		for _, item := range []struct {
			id      features.ID
			enabled bool
		}{
			{features.BackgroundMode, true},
			{features.Automation, true},
			{features.WakeReconnect, false},
		} {
			if err := cfg.Features.Set(item.id, item.enabled); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return path, t.TempDir()
}

func buildNorkad(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "norkad")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/norkad")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build norkad: %v\n%s", err, out)
	}
	return bin
}

func startNorkad(t *testing.T, bin, path, runtimeDir string) (*exec.Cmd, *bytes.Buffer, <-chan error) {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(bin, "--config", path, "--foreground")
	cmd.Env = mergeEnv(map[string]string{
		"HOME":            home,
		"USERPROFILE":     home,
		"XDG_CONFIG_HOME": filepath.Join(home, "xdg"),
		"XDG_RUNTIME_DIR": runtimeDir,
	})
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	return cmd, &output, exited
}

func waitDaemon(t *testing.T, path string, exited <-chan error, output *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			t.Fatalf("norkad exited early: %v\n%s", err, output.String())
		default:
		}
		token, err := ipc.ReadToken(ipc.TokenPath(path))
		if err == nil {
			address, addrErr := ipc.Address(path)
			if addrErr != nil {
				t.Fatal(addrErr)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, err = ipc.Call(ctx, address, token, ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpHello})
			cancel()
			if err == nil {
				return
			}
		}
		last = err
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("norkad ipc: %v\n%s", last, output.String())
}

func waitExit(t *testing.T, cmd *exec.Cmd, exited <-chan error, output *bytes.Buffer, want int) {
	t.Helper()
	select {
	case err := <-exited:
		if want == 0 && err != nil {
			t.Fatalf("exit %v\n%s", err, output.String())
		}
		if want != 0 {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != want {
				t.Fatalf("exit %v want %d\n%s", err, want, output.String())
			}
		}
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("norkad did not exit\n%s", output.String())
	}
}

func mergeEnv(overrides map[string]string) []string {
	out := make([]string, 0, len(os.Environ()))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, ok := overrides[key]; ok {
			continue
		}
		out = append(out, item)
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
