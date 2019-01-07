// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package globsuid_test

import (
	"path"
	"testing"

	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
)

func TestDeviceManagerGlobAndDebug(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	sh, deferFn := servicetest.CreateShellAndMountTable(t, ctx)
	defer deferFn()

	// Set up mock application and binary repositories.
	envelope, cleanup := utiltest.StartMockRepos(t, ctx)
	defer cleanup()

	root, cleanup := servicetest.SetupRootDir(t, "devicemanager")
	defer cleanup()
	if err := versioning.SaveCreatorInfo(ctx, root); err != nil {
		t.Fatal(err)
	}

	// Create a script wrapping the test target that implements suidhelper.
	helperPath := utiltest.GenerateSuidHelperScript(t, root)

	// Set up the device manager.  Since we won't do device manager updates,
	// don't worry about its application envelope and current link.
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Start()
	dm.S.Expect("READY")

	// Create the local server that the app uses to let us know it's ready.
	pingCh, cleanup := utiltest.SetupPingServer(t, ctx)
	defer cleanup()

	// Create the envelope for the first version of the app.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 0, 0, "appV1")

	// Device must be claimed before applications can be installed.
	utiltest.ClaimDevice(t, ctx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)
	// Install the app.
	appID := utiltest.InstallApp(t, ctx)
	install1ID := path.Base(appID)

	// Start an instance of the app.
	instance1ID := utiltest.LaunchApp(t, ctx, appID)
	defer utiltest.TerminateApp(t, ctx, appID, instance1ID)

	// Wait until the app pings us that it's ready.
	pingCh.WaitForPingArgs(t)

	app2ID := utiltest.InstallApp(t, ctx)
	install2ID := path.Base(app2ID)

	// Base name of argv[0] that the app should have when it executes
	// It will be path.Base(envelope.Title + "@" + envelope.Binary.File + "/app").
	// Note the suffix, which ensures that the result is always "app" at the moment.
	// Someday in future we may remove that and have binary names that reflect the app name.
	const appName = "app"

	testcases := []utiltest.GlobTestVector{
		{"dm", "...", []string{
			"",
			"apps",
			"apps/google naps",
			"apps/google naps/" + install1ID,
			"apps/google naps/" + install1ID + "/" + instance1ID,
			"apps/google naps/" + install1ID + "/" + instance1ID + "/logs",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/logs/STDERR-<timestamp>",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/logs/STDOUT-<timestamp>",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/logs/" + appName + ".INFO",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/logs/" + appName + ".<*>.INFO.<timestamp>",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/pprof",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/stats",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/stats/rpc",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/stats/system",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/stats/system/start-time-rfc1123",
			"apps/google naps/" + install1ID + "/" + instance1ID + "/stats/system/start-time-unix",
			"apps/google naps/" + install2ID,
			"device",
		}},
		{"dm/apps", "*", []string{"google naps"}},
		{"dm/apps/google naps", "*", []string{install1ID, install2ID}},
		{"dm/apps/google naps/" + install1ID, "*", []string{instance1ID}},
		{"dm/apps/google naps/" + install1ID + "/" + instance1ID, "*", []string{"logs", "pprof", "stats"}},
		{"dm/apps/google naps/" + install1ID + "/" + instance1ID + "/logs", "*", []string{
			"STDERR-<timestamp>",
			"STDOUT-<timestamp>",
			appName + ".INFO",
			appName + ".<*>.INFO.<timestamp>",
		}},
		{"dm/apps/google naps/" + install1ID + "/" + instance1ID + "/stats/system", "start-time*", []string{"start-time-rfc1123", "start-time-unix"}},
	}

	res := utiltest.NewGlobTestRegexHelper(appName)

	utiltest.VerifyGlob(t, ctx, appName, testcases, res)
	utiltest.VerifyLog(t, ctx, "dm", "apps/google naps", install1ID, instance1ID, "logs", "*")
	utiltest.VerifyStatsValues(t, ctx, "dm", "apps/google naps", install1ID, instance1ID, "stats/system/start-time*")
	utiltest.VerifyPProfCmdLine(t, ctx, appName, "dm", "apps/google naps", install1ID, instance1ID, "pprof")
}
