package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/features"
)

func TestFeatureDefaultsAndMissingKeys(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Normalize()
	assertFeature(t, cfg, features.Profiles, false)
	assertFeature(t, cfg, features.QuickSearch, true)
	assertFeature(t, cfg, features.Notifications, false)
	assertFeature(t, cfg, features.AutoUpdate, true)
	assertFeature(t, cfg, features.TrafficMonitor, true)
	assertFeature(t, cfg, features.Mascot, true)
	assertFeature(t, cfg, features.SSHCommand, true)
	assertFeature(t, cfg, features.WakeReconnect, true)
	assertFeature(t, cfg, features.Automation, false)
	assertFeature(t, cfg, features.TunnelStats, true)
	assertFeature(t, cfg, features.Onboarding, true)
	if _, ok := cfg.Features.Explicit(features.Onboarding); ok {
		t.Fatal("onboarding key must stay missing on a fresh config")
	}
	assertFeature(t, cfg, features.Diagnostics, true)
	if _, ok := cfg.Features.Explicit(features.Diagnostics); ok {
		t.Fatal("diagnostics key must stay missing when the default applies")
	}
	assertFeature(t, cfg, features.TunnelDiagnostics, true)
	if _, ok := cfg.Features.Explicit(features.TunnelDiagnostics); ok {
		t.Fatal("tunnel diagnostics key must stay missing when the default applies")
	}
	assertFeature(t, cfg, features.AutostartHidden, true)
	if _, ok := cfg.Features.Explicit(features.AutostartHidden); ok {
		t.Fatal("autostart hidden key must stay missing when the default applies")
	}
	assertFeature(t, cfg, features.BackgroundMode, false)
	if _, ok := cfg.Features.Explicit(features.BackgroundMode); ok {
		t.Fatal("background mode key must stay missing when the default applies")
	}
	if _, ok := cfg.Features.Explicit(features.Profiles); ok {
		t.Fatal("profiles key must stay missing on a fresh config")
	}
	if _, ok := cfg.Features.Explicit(features.QuickSearch); ok {
		t.Fatal("quick search key must stay missing when the default applies")
	}

	loaded, err := ParseConfigTOML([]byte("version = 1\nauto_run = false\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertFeature(t, loaded, features.Profiles, false)
	assertFeature(t, loaded, features.QuickSearch, true)
	assertFeature(t, loaded, features.Notifications, false)
	assertFeature(t, loaded, features.AutoUpdate, true)
	assertFeature(t, loaded, features.TrafficMonitor, true)
	assertFeature(t, loaded, features.Mascot, true)
	assertFeature(t, loaded, features.SSHCommand, true)
	assertFeature(t, loaded, features.WakeReconnect, true)
	assertFeature(t, loaded, features.Automation, false)
	if _, ok := loaded.Features.Explicit(features.Automation); ok {
		t.Fatal("automation key must stay absent so the default stays off")
	}
	assertFeature(t, loaded, features.TunnelStats, true)
	assertFeature(t, loaded, features.Onboarding, true)
	if _, ok := loaded.Features.Explicit(features.Onboarding); ok {
		t.Fatal("onboarding key must stay absent so the default stays on")
	}
	assertFeature(t, loaded, features.Diagnostics, true)
	if _, ok := loaded.Features.Explicit(features.Diagnostics); ok {
		t.Fatal("diagnostics key must stay absent so the default stays on")
	}
	assertFeature(t, loaded, features.TunnelDiagnostics, true)
	if _, ok := loaded.Features.Explicit(features.TunnelDiagnostics); ok {
		t.Fatal("tunnel diagnostics key must stay absent so the default stays on")
	}
	assertFeature(t, loaded, features.AutostartHidden, true)
	if _, ok := loaded.Features.Explicit(features.AutostartHidden); ok {
		t.Fatal("autostart hidden key must stay absent so the default stays on")
	}
	assertFeature(t, loaded, features.BackgroundMode, false)
	if _, ok := loaded.Features.Explicit(features.BackgroundMode); ok {
		t.Fatal("background mode key must stay absent so the default stays off")
	}
	if _, ok := loaded.Features.Explicit(features.Profiles); ok {
		t.Fatal("profiles key must stay absent so the default can change later")
	}
}

func TestProfilesDefaultOffKeepsData(t *testing.T) {
	if features.Default(features.Profiles) {
		t.Fatal("profiles default must be off")
	}
	raw := []byte(`
version = 1

[[profiles]]
id = 1
name = "дом"
emoji = "🏠"
tunnel_ids = [1]

[[tunnels]]
id = 1
name = "db"
mode = "local"
local_host = "127.0.0.1"
local_port = 5432
remote_host = "10.0.0.8"
remote_port = 5432
status = "running"
`)
	cfg, err := ParseConfigTOML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Features.Enabled(features.Profiles) {
		t.Fatal("saved profiles must not turn the flag on")
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "дом" {
		t.Fatalf("profiles were dropped: %+v", cfg.Profiles)
	}
	if cfg.Tunnels[0].Status != "running" {
		t.Fatalf("reading the flag changed a tunnel: %+v", cfg.Tunnels[0])
	}
	if err := cfg.Features.Set(features.Profiles, false); err != nil {
		t.Fatal(err)
	}
	cfg.Normalize()
	if len(cfg.Profiles) != 1 || cfg.Tunnels[0].Status != "running" {
		t.Fatal("turning profiles off must keep saved profiles and running tunnels")
	}
	if cfg.Features.Enabled(features.Profiles) {
		t.Fatal("explicit off did not stick")
	}
}

func TestFeatureExplicitFalsePreserved(t *testing.T) {
	raw := []byte(`
version = 1

[features]
profiles = false
quick_search = false
notifications = false
auto_update = false
traffic_monitor = false
mascot = false
ssh_command = false
wake_reconnect = false
automation = false
tunnel_stats = false
onboarding = false
diagnostics = false
tunnel_diagnostics = false
autostart_hidden = false
background_mode = false
`)
	cfg, err := ParseConfigTOML(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range features.All() {
		if cfg.Features.Enabled(flag.ID) {
			t.Fatalf("%s explicit false became true", flag.ID)
		}
		if _, ok := cfg.Features.Explicit(flag.ID); !ok {
			t.Fatalf("%s explicit false was dropped", flag.ID)
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, MarshalTOML(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	storage, err := NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range features.All() {
		if again.Features.Enabled(flag.ID) {
			t.Fatalf("%s was not preserved", flag.ID)
		}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, key := range []string{"profiles = false", "quick_search = false", "traffic_monitor = false", "mascot = false", "ssh_command = false", "wake_reconnect = false", "automation = false", "tunnel_stats = false", "onboarding = false", "diagnostics = false", "tunnel_diagnostics = false", "autostart_hidden = false", "background_mode = false"} {
		if !strings.Contains(text, key) {
			t.Fatalf("saved config missing %q:\n%s", key, text)
		}
	}
}

func TestMigrateLegacyFeatureSettings(t *testing.T) {
	raw := []byte(`
version = 1
traffic_monitor_enabled = false
quick_search_enabled = false
quick_search_hotkey = "ctrl+shift+k"
notifications_set = true

[notifications]
enabled = true
dropped = true
reconnected = false
gave_up = true
connect_failed = true
connected = false

[[profiles]]
id = 2
name = "staging"
tunnel_ids = [1]

[[tunnels]]
id = 1
name = "api"
mode = "local"
local_host = "127.0.0.1"
local_port = 8080
remote_host = "10.1.0.4"
remote_port = 80
status = "running"
`)
	cfg, err := ParseConfigTOML(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertFeature(t, cfg, features.QuickSearch, false)
	assertFeature(t, cfg, features.TrafficMonitor, false)
	assertFeature(t, cfg, features.Notifications, true)
	assertFeature(t, cfg, features.Profiles, false)
	assertFeature(t, cfg, features.AutoUpdate, true)
	assertFeature(t, cfg, features.Mascot, true)
	assertFeature(t, cfg, features.SSHCommand, true)
	assertFeature(t, cfg, features.WakeReconnect, true)
	assertFeature(t, cfg, features.Automation, false)
	assertFeature(t, cfg, features.TunnelStats, true)
	assertFeature(t, cfg, features.Onboarding, true)
	assertFeature(t, cfg, features.Diagnostics, true)
	assertFeature(t, cfg, features.TunnelDiagnostics, true)
	assertFeature(t, cfg, features.AutostartHidden, true)
	if _, ok := cfg.Features.Explicit(features.AutostartHidden); ok {
		t.Fatal("legacy config must not invent an autostart hidden key")
	}
	assertFeature(t, cfg, features.BackgroundMode, false)
	if _, ok := cfg.Features.Explicit(features.BackgroundMode); ok {
		t.Fatal("legacy config must not invent a background mode key")
	}
	if cfg.QuickSearchHotkey != "ctrl+shift+k" {
		t.Fatalf("hotkey lost: %q", cfg.QuickSearchHotkey)
	}
	if !cfg.Notifications.Dropped || cfg.Notifications.Reconnected {
		t.Fatalf("per-event toggles changed: %+v", cfg.Notifications)
	}
	if len(cfg.Profiles) != 1 || cfg.Tunnels[0].Status != "running" {
		t.Fatal("migration touched profiles or tunnels")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	storage, err := NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(next *Config) error {
		*next = *cfg.Clone()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	again, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertFeature(t, again, features.QuickSearch, false)
	assertFeature(t, again, features.TrafficMonitor, false)
	assertFeature(t, again, features.Notifications, true)
	assertFeature(t, again, features.Profiles, false)
	if again.QuickSearchHotkey != "ctrl+shift+k" {
		t.Fatalf("hotkey not saved: %q", again.QuickSearchHotkey)
	}
	if len(again.Profiles) != 1 || again.Profiles[0].Name != "staging" {
		t.Fatalf("profiles: %+v", again.Profiles)
	}
}

func TestExplicitFeatureWinsOverLegacy(t *testing.T) {
	raw := []byte(`
version = 1
traffic_monitor_enabled = true
quick_search_enabled = true
notifications_set = true

[notifications]
enabled = true

[features]
quick_search = false
notifications = false
traffic_monitor = false
`)
	cfg, err := ParseConfigTOML(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertFeature(t, cfg, features.QuickSearch, false)
	assertFeature(t, cfg, features.Notifications, false)
	assertFeature(t, cfg, features.TrafficMonitor, false)
	if cfg.TrafficMonitorEnabled {
		t.Fatal("legacy traffic mirror was not updated from the flag")
	}
	if cfg.QuickSearchOn() {
		t.Fatal("quick search mirror ignored the flag")
	}
	if cfg.Notifications.Enabled {
		t.Fatal("notification mirror ignored the flag")
	}
}

func assertFeature(t *testing.T, cfg *Config, id features.ID, want bool) {
	t.Helper()
	if cfg.Features.Enabled(id) != want {
		t.Fatalf("%s: got %v want %v", id, cfg.Features.Enabled(id), want)
	}
}
