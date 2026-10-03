package engine

import (
	"context"
	"log/slog"

	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/notify"
)

// sub is one subscribe client. A full buffer drops the event so a slow
// GUI cannot stall tunnel status updates.
type sub struct {
	ch chan ipc.Event
}

func (e *Engine) publish(ev ipc.Event) {
	if e == nil || ev.Type == "" {
		return
	}
	e.subMu.Lock()
	subs := append([]*sub(nil), e.subs...)
	e.subMu.Unlock()
	for _, s := range subs {
		select {
		case s.ch <- ev:
		default:
		}
	}
}

func (e *Engine) addSub() *sub {
	s := &sub{ch: make(chan ipc.Event, 64)}
	e.subMu.Lock()
	e.subs = append(e.subs, s)
	e.subMu.Unlock()
	return s
}

func (e *Engine) removeSub(target *sub) {
	if e == nil || target == nil {
		return
	}
	e.subMu.Lock()
	defer e.subMu.Unlock()
	kept := e.subs[:0]
	for _, s := range e.subs {
		if s != target {
			kept = append(kept, s)
		}
	}
	e.subs = kept
}

func (e *Engine) publishNotification(n notify.Notice) {
	e.publish(ipc.Event{
		Type:     ipc.EventNotification,
		Title:    n.Title,
		Body:     n.Body,
		TunnelID: n.TunnelID,
	})
}

func (e *Engine) publishConfig() {
	st, err := e.snapshot()
	if err != nil {
		e.publish(ipc.Event{Type: ipc.EventConfig})
		return
	}
	e.publish(ipc.Event{Type: ipc.EventConfig, State: &st})
}

// LogHandler copies each log record onto the subscribe stream and then
// passes it to next. norkad wraps its file or stderr handler with this.
func (e *Engine) LogHandler(next slog.Handler) slog.Handler {
	if e == nil {
		return next
	}
	return &logFanout{
		next: next,
		emit: func(level, line string) {
			e.publish(ipc.Event{Type: ipc.EventLog, Level: level, Line: line})
		},
	}
}

type logFanout struct {
	next slog.Handler
	emit func(level, line string)
}

func (h *logFanout) Enabled(ctx context.Context, level slog.Level) bool {
	if h == nil {
		return false
	}
	if h.next != nil {
		return h.next.Enabled(ctx, level)
	}
	return true
}

func (h *logFanout) Handle(ctx context.Context, rec slog.Record) error {
	if h == nil {
		return nil
	}
	if h.emit != nil {
		if line := formatLog(rec); line != "" {
			h.emit(rec.Level.String(), line)
		}
	}
	if h.next == nil {
		return nil
	}
	return h.next.Handle(ctx, rec)
}

func (h *logFanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h == nil {
		return h
	}
	next := h.next
	if next != nil {
		next = next.WithAttrs(attrs)
	}
	return &logFanout{next: next, emit: h.emit}
}

func (h *logFanout) WithGroup(name string) slog.Handler {
	if h == nil {
		return h
	}
	next := h.next
	if next != nil {
		next = next.WithGroup(name)
	}
	return &logFanout{next: next, emit: h.emit}
}

func formatLog(rec slog.Record) string {
	line := rec.Message
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "" {
			return true
		}
		line += " " + a.Key + "=" + a.Value.String()
		return true
	})
	return line
}

// publishingPoster tells subscribers about a notification, then shows it.
type publishingPoster struct {
	next notify.Poster
	emit func(notify.Notice)
}

func (p publishingPoster) Post(notice notify.Notice) error {
	if p.emit != nil {
		p.emit(notice)
	}
	if p.next == nil {
		return nil
	}
	return p.next.Post(notice)
}

// Done is closed when shutdown or handover asks this process to leave.
// The host should drop the listener and exit. Handover has already stopped
// the tunnels and released engine.lock. Shutdown has stopped the tunnels;
// the host still releases the lock, usually from Engine.Shutdown.
func (e *Engine) Done() <-chan struct{} {
	return e.stopped()
}

// NotifyStop calls fn when shutdown or handover asks this process to leave.
// fn runs in its own goroutine. A nil engine or fn does nothing.
func (e *Engine) NotifyStop(fn func()) {
	if e == nil || fn == nil {
		return
	}
	ch := e.stopped()
	go func() {
		<-ch
		fn()
	}()
}

func (e *Engine) stopped() <-chan struct{} {
	if e == nil || e.stopCh == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return e.stopCh
}

func (e *Engine) signalStop() {
	if e == nil || e.stopCh == nil {
		return
	}
	e.stopOnce.Do(func() {
		close(e.stopCh)
	})
}

// bindTunnel publishes a status event whenever a tunnel's saved status changes.
func (e *Engine) bindTunnel() {
	if e == nil || e.tunnel == nil {
		return
	}
	e.tunnel.SetStatusWatch(func(tunnel model.Tunnel) {
		info := tunnelInfos([]model.Tunnel{tunnel})
		if len(info) != 1 {
			return
		}
		e.publish(ipc.Event{Type: ipc.EventStatus, Tunnel: &info[0]})
	})
}
