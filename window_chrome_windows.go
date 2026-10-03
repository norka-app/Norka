//go:build windows

package main

import "golang.org/x/sys/windows"

// micaSupported reports Windows 11 22H2 (build 22621) or newer.
// DWM system backdrops, including Mica, exist only from that build.
func micaSupported() bool {
	major, _, build := windows.RtlGetNtVersionNumbers()
	return major > 10 || (major == 10 && build >= 22621)
}
