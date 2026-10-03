package main

import (
	"context"
	"os"
	"time"

	"github.com/norka-app/Norka/internal/cli"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/uilocale"
)

func runCLI(args []string) int {
	return cli.Run(cli.Env{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Load:      loadCLIConfig,
		Address:   ipc.Address,
		TokenPath: ipc.TokenPath,
		ReadToken: ipc.ReadToken,
		Call:      ipc.Call,
		Launch:    cli.LaunchHidden,
		Wait:      waitForIPC,
	}, args)
}

func loadCLIConfig() (*conf.Config, string, string, error) {
	path := conf.ResolveConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, "", "", err
		}
		cfg := conf.DefaultConfig()
		cfg.Normalize()
		return cfg, path, uilocale.Effective(cfg.Language), nil
	}
	cfg, err := conf.ParseConfigTOML(data)
	if err != nil {
		return nil, "", "", err
	}
	return cfg, path, uilocale.Effective(cfg.Language), nil
}

func waitForIPC(ctx context.Context, address, tokenPath string) error {
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
