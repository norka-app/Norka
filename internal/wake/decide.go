// Package wake decides which live tunnels to probe after sleep or a network change.
package wake

import (
	"context"
	"sort"
	"sync"
	"time"

	"norka/internal/netwatch"
)

const (
	// GiveUpWindow is the reconnect budget. Sleep must not consume it.
	GiveUpWindow = 15 * time.Minute
	// ProbeTimeout is the short keepalive used right after a wake.
	ProbeTimeout = 2 * time.Second

	PhaseConnected    = "connected"
	PhaseReconnecting = "reconnecting"
	PhaseStopped      = "stopped"
	PhaseStarting     = "starting"
	PhaseError        = "error"

	ActionSkip         = "skip"
	ActionProbe        = "probe"
	ActionReconnectNow = "reconnect"

	JournalResume  = "app.logs.wakeResume"
	JournalNetwork = "app.logs.wakeNetwork"
)

// Tunnel is one runtime the coordinator can see. Stopped tunnels are not included
// by the caller; a stopped phase is still skipped if it appears.
type Tunnel struct {
	ID    int
	Name  string
	Phase string
}

// Sessions is the injectable source of tunnels that should be running.
type Sessions interface {
	Sessions() []Tunnel
}

// Prober checks an SSH session and, when asked, reconnects immediately.
type Prober interface {
	Probe(ctx context.Context, id int, timeout time.Duration) (alive bool, err error)
	ReconnectNow(id int) bool
}

// Options carries the give-up window and the current notification policy.
type Options struct {
	GiveUp            time.Duration
	Notifications     bool
	NotifyOnDrop      bool
	NotifyOnReconnect bool
}

// Result is one wake. Notices is 0 or 1: a burst never becomes one notice per tunnel.
type Result struct {
	JournalKey string
	Notices    int
	Acted      []int
	Deadline   time.Time
}

// Decide maps a tunnel phase to the next step.
func Decide(phase string) string {
	switch phase {
	case PhaseConnected:
		return ActionProbe
	case PhaseReconnecting:
		return ActionReconnectNow
	default:
		return ActionSkip
	}
}

// JournalKey is the vue-i18n path for the single journal line.
func JournalKey(kind netwatch.Kind) string {
	if kind == netwatch.KindResume {
		return JournalResume
	}
	return JournalNetwork
}

// ChargeAwake subtracts only time the process was actually awake.
// A wait that ran much longer than scheduled is treated as sleep: the whole
// give-up window is restored instead of charging the gap.
func ChargeAwake(budget, full, elapsed, scheduled, sleepGap time.Duration) time.Duration {
	if full <= 0 {
		full = budget
	}
	if scheduled < 0 {
		scheduled = 0
	}
	if elapsed < 0 {
		elapsed = 0
	}
	if sleepGap < 0 {
		sleepGap = 0
	}
	if elapsed > scheduled+sleepGap {
		return full
	}
	if elapsed >= budget {
		return 0
	}
	return budget - elapsed
}

// Handle probes connected sessions and resets ones that are already reconnecting.
// now is the injectable clock. Dead sessions reconnect immediately and the
// give-up deadline restarts at now, so time spent asleep is not counted.
func Handle(ctx context.Context, kind netwatch.Kind, now time.Time, sessions Sessions, prober Prober, opts Options) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.GiveUp <= 0 {
		opts.GiveUp = GiveUpWindow
	}
	var jobs []Tunnel
	if sessions != nil {
		for _, tunnel := range sessions.Sessions() {
			if Decide(tunnel.Phase) == ActionSkip {
				continue
			}
			jobs = append(jobs, tunnel)
		}
	}
	var (
		mu    sync.Mutex
		acted []int
		wg    sync.WaitGroup
	)
	for _, tunnel := range jobs {
		wg.Add(1)
		go func(tunnel Tunnel) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			if Decide(tunnel.Phase) == ActionProbe && prober != nil {
				alive, _ := prober.Probe(ctx, tunnel.ID, ProbeTimeout)
				if alive {
					return
				}
			}
			if prober == nil || !prober.ReconnectNow(tunnel.ID) {
				return
			}
			mu.Lock()
			acted = append(acted, tunnel.ID)
			mu.Unlock()
		}(tunnel)
	}
	wg.Wait()
	sort.Ints(acted)
	result := Result{Acted: acted}
	if len(acted) == 0 {
		return result
	}
	result.JournalKey = JournalKey(kind)
	result.Deadline = now.Add(opts.GiveUp)
	if opts.Notifications && (opts.NotifyOnDrop || opts.NotifyOnReconnect) {
		result.Notices = 1
	}
	return result
}
