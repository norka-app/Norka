package tunnelstats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAccumulationBytesAndConnectedTime(t *testing.T) {
	s, clock := newTestStore(t)
	start := time.Date(2026, 10, 2, 10, 0, 0, 0, time.Local)
	*clock = start

	s.Begin(1)
	*clock = start.Add(30 * time.Second)
	s.Observe(1, 100, 50)
	*clock = start.Add(90 * time.Second)
	s.Observe(1, 140, 80)

	view := mustView(t, s, 1)
	if view.SessionBytesUp != 140 || view.SessionBytesDown != 80 {
		t.Fatalf("session bytes = %d/%d, want 140/80", view.SessionBytesUp, view.SessionBytesDown)
	}
	if view.TotalBytesUp != 140 || view.TotalBytesDown != 80 {
		t.Fatalf("total bytes = %d/%d, want 140/80", view.TotalBytesUp, view.TotalBytesDown)
	}
	if view.TodayConnectedSec != 90 || view.TotalConnectedSec != 90 {
		t.Fatalf("connected = today %d total %d, want 90/90", view.TodayConnectedSec, view.TotalConnectedSec)
	}
	if !view.Connected || view.SessionStartedUnix != start.UnixMilli() {
		t.Fatalf("session start = %d connected %v", view.SessionStartedUnix, view.Connected)
	}

	s.Pause(1)
	*clock = start.Add(2 * time.Minute)
	paused := mustView(t, s, 1)
	if paused.Connected || paused.TodayConnectedSec != 90 || paused.TotalConnectedSec != 90 {
		t.Fatalf("pause did not freeze connected time: %+v", paused)
	}
	if paused.TotalBytesUp != 140 || paused.SessionBytesUp != 0 {
		t.Fatalf("paused bytes = session %d total %d", paused.SessionBytesUp, paused.TotalBytesUp)
	}
}

func TestAccumulationSplitsMidnight(t *testing.T) {
	s, clock := newTestStore(t)
	start := time.Date(2026, 10, 2, 23, 50, 0, 0, time.Local)
	*clock = start
	s.Begin(1)
	*clock = start.Add(20 * time.Minute)
	view := mustView(t, s, 1)
	if view.TotalConnectedSec != 20*60 {
		t.Fatalf("total = %d, want %d", view.TotalConnectedSec, 20*60)
	}
	if view.TodayConnectedSec != 10*60 {
		t.Fatalf("today = %d, want %d", view.TodayConnectedSec, 10*60)
	}
}

func TestReconnectCounting(t *testing.T) {
	s, clock := newTestStore(t)
	day := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	*clock = day
	s.Begin(1)
	s.Observe(1, 10, 4)
	s.Pause(1)
	s.Reconnect(1)
	s.Pause(1)
	s.Reconnect(1)

	view := mustView(t, s, 1)
	if view.ReconnectsToday != 2 || view.ReconnectsTotal != 2 {
		t.Fatalf("reconnects today/total = %d/%d, want 2/2", view.ReconnectsToday, view.ReconnectsTotal)
	}
	if view.SessionBytesUp != 0 || view.TotalBytesUp != 10 {
		t.Fatalf("bytes after reconnect = session %d total %d", view.SessionBytesUp, view.TotalBytesUp)
	}

	s.Observe(1, 15, 9)
	view = mustView(t, s, 1)
	if view.SessionBytesUp != 5 || view.SessionBytesDown != 5 || view.TotalBytesUp != 15 || view.TotalBytesDown != 9 {
		t.Fatalf("bytes after more traffic = %+v", view)
	}

	s.Pause(1)
	*clock = day.Add(24 * time.Hour)
	s.Reconnect(1)
	view = mustView(t, s, 1)
	if view.ReconnectsToday != 1 || view.ReconnectsTotal != 3 {
		t.Fatalf("next day reconnects = %d/%d, want 1/3", view.ReconnectsToday, view.ReconnectsTotal)
	}
	if view.TodayConnectedSec != 0 {
		t.Fatalf("new day connected time = %d, want 0", view.TodayConnectedSec)
	}
}

func TestCounterRestartDoesNotSubtract(t *testing.T) {
	s, clock := newTestStore(t)
	*clock = time.Date(2026, 10, 2, 8, 0, 0, 0, time.Local)
	s.Begin(1)
	s.Observe(1, 100, 40)
	s.Observe(1, 5, 1)
	view := mustView(t, s, 1)
	if view.TotalBytesUp != 105 || view.TotalBytesDown != 41 {
		t.Fatalf("total after counter restart = %d/%d, want 105/41", view.TotalBytesUp, view.TotalBytesDown)
	}
}

func TestPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	configPath := filepath.Join(dir, "config.toml")
	const configBody = "version = 1\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := PathBeside(configPath); got != path {
		t.Fatalf("PathBeside = %s, want %s", got, path)
	}

	s, clock := openAt(t, path)
	start := time.Date(2026, 10, 2, 9, 0, 0, 0, time.Local)
	*clock = start
	s.Begin(7)
	*clock = start.Add(3 * time.Minute)
	s.Observe(7, 1000, 2000)
	s.NoteError(7, "connection refused")
	s.Pause(7)
	s.Reconnect(7)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != configBody {
		t.Fatalf("config.toml changed:\n%s", body)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(saved) {
		t.Fatalf("stats.json is not json: %s", saved)
	}

	again, later := openAt(t, path)
	*later = start.Add(3 * time.Minute)
	view := mustView(t, again, 7)
	if view.Connected {
		t.Fatal("reopened store resurrected a session")
	}
	if view.TotalBytesUp != 1000 || view.TotalBytesDown != 2000 {
		t.Fatalf("bytes = %d/%d", view.TotalBytesUp, view.TotalBytesDown)
	}
	if view.TotalConnectedSec != 180 || view.TodayConnectedSec != 180 {
		t.Fatalf("connected = %d/%d, want 180", view.TodayConnectedSec, view.TotalConnectedSec)
	}
	if view.ReconnectsTotal != 1 || view.ReconnectsToday != 1 {
		t.Fatalf("reconnects = %d/%d", view.ReconnectsToday, view.ReconnectsTotal)
	}
	if view.LastError != "connection refused" || view.LastErrorUnix == 0 {
		t.Fatalf("last error = %q at %d", view.LastError, view.LastErrorUnix)
	}
}

func TestCorruptAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	missing := Open(filepath.Join(dir, "missing.json"))
	if len(missing.Snapshot()) != 0 {
		t.Fatal("missing file should start empty")
	}

	path := filepath.Join(dir, "stats.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, clock := openAt(t, path)
	if len(s.Snapshot()) != 0 {
		t.Fatal("corrupt file should start empty")
	}
	*clock = time.Date(2026, 10, 2, 11, 0, 0, 0, time.Local)
	s.Begin(3)
	s.Observe(3, 8, 9)
	s.NoteError(3, "reset by peer")
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(body) {
		t.Fatalf("flush left corrupt json: %s", body)
	}
	view := mustView(t, s, 3)
	if view.TotalBytesUp != 8 || view.LastError != "reset by peer" {
		t.Fatalf("counters after corrupt open = %+v", view)
	}
}

func TestResetClearsOneTunnel(t *testing.T) {
	s, clock := newTestStore(t)
	start := time.Date(2026, 10, 2, 15, 4, 0, 0, time.Local)
	*clock = start
	s.Begin(1)
	s.Begin(2)
	*clock = start.Add(time.Minute)
	s.Observe(1, 30, 40)
	s.Observe(2, 5, 6)
	s.NoteError(1, "timeout")
	s.Reconnect(1)

	s.Reset(1)
	cleared := mustView(t, s, 1)
	if cleared.TotalBytesUp != 0 || cleared.TotalBytesDown != 0 || cleared.ReconnectsTotal != 0 || cleared.LastError != "" {
		t.Fatalf("reset left data: %+v", cleared)
	}
	if !cleared.Connected || cleared.SessionStartedUnix != clock.UnixMilli() {
		t.Fatalf("running tunnel should keep a fresh session: %+v", cleared)
	}
	if cleared.SessionBytesUp != 0 {
		t.Fatalf("session bytes after reset = %d", cleared.SessionBytesUp)
	}
	other := mustView(t, s, 2)
	if other.TotalBytesUp != 5 || other.TotalConnectedSec != 60 {
		t.Fatalf("reset touched another tunnel: %+v", other)
	}

	*clock = start.Add(2 * time.Minute)
	s.Observe(1, 34, 41)
	cleared = mustView(t, s, 1)
	if cleared.SessionBytesUp != 4 || cleared.TotalBytesUp != 4 || cleared.TodayConnectedSec != 60 {
		t.Fatalf("post-reset accumulation = %+v", cleared)
	}

	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded fileData
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	saved := decoded.Tunnels["1"]
	if saved.BytesUp != 4 || saved.Reconnects != 0 || saved.LastError != "" {
		t.Fatalf("tunnel 1 on disk = %+v", saved)
	}
	if decoded.Tunnels["2"].BytesUp != 5 {
		t.Fatalf("tunnel 2 on disk = %+v", decoded.Tunnels["2"])
	}
}

func TestDebounceWritesAtMostOncePerWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	s := Open(path)
	s.writeDelay = time.Hour
	clock := time.Date(2026, 10, 2, 7, 0, 0, 0, time.Local)
	s.now = func() time.Time { return clock }

	s.Begin(1)
	s.mu.Lock()
	pending := s.timer
	s.mu.Unlock()
	if pending == nil {
		t.Fatal("begin did not schedule a write")
	}
	s.Observe(1, 10, 1)
	s.NoteError(1, "nope")
	s.mu.Lock()
	same := s.timer == pending
	s.mu.Unlock()
	if !same {
		t.Fatal("a second change reset the debounce timer")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("stats were written before the debounce elapsed")
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestLastErrorSurvivesReconnect(t *testing.T) {
	s, clock := newTestStore(t)
	*clock = time.Date(2026, 10, 2, 16, 0, 0, 0, time.Local)
	s.Begin(4)
	s.NoteError(4, "broken pipe")
	s.Pause(4)
	s.Reconnect(4)
	view := mustView(t, s, 4)
	if view.LastError != "broken pipe" || view.ReconnectsTotal != 1 {
		t.Fatalf("error/reconnect = %q / %d", view.LastError, view.ReconnectsTotal)
	}
}

func newTestStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	return openAt(t, filepath.Join(t.TempDir(), "stats.json"))
}

func openAt(t *testing.T, path string) (*Store, *time.Time) {
	t.Helper()
	s := Open(path)
	s.writeDelay = time.Hour
	clock := time.Time{}
	s.now = func() time.Time { return clock }
	t.Cleanup(func() { _ = s.Close() })
	return s, &clock
}

func mustView(t *testing.T, s *Store, id int) View {
	t.Helper()
	for _, view := range s.Snapshot() {
		if view.ID == id {
			return view
		}
	}
	t.Fatalf("tunnel %d missing from snapshot", id)
	return View{}
}
