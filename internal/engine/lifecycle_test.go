package engine

import (
	"context"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/notify"
	"github.com/norka-app/Norka/internal/wake"
)

func TestLifecycleStartAutoStartAndShutdown(t *testing.T) {
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		if err := cfg.Features.Set(features.WakeReconnect, false); err != nil {
			return err
		}
		return cfg.Features.Set(features.Automation, true)
	}); err != nil {
		t.Fatal(err)
	}

	fake := &fakeRuntime{auto: make(chan int, 1), shut: make(chan struct{}, 1)}
	var login []bool
	var watched int
	ln := listenUnix(t)
	closed := make(chan struct{})
	var closeOnce sync.Once

	eng := New(Options{
		Storage: storage,
		Runtime: fake,
		Watch: func(context.Context) (<-chan netwatch.Event, error) {
			watched++
			return nil, context.Canceled
		},
		Listen: func(string) (net.Listener, error) {
			return &onceCloseListener{Listener: ln, done: closed, once: &closeOnce}, nil
		},
		SyncLogin: func(autoRun, _ bool) error {
			login = append(login, autoRun)
			return nil
		},
		SyncScheme: func(bool) error { return nil },
		Poster: func(notify.PosterConfig) notify.Poster {
			return nopPoster{}
		},
	})

	eng.Start()
	if watched != 0 {
		t.Fatalf("wake watch started while the flag is off: %d", watched)
	}
	if len(login) != 1 || login[0] {
		t.Fatalf("login sync = %v, want [false]", login)
	}
	if fake.events == nil {
		t.Fatal("start did not attach tunnel events")
	}
	select {
	case <-fake.auto:
		t.Fatal("start launched tunnels before StartAutoStart")
	case <-time.After(50 * time.Millisecond):
	}

	eng.StartAutoStart()
	select {
	case limit := <-fake.auto:
		if limit != 0 {
			t.Fatalf("autostart limit = %d, want unlimited (0)", limit)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("autostart was not called")
	}

	eng.Shutdown()
	select {
	case <-fake.shut:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not stop tunnels")
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown left the automation listener open")
	}
}

func TestShutdownWithoutStart(t *testing.T) {
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	eng := New(Options{
		Storage:    storage,
		SyncLogin:  func(bool, bool) error { return nil },
		SyncScheme: func(bool) error { return nil },
		Poster:     func(notify.PosterConfig) notify.Poster { return nopPoster{} },
	})
	eng.Shutdown()
}

type fakeRuntime struct {
	mu     sync.Mutex
	events biz.TunnelEvents
	auto   chan int
	shut   chan struct{}
}

func (f *fakeRuntime) SetEvents(events biz.TunnelEvents) {
	f.mu.Lock()
	f.events = events
	f.mu.Unlock()
}

func (f *fakeRuntime) List() ([]model.Tunnel, error) { return nil, nil }

func (f *fakeRuntime) Start(int, int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (f *fakeRuntime) Stop(int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (f *fakeRuntime) Toggle(int, int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (f *fakeRuntime) StartAutoStart(maxRunning int) error {
	f.auto <- maxRunning
	return nil
}

func (f *fakeRuntime) Shutdown() {
	f.shut <- struct{}{}
}

func (f *fakeRuntime) RecoverAfterWake(context.Context, netwatch.Kind, time.Time, wake.Options) wake.Result {
	return wake.Result{}
}

type nopPoster struct{}

func (nopPoster) Post(notify.Notice) error { return nil }

type onceCloseListener struct {
	net.Listener
	done chan struct{}
	once *sync.Once
}

func (l *onceCloseListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.done) })
	return err
}

func listenUnix(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("unix", filepath.Join(t.TempDir(), "norka.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}
