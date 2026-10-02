//go:build windows

package ipc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestWindowsPipeRoundTrip(t *testing.T) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	path := `\\.\pipe\norka-test-` + hex.EncodeToString(buf)
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	if ln.Addr().Network() != "pipe" {
		t.Fatalf("network %s", ln.Addr().Network())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = Serve(ctx, ln, "sekret", func(req Request) Response {
			return Response{OK: true, Code: CodeOK, ExitCode: ExitOK, Message: req.Op}
		})
	}()
	resp := waitCall(t, ctx, path, "sekret", Request{Op: OpStatus})
	if !resp.OK || resp.Message != OpStatus {
		t.Fatalf("response: %+v", resp)
	}
	missing := `\\.\pipe\norka-missing-` + hex.EncodeToString(buf)
	if _, err := Call(ctx, missing, "sekret", Request{Op: OpList}); !errorsIsNotRunning(err) {
		t.Fatalf("missing pipe: %v", err)
	}
}

func errorsIsNotRunning(err error) bool {
	return err != nil && (err == ErrNotRunning || isNotRunning(err))
}
