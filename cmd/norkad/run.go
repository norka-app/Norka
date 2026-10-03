package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/engine"
	"github.com/norka-app/Norka/internal/features"
)

const (
	exitOK            = 0
	exitError         = 1
	exitUsage         = 2
	exitOwned         = 9
	exitBackgroundOff = 10
)

const backgroundOffMessage = "norkad: background mode is off. Turn on «Фоновый режим» (Background mode) in Settings → Features. Until a later update the window itself does not stay in the background; this switch only lets norkad run. Pass --force to start anyway."

// env is the process surface tests replace. Wait blocks until norkad should stop.
type env struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
	Wait    func()
}

func realEnv() env {
	return env{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
		Wait:    waitForStop,
	}
}

type options struct {
	ConfigPath string
	Foreground bool
	Force      bool
	Version    bool
}

func run(args []string, e env) int {
	if e.Stdout == nil {
		e.Stdout = io.Discard
	}
	if e.Stderr == nil {
		e.Stderr = io.Discard
	}
	if e.Wait == nil {
		e.Wait = waitForStop
	}
	if strings.TrimSpace(e.Version) == "" {
		e.Version = "dev"
	}

	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			fmt.Fprint(e.Stdout, usageText())
			return exitOK
		}
	}

	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(e.Stderr, err.Error())
		fmt.Fprint(e.Stderr, usageText())
		return exitUsage
	}
	if opts.Version {
		fmt.Fprintln(e.Stdout, "norkad "+e.Version)
		return exitOK
	}

	storage, err := openStorage(opts.ConfigPath)
	if err != nil {
		fmt.Fprintf(e.Stderr, "norkad: open config: %v\n", err)
		return exitError
	}
	cfg, err := storage.Load()
	if err != nil {
		fmt.Fprintf(e.Stderr, "norkad: read config: %v\n", err)
		return exitError
	}
	if !backgroundAllowed(cfg, opts.Force) {
		fmt.Fprintln(e.Stderr, backgroundOffMessage)
		return exitBackgroundOff
	}

	if err := setupLog(storage.Path(), opts.Foreground); err != nil {
		fmt.Fprintf(e.Stderr, "norkad: log: %v\n", err)
		return exitError
	}

	return serve(e, storage)
}

func serve(e env, storage *conf.Storage) int {
	eng := engine.New(engine.Options{Storage: storage, UseSystemKeychain: true})
	if err := eng.Ready(); err != nil {
		fmt.Fprintf(e.Stderr, "norkad: %v\n", err)
		return exitError
	}
	eng.SetAutoStartSkip(skipReason)
	eng.SetVersion(e.Version)
	if _, err := eng.Acquire(engine.KindDaemon); err != nil {
		var held *engine.OwnedByOtherError
		if errors.As(err, &held) || errors.Is(err, engine.ErrOwnedByOther) {
			msg := ownedMessage(err)
			fmt.Fprintln(e.Stderr, msg)
			slog.Warn(msg)
			return exitOwned
		}
		fmt.Fprintf(e.Stderr, "norkad: engine lock: %v\n", err)
		return exitError
	}

	eng.Start()
	eng.StartAutoStart()
	slog.SetDefault(slog.New(eng.LogHandler(slog.Default().Handler())))
	logReady(storage.Path())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.NotifyStop(cancel)
	go func() {
		e.Wait()
		cancel()
	}()
	<-ctx.Done()
	defer finishStop()
	logStopping()
	eng.Shutdown()
	logStopped()
	return exitOK
}

func backgroundAllowed(cfg *conf.Config, force bool) bool {
	if force {
		return true
	}
	if cfg == nil {
		return false
	}
	return cfg.Features.Enabled(features.BackgroundMode)
}

func ownedMessage(err error) string {
	var held *engine.OwnedByOtherError
	if errors.As(err, &held) {
		kind := strings.TrimSpace(string(held.Holder.Kind))
		if kind == "" {
			kind = "unknown"
		}
		return fmt.Sprintf("norkad: engine is owned by pid %d (%s)", held.Holder.PID, kind)
	}
	return "norkad: engine is owned by another process"
}

func openStorage(configPath string) (*conf.Storage, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath != "" {
		return conf.NewStorage(configPath)
	}
	return conf.NewDefaultStorage()
}

func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--version":
			opts.Version = true
		case arg == "--foreground":
			opts.Foreground = true
		case arg == "--force":
			opts.Force = true
		case arg == "--config":
			if i+1 >= len(args) {
				return options{}, errors.New("norkad: --config needs a path")
			}
			i++
			opts.ConfigPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			opts.ConfigPath = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-"):
			return options{}, fmt.Errorf("norkad: unknown flag %q", arg)
		default:
			return options{}, fmt.Errorf("norkad: unexpected argument %q", arg)
		}
	}
	return opts, nil
}

func usageText() string {
	return `norkad runs Norka tunnels without the window.

It reads the same config.toml as the Norka window, takes the engine lock
as kind daemon, and starts autostart tunnels, wake reconnect, and automation
IPC. Keys and ssh-agent only: a tunnel that needs a password from the GUI
keychain prompt is logged and skipped.

A dialect 2 client can ask this process to stop, or to drop the engine lock
after the tunnels stop so the caller can take it. The wire format is docs/IPC.md.

  norkad [--config PATH] [--foreground] [--force] [--version]

  --config PATH   config.toml (default: the same path the window uses)
  --foreground    write the log to stderr instead of norkad.log
  --force         run even when background mode is off
  --version       print the version and exit

Exit codes:
  0   stopped after a signal, tunnels closed
  2   invalid flags
  9   another process holds the engine (the message includes its pid and kind)
  10  background mode is off
  1   any other error
`
}
