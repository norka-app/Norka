package update

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var mountPointPattern = regexp.MustCompile(`<key>mount-point</key>\s*<string>([^<]+)</string>`)

func parseMountPoint(plist []byte) (string, error) {
	match := mountPointPattern.FindSubmatch(plist)
	if len(match) < 2 {
		return "", fmt.Errorf("dmg mount point not found")
	}
	point := strings.TrimSpace(string(match[1]))
	if point == "" {
		return "", fmt.Errorf("dmg mount point is empty")
	}
	return point, nil
}

func findAppBundle(root string) (string, error) {
	preferred := filepath.Join(root, "norka.app")
	if info, err := os.Stat(preferred); err == nil && info.IsDir() {
		return preferred, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
			return filepath.Join(root, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("app bundle not found")
}

func windowsRelaunchScript(pid int, exe string) string {
	return fmt.Sprintf("@echo off\r\nset PID=%d\r\nset \"EXE=%s\"\r\n:wait\r\ntasklist /FI \"PID eq %%PID%%\" 2>nul | find \"%%PID%%\" >nul\r\nif %%errorlevel%%==0 (\r\n  ping -n 2 127.0.0.1 >nul\r\n  goto wait\r\n)\r\nstart \"\" \"%%EXE%%\"\r\ndel \"%%~f0\"\r\n", pid, exe)
}

func darwinRelaunchScript(pid int, app string) string {
	return fmt.Sprintf("#!/bin/sh\nPID=%d\nAPP=%s\nwhile kill -0 \"$PID\" 2>/dev/null; do\n  sleep 0.4\ndone\nxattr -dr com.apple.quarantine \"$APP\" 2>/dev/null || true\nopen \"$APP\"\nrm -f \"$0\"\n", pid, shellSingleQuote(app))
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
