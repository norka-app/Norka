//go:build darwin

package notify

/*
#include <stdlib.h>
*/
import "C"
import "unsafe"

//export norkaNotifyClicked
func norkaNotifyClicked(tunnelID *C.char) {
	if tunnelID == nil {
		dispatchClick(0)
		return
	}
	arg := C.GoString(tunnelID)
	C.free(unsafe.Pointer(tunnelID))
	dispatchClick(ParseFocusArg([]string{arg}))
}

func unsafePointer(value *C.char) unsafe.Pointer {
	return unsafe.Pointer(value)
}
