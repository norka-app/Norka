//go:build !windows

package ipc

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSubscribeKeepsV1OneShot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "norka.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = ServeStream(ctx, ln, "sekret", func(req Request) Response {
			if IsV2Op(req.Op) {
				t.Errorf("v1 or legacy request reached a v2 op: %+v", req)
			}
			return Response{OK: true, Code: CodeOK, ExitCode: ExitOK, Message: req.Op}
		}, func(ctx context.Context, req Request, send func(Response) error) error {
			if err := send(Response{OK: true, Code: CodeOK, State: &State{Tunnels: []TunnelInfo{{ID: 1, Name: "db"}}}}); err != nil {
				return err
			}
			if err := send(Response{OK: true, Code: CodeOK, Event: &Event{Type: EventLog, Level: "INFO", Line: "ready"}}); err != nil {
				return err
			}
			<-ctx.Done()
			return nil
		})
	}()

	legacy := waitCall(t, ctx, path, "sekret", Request{Op: OpStatus})
	if !legacy.OK || legacy.Message != OpStatus || legacy.V != ProtocolVersion || legacy.State != nil {
		t.Fatalf("legacy: %+v", legacy)
	}

	followCtx, followCancel := context.WithTimeout(ctx, 3*time.Second)
	defer followCancel()
	stream, err := Follow(followCtx, path, "sekret", Request{V: ProtocolVersion, Op: OpSubscribe})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	snap, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if snap.State == nil || len(snap.State.Tunnels) != 1 || snap.State.Tunnels[0].Name != "db" {
		t.Fatalf("snapshot: %+v", snap)
	}
	ev, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Event == nil || ev.Event.Type != EventLog || ev.Event.Line != "ready" {
		t.Fatalf("event: %+v", ev)
	}
}
