package main

import "os"

// version is set with -X main.version at release time. A local build stays "dev".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], realEnv(version)))
}
