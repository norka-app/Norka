package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/update"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed wails.json
var wailsConfig []byte

var (
	updateMu     sync.Mutex
	cachedOffer  update.Offer
	updateCancel context.CancelFunc
)

// GetAppVersion returns the version baked into this build from wails.json.
func (a *App) GetAppVersion() string {
	return appVersion()
}

// CheckForUpdate looks up the latest GitHub release. A newer stable release is
// offered when it has a download link for this operating system. A version the
// user skipped stays hidden.
func (a *App) CheckForUpdate() (update.Offer, error) {
	return a.lookupUpdate(false)
}

// CheckForUpdateNow is the manual check from Settings. It still reports a
// version the user previously skipped.
func (a *App) CheckForUpdateNow() (update.Offer, error) {
	return a.lookupUpdate(true)
}

func (a *App) lookupUpdate(manual bool) (update.Offer, error) {
	if !a.featureOn(features.AutoUpdate) {
		return update.Offer{Current: appVersion()}, nil
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()

	offer, err := update.Check(ctx, appVersion(), runtime.GOOS)
	if err != nil {
		slog.Info("update check failed", "err", err)
		return update.Offer{Current: appVersion()}, err
	}
	offer = update.SuppressSkipped(offer, update.ReadSkip(a.configDir()), manual)
	rememberOffer(offer)
	if offer.Available {
		slog.Info("update available", "current", offer.Current, "latest", offer.Latest, "apply", offer.CanApply)
	}
	return offer, nil
}

// ApplyUpdate downloads the cached offer, verifies it, and restarts into the
// new build when the install location is writable.
func (a *App) ApplyUpdate() (update.ApplyResult, error) {
	if !a.featureOn(features.AutoUpdate) {
		return update.ApplyResult{Code: update.CodeNone}, nil
	}
	updateMu.Lock()
	if updateCancel != nil {
		updateMu.Unlock()
		return update.ApplyResult{Code: update.CodeBusy}, nil
	}
	offer := cachedOffer
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	updateCancel = cancel
	updateMu.Unlock()

	defer func() {
		cancel()
		updateMu.Lock()
		updateCancel = nil
		updateMu.Unlock()
	}()

	result, err := update.Apply(ctx, offer, func(progress update.Progress) {
		if a == nil || a.ctx == nil {
			return
		}
		wailsruntime.EventsEmit(a.ctx, "update:progress", progress)
	})
	if err != nil {
		slog.Info("update apply failed", "err", err)
		return result, err
	}
	if result.Restarting {
		slog.Info("update staged; restarting")
		a.PrepareForQuit()
		go func() {
			time.Sleep(400 * time.Millisecond)
			if a.ctx != nil {
				wailsruntime.Quit(a.ctx)
			}
		}()
	}
	return result, nil
}

// SkipUpdateVersion remembers a version so automatic checks stay quiet.
func (a *App) SkipUpdateVersion(version string) error {
	if err := update.WriteSkip(a.configDir(), version); err != nil {
		return err
	}
	slog.Info("update skipped", "version", version)
	return nil
}

// CancelUpdate aborts an in-progress download.
func (a *App) CancelUpdate() {
	updateMu.Lock()
	cancel := updateCancel
	updateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func rememberOffer(offer update.Offer) {
	updateMu.Lock()
	defer updateMu.Unlock()
	// A quiet check must not wipe the offer the user is already looking at.
	if !offer.Available && cachedOffer.Available {
		return
	}
	cachedOffer = offer
}

func (a *App) configDir() string {
	if a == nil || a.storage() == nil {
		return ""
	}
	return filepath.Dir(a.storage().Path())
}

func appVersion() string {
	var doc struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsConfig, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Info.ProductVersion)
}
