package notify

import (
	"log/slog"
	"sync"
	"time"
)

// Poster shows one OS notification.
type Poster interface {
	Post(notice Notice) error
}

// Service aggregates tunnel events and posts them after a short quiet period.
type Service struct {
	mu       sync.Mutex
	window   time.Duration
	settings func() Settings
	catalog  func() Catalog
	poster   Poster
	pending  []Event
	timer    *time.Timer
}

// ServiceConfig wires the notifier.
type ServiceConfig struct {
	Window   time.Duration
	Settings func() Settings
	Catalog  func() Catalog
	Poster   Poster
}

func NewService(cfg ServiceConfig) *Service {
	window := cfg.Window
	if window <= 0 {
		window = 2 * time.Second
	}
	catalog := cfg.Catalog
	if catalog == nil {
		catalog = func() Catalog { return CatalogFor("ru") }
	}
	return &Service{
		window:   window,
		settings: cfg.Settings,
		catalog:  catalog,
		poster:   cfg.Poster,
	}
}

func (s *Service) Dropped(id int, name string) {
	s.emit(Event{Kind: KindDropped, TunnelID: id, TunnelName: name})
}

func (s *Service) Reconnected(id int, name string) {
	s.emit(Event{Kind: KindReconnected, TunnelID: id, TunnelName: name})
}

func (s *Service) GaveUp(id int, name string) {
	s.emit(Event{Kind: KindGaveUp, TunnelID: id, TunnelName: name})
}

func (s *Service) ConnectFailed(id int, name string) {
	s.emit(Event{Kind: KindConnectFailed, TunnelID: id, TunnelName: name})
}

func (s *Service) Connected(id int, name string) {
	s.emit(Event{Kind: KindConnected, TunnelID: id, TunnelName: name})
}

// PostWake sends one summary after a sleep or network change.
// Per-tunnel drop and reconnect notices for that cycle are suppressed by the caller.
// Nothing is posted when notifications are off or neither of those kinds is enabled.
func (s *Service) PostWake(resume bool) {
	if s == nil || s.poster == nil {
		return
	}
	if !s.allows(KindDropped) && !s.allows(KindReconnected) {
		return
	}
	cat := s.catalog()
	body := cat.WakeNetwork
	if resume {
		body = cat.WakeResume
	}
	if body == "" {
		return
	}
	if err := s.poster.Post(Notice{Title: cat.Title, Body: body}); err != nil {
		slog.Warn("wake notification failed", "error", err)
	}
}

func (s *Service) emit(ev Event) {
	if s == nil || !s.allows(ev.Kind) {
		return
	}
	s.mu.Lock()
	s.pending = append(s.pending, ev)
	if s.timer == nil {
		s.timer = time.AfterFunc(s.window, func() { s.FlushNow() })
	}
	s.mu.Unlock()
}

func (s *Service) allows(kind Kind) bool {
	if s.settings == nil {
		return false
	}
	return s.settings().Allows(kind)
}

// FlushNow posts everything waiting. Tests use it instead of waiting out the window.
func (s *Service) FlushNow() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	pending := s.pending
	s.pending = nil
	s.mu.Unlock()
	if len(pending) == 0 || s.poster == nil {
		return
	}
	cat := s.catalog()
	for _, group := range Coalesce(pending) {
		notice := Format(cat, group)
		if notice.Body == "" {
			continue
		}
		if err := s.poster.Post(notice); err != nil {
			slog.Warn("notification failed", "kind", group.Kind, "error", err)
		}
	}
}
