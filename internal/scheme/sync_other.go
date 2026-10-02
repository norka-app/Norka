//go:build !windows

package scheme

// Sync is a no-op outside Windows. macOS registers the scheme in Info.plist
// and Linux registers it with the packaged .desktop file. The app still
// ignores links while the automation flag is off.
func Sync(bool) error { return nil }
