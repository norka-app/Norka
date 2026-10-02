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
		defer close(out)
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

func publish(kind Kind) {
	pubMu.Lock()
	current := pub
	pubMu.Unlock()
	if current.ctx == nil || current.out == nil {
		return
	}
	emit(current.ctx, current.out, Event{Kind: kind})
}
