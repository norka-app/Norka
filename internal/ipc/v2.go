package ipc

import (
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/tunnelstats"
)

// ClientInfo is who is calling. hello sends it. The token stays out of this struct.
type ClientInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
}

// Hello is the server's greeting. Owner is "gui" or "daemon".
// Version is the application version recorded in engine.lock.
type Hello struct {
	ProtocolVersion  int    `json:"protocol_version"`
	MinClientVersion int    `json:"min_client_version"`
	Owner            string `json:"owner"`
	PID              int    `json:"pid"`
	Version          string `json:"version"`
}

// State is a full snapshot a GUI can mirror.
// Stats is omitted when the tunnel stats feature is off.
// Jumpers never include passwords.
type State struct {
	Tunnels []TunnelInfo        `json:"tunnels"`
	Jumpers []JumperInfo        `json:"jumpers"`
	Groups  []GroupInfo         `json:"groups"`
	Stats   *[]tunnelstats.View `json:"stats,omitempty"`
}

// JumperInfo is a jumper without secrets.
type JumperInfo struct {
	ID                     int    `json:"id"`
	Name                   string `json:"name"`
	Host                   string `json:"host"`
	Port                   int    `json:"port"`
	User                   string `json:"user"`
	AuthType               string `json:"authType"`
	KeyPath                string `json:"keyPath,omitempty"`
	AgentSocketPath        string `json:"agentSocketPath,omitempty"`
	HasSecret              bool   `json:"hasSecret,omitempty"`
	BypassHostVerification bool   `json:"bypassHostVerification,omitempty"`
	KeepAliveIntervalMs    int    `json:"keepAliveIntervalMs,omitempty"`
	TimeoutMs              int    `json:"timeoutMs,omitempty"`
	HostKeyAlgorithms      string `json:"hostKeyAlgorithms,omitempty"`
	Notes                  string `json:"notes,omitempty"`
}

// GroupInfo is one tunnel group.
type GroupInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// HopSecret is a password or key passphrase for one start or restart.
// The owner uses it in memory for that dial. It is not stored, logged, or returned.
type HopSecret struct {
	JumperID int    `json:"jumperId"`
	Secret   string `json:"secret,omitempty"`
}

// Control asks the owner to change a tunnel, jumper, or group.
// Start, stop and restart apply only to a tunnel.
// Save with ID 0 creates; a positive ID updates.
// Payloads match the GUI shapes. The owner writes them through conf.Storage.
// Secrets is only for start and restart. It is not a config write.
type Control struct {
	Action  string                    `json:"action"`
	Kind    string                    `json:"kind"`
	ID      int                       `json:"id,omitempty"`
	Tunnel  *model.TunnelPayload      `json:"tunnel,omitempty"`
	Jumper  *model.JumperPayload      `json:"jumper,omitempty"`
	Group   *model.TunnelGroupPayload `json:"group,omitempty"`
	Secrets []HopSecret               `json:"secrets,omitempty"`
}

// Event is one line on a subscribe connection after the opening snapshot.
// Type is status, log, notification, or config.
// A slow subscriber can miss lines; it should call state again.
type Event struct {
	Type     string      `json:"type"`
	Tunnel   *TunnelInfo `json:"tunnel,omitempty"`
	Level    string      `json:"level,omitempty"`
	Line     string      `json:"line,omitempty"`
	Title    string      `json:"title,omitempty"`
	Body     string      `json:"body,omitempty"`
	TunnelID int         `json:"tunnelId,omitempty"`
	State    *State      `json:"state,omitempty"`
}

// IsV2Op reports operations that exist only in dialect 2.
func IsV2Op(op string) bool {
	switch op {
	case OpHello, OpState, OpSubscribe, OpControl, OpShutdown, OpHandover:
		return true
	default:
		return false
	}
}
