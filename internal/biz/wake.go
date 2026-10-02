package biz

import (
	"context"
	"time"

	"norka/internal/forward"
	"norka/internal/netwatch"
	"norka/internal/wake"
)

// RecoverAfterWake probes every tunnel that is still supposed to be running.
// Tunnels the user stopped are not in the runtime map and stay stopped.
func (b *TunnelBiz) RecoverAfterWake(ctx context.Context, kind netwatch.Kind, now time.Time, opts wake.Options) wake.Result {
	if b == nil {
		return wake.Result{}
	}
	if now.IsZero() {
		now = time.Now()
	}
	b.mu.Lock()
	sessions := make(wakeSessions, 0, len(b.runs))
	prober := wakeProber{byID: make(map[int]*forward.LocalForward, len(b.runs))}
	for id, run := range b.runs {
		if run == nil {
			continue
		}
		sessions = append(sessions, wake.Tunnel{ID: id, Phase: run.Phase()})
		prober.byID[id] = run
	}
	b.mu.Unlock()
	return wake.Handle(ctx, kind, now, sessions, prober, opts)
}

type wakeSessions []wake.Tunnel

func (s wakeSessions) Sessions() []wake.Tunnel { return s }

type wakeProber struct {
	byID map[int]*forward.LocalForward
}

func (p wakeProber) Probe(ctx context.Context, id int, timeout time.Duration) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	run := p.byID[id]
	if run == nil {
		return false, nil
	}
	return run.Probe(timeout)
}

func (p wakeProber) ReconnectNow(id int) bool {
	run := p.byID[id]
	if run == nil {
		return false
	}
	return run.ReconnectNow()
}
