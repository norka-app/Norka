//go:build linux

package notify

import (
	"context"
	"errors"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	fdoNotificationsName = "org.freedesktop.Notifications"
	fdoNotificationsPath = "/org/freedesktop/Notifications"
	notifyTimeout        = 5 * time.Second
)

// linuxPoster shows notifications through the Freedesktop notification service.
// A missing session bus or notification service is not an error: Post returns nil.
type linuxPoster struct {
	appID string
	icon  string
}

func newPlatformPoster(cfg PosterConfig) Poster {
	setClickHandler(cfg.OnClick)
	return linuxPoster{appID: cfg.AppID, icon: cfg.IconPath}
}

func (p linuxPoster) Post(notice Notice) error {
	conn, err := sessionNotificationsBus()
	if err != nil {
		return nil
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	if !notificationsOwned(ctx, conn) {
		return nil
	}
	obj := conn.Object(fdoNotificationsName, dbus.ObjectPath(fdoNotificationsPath))
	call := obj.CallWithContext(
		ctx,
		fdoNotificationsName+".Notify",
		dbus.FlagNoAutoStart,
		p.appID,
		uint32(0),
		p.icon,
		notice.Title,
		notice.Body,
		[]string{},
		map[string]dbus.Variant{},
		int32(-1),
	)
	if call.Err == nil || notificationsUnavailable(call.Err) {
		return nil
	}
	return call.Err
}

// sessionNotificationsBus opens the session bus without launching one.
// SessionBusPrivate does not authenticate; Auth and Hello make it usable.
func sessionNotificationsBus() (*dbus.Conn, error) {
	conn, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return nil, err
	}
	if err := conn.Auth(nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func notificationsOwned(ctx context.Context, conn *dbus.Conn) bool {
	var owned bool
	err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, fdoNotificationsName).Store(&owned)
	return err == nil && owned
}

func notificationsUnavailable(err error) bool {
	var dbusErr *dbus.Error
	if !errors.As(err, &dbusErr) || dbusErr == nil {
		return false
	}
	switch dbusErr.Name {
	case "org.freedesktop.DBus.Error.ServiceUnknown",
		"org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.DBus.Error.Spawn.ExecFailed",
		"org.freedesktop.DBus.Error.Spawn.ChildExited",
		"org.freedesktop.DBus.Error.Spawn.ForkFailed",
		"org.freedesktop.DBus.Error.Spawn.Failed":
		return true
	default:
		return false
	}
}
