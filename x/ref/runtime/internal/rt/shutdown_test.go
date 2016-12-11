// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	"fmt"
	"io"
	"os"
	"testing"

	"v.io/v23"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/test/v23test"
)

var cstderr io.Writer

func init() {
	if testing.Verbose() {
		cstderr = os.Stderr
	}
}

func writeln(w io.Writer, s string) {
	w.Write([]byte(s + "\n"))
}

// TestSimpleServerSignal verifies that sending a signal to the simple server
// causes it to exit cleanly.
func TestSimpleServerSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(simpleServerProgram)
	stdin := c.StdinPipe()
	c.Start()
	c.S.Expect("Ready")
	c.Signal(os.Interrupt)
	c.S.Expect("Received signal interrupt")
	c.S.Expect("Interruptible cleanup")
	c.S.Expect("Deferred cleanup")
	writeln(stdin, "close")
	c.S.ExpectEOF()
}

// TestSimpleServerLocalStop verifies that sending a local stop command to the
// simple server causes it to exit cleanly.
func TestSimpleServerLocalStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(simpleServerProgram)
	stdin := c.StdinPipe()
	c.Start()
	c.S.Expect("Ready")
	writeln(stdin, "stop")
	c.S.Expect(fmt.Sprintf("Received signal %s", v23.LocalStop))
	c.S.Expect("Interruptible cleanup")
	c.S.Expect("Deferred cleanup")
	writeln(stdin, "close")
	c.S.ExpectEOF()
}

// TestSimpleServerDoubleSignal verifies that sending a succession of two
// signals to the simple server causes it to initiate the cleanup sequence on
// the first signal and then exit immediately on the second signal.
func TestSimpleServerDoubleSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(simpleServerProgram)
	c.ExitErrorIsOk = true
	c.Start()
	c.S.Expect("Ready")
	c.Signal(os.Interrupt)
	c.S.Expect("Received signal interrupt")
	c.Signal(os.Interrupt)
	c.Wait()
	if c.Err == nil {
		t.Fatalf("expected an error")
	}
	if got, want := c.Err.Error(), fmt.Sprintf("exit status %d", signals.DoubleStopExitCode); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSimpleServerLocalForceStop verifies that sending a local ForceStop
// command to the simple server causes it to exit immediately.
func TestSimpleServerLocalForceStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(simpleServerProgram)
	stdin := c.StdinPipe()
	c.ExitErrorIsOk = true
	c.Start()
	c.S.Expect("Ready")
	writeln(stdin, "forcestop")
	c.S.Expect("straight exit")
	c.Wait()
	if c.Err == nil {
		t.Fatalf("expected an error")
	}
	if got, want := c.Err.Error(), fmt.Sprintf("exit status %d", testForceStopExitCode); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSimpleServerKill demonstrates that a SIGKILL still forces the server
// to exit regardless of our signal handling.
func TestSimpleServerKill(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(simpleServerProgram)
	c.ExitErrorIsOk = true
	c.Start()
	c.S.Expect("Ready")
	c.Signal(os.Kill)
	c.Wait()
	if c.Err == nil {
		t.Fatalf("expected an error")
	}
	if got, want := c.Err.Error(), "signal: killed"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComplexServerSignal verifies that sending a signal to the complex server
// initiates the cleanup sequence in that server (we observe the printouts
// corresponding to all the simulated sequential/parallel and
// blocking/interruptible shutdown steps), and then exits cleanly.
func TestComplexServerSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(complexServerProgram)
	stdin := c.StdinPipe()
	c.Start()
	c.S.Expect("Ready")
	c.Signal(os.Interrupt)
	c.S.Expect("Received signal interrupt")
	c.S.ExpectSetRE("Sequential blocking cleanup",
		"Sequential interruptible cleanup",
		"Parallel blocking cleanup1",
		"Parallel blocking cleanup2",
		"Parallel interruptible cleanup1",
		"Parallel interruptible cleanup2")
	writeln(stdin, "close")
	c.S.ExpectEOF()
}

// TestComplexServerLocalStop verifies that sending a local stop command to the
// complex server initiates the cleanup sequence in that server (we observe the
// printouts corresponding to all the simulated sequential/parallel and
// blocking/interruptible shutdown steps), and then exits cleanly.
func TestComplexServerLocalStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(complexServerProgram)
	stdin := c.StdinPipe()
	c.Start()
	c.S.Expect("Ready")
	writeln(stdin, "stop")
	c.S.Expect(fmt.Sprintf("Stop %s", v23.LocalStop))
	c.S.ExpectSetRE(
		"Sequential blocking cleanup",
		"Sequential interruptible cleanup",
		"Parallel blocking cleanup1",
		"Parallel blocking cleanup2",
		"Parallel interruptible cleanup1",
		"Parallel interruptible cleanup2",
	)
	writeln(stdin, "close")
	c.S.ExpectEOF()
}

// TestComplexServerDoubleSignal verifies that sending a succession of two
// signals to the complex server has the expected effect: the first signal
// initiates the cleanup steps and the second signal kills the process, but only
// after the blocking shutdown steps were allowed to complete (as observed by
// the corresponding printouts from the server).  Note that we have no
// expectations on whether or not the interruptible shutdown steps execute.
func TestComplexServerDoubleSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(complexServerProgram)
	c.ExitErrorIsOk = true
	c.Start()
	c.S.Expect("Ready")
	c.Signal(os.Interrupt)
	c.S.Expect("Received signal interrupt")
	c.Signal(os.Interrupt)
	c.S.ExpectSetEventuallyRE(
		"Sequential blocking cleanup",
		"Parallel blocking cleanup1",
		"Parallel blocking cleanup2")
	c.Wait()
	if c.Err == nil {
		t.Fatalf("expected an error")
	}
	if got, want := c.Err.Error(), fmt.Sprintf("exit status %d", signals.DoubleStopExitCode); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComplexServerLocalForceStop verifies that sending a local ForceStop
// command to the complex server forces it to exit immediately.
func TestComplexServerLocalForceStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(complexServerProgram)
	stdin := c.StdinPipe()
	c.ExitErrorIsOk = true
	c.Start()
	c.S.Expect("Ready")
	writeln(stdin, "forcestop")
	c.S.Expect("straight exit")
	c.Wait()
	if c.Err == nil {
		t.Fatalf("expected an error")
	}
	if got, want := c.Err.Error(), fmt.Sprintf("exit status %d", testForceStopExitCode); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComplexServerKill demonstrates that a SIGKILL still forces the server to
// exit regardless of our signal handling.
func TestComplexServerKill(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	c := sh.FuncCmd(complexServerProgram)
	c.ExitErrorIsOk = true
	c.Start()
	c.S.Expect("Ready")
	c.Signal(os.Kill)
	c.Wait()
	if c.Err == nil {
		t.Fatalf("expected an error")
	}
	if got, want := c.Err.Error(), "signal: killed"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
