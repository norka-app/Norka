//go:build !linux && !windows && !darwin

package netwatch

import "context"

func startOSWatch(ctx context.Context, out chan<- Event) error {
	go watchInterfaces(ctx, out)
	return nil
}
