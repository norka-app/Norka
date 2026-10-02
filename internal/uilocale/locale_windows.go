//go:build windows

package uilocale

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var getUserDefaultLocaleName = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")

func platformLocale() string {
	buf := make([]uint16, 85)
	r, _, _ := getUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
