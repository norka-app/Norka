package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"norka/internal/update"
)

//go:embed wails.json
var wailsConfig []byte

// GetAppVersion returns the version baked into this build from wails.json.
func (a *App) GetAppVersion() string {
	return appVersion()
}

// CheckForUpdate looks up the latest GitHub release. A newer release is offered
// when it has a download link for this operating system.
func (a *App) CheckForUpdate() (update.Offer, error) {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()

	offer, err := update.Check(ctx, appVersion(), runtime.GOOS)
	if err != nil {
		slog.Info("update check failed", "err", err)
		return update.Offer{Current: appVersion()}, err
	}
	if offer.Available {
		slog.Info("update available", "current", offer.Current, "latest", offer.Latest)
	}
	return offer, nil
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
