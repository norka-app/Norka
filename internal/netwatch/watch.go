package netwatch

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// Start runs the platform watchers and the clock-jump fallback until ctx is
// cancelled. The returned channel closes after ctx is done. It never creates
// a console window. A nil ctx is rejected so a forgotten cancel cannot leak
// the watchers.
func Start(ctx context.Context) (<-chan Event, error) {
	if ctx == nil {
		return nil, errors.New("netwatch: nil context")
	}
	raw := make(chan Event, 32)
	out := make(chan Event, 8)
	setPublisher(ctx, raw)
	go runSystemJump(ctx, raw)
	startPlatform(ctx, raw)
	go func() {
		defer func() {
			// Drop the OS callback target before closing the debounced
			// channel. publish then sees no listener and returns.
			clearPublisher(ctx)
			close(out)
		}()
		Debounce(ctx, systemClock{}, DebounceWindow, raw, out)
	}()
	return out, nil
}

func startPlatform(ctx context.Context, out chan<- Event) {
	if err := startOSWatch(ctx, out); err != nil {
		slog.Info("os wake watch failed; polling interfaces", "err", err)
		go watchInterfaces(ctx, out)
	}
}

type publisher struct {
	ctx context.Context
	out chan<- Event
}

var (
	pubMu sync.Mutex
	pub   publisher
)

func setPublisher(ctx context.Context, out chan<- Event) {
	pubMu.Lock()
	pub = publisher{ctx: ctx, out: out}
	pubMu.Unlock()
}

// publish is called from OS notification threads. The send stays under pubMu
// and is non-blocking, so the callback returns even when the buffer is full.
// After stop, clearPublisher drops the listener under the same lock; publish
// then returns without touching the channel, including if that channel was closed.
func publish(kind Kind) {
	if kind == "" {
		return
	}
	pubMu.Lock()
	defer pubMu.Unlock()
	if pub.out == nil || pub.ctx == nil || pub.ctx.Err() != nil {
		return
	}
	select {
	case pub.out <- Event{Kind: kind}:
	default:
	}
}

// clearPublisher forgets the listener for ctx. A newer Start that already
// replaced the publisher is left alone. Call it before closing the channel
// publish was given.
func clearPublisher(ctx context.Context) {
	pubMu.Lock()
	defer pubMu.Unlock()
	if pub.ctx == ctx {
		pub = publisher{}
	}
}
