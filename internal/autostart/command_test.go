package autostart

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/norka-app/Norka/internal/cli"
)

func TestGeneratedAutostartCommandLine(t *testing.T) {
	t.Run("linux", func(t *testing.T) {
		exe := "/usr/bin/norka"
		legacy := quoteDesktopExec(exe)
		if linuxDesktopExec(exe, false) != legacy {
			t.Fatalf("flag off changed the old Exec line: %q", linuxDesktopExec(exe, false))
		}
		hidden := linuxDesktopExec(exe, true)
		if hidden != exe+" "+cli.HiddenArg {
			t.Fatalf("hidden Exec = %q", hidden)
		}
		spaced := linuxDesktopExec(`/home/user/My Apps/norka`, true)
		if spaced != `"/home/user/My Apps/norka" `+cli.HiddenArg {
			t.Fatalf("spaced Exec = %q", spaced)
		}
	})

	t.Run("windows", func(t *testing.T) {
		exe := `C:\Program Files\Norka\norka.exe`
		if windowsRunValue(exe, false) != strconv.Quote(exe) {
			t.Fatalf("flag off changed the old Run value: %q", windowsRunValue(exe, false))
		}
		hidden := windowsRunValue(exe, true)
		if hidden != strconv.Quote(exe)+" "+cli.HiddenArg {
			t.Fatalf("hidden Run value = %q", hidden)
		}
		plain := `C:\Norka\norka.exe`
		if windowsRunValue(plain, false) != strconv.Quote(plain) {
			t.Fatalf("plain Run value = %q", windowsRunValue(plain, false))
		}
	})

	t.Run("darwin", func(t *testing.T) {
		exe := "/Applications/Norka.app/Contents/MacOS/norka"
		if !slices.Equal(darwinProgramArguments(exe, false), []string{exe}) {
			t.Fatalf("flag off changed go-autostart Exec: %#v", darwinProgramArguments(exe, false))
		}
		hidden := darwinProgramArguments(exe, true)
		if !slices.Equal(hidden, []string{exe, cli.HiddenArg}) {
			t.Fatalf("hidden Exec = %#v", hidden)
		}
		plist := renderDarwinJob(t, hidden)
		got, ok := plistProgramArgs(plist)
		if !ok || !slices.Equal(got, hidden) {
			t.Fatalf("plist ProgramArguments: ok=%v got=%#v\n%s", ok, got, plist)
		}
		if !strings.Contains(plist, "<string>"+cli.HiddenArg+"</string>") {
			t.Fatalf("plist missing tray flag:\n%s", plist)
		}
		legacy := renderDarwinJob(t, darwinProgramArguments(exe, false))
		legacyArgs, ok := plistProgramArgs(legacy)
		if !ok || !slices.Equal(legacyArgs, []string{exe}) {
			t.Fatalf("legacy plist: ok=%v got=%#v", ok, legacyArgs)
		}
		if strings.Contains(legacy, cli.HiddenArg) {
			t.Fatalf("visible plist includes the tray flag:\n%s", legacy)
		}
	})
}

// jobTemplate matches github.com/emersion/go-autostart launchd plist.
const jobTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>{{.Name}}</string>
    <key>ProgramArguments</key>
      <array>
        {{range .Exec -}}
        <string>{{.}}</string>
        {{end}}
      </array>
    <key>RunAtLoad</key>
    <true/>
    <key>AbandonProcessGroup</key>
    <true/>
  </dict>
</plist>`

func renderDarwinJob(t *testing.T, args []string) string {
	t.Helper()
	tpl := template.Must(template.New("job").Parse(jobTemplate))
	var buf strings.Builder
	err := tpl.Execute(&buf, struct {
		Name string
		Exec []string
	}{Name: appName, Exec: args})
	if err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
