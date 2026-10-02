package netwatch

import (
	"context"
	"testing"
	"time"
)

type fakeClock struct {
	mu     chan struct{}
	now    time.Time
	timers []*fakeTimer
	armed  chan struct{}
}

type fakeTimer struct {
	c       chan time.Time
	when    time.Time
	clock   *fakeClock
	stopped bool
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{
		mu:    make(chan struct{}, 1),
		now:   now,
		armed: make(chan struct{}, 8),
	}
}

func (c *fakeClock) lock()   { c.mu <- struct{}{} }
func (c *fakeClock) unlock() { <-c.mu }

func (c *fakeClock) Now() time.Time {
	c.lock()
	defer c.unlock()
	return c.now
}

func (c *fakeClock) NewTimer(d time.Duration) Timer {
	c.lock()
	t := &fakeTimer{
		c:     make(chan time.Time, 1),
		when:  c.now.Add(d),
		clock: c,
	}
	c.timers = append(c.timers, t)
	c.unlock()
	c.armed <- struct{}{}
	return t
}

func (t *fakeTimer) C() <-chan time.Time { return t.c }

func (t *fakeTimer) Stop() bool {
	t.clock.lock()
	defer t.clock.unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

func (c *fakeClock) Advance(d time.Duration) {
	c.lock()
	c.now = c.now.Add(d)
	now := c.now
	var fire []*fakeTimer
	keep := c.timers[:0]
	for _, timer := range c.timers {
		if timer.stopped {
			continue
		}
		if !timer.when.After(now) {
			timer.stopped = true
			fire = append(fire, timer)
			continue
		}
		keep = append(keep, timer)
	}
	c.timers = keep
	c.unlock()
	for _, timer := range fire {
		timer.c <- now
	}
}

func (c *fakeClock) waitArmed(t *testing.T) {
	t.Helper()
	select {
	case <-c.armed:
	case <-time.After(2 * time.Second):
		t.Fatal("debouncer did not arm a timer")
	}
}

func TestDebounceCollapsesBurstToResume(t *testing.T) {
	clock := newFakeClock(time.Unix(1_700_000_000, 0))
	in := make(chan Event)
	out := make(chan Event, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Debounce(ctx, clock, time.Second, in, out)

	in <- Event{Kind: KindNetwork}
	clock.waitArmed(t)
	in <- Event{Kind: KindResume}
	in <- Event{Kind: KindNetwork}
	clock.Advance(time.Second)

	got := readEvent(t, out)
	if got.Kind != KindResume {
		t.Fatalf("burst kind = %q, want resume", got.Kind)
	}
	expectNoEvent(t, out)
}

func TestDebounceEmitsNetworkAndASecondBurst(t *testing.T) {
	clock := newFakeClock(time.Unix(1_700_000_000, 0))
	in := make(chan Event)
	out := make(chan Event, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Debounce(ctx, clock, 500*time.Millisecond, in, out)

	in <- Event{Kind: KindNetwork}
	clock.waitArmed(t)
	clock.Advance(500 * time.Millisecond)
	first := readEvent(t, out)
	if first.Kind != KindNetwork {
		t.Fatalf("first kind = %q", first.Kind)
	}

	in <- Event{Kind: KindNetwork}
	clock.waitArmed(t)
	in <- Event{Kind: KindNetwork}
	clock.Advance(500 * time.Millisecond)
	second := readEvent(t, out)
	if second.Kind != KindNetwork {
		t.Fatalf("second kind = %q", second.Kind)
	}
	expectNoEvent(t, out)
}

func readEvent(t *testing.T, out <-chan Event) Event {
	t.Helper()
	select {
	case ev := <-out:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a debounced event")
		return Event{}
	}
}

func expectNoEvent(t *testing.T, out <-chan Event) {
	t.Helper()
	select {
	case ev := <-out:
		t.Fatalf("unexpected extra event %q", ev.Kind)
	default:
	}
}
