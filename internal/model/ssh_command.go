package model

// SSHCommandWarning is a non-fatal parse note. Option and Detail never carry secrets.
type SSHCommandWarning struct {
	Code   string `json:"code"`
	Option string `json:"option,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// SSHCommandHost is one jump host the paste preview will reuse or create.
// Password is intentionally absent: nothing secret from the command line is stored.
type SSHCommandHost struct {
	Key                    string `json:"key"`
	Name                   string `json:"name"`
	Host                   string `json:"host"`
	Port                   int    `json:"port"`
	User                   string `json:"user"`
	AuthType               string `json:"authType"`
	KeyPath                string `json:"keyPath,omitempty"`
	AgentSocketPath        string `json:"agentSocketPath,omitempty"`
	KeepAliveIntervalMs    int    `json:"keepAliveIntervalMs"`
	TimeoutMs              int    `json:"timeoutMs"`
	BypassHostVerification bool   `json:"bypassHostVerification"`
	HostKeyAlgorithms      string `json:"hostKeyAlgorithms,omitempty"`
	Alias                  string `json:"alias,omitempty"`
	ExistingID             int    `json:"existingId,omitempty"`
	ExistingName           string `json:"existingName,omitempty"`
	Ready                  bool   `json:"ready"`
}

// SSHCommandTunnel is one forward from a pasted ssh command.
type SSHCommandTunnel struct {
	Name        string   `json:"name"`
	Mode        string   `json:"mode"`
	LocalHost   string   `json:"localHost"`
	LocalPort   int      `json:"localPort"`
	RemoteHost  string   `json:"remoteHost"`
	RemotePort  int      `json:"remotePort"`
	HostKeys    []string `json:"hostKeys"`
	ChainLabel  string   `json:"chainLabel"`
	Blocked     bool     `json:"blocked"`
	BlockReason string   `json:"blockReason,omitempty"`
}

// SSHCommandPreview is shown before any jumper or tunnel is saved.
type SSHCommandPreview struct {
	Hosts    []SSHCommandHost    `json:"hosts"`
	Tunnels  []SSHCommandTunnel  `json:"tunnels"`
	Warnings []SSHCommandWarning `json:"warnings"`
}
