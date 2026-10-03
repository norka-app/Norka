package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/wake"
)

func (e *Engine) SyncWakeWatch(enabled bool) {
	if e == nil {
		return
	}
	e.wakeMu.Lock()
	defer e.wakeMu.Unlock()
	if !enabled {
		e.stopWakeLocked()
		return
	}
	if e.wakeCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, err := e.openWatch(ctx)
	if err != nil {
		cancel()
		slog.Warn("wake reconnect watch failed", "err", err)
		return
	}
	e.wakeCancel = cancel
	go e.consumeWake(ctx, events)
}

func (e *Engine) stopWakeLocked() {
	if e.wakeCancel == nil {
		return
	}
	e.wakeCancel()
	e.wakeCancel = nil
}

func (e *Engine) openWatch(ctx context.Context) (<-chan netwatch.Event, error) {
	if e.watch != nil {
		return e.watch(ctx)
	}
	return netwatch.Start(ctx)
}

func (e *Engine) consumeWake(ctx context.Context, events <-chan netwatch.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			e.onWake(ev)
		}
	}
}

func (e *Engine) onWake(ev netwatch.Event) {
	if e == nil || e.active() == nil || !e.FeatureOn(features.WakeReconnect) {
		return
	}
	settings := e.currentNotifySettings()
	result := e.active().RecoverAfterWake(context.Background(), ev.Kind, time.Now(), wake.Options{
		GiveUp:            wake.GiveUpWindow,
		Notifications:     settings.Enabled,
		NotifyOnDrop:      settings.Dropped,
		NotifyOnReconnect: settings.Reconnected,
	})
	if result.JournalKey == "" {
		return
	}
	slog.Info("wake reconnect", "kind", string(ev.Kind), "tunnels", len(result.Acted))
	e.emitJournal(result.JournalKey)
	if result.Notices > 0 && e.notifier != nil {
		e.notifier.PostWake(ev.Kind == netwatch.KindResume)
	}
}

func (e *Engine) emitJournal(key string) {
	if e == nil || key == "" {
		return
	}
	e.emit(EventJournal, map[string]string{
		"level": "info",
		"key":   key,
	})
}
