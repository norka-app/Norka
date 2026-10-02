//go:build windows

package netwatch

import (
	"context"
	"errors"
	"log/slog"
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
	powerSubscription                       suspendResumeSubscription
	powerHandle                             uintptr
	ipHandle                                windows.Handle
	powerCallbackPtr                        uintptr
	ipCallbackPtr                           uintptr
)

func startOSWatch(ctx context.Context, out chan<- Event) error {
	powerErr := watchPower(ctx)
	netErr := watchIP(ctx)
	if powerErr != nil {
		slog.Info("power resume watch unavailable", "err", powerErr)
	}
	if netErr != nil {
		slog.Info("ip change watch unavailable; polling interfaces", "err", netErr)
		go watchInterfaces(ctx, out)
	}
	go func() {
		<-ctx.Done()
		stopWindowsWatch()
	}()
	return nil
}

func watchPower(ctx context.Context) error {
	if err := procRegisterSuspendResumeNotification.Find(); err != nil {
		return err
	}
	powerCallbackPtr = windows.NewCallback(onPowerBroadcast)
	powerSubscription = suspendResumeSubscription{Callback: powerCallbackPtr}
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
	_ = ctx
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

func watchIP(ctx context.Context) error {
	ipCallbackPtr = windows.NewCallback(onInterfaceChange)
	var handle windows.Handle
	if err := windows.NotifyIpInterfaceChange(windows.AF_UNSPEC, ipCallbackPtr, nil, false, &handle); err != nil {
		return err
	}
	ipHandle = handle
	_ = ctx
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

func stopWindowsWatch() {
	if powerHandle != 0 {
		_, _, _ = procUnregisterSuspendResumeNotification.Call(powerHandle)
		powerHandle = 0
	}
	if ipHandle != 0 {
		_ = windows.CancelMibChangeNotify2(ipHandle)
		ipHandle = 0
	}
}
