//go:build linux

package notify

import (
	"bufio"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestLinuxPosterSilentWithoutSessionBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/norka-dbus-missing")
	poster := newPlatformPoster(PosterConfig{AppID: "Norka"})
	if err := poster.Post(Notice{Title: "Norka", Body: "нет шины", TunnelID: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxPosterSilentWithoutNotificationService(t *testing.T) {
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		t.Skip("dbus-daemon is not installed")
	}
	cmd := exec.Command("dbus-daemon", "--session", "--nofork", "--nopidfile", "--print-address=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	address := strings.TrimSpace(line)
	if address == "" {
		t.Fatal("dbus-daemon printed an empty address")
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", address)

	poster := newPlatformPoster(PosterConfig{AppID: "Norka"})
	if err := poster.Post(Notice{Title: "Norka", Body: "нет службы", TunnelID: 2}); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationsUnavailable(t *testing.T) {
	missing := &dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown", Body: []any{"nope"}}
	if !notificationsUnavailable(missing) {
		t.Fatal("service unknown should be silent")
	}
	if notificationsUnavailable(errors.New("other")) {
		t.Fatal("unrelated errors should surface")
	}
	if notificationsUnavailable(nil) {
		t.Fatal("nil is not a missing service")
	}
}
