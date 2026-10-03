package main

import (
	"context"
	"errors"
	"fmt"

	"norka/internal/diagnostics"
	"norka/internal/features"
)

var errDiagnosticsDisabled = errors.New("tunnel diagnostics are turned off")

// DiagnoseTunnel runs on-demand checks for one saved tunnel.
// The feature flag gates the call. The report is not stored, and a password
// is never sent.
func (a *App) DiagnoseTunnel(id int) (diagnostics.Report, error) {
	if err := a.ensureReady(); err != nil {
		return diagnostics.Report{}, err
	}
	if !a.featureOn(features.TunnelDiagnostics) {
		return diagnostics.Report{}, errDiagnosticsDisabled
	}
	if id <= 0 {
		return diagnostics.Report{}, fmt.Errorf("invalid tunnel id")
	}
	return a.tunnel.Diagnose(context.Background(), id)
}
