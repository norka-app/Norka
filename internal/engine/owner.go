package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/norka-app/Norka/internal/filelock"
)

const engineLockFile = "engine.lock"

// Kind is who holds engine.lock. The GUI and the future daemon use different values.
type Kind string

const (
	KindGUI    Kind = "gui"
	KindDaemon Kind = "daemon"
)

// OwnerMeta is the JSON written into engine.lock.
// Another process can read it without taking the lock.
type OwnerMeta struct {
	PID       int       `json:"pid"`
	Kind      Kind      `json:"kind"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
}

// ErrOwnedByOther means engine.lock is already held.
// errors.As yields *OwnedByOtherError when the holder's metadata was readable.
var ErrOwnedByOther = errors.New("engine is owned by another process")

// OwnedByOtherError is ErrOwnedByOther plus the holder recorded in engine.lock.
type OwnedByOtherError struct {
	Holder OwnerMeta
}

func (e *OwnedByOtherError) Error() string {
	if e == nil {
		return ErrOwnedByOther.Error()
	}
	return fmt.Sprintf("%s: pid %d, kind %s, version %s", ErrOwnedByOther, e.Holder.PID, e.Holder.Kind, e.Holder.Version)
}

func (e *OwnedByOtherError) Unwrap() error { return ErrOwnedByOther }

// Owner holds engine.lock until Release or until this process exits.
// A crash closes the file descriptor and the kernel frees the lock.
type Owner struct {
	meta     OwnerMeta
	lock     *filelock.Lock
	path     string
	released bool
}

func (o *Owner) Meta() OwnerMeta {
	if o == nil {
		return OwnerMeta{}
	}
	return o.meta
}

// Release drops the lock. It is safe to call more than once.
func (o *Owner) Release() error {
	if o == nil {
		return nil
	}
	liveMu.Lock()
	defer liveMu.Unlock()
	if o.released {
		return nil
	}
	o.released = true
	if live[o.path] == o {
		delete(live, o.path)
	}
	return o.lock.Release()
}

var (
	liveMu sync.Mutex
	// live stops a second Acquire in this process from locking the same file
	// again. On Darwin flock is per process, so that second lock would succeed
	// and we would think two owners existed.
	live = map[string]*Owner{}
)

// Acquire takes engine.lock beside configPath for this process.
// kind is KindGUI or KindDaemon. version is recorded for the other process.
// A busy lock returns ErrOwnedByOther and the holder's metadata.
func Acquire(configPath string, kind Kind, version string) (*Owner, error) {
	if kind != KindGUI && kind != KindDaemon {
		return nil, fmt.Errorf("engine owner kind %q must be gui or daemon", kind)
	}
	lockPath, err := engineLockPath(configPath)
	if err != nil {
		return nil, err
	}

	liveMu.Lock()
	if existing := live[lockPath]; existing != nil {
		meta := existing.meta
		liveMu.Unlock()
		return nil, &OwnedByOtherError{Holder: meta}
	}
	lk, err := filelock.Acquire(lockPath, 0)
	if err != nil {
		liveMu.Unlock()
		if errors.Is(err, filelock.ErrHeld) {
			return nil, ownedByOther(lockPath)
		}
		return nil, err
	}
	meta := OwnerMeta{
		PID:       os.Getpid(),
		Kind:      kind,
		Version:   version,
		StartedAt: time.Now().UTC(),
	}
	if err := writeOwnerFile(lk, meta); err != nil {
		liveMu.Unlock()
		_ = lk.Release()
		return nil, err
	}
	owner := &Owner{meta: meta, lock: lk, path: lockPath}
	live[lockPath] = owner
	liveMu.Unlock()
	return owner, nil
}

// ReadOwner reads engine.lock without taking the lock.
func ReadOwner(configPath string) (OwnerMeta, error) {
	lockPath, err := engineLockPath(configPath)
	if err != nil {
		return OwnerMeta{}, err
	}
	return readOwnerFile(lockPath)
}

func ownedByOther(lockPath string) error {
	holder, err := readOwnerFile(lockPath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOwnedByOther, err)
	}
	return &OwnedByOtherError{Holder: holder}
}

func engineLockPath(configPath string) (string, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return "", fmt.Errorf("config path is empty")
	}
	dir := filepath.Dir(configPath)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, engineLockFile), nil
}

func writeOwnerFile(lk *filelock.Lock, meta OwnerMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return lk.Rewrite(data)
}

func readOwnerFile(path string) (OwnerMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OwnerMeta{}, err
	}
	var meta OwnerMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return OwnerMeta{}, fmt.Errorf("parse engine lock: %w", err)
	}
	return meta, nil
}

// SetVersion is stored in engine.lock when Acquire succeeds.
func (e *Engine) SetVersion(version string) {
	if e == nil {
		return
	}
	e.ownerMu.Lock()
	e.version = strings.TrimSpace(version)
	e.ownerMu.Unlock()
}

// Acquire locks engine.lock beside the config for this process.
// The GUI calls it with KindGUI. A second call in this process returns the same owner.
func (e *Engine) Acquire(kind Kind) (*Owner, error) {
	if e == nil || e.storage == nil {
		return nil, fmt.Errorf("app is not initialized")
	}
	e.ownerMu.Lock()
	defer e.ownerMu.Unlock()
	if e.owner != nil {
		return e.owner, nil
	}
	owner, err := Acquire(e.storage.Path(), kind, e.version)
	if err != nil {
		return nil, err
	}
	e.owner = owner
	return owner, nil
}

// Release drops engine.lock if this process holds it.
func (e *Engine) Release() {
	if e == nil {
		return
	}
	e.ownerMu.Lock()
	owner := e.owner
	e.owner = nil
	e.ownerMu.Unlock()
	if owner != nil {
		_ = owner.Release()
	}
}

// HostWithoutLock hosts tunnels, the wake watcher, and the automation IPC
// server even though this process does not hold engine.lock.
// The GUI uses it when Acquire fails, so a lock problem does not change
// what the user sees. A later step removes it once the window is only a client.
func (e *Engine) HostWithoutLock() {
	if e == nil {
		return
	}
	e.ownerMu.Lock()
	e.hostWithoutLock = true
	e.ownerMu.Unlock()
}

func (e *Engine) hosting() bool {
	if e == nil {
		return false
	}
	e.ownerMu.Lock()
	defer e.ownerMu.Unlock()
	return e.owner != nil || e.hostWithoutLock
}
