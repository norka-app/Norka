package diagnostics

import (
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

// GoVersion is the runtime Go version, for example go1.26.0.
func GoVersion() string {
	return runtime.Version()
}

// Commit is the VCS revision baked in by go build, or empty when there is none.
func Commit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return strings.TrimSpace(setting.Value)
		}
	}
	return ""
}

// ModuleVersion returns the build's version of modulePath, or empty.
func ModuleVersion(modulePath string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Path == modulePath {
		return strings.TrimSpace(info.Main.Version)
	}
	for _, mod := range info.Deps {
		if mod.Path == modulePath {
			return strings.TrimSpace(mod.Version)
		}
	}
	return ""
}

func beside(configPath, name string) string {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return name
	}
	return filepath.Join(filepath.Dir(configPath), name)
}
