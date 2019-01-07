// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utiltest

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	goexec "os/exec"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/rpc"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/services/device/deviced/internal/starter"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/device/internal/config"
	"v.io/x/ref/services/device/internal/suid"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

const (
	RedirectEnv    = "DEVICE_MANAGER_DONT_REDIRECT_STDOUT_STDERR"
	TestEnvVarName = "V23_RANDOM_ENV_VALUE"
	NoPairingToken = ""
)

// ExecScript launches the script passed as argument.
var ExecScript = gosh.RegisterFunc("ExecScript", func(script string) error {
	osenv := []string{RedirectEnv + "=1"}
	if os.Getenv("PAUSE_BEFORE_STOP") == "1" {
		osenv = append(osenv, "PAUSE_BEFORE_STOP=1")
	}
	cmd := goexec.Cmd{
		Path:   script,
		Env:    osenv,
		Stdin:  os.Stdin,
		Stderr: os.Stderr,
		Stdout: os.Stdout,
	}
	return cmd.Run()
})

// DeviceManager sets up a device manager server.  It accepts the name to
// publish the server under as an argument.  Additional arguments can optionally
// specify device manager config settings.
var DeviceManager = gosh.RegisterFunc("DeviceManager", deviceManagerFunc)

func waitForEOF(r io.Reader) {
	io.Copy(ioutil.Discard, r)
}

func deviceManagerFunc(publishName string, args ...string) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	defer fmt.Printf("%v terminated\n", publishName)
	defer ctx.VI(1).Infof("%v terminated", publishName)

	// Satisfy the contract described in doc.go by passing the config state
	// through to the device manager dispatcher constructor.
	configState, err := config.Load()
	if err != nil {
		ctx.Fatalf("Failed to decode config state: %v", err)
	}

	// This exemplifies how to override or set specific config fields, if,
	// for example, the device manager is invoked 'by hand' instead of via a
	// script prepared by a previous version of the device manager.
	var pairingToken string
	if len(args) > 0 {
		if want, got := 4, len(args); want > got {
			ctx.Fatalf("expected atleast %d additional arguments, got %d instead: %q", want, got, args)
		}
		configState.Root, configState.Helper, configState.Origin, configState.CurrentLink = args[0], args[1], args[2], args[3]
		if len(args) > 4 {
			pairingToken = args[4]
		}
	}
	// We grab the shutdown channel at this point in order to ensure that we
	// register a listener for the app cycle manager Stop before we start
	// running the device manager service.  Otherwise, any device manager
	// method that calls Stop on the app cycle manager (e.g. the Stop RPC)
	// will precipitate an immediate process exit.
	shutdownChan := signals.ShutdownOnSignals(ctx)
	listenSpec := rpc.ListenSpec{Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}}}
	blessings, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	claimableEps, stop, err := starter.Start(ctx, starter.Args{
		Namespace: starter.NamespaceArgs{
			ListenSpec: listenSpec,
		},
		Device: starter.DeviceArgs{
			Name:            publishName,
			ListenSpec:      listenSpec,
			ConfigState:     configState,
			TestMode:        strings.HasSuffix(fmt.Sprint(blessings), ":testdm"),
			RestartCallback: func() { fmt.Println("restart handler") },
			PairingToken:    pairingToken,
		},
		// TODO(rthellend): Wire up the local mounttable like the real device
		// manager, i.e. mount the device manager and the apps on it, and mount
		// the local mounttable in the global namespace.
		// MountGlobalNamespaceInLocalNamespace: true,
	})
	if err != nil {
		ctx.Errorf("starter.Start failed: %v", err)
		return err
	}
	defer stop()
	// Update the namespace roots to remove the server blessing from the
	// endpoints.  This is needed to be able to publish into the 'global'
	// mounttable before we have compatible credentials.
	ctx, err = SetNamespaceRootsForUnclaimedDevice(ctx)
	if err != nil {
		return err
	}
	// Manually mount the claimable service in the 'global' mounttable.
	for _, ep := range claimableEps {
		v23.GetNamespace(ctx).Mount(ctx, "claimable", ep.Name(), 0)
	}
	fmt.Println("READY")

	<-shutdownChan
	if os.Getenv("PAUSE_BEFORE_STOP") == "1" {
		waitForEOF(os.Stdin)
	}
	// TODO(ashankar): Figure out a way to incorporate this check in the test.
	// if impl.DispatcherLeaking(dispatcher) {
	//	ctx.Fatalf("device manager leaking resources")
	// }
	return nil
}

// This is the same as DeviceManager above, except that it has a different major
// version number.
var DeviceManagerV10 = gosh.RegisterFunc("DeviceManagerV10", func(publishName string, args ...string) error {
	versioning.CurrentVersion = versioning.Version{10, 0} // Set the version number to 10.0
	return deviceManagerFunc(publishName, args...)
})

func DeviceManagerCmd(sh *v23test.Shell, f *gosh.Func, args ...interface{}) *v23test.Cmd {
	dm := sh.FuncCmd(f, args...)
	// Make sure the device manager command is not provided with credentials.
	delete(dm.Vars, ref.EnvCredentials)
	delete(dm.Vars, ref.EnvAgentPath)
	return dm
}

func TestMainImpl(m *testing.M) {
	isSuidHelper := len(os.Getenv("V23_SUIDHELPER_TEST")) > 0
	if isSuidHelper {
		os.Exit(m.Run())
	}
	v23test.TestMain(m)
}

// TestSuidHelper is testing boilerplate for suidhelper that does not
// create a runtime because the suidhelper is not a Vanadium application.
func TestSuidHelperImpl(t *testing.T) {
	if os.Getenv("V23_SUIDHELPER_TEST") != "1" {
		return
	}
	logger.Global().VI(1).Infof("TestSuidHelper starting")
	if err := suid.Run(os.Environ()); err != nil {
		logger.Global().Fatalf("Failed to Run() setuidhelper: %v", err)
	}
	// Don't show "PASS"
	os.Exit(0)
}
