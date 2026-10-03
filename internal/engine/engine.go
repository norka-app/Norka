// Package engine is the headless core of Norka: config, keychain, tunnels,
// stats, notifications, wake and network recovery, and automation IPC.
// It does not import Wails or the system tray. The GUI and a future
// background daemon both host an Engine and supply a Sink for UI events.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/norka-app/Norka/internal/autostart"
	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/notify"
	"github.com/norka-app/Norka/internal/secrets"
	"github.com/norka-app/Norka/internal/tunnelstats"
	"github.com/norka-app/Norka/internal/uilocale"
)

// Engine owns the services the GUI used to wire up in package main.
type Engine struct {
	initErr error

	storage *conf.Storage
	vault   *secrets.Vault
	jumper  *biz.JumperBiz
	group   *biz.GroupBiz
	profile *biz.ProfileBiz
	tunnel  *biz.TunnelBiz
	runtime Runtime

	sink   Sink
	locale func() string
	icon   func() string

	watch      func(context.Context) (<-chan netwatch.Event, error)
	listen     func(address string) (net.Listener, error)
	syncLogin  func(autoRun, hidden bool) error
	syncScheme func(enabled bool) error
	poster     func(notify.PosterConfig) notify.Poster

	notifyMu       sync.Mutex
	notifySettings notify.Settings
	notifier       *notify.Service

	wakeMu     sync.Mutex
	wakeCancel context.CancelFunc

	ipcMu     sync.Mutex
	ipcCancel context.CancelFunc

	automationMu     sync.Mutex
	automationPrompt AutomationPrompt

	ownerMu         sync.Mutex
	version         string
	owner           *Owner
	hostWithoutLock bool
	// runner replaces local start/stop for automation when the window is an IPC client.
	// Nil keeps the in-process runtime.
	runner func(op string, id int) (model.Tunnel, error)

	subMu sync.Mutex
	subs  []*sub

	stopOnce sync.Once
	stopCh   chan struct{}
}

// Open loads the default config, the OS keychain, and the tunnel services.
// A storage error is kept on the engine; Ready reports it.
func Open() *Engine {
	storage, err := conf.NewDefaultStorage()
	if err != nil {
		return &Engine{initErr: err}
	}
	level := detectLogLevel()
	if err := configureLogger(storage.Path(), level); err != nil {
		fmt.Printf("logger init failed: %v\n", err)
	}
	slog.Info("app initialized", "config", storage.Path())
	return New(Options{Storage: storage, UseSystemKeychain: true})
}

// New builds an engine around an existing config. Tests use it to inject fakes.
func New(opts Options) *Engine {
	if opts.Storage == nil {
		return &Engine{initErr: fmt.Errorf("app is not initialized")}
	}
	vault := opts.Vault
	if vault == nil && opts.UseSystemKeychain {
		vault = secrets.OpenSystem()
	}
	if vault == nil {
		vault = secrets.NewVault(secrets.NewMemoryKeyring(), false)
	}
	if _, err := vault.MigrateStorage(opts.Storage); err != nil {
		slog.Error("jumper secret migration failed", "error", err)
	}
	jumper := biz.NewJumperBiz(opts.Storage)
	jumper.SetSecrets(vault)
	tunnel := biz.NewTunnelBiz(opts.Storage)
	tunnel.SetSecrets(vault)
	tunnel.SetStats(tunnelstats.Open(tunnelstats.PathBeside(opts.Storage.Path())))

	var runtime Runtime = tunnel
	if opts.Runtime != nil {
		runtime = opts.Runtime
	}
	eng := &Engine{
		storage:    opts.Storage,
		vault:      vault,
		jumper:     jumper,
		group:      biz.NewGroupBiz(opts.Storage),
		profile:    biz.NewProfileBiz(opts.Storage),
		tunnel:     tunnel,
		runtime:    runtime,
		sink:       opts.Sink,
		locale:     opts.Locale,
		icon:       opts.NotifyIcon,
		watch:      opts.Watch,
		listen:     opts.Listen,
		syncLogin:  opts.SyncLogin,
		syncScheme: opts.SyncScheme,
		poster:     opts.Poster,
		stopCh:     make(chan struct{}),
	}
	eng.bindTunnel()
	return eng
}

// SetHost installs the GUI callbacks. NewApp calls it after the App exists
// so the sink can close over that value.
func (e *Engine) SetHost(h Host) {
	if e == nil {
		return
	}
	e.sink = h.Sink
	e.locale = h.Locale
	e.icon = h.NotifyIcon
}

// Ready reports whether storage and the biz services were constructed.
func (e *Engine) Ready() error {
	if e == nil {
		return fmt.Errorf("app is not initialized")
	}
	if e.initErr != nil {
		return e.initErr
	}
	if e.storage == nil || e.jumper == nil || e.group == nil || e.profile == nil || e.tunnel == nil {
		return fmt.Errorf("app is not initialized")
	}
	return nil
}

func (e *Engine) Storage() *conf.Storage {
	if e == nil {
		return nil
	}
	return e.storage
}

func (e *Engine) Tunnel() *biz.TunnelBiz {
	if e == nil {
		return nil
	}
	return e.tunnel
}

func (e *Engine) Jumper() *biz.JumperBiz {
	if e == nil {
		return nil
	}
	return e.jumper
}

func (e *Engine) Group() *biz.GroupBiz {
	if e == nil {
		return nil
	}
	return e.group
}

func (e *Engine) Profile() *biz.ProfileBiz {
	if e == nil {
		return nil
	}
	return e.profile
}

func (e *Engine) Vault() *secrets.Vault {
	if e == nil {
		return nil
	}
	return e.vault
}

// TunnelStartLimit is unlimited. Tunnels are stored and started locally.
func (e *Engine) TunnelStartLimit() int {
	return 0
}

// FeatureOn reports the effective flag, or the catalog default when config
// cannot be read.
func (e *Engine) FeatureOn(id features.ID) bool {
	if e == nil || e.storage == nil {
		return features.Default(id)
	}
	cfg, err := e.storage.Load()
	if err != nil {
		return features.Default(id)
	}
	return cfg.Features.Enabled(id)
}

// Start brings up notifications, launch-at-login, and, when this process
// hosts the engine, the wake watcher and the automation listener.
// It does not start tunnels: the GUI handles a deep link first, then calls
// StartAutoStart, matching the previous startup order.
func (e *Engine) Start() {
	if e == nil || e.Ready() != nil {
		return
	}
	e.initNotifier()
	e.SyncAutoRun()
	if cfg, err := e.storage.Load(); err == nil {
		e.SyncWakeWatch(cfg.Features.Enabled(features.WakeReconnect))
	}
	e.SyncAutomation()
}

// SetAutoStartSkip installs a filter on the concrete tunnel service.
// Jumpers are passed as stored, before the keychain is read. A non-empty
// reason skips that tunnel. norkad uses this so a password the GUI would
// prompt for is logged and not dialed. The GUI leaves the filter unset.
func (e *Engine) SetAutoStartSkip(skip func(model.Tunnel, []model.Jumper) string) {
	if e == nil || e.tunnel == nil {
		return
	}
	e.tunnel.SetAutoStartSkip(skip)
}

// StartAutoStart launches tunnels marked autoStart. The call returns before
// the tunnels finish dialing, as startup did.
// A process that does not host the engine leaves them to the owner.
func (e *Engine) StartAutoStart() {
	if e == nil || !e.hosting() {
		return
	}
	rt := e.active()
	if rt == nil {
		return
	}
	limit := e.TunnelStartLimit()
	go func() {
		if err := rt.StartAutoStart(limit); err != nil {
			slog.Error("auto start tunnel failed", "err", err)
		}
	}()
}

// Shutdown stops the automation listener and then the tunnels, and drops
// engine.lock when this process holds it.
// The wake watcher is left to process exit, as the GUI shutdown did.
func (e *Engine) Shutdown() {
	if e == nil {
		return
	}
	e.StopAutomation()
	e.ShutdownTunnels()
	e.Release()
}

// StopAutomation closes the automation listener.
func (e *Engine) StopAutomation() {
	if e == nil {
		return
	}
	e.stopAutomationIPC()
}

// ShutdownTunnels stops running forwards and flushes tunnel stats.
// A process that does not host the engine leaves them running.
func (e *Engine) ShutdownTunnels() {
	if e == nil || !e.hosting() {
		return
	}
	if rt := e.active(); rt != nil {
		rt.Shutdown()
	}
}

func (e *Engine) active() Runtime {
	if e == nil {
		return nil
	}
	return e.runtime
}

// RebindAfterImport rebuilds jumper, group, and tunnel services after the
// config file is replaced. Stats are not reattached: import used to drop them
// with the previous TunnelBiz. The running notifier is kept.
func (e *Engine) RebindAfterImport() {
	if e == nil || e.storage == nil {
		return
	}
	old := e.tunnel
	e.jumper = biz.NewJumperBiz(e.storage)
	e.jumper.SetSecrets(e.vault)
	e.group = biz.NewGroupBiz(e.storage)
	e.tunnel = biz.NewTunnelBiz(e.storage)
	e.tunnel.SetSecrets(e.vault)
	if e.notifier != nil {
		e.tunnel.SetEvents(e.notifier)
	}
	if sameRuntime(e.runtime, old) {
		e.runtime = e.tunnel
	}
	e.bindTunnel()
}

func sameRuntime(rt Runtime, tunnel *biz.TunnelBiz) bool {
	concrete, ok := rt.(*biz.TunnelBiz)
	return ok && concrete == tunnel
}

// ReloadNotifySettings reads the notification toggles from config.
func (e *Engine) ReloadNotifySettings() {
	if e == nil {
		return
	}
	e.loadNotifySettings()
}

func (e *Engine) initNotifier() {
	if e == nil || e.storage == nil || e.active() == nil {
		return
	}
	e.loadNotifySettings()
	iconPath := ""
	if e.icon != nil {
		iconPath = e.icon()
	}
	posterCfg := notify.PosterConfig{
		AppID:    "Norka",
		IconPath: iconPath,
		OnClick: func(id int) {
			e.FocusTunnel(id)
		},
	}
	var poster notify.Poster
	if e.poster != nil {
		poster = e.poster(posterCfg)
	} else {
		poster = notify.NewSystemPoster(posterCfg)
	}
	poster = publishingPoster{next: poster, emit: e.publishNotification}
	e.notifier = notify.NewService(notify.ServiceConfig{
		Window:   2 * time.Second,
		Settings: e.currentNotifySettings,
		Catalog: func() notify.Catalog {
			return notify.CatalogFor(e.localeTag())
		},
		Poster: poster,
	})
	e.active().SetEvents(e.notifier)
}

func (e *Engine) loadNotifySettings() {
	settings := notify.Settings{}
	if e != nil && e.storage != nil {
		if cfg, err := e.storage.Load(); err == nil {
			settings = notifySettingsFromConfig(cfg.Notifications)
			settings.Enabled = cfg.Features.Enabled(features.Notifications)
		}
	}
	if e == nil {
		return
	}
	e.notifyMu.Lock()
	e.notifySettings = settings
	e.notifyMu.Unlock()
}

func (e *Engine) currentNotifySettings() notify.Settings {
	if e == nil {
		return notify.Settings{}
	}
	e.notifyMu.Lock()
	defer e.notifyMu.Unlock()
	return e.notifySettings
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

// SyncAutoRun rewrites the login entry from auto_run and autostart_hidden.
// A matching entry is left untouched. Startup and a feature toggle ignore
// a rewrite error, matching the previous GUI.
func (e *Engine) SyncAutoRun() {
	if e == nil || e.storage == nil {
		return
	}
	cfg, err := e.storage.Load()
	if err != nil {
		return
	}
	_ = e.syncLoginEntry(cfg.AutoRun, cfg.Features.Enabled(features.AutostartHidden))
}

// SetAutoRunEnabled stores launch-at-login and rewrites the OS entry.
// The hidden flag decides whether that entry starts in the tray.
func (e *Engine) SetAutoRunEnabled(enabled bool) error {
	if err := e.Ready(); err != nil {
		return err
	}
	cfg, err := e.storage.Update(func(cfg *conf.Config) error {
		cfg.AutoRun = enabled
		return nil
	})
	if err != nil {
		return err
	}
	return e.syncLoginEntry(cfg.AutoRun, cfg.Features.Enabled(features.AutostartHidden))
}

func (e *Engine) syncLoginEntry(autoRun, hidden bool) error {
	if e != nil && e.syncLogin != nil {
		return e.syncLogin(autoRun, hidden)
	}
	return autostart.Sync(autoRun, hidden)
}

func (e *Engine) localeTag() string {
	if e != nil && e.locale != nil {
		if tag := strings.TrimSpace(e.locale()); tag != "" {
			return tag
		}
	}
	return uilocale.DetectFromEnv()
}

// FocusTunnel asks the host to show the window and select the tunnel.
// id 0 only focuses the app.
func (e *Engine) FocusTunnel(id int) {
	if e == nil || e.sink == nil {
		return
	}
	e.sink.ShowWindow()
	e.sink.Emit(EventNotificationFocus, id)
}

func (e *Engine) emit(event string, payload any) {
	if e == nil || e.sink == nil || event == "" {
		return
	}
	e.sink.Emit(event, payload)
}

func (e *Engine) showWindow() {
	if e == nil || e.sink == nil {
		return
	}
	e.sink.ShowWindow()
}

func (e *Engine) tunnelsChanged() {
	if e == nil || e.sink == nil {
		return
	}
	e.sink.TunnelsChanged()
}

func detectLogLevel() slog.Level {
	if raw := strings.TrimSpace(os.Getenv("NORKA_LOG_LEVEL")); raw != "" {
		return parseLogLevel(raw)
	}
	if strings.TrimSpace(os.Getenv("devserver")) != "" {
		return slog.LevelDebug
	}
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
