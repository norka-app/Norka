//go:build linux

package diagnostics

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

func osVersion() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var name, pretty string
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "PRETTY_NAME":
			pretty = value
		case "NAME":
			name = value
		}
	}
	if pretty != "" {
		return pretty
	}
	return name
}

func webViewVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, pkg := range []string{"webkit2gtk-4.1", "webkit2gtk-4.0"} {
		out, err := exec.CommandContext(ctx, "pkg-config", "--modversion", pkg).Output()
		if err != nil {
			continue
		}
		if version := strings.TrimSpace(string(out)); version != "" {
			return version
		}
	}
	return ""
}
