package conf

import (
	"os"
	"path/filepath"
	"strings"

	"norka/internal/model"
)

const defaultConfigPath = "config.toml"

const appConfigDirName = ".norka"

// PrivateDirPerm and PrivateFilePerm keep the local config unreadable to other
// users. Jumper passwords belong in the OS keychain; these modes still protect
// the file when the keychain is unavailable.
const (
	PrivateDirPerm  os.FileMode = 0o700
	PrivateFilePerm os.FileMode = 0o600
)

// DefaultConfigFileName is the config file basename inside the config directory.
const DefaultConfigFileName = defaultConfigPath

const currentConfigVersion = 1

// isDirWritable checks if a directory is writable by attempting to create a temp file.
func isDirWritable(dir string) bool {
	tmpFile, err := os.CreateTemp(dir, ".write_test_*")
	if err != nil {
		return false
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())
	return true
}

// getDefaultConfigDir returns the default config directory for the app.
// Priority:
// 1. Current directory (if writable) - for development and portable mode
// 2. User config directory (homeConfigDir) - for installed apps
func getDefaultConfigDir() string {
	// Try current directory first (for development and portable mode)
	cwd, err := os.Getwd()
	if err == nil && isDirWritable(cwd) {
		return cwd
	}

	// Fallback to user config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Last resort: current directory even if not writable
		return "."
	}
	if dir := homeConfigDir(homeDir); dir != "" {
		return dir
	}
	return "."
}

// Config is persisted in TOML storage.
//
// Profiles group tunnels into named environments. A tunnel may sit in several
// profiles (membership is Profile.TunnelIDs). ActiveProfileID 0 means none.
// ProfileStopOthers is off by default: activating a profile connects its
// tunnels and leaves every other tunnel running. When it is on, tunnels
// outside the profile are stopped first.
//
// QuickSearchEnabled is a pointer so an old config that lacks the key stays
// nil and is treated as enabled. An explicit false is saved when the user
// turns the global hotkey off. QuickSearchHotkey empty means the platform
// default.
type Config struct {
	Version               int                  `toml:"version"`
	Jumpers               []model.Jumper       `toml:"jumpers"`
	Groups                []model.TunnelGroup  `toml:"groups"`
	Tunnels               []model.Tunnel       `toml:"tunnels"`
	Profiles              []model.Profile      `toml:"profiles"`
	ActiveProfileID       int                  `toml:"active_profile_id"`
	ProfileStopOthers     bool                 `toml:"profile_stop_others"`
	AutoRun               bool                 `toml:"auto_run"`
	TrafficMonitorEnabled bool                 `toml:"traffic_monitor_enabled"`
	Notifications         NotificationSettings `toml:"notifications"`
	// NotificationsSet is true after defaults have been applied, so an explicit
	// all-off choice is not replaced by defaults on the next load.
	NotificationsSet bool `toml:"notifications_set"`
	// Language is the UI language preference: "auto", "ru", or "en".
	// Empty means auto (follow the system locale: Russian for ru*, English otherwise).
	Language           string `toml:"language,omitempty"`
	QuickSearchEnabled *bool  `toml:"quick_search_enabled,omitempty"`
	QuickSearchHotkey  string `toml:"quick_search_hotkey,omitempty"`
}

// NotificationSettings controls opt-in OS notifications.
// The master switch defaults to off. When it is turned on, drop, reconnect,
// give-up and connect-failed are on; a successful connect stays off.
type NotificationSettings struct {
	Enabled       bool `json:"enabled" toml:"enabled"`
	Dropped       bool `json:"dropped" toml:"dropped"`
	Reconnected   bool `json:"reconnected" toml:"reconnected"`
	GaveUp        bool `json:"gaveUp" toml:"gave_up"`
	ConnectFailed bool `json:"connectFailed" toml:"connect_failed"`
	Connected     bool `json:"connected" toml:"connected"`
}

// DefaultNotificationSettings is the opt-in baseline: nothing is shown until
// the master switch is enabled, and a plain "connected" toast stays off.
func DefaultNotificationSettings() NotificationSettings {
	return NotificationSettings{
		Enabled:       false,
		Dropped:       true,
		Reconnected:   true,
		GaveUp:        true,
		ConnectFailed: true,
		Connected:     false,
	}
}

// GetHomeConfigPath returns the absolute path for the home config file.
// Windows and macOS use ~/.norka/config.toml. Linux follows XDG
// ($XDG_CONFIG_HOME/norka or ~/.config/norka), with a fallback to ~/.norka
// when that legacy file already exists. Empty string if UserHomeDir fails.
func GetHomeConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ""
	}
	return filepath.Join(homeConfigDir(homeDir), defaultConfigPath)
}

// ResolveConfigPath returns the effective config file path: implicit location
// from runtime mode, then config.root redirection when valid.
func ResolveConfigPath() string {
	return ResolveEffectiveConfigPath(ResolveImplicitConfigPath())
}

// DefaultConfig creates an empty config.
func DefaultConfig() *Config {
	return &Config{
		Version:               currentConfigVersion,
		Jumpers:               []model.Jumper{},
		Groups:                []model.TunnelGroup{},
		Tunnels:               []model.Tunnel{},
		AutoRun:               false,
		TrafficMonitorEnabled: true,
		Notifications:         DefaultNotificationSettings(),
		NotificationsSet:      true,
	}
}

// Clone returns a detached copy.
func (c *Config) Clone() *Config {
	if c == nil {
		return DefaultConfig()
	}

	out := &Config{
		Version:               c.Version,
		ActiveProfileID:       c.ActiveProfileID,
		ProfileStopOthers:     c.ProfileStopOthers,
		AutoRun:               c.AutoRun,
		TrafficMonitorEnabled: c.TrafficMonitorEnabled,
		Notifications:         c.Notifications,
		NotificationsSet:      c.NotificationsSet,
		Language:              c.Language,
		QuickSearchHotkey:     c.QuickSearchHotkey,
	}
	if c.QuickSearchEnabled != nil {
		enabled := *c.QuickSearchEnabled
		out.QuickSearchEnabled = &enabled
	}
	out.Jumpers = append(out.Jumpers, c.Jumpers...)
	out.Groups = append(out.Groups, c.Groups...)
	out.Tunnels = append(out.Tunnels, c.Tunnels...)
	out.Profiles = append(out.Profiles, c.Profiles...)
	for i := range out.Profiles {
		out.Profiles[i].TunnelIDs = append([]int{}, c.Profiles[i].TunnelIDs...)
	}
	return out
}

// QuickSearchOn reports whether the global palette hotkey should be registered.
// A missing key (nil) means enabled, so configs written before the feature load
// with the hotkey on.
func (c *Config) QuickSearchOn() bool {
	if c == nil || c.QuickSearchEnabled == nil {
		return true
	}
	return *c.QuickSearchEnabled
}

// Normalize ensures stable defaults before save.
func (c *Config) Normalize() {
	if c.Version <= 0 {
		c.Version = currentConfigVersion
	}
	if c.Jumpers == nil {
		c.Jumpers = []model.Jumper{}
	}
	if c.Groups == nil {
		c.Groups = []model.TunnelGroup{}
	}
	if c.Tunnels == nil {
		c.Tunnels = []model.Tunnel{}
	}
	if c.Profiles == nil {
		c.Profiles = []model.Profile{}
	}
	// AutoRun defaults to false; no need to set if already present
	if !c.NotificationsSet {
		c.Notifications = DefaultNotificationSettings()
		c.NotificationsSet = true
	}
	c.Language = strings.TrimSpace(c.Language)
	knownTunnels := make(map[int]struct{}, len(c.Tunnels))
	for i := range c.Tunnels {
		c.Tunnels[i].JumperIDs = normalizeJumperIDs(c.Tunnels[i].JumperIDs)
		if c.Tunnels[i].ID > 0 {
			knownTunnels[c.Tunnels[i].ID] = struct{}{}
		}
	}
	c.Profiles = normalizeProfiles(c.Profiles, knownTunnels)
	if c.ActiveProfileID > 0 && profileIndex(c.Profiles, c.ActiveProfileID) < 0 {
		c.ActiveProfileID = 0
	}
}

func normalizeProfiles(profiles []model.Profile, knownTunnels map[int]struct{}) []model.Profile {
	if profiles == nil {
		return []model.Profile{}
	}
	out := make([]model.Profile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ID <= 0 {
			continue
		}
		ids := make([]int, 0, len(profile.TunnelIDs))
		seen := make(map[int]struct{}, len(profile.TunnelIDs))
		for _, id := range profile.TunnelIDs {
			if id <= 0 {
				continue
			}
			if _, ok := knownTunnels[id]; !ok {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		profile.TunnelIDs = ids
		profile.Name = strings.TrimSpace(profile.Name)
		out = append(out, profile)
	}
	return out
}

func profileIndex(profiles []model.Profile, id int) int {
	for i := range profiles {
		if profiles[i].ID == id {
			return i
		}
	}
	return -1
}

func normalizeJumperIDs(ids []int) []int {
	out := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids)+1)
	appendID := func(id int) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range ids {
		appendID(id)
	}
	return out
}
