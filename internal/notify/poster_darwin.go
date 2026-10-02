//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#import <UserNotifications/UserNotifications.h>
#include <stdlib.h>
#include <string.h>

extern void norkaNotifyClicked(char *tunnelID);

@interface NorkaNotifyDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation NorkaNotifyDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
        willPresentNotification:(UNNotification *)notification
          withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 110000
	UNNotificationPresentationOptions options = UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionSound;
#else
	UNNotificationPresentationOptions options = UNNotificationPresentationOptionAlert | UNNotificationPresentationOptionSound;
#endif
	completionHandler(options);
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
    didReceiveNotificationResponse:(UNNotificationResponse *)response
             withCompletionHandler:(void (^)(void))completionHandler {
	id raw = response.notification.request.content.userInfo[@"tunnelID"];
	const char *utf = "";
	if ([raw isKindOfClass:[NSString class]]) {
		utf = [(NSString *)raw UTF8String];
	}
	if (utf == NULL) {
		utf = "";
	}
	char *copy = strdup(utf);
	norkaNotifyClicked(copy);
	completionHandler();
}
@end

static NorkaNotifyDelegate *norkaDelegate = nil;

static void norkaEnsureCenter(void) {
	if (norkaDelegate != nil) {
		return;
	}
	norkaDelegate = [NorkaNotifyDelegate new];
	UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
	center.delegate = norkaDelegate;
	[center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
	                      completionHandler:^(__unused BOOL granted, __unused NSError *error) {}];
}

void norkaPost(const char *title, const char *body, const char *tunnelID) {
	NSString *titleStr = [NSString stringWithUTF8String:(title ? title : "")];
	NSString *bodyStr = [NSString stringWithUTF8String:(body ? body : "")];
	NSString *tunnelStr = [NSString stringWithUTF8String:(tunnelID ? tunnelID : "0")];
	dispatch_async(dispatch_get_main_queue(), ^{
		norkaEnsureCenter();
		UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
		content.title = titleStr;
		content.body = bodyStr;
		content.sound = [UNNotificationSound defaultSound];
		content.userInfo = @{@"tunnelID": tunnelStr};
		NSString *identifier = [[NSUUID UUID] UUIDString];
		UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier content:content trigger:nil];
		[[UNUserNotificationCenter currentNotificationCenter] addNotificationRequest:request withCompletionHandler:^(__unused NSError *error) {}];
	});
}
*/
import "C"

type darwinPoster struct{}

func newPlatformPoster(cfg PosterConfig) Poster {
	setClickHandler(cfg.OnClick)
	return darwinPoster{}
}

func (darwinPoster) Post(notice Notice) error {
	title := C.CString(notice.Title)
	body := C.CString(notice.Body)
	tunnelID := C.CString(focusArg(notice.TunnelID))
	defer C.free(unsafePointer(title))
	defer C.free(unsafePointer(body))
	defer C.free(unsafePointer(tunnelID))
	C.norkaPost(title, body, tunnelID)
	return nil
}
