//go:build unix

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/engine"
)

func TestShutdownOnSignal(t *testing.T) {
	path := writeConfig(t, func(cfg *conf.Config) {
		quietDaemonFeatures(t, cfg, true)
	})
	cmd := exec.Command(os.Args[0], "--config", path, "--foreground")
	cmd.Env = daemonEnv(t)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	var buf lockedBuffer
	var copies sync.WaitGroup
	copies.Add(2)
	go func() {
		defer copies.Done()
		_, _ = io.Copy(&buf, stdout)
	}()
	go func() {
		defer copies.Done()
		_, _ = io.Copy(&buf, stderr)
	}()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	deadline := time.Now().Add(8 * time.Second)
	for !strings.Contains(buf.String(), "norkad ready") {
		if time.Now().After(deadline) {
			t.Fatalf("norkad did not become ready:\n%s", buf.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err := <-waited:
		if err != nil {
			t.Fatalf("exit after SIGTERM: %v\n%s", err, buf.String())
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("norkad did not exit after SIGTERM:\n%s", buf.String())
	}
	copies.Wait()
	text := buf.String()
	if !strings.Contains(text, "norkad shutting down") || !strings.Contains(text, "norkad stopped") {
		t.Fatalf("shutdown log:\n%s", text)
	}
	owner, err := engine.Acquire(path, engine.KindDaemon, "after-signal")
	if err != nil {
		t.Fatalf("engine lock still held after shutdown: %v", err)
	}
	_ = owner.Release()
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
