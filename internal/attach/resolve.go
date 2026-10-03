// Package attach decides whether the window should mirror norkad over IPC
// or keep hosting tunnels itself.
package attach

import (
	"context"
	"errors"
	"time"

	"github.com/norka-app/Norka/internal/ipc"
)

const (
	// NoticeMissing means the norkad binary was not found.
	NoticeMissing = "background.missing"
	// NoticeFailed means norkad exited or never opened the channel.
	NoticeFailed = "background.failed"
	// NoticeOwned means another process holds the engine and did not answer.
	NoticeOwned = "background.owned"
	// NoticeUnreachable means the channel is there but hello failed.
	NoticeUnreachable = "background.unreachable"
)

// ErrBinaryMissing means FindNorkad found nothing.
var ErrBinaryMissing = errors.New("norkad binary was not found")

// ErrOwned means norkad exited 9 because engine.lock is held.
var ErrOwned = errors.New("engine is owned by another process")

// Request is the attach attempt. Hello and Spawn are injected so tests can
// stand up a fake IPC server without starting norkad.
type Request struct {
	Enabled bool
	Hello   func(context.Context) (ipc.Hello, error)
	Spawn   func(context.Context) error
	// Kill stops a child this attempt started, after hello never arrived.
	Kill     func()
	Attempts int
	Pause    time.Duration
}

// Result is what startup should do.
// Local means host tunnels in this process, the same path as before.
// Attached means do not call engine.Acquire.
type Result struct {
	Attached bool
	Local    bool
	Notice   string
}

// Resolve probes the daemon and, when it is absent, starts one.
// A disabled flag returns Local immediately and does not call Hello or Spawn.
func Resolve(ctx context.Context, req Request) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if !req.Enabled {
		return Result{Local: true}
	}
	ok, absent := probe(ctx, req.Hello)
	if ok {
		return Result{Attached: true}
	}
	if !absent {
		return Result{Notice: NoticeUnreachable}
	}
	if req.Spawn == nil {
		return Result{Local: true, Notice: NoticeMissing}
	}
	if err := req.Spawn(ctx); err != nil {
		if errors.Is(err, ErrOwned) {
			if again, _ := probe(ctx, req.Hello); again {
				return Result{Attached: true}
			}
			return Result{Notice: NoticeOwned}
		}
		if errors.Is(err, ErrBinaryMissing) {
			return Result{Local: true, Notice: NoticeMissing}
		}
		return Result{Local: true, Notice: NoticeFailed}
	}
	attempts := req.Attempts
	if attempts <= 0 {
		attempts = 25
	}
	pause := req.Pause
	if pause <= 0 {
		pause = 100 * time.Millisecond
	}
	for i := 0; i < attempts; i++ {
		if ok, _ := probe(ctx, req.Hello); ok {
			return Result{Attached: true}
		}
		timer := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			timer.Stop()
			if req.Kill != nil {
				req.Kill()
			}
			return Result{Local: true, Notice: NoticeFailed}
		case <-timer.C:
		}
	}
	if req.Kill != nil {
		req.Kill()
	}
	return Result{Local: true, Notice: NoticeFailed}
}

func probe(ctx context.Context, hello func(context.Context) (ipc.Hello, error)) (ok bool, absent bool) {
	if hello == nil {
		return false, true
	}
	greet, err := hello(ctx)
	if err == nil {
		if greet.PID > 0 && (greet.Owner == "daemon" || greet.Owner == "gui") {
			return true, false
		}
		return false, true
	}
	if errors.Is(err, ipc.ErrNotRunning) {
		return false, true
	}
	return false, false
}
