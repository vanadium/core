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
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
)

func TestDaemonRestart(t *testing.T) {
	cleanup, ctx, sh, envelope, root, helperPath, _ := utiltest.StartupHelper(t)
	defer cleanup()

	// Set up the device manager.  Since we won't do device manager updates,
	// don't worry about its application envelope and current link.
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Start()
	dm.S.Expect("READY")
	utiltest.ClaimDevice(t, ctx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)

	// Create the local server that the app uses to let us know it's ready.
	pingCh, cleanup := utiltest.SetupPingServer(t, ctx)
	defer cleanup()

	utiltest.Resolve(t, ctx, "pingserver", 1, true)

	const nRestarts = 5
	// Create an envelope for a first version of the app that will be restarted nRestarts times.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", nRestarts, 10*time.Minute, "appV1")
	appID := utiltest.InstallApp(t, ctx)

	// Start an instance of the app.
	instanceID := utiltest.LaunchApp(t, ctx, appID)

	// Wait until the app pings us that it's ready.
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	// Get application pid.
	pid := utiltest.GetPid(t, ctx, appID, instanceID)

	utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instanceID)

	for i := 0; i < nRestarts; i++ {
		syscall.Kill(int(pid), 9)
		utiltest.PollingWait(t, int(pid))

		// instanceID should be restarted automatically.

		// Be sure to get the ping from the restarted application so
		// that the app is running again before we ask for its status.
		pingCh.WaitForPingArgs(t)

		// WaitForState must be done after WaitForPingArgs for the
		// following reason: we need to make sure the app went through
		// the restart already, otherwise, it might be still in state
		// "running" since the reaper hasn't yet noticed it died.
		utiltest.WaitForState(t, ctx, device.InstanceStateRunning, appID, instanceID)
		// Get application pid.
		pid = utiltest.GetPid(t, ctx, appID, instanceID)
	}

	// Kill the application again.
	syscall.Kill(int(pid), 9)
	utiltest.PollingWait(t, int(pid))

	// The reaper should no longer restart the application:
	// instanceID is not running because it exceeded its restart limit.
	utiltest.WaitForState(t, ctx, device.InstanceStateNotRunning, appID, instanceID)
	// This clunky sleep helps ensure that the app stays dead (it briefly
	// transitioned through state 'not running' as part of a restart, so we
	// wait a bit to see if it stays dead).
	time.Sleep(time.Second)
	utiltest.VerifyState(t, ctx, device.InstanceStateNotRunning, appID, instanceID)

	// Cleanly shut down the device manager.
	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
	utiltest.VerifyNoRunningProcesses(t)
}
