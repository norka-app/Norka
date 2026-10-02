package ipc

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func waitCall(t *testing.T, ctx context.Context, address, token string, req Request) Response {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		callCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		resp, err := Call(callCtx, address, token, req)
		cancel()
		if err == nil {
			return resp
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("call: %v", last)
	return Response{}
}

func TestServeRefusesTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	err = Serve(context.Background(), ln, "token", nil)
	if err == nil || !strings.Contains(err.Error(), "tcp") {
		t.Fatalf("expected tcp refusal, got %v", err)
	}
}
