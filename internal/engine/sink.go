package engine

// Events the GUI used to emit through Wails. The strings are part of the
// frontend contract and stay stable.
const (
	EventNotificationFocus = "notification:focus"
	EventJournal           = "journal:entry"
	EventAutomationPrompt  = "automation:prompt"
)

// Sink is how the engine reaches a host. The GUI implements it with Wails
// and the tray. A future background daemon can pass nil: every call is then
// dropped. Emit replaces runtime.EventsEmit. ShowWindow brings the main
// window forward. TunnelsChanged refreshes the tray and asks the UI to reload.
type Sink interface {
	Emit(event string, payload any)
	ShowWindow()
	TunnelsChanged()
}

// Host is the GUI (or daemon) side the engine calls back into.
// Locale and NotifyIcon are read when an event is built, not snapshotted.
type Host struct {
	Sink       Sink
	Locale     func() string
	NotifyIcon func() string
}
