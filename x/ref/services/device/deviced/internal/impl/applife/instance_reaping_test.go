// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package applife_test

import (
	"os"
	"syscall"
	"testing"

	"v.io/v23/naming"
	"v.io/v23/services/device"
	"v.io/v23/services/stats"
	"v.io/v23/vdl"

	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
)

func TestReaperNoticesAppDeath(t *testing.T) {
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

	// Create an envelope for a first version of the app.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 0, 0, "appV1")

	// Install the app.  The config-specified flag value for testFlagName
	// should override the value specified in the envelope above.
	appID := utiltest.InstallApp(t, ctx)

	// Start an instance of the app.
	instance1ID := utiltest.LaunchApp(t, ctx, appID)

	// Wait until the app pings us that it's ready.
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	// Get application pid.
	name := naming.Join("dm", "apps/"+appID+"/"+instance1ID+"/stats/system/pid")
	c := stats.StatsClient(name)
	v, err := c.Value(ctx)
	if err != nil {
		t.Fatalf("Value() failed: %v\n", err)
	}
	var pid int
	if err := vdl.Convert(&pid, v); err != nil {
		t.Fatalf("pid returned from stats interface is not an int: %v", err)
	}

	utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance1ID)
	syscall.Kill(int(pid), 9)

	// Start a second instance of the app which will force polling to happen.
	instance2ID := utiltest.LaunchApp(t, ctx, appID)
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance2ID)

	utiltest.TerminateApp(t, ctx, appID, instance2ID)
	utiltest.VerifyState(t, ctx, device.InstanceStateNotRunning, appID, instance1ID)

	// TODO(rjkroege): Exercise the polling loop code.

	// Cleanly shut down the device manager.
	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
	utiltest.VerifyNoRunningProcesses(t)
}
