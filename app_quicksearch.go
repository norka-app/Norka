package main

import (
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"

	"norka/internal/conf"
	"norka/internal/features"
	"norka/internal/hotkey"
	"norka/internal/model"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const eventQuickSearch = "quicksearch:open"

// quickSearchHotkey is the registered global chord and the last error, so
// Settings can show a registration failure without guessing.
type quickSearchHotkey struct {
	mu          sync.Mutex
	stop        func()
	registered  bool
	errorCode   string
	errorDetail string
	effective   string
}

func (a *App) bindQuickSearchHotkey() {
	if err := a.ensureReady(); err != nil {
		return
	}
	cfg, err := a.storage.Load()
	if err != nil {
		slog.Warn("quick search settings unread", "err", err)
		return
	}
	a.applyQuickSearchHotkey(cfg.QuickSearchOn(), cfg.QuickSearchHotkey)
}

func (a *App) unbindQuickSearchHotkey() {
	a.quickSearchHotkey.mu.Lock()
	stop := a.quickSearchHotkey.stop
	a.quickSearchHotkey.stop = nil
	a.quickSearchHotkey.registered = false
	a.quickSearchHotkey.mu.Unlock()
	if stop != nil {
		stop()
	}
}

func (a *App) applyQuickSearchHotkey(enabled bool, spec string) {
	spec = strings.TrimSpace(spec)
	effective := spec
	if effective == "" {
		effective = hotkey.PlatformDefault()
	}

	a.unbindQuickSearchHotkey()

	state := &a.quickSearchHotkey
	state.mu.Lock()
	state.effective = effective
	state.errorCode = ""
	state.errorDetail = ""
	state.registered = false
	state.mu.Unlock()

	if !enabled {
		return
	}
	stop, err := hotkey.Listen(effective, func() {
		go a.OpenQuickSearch()
	})
	state.mu.Lock()
	defer state.mu.Unlock()
	state.effective = effective
	if err != nil {
		state.errorCode, state.errorDetail = classifyHotkeyError(err)
		slog.Warn("quick search hotkey not registered", "hotkey", effective, "code", state.errorCode, "err", err)
		return
	}
	state.stop = stop
	state.registered = true
	slog.Info("quick search hotkey registered", "hotkey", effective)
}

func classifyHotkeyError(err error) (string, string) {
	switch {
	case err == nil:
		return "", ""
	case errors.Is(err, hotkey.ErrInvalid):
		return "invalid", ""
	case errors.Is(err, hotkey.ErrUnsupported):
		return "unsupported", ""
	case errors.Is(err, hotkey.ErrRegisterFailed):
		detail := strings.TrimSpace(strings.TrimPrefix(err.Error(), hotkey.ErrRegisterFailed.Error()))
		detail = strings.TrimPrefix(detail, ":")
		return "register_failed", strings.TrimSpace(detail)
	default:
		return "register_failed", err.Error()
	}
}

func (a *App) GetQuickSearchSettings() (model.QuickSearchSettings, error) {
	settings := model.QuickSearchSettings{
		Enabled:         true,
		EffectiveHotkey: hotkey.PlatformDefault(),
		Platform:        runtime.GOOS,
	}
	if err := a.ensureReady(); err != nil {
		settings.ErrorCode = "unsupported"
		return settings, err
	}
	cfg, err := a.storage.Load()
	if err != nil {
		return settings, err
	}
	settings.Enabled = cfg.Features.Enabled(features.QuickSearch)
	settings.Hotkey = cfg.QuickSearchHotkey
	if strings.TrimSpace(cfg.QuickSearchHotkey) != "" {
		settings.EffectiveHotkey = cfg.QuickSearchHotkey
	}
	a.quickSearchHotkey.mu.Lock()
	settings.Registered = a.quickSearchHotkey.registered
	settings.ErrorCode = a.quickSearchHotkey.errorCode
	settings.ErrorDetail = a.quickSearchHotkey.errorDetail
	if a.quickSearchHotkey.effective != "" {
		settings.EffectiveHotkey = a.quickSearchHotkey.effective
	}
	a.quickSearchHotkey.mu.Unlock()
	return settings, nil
}

func (a *App) SetQuickSearchSettings(input model.QuickSearchSettings) (model.QuickSearchSettings, error) {
	if err := a.ensureReady(); err != nil {
		return model.QuickSearchSettings{}, err
	}
	spec := strings.TrimSpace(input.Hotkey)
	if spec != "" {
		if _, err := hotkey.Parse(spec); err != nil {
			return model.QuickSearchSettings{}, err
		}
	}
	enabled := input.Enabled
	cfg, err := a.storage.Update(func(cfg *conf.Config) error {
		if err := cfg.Features.Set(features.QuickSearch, enabled); err != nil {
			return err
		}
		cfg.QuickSearchEnabled = &enabled
		cfg.QuickSearchHotkey = spec
		return nil
	})
	if err != nil {
		return model.QuickSearchSettings{}, err
	}
	a.applyQuickSearchHotkey(enabled, spec)
	a.publishFeatures(cfg.Features)
	return a.GetQuickSearchSettings()
}

// OpenQuickSearch shows the main window (even from the tray) and asks the UI
// to open the command palette. A second press closes it. If the window was
// hidden, closing the palette hides it again.
func (a *App) OpenQuickSearch() {
	if a.ctx == nil || !a.featureOn(features.QuickSearch) {
		return
	}
	if a.quickSearchOpen.Load() {
		a.FinishQuickSearch(a.quickSearchRestoreHide.Load())
		return
	}
	hidden := !a.windowVisible.Load()
	a.quickSearchRestoreHide.Store(hidden)
	a.quickSearchOpen.Store(true)
	if hidden {
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	}
	a.showMainWindow()
	wailsruntime.EventsEmit(a.ctx, eventQuickSearch, map[string]bool{
		"open":        true,
		"restoreHide": hidden,
	})
}

// FinishQuickSearch closes the palette session. restoreHide puts the window
// back in the tray when the hotkey opened it from there.
func (a *App) FinishQuickSearch(restoreHide bool) {
	a.quickSearchOpen.Store(false)
	a.quickSearchRestoreHide.Store(false)
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventQuickSearch, map[string]bool{"open": false})
	if restoreHide {
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
		a.hideMainWindow()
	}
}
