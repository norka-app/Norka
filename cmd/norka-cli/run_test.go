package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/ipc"
)

func enabledConfig() *conf.Config {
	cfg := conf.DefaultConfig()
	if err := cfg.Features.Set(features.Automation, true); err != nil {
		panic(err)
	}
	return cfg
}

func testEnv(cfg *conf.Config, stdout, stderr *bytes.Buffer) env {
	return env{
		Stdout:  stdout,
		Stderr:  stderr,
		Version: "test",
		Load: func() (*conf.Config, string, error) {
			return cfg, "/tmp/config.toml", nil
		},
		Address:   func(string) (string, error) { return "sock", nil },
		TokenPath: func(string) string { return "token" },
		ReadToken: func(string) (string, error) { return "token", nil },
		Call: func(context.Context, string, string, ipc.Request) (ipc.Response, error) {
			return ipc.Response{}, ipc.ErrNotRunning
		},
		ReadAppPath: func(string) (string, error) { return "", os.ErrNotExist },
		Launch:      func(string) error { return errors.New("launch was not expected") },
		Wait:        func(context.Context, string, string) error { return nil },
	}
}

func TestVersionAndHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	loaded := false
	e := testEnv(conf.DefaultConfig(), &stdout, &stderr)
	e.Load = func() (*conf.Config, string, error) {
		loaded = true
		return conf.DefaultConfig(), "c", nil
	}
	if code := run([]string{"--version"}, e); code != 0 || stdout.String() != "norka-cli test\n" || loaded {
		t.Fatalf("version code %d out %q loaded %v", code, stdout.String(), loaded)
	}
	stdout.Reset()
	if code := run([]string{"--help"}, e); code != 0 || !strings.Contains(stdout.String(), "norka-cli connect") || !strings.Contains(stdout.String(), "Exit codes:") || loaded {
		t.Fatalf("help code %d\n%s", code, stdout.String())
	}
	stderr.Reset()
	if code := run([]string{"connect"}, e); code != ipc.ExitUsage || !strings.Contains(stderr.String(), "needs one") {
		t.Fatalf("usage code %d\n%s", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{"nope"}, e); code != ipc.ExitUsage {
		t.Fatalf("unknown %d", code)
	}
}

func TestDisabledDoesNotDial(t *testing.T) {
	var stdout, stderr bytes.Buffer
	called := false
	e := testEnv(conf.DefaultConfig(), &stdout, &stderr)
	e.Call = func(context.Context, string, string, ipc.Request) (ipc.Response, error) {
		called = true
		return ipc.Response{}, nil
	}
	e.Launch = func(string) error {
		called = true
		return nil
	}
	code := run([]string{"status"}, e)
	if code != ipc.ExitDisabled || called {
		t.Fatalf("code %d called %v", code, called)
	}
	if !strings.Contains(stderr.String(), "Settings → Features") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestStatusNotRunning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	e := testEnv(enabledConfig(), &stdout, &stderr)
	launched := false
	e.Launch = func(string) error {
		launched = true
		return nil
	}
	e.ReadToken = func(string) (string, error) { return "", os.ErrNotExist }
	code := run([]string{"status", "--json"}, e)
	if code != ipc.ExitNotRunning || launched {
		t.Fatalf("code %d launched %v", code, launched)
	}
	if !strings.Contains(stdout.String(), `"code": "not_running"`) || stderr.Len() != 0 {
		t.Fatalf("stdout %s stderr %s", stdout.String(), stderr.String())
	}

	stdout.Reset()
	code = run([]string{"status"}, e)
	if code != ipc.ExitNotRunning || !strings.Contains(stderr.String(), "Norka is not running.") {
		t.Fatalf("human code %d stderr %q", code, stderr.String())
	}
}

func TestConnectMissingAppPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	e := testEnv(enabledConfig(), &stdout, &stderr)
	e.ReadToken = func(string) (string, error) { return "", os.ErrNotExist }
	launched := false
	e.Launch = func(string) error {
		launched = true
		return nil
	}
	code := run([]string{"connect", "db"}, e)
	if code != ipc.ExitNotRunning || launched {
		t.Fatalf("code %d launched %v", code, launched)
	}
	if !strings.Contains(stderr.String(), "Start Norka once") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestConnectLaunchesThenCalls(t *testing.T) {
	var stdout bytes.Buffer
	e := testEnv(enabledConfig(), &stdout, &bytes.Buffer{})
	ready := false
	var ops []string
	e.ReadToken = func(string) (string, error) {
		if !ready {
			return "", os.ErrNotExist
		}
		return "token", nil
	}
	e.ReadAppPath = func(string) (string, error) { return "/usr/bin/norka", nil }
	e.Launch = func(exe string) error {
		if exe != "/usr/bin/norka" {
			t.Fatalf("exe %s", exe)
		}
		ready = true
		return nil
	}
	e.Call = func(_ context.Context, address, token string, req ipc.Request) (ipc.Response, error) {
		ops = append(ops, req.Op)
		if address != "sock" || token != "token" || req.V != ipc.ProtocolVersion {
			t.Fatalf("call %+v %s %s", req, address, token)
		}
		if req.Op == ipc.OpConnect && req.Target != "db" {
			t.Fatalf("target %s", req.Target)
		}
		return ipc.Response{
			V:        ipc.ProtocolVersion,
			OK:       true,
			Code:     ipc.CodeOK,
			ExitCode: ipc.ExitOK,
			Message:  "Туннель «db» подключён.",
			Tunnels:  []ipc.TunnelInfo{{ID: 1, Name: "db", Status: "running", Mode: "local", LocalHost: "127.0.0.1", LocalPort: 5432}},
		}, nil
	}
	code := run([]string{"connect", "db"}, e)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(ops) != 2 || ops[0] != ipc.OpStatus || ops[1] != ipc.OpConnect {
		t.Fatalf("ops %v", ops)
	}
	if !strings.Contains(stdout.String(), `Tunnel "db" is connected.`) {
		t.Fatalf("stdout %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "подключён") {
		t.Fatalf("server locale leaked: %s", stdout.String())
	}
}

func TestDisconnectDoesNotLaunch(t *testing.T) {
	var stderr bytes.Buffer
	e := testEnv(enabledConfig(), &bytes.Buffer{}, &stderr)
	e.ReadToken = func(string) (string, error) { return "", os.ErrNotExist }
	looked := false
	e.ReadAppPath = func(string) (string, error) {
		looked = true
		return "/usr/bin/norka", nil
	}
	code := run([]string{"disconnect", "db"}, e)
	if code != ipc.ExitNotRunning || looked {
		t.Fatalf("code %d looked %v", code, looked)
	}
}

func TestOldServerDoesNotMutate(t *testing.T) {
	var stderr bytes.Buffer
	e := testEnv(enabledConfig(), &bytes.Buffer{}, &stderr)
	ops := 0
	e.Call = func(context.Context, string, string, ipc.Request) (ipc.Response, error) {
		ops++
		return ipc.Response{OK: true, Code: ipc.CodeOK, ExitCode: ipc.ExitOK, Message: "ran"}, nil
	}
	code := run([]string{"toggle", "db"}, e)
	if code != ipc.ExitVersion || ops != 1 {
		t.Fatalf("code %d ops %d", code, ops)
	}
	if !strings.Contains(stderr.String(), "Update Norka.") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestServerAsksToUpdateCLI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	e := testEnv(enabledConfig(), &stdout, &stderr)
	e.Call = func(context.Context, string, string, ipc.Request) (ipc.Response, error) {
		return ipc.Response{
			V:        ipc.ProtocolVersion,
			Code:     ipc.CodeUpdateCLI,
			ExitCode: ipc.ExitVersion,
			Message:  ipc.MessageUpdateCLI,
		}, nil
	}
	code := run([]string{"list", "--json"}, e)
	if code != ipc.ExitVersion || !strings.Contains(stdout.String(), `"code": "update_cli"`) {
		t.Fatalf("code %d stdout %s", code, stdout.String())
	}
}

func TestStatusHumanAndJSON(t *testing.T) {
	tunnels := []ipc.TunnelInfo{
		{ID: 1, Name: "db", Status: "running", Mode: "local", LocalHost: "127.0.0.1", LocalPort: 5432},
		{ID: 2, Name: "api", Status: "stopped", Mode: "dynamic", LocalHost: "127.0.0.1", LocalPort: 1080},
	}
	reply := ipc.Response{
		V:        ipc.ProtocolVersion,
		OK:       true,
		Code:     ipc.CodeOK,
		ExitCode: ipc.ExitOK,
		Message:  "Norka запущена. Туннелей: 2, подключено: 1.",
		Tunnels:  tunnels,
	}
	var stdout bytes.Buffer
	e := testEnv(enabledConfig(), &stdout, &bytes.Buffer{})
	e.Call = func(context.Context, string, string, ipc.Request) (ipc.Response, error) {
		return reply, nil
	}
	if code := run([]string{"status"}, e); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(stdout.String(), "Norka is running. Tunnels: 2, connected: 1.") {
		t.Fatalf("stdout %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "1\tdb\trunning\tlocal\t127.0.0.1:5432\n") {
		t.Fatalf("table %s", stdout.String())
	}
	stdout.Reset()
	if code := run([]string{"--json", "list"}, e); code != 0 || !strings.Contains(stdout.String(), `"name": "api"`) {
		t.Fatalf("json code %d %s", code, stdout.String())
	}
}

func TestStatusJSONSpeaksV2(t *testing.T) {
	var stdout bytes.Buffer
	var ops []string
	e := testEnv(enabledConfig(), &stdout, &bytes.Buffer{})
	e.Call = func(_ context.Context, _, _ string, req ipc.Request) (ipc.Response, error) {
		ops = append(ops, req.Op)
		if req.V != ipc.ProtocolVersion {
			t.Fatalf("dialect %d", req.V)
		}
		if req.Op == ipc.OpHello {
			if req.Client == nil || req.Client.Name != "norka-cli" || req.Client.Version != "test" || req.Client.Protocol != ipc.ProtocolVersion {
				t.Fatalf("client %+v", req.Client)
			}
			return ipc.Response{
				V:        ipc.ProtocolVersion,
				OK:       true,
				Code:     ipc.CodeOK,
				ExitCode: ipc.ExitOK,
				Hello:    &ipc.Hello{ProtocolVersion: 2, MinClientVersion: 1, Owner: "daemon", PID: 42, Version: "dev"},
			}, nil
		}
		if req.Op != ipc.OpState {
			t.Fatalf("op %s", req.Op)
		}
		return ipc.Response{
			V:        ipc.ProtocolVersion,
			OK:       true,
			Code:     ipc.CodeOK,
			ExitCode: ipc.ExitOK,
			State:    &ipc.State{Tunnels: []ipc.TunnelInfo{{ID: 1, Name: "db", Status: "running"}}},
		}, nil
	}
	if code := run([]string{"status", "--json"}, e); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(ops) != 2 || ops[0] != ipc.OpHello || ops[1] != ipc.OpState {
		t.Fatalf("ops %v", ops)
	}
	text := stdout.String()
	if !strings.Contains(text, `"protocol_version": 2`) || !strings.Contains(text, `"owner": "daemon"`) || !strings.Contains(text, `"name": "db"`) {
		t.Fatalf("stdout %s", text)
	}
}

func TestDaemonCommandsUseV2(t *testing.T) {
	var got ipc.Request
	e := testEnv(enabledConfig(), &bytes.Buffer{}, &bytes.Buffer{})
	e.Call = func(_ context.Context, _, _ string, req ipc.Request) (ipc.Response, error) {
		got = req
		return ipc.Response{V: ipc.ProtocolVersion, OK: true, Code: ipc.CodeOK, ExitCode: ipc.ExitOK, Message: "stopping"}, nil
	}
	var stdout bytes.Buffer
	e.Stdout = &stdout
	if code := run([]string{"daemon", "stop"}, e); code != 0 || !strings.Contains(stdout.String(), "Norka is stopping.") {
		t.Fatalf("stop code %d out %q req %+v", code, stdout.String(), got)
	}
	if got.Op != ipc.OpShutdown || got.V != ipc.ProtocolVersion || got.Force {
		t.Fatalf("stop request %+v", got)
	}
	stdout.Reset()
	if code := run([]string{"daemon", "stop", "--force"}, e); code != 0 || !got.Force {
		t.Fatalf("force code %d force %v", code, got.Force)
	}
	stdout.Reset()
	if code := run([]string{"daemon", "handover"}, e); code != 0 || !strings.Contains(stdout.String(), "engine lock is released") {
		t.Fatalf("handover code %d out %q", code, stdout.String())
	}
	if got.Op != ipc.OpHandover || got.TimeoutMS != int(ipc.DefaultHandoverTimeout/time.Millisecond) {
		t.Fatalf("handover request %+v", got)
	}
	var stderr bytes.Buffer
	e.Stderr = &stderr
	if code := run([]string{"daemon"}, e); code != ipc.ExitUsage || !strings.Contains(stderr.String(), "stop or handover") {
		t.Fatalf("usage code %d %q", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{"status", "--force"}, e); code != ipc.ExitUsage {
		t.Fatalf("--force on status: %d", code)
	}
}

func TestAmbiguous(t *testing.T) {
	var stderr bytes.Buffer
	e := testEnv(enabledConfig(), &bytes.Buffer{}, &stderr)
	calls := 0
	e.Call = func(_ context.Context, _, _ string, req ipc.Request) (ipc.Response, error) {
		calls++
		if calls == 1 {
			return ipc.Response{V: ipc.ProtocolVersion, OK: true, Code: ipc.CodeOK}, nil
		}
		return ipc.Response{
			V:        ipc.ProtocolVersion,
			Code:     ipc.CodeAmbiguous,
			ExitCode: ipc.ExitAmbiguous,
			Names:    []string{"db-a", "db-b"},
			Message:  "Имя подходит нескольким",
		}, nil
	}
	code := run([]string{"connect", "db"}, e)
	if code != ipc.ExitAmbiguous || !strings.Contains(stderr.String(), "db-a") || strings.Contains(stderr.String(), "Имя") {
		t.Fatalf("code %d stderr %q", code, stderr.String())
	}
}
