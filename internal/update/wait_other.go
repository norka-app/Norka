//go:build !windows

package update

// WaitForPIDExit is a no-op outside Windows. The relaunch switch is still
// stripped so a copied command line cannot surface in the UI.
func WaitForPIDExit(int) {}
