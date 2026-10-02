//go:build windows

package netwatch

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Power and address notifications come from user32 and iphlpapi.
// Nothing here allocates a console or shows a window.
// RegisterSuspendResumeNotification delivers resume on a thread-pool thread,
// so a hidden message window and LockOSThread are unnecessary.

const (
	deviceNotifyCallback  = 2
	pbtApmResumeSuspend   = 0x0007
	pbtApmResumeAutomatic = 0x0012
)

type suspendResumeSubscription struct {
	Callback uintptr
	Context  uintptr
}

var (
	user32                                  = windows.NewLazySystemDLL("user32.dll")
	procRegisterSuspendResumeNotification   = user32.NewProc("RegisterSuspendResumeNotification")
	procUnregisterSuspendResumeNotification = user32.NewProc("UnregisterSuspendResumeNotification")

	// NewCallback cannot be freed and the runtime only keeps a few thousand
	// of them. Build both once and reuse them across flag toggles.
	callbackOnce     sync.Once
	powerCallbackPtr uintptr
	ipCallbackPtr    uintptr

	winMu             sync.Mutex
	winGen            uint64
	powerSubscription suspendResumeSubscription
	powerHandle       uintptr
	ipHandle          windows.Handle
)

func ensureWindowsCallbacks() {
	callbackOnce.Do(func() {
		powerCallbackPtr = windows.NewCallback(onPowerBroadcast)
		ipCallbackPtr = windows.NewCallback(onInterfaceChange)
		powerSubscription.Callback = powerCallbackPtr
	})
}

func startOSWatch(ctx context.Context, out chan<- Event) error {
	powerErr, netErr, gen := registerWindowsWatch()
	if powerErr != nil {
		slog.Info("power resume watch unavailable", "err", powerErr)
	}
	if netErr != nil {
		slog.Info("ip change watch unavailable; polling interfaces", "err", netErr)
		go watchInterfaces(ctx, out)
	}
	go func() {
		<-ctx.Done()
		stopWindowsWatch(gen)
	}()
	return nil
}

// registerWindowsWatch bumps the generation and registers each source that is
// not already registered. A second start reuses the live registration instead
// of calling NewCallback or the register APIs again. The returned generation
// lets a stale stop ignore itself after a newer start has taken over.
func registerWindowsWatch() (powerErr, netErr error, gen uint64) {
	ensureWindowsCallbacks()
	if err := procRegisterSuspendResumeNotification.Find(); err != nil {
		powerErr = err
	}
	winMu.Lock()
	defer winMu.Unlock()
	winGen++
	gen = winGen
	if powerErr == nil {
		powerErr = registerPowerLocked()
	}
	netErr = registerIPLocked()
	return powerErr, netErr, gen
}

func registerPowerLocked() error {
	if powerHandle != 0 {
		return nil
	}
	handle, _, callErr := procRegisterSuspendResumeNotification.Call(
		uintptr(unsafe.Pointer(&powerSubscription)),
		uintptr(deviceNotifyCallback),
	)
	if handle == 0 {
		if callErr == nil || callErr == windows.ERROR_SUCCESS {
			callErr = errors.New("RegisterSuspendResumeNotification failed")
		}
		return callErr
	}
	powerHandle = handle
	return nil
}

func onPowerBroadcast(context, eventType, setting uintptr) uintptr {
	_, _ = context, setting
	switch eventType {
	case pbtApmResumeAutomatic, pbtApmResumeSuspend:
		publish(KindResume)
	}
	return 0
}

func registerIPLocked() error {
	if ipHandle != 0 {
		return nil
	}
	var handle windows.Handle
	if err := windows.NotifyIpInterfaceChange(windows.AF_UNSPEC, ipCallbackPtr, nil, false, &handle); err != nil {
		return err
	}
	ipHandle = handle
	return nil
}

func onInterfaceChange(callerContext, row, notificationType uintptr) uintptr {
	_, _ = callerContext, row
	if notificationType == windows.MibInitialNotification {
		return 0
	}
	publish(KindNetwork)
	return 0
}

func stopWindowsWatch(gen uint64) {
	winMu.Lock()
	defer winMu.Unlock()
	if gen != winGen {
		return
	}
	if powerHandle != 0 {
		_, _, _ = procUnregisterSuspendResumeNotification.Call(powerHandle)
		powerHandle = 0
	}
	if ipHandle != 0 {
		_ = windows.CancelMibChangeNotify2(ipHandle)
		ipHandle = 0
	}
}
