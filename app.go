package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"norka/internal/aidebug"
	"norka/internal/autostart"
	"norka/internal/biz"
	"norka/internal/conf"
	"norka/internal/features"
	"norka/internal/model"
	"norka/internal/notify"
	"norka/internal/secrets"
	"norka/internal/sshconfig"
	"norka/internal/traytext"
	"norka/internal/uilocale"
	"norka/internal/update"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type OpenReportEmailPayload struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type OpenReportEmailResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// App struct
type App struct {
	ctx     context.Context
	storage *conf.Storage
	jumper  *biz.JumperBiz
	group   *biz.GroupBiz
	profile *biz.ProfileBiz
	tunnel  *biz.TunnelBiz
	aiDebug *aidebug.Service
	initErr error

	trayMu   sync.Mutex
	trayShow *systray.MenuItem
	trayQuit *systray.MenuItem
	uiLocale string

	allowClose atomic.Bool

	trafficMu       sync.Mutex
	lastTrafficUp   uint64
	lastTrafficDown uint64
	lastTrafficAt   time.Time

	window   windowModeState
	trayMenu trayMenu

	vault    *secrets.Vault
	notifier *notify.Service

	notifyMu       sync.Mutex
	notifySettings notify.Settings

	windowVisible          atomic.Bool
	quickSearchHotkey      quickSearchHotkey
	quickSearchOpen        atomic.Bool
	quickSearchRestoreHide atomic.Bool

	wakeMu     sync.Mutex
	wakeCancel context.CancelFunc
}

// SecretsStatus tells Settings whether jumper passwords live in the OS keychain.
type SecretsStatus struct {
	KeychainAvailable bool   `json:"keychainAvailable"`
	Mode              string `json:"mode"`
}

const eventNotificationFocus = "notification:focus"

var errExportCancelled = errors.New("export cancelled")

// NewApp creates a new App application struct
func NewApp() *App {
	storage, err := conf.NewDefaultStorage()
	if err != nil {
		return &App{initErr: err}
	}
	level := detectLogLevel()
	if err := configureLogger(storage.Path(), level); err != nil {
		fmt.Printf("logger init failed: %v\n", err)
	}
	slog.Info("app initialized", "config", storage.Path())

	vault := secrets.OpenSystem()
	if _, err := vault.MigrateStorage(storage); err != nil {
		slog.Error("jumper secret migration failed", "error", err)
	}
	jumper := biz.NewJumperBiz(storage)
	jumper.SetSecrets(vault)
	tunnel := biz.NewTunnelBiz(storage)
	tunnel.SetSecrets(vault)

	app := &App{
		storage: storage,
		jumper:  jumper,
		group:   biz.NewGroupBiz(storage),
		profile: biz.NewProfileBiz(storage),
		tunnel:  tunnel,
		aiDebug: aidebug.NewService("", ""),
		vault:   vault,
	}
	app.windowVisible.Store(true)
	return app
}

func detectLogLevel() slog.Level {
	// Explicit override takes highest priority.
	if raw := strings.TrimSpace(os.Getenv("NORKA_LOG_LEVEL")); raw != "" {
		return parseLogLevel(raw)
	}
	// `wails dev` injects `devserver`; use debug logging for dev runtime.
	if strings.TrimSpace(os.Getenv("devserver")) != "" {
		return slog.LevelDebug
	}
	// Built binary defaults to info.
	return slog.LevelInfo
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func configureLogger(configPath string, level slog.Level) error {
	dir := filepath.Dir(strings.TrimSpace(configPath))
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, conf.PrivateDirPerm); err != nil {
		return fmt.Errorf("create log dir failed: %w", err)
	}

	logPath := filepath.Join(dir, "norka.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, conf.PrivateFilePerm)
	if err != nil {
		return fmt.Errorf("open log file failed: %w", err)
	}

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
	slog.Info("logger initialized", "path", logPath, "level", level.String())
	return nil
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
// SetTrayMenuItems wires systray menu entries created in main so locale changes can relabel them.
func (a *App) SetTrayMenuItems(show, quit *systray.MenuItem) {
	a.trayMu.Lock()
	defer a.trayMu.Unlock()
	a.trayShow = show
	a.trayQuit = quit
}

// LanguageSetting is the persisted preference and the locale the UI should show.
type LanguageSetting struct {
	Preference string `json:"preference"`
	Locale     string `json:"locale"`
}

func (a *App) languageSetting(preference string) LanguageSetting {
	pref := uilocale.NormalizePreference(preference)
	return LanguageSetting{Preference: pref, Locale: uilocale.Effective(pref)}
}

// ResolvedUILocale reads the language preference from config.toml.
// A missing file or an empty preference follows the system locale.
func (a *App) ResolvedUILocale() string {
	if a == nil || a.storage == nil {
		return uilocale.DetectFromEnv()
	}
	data, err := os.ReadFile(a.storage.Path())
	if err != nil {
		return uilocale.DetectFromEnv()
	}
	cfg, err := conf.ParseConfigTOML(data)
	if err != nil || cfg == nil {
		return uilocale.DetectFromEnv()
	}
	return uilocale.Effective(cfg.Language)
}

func (a *App) useUILocale(tag string) {
	a.trayMu.Lock()
	a.uiLocale = uilocale.Normalize(tag)
	a.trayMu.Unlock()
}

func (a *App) uiLocaleTag() string {
	a.trayMu.Lock()
	defer a.trayMu.Unlock()
	if a.uiLocale == "" {
		return uilocale.DetectFromEnv()
	}
	return a.uiLocale
}

func (a *App) uiText() traytext.Strings {
	return traytext.ForLocale(a.uiLocaleTag())
}

// GetLanguage returns the saved preference and the locale the interface should use.
func (a *App) GetLanguage() (LanguageSetting, error) {
	if err := a.ensureReady(); err != nil {
		return a.languageSetting(""), err
	}
	cfg, err := a.storage.Load()
	if err != nil {
		return a.languageSetting(""), err
	}
	return a.languageSetting(cfg.Language), nil
}

// SetLanguage stores auto, ru, or en in config.toml, mirrors it to ui.locale, and refreshes the tray.
func (a *App) SetLanguage(preference string) (LanguageSetting, error) {
	if err := a.ensureReady(); err != nil {
		return LanguageSetting{}, err
	}
	setting := a.languageSetting(preference)
	if _, err := a.storage.Update(func(cfg *conf.Config) error {
		cfg.Language = setting.Preference
		return nil
	}); err != nil {
		return LanguageSetting{}, err
	}
	if err := uilocale.WriteFile(filepath.Dir(a.storage.Path()), setting.Preference); err != nil {
		return LanguageSetting{}, err
	}
	a.ApplyTrayLocale(setting.Locale)
	return setting, nil
}

func (a *App) applyTrayLocaleUnlocked(tag string) {
	s := traytext.ForLocale(tag)
	if a.trayShow != nil {
		a.trayShow.SetTitle(s.ShowMainTitle)
		a.trayShow.SetTooltip(s.ShowMainTooltip)
	}
	if a.trayQuit != nil {
		a.trayQuit.SetTitle(s.QuitTitle)
		a.trayQuit.SetTooltip(s.QuitTooltip)
	}
	if runtime.GOOS != "darwin" {
		systray.SetTitle(s.AppTitle)
	}
	systray.SetTooltip(s.IconTooltip)
	a.invalidateTrayMenu()
}

// ApplyTrayLocale updates tray icon tooltip and menu item titles to match a vue-i18n locale tag.
func (a *App) ApplyTrayLocale(locale string) {
	a.trayMu.Lock()
	defer a.trayMu.Unlock()
	tag := uilocale.Normalize(locale)
	a.uiLocale = tag
	a.applyTrayLocaleUnlocked(tag)
}

// SaveUILocale persists the language preference and refreshes the tray.
// locale may be auto, ru, or en (or a tag such as ru-RU).
func (a *App) SaveUILocale(locale string) error {
	_, err := a.SetLanguage(locale)
	return err
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	slog.Info("app startup")
	update.CleanupBackup()
	a.windowVisible.Store(true)
	a.bindQuickSearchHotkey()
	if err := a.ensureReady(); err == nil {
		a.initNotifier()
		a.syncAutoRunWithConfig()
		if cfg, err := a.storage.Load(); err == nil {
			a.syncWakeWatch(cfg.Features.Enabled(features.WakeReconnect))
		}
		if id := notify.ParseFocusArg(os.Args); id > 0 {
			a.FocusTunnel(id)
		}
		go func() {
			if err := a.tunnel.StartAutoStart(0); err != nil {
				slog.Error("auto start tunnel failed", "err", err)
			}
		}()
	}
}

func (a *App) initNotifier() {
	if a.storage == nil || a.tunnel == nil {
		return
	}
	a.loadNotifySettings()
	poster := notify.NewSystemPoster(notify.PosterConfig{
		AppID:    "Norka",
		IconPath: a.writeNotifyIcon(),
		OnClick: func(id int) {
			a.FocusTunnel(id)
		},
	})
	a.notifier = notify.NewService(notify.ServiceConfig{
		Window:   2 * time.Second,
		Settings: a.currentNotifySettings,
		Catalog: func() notify.Catalog {
			return notify.CatalogFor(a.uiLocaleTag())
		},
		Poster: poster,
	})
	a.tunnel.SetEvents(a.notifier)
}

func (a *App) writeNotifyIcon() string {
	if a.storage == nil {
		return ""
	}
	name := "norka-notify.png"
	data := trayIconFallback
	if runtime.GOOS == "windows" && len(trayIconWindows) > 0 {
		name = "norka-notify.ico"
		data = trayIconWindows
	}
	if len(data) == 0 {
		return ""
	}
	path := filepath.Join(filepath.Dir(a.storage.Path()), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		slog.Warn("notification icon was not written", "error", err)
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// FocusTunnel brings the window forward and asks the UI to show that tunnel.
// id 0 only focuses the app.
func (a *App) FocusTunnel(id int) {
	if a == nil || a.ctx == nil {
		return
	}
	a.showMainWindow()
	wailsruntime.EventsEmit(a.ctx, eventNotificationFocus, id)
}

func (a *App) loadNotifySettings() {
	settings := notify.Settings{}
	if a.storage != nil {
		if cfg, err := a.storage.Load(); err == nil {
			settings = notifySettingsFromConfig(cfg.Notifications)
			settings.Enabled = cfg.Features.Enabled(features.Notifications)
		}
	}
	a.notifyMu.Lock()
	a.notifySettings = settings
	a.notifyMu.Unlock()
}

func (a *App) currentNotifySettings() notify.Settings {
	a.notifyMu.Lock()
	defer a.notifyMu.Unlock()
	return a.notifySettings
}

func notifySettingsFromConfig(cfg conf.NotificationSettings) notify.Settings {
	return notify.Settings{
		Enabled:       cfg.Enabled,
		Dropped:       cfg.Dropped,
		Reconnected:   cfg.Reconnected,
		GaveUp:        cfg.GaveUp,
		ConnectFailed: cfg.ConnectFailed,
		Connected:     cfg.Connected,
	}
}

// GetNotificationSettings returns the opt-in OS notification toggles.
func (a *App) GetNotificationSettings() (conf.NotificationSettings, error) {
	if err := a.ensureReady(); err != nil {
		return conf.NotificationSettings{}, err
	}
	cfg, err := a.storage.Load()
	if err != nil {
		return conf.NotificationSettings{}, err
	}
	settings := cfg.Notifications
	settings.Enabled = cfg.Features.Enabled(features.Notifications)
	return settings, nil
}

// SetNotificationSettings stores the notification toggles.
func (a *App) SetNotificationSettings(settings conf.NotificationSettings) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	cfg, err := a.storage.Update(func(cfg *conf.Config) error {
		cfg.Notifications = settings
		cfg.NotificationsSet = true
		return cfg.Features.Set(features.Notifications, settings.Enabled)
	})
	if err != nil {
		return err
	}
	a.loadNotifySettings()
	a.publishFeatures(cfg.Features)
	return nil
}

// GetSecretsStatus reports whether the OS keychain is used for jumper secrets.
func (a *App) GetSecretsStatus() (SecretsStatus, error) {
	if err := a.ensureReady(); err != nil {
		return SecretsStatus{}, err
	}
	if a.vault != nil && a.vault.Available() {
		return SecretsStatus{KeychainAvailable: true, Mode: "keychain"}, nil
	}
	return SecretsStatus{KeychainAvailable: false, Mode: "config"}, nil
}

func (a *App) PrepareForQuit() {
	a.allowClose.Store(true)
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return false
	}
	if a.allowClose.Load() {
		return false
	}
	slog.Info("window close intercepted; hiding to tray")
	a.hideMainWindow()
	return true
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	slog.Info("app shutdown")
	a.unbindQuickSearchHotkey()
	if a.tunnel != nil {
		a.tunnel.Shutdown()
	}
}

func (a *App) ensureReady() error {
	if a.initErr != nil {
		return a.initErr
	}
	if a.storage == nil || a.jumper == nil || a.group == nil || a.profile == nil || a.tunnel == nil {
		return fmt.Errorf("app is not initialized")
	}
	return nil
}

func (a *App) GetState() (model.State, error) {
	if err := a.ensureReady(); err != nil {
		return model.State{}, err
	}
	jumpers, err := a.jumper.List()
	if err != nil {
		return model.State{}, err
	}
	tunnels, err := a.tunnel.List()
	if err != nil {
		return model.State{}, err
	}
	groups, err := a.group.List()
	if err != nil {
		return model.State{}, err
	}
	profiles, activeID, stopOthers, err := a.profileSnapshot()
	if err != nil {
		return model.State{}, err
	}
	return model.State{
		Jumpers:           append([]model.Jumper{}, jumpers...),
		Groups:            append([]model.TunnelGroup{}, groups...),
		Tunnels:           append([]model.Tunnel{}, tunnels...),
		Profiles:          profiles,
		ActiveProfileID:   activeID,
		ProfileStopOthers: stopOthers,
	}, nil
}

func (a *App) GetTrafficStats() (model.TrafficStats, error) {
	if err := a.ensureReady(); err != nil {
		return model.TrafficStats{}, err
	}

	if !a.featureOn(features.TrafficMonitor) {
		a.trafficMu.Lock()
		a.lastTrafficUp = 0
		a.lastTrafficDown = 0
		a.lastTrafficAt = time.Time{}
		a.trafficMu.Unlock()
		return model.TrafficStats{}, nil
	}

	up, down := a.tunnel.TrafficSnapshot()
	now := time.Now()

	a.trafficMu.Lock()
	defer a.trafficMu.Unlock()

	var upBps, downBps int64
	if !a.lastTrafficAt.IsZero() {
		elapsed := now.Sub(a.lastTrafficAt).Seconds()
		if elapsed > 0 {
			upBps = int64(float64(up-a.lastTrafficUp) / elapsed)
			downBps = int64(float64(down-a.lastTrafficDown) / elapsed)
			if upBps < 0 {
				upBps = 0
			}
			if downBps < 0 {
				downBps = 0
			}
		}
	}

	a.lastTrafficUp = up
	a.lastTrafficDown = down
	a.lastTrafficAt = now

	return model.TrafficStats{
		UpBps:   upBps,
		DownBps: downBps,
	}, nil
}

func (a *App) ListJumpers() ([]model.Jumper, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.jumper.List()
}

func (a *App) GetSSHConfigImportSources() ([]model.SSHConfigImportSource, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return sshconfig.GetImportSources()
}

func (a *App) LoadSSHConfigJumpersByPath(configPath string) (model.SSHConfigImportResult, error) {
	if err := a.ensureReady(); err != nil {
		return model.SSHConfigImportResult{}, err
	}
	return sshconfig.LoadImportCandidates(configPath)
}

func (a *App) CreateJumper(payload model.JumperPayload) (model.Jumper, error) {
	if err := a.ensureReady(); err != nil {
		return model.Jumper{}, err
	}
	return a.jumper.Create(payload)
}

func (a *App) UpdateJumper(id int, payload model.JumperPayload) (model.Jumper, error) {
	if err := a.ensureReady(); err != nil {
		return model.Jumper{}, err
	}
	return a.jumper.Update(id, payload)
}

func (a *App) TestJumperConnection(payload model.JumperPayload) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	return a.jumper.TestConnection(payload)
}

func (a *App) DeleteJumper(id int) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	return a.jumper.Delete(id)
}

func (a *App) ListTunnels() ([]model.Tunnel, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.tunnel.List()
}

func (a *App) ListGroups() ([]model.TunnelGroup, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	return a.group.List()
}

func (a *App) CreateGroup(payload model.TunnelGroupPayload) (model.TunnelGroup, error) {
	if err := a.ensureReady(); err != nil {
		return model.TunnelGroup{}, err
	}
	return a.group.Create(payload)
}

func (a *App) UpdateGroup(id int, payload model.TunnelGroupPayload) (model.TunnelGroup, error) {
	if err := a.ensureReady(); err != nil {
		return model.TunnelGroup{}, err
	}
	return a.group.Update(id, payload)
}

func (a *App) DeleteGroup(id int) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	return a.group.Delete(id)
}

func (a *App) ReorderGroups(ids []int) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	return a.group.Reorder(ids)
}

func (a *App) CreateTunnel(payload model.TunnelPayload) (model.Tunnel, error) {
	if err := a.ensureReady(); err != nil {
		return model.Tunnel{}, err
	}
	return a.tunnel.Create(payload)
}

func (a *App) UpdateTunnel(id int, payload model.TunnelPayload) (model.Tunnel, error) {
	if err := a.ensureReady(); err != nil {
		return model.Tunnel{}, err
	}
	return a.tunnel.Update(id, payload)
}

func (a *App) MoveTunnelToGroup(id int, groupID int) (model.Tunnel, error) {
	if err := a.ensureReady(); err != nil {
		return model.Tunnel{}, err
	}
	return a.tunnel.MoveToGroup(id, groupID)
}

func (a *App) TestTunnelConnection(payload model.TunnelPayload, inlineJumper *model.JumperPayload) (model.TunnelConnectionTestResult, error) {
	if err := a.ensureReady(); err != nil {
		return model.TunnelConnectionTestResult{}, err
	}
	latency, err := a.tunnel.TestConnection(payload, inlineJumper)
	if err != nil {
		return model.TunnelConnectionTestResult{}, err
	}
	return model.TunnelConnectionTestResult{
		LatencyMs: latency.Milliseconds(),
	}, nil
}

func (a *App) DebugJumperFailure(payload model.JumperPayload, rawError string, uiLocale string) (model.AIDebugResult, error) {
	if err := a.ensureReady(); err != nil {
		return model.AIDebugResult{}, err
	}
	if a.aiDebug == nil {
		return model.AIDebugResult{}, fmt.Errorf("ai debug service is not initialized")
	}

	jumper := model.Jumper{
		Name:                   strings.TrimSpace(payload.Name),
		Host:                   strings.TrimSpace(payload.Host),
		Port:                   payload.Port,
		User:                   strings.TrimSpace(payload.User),
		AuthType:               strings.TrimSpace(payload.AuthType),
		KeyPath:                strings.TrimSpace(payload.KeyPath),
		AgentSocketPath:        strings.TrimSpace(payload.AgentSocketPath),
		BypassHostVerification: payload.BypassHostVerification,
		KeepAliveIntervalMs:    payload.KeepAliveIntervalMs,
		TimeoutMs:              payload.TimeoutMs,
		HostKeyAlgorithms:      strings.TrimSpace(payload.HostKeyAlgorithms),
		Notes:                  strings.TrimSpace(payload.Notes),
	}

	return a.aiDebug.Diagnose(context.Background(), aidebug.DiagnosticInput{
		TargetType:  "jumper_test",
		RawError:    rawError,
		UILocale:    uiLocale,
		JumperChain: []model.Jumper{jumper},
	})
}

func (a *App) DebugTunnelFailure(payload model.TunnelPayload, inlineJumper *model.JumperPayload, rawError string, uiLocale string) (model.AIDebugResult, error) {
	if err := a.ensureReady(); err != nil {
		return model.AIDebugResult{}, err
	}
	if a.aiDebug == nil {
		return model.AIDebugResult{}, fmt.Errorf("ai debug service is not initialized")
	}

	chain := make([]model.Jumper, 0, len(payload.JumperIDs)+1)
	if len(payload.JumperIDs) > 0 {
		cfg, err := a.storage.Load()
		if err != nil {
			return model.AIDebugResult{}, err
		}
		jumpers, err := collectJumpersForApp(cfg.Jumpers, payload.JumperIDs)
		if err != nil {
			return model.AIDebugResult{}, err
		}
		chain = append(chain, jumpers...)
	}
	if inlineJumper != nil {
		chain = append(chain, model.Jumper{
			Name:                   strings.TrimSpace(inlineJumper.Name),
			Host:                   strings.TrimSpace(inlineJumper.Host),
			Port:                   inlineJumper.Port,
			User:                   strings.TrimSpace(inlineJumper.User),
			AuthType:               strings.TrimSpace(inlineJumper.AuthType),
			KeyPath:                strings.TrimSpace(inlineJumper.KeyPath),
			AgentSocketPath:        strings.TrimSpace(inlineJumper.AgentSocketPath),
			BypassHostVerification: inlineJumper.BypassHostVerification,
			KeepAliveIntervalMs:    inlineJumper.KeepAliveIntervalMs,
			TimeoutMs:              inlineJumper.TimeoutMs,
			HostKeyAlgorithms:      strings.TrimSpace(inlineJumper.HostKeyAlgorithms),
			Notes:                  strings.TrimSpace(inlineJumper.Notes),
		})
	}

	tunnel := model.Tunnel{
		Name:        strings.TrimSpace(payload.Name),
		Mode:        strings.TrimSpace(payload.Mode),
		JumperIDs:   append([]int{}, payload.JumperIDs...),
		LocalHost:   strings.TrimSpace(payload.LocalHost),
		LocalPort:   payload.LocalPort,
		RemoteHost:  strings.TrimSpace(payload.RemoteHost),
		RemotePort:  payload.RemotePort,
		AutoStart:   payload.AutoStart,
		Status:      strings.TrimSpace(payload.Status),
		Description: strings.TrimSpace(payload.Description),
	}

	return a.aiDebug.Diagnose(context.Background(), aidebug.DiagnosticInput{
		TargetType:  "tunnel_test",
		RawError:    rawError,
		UILocale:    uiLocale,
		Tunnel:      &tunnel,
		JumperChain: chain,
	})
}

func (a *App) DebugSavedTunnelFailure(id int, rawError string, uiLocale string) (model.AIDebugResult, error) {
	if err := a.ensureReady(); err != nil {
		return model.AIDebugResult{}, err
	}
	if a.aiDebug == nil {
		return model.AIDebugResult{}, fmt.Errorf("ai debug service is not initialized")
	}
	if id <= 0 {
		return model.AIDebugResult{}, fmt.Errorf("invalid tunnel id")
	}

	cfg, err := a.storage.Load()
	if err != nil {
		return model.AIDebugResult{}, err
	}

	var tunnel model.Tunnel
	found := false
	for _, item := range cfg.Tunnels {
		if item.ID == id {
			tunnel = item
			found = true
			break
		}
	}
	if !found {
		return model.AIDebugResult{}, biz.ErrTunnelNotFound
	}

	jumpers, err := collectJumpersForApp(cfg.Jumpers, tunnel.JumperIDs)
	if err != nil {
		return model.AIDebugResult{}, err
	}

	if strings.TrimSpace(rawError) == "" {
		rawError = tunnel.LastError
	}

	return a.aiDebug.Diagnose(context.Background(), aidebug.DiagnosticInput{
		TargetType:  "tunnel_runtime_error",
		RawError:    rawError,
		UILocale:    uiLocale,
		Tunnel:      &tunnel,
		JumperChain: jumpers,
	})
}

func (a *App) DeleteTunnel(id int) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	return a.tunnel.Delete(id)
}

func (a *App) ToggleTunnel(id int) (model.Tunnel, error) {
	if err := a.ensureReady(); err != nil {
		return model.Tunnel{}, err
	}
	return a.tunnel.Toggle(id, a.tunnelStartLimit())
}

// tunnelStartLimit is unlimited. Tunnels are stored and started locally.
func (a *App) tunnelStartLimit() int {
	return 0
}

func collectJumpersForApp(items []model.Jumper, ids []int) ([]model.Jumper, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one jumper is required")
	}
	index := make(map[int]model.Jumper, len(items))
	for _, item := range items {
		index[item.ID] = item
	}
	result := make([]model.Jumper, 0, len(ids))
	for _, id := range ids {
		jumper, ok := index[id]
		if !ok {
			return nil, biz.ErrJumperNotFound
		}
		result = append(result, jumper)
	}
	return result, nil
}

// syncAutoRunWithConfig aligns OS auto-run (launch at login) state with config.
func (a *App) syncAutoRunWithConfig() {
	cfg, err := a.storage.Load()
	if err != nil {
		return
	}
	enabled, _ := autostart.IsEnabled()
	if cfg.AutoRun && !enabled {
		_ = autostart.Enable()
	} else if !cfg.AutoRun && enabled {
		_ = autostart.Disable()
	}
}

// GetAutoRunEnabled returns whether the app is currently set to launch at login (system state).
func (a *App) GetAutoRunEnabled() (bool, error) {
	if err := a.ensureReady(); err != nil {
		return false, err
	}
	return autostart.IsEnabled()
}

// SetAutoRunEnabled enables or disables launch at login and persists to config.
func (a *App) SetAutoRunEnabled(enabled bool) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	_, err := a.storage.Update(func(cfg *conf.Config) error {
		cfg.AutoRun = enabled
		return nil
	})
	if err != nil {
		return err
	}
	if enabled {
		return autostart.Enable()
	}
	return autostart.Disable()
}

func (a *App) GetTrafficMonitorEnabled() (bool, error) {
	if err := a.ensureReady(); err != nil {
		return true, err
	}
	cfg, err := a.storage.Load()
	if err != nil {
		return true, err
	}
	return cfg.Features.Enabled(features.TrafficMonitor), nil
}

func (a *App) SetTrafficMonitorEnabled(enabled bool) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	cfg, err := a.storage.Update(func(cfg *conf.Config) error {
		cfg.TrafficMonitorEnabled = enabled
		return cfg.Features.Set(features.TrafficMonitor, enabled)
	})
	if err != nil {
		return err
	}
	a.publishFeatures(cfg.Features)
	return nil
}

// GetConfigPath returns the absolute path of the current config file.
func (a *App) GetConfigPath() (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(a.storage.Path())
	if err != nil {
		return a.storage.Path(), nil
	}
	return abs, nil
}

// ExportConfig copies the current config.toml to destPath.
// ExportConfigWithDialog opens a save-file dialog, then copies the config.
// Returns empty string if the user cancelled.
func (a *App) ExportConfigWithDialog(includeSecrets bool) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	text := a.uiText()
	destPath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultFilename: "config.toml",
		Title:           text.ExportConfigTitle,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: text.TomlFilter, Pattern: "*.toml"},
		},
	})
	if err != nil {
		return fmt.Errorf("file dialog: %w", err)
	}
	if strings.TrimSpace(destPath) == "" {
		return errExportCancelled
	}
	return a.ExportConfig(destPath, includeSecrets)
}

func (a *App) ExportConfig(destPath string, includeSecrets bool) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	destPath = strings.TrimSpace(destPath)
	if destPath == "" {
		return fmt.Errorf("destination path is empty")
	}

	cfg, err := a.storage.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	data := conf.MarshalTOML(secrets.PrepareExport(cfg, a.vault, includeSecrets))

	if err := os.MkdirAll(filepath.Dir(destPath), conf.PrivateDirPerm); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	if err := os.WriteFile(destPath, data, conf.PrivateFilePerm); err != nil {
		return fmt.Errorf("write exported config: %w", err)
	}
	slog.Info("config exported", "dest", destPath, "include_secrets", includeSecrets)
	return nil
}

// SelectImportFile opens a file picker and returns the selected path. Returns empty string if cancelled.
func (a *App) SelectImportFile() (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}
	text := a.uiText()
	srcPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: text.ImportConfigTitle,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: text.TomlFilter, Pattern: "*.toml"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("file dialog: %w", err)
	}
	return strings.TrimSpace(srcPath), nil
}

// ImportConfig replaces the current config with the TOML file at srcPath.
// It validates the file first, then stops all running tunnels, replaces the
// config file, and restarts auto-start tunnels.
func (a *App) ImportConfig(srcPath string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return fmt.Errorf("source path is empty")
	}

	// Validate: can we parse it as a valid config?
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}
	if _, err := conf.ParseConfigTOML(data); err != nil {
		return fmt.Errorf("invalid config file: %w", err)
	}

	oldCfg, err := a.storage.Load()
	if err != nil {
		return err
	}
	oldRefs := secrets.SecretRefs(oldCfg.Jumpers)

	// Stop all running tunnels.
	if a.tunnel != nil {
		a.tunnel.Shutdown()
	}

	// Overwrite config file atomically.
	tmpPath := a.storage.Path() + ".import.tmp"
	if err := os.WriteFile(tmpPath, data, conf.PrivateFilePerm); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmpPath, a.storage.Path()); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}

	// Reinitialise biz layer so the new config takes effect.
	a.jumper = biz.NewJumperBiz(a.storage)
	a.jumper.SetSecrets(a.vault)
	a.group = biz.NewGroupBiz(a.storage)
	a.tunnel = biz.NewTunnelBiz(a.storage)
	a.tunnel.SetSecrets(a.vault)
	if a.notifier != nil {
		a.tunnel.SetEvents(a.notifier)
	}
	if a.vault != nil {
		if _, err := a.vault.MigrateStorage(a.storage); err != nil {
			slog.Error("imported jumper secrets were not moved to keychain", "error", err)
		}
	}
	if newCfg, err := a.storage.Load(); err == nil {
		a.forgetRemovedSecrets(oldRefs, secrets.SecretRefs(newCfg.Jumpers))
	}
	a.loadNotifySettings()
	a.bindQuickSearchHotkey()
	a.invalidateTrayMenu()
	if fresh, loadErr := a.storage.Load(); loadErr == nil {
		a.publishFeatures(fresh.Features)
	}

	// Restart auto-start tunnels.
	_ = a.tunnel.StartAutoStart(a.tunnelStartLimit())

	slog.Info("config imported", "src", srcPath)
	return nil
}

func (a *App) forgetRemovedSecrets(oldRefs, newRefs map[string]struct{}) {
	if a.vault == nil {
		return
	}
	for ref := range oldRefs {
		if _, ok := newRefs[ref]; ok {
			continue
		}
		a.vault.Forget(ref)
	}
}

// OpenConfigDir opens the config file's parent directory in the OS file manager.
// It supports macOS, Windows and Linux (via xdg-open).
func (a *App) OpenConfigDir() error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	dir := filepath.Dir(a.storage.Path())
	if err := openFolder(dir); err != nil {
		return fmt.Errorf("open config dir: %w", err)
	}
	return nil
}

// OpenReportEmail is kept for the desktop binding. Reports are not sent anywhere.
func (a *App) OpenReportEmail(payload OpenReportEmailPayload) OpenReportEmailResult {
	_ = payload
	return OpenReportEmailResult{Success: false, Error: "email reporting is not available"}
}
