package engine

import (
	"context"
	"net"
	"time"

	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/notify"
	"github.com/norka-app/Norka/internal/secrets"
	"github.com/norka-app/Norka/internal/wake"
)

// Runtime is the tunnel surface the engine starts, stops, and notifies.
// *biz.TunnelBiz implements it. Tests pass a fake.
type Runtime interface {
	SetEvents(events biz.TunnelEvents)
	List() ([]model.Tunnel, error)
	Start(id int, maxRunning int) (model.Tunnel, error)
	Stop(id int) (model.Tunnel, error)
	Toggle(id int, maxRunning int) (model.Tunnel, error)
	StartAutoStart(maxRunning int) error
	Shutdown()
	RecoverAfterWake(ctx context.Context, kind netwatch.Kind, now time.Time, opts wake.Options) wake.Result
}

// Options wires an Engine. Zero values use the production defaults:
// an in-memory keychain unless UseSystemKeychain is set, real netwatch,
// and the real automation listener.
type Options struct {
	Storage *conf.Storage
	Vault   *secrets.Vault
	// UseSystemKeychain opens the OS keychain when Vault is nil.
	UseSystemKeychain bool
	// Runtime overrides tunnel lifecycle. The concrete *biz.TunnelBiz is
	// still created so GUI accessors keep working. Nil uses that concrete value.
	Runtime Runtime
	Watch   func(context.Context) (<-chan netwatch.Event, error)
	Listen  func(address string) (net.Listener, error)
	// SyncLogin aligns the OS login entry with config.
	// autoRun is the saved switch, hidden is the autostart_hidden flag.
	// Nil uses autostart.Sync.
	SyncLogin func(autoRun, hidden bool) error
	// SyncScheme registers or removes the norka:// protocol. Nil uses scheme.Sync.
	SyncScheme func(enabled bool) error
	// Poster builds the OS notifier. Nil uses notify.NewSystemPoster.
	Poster     func(notify.PosterConfig) notify.Poster
	Sink       Sink
	Locale     func() string
	NotifyIcon func() string
}
