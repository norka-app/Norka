//go:build windows

package aidebug

import (
	"context"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCommandContextHidesConsole(t *testing.T) {
	cmd := commandContext(context.Background(), "ping", "-n", "1", "127.0.0.1")
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow is not set")
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("CREATE_NO_WINDOW is not set")
	}
}
