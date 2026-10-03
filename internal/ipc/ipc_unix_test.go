//go:build !windows

package ipc

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
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

func TestProtocolVersionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "norka.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var handled atomic.Int32
	go func() {
		_ = Serve(ctx, ln, "sekret", func(req Request) Response {
			handled.Add(1)
			return Response{OK: true, Code: CodeOK, ExitCode: ExitOK, Message: req.Op}
		})
	}()

	legacy := waitCall(t, ctx, path, "sekret", Request{Op: OpStatus})
	if !legacy.OK || legacy.V != ProtocolVersion || handled.Load() != 1 {
		t.Fatalf("legacy: %+v handled %d", legacy, handled.Load())
	}

	current := waitCall(t, ctx, path, "sekret", Request{V: ProtocolVersion, Op: OpList})
	if !current.OK || current.V != ProtocolVersion || handled.Load() != 2 {
		t.Fatalf("current: %+v handled %d", current, handled.Load())
	}

	newer, err := Call(ctx, path, "sekret", Request{V: ProtocolVersion + 1, Op: OpStatus})
	if err != nil {
		t.Fatal(err)
	}
	if newer.OK || newer.Code != CodeUpdateApp || newer.ExitCode != ExitVersion || newer.V != ProtocolVersion || handled.Load() != 2 {
		t.Fatalf("newer: %+v handled %d", newer, handled.Load())
	}
	if newer.Message != MessageUpdateApp {
		t.Fatalf("newer message: %q", newer.Message)
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"token": "sekret", "op": OpStatus})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		t.Fatal(err)
	}
	var raw Response
	if err := readJSON(conn, &raw); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if !raw.OK || raw.V != ProtocolVersion || handled.Load() != 3 {
		t.Fatalf("raw legacy: %+v handled %d", raw, handled.Load())
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
