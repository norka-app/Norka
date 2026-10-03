package attach

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/ipc"
)

func TestResolveFlagOffDoesNotDial(t *testing.T) {
	result := Resolve(context.Background(), Request{
		Enabled: false,
		Hello: func(context.Context) (ipc.Hello, error) {
			t.Fatal("hello was called while background mode is off")
			return ipc.Hello{}, nil
		},
		Spawn: func(context.Context) error {
			t.Fatal("spawn was called while background mode is off")
			return nil
		},
	})
	if !result.Local || result.Attached || result.Notice != "" {
		t.Fatalf("flag off: %+v", result)
	}
}

func TestResolveAttachesToFakeServer(t *testing.T) {
	address, stop := fakeDaemon(t, "daemon", 42)
	defer stop()
	hello := helloClient(address)

	result := Resolve(context.Background(), Request{Enabled: true, Hello: hello, Spawn: func(context.Context) error {
		t.Fatal("spawn is not needed when the daemon is already up")
		return nil
	}})
	if !result.Attached || result.Local || result.Notice != "" {
		t.Fatalf("attached: %+v", result)
	}
}

func TestResolveSpawnsFakeServerThenAttaches(t *testing.T) {
	dir := t.TempDir()
	address := filepath.Join(dir, "norka.sock")
	const token = "test-token"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spawned := false
	result := Resolve(ctx, Request{
		Enabled: true,
		Hello:   helloClient(address),
		Spawn: func(context.Context) error {
			spawned = true
			ln, err := ipc.Listen(address)
			if err != nil {
				return err
			}
			go func() {
				_ = ipc.Serve(ctx, ln, token, func(ipc.Request) ipc.Response {
					return ipc.Response{OK: true, Code: ipc.CodeOK, Hello: &ipc.Hello{Owner: "daemon", PID: 7, Version: "test"}}
				})
			}()
			return nil
		},
		Attempts: 30,
		Pause:    20 * time.Millisecond,
	})
	if !spawned || !result.Attached || result.Notice != "" {
		t.Fatalf("spawned=%v result=%+v", spawned, result)
	}
}

func TestResolveMissingBinaryFallsBack(t *testing.T) {
	result := Resolve(context.Background(), Request{
		Enabled: true,
		Hello: func(context.Context) (ipc.Hello, error) {
			return ipc.Hello{}, ipc.ErrNotRunning
		},
		Spawn: func(context.Context) error { return ErrBinaryMissing },
	})
	if !result.Local || result.Attached || result.Notice != NoticeMissing {
		t.Fatalf("missing: %+v", result)
	}
}

func TestResolveOwnedDoesNotHostLocally(t *testing.T) {
	result := Resolve(context.Background(), Request{
		Enabled: true,
		Hello: func(context.Context) (ipc.Hello, error) {
			return ipc.Hello{}, ipc.ErrNotRunning
		},
		Spawn: func(context.Context) error { return ErrOwned },
	})
	if result.Local || result.Attached || result.Notice != NoticeOwned {
		t.Fatalf("owned: %+v", result)
	}
}

func TestResolveUnreachableDoesNotHostLocally(t *testing.T) {
	result := Resolve(context.Background(), Request{
		Enabled: true,
		Hello: func(context.Context) (ipc.Hello, error) {
			return ipc.Hello{}, errors.New("unauthorized")
		},
	})
	if result.Local || result.Attached || result.Notice != NoticeUnreachable {
		t.Fatalf("unreachable: %+v", result)
	}
}

func TestFindNorkadNextToExecutable(t *testing.T) {
	dir := t.TempDir()
	name := "norkad"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe := filepath.Join(dir, "norka")
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindNorkad(exe); got != bin {
		t.Fatalf("found %q, want %q", got, bin)
	}
	if got := FindNorkad(filepath.Join(dir, "missing")); got != "" && got != bin {
		// PATH may contain a real norkad. The sibling of a missing exe is still this dir,
		// so the sibling binary is a valid find.
		if filepath.Dir(got) != dir && got != bin {
			t.Fatalf("unexpected %q", got)
		}
	}
}

func fakeDaemon(t *testing.T, owner string, pid int) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	address := filepath.Join(dir, "norka.sock")
	ln, err := ipc.Listen(address)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = ipc.Serve(ctx, ln, "test-token", func(ipc.Request) ipc.Response {
			return ipc.Response{OK: true, Code: ipc.CodeOK, Hello: &ipc.Hello{Owner: owner, PID: pid, Version: "test"}}
		})
	}()
	return address, func() {
		cancel()
		_ = ln.Close()
	}
}

func helloClient(address string) func(context.Context) (ipc.Hello, error) {
	return func(ctx context.Context) (ipc.Hello, error) {
		callCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		resp, err := ipc.Call(callCtx, address, "test-token", ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpHello})
		if err != nil {
			return ipc.Hello{}, err
		}
		if !resp.OK || resp.Hello == nil {
			return ipc.Hello{}, errors.New("hello failed")
		}
		return *resp.Hello, nil
	}
}
