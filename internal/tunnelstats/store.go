// Package tunnelstats keeps per-tunnel counters outside config.toml.
//
// Totals live in a compact stats.json next to the config file. Writes are
// debounced (at most one every writeDelay, 30s by default) and flushed on
// Close. A missing or corrupt file starts from empty counters.
package tunnelstats

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const fileName = "stats.json"

// PathBeside returns stats.json in the same directory as the config file.
func PathBeside(configPath string) string {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return fileName
	}
	return filepath.Join(filepath.Dir(configPath), fileName)
}

// View is one tunnel's counters for the UI. Times are whole seconds.
// SessionStartedUnix and AsOfUnix are Unix milliseconds; zero means "not connected".
type View struct {
	ID                 int    `json:"id"`
	Connected          bool   `json:"connected"`
	SessionStartedUnix int64  `json:"sessionStartedUnix,omitempty"`
	AsOfUnix           int64  `json:"asOfUnix"`
	TodayConnectedSec  int64  `json:"todayConnectedSec"`
	TotalConnectedSec  int64  `json:"totalConnectedSec"`
	ReconnectsToday    int    `json:"reconnectsToday"`
	ReconnectsTotal    int    `json:"reconnectsTotal"`
	LastError          string `json:"lastError,omitempty"`
	LastErrorUnix      int64  `json:"lastErrorUnix,omitempty"`
	SessionBytesUp     uint64 `json:"sessionBytesUp"`
	SessionBytesDown   uint64 `json:"sessionBytesDown"`
	TotalBytesUp       uint64 `json:"totalBytesUp"`
	TotalBytesDown     uint64 `json:"totalBytesDown"`
}

// persisted is the on-disk record. Keys stay short so the file stays small.
type persisted struct {
	BytesUp         uint64 `json:"u,omitempty"`
	BytesDown       uint64 `json:"d,omitempty"`
	ConnectedNs     int64  `json:"c,omitempty"`
	Day             string `json:"day,omitempty"`
	TodayNs         int64  `json:"tc,omitempty"`
	Reconnects      int    `json:"r,omitempty"`
	ReconnectsToday int    `json:"rt,omitempty"`
	LastError       string `json:"e,omitempty"`
	LastErrorUnix   int64  `json:"et,omitempty"`
}

type fileData struct {
	Version int                  `json:"v"`
	Tunnels map[string]persisted `json:"tunnels,omitempty"`
}

// session is the open connection. It is not written as-is: elapsed time and
// byte deltas are folded into persisted before a write.
type session struct {
	started        time.Time
	accountedUntil time.Time
	markUp         uint64
	markDown       uint64
	seenUp         uint64
	seenDown       uint64
}

// Store accumulates counters and persists them.
type Store struct {
	path        string
	writeDelay  time.Duration
	now         func() time.Time
	mu          sync.Mutex
	data        fileData
	live        map[int]*session
	dirty       bool
	timer       *time.Timer
	closed      bool
	beforeFlush func()
}

// Open reads stats.json. A missing or unreadable file yields an empty store.
func Open(path string) *Store {
	s := &Store{
		path:       path,
		writeDelay: 30 * time.Second,
		now:        time.Now,
		data:       fileData{Version: 1, Tunnels: map[string]persisted{}},
		live:       map[int]*session{},
	}
	s.load()
	return s
}

// SetBeforeFlush runs fn outside the store lock just before a write, so the
// caller can fold the latest traffic counters in.
func (s *Store) SetBeforeFlush(fn func()) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.beforeFlush = fn
	s.mu.Unlock()
}

// Begin starts a connected session. Uptime counts from now.
func (s *Store) Begin(id int) {
	if s == nil || id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if _, ok := s.live[id]; ok {
		s.endLocked(id)
	}
	now := s.now()
	s.live[id] = &session{started: now, accountedUntil: now}
	s.scheduleLocked()
}

// Pause folds connected time and stops the uptime clock. Byte counters stay,
// so a later reconnect can keep the same forwarder totals.
func (s *Store) Pause(id int) {
	if s == nil || id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	sess := s.live[id]
	if sess == nil || sess.started.IsZero() {
		return
	}
	s.foldTimeLocked(id, s.now())
	sess.started = time.Time{}
	sess.accountedUntil = time.Time{}
	s.scheduleLocked()
}

// Reconnect counts one successful reconnect and starts a new uptime session.
// Bytes already seen stay in the total; the session counter restarts.
func (s *Store) Reconnect(id int) {
	if s == nil || id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	now := s.now()
	s.foldTimeLocked(id, now)
	rec := s.must(id)
	s.rollDay(&rec, localDay(now))
	rec.Reconnects++
	rec.ReconnectsToday++
	s.put(id, rec)
	sess := s.live[id]
	if sess == nil {
		sess = &session{}
		s.live[id] = sess
	}
	sess.markUp = sess.seenUp
	sess.markDown = sess.seenDown
	sess.started = now
	sess.accountedUntil = now
	s.dirty = true
	s.scheduleLocked()
}

// End folds the open session and drops it. Totals stay on disk.
func (s *Store) End(id int) {
	if s == nil || id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.endLocked(id)
}

// Observe reports the forwarder's absolute counters for this run.
// Deltas since the previous call are added to the total. A drop in the
// counter (a new forwarder) is treated as a fresh zero, not a negative delta.
func (s *Store) Observe(id int, up, down uint64) {
	if s == nil || id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	sess := s.live[id]
	if sess == nil {
		return
	}
	up, down = acceptCounter(sess, up, down)
	deltaUp := up - sess.seenUp
	deltaDown := down - sess.seenDown
	sess.seenUp = up
	sess.seenDown = down
	if deltaUp == 0 && deltaDown == 0 {
		return
	}
	rec := s.must(id)
	rec.BytesUp += deltaUp
	rec.BytesDown += deltaDown
	s.put(id, rec)
	s.dirty = true
	s.scheduleLocked()
}

// NoteError records the latest error and when it happened. A later success
// does not clear it; Reset does.
func (s *Store) NoteError(id int, message string) {
	if s == nil || id <= 0 {
		return
	}
	message = clip(message)
	if message == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	rec := s.must(id)
	rec.LastError = message
	rec.LastErrorUnix = s.now().Unix()
	s.put(id, rec)
	s.dirty = true
	s.scheduleLocked()
}

// Reset clears saved and session counters for one tunnel. A tunnel that is
// still connected starts a new session from zero.
func (s *Store) Reset(id int) {
	if s == nil || id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	delete(s.data.Tunnels, strconv.Itoa(id))
	if sess := s.live[id]; sess != nil {
		now := s.now()
		sess.started = now
		sess.accountedUntil = now
		sess.markUp = sess.seenUp
		sess.markDown = sess.seenDown
	}
	s.dirty = true
	s.scheduleLocked()
}

// Forget drops a tunnel entirely, including an open session.
func (s *Store) Forget(id int) {
	if s == nil || id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	delete(s.live, id)
	if _, ok := s.data.Tunnels[strconv.Itoa(id)]; !ok && !s.dirty {
		return
	}
	delete(s.data.Tunnels, strconv.Itoa(id))
	s.dirty = true
	s.scheduleLocked()
}

// Snapshot returns every tunnel that has saved data or an open session.
func (s *Store) Snapshot() []View {
	if s == nil {
		return []View{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return []View{}
	}
	now := s.now()
	ids := s.idsLocked()
	out := make([]View, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.viewLocked(id, now))
	}
	return out
}

// Flush writes immediately when something changed. Shutdown uses this.
func (s *Store) Flush() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	hook := s.beforeFlush
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	return s.flushLocked()
}

// Close stops the debounce timer and writes pending totals.
// The before-flush hook still runs, so the latest byte counters are included.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	hook := s.beforeFlush
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	return s.flushLocked()
}

func (s *Store) load() {
	if strings.TrimSpace(s.path) == "" {
		return
	}
	body, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("tunnel stats ignored", "path", s.path, "err", err)
		}
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return
	}
	var decoded fileData
	if err := json.Unmarshal(body, &decoded); err != nil {
		slog.Warn("tunnel stats ignored", "path", s.path, "err", err)
		return
	}
	if decoded.Tunnels == nil {
		decoded.Tunnels = map[string]persisted{}
	}
	decoded.Version = 1
	s.data = decoded
}

func (s *Store) endLocked(id int) {
	if sess := s.live[id]; sess != nil && !sess.started.IsZero() {
		s.foldTimeLocked(id, s.now())
	}
	delete(s.live, id)
	s.scheduleLocked()
}

func (s *Store) foldTimeLocked(id int, now time.Time) {
	sess := s.live[id]
	if sess == nil || sess.started.IsZero() || !now.After(sess.accountedUntil) {
		return
	}
	rec := s.must(id)
	from := sess.accountedUntil
	for from.Before(now) {
		next := startOfNextLocalDay(from)
		if !next.After(from) {
			break
		}
		to := next
		if to.After(now) {
			to = now
		}
		if !to.After(from) {
			break
		}
		delta := to.Sub(from)
		rec.ConnectedNs += int64(delta)
		s.rollDay(&rec, localDay(from))
		if rec.Day == localDay(from) {
			rec.TodayNs += int64(delta)
		}
		from = to
	}
	sess.accountedUntil = now
	s.put(id, rec)
	s.dirty = true
}

func (s *Store) viewLocked(id int, now time.Time) View {
	if sess := s.live[id]; sess != nil && !sess.started.IsZero() {
		s.foldTimeLocked(id, now)
	}
	rec, ok := s.data.Tunnels[strconv.Itoa(id)]
	if ok {
		day := localDay(now)
		if rec.Day != day {
			s.rollDay(&rec, day)
			s.put(id, rec)
			s.dirty = true
			s.scheduleLocked()
		}
	}
	view := View{ID: id, AsOfUnix: now.UnixMilli()}
	if ok {
		view.TodayConnectedSec = rec.TodayNs / int64(time.Second)
		view.TotalConnectedSec = rec.ConnectedNs / int64(time.Second)
		view.ReconnectsToday = rec.ReconnectsToday
		view.ReconnectsTotal = rec.Reconnects
		view.LastError = rec.LastError
		view.LastErrorUnix = rec.LastErrorUnix
		view.TotalBytesUp = rec.BytesUp
		view.TotalBytesDown = rec.BytesDown
	}
	sess := s.live[id]
	if sess != nil && !sess.started.IsZero() {
		view.Connected = true
		view.SessionStartedUnix = sess.started.UnixMilli()
		if sess.seenUp >= sess.markUp {
			view.SessionBytesUp = sess.seenUp - sess.markUp
		}
		if sess.seenDown >= sess.markDown {
			view.SessionBytesDown = sess.seenDown - sess.markDown
		}
	}
	if s.dirty {
		s.scheduleLocked()
	}
	return view
}

func (s *Store) idsLocked() []int {
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(s.data.Tunnels)+len(s.live))
	for key := range s.data.Tunnels {
		id, err := strconv.Atoi(key)
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for id := range s.live {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sortInts(ids)
	return ids
}

func (s *Store) must(id int) persisted {
	if s.data.Tunnels == nil {
		s.data.Tunnels = map[string]persisted{}
	}
	return s.data.Tunnels[strconv.Itoa(id)]
}

func (s *Store) put(id int, rec persisted) {
	if s.data.Tunnels == nil {
		s.data.Tunnels = map[string]persisted{}
	}
	key := strconv.Itoa(id)
	if rec == (persisted{}) {
		delete(s.data.Tunnels, key)
		return
	}
	s.data.Tunnels[key] = rec
}

func (s *Store) rollDay(rec *persisted, day string) {
	if rec.Day == day {
		return
	}
	rec.Day = day
	rec.TodayNs = 0
	rec.ReconnectsToday = 0
}

func (s *Store) scheduleLocked() {
	if s.closed || s.timer != nil {
		return
	}
	if !s.dirty && !s.hasOpenSessionLocked() {
		return
	}
	delay := s.writeDelay
	if delay <= 0 {
		delay = 30 * time.Second
	}
	s.timer = time.AfterFunc(delay, func() {
		_ = s.Flush()
	})
}

func (s *Store) flushLocked() error {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	now := s.now()
	for id, sess := range s.live {
		if sess != nil && !sess.started.IsZero() {
			s.foldTimeLocked(id, now)
		}
	}
	if s.dirty {
		if err := s.writeLocked(); err != nil {
			if !s.closed {
				s.scheduleLocked()
			}
			return err
		}
		s.dirty = false
	}
	if !s.closed && s.hasOpenSessionLocked() {
		s.scheduleLocked()
	}
	return nil
}

func (s *Store) hasOpenSessionLocked() bool {
	for _, sess := range s.live {
		if sess != nil && !sess.started.IsZero() {
			return true
		}
	}
	return false
}

func (s *Store) writeLocked() error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	payload := fileData{Version: 1, Tunnels: s.data.Tunnels}
	if payload.Tunnels == nil {
		payload.Tunnels = map[string]persisted{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeAtomic(s.path, body)
}

func acceptCounter(sess *session, up, down uint64) (uint64, uint64) {
	if up < sess.seenUp {
		sess.seenUp = 0
		if sess.markUp > up {
			sess.markUp = 0
		}
	}
	if down < sess.seenDown {
		sess.seenDown = 0
		if sess.markDown > down {
			sess.markDown = 0
		}
	}
	return up, down
}

func writeAtomic(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".stats-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmpName, path); err2 != nil {
			return err
		}
	}
	ok = true
	return nil
}

func localDay(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02")
}

func startOfNextLocalDay(t time.Time) time.Time {
	local := t.In(time.Local)
	year, month, day := local.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, time.Local)
}

func clip(message string) string {
	message = strings.TrimSpace(message)
	runes := []rune(message)
	const maxRunes = 400
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return message
}

func sortInts(ids []int) {
	for i := 1; i < len(ids); i++ {
		j := i
		for j > 0 && ids[j] < ids[j-1] {
			ids[j], ids[j-1] = ids[j-1], ids[j]
			j--
		}
	}
}
