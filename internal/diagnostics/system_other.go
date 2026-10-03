//go:build !linux && !darwin && !windows

package diagnostics

import "runtime"

func osVersion() string {
	return runtime.GOOS
}

func webViewVersion() string {
	return ""
}
