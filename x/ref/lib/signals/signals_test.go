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

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/services/appcycle"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	vexec "v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/mgmt"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/device"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

func stopLoop(stop func(), stdin io.Reader, ch chan<- struct{}) {
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "close":
			close(ch)
			return
		case "stop":
			stop()
		}
	}
}

func program(sigs ...os.Signal) {
	ctx, shutdown := test.V23Init()
	closeStopLoop := make(chan struct{})
	// obtain ac here since stopLoop may execute after shutdown is called below
	ac := v23.GetAppCycle(ctx)
	go stopLoop(func() { ac.Stop(ctx) }, os.Stdin, closeStopLoop)
	wait := signals.ShutdownOnSignals(ctx, sigs...)
	fmt.Printf("ready\n")
	fmt.Printf("received signal %s\n", <-wait)
	shutdown()
	<-closeStopLoop
}

var handleDefaults = gosh.RegisterFunc("handleDefaults", func() {
	program()
})

var handleCustom = gosh.RegisterFunc("handleCustom", func() {
	program(syscall.SIGABRT)
})

var handleCustomWithStop = gosh.RegisterFunc("handleCustomWithStop", func() {
	program(signals.STOP, syscall.SIGABRT, syscall.SIGHUP)
})

var handleDefaultsIgnoreChan = gosh.RegisterFunc("handleDefaultsIgnoreChan", func() {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	closeStopLoop := make(chan struct{})
	// obtain ac here since stopLoop may execute after shutdown is called below
	ac := v23.GetAppCycle(ctx)
	go stopLoop(func() { ac.Stop(ctx) }, os.Stdin, closeStopLoop)
	signals.ShutdownOnSignals(ctx)
	fmt.Printf("ready\n")
	<-closeStopLoop
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

// TestCleanShutdownStop verifies that sending a stop command to a child that
// handles stop commands by default causes the child to shut down cleanly.
func TestCleanShutdownStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleDefaults, false)
	cmd.S.Expect("ready")
	fmt.Fprintf(stdinPipe, "stop\n")
	cmd.S.Expectf("received signal %s", v23.LocalStop)
	fmt.Fprintf(stdinPipe, "close\n")
	cmd.Wait()
}

// TestCleanShutdownStopCustom verifies that sending a stop command to a child
// that handles stop command as part of a custom set of signals handled, causes
// the child to shut down cleanly.
func TestCleanShutdownStopCustom(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleCustomWithStop, false)
	cmd.S.Expect("ready")
	fmt.Fprintf(stdinPipe, "stop\n")
	cmd.S.Expectf("received signal %s", v23.LocalStop)
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

// TestStopNoHandler verifies that sending a stop command to a child that does
// not handle stop commands causes the child to exit immediately.
func TestStopNoHandler(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleCustom, true)
	cmd.S.Expect("ready")
	fmt.Fprintf(stdinPipe, "stop\n")
	cmd.Wait()
	checkExitStatus(t, cmd, v23.UnhandledStopExitCode)
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

// TestSignalAndStop verifies that sending a signal followed by a stop command
// to a child that handles these by default causes the child to exit immediately
// upon receiving the stop command.
func TestSignalAndStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleDefaults, true)
	cmd.S.Expect("ready")
	checkSignalIsDefault(t, syscall.SIGTERM)
	cmd.Signal(syscall.SIGTERM)
	cmd.S.Expectf("received signal %s", syscall.SIGTERM)
	fmt.Fprintf(stdinPipe, "stop\n")
	cmd.Wait()
	checkExitStatus(t, cmd, signals.DoubleStopExitCode)
}

// TestDoubleStop verifies that sending a succession of stop commands to a child
// that handles stop commands by default causes the child to exit immediately
// upon receiving the second stop command.
func TestDoubleStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd, stdinPipe := startFunc(t, sh, handleDefaults, true)
	cmd.S.Expect("ready")
	fmt.Fprintf(stdinPipe, "stop\n")
	cmd.S.Expectf("received signal %s", v23.LocalStop)
	fmt.Fprintf(stdinPipe, "stop\n")
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

// TestHandlerCustomSignalWithStop verifies that sending a custom stop signal
// to a server that listens for that signal causes the server to shut down
// cleanly, even when a STOP signal is also among the handled signals.
func TestHandlerCustomSignalWithStop(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGABRT, syscall.SIGHUP} {
		func() {
			sh := v23test.NewShell(t, nil)
			defer sh.Cleanup()

			cmd, stdinPipe := startFunc(t, sh, handleCustomWithStop, false)
			cmd.S.Expect("ready")
			checkSignalIsNotDefault(t, signal)
			cmd.Signal(signal)
			cmd.S.Expectf("received signal %s", signal)
			fmt.Fprintf(stdinPipe, "close\n")
			cmd.Wait()
		}()
	}
}

// TestParseSignalsList verifies that ShutdownOnSignals correctly interprets
// the input list of signals.
func TestParseSignalsList(t *testing.T) {
	list := []os.Signal{signals.STOP, syscall.SIGTERM}
	signals.ShutdownOnSignals(nil, list...)
	if !isSignalInSet(syscall.SIGTERM, list) {
		t.Errorf("signal %s not in signal set, as expected: %v", syscall.SIGTERM, list)
	}
	if !isSignalInSet(signals.STOP, list) {
		t.Errorf("signal %s not in signal set, as expected: %v", signals.STOP, list)
	}
}

type configServer struct {
	ch chan<- string
}

func (c *configServer) Set(_ *context.T, _ rpc.ServerCall, key, value string) error {
	if key != mgmt.AppCycleManagerConfigKey {
		return fmt.Errorf("Unexpected key: %v", key)
	}
	c.ch <- value
	return nil

}

// TestCleanRemoteShutdown verifies that remote shutdown works correctly.
func TestCleanRemoteShutdown(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	ctx := sh.Ctx

	cmd := sh.FuncCmd(handleDefaults)

	ch := make(chan string, 1)
	_, server, err := v23.WithNewServer(ctx, "", device.ConfigServer(&configServer{ch}), securityflag.NewAuthorizerOrDie())
	if err != nil {
		t.Fatalf("WithNewServer failed: %v", err)
	}
	configServiceName := server.Status().Endpoints[0].Name()

	config := vexec.NewConfig()
	config.Set(mgmt.ParentNameConfigKey, configServiceName)
	config.Set(mgmt.ProtocolConfigKey, "tcp")
	config.Set(mgmt.AddressConfigKey, "127.0.0.1:0")
	config.Set(mgmt.SecurityAgentPathConfigKey, cmd.Vars[ref.EnvAgentPath])
	val, err := vexec.EncodeForEnvVar(config)
	if err != nil {
		t.Fatalf("encoding config failed %v", err)
	}
	cmd.Vars[vexec.V23_EXEC_CONFIG] = val
	stdin := cmd.StdinPipe()
	cmd.Start()

	appCycleName := <-ch
	cmd.S.Expect("ready")
	appCycle := appcycle.AppCycleClient(appCycleName)
	stream, err := appCycle.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	rStream := stream.RecvStream()
	if rStream.Advance() || rStream.Err() != nil {
		t.Errorf("Expected EOF, got (%v, %v) instead: ", rStream.Value(), rStream.Err())
	}
	if err := stream.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	cmd.S.Expectf("received signal %s", v23.RemoteStop)
	fmt.Fprintf(stdin, "close\n")
	cmd.S.ExpectEOF()
	cmd.Wait()
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
