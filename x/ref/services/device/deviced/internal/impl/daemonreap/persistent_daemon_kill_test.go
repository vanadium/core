// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package daemonreap_test

import (
	"os"
	"syscall"
	"testing"
	"time"

	"v.io/v23/services/device"
	"v.io/x/ref"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
)

func TestReapRestartsDaemonMode(t *testing.T) {
	cleanup, ctx, sh, envelope, root, helperPath, _ := utiltest.StartupHelper(t)
	defer cleanup()

	// Start a device manager.
	// (Since it will be restarted, use the VeyronCredentials environment
	// to maintain the same set of credentials across runs)
	dmCreds := utiltest.CreatePrincipal(t, sh)
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Vars[ref.EnvCredentials] = dmCreds
	dm.Start()
	dm.S.Expect("READY")
	utiltest.ClaimDevice(t, ctx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)

	// Create the local server that the app uses to let us know it's ready.
	pingCh, cleanup := utiltest.SetupPingServer(t, ctx)
	defer cleanup()
	utiltest.Resolve(t, ctx, "pingserver", 1, true)

	// Create an envelope for a daemon app.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 10, time.Hour, "appV1")

	// Install the app.
	appID := utiltest.InstallApp(t, ctx)

	instance1 := utiltest.LaunchApp(t, ctx, appID)
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	// Get pid of first instance.
	pid := utiltest.GetPid(t, ctx, appID, instance1)

	// Shutdown the first device manager.
	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
	utiltest.ResolveExpectNotFound(t, ctx, "dm", false) // Ensure a clean slate.

	// Kill instance[0] and wait until it exits before proceeding.
	syscall.Kill(pid, 9)
	utiltest.PollingWait(t, int(pid))

	// Run another device manager to replace the dead one.
	dm = utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Vars[ref.EnvCredentials] = dmCreds
	dm.Start()

	defer func() {
		utiltest.TerminateApp(t, ctx, appID, instance1)
		dm.Terminate(os.Interrupt)
		dm.S.Expect("dm terminated")
		utiltest.VerifyNoRunningProcesses(t)
	}()

	dm.S.Expect("READY")
	utiltest.Resolve(t, ctx, "dm", 1, true) // Verify the device manager has published itself.

	// The app will ping us. Wait for it.
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	// By now, we've reconciled the state of the tree with which processes
	// are actually alive. instance1 was not alive but since it is configured as a
	// daemon, it will have been restarted.
	utiltest.WaitForState(t, ctx, device.InstanceStateRunning, appID, instance1)
}
