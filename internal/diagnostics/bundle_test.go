package diagnostics

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/model"
)

func TestZipContentsAndReadme(t *testing.T) {
	cfg := sampleConfig()
	cfg.Jumpers[0].Password = secretCanary
	in := sampleInput(cfg, false, []byte("hello log\n"))
	in.StatsPresent = true
	in.Stats = []StatTotals{{
		ID:                1,
		TotalConnectedSec: 42,
		TotalBytesUp:      9,
		LastError:         "dial " + secretCanary,
	}}
	in.Tunnels = []TunnelState{{
		ID: 1, Name: "db", Mode: "local", Status: "error",
		LocalPort: 5432, RemotePort: 5432, AutoStart: true,
		LastError: "dial db.internal: " + secretCanary,
	}}
	in.System.Flags = []FlagState{{ID: "diagnostics", Enabled: true}}
	bundle := mustBuild(t, in)
	raw, err := ZipBytes(bundle.Files, in.Now)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, file := range reader.File {
		if strings.Contains(file.Name, "/") || strings.Contains(file.Name, `\`) {
			t.Fatalf("entry is a path: %s", file.Name)
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		_, err = buf.ReadFrom(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == "norka.log" && buf.Len() > MaxLogBytes {
			t.Fatalf("log entry is %d bytes", buf.Len())
		}
		got[file.Name] = buf.String()
	}
	for _, name := range []string{"README.txt", "system.txt", "config.toml", "tunnels.txt", "norka.log", "stats.json"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("missing %s", name)
		}
	}
	readme := got["README.txt"]
	for _, phrase := range []string{"Секретов нет", "No secrets", "паролей", "passwords", "токен", "token"} {
		if !strings.Contains(readme, phrase) {
			t.Fatalf("readme missing %q", phrase)
		}
	}
	if !strings.Contains(got["system.txt"], "version: 1.2.1") || !strings.Contains(got["system.txt"], "diagnostics=true") {
		t.Fatalf("system.txt:\n%s", got["system.txt"])
	}
	if !strings.Contains(got["tunnels.txt"], "status=error") || !strings.Contains(got["tunnels.txt"], "local_port=5432") {
		t.Fatalf("tunnels.txt:\n%s", got["tunnels.txt"])
	}
	if !strings.Contains(got["stats.json"], `"totalConnectedSec": 42`) {
		t.Fatalf("stats.json:\n%s", got["stats.json"])
	}
	joined := strings.Join([]string{got["system.txt"], got["config.toml"], got["tunnels.txt"], got["norka.log"], got["stats.json"]}, "\n")
	if strings.Contains(joined, secretCanary) || strings.Contains(joined, "db.internal") {
		t.Fatal("zip kept a secret or a host")
	}
	if strings.Contains(bundle.IssueURL, secretCanary) || !strings.Contains(bundle.IssueURL, "1.2.1") {
		t.Fatalf("issue url: %s", bundle.IssueURL)
	}
	if !strings.HasPrefix(bundle.IssueURL, IssuesNewURL+"?") {
		t.Fatalf("issue url: %s", bundle.IssueURL)
	}
}

func TestStatsOmittedWhenAbsent(t *testing.T) {
	bundle := mustBuild(t, sampleInput(sampleConfig(), false, nil))
	for _, file := range bundle.Files {
		if file.Name == "stats.json" {
			t.Fatal("stats.json was added without a stats file")
		}
	}
}

func TestLogByteAndLineCaps(t *testing.T) {
	prefix := []byte("START-MARKER\n")
	suffix := []byte("TAIL-MARKER\n")
	data := append(append(prefix, bytes.Repeat([]byte("x"), MaxLogBytes)...), suffix...)
	got := Tail(data, MaxLogBytes, MaxLogLines)
	if len(got) > MaxLogBytes {
		t.Fatalf("tail is %d bytes", len(got))
	}
	if bytes.Contains(got, []byte("START-MARKER")) || !bytes.Contains(got, []byte("TAIL-MARKER")) {
		t.Fatal("byte cap kept the wrong end")
	}

	var lines strings.Builder
	for i := 0; i < MaxLogLines+500; i++ {
		fmt.Fprintf(&lines, "line-%04d\n", i)
	}
	tailed := string(Tail([]byte(lines.String()), MaxLogBytes, MaxLogLines))
	if strings.Contains(tailed, "line-0000") || !strings.Contains(tailed, fmt.Sprintf("line-%04d", MaxLogLines+499)) {
		t.Fatal("line cap kept the wrong end")
	}
	if n := len(strings.Split(strings.TrimSuffix(tailed, "\n"), "\n")); n > MaxLogLines {
		t.Fatalf("kept %d lines", n)
	}

	cfg := sampleConfig()
	bundle := mustBuild(t, sampleInput(cfg, false, data))
	for _, file := range bundle.Files {
		if file.Name == "norka.log" && len(file.Data) > MaxLogBytes {
			t.Fatalf("archived log is %d bytes", len(file.Data))
		}
	}
}

func TestReadLogTailFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "norka.log")
	var body bytes.Buffer
	body.WriteString("START-MARKER\n")
	body.Write(bytes.Repeat([]byte("y"), MaxLogBytes))
	body.WriteString("TAIL-MARKER\n")
	if err := os.WriteFile(path, body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLogTail(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > MaxLogBytes {
		t.Fatalf("read %d bytes", len(got))
	}
	if bytes.Contains(got, []byte("START-MARKER")) || !bytes.Contains(got, []byte("TAIL-MARKER")) {
		t.Fatal("file tail kept the wrong end")
	}
	missing, err := ReadLogTail(filepath.Join(dir, "missing.log"))
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing log err=%v len=%d", err, len(missing))
	}
	if LogPath(filepath.Join(dir, "config.toml")) != path {
		t.Fatalf("log path %s", LogPath(filepath.Join(dir, "config.toml")))
	}
}

func TestFilenameUsesLocalTime(t *testing.T) {
	now := time.Date(2026, 10, 3, 16, 7, 45, 0, time.UTC)
	if got := Filename(now); got != "norka-diagnostics-20261003-1607.zip" {
		t.Fatalf("filename %s", got)
	}
}

func TestIssueURLCarriesSystemTextOnly(t *testing.T) {
	cfg := sampleConfig()
	cfg.Jumpers[0].Password = secretCanary
	cfg.Jumpers[0].Host = "db.internal"
	bundle := mustBuild(t, sampleInput(cfg, false, []byte(secretCanary+" db.internal\n")))
	if strings.Contains(bundle.IssueURL, secretCanary) || strings.Contains(bundle.IssueURL, "db.internal") {
		t.Fatal("issue url contains a secret or a host")
	}
	if !strings.Contains(bundle.IssueURL, "version") || !strings.Contains(bundle.SystemText, "theme: dark") {
		t.Fatalf("system text was not prefilled:\n%s", bundle.SystemText)
	}
}

func sampleConfig() *conf.Config {
	cfg := conf.DefaultConfig()
	cfg.Language = "ru"
	cfg.Jumpers = []model.Jumper{{
		ID: 1, Name: "bastion", Host: "db.internal", Port: 22, User: "alice",
		AuthType: "password", KeyPath: "/home/alice/.ssh/id_ed25519",
		AgentSocketPath: "/tmp/ssh-agent.sock", Password: "plain-password",
		SecretRef: "keychain-account", Notes: "do not copy",
	}}
	cfg.Groups = []model.TunnelGroup{{ID: 1, Name: "work"}}
	cfg.Tunnels = []model.Tunnel{{
		ID: 1, Name: "db", Mode: "local", LocalHost: "127.0.0.1", LocalPort: 5432,
		RemoteHost: "10.1.2.3", RemotePort: 5432, Status: "error",
		LastError: "dial 10.1.2.3:5432: refused", Description: "database",
	}}
	cfg.Profiles = []model.Profile{{ID: 1, Name: "home", TunnelIDs: []int{1}}}
	return cfg
}

func sampleInput(cfg *conf.Config, includeHosts bool, log []byte, extra ...string) Input {
	return Input{
		Now:          time.Date(2026, 10, 3, 4, 7, 0, 0, time.UTC),
		IncludeHosts: includeHosts,
		System: SystemInfo{
			Version: "1.2.1", Commit: "abc123", OS: "linux", Arch: "amd64",
			OSVersion: "Test OS", GoVersion: "go1.25.0", Locale: "ru", Theme: "dark",
			WailsVersion: "v2.16.0", WebViewVersion: "",
			Flags: []FlagState{{ID: string(features.Diagnostics), Enabled: true}},
		},
		Config:  cfg,
		Log:     log,
		Tunnels: []TunnelState{{ID: 1, Name: "db", Mode: "local", Status: "stopped", LocalPort: 5432, RemotePort: 5432}},
		Secrets: extra,
	}
}

func mustBuild(t *testing.T, in Input) Bundle {
	t.Helper()
	bundle, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func mustBundle(t *testing.T, cfg *conf.Config, includeHosts bool, log []byte, extra ...string) Bundle {
	t.Helper()
	return mustBuild(t, sampleInput(cfg, includeHosts, log, extra...))
}

func archiveText(t *testing.T, cfg *conf.Config, includeHosts bool, log []byte, extra ...string) string {
	t.Helper()
	bundle := mustBundle(t, cfg, includeHosts, log, extra...)
	var b strings.Builder
	for _, file := range bundle.Files {
		if file.Name == "README.txt" {
			continue
		}
		b.Write(file.Data)
		b.WriteByte('\n')
	}
	b.WriteString(bundle.IssueURL)
	return b.String()
}

func fileText(t *testing.T, cfg *conf.Config, includeHosts bool, log []byte, name string) string {
	t.Helper()
	for _, file := range mustBundle(t, cfg, includeHosts, log).Files {
		if file.Name == name {
			return string(file.Data)
		}
	}
	t.Fatalf("missing %s", name)
	return ""
}
