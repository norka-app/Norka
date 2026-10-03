package main

import (
	"fmt"

	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/tunnelstats"
)

// GetTunnelStats returns per-tunnel counters. An empty list when the feature
// is off: the counters may keep running, but the UI has nothing to draw.
func (a *App) GetTunnelStats() ([]tunnelstats.View, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	if !a.featureOn(features.TunnelStats) {
		return []tunnelstats.View{}, nil
	}
	return a.tunnel.Stats(), nil
}

// ResetTunnelStats clears one tunnel's counters and keeps the saved file in
// sync on the next debounced write (and on shutdown).
func (a *App) ResetTunnelStats(id int) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	if id <= 0 {
		return fmt.Errorf("invalid tunnel id")
	}
	a.tunnel.ResetStats(id)
	return nil
}
