package conf

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/norka-app/Norka/internal/filelock"
)

var (
	ErrInvalidTOMLConfigurationFile = errors.New("invalid config TOML")
)

// configLockTimeout is how long a read-modify-write waits for another process
// to finish saving config.toml. The lock file is config.toml.lock beside it.
var configLockTimeout = 10 * time.Second

type Storage struct {
	path string
	mu   sync.Mutex
}

func (r *Storage) Path() string {
	return r.path
}

func NewStorage(path string) (*Storage, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	return &Storage{path: trimmed}, nil
}

// MigrateFromLocalConfigIfNeeded runs a one-time migration for users who previously
// used config.toml from the current directory. When the target config path is
// ~/.norka/config.toml and that file does not exist, but ./config.toml exists
// in the process working directory, it copies the local file to the home path and
// renames the local file to config.toml.old. No-op if any condition is not met.
func MigrateFromLocalConfigIfNeeded(targetPath string) error {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil
	}

	homePath := GetHomeConfigPath()
	if homePath == "" {
		return nil
	}

	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return nil
	}
	homeAbs, err := filepath.Abs(homePath)
	if err != nil {
		return nil
	}
	if filepath.Clean(targetAbs) != filepath.Clean(homeAbs) {
		return nil
	}

	if _, err := os.Stat(targetPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check target config exists: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	localPath := filepath.Join(cwd, "config.toml")
	if _, err := os.Stat(localPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("check local config exists: %w", err)
	}
	if localAbs, err := filepath.Abs(localPath); err == nil && filepath.Clean(localAbs) == filepath.Clean(targetAbs) {
		return nil
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(targetPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, PrivateDirPerm); err != nil {
			return fmt.Errorf("create config dir for migration: %w", err)
		}
	}
	if err := os.WriteFile(targetPath, data, PrivateFilePerm); err != nil {
		return fmt.Errorf("write migrated config: %w", err)
	}

	oldPath := localPath + ".old"
	if err := os.Rename(localPath, oldPath); err != nil {
		return fmt.Errorf("rename local config to .old: %w", err)
	}

	return nil
}

func NewDefaultStorage() (*Storage, error) {
	implicitPath := ResolveImplicitConfigPath()
	if err := MigrateFromLocalConfigIfNeeded(implicitPath); err != nil {
		slog.Warn("config migration failed", "target", implicitPath, "error", err)
	}
	effectivePath := ResolveEffectiveConfigPath(implicitPath)
	return NewStorage(effectivePath)
}

func (r *Storage) Load() (*Config, error) {
	var cfg *Config
	err := r.withLock(func() error {
		loaded, err := r.loadLocked()
		if err != nil {
			return err
		}
		cfg = loaded.Clone()
		return nil
	})
	return cfg, err
}

func (r *Storage) Update(mutator func(cfg *Config) error) (*Config, error) {
	var out *Config
	err := r.withLock(func() error {
		// Reload inside the file lock so a concurrent process cannot lose
		// its write between our read and our rename.
		cfg, err := r.loadLocked()
		if err != nil {
			return err
		}
		if err := mutator(cfg); err != nil {
			return err
		}
		cfg.Normalize()
		if err := r.saveLocked(cfg); err != nil {
			return err
		}
		out = cfg.Clone()
		return nil
	})
	return out, err
}

func (r *Storage) lockPath() string {
	return r.path + ".lock"
}

// withLock holds the in-process mutex and the cross-process file lock.
// The mutex stops a mutator from calling back into Update while the lock is held.
func (r *Storage) withLock(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureParentDirLocked(); err != nil {
		return err
	}
	lk, err := filelock.Acquire(r.lockPath(), configLockTimeout)
	if err != nil {
		return fmt.Errorf("lock %s: %w", r.lockPath(), err)
	}
	defer lk.Release()
	return fn()
}

func (r *Storage) loadLocked() (*Config, error) {
	if err := r.ensureParentDirLocked(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		cfg := DefaultConfig()
		if err := r.saveLocked(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		cfg := DefaultConfig()
		if err := r.saveLocked(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg, err := ParseConfigTOML(data)
	if err != nil {
		return nil, err
	}
	cfg.Normalize()
	return cfg, nil
}

func (r *Storage) saveLocked(cfg *Config) error {
	if err := r.ensureParentDirLocked(); err != nil {
		return err
	}

	cfg.Normalize()
	data := encodeConfigTOML(cfg)

	dir := filepath.Dir(r.path)
	if dir == "" {
		dir = "."
	}
	// A unique name so two processes never share one config.toml.tmp.
	tmp, err := os.CreateTemp(dir, ".norka-config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(PrivateFilePerm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, r.path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (r *Storage) ensureParentDirLocked() error {
	dir := filepath.Dir(r.path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, PrivateDirPerm)
}

// ParseConfigTOML parses a TOML configuration from raw bytes.
// Exported so callers (e.g. import validation in app.go) can validate a file
// before replacing the live config.
func ParseConfigTOML(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTOMLConfigurationFile, err)
	}
	cfg.Normalize()
	return cfg, nil
}

// MarshalTOML encodes a config the same way Storage saves it.
func MarshalTOML(cfg *Config) []byte {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return encodeConfigTOML(cfg.Clone())
}

func encodeConfigTOML(cfg *Config) []byte {
	cfg.Normalize()
	var buf strings.Builder
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(cfg); err != nil {
		// This should generally not happen with our Config struct
		return []byte("")
	}
	return []byte(buf.String())
}
