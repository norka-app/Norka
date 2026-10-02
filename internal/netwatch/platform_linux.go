//go:build linux

package netwatch

import (
	"context"
	"errors"
	"log/slog"
	"syscall"

	"github.com/godbus/dbus/v5"
)

const (
	logindPath      = "/org/freedesktop/login1"
	logindInterface = "org.freedesktop.login1.Manager"
	logindSignal    = logindInterface + ".PrepareForSleep"
	// link, IPv4 address, and IPv6 address groups.
	netlinkGroups = 0x1 | 0x10 | 0x100
)

func startOSWatch(ctx context.Context, out chan<- Event) error {
	powerErr := watchLogind(ctx, out)
	netErr := watchNetlink(ctx, out)
	if powerErr != nil {
		slog.Info("logind sleep watch unavailable", "err", powerErr)
	}
	if netErr != nil {
		slog.Info("netlink watch unavailable; polling interfaces", "err", netErr)
		go watchInterfaces(ctx, out)
	}
	return nil
}

// watchLogind listens for PrepareForSleep(false), which logind sends after resume.
func watchLogind(ctx context.Context, out chan<- Event) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(logindInterface),
		dbus.WithMatchMember("PrepareForSleep"),
		dbus.WithMatchObjectPath(logindPath),
	); err != nil {
		_ = conn.Close()
		return err
	}
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	go func() {
		defer func() { _ = conn.Close() }()
		for {
			select {
			case <-ctx.Done():
				conn.RemoveSignal(signals)
				return
			case sig, ok := <-signals:
				if !ok || sig == nil {
					return
				}
				if sig.Name != logindSignal || len(sig.Body) == 0 {
					continue
				}
				sleeping, ok := sig.Body[0].(bool)
				if ok && !sleeping {
					emit(ctx, out, Event{Kind: KindResume})
				}
			}
		}
	}()
	return nil
}

func watchNetlink(ctx context.Context, out chan<- Event) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: netlinkGroups,
	}); err != nil {
		_ = syscall.Close(fd)
		return err
	}
	tv := syscall.Timeval{Sec: 1}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		_ = syscall.Close(fd)
		return err
	}
	go func() {
		defer func() { _ = syscall.Close(fd) }()
		buf := make([]byte, 8192)
		for ctx.Err() == nil {
			_, _, recvErr := syscall.Recvfrom(fd, buf, 0)
			if recvErr != nil {
				if errors.Is(recvErr, syscall.EAGAIN) || errors.Is(recvErr, syscall.EWOULDBLOCK) || errors.Is(recvErr, syscall.EINTR) {
					continue
				}
				return
			}
			emit(ctx, out, Event{Kind: KindNetwork})
		}
	}()
	return nil
}
