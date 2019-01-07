// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package perms_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"os"
	"testing"

	"v.io/v23"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/device"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

// TestDeviceManagerClaim claims a devicemanager and tests AccessList
// permissions on its methods.
func TestDeviceManagerClaim(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Use a common root identity provider so that all principals can talk to one
	// another.
	idp := testutil.NewIDProvider("root")
	if err := idp.Bless(v23.GetPrincipal(ctx), "ctx"); err != nil {
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

	// Create a script wrapping the test target that implements suidhelper.
	helperPath := utiltest.GenerateSuidHelperScript(t, root)

	// Set up the device manager.  Since we won't do device manager updates,
	// don't worry about its application envelope and current link.
	pairingToken := "abcxyz"
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link", pairingToken)
	dm.Start()
	dm.S.Expect("READY")

	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 0, 0, "trapp")

	claimantCtx := utiltest.CtxWithNewPrincipal(t, ctx, idp, "claimant")
	octx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("other"))
	if err != nil {
		t.Fatal(err)
	}

	// Unclaimed devices cannot do anything but be claimed.
	// TODO(ashankar,caprita): The line below will currently fail with
	// ErrUnclaimedDevice != NotTrusted. NotTrusted can be avoided by
	// passing options.ServerAuthorizer{security.AllowEveryone()} to the
	// "Install" RPC.  Refactor the helper function to make this possible.
	//installAppExpectError(t, octx, impl.ErrUnclaimedDevice.ID)

	// Claim the device with an incorrect pairing token should fail.
	utiltest.ClaimDeviceExpectError(t, claimantCtx, "claimable", "mydevice", "badtoken", errors.ErrInvalidPairingToken.ID)
	// But succeed with a valid pairing token
	utiltest.ClaimDevice(t, claimantCtx, "claimable", "dm", "mydevice", pairingToken)

	// Installation should succeed since claimantRT is now the "owner" of
	// the devicemanager.
	appID := utiltest.InstallApp(t, claimantCtx)

	// octx will not install the app now since it doesn't recognize the
	// device's blessings. The error returned will be ErrNoServers as that
	// is what the IPC stack does when there are no authorized servers.
	utiltest.InstallAppExpectError(t, octx, verror.ErrNoServers.ID)
	// Even if it does recognize the device (by virtue of recognizing the
	// claimant), the device will not allow it to install.
	claimantB, _ := v23.GetPrincipal(claimantCtx).BlessingStore().Default()
	if err := security.AddToRoots(v23.GetPrincipal(octx), claimantB); err != nil {
		t.Fatal(err)
	}
	utiltest.InstallAppExpectError(t, octx, verror.ErrNoAccess.ID)

	// Create the local server that the app uses to let us know it's ready.
	pingCh, cleanup := utiltest.SetupPingServer(t, claimantCtx)
	defer cleanup()

	// Start an instance of the app.
	instanceID := utiltest.LaunchApp(t, claimantCtx, appID)

	// Wait until the app pings us that it's ready.
	pingCh.WaitForPingArgs(t)
	utiltest.Resolve(t, ctx, "trapp", 1, false)
	utiltest.KillApp(t, claimantCtx, appID, instanceID)

	// TODO(gauthamt): Test that AccessLists persist across devicemanager restarts
}

func TestDeviceManagerUpdateAccessList(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Use a common root identity provider so that all principals can talk to one
	// another.
	idp := testutil.NewIDProvider("root")
	ctx = utiltest.CtxWithNewPrincipal(t, ctx, idp, "self")

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
	octx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "other")

	// Set up the device manager.  Since we won't do device manager updates,
	// don't worry about its application envelope and current link.
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, "unused_helper", "unused_app_repo_name", "unused_curr_link")
	dm.Start()
	dm.S.Expect("READY")
	defer func() {
		dm.Terminate(os.Interrupt)
		dm.S.Expect("dm terminated")
		utiltest.VerifyNoRunningProcesses(t)
	}()

	// Create an envelope for an app.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 0, 0, "app")

	// On an unclaimed device manager, there will be no AccessLists.
	if _, _, err := device.DeviceClient("claimable").GetPermissions(selfCtx); err == nil {
		t.Fatalf("GetPermissions should have failed but didn't.")
	}

	// Claim the devicemanager as "root:self:mydevice"
	utiltest.ClaimDevice(t, selfCtx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)
	expectedAccessList := make(access.Permissions)
	for _, tag := range access.AllTypicalTags() {
		expectedAccessList[string(tag)] = access.AccessList{In: []security.BlessingPattern{"root:$", "root:self:$", "root:self:mydevice:$"}}
	}
	var b bytes.Buffer
	if err := access.WritePermissions(&b, expectedAccessList); err != nil {
		t.Fatalf("Failed to save AccessList:%v", err)
	}
	// Note, "version" below refers to the Permissions version, not the device
	// manager version.
	md5hash := md5.Sum(b.Bytes())
	expectedVersion := hex.EncodeToString(md5hash[:])
	deviceStub := device.DeviceClient("dm/device")
	perms, version, err := deviceStub.GetPermissions(selfCtx)
	if err != nil {
		t.Fatal(err)
	}
	if version != expectedVersion {
		t.Fatalf("getAccessList expected:%v(%v), got:%v(%v)", expectedAccessList, expectedVersion, perms, version)
	}
	// Install from octx should fail, since it does not match the AccessList.
	utiltest.InstallAppExpectError(t, octx, verror.ErrNoAccess.ID)

	newAccessList := make(access.Permissions)
	for _, tag := range access.AllTypicalTags() {
		newAccessList.Add("root:other", string(tag))
	}
	if err := deviceStub.SetPermissions(selfCtx, newAccessList, "invalid"); err == nil {
		t.Fatalf("SetPermissions should have failed with invalid version")
	}
	if err := deviceStub.SetPermissions(selfCtx, newAccessList, version); err != nil {
		t.Fatal(err)
	}
	// Install should now fail with selfCtx, which no longer matches the
	// AccessLists but succeed with octx, which does.
	utiltest.InstallAppExpectError(t, selfCtx, verror.ErrNoAccess.ID)
	utiltest.InstallApp(t, octx)
}
