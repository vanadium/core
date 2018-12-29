// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package daemonreap_test

import (
	"os"
	"syscall"
	"testing"

	"v.io/v23/services/device"
	"v.io/x/ref"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
)

func TestReapReconciliationViaKill(t *testing.T) {
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

	// Create an envelope for the app.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 0, 0, "appV1")

	// Install the app.
	appID := utiltest.InstallApp(t, ctx)

	// Start three app instances.
	instances := make([]string, 3)
	for i, _ := range instances {
		instances[i] = utiltest.LaunchApp(t, ctx, appID)
		pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")
	}

	// Get pid of instance[0]
	pid := utiltest.GetPid(t, ctx, appID, instances[0])

	// Shutdown the first device manager.
	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
	utiltest.ResolveExpectNotFound(t, ctx, "dm", false) // Ensure a clean slate.

	// Kill instance[0] and wait until it exits before proceeding.
	syscall.Kill(pid, 9)
	utiltest.PollingWait(t, pid)

	// Run another device manager to replace the dead one.
	dm = utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Vars[ref.EnvCredentials] = dmCreds
	dm.Start()
	dm.S.Expect("READY")
	utiltest.Resolve(t, ctx, "dm", 1, true) // Verify the device manager has published itself.

	// By now, we've reconciled the state of the tree with which processes
	// are actually alive. instance-0 is not alive.
	expected := []device.InstanceState{device.InstanceStateNotRunning, device.InstanceStateRunning, device.InstanceStateRunning}
	for i, _ := range instances {
		utiltest.VerifyState(t, ctx, expected[i], appID, instances[i])
	}

	// Start instance[0] over-again to show that an app marked not running
	// by reconciliation can be restarted.
	utiltest.RunApp(t, ctx, appID, instances[0])
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	// Kill instance[1]
	pid = utiltest.GetPid(t, ctx, appID, instances[1])
	syscall.Kill(pid, 9)

	// Make a fourth instance. This forces a polling of processes so that
	// the state is updated.
	instances = append(instances, utiltest.LaunchApp(t, ctx, appID))
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	// Stop the fourth instance to make sure that there's no way we could
	// still be running the polling loop before doing the below.
	utiltest.TerminateApp(t, ctx, appID, instances[3])

	// Verify that reaper picked up the previous instances and was watching
	// instance[1]
	expected = []device.InstanceState{device.InstanceStateRunning, device.InstanceStateNotRunning, device.InstanceStateRunning, device.InstanceStateDeleted}
	for i, _ := range instances {
		utiltest.VerifyState(t, ctx, expected[i], appID, instances[i])
	}

	utiltest.TerminateApp(t, ctx, appID, instances[2])

	expected = []device.InstanceState{device.InstanceStateRunning, device.InstanceStateNotRunning, device.InstanceStateDeleted, device.InstanceStateDeleted}
	for i, _ := range instances {
		utiltest.VerifyState(t, ctx, expected[i], appID, instances[i])
	}
	utiltest.TerminateApp(t, ctx, appID, instances[0])

	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
	utiltest.VerifyNoRunningProcesses(t)
}
