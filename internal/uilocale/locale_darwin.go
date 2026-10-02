//go:build darwin

package uilocale

import (
	"os/exec"
	"strings"
)

func platformLocale() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		out, err = exec.Command("defaults", "read", "-g", "AppleLocale").Output()
		if err != nil {
			return ""
		}
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.Trim(line, " \t\",()")
		if line == "" {
			continue
		}
		return line
	}
	return ""
}
