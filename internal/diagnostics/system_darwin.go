//go:build darwin

package diagnostics

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

func osVersion() string {
	product := commandOutput("sw_vers", "-productVersion")
	build := commandOutput("sw_vers", "-buildVersion")
	switch {
	case product != "" && build != "":
		return product + " (" + build + ")"
	case product != "":
		return product
	default:
		return build
	}
}

func webViewVersion() string {
	return commandOutput("defaults", "read", "/System/Library/Frameworks/WebKit.framework/Resources/Info", "CFBundleShortVersionString")
}

func commandOutput(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
