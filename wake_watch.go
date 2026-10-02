package main

import (
	"context"
	"log/slog"
	"time"

	"norka/internal/features"
	"norka/internal/netwatch"
	"norka/internal/wake"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const eventJournal = "journal:entry"

func (a *App) syncWakeWatch(enabled bool) {
	if a == nil {
		return
	}
	a.wakeMu.Lock()
	defer a.wakeMu.Unlock()
	if !enabled {
		a.stopWakeLocked()
		return
	}
	if a.wakeCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, err := netwatch.Start(ctx)
	if err != nil {
		cancel()
		slog.Warn("wake reconnect watch failed", "err", err)
		return
	}
	a.wakeCancel = cancel
	go a.consumeWake(ctx, events)
}

func (a *App) stopWakeLocked() {
	if a.wakeCancel == nil {
		return
	}
	a.wakeCancel()
	a.wakeCancel = nil
}

func (a *App) consumeWake(ctx context.Context, events <-chan netwatch.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			a.onWake(ev)
		}
	}
}

func (a *App) onWake(ev netwatch.Event) {
	if a == nil || a.tunnel == nil || !a.featureOn(features.WakeReconnect) {
		return
	}
	settings := a.currentNotifySettings()
	result := a.tunnel.RecoverAfterWake(context.Background(), ev.Kind, time.Now(), wake.Options{
		GiveUp:            wake.GiveUpWindow,
		Notifications:     settings.Enabled,
		NotifyOnDrop:      settings.Dropped,
		NotifyOnReconnect: settings.Reconnected,
	})
	if result.JournalKey == "" {
		return
	}
	slog.Info("wake reconnect", "kind", string(ev.Kind), "tunnels", len(result.Acted))
	a.emitJournal(result.JournalKey)
	if result.Notices > 0 && a.notifier != nil {
		a.notifier.PostWake(ev.Kind == netwatch.KindResume)
	}
}

func (a *App) emitJournal(key string) {
	if a == nil || a.ctx == nil || key == "" {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventJournal, map[string]string{
		"level": "info",
		"key":   key,
	})
}
