package attach

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Grace is how long Spawn waits to see norkad exit before treating it as detached.
const Grace = 300 * time.Millisecond

// Child is a norkad this process started. Stop kills it. It is used when
// attach fails after spawn, so the window does not leave an orphan it cannot see.
type Child struct {
	stop func()
}

// Stop kills the child. A nil child is fine.
func (c *Child) Stop() {
	if c == nil || c.stop == nil {
		return
	}
	c.stop()
}

// Spawn starts norkad detached with the same config as the window.
// A fast exit is returned as an error. Exit code 9 is ErrOwned.
// A process that is still alive after Grace is returned as a Child.
func Spawn(ctx context.Context, exe, configPath string) (*Child, error) {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return nil, ErrBinaryMissing
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.Command(exe, "--config", configPath)
	cmd.SysProcAttr = detachAttr()
	cmd.Stdin = nil
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(Grace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		killProcess(cmd)
		return nil, ctx.Err()
	case err := <-done:
		if err == nil {
			return nil, errors.New("norkad exited before it was ready")
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 9 {
			return nil, fmt.Errorf("%w: %s", ErrOwned, strings.TrimSpace(stderr.String()))
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, detail)
	case <-timer.C:
		child := &Child{stop: func() { killProcess(cmd) }}
		go func() { <-done }()
		return child, nil
	}
}

func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

// limitedBuffer keeps the first bytes of norkad's early stderr.
// The daemon logs to norkad.log after setup, so this stays small.
type limitedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	const max = 4096
	n := len(p)
	if b.buf.Len() >= max {
		return n, nil
	}
	room := max - b.buf.Len()
	if len(p) > room {
		p = p[:room]
	}
	_, _ = b.buf.Write(p)
	return n, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
