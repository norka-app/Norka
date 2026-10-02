package wake

import (
	"context"
	"testing"
	"time"

	"norka/internal/netwatch"
)

type staticSessions []Tunnel

func (s staticSessions) Sessions() []Tunnel { return s }

type fakeProber struct {
	mu         chan struct{}
	alive      map[int]bool
	probes     []int
	reconnects []int
	timeout    time.Duration
	reject     map[int]bool
}

func newFakeProber() *fakeProber {
	return &fakeProber{mu: make(chan struct{}, 1), alive: map[int]bool{}, reject: map[int]bool{}}
}

func (p *fakeProber) lock()   { p.mu <- struct{}{} }
func (p *fakeProber) unlock() { <-p.mu }

func (p *fakeProber) Probe(ctx context.Context, id int, timeout time.Duration) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	p.lock()
	defer p.unlock()
	p.probes = append(p.probes, id)
	p.timeout = timeout
	return p.alive[id], nil
}

func (p *fakeProber) ReconnectNow(id int) bool {
	p.lock()
	defer p.unlock()
	if p.reject[id] {
		return false
	}
	p.reconnects = append(p.reconnects, id)
	return true
}

func (p *fakeProber) probed() []int {
	p.lock()
	defer p.unlock()
	return append([]int(nil), p.probes...)
}

func (p *fakeProber) reconnected() []int {
	p.lock()
	defer p.unlock()
	return append([]int(nil), p.reconnects...)
}

func TestHandleSkipsStoppedAndError(t *testing.T) {
	prober := newFakeProber()
	now := time.Unix(1_700_000_000, 0)
	result := Handle(context.Background(), netwatch.KindResume, now, staticSessions{
		{ID: 1, Phase: PhaseStopped},
		{ID: 2, Phase: PhaseError},
		{ID: 3, Phase: PhaseStarting},
	}, prober, Options{Notifications: true, NotifyOnReconnect: true})
	if len(result.Acted) != 0 || result.JournalKey != "" || result.Notices != 0 {
		t.Fatalf("stopped tunnels were touched: %+v", result)
	}
	if len(prober.probed()) != 0 || len(prober.reconnected()) != 0 {
		t.Fatal("prober was called for a tunnel the user is not running")
	}
}

func TestHandleProbesLiveAndResetsReconnecting(t *testing.T) {
	prober := newFakeProber()
	prober.alive[1] = true
	now := time.Unix(1_700_000_000, 0)
	result := Handle(context.Background(), netwatch.KindNetwork, now, staticSessions{
		{ID: 1, Name: "alive", Phase: PhaseConnected},
		{ID: 2, Name: "dead", Phase: PhaseConnected},
		{ID: 4, Name: "backing off", Phase: PhaseReconnecting},
		{ID: 9, Name: "stopped", Phase: PhaseStopped},
	}, prober, Options{
		GiveUp:            GiveUpWindow,
		Notifications:     true,
		NotifyOnDrop:      true,
		NotifyOnReconnect: true,
	})
	if result.JournalKey != JournalNetwork {
		t.Fatalf("journal = %q", result.JournalKey)
	}
	if result.Notices != 1 {
		t.Fatalf("notices = %d, want one notice for the whole burst", result.Notices)
	}
	if !result.Deadline.Equal(now.Add(GiveUpWindow)) {
		t.Fatalf("deadline = %v, want the give-up window restarted", result.Deadline)
	}
	if len(result.Acted) != 2 || result.Acted[0] != 2 || result.Acted[1] != 4 {
		t.Fatalf("acted = %v, want dead connected and reconnecting", result.Acted)
	}
	probes := prober.probed()
	if len(probes) != 2 {
		t.Fatalf("probes = %v, want only the connected tunnels", probes)
	}
	if prober.timeout != ProbeTimeout {
		t.Fatalf("probe timeout = %v, want %v", prober.timeout, ProbeTimeout)
	}
	reconnects := prober.reconnected()
	if len(reconnects) != 2 {
		t.Fatalf("reconnects = %v", reconnects)
	}
}

func TestHandleResumeJournalAndNotificationPolicy(t *testing.T) {
	prober := newFakeProber()
	now := time.Unix(1_700_000_000, 0)
	sessions := staticSessions{{ID: 7, Phase: PhaseConnected}}
	off := Handle(context.Background(), netwatch.KindResume, now, sessions, prober, Options{})
	if off.JournalKey != JournalResume {
		t.Fatalf("journal = %q", off.JournalKey)
	}
	if off.Notices != 0 {
		t.Fatal("notifications feature off must not post")
	}
	quiet := Handle(context.Background(), netwatch.KindResume, now, sessions, prober, Options{Notifications: true})
	if quiet.Notices != 0 {
		t.Fatal("drop and reconnect notifications are both off")
	}
	on := Handle(context.Background(), netwatch.KindResume, now, staticSessions{
		{ID: 1, Phase: PhaseConnected},
		{ID: 2, Phase: PhaseConnected},
		{ID: 3, Phase: PhaseReconnecting},
	}, prober, Options{Notifications: true, NotifyOnReconnect: true})
	if on.Notices != 1 || len(on.Acted) != 3 {
		t.Fatalf("want one notice for three tunnels, got %+v", on)
	}
}

func TestHandleIgnoresRejectedReconnect(t *testing.T) {
	prober := newFakeProber()
	prober.reject[5] = true
	result := Handle(context.Background(), netwatch.KindResume, time.Unix(0, 0), staticSessions{
		{ID: 5, Phase: PhaseReconnecting},
	}, prober, Options{Notifications: true, NotifyOnDrop: true})
	if len(result.Acted) != 0 || result.JournalKey != "" || result.Notices != 0 {
		t.Fatalf("a tunnel that stayed stopped was reported: %+v", result)
	}
}

func TestChargeAwakeDoesNotCountSleep(t *testing.T) {
	full := GiveUpWindow
	slept := ChargeAwake(full, full, 2*time.Hour, time.Minute, netwatch.JumpGap)
	if slept != full {
		t.Fatalf("sleep consumed the window: %v", slept)
	}
	awake := ChargeAwake(full, full, 500*time.Millisecond, 500*time.Millisecond, netwatch.JumpGap)
	if awake != full-500*time.Millisecond {
		t.Fatalf("awake wait = %v", awake)
	}
	border := ChargeAwake(full, full, time.Minute+netwatch.JumpGap, time.Minute, netwatch.JumpGap)
	if border != full-(time.Minute+netwatch.JumpGap) {
		t.Fatalf("threshold gap was treated as sleep: %v", border)
	}
	over := ChargeAwake(full, full, time.Minute+netwatch.JumpGap+time.Millisecond, time.Minute, netwatch.JumpGap)
	if over != full {
		t.Fatalf("oversized wait = %v, want a reset window", over)
	}
}

func TestDebouncedResumeDrivesOneReconnect(t *testing.T) {
	clock := newWakeClock(time.Unix(1_700_000_000, 0))
	in := make(chan netwatch.Event)
	out := make(chan netwatch.Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go netwatch.Debounce(ctx, clock, time.Second, in, out)

	in <- netwatch.Event{Kind: netwatch.KindNetwork}
	clock.waitArmed(t)
	in <- netwatch.Event{Kind: netwatch.KindResume}
	clock.Advance(time.Second)
	ev := <-out

	prober := newFakeProber()
	result := Handle(ctx, ev.Kind, clock.Now(), staticSessions{
		{ID: 1, Phase: PhaseConnected},
		{ID: 2, Phase: PhaseStopped},
	}, prober, Options{GiveUp: GiveUpWindow, Notifications: true, NotifyOnReconnect: true})
	if ev.Kind != netwatch.KindResume || result.JournalKey != JournalResume {
		t.Fatalf("event %q journal %q", ev.Kind, result.JournalKey)
	}
	if result.Notices != 1 || len(result.Acted) != 1 || result.Acted[0] != 1 {
		t.Fatalf("result = %+v", result)
	}
	if !result.Deadline.Equal(clock.Now().Add(GiveUpWindow)) {
		t.Fatalf("deadline = %v", result.Deadline)
	}
}

// wakeClock is the same shape as the netwatch test clock, kept here so the
// reconnect decision can be driven by an injected clock and event source.
type wakeClock struct {
	mu     chan struct{}
	now    time.Time
	timers []*wakeTimer
	armed  chan struct{}
}

type wakeTimer struct {
	c       chan time.Time
	when    time.Time
	clock   *wakeClock
	stopped bool
}

func newWakeClock(now time.Time) *wakeClock {
	return &wakeClock{mu: make(chan struct{}, 1), now: now, armed: make(chan struct{}, 4)}
}

func (c *wakeClock) lock()   { c.mu <- struct{}{} }
func (c *wakeClock) unlock() { <-c.mu }

func (c *wakeClock) Now() time.Time {
	c.lock()
	defer c.unlock()
	return c.now
}

func (c *wakeClock) NewTimer(d time.Duration) netwatch.Timer {
	c.lock()
	timer := &wakeTimer{c: make(chan time.Time, 1), when: c.now.Add(d), clock: c}
	c.timers = append(c.timers, timer)
	c.unlock()
	c.armed <- struct{}{}
	return timer
}

func (t *wakeTimer) C() <-chan time.Time { return t.c }

func (t *wakeTimer) Stop() bool {
	t.clock.lock()
	defer t.clock.unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

func (c *wakeClock) Advance(d time.Duration) {
	c.lock()
	c.now = c.now.Add(d)
	now := c.now
	var fire []*wakeTimer
	for _, timer := range c.timers {
		if timer.stopped {
			continue
		}
		if !timer.when.After(now) {
			timer.stopped = true
			fire = append(fire, timer)
		}
	}
	c.unlock()
	for _, timer := range fire {
		timer.c <- now
	}
}

func (c *wakeClock) waitArmed(t *testing.T) {
	t.Helper()
	select {
	case <-c.armed:
	case <-time.After(2 * time.Second):
		t.Fatal("debouncer did not arm a timer")
	}
}
