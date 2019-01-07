// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utiltest

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/mgmt"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/services/device/internal/suid"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

const (
	TestFlagName = "random_test_flag"
)

var flagValue = flag.String(TestFlagName, "default", "")

func init() {
	// The installer sets this flag on the installed device manager, so we
	// need to ensure it's defined.
	flag.String("name", "", "")
}

// appService defines a test service that the test app should be running.
// TODO(caprita): Use this to make calls to the app and verify how Kill
// interacts with an active service.
type appService struct{}

func (appService) Echo(_ *context.T, _ rpc.ServerCall, message string) (string, error) {
	return message, nil
}

func (appService) Cat(_ *context.T, _ rpc.ServerCall, file string) (string, error) {
	if file == "" || file[0] == filepath.Separator || file[0] == '.' {
		return "", fmt.Errorf("illegal file name: %q", file)
	}
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

type PingArgs struct {
	Username, FlagValue, EnvValue,
	DefaultPeerBlessings, PubBlessingPrefixes, InstanceName string
	Pid int
}

// ping makes an RPC from the App back to the invoking device manager
// carrying a PingArgs instance.
func ping(ctx *context.T, flagValue string) {
	ctx.Errorf("ping flagValue: %s", flagValue)

	helperEnv := os.Getenv(suid.SavedArgs)
	d := json.NewDecoder(strings.NewReader(helperEnv))
	var savedArgs suid.ArgsSavedForTest
	if err := d.Decode(&savedArgs); err != nil {
		ctx.Fatalf("Failed to decode preserved argument %v: %v", helperEnv, err)
	}
	args := &PingArgs{
		// TODO(rjkroege): Consider validating additional parameters
		// from helper.
		Username:             savedArgs.Uname,
		FlagValue:            flagValue,
		EnvValue:             os.Getenv(TestEnvVarName),
		Pid:                  os.Getpid(),
		DefaultPeerBlessings: v23.GetPrincipal(ctx).BlessingStore().ForPeer("nonexistent").String(),
	}

	config, err := exec.ReadConfigFromOSEnv()
	if config == nil || err != nil {
		vlog.Fatalf("Couldn't get Config: %v", err)
	} else {
		args.PubBlessingPrefixes, _ = config.Get(mgmt.PublisherBlessingPrefixesKey)
		args.InstanceName, _ = config.Get(mgmt.InstanceNameKey)
	}

	client := v23.GetClient(ctx)
	if call, err := client.StartCall(ctx, "pingserver", "Ping", []interface{}{args}); err != nil {
		ctx.Fatalf("StartCall failed: %v", err)
	} else if err := call.Finish(); err != nil {
		ctx.Fatalf("Finish failed: %v", err)
	}
}

// Cat is an RPC invoked from the test harness process to the application
// process.
func Cat(ctx *context.T, name, file string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, "Cat", []interface{}{file})
	if err != nil {
		return "", err
	}
	var content string
	if err := call.Finish(&content); err != nil {
		return "", err
	}
	return content, nil
}

// App is a test application. It pings the invoking device manager with state
// information.
var App = gosh.RegisterFunc("App", appFunc)

func appFunc(publishName string) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	ctx, server, err := v23.WithNewServer(ctx, publishName, new(appService), nil)
	if err != nil {
		ctx.Fatalf("NewServer(%v) failed: %v", publishName, err)
	}
	WaitForMount(ctx, ctx, publishName, server)
	// Some of our tests look for log files, so make sure they are flushed
	// to ensure that at least the files exist.
	ctx.FlushLog()

	shutdownChan := signals.ShutdownOnSignals(ctx)
	ping(ctx, *flagValue)

	<-shutdownChan
	if err := ioutil.WriteFile("testfile", []byte("goodbye world"), 0600); err != nil {
		ctx.Fatalf("Failed to write testfile: %v", err)
	}
	return nil
}

type PingServer struct {
	ing chan PingArgs
}

// TODO(caprita): Set the timeout in a more principled manner.
const pingTimeout = time.Minute

func (p PingServer) Ping(_ *context.T, _ rpc.ServerCall, arg PingArgs) error {
	p.ing <- arg
	return nil
}

// SetupPingServer creates a server listening for a ping from a child app; it
// returns a channel on which the app's ping message is returned, and a cleanup
// function.
func SetupPingServer(t *testing.T, ctx *context.T) (PingServer, func()) {
	pingCh := make(chan PingArgs, 1)
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewServer(ctx, "pingserver", PingServer{pingCh}, security.AllowEveryone())
	if err != nil {
		t.Fatalf("NewServer(%q, <dispatcher>) failed: %v", "pingserver", err)
	}
	WaitForMount(t, ctx, "pingserver", server)
	return PingServer{pingCh}, func() {
		cancel()
		<-server.Closed()
	}
}

func (p PingServer) WaitForPingArgs(t *testing.T) PingArgs {
	var args PingArgs
	select {
	case args = <-p.ing:
	case <-time.After(pingTimeout):
		t.Fatal(testutil.FormatLogLine(2, "failed to get ping"))
	}
	return args
}

func (p PingServer) VerifyPingArgs(t *testing.T, username, flagValue, envValue string) PingArgs {
	args := p.WaitForPingArgs(t)
	if args.Username != username || args.FlagValue != flagValue || args.EnvValue != envValue {
		t.Fatal(testutil.FormatLogLine(2, "got ping args %v, expected [username = %v, flag value = %v, env value = %v]", args, username, flagValue, envValue))
	}
	return args // Useful for tests that want to check other values in the PingArgs result.
}

// HangingApp is the same as App, except that it does not exit properly after
// being stopped.
var HangingApp = gosh.RegisterFunc("HangingApp", func(publishName string) error {
	err := appFunc(publishName)
	time.Sleep(24 * time.Hour)
	return err
})
