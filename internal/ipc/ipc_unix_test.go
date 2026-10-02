//go:build !windows

package ipc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnixRoundTripAndPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "")
	path := filepath.Join(dir, "norka.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("socket mode %o", info.Mode().Perm())
	}
	if ln.Addr().Network() != "unix" {
		t.Fatalf("network %s", ln.Addr().Network())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = Serve(ctx, ln, "sekret", func(req Request) Response {
			return Response{OK: true, Code: CodeOK, ExitCode: ExitOK, Message: req.Op + ":" + req.Target}
		})
	}()

	resp := waitCall(t, ctx, path, "sekret", Request{Op: OpConnect, Target: "db"})
	if !resp.OK || resp.Message != "connect:db" || !resp.Running {
		t.Fatalf("response: %+v", resp)
	}

	denied, err := Call(ctx, path, "nope", Request{Op: OpStatus})
	if err != nil {
		t.Fatal(err)
	}
	if denied.OK || denied.Code != CodeUnauthorized || denied.ExitCode != ExitIPC {
		t.Fatalf("unauthorized: %+v", denied)
	}

	_, err = Call(ctx, filepath.Join(dir, "missing.sock"), "sekret", Request{Op: OpStatus})
	if err == nil || !errorsIsNotRunning(err) {
		t.Fatalf("missing socket: %v", err)
	}
}

func TestAddressUsesRuntimeDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	got, err := Address(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "norka", "norka.sock")
	if got != want {
		t.Fatalf("address %s want %s", got, want)
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	got, err = Address(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "norka.sock") {
		t.Fatalf("config socket: %s", got)
	}
}

func TestListenLeavesLiveSocket(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "norka.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()
	time.Sleep(30 * time.Millisecond)
	if _, err := Listen(path); err == nil {
		t.Fatal("second listen stole a live socket")
	}
}

func TestTokenFileIsUserOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "automation.token")
	token, err := EnsureToken(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("token mode %o", info.Mode().Perm())
	}
	again, err := ReadToken(path)
	if err != nil || again != token {
		t.Fatalf("read token: %q %v", again, err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadToken(path); err == nil {
		t.Fatal("world-readable token was accepted")
	}
}

func errorsIsNotRunning(err error) bool {
	return err != nil && (err == ErrNotRunning || isNotRunning(err))
}
