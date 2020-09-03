// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package signals_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"v.io/v23/context"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

func stopLoop(stdin io.Reader, ch chan<- struct{}) {
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		if scanner.Text() == "close" {
			close(ch)
			return
		}
	}
}

func program(cancelSelf bool, sigs ...os.Signal) {
	ctx, shutdown := test.V23Init()
	if cancelSelf {
		nctx, cancel := context.WithCancel(ctx)
		go cancel()
		ctx = nctx
	}
	closeStopLoop := make(chan struct{})
	// obtain ac here since stopLoop may execute after shutdown is called below
	go stopLoop(os.Stdin, closeStopLoop)
	wait := signals.ShutdownOnSignals(ctx, sigs...)
	fmt.Printf("ready\n")
	fmt.Printf("received signal %s\n", <-wait)
	shutdown()
	<-closeStopLoop
}

var handleDefaults = gosh.RegisterFunc("handleDefaults", func() {
	program(false)
})

var handleCustom = gosh.RegisterFunc("handleCustom", func() {
	program(false, syscall.SIGABRT)
})

var handleCancel = gosh.RegisterFunc("handlCancel", func() {
	program(true, syscall.SIGABRT)
})

var handleDefaultsIgnoreChan = gosh.RegisterFunc("handleDefaultsIgnoreChan", func() {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	closeStopLoop := make(chan struct{})
	// obtain ac here since stopLoop may execute after shutdown is called below
	go stopLoop(os.Stdin, closeStopLoop)
	signals.ShutdownOnSignals(ctx)
	fmt.Printf("ready\n")
	<-closeStopLoop
})

func programWithContext(cancelSelf bool, sig ...os.Signal) {
	ctx, shutdown := test.V23Init()
	defer func() {
		shutdown()
		fmt.Printf("shutdown complete\n")
	}()
	if cancelSelf {
		nctx, cancel := context.WithCancel(ctx)
		go cancel()
		ctx = nctx
	}
	ctx, handler := signals.ShutdownOnSignalsWithCancel(ctx)
	defer func() {
		sig := handler.WaitForSignal()
		fmt.Printf("received signal %v\n", sig)
		<-ctx.Done()
		fmt.Println(ctx.Err())
	}()
	fmt.Printf("ready\n")
}

var handleWithContext = gosh.RegisterFunc("handleWithContext", func() {
	programWithContext(false, syscall.SIGINT)
})

var handleWithSelfCancel = gosh.RegisterFunc("handleWithSelfCancel", func() {
	programWithContext(true, syscall.SIGINT)
})

func isSignalInSet(sig os.Signal, set []os.Signal) bool {
	for _, s := range set {
		if sig == s {
			return true
		}
	}
	return false
}

func checkSignalIsDefault(t *testing.T, sig os.Signal) {
	if !isSignalInSet(sig, signals.Default()) {
		t.Errorf("signal %s not in default signal set, as expected", sig)
	}
}

func checkSignalIsNotDefault(t *testing.T, sig os.Signal) {
	if isSignalInSet(sig, signals.Default()) {
		t.Errorf("signal %s unexpectedly in default signal set", sig)
	}
}

func startFunc(t *testing.T, sh *v23test.Shell, f *gosh.Func, exitErrorIsOk bool) (*v23test.Cmd, io.WriteCloser) {
	cmd := sh.FuncCmd(f)
	wc := cmd.StdinPipe()
	cmd.ExitErrorIsOk = exitErrorIsOk
	//	cmd.AddStdoutWriter(os.Stdout)
	//	cmd.AddStderrWriter(os.Stdout)
	cmd.Start()
	return cmd, wc
}

// TestCleanShutdownSignal verifies that sending a signal to a child that
// handles it by default causes the child to shut down cleanly.
func TestCleanShutdownSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleDefaults, false)
	cmd.S.Expect("ready")
	checkSignalIsDefault(t, syscall.SIGINT)
	cmd.Signal(syscall.SIGINT)
	cmd.S.Expectf("received signal %s", syscall.SIGINT)
	fmt.Fprintf(stdinPipe, "close\n")
	cmd.Wait()
}

// TestCleanContextCancel verifies that canceling the context will
// lead to a clean exit.
func TestCleanContextCancel(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleCancel, false)
	cmd.S.Expect("ready")
	cmd.S.Expectf("received signal %s", signals.ContextDoneSignal("context canceled"))
	fmt.Fprintf(stdinPipe, "close\n")
	cmd.Wait()
}

func checkExitStatus(t *testing.T, cmd *v23test.Cmd, code int) {
	if got, want := cmd.Err, fmt.Errorf("exit status %d", code); got.Error() != want.Error() {
		_, file, line, _ := runtime.Caller(1)
		file = filepath.Base(file)
		t.Errorf("%s:%d: got %q, want %q", file, line, got, want)
	}
}

// TestDoubleSignal verifies that sending a succession of two signals to a child
// that handles these signals by default causes the child to exit immediately
// upon receiving the second signal.
func TestDoubleSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, _ := startFunc(t, sh, handleDefaults, true)
	cmd.S.Expect("ready")
	checkSignalIsDefault(t, syscall.SIGTERM)
	cmd.Signal(syscall.SIGTERM)
	cmd.S.Expectf("received signal %s", syscall.SIGTERM)
	checkSignalIsDefault(t, syscall.SIGINT)
	cmd.Signal(syscall.SIGINT)
	cmd.Wait()
	checkExitStatus(t, cmd, signals.DoubleStopExitCode)
}

// TestSendUnhandledSignal verifies that sending a signal that the child does
// not handle causes the child to exit as per the signal being sent.
func TestSendUnhandledSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, _ := startFunc(t, sh, handleDefaults, true)
	cmd.S.Expect("ready")
	checkSignalIsNotDefault(t, syscall.SIGABRT)
	cmd.Signal(syscall.SIGABRT)
	cmd.Wait()
	checkExitStatus(t, cmd, 2)
}

// TestDoubleSignalIgnoreChan verifies that, even if we ignore the channel that
// ShutdownOnSignals returns, sending two signals should still cause the
// process to exit (ensures that there is no dependency in ShutdownOnSignals
// on having a goroutine read from the returned channel).
func TestDoubleSignalIgnoreChan(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, _ := startFunc(t, sh, handleDefaultsIgnoreChan, true)
	cmd.S.Expect("ready")
	// Even if we ignore the channel that ShutdownOnSignals returns,
	// sending two signals should still cause the process to exit.
	checkSignalIsDefault(t, syscall.SIGTERM)
	cmd.Signal(syscall.SIGTERM)
	checkSignalIsDefault(t, syscall.SIGINT)
	cmd.Signal(syscall.SIGINT)
	cmd.Wait()
	checkExitStatus(t, cmd, signals.DoubleStopExitCode)
}

// TestHandlerCustomSignal verifies that sending a non-default signal to a
// server that listens for that signal causes the server to shut down cleanly.
func TestHandlerCustomSignal(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleCustom, false)
	cmd.S.Expect("ready")
	checkSignalIsNotDefault(t, syscall.SIGABRT)
	cmd.Signal(syscall.SIGABRT)
	cmd.S.Expectf("received signal %s", syscall.SIGABRT)
	fmt.Fprintf(stdinPipe, "close\n")
	cmd.Wait()
}

// TestParseSignalsList verifies that ShutdownOnSignals correctly interprets
// the input list of signals.
func TestParseSignalsList(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	list := []os.Signal{syscall.SIGTERM}
	signals.ShutdownOnSignals(ctx, list...)
	if !isSignalInSet(syscall.SIGTERM, list) {
		t.Errorf("signal %s not in signal set, as expected: %v", syscall.SIGTERM, list)
	}
}

func TestCleanShutdownSignalWithContext(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	cmd, stdinPipe := startFunc(t, sh, handleWithContext, false)
	cmd.S.Expect("ready")
	cmd.Signal(syscall.SIGINT)
	cmd.S.Expectf("received signal %s", syscall.SIGINT)
	cmd.S.Expectf("context canceled")
	cmd.S.Expectf("shutdown complete")
	fmt.Fprintf(stdinPipe, "close\n")
	cmd.Wait()
}

func TestCleanShutdownSignalWithSelfCancel(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	cmd, stdinPipe := startFunc(t, sh, handleWithSelfCancel, false)
	cmd.S.Expect("ready")
	// No need to send a signal since handleWithSelfCancel will cancel
	// its context and hence exit as if it received the pseudo signal.
	cmd.S.Expectf("received signal %s", signals.ContextDoneSignal("context canceled"))
	cmd.S.Expectf("context canceled")
	cmd.S.Expectf("shutdown complete")
	fmt.Fprintf(stdinPipe, "close\n")
	cmd.Wait()
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
