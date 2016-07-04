// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package globsuid_test

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/device"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/deviced/internal/impl"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

var mockIsSetuid = flag.Bool("mocksetuid", false, "set flag to pretend to have a helper with setuid permissions")

func possiblyMockIsSetuid(ctx *context.T, fileStat os.FileInfo) bool {
	ctx.VI(2).Infof("Mock isSetuid is reporting: %v", *mockIsSetuid)
	return *mockIsSetuid
}

func init() {
	impl.IsSetuid = possiblyMockIsSetuid
}

func TestAppWithSuidHelper(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Use a common root identity provider so that all principals can talk to one
	// another.
	idp := testutil.NewIDProvider("root")
	if err := idp.Bless(v23.GetPrincipal(ctx), "self"); err != nil {
		t.Fatal(err)
	}

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

	selfCtx := ctx
	otherCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "other")

	// Create a script wrapping the test target that implements suidhelper.
	helperPath := utiltest.GenerateSuidHelperScript(t, root)

	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Args = append(dm.Args, "-mocksetuid")
	dm.Start()
	dm.S.Expect("READY")
	defer utiltest.VerifyNoRunningProcesses(t)
	// Claim the devicemanager with selfCtx as root:self:alice
	utiltest.ClaimDevice(t, selfCtx, "claimable", "dm", "alice", utiltest.NoPairingToken)

	deviceStub := device.DeviceClient("dm/device")

	// Create the local server that the app uses to tell us which system
	// name the device manager wished to run it as.
	pingCh, cleanup := utiltest.SetupPingServer(t, ctx)
	defer cleanup()

	// Create an envelope for a first version of the app.
	*envelope = utiltest.EnvelopeFromShell(sh, []string{utiltest.TestEnvVarName + "=env-var"}, []string{fmt.Sprintf("--%s=flag-val-envelope", utiltest.TestFlagName)}, utiltest.App, "google naps", 0, 0, "appV1")

	// Install and start the app as root:self.
	appID := utiltest.InstallApp(t, selfCtx)

	ctx.VI(2).Infof("Validate that the created app has the right permission lists.")
	perms, _, err := utiltest.AppStub(appID).GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions on appID: %v failed %v", appID, err)
	}
	expected := make(access.Permissions)
	for _, tag := range access.AllTypicalTags() {
		expected[string(tag)] = access.AccessList{In: []security.BlessingPattern{"root:self:$"}}
	}
	if got, want := perms.Normalize(), expected.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v", got, want)
	}

	// Start an instance of the app but this time it should fail: we do not
	// have an associated uname for the invoking identity.
	utiltest.LaunchAppExpectError(t, selfCtx, appID, verror.ErrNoAccess.ID)

	// Create an association for selfCtx
	if err := deviceStub.AssociateAccount(selfCtx, []string{"root:self"}, testUserName); err != nil {
		t.Fatalf("AssociateAccount failed %v", err)
	}

	instance1ID := utiltest.LaunchApp(t, selfCtx, appID)
	pingCh.VerifyPingArgs(t, testUserName, "flag-val-envelope", "env-var") // Wait until the app pings us that it's ready.
	utiltest.TerminateApp(t, selfCtx, appID, instance1ID)

	ctx.VI(2).Infof("other attempting to run an app without access. Should fail.")
	utiltest.LaunchAppExpectError(t, otherCtx, appID, verror.ErrNoAccess.ID)

	// Self will now let other also install apps.
	if err := deviceStub.AssociateAccount(selfCtx, []string{"root:other"}, testUserName); err != nil {
		t.Fatalf("AssociateAccount failed %v", err)
	}
	// Add Start to the AccessList list for root:other.
	newAccessList, _, err := deviceStub.GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed %v", err)
	}
	newAccessList.Add("root:other", string(access.Write))
	if err := deviceStub.SetPermissions(selfCtx, newAccessList, ""); err != nil {
		t.Fatalf("SetPermissions failed %v", err)
	}

	// With the introduction of per installation and per instance AccessLists,
	// while other now has administrator permissions on the device manager,
	// other doesn't have execution permissions for the app. So this will
	// fail.
	ctx.VI(2).Infof("other attempting to run an app still without access. Should fail.")
	utiltest.LaunchAppExpectError(t, otherCtx, appID, verror.ErrNoAccess.ID)

	// But self can give other permissions  to start applications.
	ctx.VI(2).Infof("self attempting to give other permission to start %s", appID)
	newAccessList, _, err = utiltest.AppStub(appID).GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions on appID: %v failed %v", appID, err)
	}
	newAccessList.Add("root:other", string(access.Read))
	if err = utiltest.AppStub(appID).SetPermissions(selfCtx, newAccessList, ""); err != nil {
		t.Fatalf("SetPermissions on appID: %v failed: %v", appID, err)
	}

	ctx.VI(2).Infof("other attempting to run an app with access. Should succeed.")
	instance2ID := utiltest.LaunchApp(t, otherCtx, appID)
	pingCh.VerifyPingArgs(t, testUserName, "flag-val-envelope", "env-var") // Wait until the app pings us that it's ready.

	ctx.VI(2).Infof("Validate that created instance has the right permissions.")
	expected = make(access.Permissions)
	for _, tag := range access.AllTypicalTags() {
		expected[string(tag)] = access.AccessList{In: []security.BlessingPattern{"root:other:$"}}
	}
	perms, _, err = utiltest.AppStub(appID, instance2ID).GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions on instance %v/%v failed: %v", appID, instance2ID, err)
	}
	if got, want := perms.Normalize(), expected.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}

	// Shutdown the app.
	utiltest.KillApp(t, otherCtx, appID, instance2ID)

	ctx.VI(2).Infof("Verify that Run with the same systemName works.")
	utiltest.RunApp(t, otherCtx, appID, instance2ID)
	pingCh.VerifyPingArgs(t, testUserName, "flag-val-envelope", "env-var") // Wait until the app pings us that it's ready.
	utiltest.KillApp(t, otherCtx, appID, instance2ID)

	ctx.VI(2).Infof("Verify that other can install and run applications.")
	otherAppID := utiltest.InstallApp(t, otherCtx)

	ctx.VI(2).Infof("other attempting to run an app that other installed. Should succeed.")
	instance4ID := utiltest.LaunchApp(t, otherCtx, otherAppID)
	pingCh.VerifyPingArgs(t, testUserName, "flag-val-envelope", "env-var") // Wait until the app pings us that it's ready.

	// Clean up.
	utiltest.TerminateApp(t, otherCtx, otherAppID, instance4ID)

	// Change the associated system name.
	if err := deviceStub.AssociateAccount(selfCtx, []string{"root:other"}, anotherTestUserName); err != nil {
		t.Fatalf("AssociateAccount failed %v", err)
	}

	ctx.VI(2).Infof("Show that Run with a different systemName fails.")
	utiltest.RunAppExpectError(t, otherCtx, appID, instance2ID, verror.ErrNoAccess.ID)

	// Clean up.
	utiltest.DeleteApp(t, otherCtx, appID, instance2ID)

	ctx.VI(2).Infof("Show that Start with different systemName works.")
	instance3ID := utiltest.LaunchApp(t, otherCtx, appID)
	pingCh.VerifyPingArgs(t, anotherTestUserName, "flag-val-envelope", "env-var") // Wait until the app pings us that it's ready.

	// Clean up.
	utiltest.TerminateApp(t, otherCtx, appID, instance3ID)
}
