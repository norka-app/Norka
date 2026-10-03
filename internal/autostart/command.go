package autostart

import (
	"strconv"
	"strings"

	"github.com/norka-app/Norka/internal/cli"
)

// execArgs is the argv a login entry should launch.
// hidden appends --norka-hidden so the window stays in the tray.
func execArgs(exe string, hidden bool) []string {
	if strings.TrimSpace(exe) == "" {
		exe = appName
	}
	if !hidden {
		return []string{exe}
	}
	return []string{exe, cli.HiddenArg}
}

// linuxDesktopExec is the Exec= value of the XDG autostart file.
func linuxDesktopExec(exe string, hidden bool) string {
	args := execArgs(exe, hidden)
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = quoteDesktopExec(arg)
	}
	return strings.Join(parts, " ")
}

// windowsRunValue is the HKCU Run string. The executable stays quoted the
// way older versions wrote it; the tray flag is a separate argument.
func windowsRunValue(exe string, hidden bool) string {
	args := execArgs(exe, hidden)
	parts := make([]string, len(args))
	parts[0] = strconv.Quote(args[0])
	for i := 1; i < len(args); i++ {
		parts[i] = args[i]
		if strings.ContainsAny(args[i], " \t\"") {
			parts[i] = strconv.Quote(args[i])
		}
	}
	return strings.Join(parts, " ")
}

// darwinProgramArguments is the Exec slice passed to go-autostart.
// The library writes each element as a ProgramArguments string.
func darwinProgramArguments(exe string, hidden bool) []string {
	return execArgs(exe, hidden)
}

func quoteDesktopExec(exe string) string {
	if exe == "" {
		return appName
	}
	if strings.ContainsAny(exe, " \t\"\\$`") {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(exe)
		return `"` + escaped + `"`
	}
	return exe
}

// plistProgramArgs reads ProgramArguments from a launchd plist.
// ok is false when the array is missing or empty.
func plistProgramArgs(content string) ([]string, bool) {
	const key = "<key>ProgramArguments</key>"
	i := strings.Index(content, key)
	if i < 0 {
		return nil, false
	}
	rest := content[i+len(key):]
	end := strings.Index(rest, "</array>")
	if end < 0 {
		return nil, false
	}
	block := rest[:end]
	var args []string
	for {
		start := strings.Index(block, "<string>")
		if start < 0 {
			break
		}
		block = block[start+len("<string>"):]
		stop := strings.Index(block, "</string>")
		if stop < 0 {
			return nil, false
		}
		args = append(args, block[:stop])
		block = block[stop+len("</string>"):]
	}
	if len(args) == 0 {
		return nil, false
	}
	return args, true
}
