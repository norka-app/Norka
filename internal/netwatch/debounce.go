package netwatch

import (
	"context"
	"time"
)

// DebounceWindow is how long a burst of resume and network events stays open.
// The burst becomes one event. Resume wins when both kinds arrive together.
const DebounceWindow = time.Second

// Debounce reads in until ctx is cancelled. A burst that starts with any event
// and continues until window has elapsed is written to out as a single event.
// The caller owns out and decides when to close it.
func Debounce(ctx context.Context, clock Clock, window time.Duration, in <-chan Event, out chan<- Event) {
	if clock == nil {
		clock = systemClock{}
	}
	if window <= 0 {
		window = DebounceWindow
	}
	var (
		pending *Event
		timer   Timer
	)
	fire := func() {
		if pending == nil {
			return
		}
		ev := *pending
		pending = nil
		if timer != nil {
			timer.Stop()
			timer = nil
		}
		select {
		case <-ctx.Done():
		case out <- ev:
		}
	}
	for {
		var tick <-chan time.Time
		if timer != nil {
			tick = timer.C()
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case ev, ok := <-in:
			if !ok {
				fire()
				return
			}
			if ev.Kind == "" {
				continue
			}
			if pending == nil {
				copied := ev
				pending = &copied
				timer = clock.NewTimer(window)
				continue
			}
			if ev.Kind == KindResume {
				pending.Kind = KindResume
			}
		case <-tick:
			fire()
		}
	}
}
