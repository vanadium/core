// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package perms_test

import (
	"io/ioutil"
	"os"
	"testing"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/permissions"
	"v.io/v23/verror"

	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
	"v.io/x/ref/test/testutil"
)

func updateAccessList(t *testing.T, ctx *context.T, blessing, right string, name ...string) {
	accessStub := permissions.ObjectClient(naming.Join(name...))
	perms, version, err := accessStub.GetPermissions(ctx)
	if err != nil {
		t.Fatal(testutil.FormatLogLine(2, "GetPermissions(%v) failed %v", name, err))
	}
	perms.Add(security.BlessingPattern(blessing), right)
	if err = accessStub.SetPermissions(ctx, perms, version); err != nil {
		t.Fatal(testutil.FormatLogLine(2, "SetPermissions(%v, %v, %v) failed: %v", name, blessing, right, err))
	}
}

func testAccessFail(t *testing.T, expected verror.ID, ctx *context.T, who string, name ...string) {
	if _, err := utiltest.StatsStub(name...).Value(ctx); verror.ErrorID(err) != expected {
		t.Fatal(testutil.FormatLogLine(2, "%s got error %v but expected %v", who, err, expected))
	}
}

func TestDebugPermissionsPropagation(t *testing.T) {
	cleanup, ctx, sh, envelope, root, helperPath, idp := utiltest.StartupHelper(t)
	defer cleanup()

	// Set up the device manager.
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Start()
	dm.S.Expect("READY")
	utiltest.ClaimDevice(t, ctx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)

	// Create the local server that the app uses to let us know it's ready.
	pingCh, cleanup := utiltest.SetupPingServer(t, ctx)
	defer cleanup()
	utiltest.Resolve(t, ctx, "pingserver", 1, true)

	// Make some users.
	selfCtx := ctx
	bobCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "bob")
	hjCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "hackerjoe")
	aliceCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "alice")

	// TODO(rjkroege): Set AccessLists here that conflict with the one provided by the device
	// manager and show that the one set here is overridden.
	// Create the envelope for the first version of the app.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 0, 0, "appV1")

	// Install the app.
	appID := utiltest.InstallApp(t, ctx)

	// Give bob rights to start an app.
	updateAccessList(t, selfCtx, "root:bob:$", string(access.Read), "dm/apps", appID)

	// Bob starts an instance of the app.
	bobApp := utiltest.LaunchApp(t, bobCtx, appID)
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "default", "")

	// Bob permits Alice to read from his app.
	updateAccessList(t, bobCtx, "root:alice:$", string(access.Read), "dm/apps", appID, bobApp)

	// Create some globbing test vectors.
	globtests := []utiltest.GlobTestVector{
		{naming.Join("dm", "apps", appID, bobApp), "*",
			[]string{"logs", "pprof", "stats"},
		},
		{naming.Join("dm", "apps", appID, bobApp, "stats", "system"),
			"start-time*",
			[]string{"start-time-rfc1123", "start-time-unix"},
		},
		{naming.Join("dm", "apps", appID, bobApp, "logs"),
			"*",
			[]string{
				"STDERR-<timestamp>",
				"STDOUT-<timestamp>",
				"app.INFO",
				"app.<*>.INFO.<timestamp>",
			},
		},
	}
	appGlobtests := []utiltest.GlobTestVector{
		{naming.Join("appV1", "__debug"), "*",
			[]string{"logs", "pprof", "stats", "vtrace"},
		},
		{naming.Join("appV1", "__debug", "stats", "system"),
			"start-time*",
			[]string{"start-time-rfc1123", "start-time-unix"},
		},
		{naming.Join("appV1", "__debug", "logs"),
			"*",
			[]string{
				"STDERR-<timestamp>",
				"STDOUT-<timestamp>",
				"app.INFO",
				"app.<*>.INFO.<timestamp>",
			},
		},
	}
	globtestminus := globtests[1:]
	res := utiltest.NewGlobTestRegexHelper("app")

	// Confirm that self can access __debug names.
	utiltest.VerifyGlob(t, selfCtx, "app", globtests, res)
	utiltest.VerifyStatsValues(t, selfCtx, "dm", "apps", appID, bobApp, "stats/system/start-time*")
	utiltest.VerifyLog(t, selfCtx, "dm", "apps", appID, bobApp, "logs", "*")
	utiltest.VerifyPProfCmdLine(t, selfCtx, "app", "dm", "apps", appID, bobApp, "pprof")

	// Bob started the app so selfCtx can't connect to the app.
	utiltest.VerifyFailGlob(t, selfCtx, appGlobtests)
	testAccessFail(t, verror.ErrNoAccess.ID, selfCtx, "self", "appV1", "__debug", "stats/system/pid")

	// hackerjoe (for example) can't either.
	utiltest.VerifyFailGlob(t, hjCtx, appGlobtests)
	testAccessFail(t, verror.ErrNoAccess.ID, hjCtx, "hackerjoe", "appV1", "__debug", "stats/system/pid")

	// Bob has an issue with his app and tries to use the debug output to figure it out.
	utiltest.VerifyGlob(t, bobCtx, "app", globtests, res)
	utiltest.VerifyStatsValues(t, bobCtx, "dm", "apps", appID, bobApp, "stats/system/start-time*")
	utiltest.VerifyLog(t, bobCtx, "dm", "apps", appID, bobApp, "logs", "*")
	utiltest.VerifyPProfCmdLine(t, bobCtx, "app", "dm", "apps", appID, bobApp, "pprof")

	// Bob can also connect directly to his app.
	utiltest.VerifyGlob(t, bobCtx, "app", appGlobtests, res)
	utiltest.VerifyStatsValues(t, bobCtx, "appV1", "__debug", "stats/system/start-time*")

	// But Bob can't figure it out and hopes that hackerjoe can debug it.
	updateAccessList(t, bobCtx, "root:hackerjoe:$", string(access.Debug), "dm/apps", appID, bobApp)

	// Fortunately the device manager permits hackerjoe to access the stats.
	// But hackerjoe can't solve Bob's problem.
	// Because hackerjoe has Debug, hackerjoe can glob the __debug resources
	// of Bob's app but can't glob Bob's app.
	utiltest.VerifyGlob(t, hjCtx, "app", globtestminus, res)
	utiltest.VerifyFailGlob(t, hjCtx, globtests[0:1])
	utiltest.VerifyStatsValues(t, hjCtx, "dm", "apps", appID, bobApp, "stats", "system/start-time*")
	utiltest.VerifyLog(t, hjCtx, "dm", "apps", appID, bobApp, "logs", "*")
	utiltest.VerifyPProfCmdLine(t, hjCtx, "app", "dm", "apps", appID, bobApp, "pprof")

	// Permissions are propagated to the app so hackerjoe can connect
	// directly to the app too.
	utiltest.VerifyGlob(t, hjCtx, "app", globtestminus, res)
	utiltest.VerifyStatsValues(t, hjCtx, "appV1", "__debug", "stats/system/start-time*")

	// Alice might be able to help but Bob didn't give Alice access to the debug Permissionss.
	testAccessFail(t, verror.ErrNoAccess.ID, aliceCtx, "Alice", "dm", "apps", appID, bobApp, "stats/system/pid")

	// Bob forgets that Alice can't read the stats when he can.
	utiltest.VerifyGlob(t, bobCtx, "app", globtests, res)
	utiltest.VerifyStatsValues(t, bobCtx, "dm", "apps", appID, bobApp, "stats/system/start-time*")

	// So Bob changes the permissions so that Alice can help debug too.
	updateAccessList(t, bobCtx, "root:alice:$", string(access.Debug), "dm/apps", appID, bobApp)

	// Alice can access __debug content.
	utiltest.VerifyGlob(t, aliceCtx, "app", globtestminus, res)
	utiltest.VerifyFailGlob(t, aliceCtx, globtests[0:1])
	utiltest.VerifyStatsValues(t, aliceCtx, "dm", "apps", appID, bobApp, "stats", "system/start-time*")
	utiltest.VerifyLog(t, aliceCtx, "dm", "apps", appID, bobApp, "logs", "*")
	utiltest.VerifyPProfCmdLine(t, aliceCtx, "app", "dm", "apps", appID, bobApp, "pprof")

	// Alice can also now connect directly to the app.
	utiltest.VerifyGlob(t, aliceCtx, "app", globtestminus, res)
	utiltest.VerifyStatsValues(t, aliceCtx, "appV1", "__debug", "stats/system/start-time*")

	// Bob is glum because no one can help him fix his app so he terminates
	// it.
	utiltest.TerminateApp(t, bobCtx, appID, bobApp)

	// Cleanly shut down the device manager.
	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
}

func TestClaimSetsDebugPermissions(t *testing.T) {
	cleanup, ctx, sh, _, root, helperPath, idp := utiltest.StartupHelper(t)
	defer cleanup()

	extraLogDir, err := ioutil.TempDir(root, "testlogs")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}

	// Set up the device manager.
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused", "unused_curr_link")
	dm.Args = append(dm.Args, "--log_dir="+extraLogDir)
	dm.Start()
	dm.S.Expect("READY")

	// Make some users.
	selfCtx := ctx
	bobCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "bob")
	aliceCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "alice")
	hjCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "hackerjoe")

	// Bob claims the device manager.
	utiltest.ClaimDevice(t, bobCtx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)

	// Create some globbing test vectors.
	dmGlobtests := []utiltest.GlobTestVector{
		{naming.Join("dm", "__debug"), "*",
			[]string{"logs", "pprof", "stats", "vtrace"},
		},
		{naming.Join("dm", "__debug", "stats", "system"),
			"start-time*",
			[]string{"start-time-rfc1123", "start-time-unix"},
		},
		{naming.Join("dm", "__debug", "logs"),
			"*",
			[]string{
				// STDERR and STDOUT are not handled through the log package so
				// are not included here.
				"perms.test.INFO",
				"perms.test.<*>.INFO.<timestamp>",
			},
		},
	}
	res := utiltest.NewGlobTestRegexHelper(`perms\.test`)

	// Bob claimed the DM so can access it.
	utiltest.VerifyGlob(t, bobCtx, "perms.test", dmGlobtests, res)
	utiltest.VerifyStatsValues(t, bobCtx, "dm", "__debug", "stats/system/start-time*")

	// Without permissions, hackerjoe can't access the device manager.
	utiltest.VerifyFailGlob(t, hjCtx, dmGlobtests)
	testAccessFail(t, verror.ErrNoAccess.ID, hjCtx, "hackerjoe", "dm", "__debug", "stats/system/pid")

	// Bob gives system administrator Alice admin access to the dm and hence Alice
	// can access the __debug space.
	updateAccessList(t, bobCtx, "root:alice:$", string(access.Admin), "dm", "device")

	// Alice is an adminstrator and so can can access device manager __debug
	// values.
	utiltest.VerifyGlob(t, aliceCtx, "perms.test", dmGlobtests, res)
	utiltest.VerifyStatsValues(t, aliceCtx, "dm", "__debug", "stats/system/start-time*")

	// Bob gives debug access to the device manager to hackerjoe
	updateAccessList(t, bobCtx, "root:hackerjoe:$", string(access.Debug), "dm", "device")

	// hackerjoe can now access the device manager
	utiltest.VerifyGlob(t, hjCtx, "perms.test", dmGlobtests, res)
	utiltest.VerifyStatsValues(t, hjCtx, "dm", "__debug", "stats/system/start-time*")

	// Cleanly shut down the device manager.
	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
}
