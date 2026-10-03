//go:build windows

package main

import (
	"os"
	"os/signal"
	"sync"

	"golang.org/x/sys/windows"
)

// stopCh is closed once, when Ctrl+C, Ctrl+Break, or a console close arrives.
// doneCh is closed after ShutdownTunnels and Release. A close handler waits
// on it: Windows ends the process when that handler returns.
var (
	stopCh   = make(chan struct{})
	stopOnce sync.Once
	doneCh   = make(chan struct{})
	doneOnce sync.Once

	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
	consoleCallback           uintptr
	consoleHandlerOnce        sync.Once
)

func waitForStop() {
	consoleHandlerOnce.Do(installConsoleHandler)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		requestStop()
	}()
	<-stopCh
}

func finishStop() {
	doneOnce.Do(func() { close(doneCh) })
}

func requestStop() {
	stopOnce.Do(func() { close(stopCh) })
}

func installConsoleHandler() {
	consoleCallback = windows.NewCallback(func(ctrl uintptr) uintptr {
		switch uint32(ctrl) {
		case windows.CTRL_C_EVENT, windows.CTRL_BREAK_EVENT:
			requestStop()
			return 1
		case windows.CTRL_CLOSE_EVENT, windows.CTRL_LOGOFF_EVENT, windows.CTRL_SHUTDOWN_EVENT:
			requestStop()
			<-doneCh
			return 1
		default:
			return 0
		}
	})
	_, _, _ = procSetConsoleCtrlHandler.Call(consoleCallback, 1)
}
