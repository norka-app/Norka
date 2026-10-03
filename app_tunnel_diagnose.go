package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/tunneldiag"
)

var errTunnelCheckDisabled = errors.New("tunnel check is turned off")

// DiagnoseTunnel runs on-demand checks for one saved tunnel.
// The tunnel_diagnostics flag gates the call. The report is not stored,
// and a password is never sent.
func (a *App) DiagnoseTunnel(id int) (tunneldiag.Report, error) {
	if err := a.ensureReady(); err != nil {
		return tunneldiag.Report{}, err
	}
	if !a.featureOn(features.TunnelDiagnostics) {
		return tunneldiag.Report{}, errTunnelCheckDisabled
	}
	if id <= 0 {
		return tunneldiag.Report{}, fmt.Errorf("invalid tunnel id")
	}
	return a.tunnel.Diagnose(context.Background(), id)
}
