// +build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func signalWindowSizeChange(winch chan os.Signal) {
	signal.Notify(winch, syscall.SIGWINCH)
}
