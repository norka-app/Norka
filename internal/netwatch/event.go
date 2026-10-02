// Package netwatch reports system resume and network changes.
//
// Platform watchers are best-effort. A monotonic-versus-wall-clock jump
// detector runs everywhere, so a long sleep is still noticed when the OS
// notification is missing. Bursts collapse to one event.
package netwatch

import "context"

// Kind is why the machine should recheck live SSH sessions.
type Kind string

const (
	// KindResume is a return from sleep, or a wall-clock jump that looks like one.
	KindResume Kind = "resume"
	// KindNetwork is a link, address, or route change.
	KindNetwork Kind = "network"
)

// Event is one debounced wake or network change.
type Event struct {
	Kind Kind
}

func emit(ctx context.Context, out chan<- Event, ev Event) {
	if ev.Kind == "" || out == nil {
		return
	}
	select {
	case <-ctx.Done():
	case out <- ev:
	default:
	}
}
