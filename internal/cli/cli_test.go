package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"norka/internal/conf"
	"norka/internal/features"
	"norka/internal/ipc"
)

func TestParseCommands(t *testing.T) {
	cmd, err := Parse([]string{"connect", "db"})
	if err != nil || cmd.Op != "connect" || cmd.Target != "db" || cmd.JSON {
		t.Fatalf("%+v %v", cmd, err)
	}
	cmd, err = Parse([]string{"--json", "status"})
	if err != nil || cmd.Op != "status" || !cmd.JSON {
		t.Fatalf("%+v %v", cmd, err)
	}
	cmd, err = Parse([]string{"list", "--json"})
	if err != nil || cmd.Op != "list" || !cmd.JSON {
		t.Fatalf("%+v %v", cmd, err)
	}
	cmd, err = Parse([]string{"toggle", "4"})
	if err != nil || cmd.Target != "4" {
		t.Fatalf("%+v %v", cmd, err)
	}
	if _, err = Parse([]string{"connect"}); err == nil {
		t.Fatal("connect needs a name")
	}
	if _, err = Parse([]string{"status", "db"}); err == nil {
		t.Fatal("status takes no name")
	}
	if _, err = Parse([]string{"nope"}); err == nil {
		t.Fatal("unknown command")
	}
	cmd, err = Parse([]string{"--help"})
	if err != nil || !cmd.Help {
		t.Fatalf("help: %+v %v", cmd, err)
	}
	if !IsCommand([]string{"status"}) || IsCommand([]string{"norka://connect/db"}) {
		t.Fatal("command detection")
	}
	args, hidden := ConsumeHidden([]string{"norka", HiddenArg, "norka://open/db"})
	if !hidden || len(args) != 2 || args[1] != "norka://open/db" {
		t.Fatalf("hidden args: %v %v", args, hidden)
	}
}

func TestDisabledDoesNotDial(t *testing.T) {
	for _, name := range []string{"missing", "explicit"} {
		t.Run(name, func(t *testing.T) {
			cfg := conf.DefaultConfig()
			cfg.Language = "ru"
			if name == "explicit" {
				if err := cfg.Features.Set(features.Automation, false); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			called := false
			code := Run(Env{
				Stdout: &stdout,
				Stderr: &stderr,
				Load: func() (*conf.Config, string, string, error) {
					return cfg, "/tmp/config.toml", "ru", nil
				},
				Call: func(context.Context, string, string, ipc.Request) (ipc.Response, error) {
					called = true
					return ipc.Response{}, nil
				},
				Launch: func() error {
					called = true
					return nil
				},
			}, []string{"status"})
			if code != ipc.ExitDisabled {
				t.Fatalf("exit %d", code)
			}
			if called {
				t.Fatal("disabled automation dialed the local channel")
			}
			if !strings.Contains(stderr.String(), "Настройки → Функции") {
				t.Fatalf("stderr: %s", stderr.String())
			}
		})
	}
}

func TestStatusNotRunning(t *testing.T) {
	cfg := conf.DefaultConfig()
	if err := cfg.Features.Set(features.Automation, true); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	launched := false
	code := Run(Env{
		Stdout: &stdout,
		Stderr: &stderr,
		Load: func() (*conf.Config, string, string, error) {
			return cfg, "/tmp/config.toml", "en", nil
		},
		Address:   func(string) (string, error) { return "sock", nil },
		TokenPath: func(string) string { return "token" },
		ReadToken: func(string) (string, error) { return "", os.ErrNotExist },
		Call: func(context.Context, string, string, ipc.Request) (ipc.Response, error) {
			return ipc.Response{}, ipc.ErrNotRunning
		},
		Launch: func() error {
			launched = true
			return nil
		},
	}, []string{"status", "--json"})
	if code != ipc.ExitNotRunning || launched {
		t.Fatalf("exit %d launched %v", code, launched)
	}
	if !strings.Contains(stdout.String(), `"code": "not_running"`) {
		t.Fatalf("stdout: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr: %s", stderr.String())
	}
}

func TestConnectLaunchesThenCalls(t *testing.T) {
	cfg := conf.DefaultConfig()
	if err := cfg.Features.Set(features.Automation, true); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	launched := false
	ready := false
	calls := 0
	code := Run(Env{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Load: func() (*conf.Config, string, string, error) {
			return cfg, "/tmp/config.toml", "ru", nil
		},
		Address:   func(string) (string, error) { return "sock", nil },
		TokenPath: func(string) string { return "token" },
		ReadToken: func(string) (string, error) {
			if !ready {
				return "", os.ErrNotExist
			}
			return "token", nil
		},
		Call: func(_ context.Context, address, token string, req ipc.Request) (ipc.Response, error) {
			calls++
			if address != "sock" || token != "token" || req.Op != ipc.OpConnect || req.Target != "db" {
				t.Fatalf("call %s %s %+v", address, token, req)
			}
			return ipc.Response{OK: true, Code: ipc.CodeOK, ExitCode: ipc.ExitOK, Message: "Туннель «db» подключён."}, nil
		},
		Launch: func() error {
			ready = true
			launched = true
			return nil
		},
		Wait: func(context.Context, string, string) error { return nil },
	}, []string{"connect", "db"})
	if code != 0 || !launched || calls != 1 {
		t.Fatalf("exit %d launched %v calls %d", code, launched, calls)
	}
	if !strings.Contains(stdout.String(), "подключён") {
		t.Fatalf("stdout: %s", stdout.String())
	}
}

func TestUsageExitCode(t *testing.T) {
	var stderr bytes.Buffer
	code := Run(Env{
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
		Load: func() (*conf.Config, string, string, error) {
			return conf.DefaultConfig(), "c", "en", nil
		},
	}, []string{"connect"})
	if code != ipc.ExitUsage {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "Exit codes:") {
		t.Fatalf("usage: %s", stderr.String())
	}
}

func TestNotRunningError(t *testing.T) {
	if !errors.Is(ipc.ErrNotRunning, ipc.ErrNotRunning) {
		t.Fatal("sentinel")
	}
}
