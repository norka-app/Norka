package biz

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/model"
)

func TestToggleCancelsInProgressStart(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	jumperBiz := NewJumperBiz(storage)
	jumper, err := jumperBiz.Create(model.JumperPayload{
		Name:                   "jump",
		Host:                   "127.0.0.1",
		Port:                   port,
		User:                   "root",
		AuthType:               "password",
		Password:               "secret",
		BypassHostVerification: true,
		TimeoutMs:              30000,
	})
	if err != nil {
		t.Fatalf("create jumper: %v", err)
	}

	tunnelBiz := NewTunnelBiz(storage)
	created, err := tunnelBiz.Create(model.TunnelPayload{
		Name:       "slow",
		Mode:       "local",
		JumperIDs:  []int{jumper.ID},
		LocalHost:  "127.0.0.1",
		LocalPort:  19001,
		RemoteHost: "10.0.0.1",
		RemotePort: 22,
		Status:     "stopped",
	})
	if err != nil {
		t.Fatalf("create tunnel: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := tunnelBiz.Toggle(created.ID, 0)
		done <- err
	}()

	select {
	case conn := <-accepted:
		defer conn.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for dial")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		tunnelBiz.mu.Lock()
		_, starting := tunnelBiz.starting[created.ID]
		tunnelBiz.mu.Unlock()
		if starting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("start was not in progress")
		}
		time.Sleep(10 * time.Millisecond)
	}

	updated, err := tunnelBiz.Toggle(created.ID, 0)
	if err != nil {
		t.Fatalf("cancel toggle: %v", err)
	}
	if updated.Status != "stopped" {
		t.Fatalf("status after cancel = %q, want stopped", updated.Status)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("original toggle: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("original toggle did not return after cancel")
	}

	cfg, err := storage.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, tunnel := range cfg.Tunnels {
		if tunnel.ID == created.ID && tunnel.Status != "stopped" {
			t.Fatalf("persisted status = %q, want stopped", tunnel.Status)
		}
	}
}
