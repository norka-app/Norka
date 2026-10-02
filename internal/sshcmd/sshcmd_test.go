package sshcmd

import (
	"encoding/json"
	"strings"
	"testing"

	"norka/internal/model"
)

func TestParseTable(t *testing.T) {
	resolve := func(_, name string) (Alias, bool, error) {
		switch strings.ToLower(name) {
		case "dev-host":
			return Alias{
				Name:                "dev-host",
				Host:                "10.0.0.8",
				Port:                2222,
				User:                "deploy",
				KeyPath:             "/home/dev/.ssh/id_ed25519",
				KeepAliveIntervalMs: 15000,
				TimeoutMs:           8000,
				ProxyJump:           "jump@bastion.internal",
			}, true, nil
		case "bastion":
			return Alias{
				Name: "bastion",
				Host: "bastion.example",
				Port: 22,
				User: "ops",
			}, true, nil
		default:
			return Alias{}, false, nil
		}
	}

	tests := []struct {
		name      string
		input     string
		resolve   Resolver
		tunnels   int
		destUser  string
		destHost  string
		destPort  int
		jumpUsers []string
		keyPath   string
		keepAlive int
		timeout   int
		modes     []string
		warnCodes []string
		secret    string
	}{
		{
			name:      "local forward with harmless flags",
			input:     "ssh -fN -L 3000:127.0.0.1:3000 user@dev-host",
			tunnels:   1,
			destUser:  "user",
			destHost:  "dev-host",
			destPort:  22,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:      "several forwards one command",
			input:     "ssh -N -L 3000:127.0.0.1:3000 -R 0.0.0.0:9000:127.0.0.1:80 -D 1080 -D 127.0.0.1:1081 dev@db.internal",
			tunnels:   4,
			destUser:  "dev",
			destHost:  "db.internal",
			destPort:  22,
			modes:     []string{"local", "remote", "dynamic", "dynamic"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:      "port user key keepalive and jump",
			input:     "ssh -p 2200 -l dev -i ~/.ssh/id_ed25519 -o ServerAliveInterval=30 -o ConnectTimeout=8 -J jump@bastion.example:2222 -L 5432:127.0.0.1:5432 db.internal",
			tunnels:   1,
			destUser:  "dev",
			destHost:  "db.internal",
			destPort:  2200,
			jumpUsers: []string{"jump"},
			keyPath:   "~/.ssh/id_ed25519",
			keepAlive: 30000,
			timeout:   8000,
			modes:     []string{"local"},
		},
		{
			name:      "user at host and attached flags",
			input:     "ssh -fNv -L3000:10.0.0.4:80 -p2222 dev@dev-host",
			tunnels:   1,
			destUser:  "dev",
			destHost:  "dev-host",
			destPort:  2222,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:      "unknown option and secret option value dropped",
			input:     `ssh -X -W hidden.example:22 -o Foo=super-secret-value -o ProxyCommand="sshpass -p super-secret-value ssh -W %h:%p jump" -L 3000:127.0.0.1:3000 dev@dev-host`,
			tunnels:   1,
			destUser:  "dev",
			destHost:  "dev-host",
			destPort:  22,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
			warnCodes: []string{WarnUnknownOption, WarnIgnoredSecret},
			secret:    "super-secret-value",
		},
		{
			name:      "posix quotes",
			input:     `ssh -N -L '3000:127.0.0.1:3000' -i '/home/dev/my keys/id' 'dev@dev-host'`,
			tunnels:   1,
			destUser:  "dev",
			destHost:  "dev-host",
			destPort:  22,
			keyPath:   "/home/dev/my keys/id",
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:      "windows caret and quoted path",
			input:     `ssh -N -L "3000:127.0.0.1:3000" -i "C:\Users\dev\My Keys\id_ed25519" dev^@dev-host`,
			tunnels:   1,
			destUser:  "dev",
			destHost:  "dev-host",
			destPort:  22,
			keyPath:   `C:\Users\dev\My Keys\id_ed25519`,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:      "windows doubled quotes",
			input:     `ssh -N -i "C:\keys\my""id" -L 3000:127.0.0.1:80 dev@host`,
			tunnels:   1,
			destUser:  "dev",
			destHost:  "host",
			destPort:  22,
			keyPath:   `C:\keys\my"id`,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:    "two commands and line continuation",
			input:   "ssh -N -L 3000:127.0.0.1:3000 \\\n  dev@one && ssh -N -D 1080 dev@two",
			tunnels: 2,
			modes:   []string{"local", "dynamic"},
		},
		{
			name:      "ipv6 bind",
			input:     "ssh -N -L [::1]:3000:127.0.0.1:80 dev@host",
			tunnels:   1,
			destUser:  "dev",
			destHost:  "host",
			destPort:  22,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:      "alias from ssh config including proxyjump",
			input:     "ssh -L 3000:127.0.0.1:3000 dev-host",
			resolve:   resolve,
			tunnels:   1,
			destUser:  "deploy",
			destHost:  "10.0.0.8",
			destPort:  2222,
			jumpUsers: []string{"jump"},
			keyPath:   "/home/dev/.ssh/id_ed25519",
			keepAlive: 15000,
			timeout:   8000,
			modes:     []string{"local"},
		},
		{
			name:      "cli overrides alias user port key and jump",
			input:     "ssh -p 22 -l root -i /other/key -J other@edge -L 3000:127.0.0.1:80 dev-host",
			resolve:   resolve,
			tunnels:   1,
			destUser:  "root",
			destHost:  "10.0.0.8",
			destPort:  22,
			jumpUsers: []string{"other"},
			keyPath:   "/other/key",
			keepAlive: 15000,
			timeout:   8000,
			modes:     []string{"local"},
		},
		{
			name:      "jump alias",
			input:     "ssh -J bastion -L 9:127.0.0.1:9 dev@dest",
			resolve:   resolve,
			tunnels:   1,
			destUser:  "dev",
			destHost:  "dest",
			destPort:  22,
			jumpUsers: []string{"ops"},
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
		},
		{
			name:      "sshpass password is not stored",
			input:     "sshpass -p super-secret-value ssh -fN -L 3000:127.0.0.1:3000 dev@host",
			tunnels:   1,
			destUser:  "dev",
			destHost:  "host",
			destPort:  22,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
			warnCodes: []string{WarnIgnoredSecret},
			secret:    "super-secret-value",
		},
		{
			name:      "invalid forward is a warning",
			input:     "ssh -L nope -L 3000:127.0.0.1:80 dev@host",
			tunnels:   1,
			destUser:  "dev",
			destHost:  "host",
			destPort:  22,
			modes:     []string{"local"},
			keepAlive: defaultKeepAliveMs,
			timeout:   defaultTimeoutMs,
			warnCodes: []string{WarnInvalidForward},
		},
		{
			name:      "not an ssh command",
			input:     "echo hello",
			tunnels:   0,
			warnCodes: []string{WarnNotSSH},
		},
		{
			name:      "missing forward",
			input:     "ssh dev@host",
			tunnels:   0,
			warnCodes: []string{WarnNoForward},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preview, err := Preview(tt.input, tt.resolve, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Tunnels) != tt.tunnels {
				t.Fatalf("tunnels %d, want %d (%+v)", len(preview.Tunnels), tt.tunnels, preview)
			}
			for i, mode := range tt.modes {
				if preview.Tunnels[i].Mode != mode {
					t.Fatalf("tunnel %d mode %s, want %s", i, preview.Tunnels[i].Mode, mode)
				}
			}
			if tt.destHost != "" {
				dest := preview.Hosts[len(preview.Hosts)-1]
				if dest.User != tt.destUser || dest.Host != tt.destHost || dest.Port != tt.destPort {
					t.Fatalf("dest %+v, want %s@%s:%d", dest, tt.destUser, tt.destHost, tt.destPort)
				}
				if dest.KeyPath != tt.keyPath {
					t.Fatalf("key %q, want %q", dest.KeyPath, tt.keyPath)
				}
				if dest.KeepAliveIntervalMs != tt.keepAlive || dest.TimeoutMs != tt.timeout {
					t.Fatalf("timers alive=%d timeout=%d, want %d %d", dest.KeepAliveIntervalMs, dest.TimeoutMs, tt.keepAlive, tt.timeout)
				}
				if len(tt.jumpUsers) != len(preview.Hosts)-1 {
					t.Fatalf("jumps %+v, want users %v", preview.Hosts, tt.jumpUsers)
				}
				for i, user := range tt.jumpUsers {
					if preview.Hosts[i].User != user {
						t.Fatalf("jump %d user %s, want %s", i, preview.Hosts[i].User, user)
					}
				}
			}
			for _, code := range tt.warnCodes {
				if !hasWarning(preview, code) {
					t.Fatalf("missing warning %s in %+v", code, preview.Warnings)
				}
			}
			if tt.name == "two commands and line continuation" {
				if len(preview.Hosts) != 2 || preview.Hosts[0].Host != "one" || preview.Hosts[1].Host != "two" {
					t.Fatalf("hosts %+v", preview.Hosts)
				}
			}
			if tt.secret != "" {
				raw, err := json.Marshal(preview)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), tt.secret) {
					t.Fatalf("secret leaked: %s", raw)
				}
			}
			if strings.Contains(tt.input, "-fN") && hasWarning(preview, WarnUnknownOption) {
				t.Fatalf("harmless flags warned: %+v", preview.Warnings)
			}
		})
	}
}

func TestReuseExistingJumpHost(t *testing.T) {
	existing := []model.Jumper{{
		ID:   7,
		Name: "bastion",
		User: "jump",
		Host: "bastion.example",
		Port: 22,
	}}
	preview, err := Preview("ssh -N -L 3000:127.0.0.1:3000 -J jump@bastion.example dev@dev-host", nil, existing, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Hosts) != 2 {
		t.Fatalf("hosts %+v", preview.Hosts)
	}
	if preview.Hosts[0].ExistingID != 7 || preview.Hosts[0].ExistingName != "bastion" {
		t.Fatalf("jump was not reused: %+v", preview.Hosts[0])
	}
	if preview.Hosts[1].ExistingID != 0 || !preview.Hosts[1].Ready {
		t.Fatalf("destination should be created: %+v", preview.Hosts[1])
	}
	if preview.Hosts[1].AuthType != "ssh_agent" {
		t.Fatalf("auth %s", preview.Hosts[1].AuthType)
	}
}

func TestDuplicateTunnelBlocked(t *testing.T) {
	jumpers := []model.Jumper{{ID: 3, Name: "dev", User: "dev", Host: "dev-host", Port: 22}}
	tunnels := []model.Tunnel{{
		Name:       "api",
		Mode:       "local",
		JumperIDs:  []int{3},
		LocalHost:  "127.0.0.1",
		LocalPort:  3000,
		RemoteHost: "127.0.0.1",
		RemotePort: 3000,
	}}
	preview, err := Preview("ssh -N -L 3000:127.0.0.1:3000 -L 3000:127.0.0.1:3000 dev@dev-host", nil, jumpers, tunnels)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Tunnels) != 2 {
		t.Fatalf("tunnels %+v", preview.Tunnels)
	}
	if !preview.Tunnels[0].Blocked || preview.Tunnels[0].BlockReason != WarnDuplicateExisting {
		t.Fatalf("first tunnel %+v", preview.Tunnels[0])
	}
	if !preview.Tunnels[1].Blocked || preview.Tunnels[1].BlockReason != WarnDuplicateBatch {
		t.Fatalf("second tunnel %+v", preview.Tunnels[1])
	}
}

func TestRoundTrip(t *testing.T) {
	tunnel := model.Tunnel{
		Mode:       "local",
		LocalHost:  "127.0.0.1",
		LocalPort:  3000,
		RemoteHost: "10.1.0.5",
		RemotePort: 5432,
	}
	jumpers := []model.Jumper{
		{User: "jump", Host: "bastion.example", Port: 2222, AuthType: "ssh_agent", KeepAliveIntervalMs: defaultKeepAliveMs, TimeoutMs: defaultTimeoutMs},
		{User: "dev", Host: "dev-host", Port: 2200, AuthType: "ssh_key", KeyPath: "/home/dev/.ssh/id_ed25519", KeepAliveIntervalMs: 15000, TimeoutMs: 8000},
	}
	command, err := FormatTunnel(tunnel, jumpers)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := Preview(command, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Tunnels) != 1 || len(preview.Hosts) != 2 {
		t.Fatalf("command %s preview %+v", command, preview)
	}
	got := preview.Tunnels[0]
	if got.Mode != tunnel.Mode || got.LocalHost != tunnel.LocalHost || got.LocalPort != tunnel.LocalPort || got.RemoteHost != tunnel.RemoteHost || got.RemotePort != tunnel.RemotePort {
		t.Fatalf("forward %+v from %s", got, command)
	}
	if preview.Hosts[0].User != "jump" || preview.Hosts[0].Host != "bastion.example" || preview.Hosts[0].Port != 2222 {
		t.Fatalf("jump %+v", preview.Hosts[0])
	}
	dest := preview.Hosts[1]
	if dest.User != "dev" || dest.Host != "dev-host" || dest.Port != 2200 || dest.KeyPath != jumpers[1].KeyPath {
		t.Fatalf("dest %+v from %s", dest, command)
	}
	if dest.KeepAliveIntervalMs != 15000 || dest.TimeoutMs != 8000 {
		t.Fatalf("timers %+v", dest)
	}
	if len(preview.Warnings) != 0 {
		t.Fatalf("warnings %+v from %s", preview.Warnings, command)
	}
}

func TestRoundTripRemoteAndDynamic(t *testing.T) {
	cases := []model.Tunnel{
		{Mode: "remote", LocalHost: "127.0.0.1", LocalPort: 8080, RemoteHost: "0.0.0.0", RemotePort: 9000},
		{Mode: "dynamic", LocalHost: "127.0.0.1", LocalPort: 1080},
	}
	jumper := []model.Jumper{{User: "dev", Host: "dev-host", Port: 22, AuthType: "ssh_agent", KeepAliveIntervalMs: defaultKeepAliveMs, TimeoutMs: defaultTimeoutMs}}
	for _, tunnel := range cases {
		command, err := FormatTunnel(tunnel, jumper)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := Preview(command, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Tunnels) != 1 {
			t.Fatalf("%s -> %+v", command, preview)
		}
		got := preview.Tunnels[0]
		if got.Mode != tunnel.Mode || got.LocalHost != "127.0.0.1" || got.LocalPort != tunnel.LocalPort {
			t.Fatalf("got %+v from %s", got, command)
		}
		if tunnel.Mode == "remote" && (got.RemoteHost != tunnel.RemoteHost || got.RemotePort != tunnel.RemotePort) {
			t.Fatalf("remote %+v from %s", got, command)
		}
	}
}

func TestPasswordOmittedFromCommand(t *testing.T) {
	command, err := FormatTunnel(model.Tunnel{
		Mode: "local", LocalHost: "127.0.0.1", LocalPort: 3000, RemoteHost: "127.0.0.1", RemotePort: 3000,
	}, []model.Jumper{{
		User: "dev", Host: "dev-host", Port: 22, AuthType: "password", Password: "super-secret-value",
		KeepAliveIntervalMs: defaultKeepAliveMs, TimeoutMs: defaultTimeoutMs,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(command, "super-secret-value") || strings.Contains(command, "-i") {
		t.Fatalf("password auth leaked into %s", command)
	}
	preview, err := Preview(command, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Hosts[0].AuthType != "ssh_agent" {
		t.Fatalf("parsed auth %s", preview.Hosts[0].AuthType)
	}
}

func TestWindowsQuotingMatchesPOSIX(t *testing.T) {
	posix := `ssh -N -L '3000:127.0.0.1:3000' -i '/home/dev/my keys/id' 'dev@dev-host'`
	windows := `ssh -N -L "3000:127.0.0.1:3000" -i "C:\Users\dev\My Keys\id_ed25519" dev^@dev-host`
	left, err := Preview(posix, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	right, err := Preview(windows, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if left.Tunnels[0].LocalPort != right.Tunnels[0].LocalPort || left.Hosts[0].User != right.Hosts[0].User || left.Hosts[0].Host != right.Hosts[0].Host {
		t.Fatalf("posix %+v windows %+v", left, right)
	}
	if right.Hosts[0].KeyPath != `C:\Users\dev\My Keys\id_ed25519` {
		t.Fatalf("windows key %q", right.Hosts[0].KeyPath)
	}
}

func hasWarning(preview model.SSHCommandPreview, code string) bool {
	for _, warning := range preview.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
