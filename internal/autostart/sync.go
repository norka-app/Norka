package autostart

// Sync makes the login entry match config.
// When enabled is false, an existing entry is removed.
// When enabled is true, the entry is created or rewritten so its command
// matches hidden: tray (--norka-hidden) or the plain executable (window).
// A matching entry is left as it is, including one written by an older version
// when hidden is false.
func Sync(enabled, hidden bool) error {
	present, err := IsEnabled()
	if err != nil {
		return err
	}
	if !enabled {
		if !present {
			return nil
		}
		return Disable()
	}
	if present {
		ok, matchErr := matches(hidden)
		if matchErr == nil && ok {
			return nil
		}
	}
	return Enable(hidden)
}
