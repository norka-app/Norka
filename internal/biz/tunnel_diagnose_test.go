package biz

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/model"
)

func TestDiagnoseDoesNotStoreOrSendPassword(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("SSH-2.0-TestBanner\r\n"))
			time.Sleep(50 * time.Millisecond)
			_ = conn.Close()
		}
	}()

	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	localPort := free.Addr().(*net.TCPAddr).Port
	_ = free.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	const secret = "super-secret-value"
	jumperBiz := NewJumperBiz(storage)
	jumper, err := jumperBiz.Create(model.JumperPayload{
		Name:                   "jump",
		Host:                   "127.0.0.1",
		Port:                   port,
		User:                   "root",
		AuthType:               "password",
		Password:               secret,
		BypassHostVerification: true,
		TimeoutMs:              2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	tunnelBiz := NewTunnelBiz(storage)
	created, err := tunnelBiz.Create(model.TunnelPayload{
		Name:       "db",
		Mode:       "local",
		JumperIDs:  []int{jumper.ID},
		LocalHost:  "127.0.0.1",
		LocalPort:  localPort,
		RemoteHost: "10.0.0.8",
		RemotePort: 5432,
		Status:     "stopped",
	})
	if err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := tunnelBiz.Diagnose(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("diagnostics rewrote config.toml")
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("report contains the password: %s", raw)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "auth" && check.Code == "auth_skipped_password" {
			found = true
			if !strings.Contains(check.Detail, "SSH-2.0-TestBanner") {
				t.Fatalf("banner detail %q", check.Detail)
			}
		}
	}
	if !found {
		t.Fatalf("auth check missing: %+v", report.Checks)
	}
}
