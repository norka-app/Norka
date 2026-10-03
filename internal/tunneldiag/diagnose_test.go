package tunneldiag

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"norka/internal/forward"
	"norka/internal/model"
)

func TestRunKeyTunnelExplainsReachability(t *testing.T) {
	var probed []model.Jumper
	var targetHost string
	var targetPort int
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID:         7,
			Name:       "db",
			Mode:       "local",
			LocalHost:  "127.0.0.1",
			LocalPort:  15432,
			RemoteHost: "10.0.0.8",
			RemotePort: 5432,
			Status:     "stopped",
		},
		Jumpers: []model.Jumper{{
			Host:     "bastion.example",
			Port:     22,
			User:     "dev",
			AuthType: "ssh_agent",
		}},
	}, &Hooks{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		},
		DialContext: dialOK,
		Listen:      listenOK,
		Probe: func(_ context.Context, jumpers []model.Jumper, host string, port int) (forward.ChainProbe, error) {
			probed = jumpers
			targetHost = host
			targetPort = port
			return forward.ChainProbe{
				Latency:       15 * time.Millisecond,
				TargetChecked: true,
			}, nil
		},
	})

	if report.Status != "ok" || report.Code != "summary_ok" {
		t.Fatalf("summary %+v", report)
	}
	if len(probed) != 1 || targetHost != "10.0.0.8" || targetPort != 5432 {
		t.Fatalf("probe jumpers=%d target=%s:%d", len(probed), targetHost, targetPort)
	}
	assertCode(t, report, "dns", "ok", "dns_ok")
	assertCode(t, report, "tcp", "ok", "tcp_ok")
	assertCode(t, report, "listen", "ok", "listen_free")
	assertCode(t, report, "auth", "ok", "auth_ok")
	assertCode(t, report, "latency", "ok", "latency_ssh")
	assertCode(t, report, "target", "ok", "target_ok")
	if got := checkByID(report, "latency").Params["ms"]; got != "15" {
		t.Fatalf("latency ms %q", got)
	}
}

func TestRunSkipsPasswordAndRedactsSecrets(t *testing.T) {
	const secret = "super-secret-value"
	probed := false
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID: 1, Name: "api", Mode: "local",
			LocalHost: "127.0.0.1", LocalPort: 18080,
			RemoteHost: "10.1.0.4", RemotePort: 80, Status: "error",
		},
		Jumpers: []model.Jumper{{
			Host: "203.0.113.20", Port: 22, User: "root",
			AuthType: "password", Password: secret,
		}},
	}, &Hooks{
		DialContext: dialOK,
		Listen:      listenOK,
		Banner: func(context.Context, string) (string, error) {
			return "SSH-2.0-OpenSSH_9.6", nil
		},
		Probe: func(context.Context, []model.Jumper, string, int) (forward.ChainProbe, error) {
			probed = true
			return forward.ChainProbe{}, errors.New("should not probe a password")
		},
	})

	if probed {
		t.Fatal("password hop was probed")
	}
	if report.Code != "summary_password" {
		t.Fatalf("summary %s", report.Code)
	}
	auth := assertCode(t, report, "auth", "skipped", "auth_skipped_password")
	if auth.Detail != "SSH-2.0-OpenSSH_9.6" {
		t.Fatalf("banner detail %q", auth.Detail)
	}
	assertNoSecret(t, report, secret)
}

func TestRunRedactsPasswordFromAuthError(t *testing.T) {
	const secret = "super-secret-value"
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID: 2, Name: "leak", Mode: "dynamic",
			LocalHost: "127.0.0.1", LocalPort: 1080, Status: "stopped",
		},
		Jumpers: []model.Jumper{{
			Host: "203.0.113.21", Port: 22, User: "dev",
			AuthType: "ssh_agent", Password: secret,
		}},
	}, &Hooks{
		DialContext: dialOK,
		Listen:      listenOK,
		Probe: func(context.Context, []model.Jumper, string, int) (forward.ChainProbe, error) {
			return forward.ChainProbe{}, errors.New("auth failed " + secret + " " + pem)
		},
	})

	auth := assertCode(t, report, "auth", "error", "auth_error")
	if strings.Contains(auth.Detail, secret) || strings.Contains(auth.Detail, "PRIVATE KEY") {
		t.Fatalf("detail leaked: %s", auth.Detail)
	}
	if !strings.Contains(auth.Detail, "••••") {
		t.Fatalf("detail was not redacted: %s", auth.Detail)
	}
	assertCode(t, report, "target", "skipped", "target_skipped_dynamic")
	assertNoSecret(t, report, secret)
	assertNoSecret(t, report, "PRIVATE KEY")
}

func TestRunMissingKeyDoesNotDialSSH(t *testing.T) {
	probed := false
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID: 3, Name: "key", Mode: "local",
			LocalHost: "127.0.0.1", LocalPort: 12222,
			RemoteHost: "10.0.0.2", RemotePort: 22, Status: "stopped",
		},
		Jumpers: []model.Jumper{{
			Host: "bastion.example", Port: 22, User: "dev",
			AuthType: "ssh_key", KeyPath: filepath.Join(t.TempDir(), "missing"),
		}},
	}, &Hooks{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("203.0.113.30")}, nil
		},
		DialContext: dialOK,
		Listen:      listenOK,
		Probe: func(context.Context, []model.Jumper, string, int) (forward.ChainProbe, error) {
			probed = true
			return forward.ChainProbe{}, nil
		},
	})
	if probed {
		t.Fatal("missing key still probed")
	}
	assertCode(t, report, "auth", "error", "auth_key_missing")
	assertCode(t, report, "target", "skipped", "target_skipped_auth")
}

func TestRunLocalPortStates(t *testing.T) {
	base := model.Tunnel{
		ID: 4, Name: "web", Mode: "local",
		LocalHost: "127.0.0.1", LocalPort: 13000,
		RemoteHost: "10.0.0.3", RemotePort: 80,
	}
	jumper := []model.Jumper{{Host: "203.0.113.40", Port: 22, User: "dev", AuthType: "ssh_agent"}}
	hooks := &Hooks{DialContext: dialRefused, Listen: listenBusy, Probe: func(context.Context, []model.Jumper, string, int) (forward.ChainProbe, error) {
		t.Fatal("tcp failure should skip ssh")
		return forward.ChainProbe{}, nil
	}}

	stopped := base
	stopped.Status = "stopped"
	busy := Run(context.Background(), Input{Tunnel: stopped, Jumpers: jumper}, hooks)
	assertCode(t, busy, "listen", "error", "listen_busy")
	assertCode(t, busy, "tcp", "error", "tcp_refused")
	assertCode(t, busy, "auth", "skipped", "auth_skipped_down")

	running := base
	running.Status = "running"
	self := Run(context.Background(), Input{Tunnel: running, Jumpers: jumper}, hooks)
	assertCode(t, self, "listen", "ok", "listen_self")

	other := base
	other.Status = "stopped"
	held := Run(context.Background(), Input{
		Tunnel:  other,
		Jumpers: jumper,
		Siblings: []model.Tunnel{{
			ID: 9, Name: "other", Mode: "local", Status: "running",
			LocalHost: "127.0.0.1", LocalPort: 13000,
		}},
	}, hooks)
	listen := assertCode(t, held, "listen", "error", "listen_other")
	if listen.Params["name"] != "other" {
		t.Fatalf("holder %q", listen.Params["name"])
	}
}

func TestRunRemoteChecksLocalService(t *testing.T) {
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID: 5, Name: "rev", Mode: "remote", Status: "stopped",
			LocalHost: "127.0.0.1", LocalPort: 1,
			RemoteHost: "0.0.0.0", RemotePort: 2222,
		},
		Jumpers: []model.Jumper{{Host: "203.0.113.50", Port: 22, User: "dev", AuthType: "ssh_agent"}},
	}, &Hooks{
		DialContext: func(_ context.Context, _ string, address string) (net.Conn, error) {
			if strings.HasSuffix(address, ":22") {
				return dialOK(context.Background(), "tcp", address)
			}
			return nil, errors.New("connect: connection refused")
		},
		Listen: listenOK,
		Probe: func(context.Context, []model.Jumper, string, int) (forward.ChainProbe, error) {
			return forward.ChainProbe{Latency: 4 * time.Millisecond}, nil
		},
	})
	assertCode(t, report, "listen", "skipped", "listen_remote")
	assertCode(t, report, "target", "error", "target_local_refused")
}

func TestRunPrefixKeyThenPassword(t *testing.T) {
	var got int
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID: 6, Name: "chain", Mode: "local", Status: "stopped",
			LocalHost: "127.0.0.1", LocalPort: 14000,
			RemoteHost: "10.8.0.2", RemotePort: 443,
		},
		Jumpers: []model.Jumper{
			{Host: "edge.example", Port: 22, User: "jump", AuthType: "ssh_agent"},
			{Host: "inner.example", Port: 22, User: "dev", AuthType: "password", Password: "super-secret-value"},
		},
	}, &Hooks{
		LookupIP:    func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.60")}, nil },
		DialContext: dialOK,
		Listen:      listenOK,
		Probe: func(_ context.Context, jumpers []model.Jumper, host string, port int) (forward.ChainProbe, error) {
			got = len(jumpers)
			if host != "" || port != 0 {
				t.Fatalf("target dial on a partial chain %s:%d", host, port)
			}
			if jumpers[0].Password != "" {
				t.Fatal("password copied onto the safe hop")
			}
			return forward.ChainProbe{Latency: 8 * time.Millisecond}, nil
		},
	})
	if got != 1 {
		t.Fatalf("probed %d hops", got)
	}
	auth := assertCode(t, report, "auth", "skipped", "auth_prefix_ok")
	if auth.Params["next"] != "inner.example" {
		t.Fatalf("next %q", auth.Params["next"])
	}
	assertCode(t, report, "tcp", "ok", "tcp_ok_chain")
	assertCode(t, report, "target", "skipped", "target_skipped_auth")
	assertNoSecret(t, report, "super-secret-value")
}

func TestRunNoJumper(t *testing.T) {
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID: 8, Name: "empty", Mode: "local", Status: "stopped",
			LocalHost: "127.0.0.1", LocalPort: 15000,
			RemoteHost: "10.0.0.9", RemotePort: 25,
		},
	}, &Hooks{Listen: listenOK})
	if report.Code != "summary_error" || report.Params["count"] != "1" {
		t.Fatalf("summary %+v %v", report.Code, report.Params)
	}
	assertCode(t, report, "auth", "error", "auth_no_jumper")
	assertCode(t, report, "dns", "skipped", "dns_no_jumper")
}

func TestLiteralHostSkipsDNS(t *testing.T) {
	report := Run(context.Background(), Input{
		Tunnel: model.Tunnel{
			ID: 10, Name: "ip", Mode: "dynamic", Status: "stopped",
			LocalHost: "127.0.0.1", LocalPort: 1080,
		},
		Jumpers: []model.Jumper{{Host: "203.0.113.70", Port: 22, User: "dev", AuthType: "ssh_agent"}},
	}, &Hooks{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			t.Fatal("literal host was resolved")
			return nil, nil
		},
		DialContext: dialOK,
		Listen:      listenOK,
		Probe: func(context.Context, []model.Jumper, string, int) (forward.ChainProbe, error) {
			return forward.ChainProbe{Latency: time.Millisecond}, nil
		},
	})
	assertCode(t, report, "dns", "skipped", "dns_literal")
}

func dialOK(context.Context, string, string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		_, _ = io.Copy(io.Discard, server)
		_ = server.Close()
	}()
	return client, nil
}

func dialRefused(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("dial tcp: connect: connection refused")
}

func listenOK(string, string) (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func listenBusy(string, string) (net.Listener, error) {
	return nil, errors.New("listen tcp: bind: address already in use")
}

func assertCode(t *testing.T, report Report, id, status, code string) Check {
	t.Helper()
	check := checkByID(report, id)
	if check.Status != status || check.Code != code {
		t.Fatalf("%s: got %s/%s want %s/%s", id, check.Status, check.Code, status, code)
	}
	return check
}

func checkByID(report Report, id string) Check {
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	return Check{}
}

func assertNoSecret(t *testing.T, report Report, secret string) {
	t.Helper()
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("report contains %q: %s", secret, raw)
	}
}
