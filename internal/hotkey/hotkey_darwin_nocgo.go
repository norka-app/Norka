//go:build darwin && !cgo

package hotkey

import "fmt"

// listen is the cgo-off stub used by cross-compiles (build-linux runs
// CGO_ENABLED=0 GOOS=darwin). The packaged macOS app uses the Carbon backend.
func listen(acc Accelerator, callback func()) (func(), error) {
	_ = acc
	_ = callback
	return nil, fmt.Errorf("%w: global hotkey is not available without cgo", ErrUnsupported)
}
