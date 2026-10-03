package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/norka-app/Norka/internal/automation"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/ipc"
)

// Env is what the CLI needs from the machine. Tests replace the functions.
type Env struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Load      func() (cfg *conf.Config, path, locale string, err error)
	Address   func(configPath string) (string, error)
	TokenPath func(configPath string) string
	ReadToken func(path string) (string, error)
	Call      func(ctx context.Context, address, token string, req ipc.Request) (ipc.Response, error)
	Launch    func() error
	Wait      func(ctx context.Context, address, tokenPath string) error
}

// Run executes one CLI invocation. args do not include the program name.
func Run(env Env, args []string) int {
	locale := "en"
	if env.Load != nil {
		if cfg, _, loaded, err := env.Load(); err == nil {
			locale = loaded
			_ = cfg
		}
	}
	cmd, err := Parse(args)
	if err != nil || cmd.Help {
		out := env.Stderr
		code := ipc.ExitUsage
		if cmd.Help && err == nil {
			out = env.Stdout
			code = ipc.ExitOK
		}
		fmtUsage(out, locale, err)
		return code
	}
	cfg, path, loaded, err := env.Load()
	if err != nil {
		return fail(env, cmd.JSON, locale, ipc.ExitIPC, err.Error())
	}
	if loaded != "" {
		locale = loaded
	}
	if cfg == nil || !cfg.Features.Enabled(features.Automation) {
		return fail(env, cmd.JSON, locale, ipc.ExitDisabled, automation.Message(locale, ipc.CodeDisabled, "", "", nil))
	}

	req := ipc.Request{Op: cmd.Op, Target: cmd.Target}
	resp, err := call(env, path, req, 8*time.Second)
	if errors.Is(err, ipc.ErrNotRunning) && cmd.Op == ipc.OpConnect {
		if env.Launch == nil || env.Wait == nil {
			return fail(env, cmd.JSON, locale, ipc.ExitNotRunning, automation.Message(locale, ipc.CodeNotRunning, "", "", nil))
		}
		if err := env.Launch(); err != nil {
			return fail(env, cmd.JSON, locale, ipc.ExitIPC, err.Error())
		}
		waitCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		err = env.Wait(waitCtx, mustAddress(env, path), env.TokenPath(path))
		cancel()
		if err != nil {
			return fail(env, cmd.JSON, locale, ipc.ExitNotRunning, automation.Message(locale, ipc.CodeNotRunning, "", "", nil))
		}
		resp, err = call(env, path, req, 60*time.Second)
	}
	if errors.Is(err, ipc.ErrNotRunning) {
		return fail(env, cmd.JSON, locale, ipc.ExitNotRunning, automation.Message(locale, ipc.CodeNotRunning, "", "", nil))
	}
	if err != nil {
		return fail(env, cmd.JSON, locale, ipc.ExitIPC, err.Error())
	}
	if resp.ExitCode == 0 && resp.OK {
		resp.ExitCode = ipc.ExitOK
	}
	if cmd.JSON {
		if err := writeJSON(env.Stdout, resp); err != nil {
			return ipc.ExitIPC
		}
	} else if resp.OK {
		writeHuman(env.Stdout, resp)
	} else {
		writeHuman(env.Stderr, resp)
	}
	if resp.ExitCode != 0 {
		return resp.ExitCode
	}
	if !resp.OK {
		return ipc.ExitFailed
	}
	return ipc.ExitOK
}

func call(env Env, configPath string, req ipc.Request, timeout time.Duration) (ipc.Response, error) {
	address, err := env.Address(configPath)
	if err != nil {
		return ipc.Response{}, err
	}
	token, err := env.ReadToken(env.TokenPath(configPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ipc.Response{}, ipc.ErrNotRunning
		}
		return ipc.Response{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return env.Call(ctx, address, token, req)
}

func mustAddress(env Env, configPath string) string {
	address, err := env.Address(configPath)
	if err != nil {
		return ""
	}
	return address
}

func fail(env Env, asJSON bool, locale string, code int, message string) int {
	resp := ipc.Response{
		OK:       false,
		Running:  false,
		ExitCode: code,
		Message:  message,
		Code:     codeName(code),
	}
	if asJSON {
		_ = writeJSON(env.Stdout, resp)
		return code
	}
	if strings.TrimSpace(message) != "" {
		fmtLine(env.Stderr, message)
	} else {
		fmtUsage(env.Stderr, locale, nil)
	}
	return code
}

func codeName(code int) string {
	switch code {
	case ipc.ExitDisabled:
		return ipc.CodeDisabled
	case ipc.ExitNotRunning:
		return ipc.CodeNotRunning
	case ipc.ExitNotFound:
		return ipc.CodeNotFound
	case ipc.ExitAmbiguous:
		return ipc.CodeAmbiguous
	case ipc.ExitUsage:
		return "usage"
	case ipc.ExitFailed:
		return ipc.CodeFailed
	default:
		return "ipc"
	}
}

func fmtUsage(w io.Writer, locale string, err error) {
	if w == nil {
		return
	}
	if err != nil {
		fmtLine(w, err.Error())
	}
	fmtLine(w, usage(locale))
}

func fmtLine(w io.Writer, text string) {
	if w == nil || text == "" {
		return
	}
	_, _ = io.WriteString(w, text)
	if !strings.HasSuffix(text, "\n") {
		_, _ = io.WriteString(w, "\n")
	}
}
