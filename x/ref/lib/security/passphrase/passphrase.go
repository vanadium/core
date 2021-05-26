// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package passphrase contains utilities for reading a passphrase.
package passphrase

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"v.io/x/ref/internal/logger"
)

// Get reads a passphrase over stdin/stdout.
func Get(prompt string) ([]byte, error) {
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		// If the standard input is not a terminal, the password is
		// obtained by reading a line from it.
		return readPassphrase()
	}
	fmt.Print(prompt)
	stop := make(chan bool)
	defer close(stop)
	state, err := terminal.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	go catchTerminationSignals(stop, state)
	defer fmt.Printf("\n")
	return terminal.ReadPassword(int(os.Stdin.Fd()))
}

// readPassphrase reads from stdin until it sees '\n' or EOF.
func readPassphrase() ([]byte, error) {
	var pass []byte
	var total int
	for {
		b := make([]byte, 1)
		count, err := os.Stdin.Read(b)
		if count > 0 {
			if b[0] == '\n' {
				if err == nil {
					return pass[:total], nil
				}
			} else {
				total++
				pass = secureAppend(pass, b[0])
				// Clear out the byte.
				b[0] = 0
			}
		}
		if err == io.EOF {
			return pass[:total], nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func secureAppend(s []byte, t byte) []byte {
	res := append(s, t)
	if len(res) > cap(s) {
		// When append needs to allocate a new array, clear out the old
		// one.
		copy(s, make([]byte, len(s)))
	}
	// Clear out the byte.
	t = 0 //nolint:ineffassign
	return res
}

// catchTerminationSignals catches signals to allow us to turn terminal echo
// back on.
func catchTerminationSignals(stop <-chan bool, state *terminal.State) {
	var successErrno syscall.Errno
	sig := make(chan os.Signal, 4)
	// Catch the blockable termination signals.
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
	select {
	case <-sig:
		// Start on new line in terminal.
		fmt.Printf("\n")
		if err := terminal.Restore(int(os.Stdin.Fd()), state); err != successErrno {
			logger.Global().Errorf("Failed to restore terminal state (%v), your words may not show up when you type, enter 'stty echo' to fix this.", err)
		}
		os.Exit(-1)
	case <-stop:
		signal.Stop(sig)
	}
}
