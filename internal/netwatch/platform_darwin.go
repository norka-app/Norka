//go:build darwin && cgo

package netwatch

/*
#cgo darwin LDFLAGS: -framework IOKit -framework CoreFoundation -framework SystemConfiguration
#cgo darwin CFLAGS: -Wno-deprecated-declarations

#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/IOMessage.h>
#include <IOKit/pwr_mgt/IOPMLib.h>
#include <SystemConfiguration/SystemConfiguration.h>
#include <netinet/in.h>
#include <string.h>

extern void goNetwatchResume(void);
extern void goNetwatchNetwork(void);

static IONotificationPortRef gNotifyPort = NULL;
static io_connect_t gRootPort = 0;
static io_object_t gNotifier = 0;
static CFRunLoopRef gRunLoop = NULL;
static SCNetworkReachabilityRef gReach = NULL;
static int gReachSeen = 0;

static void norkaPowerCallback(void *refCon, io_service_t service, natural_t messageType, void *messageArgument) {
	(void)refCon;
	(void)service;
	switch (messageType) {
	case kIOMessageCanSystemSleep:
	case kIOMessageSystemWillSleep:
		IOAllowPowerChange(gRootPort, (long)messageArgument);
		break;
	case kIOMessageSystemHasPoweredOn:
		goNetwatchResume();
		break;
	default:
		break;
	}
}

static void norkaReachCallback(SCNetworkReachabilityRef target, SCNetworkReachabilityFlags flags, void *info) {
	(void)target;
	(void)flags;
	(void)info;
	if (!gReachSeen) {
		gReachSeen = 1;
		return;
	}
	goNetwatchNetwork();
}

static int norkaStartWakeWatch(void) {
	struct sockaddr_in addr;
	SCNetworkReachabilityContext reachCtx;

	gRunLoop = CFRunLoopGetCurrent();
	gRootPort = IORegisterForSystemPower(NULL, &gNotifyPort, norkaPowerCallback, &gNotifier);
	if (gRootPort == 0 || gNotifyPort == NULL) {
		return 1;
	}
	CFRunLoopAddSource(gRunLoop, IONotificationPortGetRunLoopSource(gNotifyPort), kCFRunLoopDefaultMode);

	memset(&addr, 0, sizeof(addr));
	addr.sin_len = sizeof(addr);
	addr.sin_family = AF_INET;
	gReach = SCNetworkReachabilityCreateWithAddress(NULL, (struct sockaddr *)&addr);
	if (gReach != NULL) {
		memset(&reachCtx, 0, sizeof(reachCtx));
		if (SCNetworkReachabilitySetCallback(gReach, norkaReachCallback, &reachCtx)) {
			SCNetworkReachabilityScheduleWithRunLoop(gReach, gRunLoop, kCFRunLoopDefaultMode);
		}
	}
	return 0;
}

static int norkaReachabilityReady(void) {
	return gReach != NULL;
}

static void norkaStopWakeWatch(void) {
	if (gRunLoop != NULL) {
		CFRunLoopStop(gRunLoop);
	}
}

// Returns 1 when the run loop was stopped. A short timeout lets Go notice cancellation.
static int norkaPumpWakeWatch(double seconds) {
	SInt32 result = CFRunLoopRunInMode(kCFRunLoopDefaultMode, seconds, false);
	return result == kCFRunLoopRunStopped || result == kCFRunLoopRunFinished;
}

static void norkaCleanupWakeWatch(void) {
	if (gReach != NULL) {
		SCNetworkReachabilitySetCallback(gReach, NULL, NULL);
		if (gRunLoop != NULL) {
			SCNetworkReachabilityUnscheduleFromRunLoop(gReach, gRunLoop, kCFRunLoopDefaultMode);
		}
		CFRelease(gReach);
		gReach = NULL;
	}
	if (gNotifier != 0) {
		IODeregisterForSystemPower(&gNotifier);
	}
	if (gNotifyPort != NULL) {
		if (gRunLoop != NULL) {
			CFRunLoopRemoveSource(gRunLoop, IONotificationPortGetRunLoopSource(gNotifyPort), kCFRunLoopDefaultMode);
		}
		IONotificationPortDestroy(gNotifyPort);
		gNotifyPort = NULL;
	}
	if (gRootPort != 0) {
		IOServiceClose(gRootPort);
		gRootPort = 0;
	}
	gRunLoop = NULL;
	gReachSeen = 0;
}
*/
import "C"

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
)

func startOSWatch(ctx context.Context, out chan<- Event) error {
	errCh := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if rc := C.norkaStartWakeWatch(); rc != 0 {
			errCh <- fmt.Errorf("IORegisterForSystemPower failed (%d)", int(rc))
			return
		}
		if C.norkaReachabilityReady() == 0 {
			slog.Info("reachability watch unavailable; polling interfaces")
			go watchInterfaces(ctx, out)
		}
		errCh <- nil
		defer C.norkaCleanupWakeWatch()
		go func() {
			<-ctx.Done()
			C.norkaStopWakeWatch()
		}()
		for ctx.Err() == nil {
			if C.norkaPumpWakeWatch(0.5) != 0 {
				break
			}
		}
	}()
	return <-errCh
}
