// Command norkad keeps Norka tunnels running without the window.
// It is pure Go: no Wails and no system tray.
package main

import "os"

// version is set with -X main.version at release time. A local build stays "dev".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], realEnv()))
}
