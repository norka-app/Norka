package model

// Profile is a named set of tunnels (an environment). A tunnel may belong to
// several profiles. Membership is the list of tunnel ids, not a single group.
type Profile struct {
	ID        int    `json:"id" toml:"id"`
	Name      string `json:"name" toml:"name"`
	Color     string `json:"color,omitempty" toml:"color,omitempty"`
	Emoji     string `json:"emoji,omitempty" toml:"emoji,omitempty"`
	TunnelIDs []int  `json:"tunnelIds" toml:"tunnel_ids"`
}

// ProfilePayload is used by create/update profile APIs.
type ProfilePayload struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	Emoji     string `json:"emoji"`
	TunnelIDs []int  `json:"tunnelIds"`
}

// ProfileConflict is a tunnel that activation refused to start because its
// local port is held by another tunnel.
type ProfileConflict struct {
	TunnelID      int    `json:"tunnelId"`
	TunnelName    string `json:"tunnelName"`
	HolderID      int    `json:"holderId"`
	HolderName    string `json:"holderName"`
	Port          int    `json:"port"`
	InsideProfile bool   `json:"insideProfile"`
}

// ProfileTunnelError is a start or stop that failed while activating a profile.
type ProfileTunnelError struct {
	TunnelID int    `json:"tunnelId"`
	Action   string `json:"action"`
	Error    string `json:"error"`
}

// ProfileActivationResult is what ActivateProfile returns to the UI.
type ProfileActivationResult struct {
	ProfileID int                  `json:"profileId"`
	Started   []int                `json:"started"`
	Stopped   []int                `json:"stopped"`
	AlreadyOn []int                `json:"alreadyOn"`
	Conflicts []ProfileConflict    `json:"conflicts"`
	Errors    []ProfileTunnelError `json:"errors"`
}

// ActivationPlan is the pure decision of PlanProfileActivation: which tunnels
// to stop, which to start, and which to leave because of a port conflict.
// Stops run before starts. Conflicts are not started.
type ActivationPlan struct {
	StartIDs   []int
	StopIDs    []int
	AlreadyIDs []int
	Conflicts  []ProfileConflict
}

// QuickSearchSettings is the global command-palette hotkey.
// Enabled defaults to true when the config key is absent (old files).
// Hotkey empty means the platform default (ctrl+alt+space / option+space).
type QuickSearchSettings struct {
	Enabled         bool   `json:"enabled"`
	Hotkey          string `json:"hotkey"`
	EffectiveHotkey string `json:"effectiveHotkey"`
	Platform        string `json:"platform"`
	Registered      bool   `json:"registered"`
	ErrorCode       string `json:"errorCode"`
	ErrorDetail     string `json:"errorDetail"`
}
