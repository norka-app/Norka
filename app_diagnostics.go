package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/diagnostics"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/tunnelstats"
	"github.com/norka-app/Norka/internal/uilocale"
)

const eventDiagnosticsSaved = "diagnostics:saved"

// DiagnosticsResult is what Settings and the tray show after a save.
type DiagnosticsResult struct {
	Cancelled bool   `json:"cancelled"`
	Path      string `json:"path"`
	System    string `json:"system"`
	IssueURL  string `json:"issueUrl"`
}

// SaveDiagnostics asks where to write a redacted bug-report zip.
// Hosts and usernames are omitted unless includeHosts is true.
// Theme is "light" or "dark" from the interface; the tray passes an empty theme.
func (a *App) SaveDiagnostics(includeHosts bool, theme string) (DiagnosticsResult, error) {
	if err := a.ensureReady(); err != nil {
		return DiagnosticsResult{}, err
	}
	if !a.featureOn(features.Diagnostics) {
		return DiagnosticsResult{}, fmt.Errorf("diagnostics are disabled")
	}
	if a.ctx == nil {
		return DiagnosticsResult{}, fmt.Errorf("window is not ready")
	}
	text := a.uiText()
	dest, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultFilename: diagnostics.Filename(time.Now()),
		Title:           text.DiagnosticsSaveTitle,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: text.ZipFilter, Pattern: "*.zip"},
		},
	})
	if err != nil {
		return DiagnosticsResult{}, fmt.Errorf("file dialog: %w", err)
	}
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return DiagnosticsResult{Cancelled: true}, nil
	}
	if !strings.EqualFold(filepath.Ext(dest), ".zip") {
		dest += ".zip"
	}
	bundle, err := a.diagnosticsBundle(includeHosts, theme)
	if err != nil {
		return DiagnosticsResult{}, err
	}
	if err := writeDiagnosticsZip(dest, bundle); err != nil {
		return DiagnosticsResult{}, err
	}
	a.rememberDiagnostics(dest)
	slog.Info("diagnostics saved", "path", dest, "include_hosts", includeHosts)
	return DiagnosticsResult{
		Path:     dest,
		System:   bundle.SystemText,
		IssueURL: bundle.IssueURL,
	}, nil
}

// ShowDiagnosticsInFolder opens the folder of a zip this session just wrote.
func (a *App) ShowDiagnosticsInFolder(path string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	if !a.knownDiagnostics(path) {
		return fmt.Errorf("unknown diagnostics archive")
	}
	dir := filepath.Dir(filepath.Clean(path))
	if dir == "" || dir == "." {
		return fmt.Errorf("unknown diagnostics folder")
	}
	if err := openFolder(dir); err != nil {
		return fmt.Errorf("open diagnostics folder: %w", err)
	}
	return nil
}

func (a *App) collectDiagnosticsFromTray() {
	if a == nil || !a.featureOn(features.Diagnostics) || a.ctx == nil {
		return
	}
	wailsruntime.WindowShow(a.ctx)
	result, err := a.SaveDiagnostics(false, "")
	if err != nil {
		slog.Error("diagnostics save failed", "err", err)
		return
	}
	if result.Cancelled || result.Path == "" {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventDiagnosticsSaved, result)
}

func (a *App) diagnosticsBundle(includeHosts bool, theme string) (diagnostics.Bundle, error) {
	cfg, err := a.storage.Load()
	if err != nil {
		return diagnostics.Bundle{}, err
	}
	tunnels, err := a.tunnel.List()
	if err != nil {
		return diagnostics.Bundle{}, err
	}
	logPath := diagnostics.LogPath(a.storage.Path())
	logData, err := diagnostics.ReadLogTail(logPath)
	if err != nil {
		slog.Warn("diagnostics log was not readable")
		logData = nil
	}
	var extra []string
	if token, err := ipc.ReadToken(ipc.TokenPath(a.storage.Path())); err == nil && token != "" {
		extra = []string{token}
	}
	statsPath := tunnelstats.PathBeside(a.storage.Path())
	statsPresent := false
	var totals []diagnostics.StatTotals
	if info, err := os.Stat(statsPath); err == nil && info.Mode().IsRegular() {
		statsPresent = true
		totals = statTotals(a.tunnel.Stats())
	}
	views := cfg.Features.Views()
	flags := make([]diagnostics.FlagState, 0, len(views))
	for _, view := range views {
		flags = append(flags, diagnostics.FlagState{ID: view.ID, Enabled: view.Enabled})
	}
	return diagnostics.Build(diagnostics.Input{
		Now:          time.Now(),
		IncludeHosts: includeHosts,
		System: diagnostics.SystemInfo{
			Version:        appVersion(),
			Commit:         diagnostics.Commit(),
			OS:             runtime.GOOS,
			Arch:           runtime.GOARCH,
			OSVersion:      diagnostics.OSVersion(),
			GoVersion:      diagnostics.GoVersion(),
			Locale:         uilocale.Effective(cfg.Language),
			Theme:          theme,
			WailsVersion:   diagnostics.ModuleVersion("github.com/wailsapp/wails/v2"),
			WebViewVersion: diagnostics.WebViewVersion(),
			Flags:          flags,
		},
		Config:       cfg,
		Log:          logData,
		Tunnels:      tunnelStates(tunnels),
		Stats:        totals,
		StatsPresent: statsPresent,
		Secrets:      extra,
	})
}

func writeDiagnosticsZip(dest string, bundle diagnostics.Bundle) error {
	dir := filepath.Dir(dest)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, conf.PrivateDirPerm); err != nil {
			return fmt.Errorf("create diagnostics folder: %w", err)
		}
	}
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, conf.PrivateFilePerm)
	if err != nil {
		return fmt.Errorf("write diagnostics: %w", err)
	}
	writeErr := diagnostics.WriteZip(file, bundle.Files, time.Now())
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(dest)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return closeErr
	}
	return nil
}

func tunnelStates(tunnels []model.Tunnel) []diagnostics.TunnelState {
	out := make([]diagnostics.TunnelState, 0, len(tunnels))
	for _, tunnel := range tunnels {
		out = append(out, diagnostics.TunnelState{
			ID:         tunnel.ID,
			Name:       tunnel.Name,
			Mode:       tunnel.Mode,
			Status:     tunnel.Status,
			LocalPort:  tunnel.LocalPort,
			RemotePort: tunnel.RemotePort,
			AutoStart:  tunnel.AutoStart,
			LastError:  tunnel.LastError,
		})
	}
	return out
}

func statTotals(views []tunnelstats.View) []diagnostics.StatTotals {
	out := make([]diagnostics.StatTotals, 0, len(views))
	for _, view := range views {
		out = append(out, diagnostics.StatTotals{
			ID:                view.ID,
			TodayConnectedSec: view.TodayConnectedSec,
			TotalConnectedSec: view.TotalConnectedSec,
			ReconnectsToday:   view.ReconnectsToday,
			ReconnectsTotal:   view.ReconnectsTotal,
			TotalBytesUp:      view.TotalBytesUp,
			TotalBytesDown:    view.TotalBytesDown,
			LastError:         view.LastError,
		})
	}
	return out
}

func (a *App) rememberDiagnostics(path string) {
	cleaned := filepath.Clean(path)
	a.diagMu.Lock()
	defer a.diagMu.Unlock()
	if a.diagPaths == nil {
		a.diagPaths = map[string]struct{}{}
	}
	a.diagPaths[path] = struct{}{}
	a.diagPaths[cleaned] = struct{}{}
	if len(a.diagPaths) <= 16 {
		return
	}
	for key := range a.diagPaths {
		if key == path || key == cleaned {
			continue
		}
		delete(a.diagPaths, key)
		if len(a.diagPaths) <= 16 {
			return
		}
	}
}

func (a *App) knownDiagnostics(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	a.diagMu.Lock()
	defer a.diagMu.Unlock()
	if _, ok := a.diagPaths[path]; ok {
		return true
	}
	_, ok := a.diagPaths[filepath.Clean(path)]
	return ok
}
