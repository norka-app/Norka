package scheme

import (
	"fmt"
	"strings"
)

// OpenCommand is the Windows shell command that receives a norka:// URL.
func OpenCommand(exe string) (string, error) {
	exe = strings.TrimSpace(exe)
	if exe == "" || strings.ContainsAny(exe, "\"\r\n") {
		return "", fmt.Errorf("invalid executable path")
	}
	return `"` + exe + `" "%1"`, nil
}
