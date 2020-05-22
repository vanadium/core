// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
	"testing"

	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

func simpleEchoProgram() {
	fmt.Printf("ready\n")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		fmt.Printf("%s\n", scanner.Text())
	}
	<-signals.ShutdownOnSignals(nil)
}

var withRuntime = gosh.RegisterFunc("withRuntime", func() {
	_, shutdown := test.V23Init()
	defer shutdown()
	simpleEchoProgram()
})

var withoutRuntime = gosh.RegisterFunc("withoutRuntime", func() {
	simpleEchoProgram()
})

func TestWithRuntime(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.PropagateChildOutput = true

	c := sh.FuncCmd(withRuntime)
	stdin := c.StdinPipe()
	c.Start()
	c.S.Expect("ready")
	// The Vanadium runtime spawns a goroutine that listens for SIGHUP and
	// prevents process exit.
	c.Signal(syscall.SIGHUP)
	stdin.Write([]byte("foo\n")) //nolint:errcheck
	c.S.Expect("foo")
	c.Terminate(os.Interrupt)
	c.S.ExpectEOF() //nolint:errcheck
}

func TestWithoutRuntime(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.PropagateChildOutput = true

	c := sh.FuncCmd(withoutRuntime)
	c.ExitErrorIsOk = true
	c.Start()
	c.S.Expect("ready")
	// Processes without a Vanadium runtime should exit on SIGHUP.
	c.Terminate(syscall.SIGHUP)
	c.S.ExpectEOF() //nolint:errcheck
}
