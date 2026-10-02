package update

import (
	"os"
	"path/filepath"
	"regexp"
)

// relaunchScriptName matches the batch files written by Norka 1.2.0 while it
// waited for the previous process. Later versions do not create them.
var relaunchScriptName = regexp.MustCompile(`(?i)^norka-relaunch-[0-9]+\.cmd$`)

// CleanupStaleRelaunchScripts deletes leftover norka-relaunch-*.cmd files from
// the temp directory. A running update from 1.2.0 still uses that script; if
// the file is locked, removal is skipped and the script deletes itself.
func CleanupStaleRelaunchScripts() {
	removeStaleRelaunchScripts(os.TempDir())
}

func removeStaleRelaunchScripts(dir string) {
	matches, err := filepath.Glob(filepath.Join(dir, "norka-relaunch-*.cmd"))
	if err != nil {
		return
	}
	for _, name := range matches {
		if !relaunchScriptName.MatchString(filepath.Base(name)) {
			continue
		}
		info, err := os.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		_ = os.Remove(name)
	}
}
