//go:build !windows && !darwin

package hotkey

import "fmt"

func listen(acc Accelerator, callback func()) (func(), error) {
	_ = acc
	_ = callback
	return nil, fmt.Errorf("%w: global hotkey is not available on this system", ErrUnsupported)
}
