// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package applife_test

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/application"
	"v.io/v23/services/device"
	"v.io/x/ref"
	"v.io/x/ref/lib/mgmt"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func instanceDirForApp(root, appID, instanceID string) string {
	applicationDirName := func(title string) string {
		h := md5.New()
		h.Write([]byte(title))
		hash := strings.TrimRight(base64.URLEncoding.EncodeToString(h.Sum(nil)), "=")
		return "app-" + hash
	}
	components := strings.Split(appID, "/")
	appTitle, installationID := components[0], components[1]
	return filepath.Join(root, applicationDirName(appTitle), "installation-"+installationID, "instances", "instance-"+instanceID)
}

func verifyAppWorkspace(t *testing.T, root, appID, instanceID string) {
	// HACK ALERT: for now, we peek inside the device manager's directory
	// structure (which ought to be opaque) to check for what the app has
	// written to its local root.
	//
	// TODO(caprita): add support to device manager to browse logs/app local
	// root.
	rootDir := filepath.Join(instanceDirForApp(root, appID, instanceID), "root")
	testFile := filepath.Join(rootDir, "testfile")
	if read, err := ioutil.ReadFile(testFile); err != nil {
		t.Fatalf("Failed to read %v: %v", testFile, err)
	} else if want, got := "goodbye world", string(read); want != got {
		t.Fatalf("Expected to read %v, got %v instead", want, got)
	}
	// END HACK
}

// TestLifeOfAnApp installs an app, instantiates, runs, kills, and deletes
// several instances, and performs updates.
func TestLifeOfAnApp(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Get app publisher context (used later to publish apps)
	var pubCtx *context.T
	var err error
	if pubCtx, err = setupPublishingCredentials(ctx); err != nil {
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
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, helperPath, "unused_app_repo_name", "unused_curr_link")
	dm.Start()
	dm.S.Expect("READY")
	utiltest.ClaimDevice(t, ctx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)

	// Create the local server that the app uses to let us know it's ready.
	pingCh, cleanup := utiltest.SetupPingServer(t, ctx)
	defer cleanup()

	utiltest.Resolve(t, ctx, "pingserver", 1, true)

	// Create an envelope for a first version of the app.
	e, err := utiltest.SignedEnvelopeFromShell(pubCtx, sh, []string{utiltest.TestEnvVarName + "=env-val-envelope"}, []string{fmt.Sprintf("--%s=flag-val-envelope", utiltest.TestFlagName)}, utiltest.App, "google naps", 0, 0, "appV1")
	if err != nil {
		t.Fatalf("Unable to get signed envelope: %v", err)
	}
	*envelope = e

	// Install the app.  The config-specified flag value for testFlagName
	// should override the value specified in the envelope above, and the
	// config-specified value for origin should override the value in the
	// Install rpc argument.
	mtName, ok := sh.Vars[ref.EnvNamespacePrefix]
	if !ok {
		t.Fatalf("failed to get namespace root var from shell")
	}
	// This rooted name should be equivalent to the relative name "ar", but
	// we want to test that the config override for origin works.
	rootedAppRepoName := naming.Join(mtName, "ar")
	appID := utiltest.InstallApp(t, ctx, device.Config{utiltest.TestFlagName: "flag-val-install", mgmt.AppOriginConfigKey: rootedAppRepoName})
	v1 := utiltest.VerifyState(t, ctx, device.InstallationStateActive, appID)
	installationDebug := utiltest.Debug(t, ctx, appID)
	// We spot-check a couple pieces of information we expect in the debug
	// output.
	// TODO(caprita): Is there a way to verify more without adding brittle
	// logic that assumes too much about the format?  This may be one
	// argument in favor of making the output of Debug a struct instead of
	// free-form string.
	if !strings.Contains(installationDebug, fmt.Sprintf("Origin: %v", rootedAppRepoName)) {
		t.Fatalf("debug response doesn't contain expected info: %v", installationDebug)
	}
	if !strings.Contains(installationDebug, "Config: map[random_test_flag:flag-val-install]") {
		t.Fatalf("debug response doesn't contain expected info: %v", installationDebug)
	}

	// Start requires the caller to bless the app instance.
	expectedErr := "bless failed"
	if _, err := utiltest.LaunchAppImpl(t, ctx, appID, ""); err == nil || err.Error() != expectedErr {
		t.Fatalf("Start(%v) expected to fail with %v, got %v instead", appID, expectedErr, err)
	}

	// Start an instance of the app.
	instance1ID := utiltest.LaunchApp(t, ctx, appID)
	if v := utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance1ID); v != v1 {
		t.Fatalf("Instance version expected to be %v, got %v instead", v1, v)
	}

	instanceDebug := utiltest.Debug(t, ctx, appID, instance1ID)

	// Verify the app's default blessings.
	if def, _ := v23.GetPrincipal(ctx).BlessingStore().Default(); !strings.Contains(instanceDebug, fmt.Sprintf("Default Blessings                %s:forapp", def)) {
		t.Fatalf("debug response doesn't contain expected info: %v", instanceDebug)
	}

	// Verify the "..." blessing, which will include the publisher blessings
	verifyAppPeerBlessings(t, ctx, pubCtx, instanceDebug, envelope)

	// Wait until the app pings us that it's ready.
	pingResult := pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope")
	v1EP1 := utiltest.Resolve(t, ctx, "appV1", 1, true)[0]

	// Check that the instance name handed to the app looks plausible
	nameRE := regexp.MustCompile(".*/apps/google naps/[^/]+/[^/]+$")
	if nameRE.FindString(pingResult.InstanceName) == "" {
		t.Fatalf("Unexpected instance name: %v", pingResult.InstanceName)
	}

	// There should be at least one publisher blessing prefix, and all prefixes should
	// end in ":mydevice" because they are just the device manager's blessings
	prefixes := strings.Split(pingResult.PubBlessingPrefixes, ",")
	if len(prefixes) == 0 {
		t.Fatalf("No publisher blessing prefixes found: %v", pingResult)
	}
	for _, p := range prefixes {
		if !strings.HasSuffix(p, ":mydevice") {
			t.Fatalf("publisher Blessing prefixes don't look right: %v", pingResult.PubBlessingPrefixes)
		}
	}

	// We used a signed envelope, so there should have been some publisher blessings
	if !hasPrefixMatches(pingResult.PubBlessingPrefixes, pingResult.DefaultPeerBlessings) {
		t.Fatalf("Publisher Blessing Prefixes are not as expected: %v vs %v", pingResult.PubBlessingPrefixes, pingResult.DefaultPeerBlessings)
	}

	// Stop the app instance.
	utiltest.KillApp(t, ctx, appID, instance1ID)
	utiltest.VerifyState(t, ctx, device.InstanceStateNotRunning, appID, instance1ID)
	utiltest.ResolveExpectNotFound(t, ctx, "appV1", true)

	utiltest.RunApp(t, ctx, appID, instance1ID)
	utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance1ID)
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope") // Wait until the app pings us that it's ready.
	oldV1EP1 := v1EP1
	if v1EP1 = utiltest.Resolve(t, ctx, "appV1", 1, true)[0]; v1EP1 == oldV1EP1 {
		t.Fatalf("Expected a new endpoint for the app after kill/run")
	}

	// Start a second instance.
	instance2ID := utiltest.LaunchApp(t, ctx, appID)
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope") // Wait until the app pings us that it's ready.

	// There should be two endpoints mounted as "appV1", one for each
	// instance of the app.
	endpoints := utiltest.Resolve(t, ctx, "appV1", 2, true)
	v1EP2 := endpoints[0]
	if endpoints[0] == v1EP1 {
		v1EP2 = endpoints[1]
		if v1EP2 == v1EP1 {
			t.Fatalf("Both endpoints are the same")
		}
	} else if endpoints[1] != v1EP1 {
		t.Fatalf("Second endpoint should have been v1EP1: %v, %v", endpoints, v1EP1)
	}

	// TODO(caprita): verify various non-standard combinations (kill when
	// canceled; run while still running).

	// Kill the first instance.
	utiltest.KillApp(t, ctx, appID, instance1ID)
	// Only the second instance should still be running and mounted.
	// In this case, we don't want to retry since we shouldn't need to.
	if want, got := v1EP2, utiltest.Resolve(t, ctx, "appV1", 1, false)[0]; want != got {
		t.Fatalf("Resolve(%v): want: %v, got %v", "appV1", want, got)
	}

	// Updating the installation to itself is a no-op.
	utiltest.UpdateAppExpectError(t, ctx, appID, errors.ErrUpdateNoOp.ID)

	// Updating the installation should not work with a mismatched title.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "bogus", 0, 0, "bogus")

	utiltest.UpdateAppExpectError(t, ctx, appID, errors.ErrAppTitleMismatch.ID)

	// Create a second version of the app and update the app to it.
	*envelope = utiltest.EnvelopeFromShell(sh, []string{utiltest.TestEnvVarName + "=env-val-envelope"}, nil, utiltest.App, "google naps", 0, 0, "appV2")

	utiltest.UpdateApp(t, ctx, appID)

	v2 := utiltest.VerifyState(t, ctx, device.InstallationStateActive, appID)
	if v1 == v2 {
		t.Fatalf("Version did not change for %v: %v", appID, v1)
	}

	// Second instance should still be running, don't retry.
	if want, got := v1EP2, utiltest.Resolve(t, ctx, "appV1", 1, false)[0]; want != got {
		t.Fatalf("Resolve(%v): want: %v, got %v", "appV1", want, got)
	}
	if v := utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance2ID); v != v1 {
		t.Fatalf("Instance version expected to be %v, got %v instead", v1, v)
	}

	// Resume first instance.
	utiltest.RunApp(t, ctx, appID, instance1ID)
	if v := utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance1ID); v != v1 {
		t.Fatalf("Instance version expected to be %v, got %v instead", v1, v)
	}
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope") // Wait until the app pings us that it's ready.
	// Both instances should still be running the first version of the app.
	// Check that the mounttable contains two endpoints, one of which is
	// v1EP2.
	endpoints = utiltest.Resolve(t, ctx, "appV1", 2, true)
	if endpoints[0] == v1EP2 {
		if endpoints[1] == v1EP2 {
			t.Fatalf("Both endpoints are the same")
		}
	} else if endpoints[1] != v1EP2 {
		t.Fatalf("Second endpoint should have been v1EP2: %v, %v", endpoints, v1EP2)
	}

	// Trying to update first instance while it's running should fail.
	utiltest.UpdateInstanceExpectError(t, ctx, appID, instance1ID, errors.ErrInvalidOperation.ID)
	// Stop first instance and try again.
	utiltest.KillApp(t, ctx, appID, instance1ID)
	// Only the second instance should still be running and mounted, don't retry.
	if want, got := v1EP2, utiltest.Resolve(t, ctx, "appV1", 1, false)[0]; want != got {
		t.Fatalf("Resolve(%v): want: %v, got %v", "appV1", want, got)
	}
	// Update succeeds now.
	utiltest.UpdateInstance(t, ctx, appID, instance1ID)
	if v := utiltest.VerifyState(t, ctx, device.InstanceStateNotRunning, appID, instance1ID); v != v2 {
		t.Fatalf("Instance version expected to be %v, got %v instead", v2, v)
	}
	// Resume the first instance and verify it's running v2 now.
	utiltest.RunApp(t, ctx, appID, instance1ID)
	pingResult = pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope")
	utiltest.Resolve(t, ctx, "appV1", 1, false)
	utiltest.Resolve(t, ctx, "appV2", 1, false)

	// Although v2 does not have a signed envelope, this was an update of v1, which did.
	// This app's config still includes publisher blessing prefixes, and it should still
	// have the publisher blessing it acquired earlier.
	//
	// TODO: This behavior is non-ideal. A reasonable requirement in future would be that
	// the publisher blessing string remain unchanged on updates to an installation, just as the
	// title is not allowed to change.
	if !hasPrefixMatches(pingResult.PubBlessingPrefixes, pingResult.DefaultPeerBlessings) {
		t.Fatalf("Publisher Blessing Prefixes are not as expected: %v vs %v", pingResult.PubBlessingPrefixes, pingResult.DefaultPeerBlessings)
	}

	// Reverting first instance fails since it's still running.
	utiltest.RevertAppExpectError(t, ctx, appID+"/"+instance1ID, errors.ErrInvalidOperation.ID)
	// Stop first instance and try again.
	utiltest.KillApp(t, ctx, appID, instance1ID)
	verifyAppWorkspace(t, root, appID, instance1ID)
	utiltest.ResolveExpectNotFound(t, ctx, "appV2", true)
	utiltest.RevertApp(t, ctx, appID+"/"+instance1ID)
	// Resume the first instance and verify it's running v1 now.
	utiltest.RunApp(t, ctx, appID, instance1ID)
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope")
	utiltest.Resolve(t, ctx, "appV1", 2, false)
	utiltest.TerminateApp(t, ctx, appID, instance1ID)
	utiltest.Resolve(t, ctx, "appV1", 1, false)

	// Start a third instance.
	instance3ID := utiltest.LaunchApp(t, ctx, appID)
	if v := utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance3ID); v != v2 {
		t.Fatalf("Instance version expected to be %v, got %v instead", v2, v)
	}
	// Wait until the app pings us that it's ready.
	pingResult = pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope")
	// This app should not have publisher blessings. It was started from an installation
	// that did not have a signed envelope.
	if hasPrefixMatches(pingResult.PubBlessingPrefixes, pingResult.DefaultPeerBlessings) {
		t.Fatalf("Publisher Blessing Prefixes are not as expected: %v vs %v", pingResult.PubBlessingPrefixes, pingResult.DefaultPeerBlessings)
	}

	utiltest.Resolve(t, ctx, "appV2", 1, true)

	// Suspend second instance.
	utiltest.KillApp(t, ctx, appID, instance2ID)
	utiltest.ResolveExpectNotFound(t, ctx, "appV1", true)

	// Reverting second instance is a no-op since it's already running v1.
	utiltest.RevertAppExpectError(t, ctx, appID+"/"+instance2ID, errors.ErrUpdateNoOp.ID)

	// Stop third instance.
	utiltest.TerminateApp(t, ctx, appID, instance3ID)
	utiltest.ResolveExpectNotFound(t, ctx, "appV2", true)

	// Revert the app.
	utiltest.RevertApp(t, ctx, appID)
	if v := utiltest.VerifyState(t, ctx, device.InstallationStateActive, appID); v != v1 {
		t.Fatalf("Installation version expected to be %v, got %v instead", v1, v)
	}

	// Start a fourth instance.  It should be running from version 1.
	instance4ID := utiltest.LaunchApp(t, ctx, appID)
	if v := utiltest.VerifyState(t, ctx, device.InstanceStateRunning, appID, instance4ID); v != v1 {
		t.Fatalf("Instance version expected to be %v, got %v instead", v1, v)
	}
	pingCh.VerifyPingArgs(t, utiltest.UserName(t), "flag-val-install", "env-val-envelope") // Wait until the app pings us that it's ready.
	utiltest.Resolve(t, ctx, "appV1", 1, true)
	utiltest.TerminateApp(t, ctx, appID, instance4ID)
	utiltest.ResolveExpectNotFound(t, ctx, "appV1", true)

	// We are already on the first version, no further revert possible.
	utiltest.RevertAppExpectError(t, ctx, appID, errors.ErrUpdateNoOp.ID)

	// Uninstall the app.
	utiltest.UninstallApp(t, ctx, appID)
	utiltest.VerifyState(t, ctx, device.InstallationStateUninstalled, appID)

	// Updating the installation should no longer be allowed.
	utiltest.UpdateAppExpectError(t, ctx, appID, errors.ErrInvalidOperation.ID)

	// Reverting the installation should no longer be allowed.
	utiltest.RevertAppExpectError(t, ctx, appID, errors.ErrInvalidOperation.ID)

	// Starting new instances should no longer be allowed.
	utiltest.LaunchAppExpectError(t, ctx, appID, errors.ErrInvalidOperation.ID)

	// Make sure that Kill will actually kill an app that doesn't exit
	// cleanly Do this by installing, instantiating, running, and killing
	// hangingApp, which sleeps (rather than exits) after being asked to
	// Stop()
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.HangingApp, "hanging app", 0, 0, "hAppV1")
	hAppID := utiltest.InstallApp(t, ctx)
	hInstanceID := utiltest.LaunchApp(t, ctx, hAppID)
	hangingPid := pingCh.WaitForPingArgs(t).Pid
	if err := syscall.Kill(hangingPid, 0); err != nil && err != syscall.EPERM {
		t.Fatalf("Pid of hanging app (%v) is not live", hangingPid)
	}
	utiltest.KillApp(t, ctx, hAppID, hInstanceID)
	pidIsAlive := true
	for i := 0; i < 10 && pidIsAlive; i++ {
		if err := syscall.Kill(hangingPid, 0); err == nil || err == syscall.EPERM {
			time.Sleep(time.Second) // pid is still alive
		} else {
			pidIsAlive = false
		}
	}
	if pidIsAlive {
		t.Fatalf("Pid of hanging app (%d) has not exited after Stop() call", hangingPid)
	}

	// In the first pass, TidyNow (below), finds that everything should be too
	// young to be tidied becasue TidyNow's first call to MockableNow()
	// provides the current time.
	shouldKeepInstances := keepAll(t, root, filepath.Join(root, "app*", "installation*", "instances", "instance*"))
	shouldKeepInstallations := keepAll(t, root, filepath.Join(root, "app*", "installation*"))
	shouldKeepLogFiles := keepAll(t, root, filepath.Join(root, "app*", "installation*", "instances", "instance*", "logs", "*"))

	if err := utiltest.DeviceStub("dm").TidyNow(ctx); err != nil {
		t.Fatalf("TidyNow failed: %v", err)
	}

	verifyTidying(t, root, filepath.Join(root, "app*", "installation*", "instances", "instance*"), shouldKeepInstances)
	verifyTidying(t, root, filepath.Join(root, "app*", "installation*"), shouldKeepInstallations)
	verifyTidying(t, root, filepath.Join(root, "app*", "installation*", "instances", "instance*", "logs", "*"), shouldKeepLogFiles)

	// In the second pass, TidyNow() (below) calls MockableNow() again
	// which has advanced to tomorrow so it should find that all items have
	// become old enough to tidy.
	shouldKeepInstances = determineShouldKeep(t, root, filepath.Join(root, "app*", "installation*", "instances", "instance*"), "Deleted")
	shouldKeepInstallations = addBackLinks(t, root, determineShouldKeep(t, root, filepath.Join(root, "app*", "installation*"), "Uninstalled"))
	shouldKeepLogFiles = determineLogFilesToKeep(t, shouldKeepInstances)

	if err := utiltest.DeviceStub("dm").TidyNow(ctx); err != nil {
		t.Fatalf("TidyNow failed: %v", err)
	}

	verifyTidying(t, root, filepath.Join(root, "app*", "installation*", "instances", "instance*"), shouldKeepInstances)
	verifyTidying(t, root, filepath.Join(root, "app*", "installation*"), shouldKeepInstallations)
	verifyTidying(t, root, filepath.Join(root, "app*", "installation*", "instances", "instance*", "logs", "*"), shouldKeepLogFiles)

	// Cleanly shut down the device manager.
	dm.Terminate(os.Interrupt)
	dm.S.Expect("dm terminated")
	utiltest.VerifyNoRunningProcesses(t)
}

func keepAll(t *testing.T, root, globpath string) map[string]bool {
	paths, err := filepath.Glob(globpath)
	if err != nil {
		t.Errorf("keepAll %v", err)
	}
	shouldKeep := make(map[string]bool)
	for _, idir := range paths {
		shouldKeep[idir] = true
	}
	return shouldKeep
}

func determineShouldKeep(t *testing.T, root, globpath, state string) map[string]bool {
	paths, err := filepath.Glob(globpath)
	if err != nil {
		t.Errorf("determineShouldKeep %v", err)
	}

	shouldKeep := make(map[string]bool)
	for _, idir := range paths {
		p := filepath.Join(idir, state)
		_, err := os.Stat(p)
		if os.IsNotExist(err) {
			shouldKeep[idir] = true
		} else if err == nil {
			shouldKeep[idir] = false
		} else {
			t.Errorf("determineShouldKeep Stat(%s) failed: %v", p, err)
		}
	}
	return shouldKeep

}

func addBackLinks(t *testing.T, root string, installationShouldKeep map[string]bool) map[string]bool {
	paths, err := filepath.Glob(filepath.Join(root, "app*", "installation*", "instances", "instance*", "installation"))
	if err != nil {
		t.Errorf("addBackLinks %v", err)
	}

	for _, idir := range paths {
		pth, err := os.Readlink(idir)
		if err != nil {
			t.Errorf("addBackLinks %v", err)
			continue
		}
		if _, ok := installationShouldKeep[pth]; ok {
			// An instance symlinks to this pth so must be kept.
			installationShouldKeep[pth] = true
		}
	}
	return installationShouldKeep
}

// determineLogFilesToKeep produces a map of the log files that
// should remain after tidying. It returns a map to be compatible
// with the verifyTidying.
func determineLogFilesToKeep(t *testing.T, instances map[string]bool) map[string]bool {
	shouldKeep := make(map[string]bool)
	for idir, keep := range instances {
		if !keep {
			continue
		}

		paths, err := filepath.Glob(filepath.Join(idir, "logs", "*"))
		if err != nil {
			t.Errorf("determineLogFilesToKeep filepath.Glob(%s) failed: %v", idir, err)
			return shouldKeep
		}

		for _, p := range paths {
			fi, err := os.Stat(p)
			if err != nil {
				t.Errorf("determineLogFilesToKeep os.Stat(%s): %v", p, err)
				return shouldKeep
			}

			if fi.Mode()&os.ModeSymlink == 0 {
				continue
			}

			shouldKeep[p] = true
			target, err := os.Readlink(p)
			if err != nil {
				t.Errorf("determineLogFilesToKeep os.Readlink(%s): %v", p, err)
				return shouldKeep
			}
			shouldKeep[target] = true
		}
	}
	return shouldKeep
}

func verifyTidying(t *testing.T, root, globpath string, shouldKeep map[string]bool) {
	paths, err := filepath.Glob(globpath)
	if err != nil {
		t.Errorf("verifyTidying %v", err)
	}

	// TidyUp adds nothing: pth should be a subset of shouldKeep.
	for _, pth := range paths {
		if !shouldKeep[pth] {
			t.Errorf("TidyUp (%s) wrongly added path: %s", globpath, pth)
			return
		}
	}

	// Tidy should not leave unkept instances: shouldKeep ^ pth should be entirely true.
	for _, pth := range paths {
		if !shouldKeep[pth] {
			t.Errorf("TidyUp (%s) failed to delete: %s", globpath, pth)
			return
		}
	}

	// Tidy must not delete any kept instances.
	for k, v := range shouldKeep {
		if v {
			if _, err := os.Stat(k); os.IsNotExist(err) {
				t.Errorf("TidyUp (%s) deleted an instance it shouldn't have: %s", globpath, k)
			}
		}
	}
}

// setupPublishingCredentials creates two principals, which, in addition to the one passed in
// (which is "the user") allow us to have an "identity provider", and a "publisher". The
// user and the publisher are both blessed by the identity provider. The return value is
// a context that can be used to publish an envelope with a signed binary.
func setupPublishingCredentials(ctx *context.T) (*context.T, error) {
	IDPPrincipal := testutil.NewPrincipal("identitypro")
	IDPBlessing, _ := IDPPrincipal.BlessingStore().Default()

	PubPrincipal := testutil.NewPrincipal()
	UserPrincipal := v23.GetPrincipal(ctx)

	var b security.Blessings
	var c security.Caveat
	var err error
	if c, err = security.NewExpiryCaveat(time.Now().Add(time.Hour * 24 * 30)); err != nil {
		return nil, err
	}
	if b, err = IDPPrincipal.Bless(UserPrincipal.PublicKey(), IDPBlessing, "u:alice", c); err != nil {
		return nil, err
	}
	if err := vsecurity.SetDefaultBlessings(UserPrincipal, b); err != nil {
		return nil, err
	}

	if b, err = IDPPrincipal.Bless(PubPrincipal.PublicKey(), IDPBlessing, "m:publisher", security.UnconstrainedUse()); err != nil {
		return nil, err
	}
	if err := vsecurity.SetDefaultBlessings(PubPrincipal, b); err != nil {
		return nil, err
	}

	var pubCtx *context.T
	if pubCtx, err = v23.WithPrincipal(ctx, PubPrincipal); err != nil {
		return nil, err
	}

	return pubCtx, nil
}

// findPrefixMatches takes a set of comma-separated prefixes, and a set of comma-separated
// strings, and checks if any of the strings match any of the prefixes
func hasPrefixMatches(prefixList, stringList string) bool {
	prefixes := strings.Split(prefixList, ",")
	inStrings := strings.Split(stringList, ",")

	for _, s := range inStrings {
		for _, p := range prefixes {
			if strings.HasPrefix(s, p) {
				return true
			}
		}
	}
	return false
}

// verifyAppPeerBlessings checks the instanceDebug string to ensure that the app is running with
// the expected blessings for peer "..." (i.e. security.AllPrincipals) .
//
// The app should have one blessing that came from the user, of the form
// <base_blessing>:forapp. It should also have one or more publisher blessings, that are the
// cross product of the device manager blessings and the publisher blessings in the app
// envelope.
func verifyAppPeerBlessings(t *testing.T, ctx, pubCtx *context.T, instanceDebug string, e *application.Envelope) {
	// Extract the blessings from the debug output
	//
	// TODO(caprita): This is flaky, since the '...' peer pattern may not be
	// the first one in the sorted pattern list.  See v.io/i/680
	blessingRE := regexp.MustCompile(`Blessings\s?\n\s?\.\.\.\s*([^\n]+)`)
	blessingMatches := blessingRE.FindStringSubmatch(instanceDebug)
	if len(blessingMatches) < 2 {
		t.Fatalf("Failed to match blessing regex: [%v] [%v]", blessingMatches, instanceDebug)
	}
	blessingList := strings.Split(blessingMatches[1], ",")

	// Compute a map of the blessings we expect to find
	expBlessings := make(map[string]bool)
	baseBlessing, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	expBlessings[baseBlessing.String()+":forapp"] = false

	// App blessings should be the cross product of device manager and publisher blessings

	// dmBlessings below is a slice even though we have just one entry because we'll likely
	// want it to have more than one in future. (Today, a device manager typically has a
	// blessing from its claimer, but in many cases there might be other blessings too, such
	// as one from the manufacturer, or one from the organization that owns the device.)
	dmBlessings := []string{baseBlessing.String() + ":mydevice"}
	pubBlessings := strings.Split(e.Publisher.String(), ",")
	for _, dmb := range dmBlessings {
		for _, pb := range pubBlessings {
			expBlessings[dmb+":a:"+pb] = false
		}
	}

	// Check the list of blessings against the map of expected blessings
	matched := 0
	for _, b := range blessingList {
		if seen, ok := expBlessings[b]; ok && !seen {
			expBlessings[b] = true
			matched++
		}
	}
	if matched != len(expBlessings) {
		t.Fatalf("Missing some blessings in set %v. App blessings were: %v", expBlessings, blessingList)
	}
}
