// Package features is the registry of optional product flags.
//
// Each flag has a stable id, a default, and i18n keys for the Settings row.
// Config stores only explicit choices in the [features] table. A missing key
// uses the default, so an old config.toml keeps working. An explicit false is
// stored as false and must survive a later load.
//
// Every new user-facing capability goes behind a flag. Language and storing
// secrets in the OS keychain are not flags.
package features

import "fmt"

// ID is the stable key stored in config.toml and sent to the frontend.
type ID string

const (
	Profiles       ID = "profiles"
	QuickSearch    ID = "quick_search"
	Notifications  ID = "notifications"
	AutoUpdate     ID = "auto_update"
	TrafficMonitor ID = "traffic_monitor"
	Mascot         ID = "mascot"
	SSHCommand     ID = "ssh_command"
	WakeReconnect  ID = "wake_reconnect"
	Automation     ID = "automation"
	TunnelStats    ID = "tunnel_stats"
	Onboarding     ID = "onboarding"
	Diagnostics    ID = "diagnostics"
	// TunnelDiagnostics is on so a failed connect can be explained from the
	// tunnel menu without a trip to Settings. Checks run only when asked,
	// store nothing, and never send a password. Profiles stay off.
	TunnelDiagnostics ID = "tunnel_diagnostics"
	// AutostartHidden adds --norka-hidden to the login entry so the window
	// stays in the tray. Off restores the previous behavior: the window opens.
	// Default is on. An existing entry is rewritten on the next start.
	AutostartHidden ID = "autostart_hidden"
)

// Flag is one catalog entry. TitleKey and DescriptionKey are vue-i18n paths.
type Flag struct {
	ID             ID
	Default        bool
	TitleKey       string
	DescriptionKey string
}

// View is the resolved flag the frontend renders. Enabled already applies the default.
type View struct {
	ID             string `json:"id"`
	Enabled        bool   `json:"enabled"`
	Default        bool   `json:"default"`
	TitleKey       string `json:"titleKey"`
	DescriptionKey string `json:"descriptionKey"`
}

// Legacy is the previous on/off fields, read once when the matching [features]
// key is still missing.
type Legacy struct {
	// QuickSearch is nil when the old key was absent (treat as the default).
	QuickSearch *bool
	// NotificationsOn is the old master switch. Only true is migrated:
	// false is already the default.
	NotificationsOn bool
	// TrafficExplicitOff is the old traffic_monitor_enabled = false.
	// The historical default is on, so a missing key stays on.
	TrafficExplicitOff bool
}

// All returns the catalog in Settings order.
func All() []Flag {
	return []Flag{
		{ID: Profiles, Default: false, TitleKey: "features.profiles", DescriptionKey: "features.profilesDesc"},
		{ID: QuickSearch, Default: true, TitleKey: "features.quickSearch", DescriptionKey: "features.quickSearchDesc"},
		{ID: Notifications, Default: false, TitleKey: "features.notifications", DescriptionKey: "features.notificationsDesc"},
		{ID: AutoUpdate, Default: true, TitleKey: "features.autoUpdate", DescriptionKey: "features.autoUpdateDesc"},
		{ID: TrafficMonitor, Default: true, TitleKey: "features.trafficMonitor", DescriptionKey: "features.trafficMonitorDesc"},
		{ID: Mascot, Default: true, TitleKey: "features.mascot", DescriptionKey: "features.mascotDesc"},
		{ID: SSHCommand, Default: true, TitleKey: "features.sshCommand", DescriptionKey: "features.sshCommandDesc"},
		{ID: WakeReconnect, Default: true, TitleKey: "features.wakeReconnect", DescriptionKey: "features.wakeReconnectDesc"},
		{ID: Automation, Default: false, TitleKey: "features.automation", DescriptionKey: "features.automationDesc"},
		{ID: TunnelStats, Default: true, TitleKey: "features.tunnelStats", DescriptionKey: "features.tunnelStatsDesc"},
		{ID: Onboarding, Default: true, TitleKey: "features.onboarding", DescriptionKey: "features.onboardingDesc"},
		{ID: Diagnostics, Default: true, TitleKey: "features.diagnostics", DescriptionKey: "features.diagnosticsDesc"},
		{ID: TunnelDiagnostics, Default: true, TitleKey: "features.tunnelDiagnostics", DescriptionKey: "features.tunnelDiagnosticsDesc"},
		{ID: AutostartHidden, Default: true, TitleKey: "features.autostartHidden", DescriptionKey: "features.autostartHiddenDesc"},
	}
}

// Known reports whether id is in the catalog.
func Known(id ID) bool {
	_, ok := find(id)
	return ok
}

// Default returns the catalog default. An unknown id is off.
func Default(id ID) bool {
	flag, ok := find(id)
	if !ok {
		return false
	}
	return flag.Default
}

func find(id ID) (Flag, bool) {
	for _, flag := range All() {
		if flag.ID == id {
			return flag, true
		}
	}
	return Flag{}, false
}

// Flags holds explicit overrides. A nil pointer means "use the default".
type Flags struct {
	Profiles          *bool `toml:"profiles,omitempty" json:"profiles,omitempty"`
	QuickSearch       *bool `toml:"quick_search,omitempty" json:"quickSearch,omitempty"`
	Notifications     *bool `toml:"notifications,omitempty" json:"notifications,omitempty"`
	AutoUpdate        *bool `toml:"auto_update,omitempty" json:"autoUpdate,omitempty"`
	TrafficMonitor    *bool `toml:"traffic_monitor,omitempty" json:"trafficMonitor,omitempty"`
	Mascot            *bool `toml:"mascot,omitempty" json:"mascot,omitempty"`
	SSHCommand        *bool `toml:"ssh_command,omitempty" json:"sshCommand,omitempty"`
	WakeReconnect     *bool `toml:"wake_reconnect,omitempty" json:"wakeReconnect,omitempty"`
	Automation        *bool `toml:"automation,omitempty" json:"automation,omitempty"`
	TunnelStats       *bool `toml:"tunnel_stats,omitempty" json:"tunnelStats,omitempty"`
	Onboarding        *bool `toml:"onboarding,omitempty" json:"onboarding,omitempty"`
	Diagnostics       *bool `toml:"diagnostics,omitempty" json:"diagnostics,omitempty"`
	TunnelDiagnostics *bool `toml:"tunnel_diagnostics,omitempty" json:"tunnelDiagnostics,omitempty"`
	AutostartHidden   *bool `toml:"autostart_hidden,omitempty" json:"autostartHidden,omitempty"`
}

// Enabled reports the effective value: the explicit choice, or the default.
func (f Flags) Enabled(id ID) bool {
	if p := f.ptr(id); p != nil {
		return *p
	}
	return Default(id)
}

// Set stores an explicit choice, including false. Unknown ids are rejected.
func (f *Flags) Set(id ID, enabled bool) error {
	if f == nil {
		return fmt.Errorf("feature flags are nil")
	}
	if !Known(id) {
		return fmt.Errorf("unknown feature %q", id)
	}
	value := enabled
	switch id {
	case Profiles:
		f.Profiles = &value
	case QuickSearch:
		f.QuickSearch = &value
	case Notifications:
		f.Notifications = &value
	case AutoUpdate:
		f.AutoUpdate = &value
	case TrafficMonitor:
		f.TrafficMonitor = &value
	case Mascot:
		f.Mascot = &value
	case SSHCommand:
		f.SSHCommand = &value
	case WakeReconnect:
		f.WakeReconnect = &value
	case Automation:
		f.Automation = &value
	case TunnelStats:
		f.TunnelStats = &value
	case Onboarding:
		f.Onboarding = &value
	case Diagnostics:
		f.Diagnostics = &value
	case TunnelDiagnostics:
		f.TunnelDiagnostics = &value
	case AutostartHidden:
		f.AutostartHidden = &value
	default:
		return fmt.Errorf("unknown feature %q", id)
	}
	return nil
}

// Views resolves every catalog flag for the frontend.
func (f Flags) Views() []View {
	all := All()
	out := make([]View, 0, len(all))
	for _, flag := range all {
		out = append(out, View{
			ID:             string(flag.ID),
			Enabled:        f.Enabled(flag.ID),
			Default:        flag.Default,
			TitleKey:       flag.TitleKey,
			DescriptionKey: flag.DescriptionKey,
		})
	}
	return out
}

// Clone returns a detached copy so later Set calls do not alias stored config.
func (f Flags) Clone() Flags {
	return Flags{
		Profiles:          cloneBool(f.Profiles),
		QuickSearch:       cloneBool(f.QuickSearch),
		Notifications:     cloneBool(f.Notifications),
		AutoUpdate:        cloneBool(f.AutoUpdate),
		TrafficMonitor:    cloneBool(f.TrafficMonitor),
		Mascot:            cloneBool(f.Mascot),
		SSHCommand:        cloneBool(f.SSHCommand),
		WakeReconnect:     cloneBool(f.WakeReconnect),
		Automation:        cloneBool(f.Automation),
		TunnelStats:       cloneBool(f.TunnelStats),
		Onboarding:        cloneBool(f.Onboarding),
		Diagnostics:       cloneBool(f.Diagnostics),
		TunnelDiagnostics: cloneBool(f.TunnelDiagnostics),
		AutostartHidden:   cloneBool(f.AutostartHidden),
	}
}

// MigrateLegacy copies an old on/off field into the registry when that flag
// has no explicit key yet. Existing explicit values are left alone.
func (f *Flags) MigrateLegacy(legacy Legacy) {
	if f == nil {
		return
	}
	if f.QuickSearch == nil && legacy.QuickSearch != nil {
		value := *legacy.QuickSearch
		f.QuickSearch = &value
	}
	if f.Notifications == nil && legacy.NotificationsOn {
		value := true
		f.Notifications = &value
	}
	if f.TrafficMonitor == nil && legacy.TrafficExplicitOff {
		value := false
		f.TrafficMonitor = &value
	}
}

// Explicit reports a stored choice. ok is false when the key is missing.
func (f Flags) Explicit(id ID) (bool, bool) {
	value := f.ptr(id)
	if value == nil {
		return false, false
	}
	return *value, true
}

func (f Flags) ptr(id ID) *bool {
	switch id {
	case Profiles:
		return f.Profiles
	case QuickSearch:
		return f.QuickSearch
	case Notifications:
		return f.Notifications
	case AutoUpdate:
		return f.AutoUpdate
	case TrafficMonitor:
		return f.TrafficMonitor
	case Mascot:
		return f.Mascot
	case SSHCommand:
		return f.SSHCommand
	case WakeReconnect:
		return f.WakeReconnect
	case Automation:
		return f.Automation
	case TunnelStats:
		return f.TunnelStats
	case Onboarding:
		return f.Onboarding
	case Diagnostics:
		return f.Diagnostics
	case TunnelDiagnostics:
		return f.TunnelDiagnostics
	case AutostartHidden:
		return f.AutostartHidden
	default:
		return nil
	}
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
