//go:build unix

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func waitForStop() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	signal.Stop(ch)
}

func finishStop() {}
