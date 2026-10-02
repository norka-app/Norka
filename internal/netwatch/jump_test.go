package netwatch

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestJumpDetectedIgnoresOrdinaryTick(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	if JumpDetected(start, 0, start.Add(5*time.Second), 5*time.Second, JumpGap) {
		t.Fatal("a 5s tick must not look like sleep")
	}
}

func TestJumpDetectedSeesWallGap(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	// Monotonic time moved 5s; the wall clock moved 45s.
	if !JumpDetected(start, 0, start.Add(45*time.Second), 5*time.Second, 30*time.Second) {
		t.Fatal("expected a sleep-sized gap")
	}
	// Exactly the threshold stays quiet. The detector uses a strict greater-than.
	if JumpDetected(start, 0, start.Add(35*time.Second), 5*time.Second, 30*time.Second) {
		t.Fatal("gap equal to the threshold must not fire")
	}
}

type manualClock struct {
	mu   sync.Mutex
	wall time.Time
	mono time.Duration
}

func (c *manualClock) Wall() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wall
}

func (c *manualClock) Mono() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mono
}

func (c *manualClock) Set(wall time.Time, mono time.Duration) {
	c.mu.Lock()
	c.wall = wall
	c.mono = mono
	c.mu.Unlock()
}

type readyClock struct {
	*manualClock
	ready chan struct{}
	once  sync.Once
}

func (c *readyClock) Wall() time.Time {
	c.once.Do(func() { close(c.ready) })
	return c.manualClock.Wall()
}

func TestRunJumpDetectorEmitsResume(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	clock := &readyClock{
		manualClock: &manualClock{wall: start, mono: 0},
		ready:       make(chan struct{}),
	}
	ticks := make(chan struct{})
	out := make(chan Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunJumpDetector(ctx, clock, ticks, 30*time.Second, out)
	select {
	case <-clock.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("detector did not sample the clock")
	}

	clock.Set(start.Add(5*time.Second), 5*time.Second)
	ticks <- struct{}{}
	ticks <- struct{}{}
	expectNoEvent(t, out)

	clock.Set(start.Add(50*time.Second), 10*time.Second)
	ticks <- struct{}{}
	got := readEvent(t, out)
	if got.Kind != KindResume {
		t.Fatalf("kind = %q, want resume", got.Kind)
	}
}
