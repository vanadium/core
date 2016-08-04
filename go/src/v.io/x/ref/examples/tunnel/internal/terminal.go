// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package internal defines common types and functions used by both tunnel
// clients and servers.
package internal

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"v.io/v23/context"
)

// Winsize defines the window size used by ioctl TIOCGWINSZ and TIOCSWINSZ.
type Winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
}

// SetWindowSize sets the terminal's window size.
func SetWindowSize(fd uintptr, ws Winsize) error {
	ret, _, _ := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&ws)))
	if int(ret) == -1 {
		return errors.New("ioctl(TIOCSWINSZ) failed")
	}
	return nil
}

// GetWindowSize gets the terminal's window size.
func GetWindowSize() (*Winsize, error) {
	ws := &Winsize{}
	ret, _, _ := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))
	if int(ret) == -1 {
		return nil, errors.New("ioctl(TIOCGWINSZ) failed")
	}
	return ws, nil
}

// EnterRawTerminalMode uses stty to enter the terminal into raw mode; stdin is
// unbuffered, local echo of input characters is disabled, and special signal
// characters are disabled.  Returns a string which may be passed to
// RestoreTerminalSettings to restore to the original terminal settings.
func EnterRawTerminalMode(ctx *context.T) string {
	var savedBytes []byte
	var err error
	if savedBytes, err = exec.Command("stty", "-F", "/dev/tty", "-g").Output(); err != nil {
		ctx.Infof("Failed to save terminal settings: %q (%v)", savedBytes, err)
	}
	saved := strings.TrimSpace(string(savedBytes))

	args := []string{
		"-F", "/dev/tty",
		// Don't buffer stdin. Read characters as they are typed.
		"-icanon", "min", "1", "time", "0",
		// Turn off local echo of input characters.
		"-echo", "-echoe", "-echok", "-echonl",
		// Disable interrupt, quit, and suspend special characters.
		"-isig",
		// Ignore characters with parity errors.
		"ignpar",
		// Disable translate newline to carriage return.
		"-inlcr",
		// Disable ignore carriage return.
		"-igncr",
		// Disable translate carriage return to newline.
		"-icrnl",
		// Disable flow control.
		"-ixon", "-ixany", "-ixoff",
		// Disable non-POSIX special characters.
		"-iexten",
	}
	if out, err := exec.Command("stty", args...).CombinedOutput(); err != nil {
		ctx.Infof("stty failed (%v) (%q)", err, out)
	}

	return string(saved)
}

// RestoreTerminalSettings uses stty to restore the terminal to the original
// settings, taking the saved settings returned by EnterRawTerminalMode.
func RestoreTerminalSettings(ctx *context.T, saved string) {
	args := []string{
		"-F", "/dev/tty",
		saved,
	}
	if out, err := exec.Command("stty", args...).CombinedOutput(); err != nil {
		ctx.Infof("stty failed (%v) (%q)", err, out)
	}
}
