//go:build darwin && cgo

package hotkey

import "C"

//export goHotkeyCallback
func goHotkeyCallback(id C.int) {
	dispatchDarwinHotkey(int(id))
}
