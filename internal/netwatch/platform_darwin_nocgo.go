//go:build darwin && !cgo

package netwatch

import (
	"context"
	"log/slog"
)

// Cross-compiles and CGO_ENABLED=0 builds cannot link IOKit.
// The packaged macOS app uses the cgo watcher. Sleep is still caught by the
// clock-jump detector, and network changes fall back to an interface poll.
func startOSWatch(ctx context.Context, out chan<- Event) error {
	slog.Info("sleep notifications need cgo; polling interfaces and watching the clock")
	go watchInterfaces(ctx, out)
	return nil
}
