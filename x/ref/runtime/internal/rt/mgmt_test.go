// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/services/appcycle"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	vexec "v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/mgmt"
	"v.io/x/ref/lib/security/securityflag"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/device"
	"v.io/x/ref/test"
	"v.io/x/ref/test/expect"
	"v.io/x/ref/test/v23test"
)

// TestBasic verifies that the basic plumbing works: LocalStop calls result in
// stop messages being sent on the channel passed to WaitForStop.
func TestBasic(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	m := v23.GetAppCycle(ctx)
	ch := make(chan string, 1)
	m.WaitForStop(ctx, ch)
	for i := 0; i < 10; i++ {
		m.Stop(ctx)
		if want, got := v23.LocalStop, <-ch; want != got {
			t.Errorf("WaitForStop want %q got %q", want, got)
		}
		select {
		case s := <-ch:
			t.Errorf("channel expected to be empty, got %q instead", s)
		default:
		}
	}
}

// TestMultipleWaiters verifies that the plumbing works with more than one
// registered wait channel.
func TestMultipleWaiters(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	m := v23.GetAppCycle(ctx)
	ch1 := make(chan string, 1)
	m.WaitForStop(ctx, ch1)
	ch2 := make(chan string, 1)
	m.WaitForStop(ctx, ch2)
	for i := 0; i < 10; i++ {
		m.Stop(ctx)
		if want, got := v23.LocalStop, <-ch1; want != got {
			t.Errorf("WaitForStop want %q got %q", want, got)
		}
		if want, got := v23.LocalStop, <-ch2; want != got {
			t.Errorf("WaitForStop want %q got %q", want, got)
		}
	}
}

// TestMultipleStops verifies that LocalStop does not block even if the wait
// channel is not being drained: once the channel's buffer fills up, future
// Stops become no-ops.
func TestMultipleStops(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	m := v23.GetAppCycle(ctx)
	ch := make(chan string, 1)
	m.WaitForStop(ctx, ch)
	for i := 0; i < 10; i++ {
		m.Stop(ctx)
	}
	if want, got := v23.LocalStop, <-ch; want != got {
		t.Errorf("WaitForStop want %q got %q", want, got)
	}
	select {
	case s := <-ch:
		t.Errorf("channel expected to be empty, got %q instead", s)
	default:
	}
}

func waitForEOF(r io.Reader) {
	io.Copy(ioutil.Discard, r)
}

var noWaiters = gosh.RegisterFunc("noWaiters", func() {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	v23.GetAppCycle(ctx).Stop(ctx)
	os.Exit(42) // This should not be reached.
})

// TestNoWaiters verifies that the child process exits in the absence of any
// wait channel being registered with its runtime.
func TestNoWaiters(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd := sh.FuncCmd(noWaiters)
	cmd.ExitErrorIsOk = true

	cmd.Run()
	want := fmt.Sprintf("exit status %d", testUnhandledStopExitCode)
	if err := cmd.Err; err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

var forceStop = gosh.RegisterFunc("forceStop", func() {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	m := v23.GetAppCycle(ctx)
	m.WaitForStop(ctx, make(chan string, 1))
	m.ForceStop(ctx)
	os.Exit(42) // This should not be reached.
})

// TestForceStop verifies that ForceStop causes the child process to exit
// immediately.
func TestForceStop(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	cmd := sh.FuncCmd(forceStop)
	cmd.ExitErrorIsOk = true

	cmd.Run()
	want := fmt.Sprintf("exit status %d", testForceStopExitCode)
	if err := cmd.Err; err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

func checkProgress(t *testing.T, ch <-chan v23.Task, progress, goal int32) {
	if want, got := (v23.Task{Progress: progress, Goal: goal}), <-ch; !reflect.DeepEqual(want, got) {
		t.Errorf("Unexpected progress: want %+v, got %+v", want, got)
	}
}

func checkNoProgress(t *testing.T, ch <-chan v23.Task) {
	select {
	case p := <-ch:
		t.Errorf("channel expected to be empty, got %+v instead", p)
	default:
	}
}

// TestProgress verifies that the ticker update/track logic works for a single
// tracker.
func TestProgress(t *testing.T) {
	ctx, shutdown := test.V23Init()

	m := v23.GetAppCycle(ctx)
	m.AdvanceGoal(50)
	ch := make(chan v23.Task, 1)
	m.TrackTask(ch)
	checkNoProgress(t, ch)
	m.AdvanceProgress(10)
	checkProgress(t, ch, 10, 50)
	checkNoProgress(t, ch)
	m.AdvanceProgress(5)
	checkProgress(t, ch, 15, 50)
	m.AdvanceGoal(50)
	checkProgress(t, ch, 15, 100)
	m.AdvanceProgress(1)
	checkProgress(t, ch, 16, 100)
	m.AdvanceGoal(10)
	checkProgress(t, ch, 16, 110)
	m.AdvanceProgress(-13)
	checkNoProgress(t, ch)
	m.AdvanceGoal(0)
	checkNoProgress(t, ch)
	shutdown()
	if _, ok := <-ch; ok {
		t.Errorf("Expected channel to be closed")
	}
}

// TestProgressMultipleTrackers verifies that the ticker update/track logic
// works for more than one tracker.  It also ensures that the runtime doesn't
// block when the tracker channels are full.
func TestProgressMultipleTrackers(t *testing.T) {
	ctx, shutdown := test.V23Init()

	m := v23.GetAppCycle(ctx)
	// ch1 is 1-buffered, ch2 is 2-buffered.
	ch1, ch2 := make(chan v23.Task, 1), make(chan v23.Task, 2)
	m.TrackTask(ch1)
	m.TrackTask(ch2)
	checkNoProgress(t, ch1)
	checkNoProgress(t, ch2)
	m.AdvanceProgress(1)
	checkProgress(t, ch1, 1, 0)
	checkNoProgress(t, ch1)
	checkProgress(t, ch2, 1, 0)
	checkNoProgress(t, ch2)
	for i := 0; i < 10; i++ {
		m.AdvanceProgress(1)
	}
	checkProgress(t, ch1, 2, 0)
	checkNoProgress(t, ch1)
	checkProgress(t, ch2, 2, 0)
	checkProgress(t, ch2, 3, 0)
	checkNoProgress(t, ch2)
	m.AdvanceGoal(4)
	checkProgress(t, ch1, 11, 4)
	checkProgress(t, ch2, 11, 4)
	shutdown()
	if _, ok := <-ch1; ok {
		t.Errorf("Expected channel to be closed")
	}
	if _, ok := <-ch2; ok {
		t.Errorf("Expected channel to be closed")
	}
}

var app = gosh.RegisterFunc("app", func() {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	m := v23.GetAppCycle(ctx)
	ch := make(chan string, 1)
	m.WaitForStop(ctx, ch)
	fmt.Println("ready")
	fmt.Printf("Got %s\n", <-ch)
	m.AdvanceGoal(10)
	fmt.Println("Doing some work")
	m.AdvanceProgress(2)
	fmt.Println("Doing some more work")
	m.AdvanceProgress(5)
})

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

func setupRemoteAppCycleMgr(t *testing.T) (*context.T, *v23test.Cmd, *expect.Session, appcycle.AppCycleClientMethods, func()) {
	sh := v23test.NewShell(t, nil)
	ctx := sh.Ctx
	failed := true
	defer func() {
		if failed {
			sh.Cleanup()
		}
	}()
	cmd := sh.FuncCmd(app)

	ch := make(chan string, 1)
	service := device.ConfigServer(&configServer{ch})
	authorizer := securityflag.NewAuthorizerOrDie()
	_, configServer, err := v23.WithNewServer(ctx, "", service, authorizer)
	if err != nil {
		t.Fatalf("WithNewServer failed: %v", err)
	}
	configServiceName := configServer.Status().Endpoints[0].Name()

	config := vexec.NewConfig()
	config.Set(mgmt.ParentNameConfigKey, configServiceName)
	config.Set(mgmt.ProtocolConfigKey, "tcp")
	config.Set(mgmt.AddressConfigKey, "127.0.0.1:0")
	config.Set(mgmt.SecurityAgentPathConfigKey, cmd.Vars[ref.EnvAgentPath])
	encodedConfig, err := vexec.EncodeForEnvVar(config)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Vars[vexec.V23_EXEC_CONFIG] = encodedConfig
	cmd.Start()

	appCycleName := ""
	select {
	case appCycleName = <-ch:
	case <-time.After(time.Minute):
		t.Fatalf("timeout")
	}
	cmd.S.Expect("ready")
	appCycle := appcycle.AppCycleClient(appCycleName)
	failed = false
	return ctx, cmd, cmd.S, appCycle, sh.Cleanup
}

// TestRemoteForceStop verifies that the child process exits when sending it
// a remote ForceStop rpc.
func TestRemoteForceStop(t *testing.T) {
	ctx, h, s, appCycle, cleanup := setupRemoteAppCycleMgr(t)
	defer cleanup()
	if err := appCycle.ForceStop(ctx); err == nil || !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("Expected EOF error, got %v instead", err)
	}
	h.ExitErrorIsOk = true
	s.ExpectEOF()
	want := fmt.Sprintf("exit status %d", testForceStopExitCode)
	h.Wait()
	if err := h.Err; err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

// TestRemoteStop verifies that the child shuts down cleanly when sending it
// a remote Stop rpc.
func TestRemoteStop(t *testing.T) {
	ctx, h, s, appCycle, cleanup := setupRemoteAppCycleMgr(t)
	defer cleanup()
	stream, err := appCycle.Stop(ctx)
	if err != nil {
		t.Fatalf("Got error: %v", err)
	}
	rStream := stream.RecvStream()
	expectTask := func(progress, goal int32) {
		if !rStream.Advance() {
			t.Fatalf("unexpected streaming error: %v", rStream.Err())
		}
		task := rStream.Value()
		if task.Progress != progress || task.Goal != goal {
			t.Errorf("Got (%d, %d), want (%d, %d)", task.Progress, task.Goal, progress, goal)
		}
	}
	expectTask(0, 10)
	expectTask(2, 10)
	expectTask(7, 10)
	if rStream.Advance() || rStream.Err() != nil {
		t.Errorf("Expected EOF, got (%v, %v) instead", rStream.Value(), rStream.Err())
	}
	if err := stream.Finish(); err != nil {
		t.Errorf("Got error %v", err)
	}
	s.Expect(fmt.Sprintf("Got %s", v23.RemoteStop))
	s.Expect("Doing some work")
	s.Expect("Doing some more work")
	s.ExpectEOF()
	h.Wait()
	if err := h.Err; err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}
