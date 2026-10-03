package attach

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FindNorkad looks for the daemon next to the app executable.
// On macOS it also looks inside the .app bundle.
// The last resort is PATH, so a Homebrew or Scoop install is found
// when the window and norkad were packaged separately.
func FindNorkad(exe string) string {
	name := "norkad"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var candidates []string
	exe = strings.TrimSpace(exe)
	if exe != "" {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, name))
		if runtime.GOOS == "darwin" {
			candidates = append(candidates, bundleCandidates(dir, name)...)
		}
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return ""
}

func bundleCandidates(dir, name string) []string {
	for current := dir; current != "" && current != string(filepath.Separator); current = filepath.Dir(current) {
		if !strings.HasSuffix(current, ".app") {
			if current == filepath.Dir(current) {
				break
			}
			continue
		}
		return []string{
			filepath.Join(current, "Contents", "MacOS", name),
			filepath.Join(current, "Contents", "Resources", name),
		}
	}
	return nil
}
