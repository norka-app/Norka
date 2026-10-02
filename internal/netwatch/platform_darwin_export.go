//go:build darwin && cgo

package netwatch

import "C"

//export goNetwatchResume
func goNetwatchResume() {
	publish(KindResume)
}

//export goNetwatchNetwork
func goNetwatchNetwork() {
	publish(KindNetwork)
}
