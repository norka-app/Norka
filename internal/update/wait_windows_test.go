//go:build windows

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWaitForPIDExitAlreadyExited(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/C", "exit")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	WaitForPIDExit(cmd.Process.Pid)
	if time.Since(start) > 2*time.Second {
		t.Fatalf("exited pid blocked for %s", time.Since(start))
	}
}

func TestWaitForPIDExitTimesOutWhileRunning(t *testing.T) {
	start := time.Now()
	waitForPIDExit(os.Getpid(), 200*time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 150*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("elapsed %s", elapsed)
	}
}

func TestRelaunchCommandHasNoConsole(t *testing.T) {
	cmd := relaunchCommand(`C:\Program Files\norka.exe`, 8125)
	if strings.EqualFold(filepath.Base(cmd.Args[0]), "cmd.exe") {
		t.Fatalf("relaunch still uses cmd: %#v", cmd.Args)
	}
	text := strings.ToLower(strings.Join(cmd.Args, " "))
	for _, tool := range []string{"tasklist", "find ", "ping "} {
		if strings.Contains(text, tool) {
			t.Fatalf("relaunch args spawn a console tool: %#v", cmd.Args)
		}
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow is not set")
	}
	flags := cmd.SysProcAttr.CreationFlags
	if flags&windows.CREATE_NO_WINDOW == 0 || flags&windows.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatalf("flags = %#x", flags)
	}
	if flags&windows.DETACHED_PROCESS != 0 {
		t.Fatal("DETACHED_PROCESS would give console children their own windows")
	}
	if cmd.Args[1] != AfterUpdateWaitFlag || cmd.Args[2] != "8125" {
		t.Fatalf("args = %#v", cmd.Args)
	}
}
