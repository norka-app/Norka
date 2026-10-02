//go:build darwin

package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func installPrepared(ctx context.Context, dmgPath string) error {
	mount, err := attachDMG(ctx, dmgPath)
	if err != nil {
		return err
	}
	defer detachDMG(mount)

	srcApp, err := findAppBundle(mount)
	if err != nil {
		return err
	}
	destApp, ok := runningBundlePath()
	if !ok {
		return errNotWritable
	}
	parent := filepath.Dir(destApp)
	if !dirWritable(parent) {
		return errNotWritable
	}
	backup := destApp + ".old"
	_ = os.RemoveAll(backup)
	if err := os.Rename(destApp, backup); err != nil {
		return err
	}
	staged := filepath.Join(parent, ".norka-update.app")
	_ = os.RemoveAll(staged)
	if err := copyApp(srcApp, staged); err != nil {
		_ = os.Rename(backup, destApp)
		return err
	}
	if err := os.Rename(staged, destApp); err != nil {
		_ = os.RemoveAll(staged)
		_ = os.Rename(backup, destApp)
		return err
	}
	if err := clearQuarantine(destApp); err != nil {
		_ = os.RemoveAll(destApp)
		_ = os.Rename(backup, destApp)
		return err
	}
	return nil
}

func attachDMG(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "hdiutil", "attach", "-nobrowse", "-readonly", "-plist", path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return parseMountPoint(out)
}

func detachDMG(mount string) {
	if mount == "" {
		return
	}
	_ = exec.Command("hdiutil", "detach", mount, "-quiet").Run()
}

func copyApp(src, dst string) error {
	cmd := exec.Command("/usr/bin/ditto", src, dst)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("copy app: %w", err)
	}
	return nil
}

func clearQuarantine(path string) error {
	return exec.Command("xattr", "-dr", "com.apple.quarantine", path).Run()
}

func scheduleRelaunch() error {
	bundle, ok := runningBundlePath()
	if !ok {
		return errNotWritable
	}
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("norka-relaunch-%d.sh", os.Getpid()))
	body := darwinRelaunchScript(os.Getpid(), bundle)
	if err := os.WriteFile(scriptPath, []byte(body), 0o700); err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}

func openLocalFile(path string) error {
	return exec.Command("open", path).Start()
}
