package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/ipc"
)

// command is one parsed invocation. Args do not include the program name.
type command struct {
	Op      string
	Target  string
	JSON    bool
	Help    bool
	Version bool
}

// env is what the client needs from the machine. Tests replace the functions.
type env struct {
	Stdout      io.Writer
	Stderr      io.Writer
	Version     string
	Load        func() (*conf.Config, string, error)
	Address     func(configPath string) (string, error)
	TokenPath   func(configPath string) string
	ReadToken   func(path string) (string, error)
	Call        func(ctx context.Context, address, token string, req ipc.Request) (ipc.Response, error)
	ReadAppPath func(configPath string) (string, error)
	Launch      func(exe string) error
	Wait        func(ctx context.Context, address, tokenPath string) error
}

func realEnv(version string) env {
	return env{
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Version:     version,
		Load:        loadConfig,
		Address:     ipc.Address,
		TokenPath:   ipc.TokenPath,
		ReadToken:   ipc.ReadToken,
		Call:        ipc.Call,
		ReadAppPath: conf.ReadAppPath,
		Launch:      startHidden,
		Wait:        waitReady,
	}
}

func run(args []string, e env) int {
	cmd, err := parse(args)
	if cmd.Version && err == nil {
		writeHuman(e.Stdout, versionLine(e.Version))
		return ipc.ExitOK
	}
	if err != nil || cmd.Help {
		out := e.Stderr
		code := ipc.ExitUsage
		if cmd.Help && err == nil {
			out = e.Stdout
			code = ipc.ExitOK
		}
		if err != nil {
			writeHuman(out, err.Error())
		}
		writeHuman(out, usageText())
		return code
	}

	cfg, path, err := e.Load()
	if err != nil {
		return fail(e, cmd, ipc.ExitIPC, "ipc", err.Error())
	}
	if cfg == nil || !cfg.Features.Enabled(features.Automation) {
		return fail(e, cmd, ipc.ExitDisabled, ipc.CodeDisabled, "")
	}

	resp, err := invoke(e, cmd, path)
	if errors.Is(err, errAppUnavailable) {
		return fail(e, cmd, ipc.ExitNotRunning, ipc.CodeNotRunning, publicError(err))
	}
	if errors.Is(err, ipc.ErrNotRunning) {
		return fail(e, cmd, ipc.ExitNotRunning, ipc.CodeNotRunning, "")
	}
	if err != nil {
		return fail(e, cmd, ipc.ExitIPC, "ipc", err.Error())
	}
	if rejected, ok := outdated(resp); ok {
		return finish(e, cmd, rejected)
	}
	if resp.ExitCode == 0 && resp.OK {
		resp.ExitCode = ipc.ExitOK
	}
	return finish(e, cmd, resp)
}

func invoke(e env, cmd command, configPath string) (ipc.Response, error) {
	if !mutates(cmd.Op) {
		return call(e, configPath, request(cmd), 8*time.Second)
	}
	// Ask the running app which dialect it speaks before connect, disconnect,
	// or toggle. An older app would otherwise run the command and ignore v.
	probe, err := call(e, configPath, ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpStatus}, 8*time.Second)
	if errors.Is(err, ipc.ErrNotRunning) && cmd.Op == ipc.OpConnect {
		if err := startApp(e, configPath); err != nil {
			return ipc.Response{}, err
		}
		probe, err = call(e, configPath, ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpStatus}, 60*time.Second)
	}
	if err != nil {
		return ipc.Response{}, err
	}
	if _, stale := outdated(probe); stale || !probe.OK {
		return probe, nil
	}
	return call(e, configPath, request(cmd), 60*time.Second)
}

func request(cmd command) ipc.Request {
	return ipc.Request{V: ipc.ProtocolVersion, Op: cmd.Op, Target: cmd.Target}
}

func mutates(op string) bool {
	return op == ipc.OpConnect || op == ipc.OpDisconnect || op == ipc.OpToggle
}

func outdated(resp ipc.Response) (ipc.Response, bool) {
	if resp.Code == ipc.CodeUpdateApp || resp.Code == ipc.CodeUpdateCLI {
		if resp.ExitCode == 0 {
			resp.ExitCode = ipc.ExitVersion
		}
		return resp, true
	}
	if resp.V == 0 {
		return ipc.Response{
			Code:     ipc.CodeUpdateApp,
			ExitCode: ipc.ExitVersion,
			Message:  ipc.MessageUpdateApp,
			Running:  true,
		}, true
	}
	return ipc.Response{}, false
}

func startApp(e env, configPath string) error {
	if e.ReadAppPath == nil || e.Launch == nil || e.Wait == nil {
		return ipc.ErrNotRunning
	}
	exe, err := e.ReadAppPath(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return unavailable("Norka is not running, and norka-cli does not know where it is installed. Start Norka once, then try again.")
		}
		return unavailable("Norka is not running, and the saved application path cannot be used: " + err.Error())
	}
	if err := e.Launch(exe); err != nil {
		return fmt.Errorf("could not start Norka: %w", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := e.Wait(waitCtx, mustAddress(e, configPath), e.TokenPath(configPath)); err != nil {
		return ipc.ErrNotRunning
	}
	return nil
}

// errAppUnavailable marks a failure to find or use the saved Norka executable.
var errAppUnavailable = errors.New("norka app unavailable")

func unavailable(message string) error {
	return fmt.Errorf("%w: %s", errAppUnavailable, message)
}

func publicError(err error) string {
	prefix := errAppUnavailable.Error() + ": "
	return strings.TrimPrefix(err.Error(), prefix)
}

func call(e env, configPath string, req ipc.Request, timeout time.Duration) (ipc.Response, error) {
	address, err := e.Address(configPath)
	if err != nil {
		return ipc.Response{}, err
	}
	token, err := e.ReadToken(e.TokenPath(configPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ipc.Response{}, ipc.ErrNotRunning
		}
		return ipc.Response{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return e.Call(ctx, address, token, req)
}

func mustAddress(e env, configPath string) string {
	address, err := e.Address(configPath)
	if err != nil {
		return ""
	}
	return address
}

func finish(e env, cmd command, resp ipc.Response) int {
	code := resp.ExitCode
	if code == 0 && !resp.OK {
		code = ipc.ExitFailed
	}
	if cmd.JSON {
		if err := writeJSON(e.Stdout, resp); err != nil {
			return ipc.ExitIPC
		}
		return code
	}
	text := explain(cmd, resp)
	out := e.Stdout
	if code != 0 {
		out = e.Stderr
	}
	writeHuman(out, text)
	if code == 0 && out != nil {
		if table := tunnelTable(resp.Tunnels); table != "" {
			_, _ = io.WriteString(out, "\n")
			_, _ = io.WriteString(out, table)
		}
	}
	return code
}

func fail(e env, cmd command, code int, name, message string) int {
	resp := ipc.Response{
		OK:       false,
		Running:  false,
		ExitCode: code,
		Code:     name,
		Message:  message,
	}
	if message == "" {
		resp.Message = explain(cmd, resp)
	}
	return finish(e, cmd, resp)
}

func parse(args []string) (command, error) {
	var cmd command
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--json":
			cmd.JSON = true
		case "--version":
			cmd.Version = true
		case "--help", "-h", "help":
			cmd.Help = true
		default:
			if strings.HasPrefix(arg, "-") {
				return command{}, fmt.Errorf("unknown flag %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if cmd.Version || cmd.Help {
		return cmd, nil
	}
	if len(positional) == 0 {
		return command{}, errors.New("missing command")
	}
	cmd.Op = positional[0]
	rest := positional[1:]
	switch cmd.Op {
	case "status", "list":
		if len(rest) != 0 {
			return command{}, fmt.Errorf("%s does not take a tunnel name", cmd.Op)
		}
	case "connect", "disconnect", "toggle":
		if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
			return command{}, fmt.Errorf("%s needs one tunnel name or id", cmd.Op)
		}
		cmd.Target = rest[0]
	default:
		return command{}, fmt.Errorf("unknown command %q", cmd.Op)
	}
	return cmd, nil
}

func versionLine(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "dev"
	}
	return "norka-cli " + version
}

func loadConfig() (*conf.Config, string, error) {
	path := conf.ResolveConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, "", err
		}
		cfg := conf.DefaultConfig()
		cfg.Normalize()
		return cfg, path, nil
	}
	cfg, err := conf.ParseConfigTOML(data)
	if err != nil {
		return nil, "", err
	}
	return cfg, path, nil
}

func waitReady(ctx context.Context, address, tokenPath string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var last error
	for {
		token, err := ipc.ReadToken(tokenPath)
		if err == nil {
			dialCtx, cancel := context.WithTimeout(ctx, time.Second)
			_, err = ipc.Call(dialCtx, address, token, ipc.Request{Op: ipc.OpStatus})
			cancel()
			if err == nil {
				return nil
			}
			last = err
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			if last != nil {
				return last
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
