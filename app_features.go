package main

import (
	"fmt"
	"strings"

	"norka/internal/conf"
	"norka/internal/features"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const eventFeaturesChanged = "features:changed"

// GetFeatures returns every flag with its effective on/off state.
func (a *App) GetFeatures() ([]features.View, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	cfg, err := a.storage.Load()
	if err != nil {
		return nil, err
	}
	return cfg.Features.Views(), nil
}

// SetFeature stores an explicit on/off choice and applies it immediately.
// Turning a feature off does not delete its saved data. Profiles in particular
// stay in config.toml, and tunnels that are already running stay running.
func (a *App) SetFeature(id string, enabled bool) ([]features.View, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	featureID := features.ID(strings.TrimSpace(id))
	if !features.Known(featureID) {
		return nil, fmt.Errorf("unknown feature %q", id)
	}
	cfg, err := a.storage.Update(func(cfg *conf.Config) error {
		return cfg.Features.Set(featureID, enabled)
	})
	if err != nil {
		return nil, err
	}
	a.applyFeatureSideEffects(cfg)
	a.publishFeatures(cfg.Features)
	return cfg.Features.Views(), nil
}

func (a *App) featureOn(id features.ID) bool {
	if a == nil || a.storage == nil {
		return features.Default(id)
	}
	cfg, err := a.storage.Load()
	if err != nil {
		return features.Default(id)
	}
	return cfg.Features.Enabled(id)
}

func (a *App) publishFeatures(flags features.Flags) {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventFeaturesChanged, flags.Views())
}

// applyFeatureSideEffects makes the backend match the flags: hotkey, notifier, tray.
func (a *App) applyFeatureSideEffects(cfg *conf.Config) {
	if a == nil || cfg == nil {
		return
	}
	a.applyQuickSearchHotkey(cfg.Features.Enabled(features.QuickSearch), cfg.QuickSearchHotkey)
	a.loadNotifySettings()
	a.invalidateTrayMenu()
	a.syncWakeWatch(cfg.Features.Enabled(features.WakeReconnect))
}
