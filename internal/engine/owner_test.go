package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/notify"
)

func TestMain(m *testing.M) {
	if os.Getenv("NORKA_ENGINE_HELPER") == "hold" {
		holdEngineLock()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestAcquireReportsHolder(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cmd, _, _ := startEngineHolder(t, configPath, KindGUI, "test-version")

	_, err := Acquire(configPath, KindDaemon, "other")
	var held *OwnedByOtherError
	if !errors.As(err, &held) || !errors.Is(err, ErrOwnedByOther) {
		t.Fatalf("Acquire error = %v, want OwnedByOtherError", err)
	}
	if held.Holder.PID != cmd.Process.Pid {
		t.Fatalf("holder pid = %d, want %d", held.Holder.PID, cmd.Process.Pid)
	}
	if held.Holder.Kind != KindGUI {
		t.Fatalf("holder kind = %q", held.Holder.Kind)
	}
	if held.Holder.Version != "test-version" {
		t.Fatalf("holder version = %q", held.Holder.Version)
	}
	if held.Holder.StartedAt.IsZero() || time.Since(held.Holder.StartedAt) > time.Minute {
		t.Fatalf("holder started_at = %s", held.Holder.StartedAt)
	}

	meta, err := ReadOwner(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if meta.PID != held.Holder.PID || meta.Kind != held.Holder.Kind || meta.Version != held.Holder.Version || !meta.StartedAt.Equal(held.Holder.StartedAt) {
		t.Fatalf("ReadOwner = %+v, holder = %+v", meta, held.Holder)
	}

	raw, err := os.ReadFile(filepath.Join(filepath.Dir(configPath), engineLockFile))
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pid", "kind", "version", "started_at"} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("engine.lock missing %s: %s", key, raw)
		}
	}
}

func TestAcquireAfterHolderExits(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	_, stdin, reap := startEngineHolder(t, configPath, KindGUI, "test-version")
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reap(); err != nil {
		t.Fatal(err)
	}
	owner := acquireAfterHolderGone(t, configPath)
	t.Cleanup(func() { _ = owner.Release() })
	if owner.Meta().PID != os.Getpid() || owner.Meta().Kind != KindDaemon {
		t.Fatalf("new owner = %+v", owner.Meta())
	}
}

func TestAcquireAfterHolderCrashes(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cmd, _, reap := startEngineHolder(t, configPath, KindGUI, "test-version")
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = reap()
	owner := acquireAfterHolderGone(t, configPath)
	t.Cleanup(func() { _ = owner.Release() })
	if owner.Meta().PID != os.Getpid() {
		t.Fatalf("new owner pid = %d", owner.Meta().PID)
	}
}

func TestAcquireTwiceInOneProcess(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	first, err := Acquire(configPath, KindGUI, "v1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	_, err = Acquire(configPath, KindDaemon, "v2")
	var held *OwnedByOtherError
	if !errors.As(err, &held) || held.Holder.PID != os.Getpid() || held.Holder.Kind != KindGUI {
		t.Fatalf("second Acquire = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(configPath, KindDaemon, "v2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Release() })
	if second.Meta().Kind != KindDaemon {
		t.Fatalf("kind = %s", second.Meta().Kind)
	}
}

func TestNonOwnerSkipsExclusiveWork(t *testing.T) {
	eng, fake, watched, listened, schemes := engineWithExclusiveCounters(t, true)
	eng.Start()
	eng.StartAutoStart()
	select {
	case <-fake.auto:
		t.Fatal("non-owner started tunnels")
	case <-time.After(50 * time.Millisecond):
	}
	if *watched != 0 || *listened != 0 {
		t.Fatalf("watch %d listen %d", *watched, *listened)
	}
	if *schemes != 1 {
		t.Fatalf("scheme sync = %d, want 1", *schemes)
	}
	eng.Shutdown()
	select {
	case <-fake.shut:
		t.Fatal("non-owner stopped tunnels")
	default:
	}
}

func TestHostWithoutLockKeepsHosting(t *testing.T) {
	eng, fake, watched, listened, _ := engineWithExclusiveCounters(t, true)
	eng.HostWithoutLock()
	eng.Start()
	if *watched != 1 {
		t.Fatalf("wake watch calls = %d", *watched)
	}
	if *listened != 1 {
		t.Fatalf("automation listen calls = %d", *listened)
	}
	eng.StartAutoStart()
	select {
	case <-fake.auto:
	case <-time.After(2 * time.Second):
		t.Fatal("host without the lock did not start tunnels")
	}
	eng.Shutdown()
	select {
	case <-fake.shut:
	case <-time.After(2 * time.Second):
		t.Fatal("host without the lock did not stop tunnels")
	}
}

func engineWithExclusiveCounters(t *testing.T, wakeOn bool) (*Engine, *fakeRuntime, *int, *int, *int) {
	t.Helper()
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		if err := cfg.Features.Set(features.WakeReconnect, wakeOn); err != nil {
			return err
		}
		return cfg.Features.Set(features.Automation, true)
	}); err != nil {
		t.Fatal(err)
	}
	fake := &fakeRuntime{auto: make(chan int, 1), shut: make(chan struct{}, 1)}
	var watched, listened, schemes int
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	eng := New(Options{
		Storage: storage,
		Runtime: fake,
		Watch: func(context.Context) (<-chan netwatch.Event, error) {
			watched++
			return nil, context.Canceled
		},
		Listen: func(string) (net.Listener, error) {
			listened++
			return ln, nil
		},
		SyncLogin:  func(bool, bool) error { return nil },
		SyncScheme: func(bool) error { schemes++; return nil },
		Poster:     func(notify.PosterConfig) notify.Poster { return nopPoster{} },
	})
	return eng, fake, &watched, &listened, &schemes
}

func startEngineHolder(t *testing.T, configPath string, kind Kind, version string) (*exec.Cmd, io.WriteCloser, func() error) {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(),
		"NORKA_ENGINE_HELPER=hold",
		"NORKA_ENGINE_CONFIG="+configPath,
		"NORKA_ENGINE_KIND="+string(kind),
		"NORKA_ENGINE_VERSION="+version,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	var waitErr error
	reap := func() error {
		once.Do(func() { waitErr = cmd.Wait() })
		return waitErr
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = reap()
	})
	if err := readReady(stdout); err != nil {
		t.Fatalf("%v\nstderr: %s", err, stderr.String())
	}
	return cmd, stdin, reap
}

func acquireAfterHolderGone(t *testing.T, configPath string) *Owner {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for {
		owner, err := Acquire(configPath, KindDaemon, "next")
		if err == nil {
			return owner
		}
		last = err
		if time.Now().After(deadline) {
			t.Fatalf("acquire after holder was gone: %v", last)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func readReady(r io.Reader) error {
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 32)
		n, err := r.Read(buf)
		if err != nil {
			done <- err
			return
		}
		if !strings.HasPrefix(string(buf[:n]), "ready") {
			done <- fmt.Errorf("child said %q", buf[:n])
			return
		}
		done <- nil
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		return errors.New("timeout waiting for engine holder")
	}
}

func holdEngineLock() {
	owner, err := Acquire(
		os.Getenv("NORKA_ENGINE_CONFIG"),
		Kind(os.Getenv("NORKA_ENGINE_KIND")),
		os.Getenv("NORKA_ENGINE_VERSION"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquire: %v\n", err)
		os.Exit(1)
	}
	defer owner.Release()
	fmt.Println("ready")
	_ = os.Stdout.Sync()
	_, _ = io.Copy(io.Discard, os.Stdin)
}
