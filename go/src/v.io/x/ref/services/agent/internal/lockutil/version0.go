// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lockutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
)

func makePsCommandV0(pid int) *exec.Cmd {
	return exec.Command("ps", "-o", "pid,lstart,user,comm", "-p", strconv.Itoa(pid))
}

// createV0 writes information about the current process (like its PID) to
// the specified writer.
func createV0(w io.Writer) error {
	cmd := makePsCommandV0(os.Getpid())
	cmd.Stdout = w
	cmd.Stderr = nil
	return cmd.Run()
}

var pidRegexV0 = regexp.MustCompile("\n\\s*(\\d+)")

func stillHeldV0(info []byte) (bool, error) {
	match := pidRegexV0.FindSubmatch(info)
	if match == nil {
		// Corrupt/invalid lockfile.
		return false, fmt.Errorf("failed to parse %s", string(info))
	}
	pid, err := strconv.Atoi(string(match[1]))
	if err != nil {
		// Corrupt/invalid lockfile.
		return false, fmt.Errorf("failed to parse %s", string(info))
	}
	// Go's os turns the standard errors into indecipherable ones,
	// so use syscall directly.
	if err := syscall.Kill(pid, syscall.Signal(0)); err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == syscall.ESRCH {
			return false, nil
		}
	}
	cmd := makePsCommandV0(pid)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return bytes.Equal(info, out), nil
}
