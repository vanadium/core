// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(rjkroege): Add a more extensive unit test case to exercise AccessList
// logic.

package impl_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/application"
	"v.io/v23/services/device"
	"v.io/x/lib/envvar"
	"v.io/x/ref"
	"v.io/x/ref/services/device/deviced/internal/impl"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
	"v.io/x/ref/services/device/deviced/internal/installer"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/device/internal/config"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/internal/binarylib"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
	"v.io/x/ref/test/expect"
	"v.io/x/ref/test/testutil"
)

func TestMain(m *testing.M) {
	utiltest.TestMainImpl(m)
}

// TestSuidHelper is testing boilerplate for suidhelper that does not
// create a runtime because the suidhelper is not a Vanadium application.
func TestSuidHelper(t *testing.T) {
	utiltest.TestSuidHelperImpl(t)
}

// TODO(rjkroege): generateDeviceManagerScript and generateSuidHelperScript have
// code similarity that might benefit from refactoring.
// generateDeviceManagerScript is very similar in behavior to generateScript in
// device_invoker.go.  However, we chose to re-implement it here for two
// reasons: (1) avoid making generateScript public; and (2) how the test choses
// to invoke the device manager subprocess the first time should be independent
// of how device manager implementation sets up its updated versions.
func generateDeviceManagerScript(t *testing.T, root string, args, env []string) string {
	env = impl.VanadiumEnvironment(env)
	output := "#!" + impl.ShellPath + "\n"
	output += strings.Join(config.QuoteEnv(env), " ") + " exec "
	output += strings.Join(args, " ")
	if err := os.MkdirAll(filepath.Join(root, "factory"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	// Why pigeons? To show that the name we choose for the initial script
	// doesn't matter and in particular is independent of how device manager
	// names its updated version scripts (deviced.sh).
	path := filepath.Join(root, "factory", "pigeons.sh")
	if err := ioutil.WriteFile(path, []byte(output), 0755); err != nil {
		t.Fatalf("WriteFile(%v) failed: %v", path, err)
	}
	return path
}

// TestDeviceManagerUpdateAndRevert makes the device manager go through the
// motions of updating itself to newer versions (twice), and reverting itself
// back (twice). It also checks that update and revert fail when they're
// supposed to. The initial device manager is running 'by hand' via a module
// command. Further versions are running through the soft link that the device
// manager itself updates.
func TestDeviceManagerUpdateAndRevert(t *testing.T) {
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

	// Current link does not have to live in the root dir, but it's
	// convenient to put it there so we have everything in one place.
	currLink := filepath.Join(root, "current_link")

	// Since the device manager will be restarting, use the
	// VeyronCredentials environment variable to maintain the same set of
	// credentials across runs.
	// Without this, authentication/authorization state - such as the blessings of
	// the device manager and the signatures used for AccessList integrity checks
	// - will not carry over between updates to the binary, which would not be
	// reflective of intended use.
	dmVars := map[string]string{ref.EnvCredentials: utiltest.CreatePrincipal(t, sh)}
	dmArgs := []interface{}{"factoryDM", root, "unused_helper", utiltest.MockApplicationRepoName, currLink}
	dmCmd := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, dmArgs...)
	dmCmd.Vars = envvar.MergeMaps(dmCmd.Vars, dmVars)
	scriptPathFactory := generateDeviceManagerScript(t, root, dmCmd.Args, envvar.MapToSlice(dmCmd.Vars))

	if err := os.Symlink(scriptPathFactory, currLink); err != nil {
		t.Fatalf("Symlink(%q, %q) failed: %v", scriptPathFactory, currLink, err)
	}

	// We instruct the initial device manager that we run to pause before
	// stopping its service, so that we get a chance to verify that
	// attempting an update while another one is ongoing will fail.
	dmPauseBeforeStopVars := envvar.MergeMaps(dmVars, map[string]string{"PAUSE_BEFORE_STOP": "1"})

	// Start the initial version of the device manager, the so-called
	// "factory" version. We use the v23test-generated command to start it.
	// We could have also used the scriptPathFactory to start it, but this
	// demonstrates that the initial device manager could be running by hand
	// as long as the right initial configuration is passed into the device
	// manager implementation.
	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, dmArgs...)
	dm.Vars = envvar.MergeMaps(dm.Vars, dmPauseBeforeStopVars)
	dmStdin := dm.StdinPipe()
	dm.Start()
	dm.S.Expect("READY")
	defer func() {
		dm.Terminate(os.Interrupt)
		utiltest.VerifyNoRunningProcesses(t)
	}()

	utiltest.Resolve(t, ctx, "claimable", 1, true)
	// Brand new device manager must be claimed first.
	utiltest.ClaimDevice(t, ctx, "claimable", "factoryDM", "mydevice", utiltest.NoPairingToken)

	if v := utiltest.VerifyDeviceState(t, ctx, device.InstanceStateRunning, "factoryDM"); v != "factory" {
		t.Errorf("Expected factory version, got %v instead", v)
	}

	// Simulate an invalid envelope in the application repository.
	*envelope = utiltest.EnvelopeFromShell(sh, envvar.MapToSlice(dmVars), nil, utiltest.DeviceManager, "bogus", 0, 0, dmArgs...)

	utiltest.UpdateDeviceExpectError(t, ctx, "factoryDM", errors.ErrAppTitleMismatch.ID)
	utiltest.RevertDeviceExpectError(t, ctx, "factoryDM", errors.ErrUpdateNoOp.ID)

	// Set up a second version of the device manager. The information in the
	// envelope will be used by the device manager to stage the next
	// version.
	*envelope = utiltest.EnvelopeFromShell(sh, envvar.MapToSlice(dmVars), nil, utiltest.DeviceManager, application.DeviceManagerTitle, 0, 0, "v2DM")
	utiltest.UpdateDevice(t, ctx, "factoryDM")

	// Current link should have been updated to point to v2.
	evalLink := func() string {
		path, err := filepath.EvalSymlinks(currLink)
		if err != nil {
			t.Fatalf("EvalSymlinks(%v) failed: %v", currLink, err)
		}
		return path
	}
	scriptPathV2 := evalLink()
	if scriptPathFactory == scriptPathV2 {
		t.Fatalf("current link didn't change")
	}
	v2 := utiltest.VerifyDeviceState(t, ctx, device.InstanceStateUpdating, "factoryDM")

	utiltest.UpdateDeviceExpectError(t, ctx, "factoryDM", errors.ErrOperationInProgress.ID)

	dmStdin.Close()
	dm.S.Expect("restart handler")
	dm.S.Expect("factoryDM terminated")

	// A successful update means the device manager has stopped itself.  We
	// relaunch it from the current link.
	utiltest.ResolveExpectNotFound(t, ctx, "v2DM", false) // Ensure a clean slate.

	dm = sh.FuncCmd(utiltest.ExecScript, currLink)
	dm.Vars = envvar.MergeMaps(dm.Vars, dmVars)
	dm.Start()
	dm.S.Expect("READY")

	utiltest.Resolve(t, ctx, "v2DM", 1, true) // Current link should have been launching v2.

	// Try issuing an update without changing the envelope in the
	// application repository: this should fail, and current link should be
	// unchanged.
	utiltest.UpdateDeviceExpectError(t, ctx, "v2DM", errors.ErrUpdateNoOp.ID)
	if evalLink() != scriptPathV2 {
		t.Fatalf("script changed")
	}

	// Try issuing an update with a binary that has a different major version
	// number. It should fail.
	utiltest.ResolveExpectNotFound(t, ctx, "v2.5DM", false) // Ensure a clean slate.
	*envelope = utiltest.EnvelopeFromShell(sh, envvar.MapToSlice(dmVars), nil, utiltest.DeviceManagerV10, application.DeviceManagerTitle, 0, 0, "v2.5DM")
	utiltest.UpdateDeviceExpectError(t, ctx, "v2DM", errors.ErrOperationFailed.ID)

	if evalLink() != scriptPathV2 {
		t.Fatalf("script changed")
	}

	// Create a third version of the device manager and issue an update.
	*envelope = utiltest.EnvelopeFromShell(sh, envvar.MapToSlice(dmVars), nil, utiltest.DeviceManager, application.DeviceManagerTitle, 0, 0, "v3DM")
	utiltest.UpdateDevice(t, ctx, "v2DM")

	scriptPathV3 := evalLink()
	if scriptPathV3 == scriptPathV2 {
		t.Fatalf("current link didn't change")
	}

	dm.S.Expect("restart handler")
	dm.S.Expect("v2DM terminated")

	utiltest.ResolveExpectNotFound(t, ctx, "v3DM", false) // Ensure a clean slate.

	// Re-launch the device manager from current link.  We instruct the
	// device manager to pause before stopping its server, so that we can
	// verify that a second revert fails while a revert is in progress.
	dm = sh.FuncCmd(utiltest.ExecScript, currLink)
	dm.Vars = envvar.MergeMaps(dm.Vars, dmPauseBeforeStopVars)
	dmStdin = dm.StdinPipe()
	dm.Start()
	dm.S.Expect("READY")

	utiltest.Resolve(t, ctx, "v3DM", 1, true) // Current link should have been launching v3.
	v3 := utiltest.VerifyDeviceState(t, ctx, device.InstanceStateRunning, "v3DM")
	if v2 == v3 {
		t.Fatalf("version didn't change")
	}

	// Revert the device manager to its previous version (v2).
	utiltest.RevertDevice(t, ctx, "v3DM")
	utiltest.RevertDeviceExpectError(t, ctx, "v3DM", errors.ErrOperationInProgress.ID) // Revert already in progress.
	dmStdin.Close()
	dm.S.Expect("restart handler")
	dm.S.Expect("v3DM terminated")
	if evalLink() != scriptPathV2 {
		t.Fatalf("current link was not reverted correctly")
	}

	utiltest.ResolveExpectNotFound(t, ctx, "v2DM", false) // Ensure a clean slate.

	dm = sh.FuncCmd(utiltest.ExecScript, currLink)
	dm.Vars = envvar.MergeMaps(dm.Vars, dmVars)
	dm.Start()
	dm.S.Expect("READY")

	utiltest.Resolve(t, ctx, "v2DM", 1, true) // Current link should have been launching v2.

	// Revert the device manager to its previous version (factory).
	utiltest.RevertDevice(t, ctx, "v2DM")
	dm.S.Expect("restart handler")
	dm.S.Expect("v2DM terminated")
	dm.S.ExpectEOF()
	if evalLink() != scriptPathFactory {
		t.Fatalf("current link was not reverted correctly")
	}

	utiltest.ResolveExpectNotFound(t, ctx, "factoryDM", false) // Ensure a clean slate.

	dm = sh.FuncCmd(utiltest.ExecScript, currLink)
	dm.Vars = envvar.MergeMaps(dm.Vars, dmVars)
	dm.Start()
	dm.S.Expect("READY")

	utiltest.Resolve(t, ctx, "factoryDM", 1, true) // Current link should have been launching factory version.
	utiltest.ShutdownDevice(t, ctx, "factoryDM")
	dm.S.Expect("factoryDM terminated")
	dm.S.ExpectEOF()

	// Re-launch the device manager, to exercise the behavior of Stop.
	utiltest.ResolveExpectNotFound(t, ctx, "factoryDM", false) // Ensure a clean slate.
	dm = sh.FuncCmd(utiltest.ExecScript, currLink)
	dm.Vars = envvar.MergeMaps(dm.Vars, dmVars)
	dm.Start()
	dm.S.Expect("READY")

	utiltest.Resolve(t, ctx, "factoryDM", 1, true)
	utiltest.KillDevice(t, ctx, "factoryDM")
	dm.S.Expect("restart handler")
	dm.S.Expect("factoryDM terminated")
	dm.S.ExpectEOF()
}

type simpleRW chan []byte

func (s simpleRW) Write(p []byte) (n int, err error) {
	s <- p
	return len(p), nil
}
func (s simpleRW) Read(p []byte) (n int, err error) {
	return copy(p, <-s), nil
}

// TestDeviceManagerInstallation verifies the 'self install' and 'uninstall'
// functionality of the device manager: it runs SelfInstall in a child process,
// then runs the executable from the soft link that the installation created.
// This should bring up a functioning device manager.  In the end it runs
// Uninstall and verifies that the installation is gone.
func TestDeviceManagerInstallation(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	sh, deferFn := servicetest.CreateShellAndMountTable(t, ctx)
	defer deferFn()
	testDir, cleanup := servicetest.SetupRootDir(t, "devicemanager")
	defer cleanup()
	// No need to call SaveCreatorInfo() here because that's part of SelfInstall below

	// Create a script wrapping the test target that implements suidhelper.
	suidHelperPath := utiltest.GenerateSuidHelperScript(t, testDir)
	// Create a dummy script mascarading as the restarter.
	restarterPath := utiltest.GenerateRestarter(t, testDir)
	initHelperPath := ""

	// Create an 'envelope' for the device manager that we can pass to the
	// installer, to ensure that the device manager that the installer
	// configures can run.
	dmCmd := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm")
	dmDir := filepath.Join(testDir, "dm")
	// TODO(caprita): Add test logic when initMode = true.
	singleUser, sessionMode, initMode := true, true, false
	if err := installer.SelfInstall(ctx, dmDir, suidHelperPath, restarterPath, "", initHelperPath, "", singleUser, sessionMode, initMode, dmCmd.Args[1:], envvar.MapToSlice(dmCmd.Vars), os.Stderr, os.Stdout); err != nil {
		t.Fatalf("SelfInstall failed: %v", err)
	}

	utiltest.ResolveExpectNotFound(t, ctx, "dm", false)
	// Start the device manager.
	stdout := make(simpleRW, 100)
	defer os.Setenv(utiltest.RedirectEnv, os.Getenv(utiltest.RedirectEnv))
	os.Setenv(utiltest.RedirectEnv, "1")
	if err := installer.Start(ctx, dmDir, os.Stderr, stdout); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	dms := expect.NewSession(t, stdout, servicetest.ExpectTimeout)
	dms.Expect("READY")
	utiltest.ClaimDevice(t, ctx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)
	utiltest.RevertDeviceExpectError(t, ctx, "dm", errors.ErrUpdateNoOp.ID) // No previous version available.

	// Stop the device manager.
	if err := installer.Stop(ctx, dmDir, os.Stderr, os.Stdout); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	dms.Expect("dm terminated")

	// Uninstall.
	if err := installer.Uninstall(ctx, dmDir, suidHelperPath, os.Stderr, os.Stdout); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	// Ensure that the installation is gone.
	if files, err := ioutil.ReadDir(dmDir); err != nil || len(files) > 0 {
		var finfo []string
		for _, f := range files {
			finfo = append(finfo, f.Name())
		}
		t.Fatalf("ReadDir returned (%v, %v)", err, finfo)
	}
}

// TODO(caprita): We need better test coverage for how updating/reverting apps
// affects the package configured for the app.
func TestDeviceManagerPackages(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	sh, deferFn := servicetest.CreateShellAndMountTable(t, ctx)
	defer deferFn()

	// Set up mock application and binary repositories.
	envelope, cleanup := utiltest.StartMockRepos(t, ctx)
	defer cleanup()

	binaryVON := "realbin"
	defer utiltest.StartRealBinaryRepository(t, ctx, binaryVON)()

	// upload package to binary repository
	tmpdir, err := ioutil.TempDir("", "test-package-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	createFile := func(name, contents string) {
		if err := ioutil.WriteFile(filepath.Join(tmpdir, name), []byte(contents), 0600); err != nil {
			t.Fatalf("ioutil.WriteFile failed: %v", err)
		}
	}
	createFile("hello.txt", "Hello World!")
	if _, err := binarylib.UploadFromDir(ctx, naming.Join(binaryVON, "testpkg"), tmpdir); err != nil {
		t.Fatalf("binarylib.UploadFromDir failed: %v", err)
	}
	createAndUpload := func(von, contents string) {
		createFile("tempfile", contents)
		if _, err := binarylib.UploadFromFile(ctx, naming.Join(binaryVON, von), filepath.Join(tmpdir, "tempfile")); err != nil {
			t.Fatalf("binarylib.UploadFromFile failed: %v", err)
		}
	}
	createAndUpload("testfile", "Goodbye World!")
	createAndUpload("leftshark", "Left shark")
	createAndUpload("rightshark", "Right shark")
	createAndUpload("beachball", "Beach ball")

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
	defer func() {
		dm.Terminate(os.Interrupt)
		dm.S.Expect("dm terminated")
		utiltest.VerifyNoRunningProcesses(t)
	}()

	// Create the local server that the app uses to let us know it's ready.
	pingCh, cleanup := utiltest.SetupPingServer(t, ctx)
	defer cleanup()

	// Create the envelope for the first version of the app.
	*envelope = utiltest.EnvelopeFromShell(sh, nil, nil, utiltest.App, "google naps", 0, 0, "appV1")
	envelope.Packages = map[string]application.SignedFile{
		"test": application.SignedFile{
			File: "realbin/testpkg",
		},
		"test2": application.SignedFile{
			File: "realbin/testfile",
		},
		"shark": application.SignedFile{
			File: "realbin/leftshark",
		},
	}

	// These are install-time overrides for packages.
	// Specifically, we override the 'shark' package and add a new
	// 'ball' package on top of what's specified in the envelope.
	packages := application.Packages{
		"shark": application.SignedFile{
			File: "realbin/rightshark",
		},
		"ball": application.SignedFile{
			File: "realbin/beachball",
		},
	}
	// Device must be claimed before apps can be installed.
	utiltest.ClaimDevice(t, ctx, "claimable", "dm", "mydevice", utiltest.NoPairingToken)
	// Install the app.
	appID := utiltest.InstallApp(t, ctx, packages)

	// Start an instance of the app.
	instance1ID := utiltest.LaunchApp(t, ctx, appID)
	defer utiltest.TerminateApp(t, ctx, appID, instance1ID)

	// Wait until the app pings us that it's ready.
	pingCh.WaitForPingArgs(t)

	for _, c := range []struct {
		path, content string
	}{
		{
			filepath.Join("test", "hello.txt"),
			"Hello World!",
		},
		{
			"test2",
			"Goodbye World!",
		},
		{
			"shark",
			"Right shark",
		},
		{
			"ball",
			"Beach ball",
		},
	} {
		// Ask the app to cat the file.
		file := filepath.Join("packages", c.path)
		name := "appV1"
		content, err := utiltest.Cat(ctx, name, file)
		if err != nil {
			t.Errorf("utiltest.Cat(%q, %q) failed: %v", name, file, err)
		}
		if expected := c.content; content != expected {
			t.Errorf("unexpected content: expected %q, got %q", expected, content)
		}
	}
}

func listAndVerifyAssociations(t *testing.T, ctx *context.T, stub device.DeviceClientMethods, expected []device.Association) {
	assocs, err := stub.ListAssociations(ctx)
	if err != nil {
		t.Fatalf("ListAssociations failed %v", err)
	}
	utiltest.CompareAssociations(t, assocs, expected)
}

// TODO(rjkroege): Verify that associations persist across restarts once
// permanent storage is added.
func TestAccountAssociation(t *testing.T) {
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

	root, cleanup := servicetest.SetupRootDir(t, "devicemanager")
	defer cleanup()
	if err := versioning.SaveCreatorInfo(ctx, root); err != nil {
		t.Fatal(err)
	}

	// By default, the two processes (selfCtx and otherCtx) will have blessings
	// generated based on the username/machine name running this process. Since
	// these blessings will appear in AccessLists, give them recognizable names.
	selfCtx := utiltest.CtxWithNewPrincipal(t, ctx, idp, "self")
	otherCtx := utiltest.CtxWithNewPrincipal(t, selfCtx, idp, "other")

	dm := utiltest.DeviceManagerCmd(sh, utiltest.DeviceManager, "dm", root, "unused_helper", "unused_app_repo_name", "unused_curr_link")
	dm.Start()
	dm.S.Expect("READY")
	defer func() {
		dm.Terminate(os.Interrupt)
		dm.S.Expect("dm terminated")
		utiltest.VerifyNoRunningProcesses(t)
	}()

	// Attempt to list associations on the device manager without having
	// claimed it.
	if list, err := device.DeviceClient("claimable").ListAssociations(otherCtx); err == nil {
		t.Fatalf("ListAssociations should fail on unclaimed device manager but did not: (%v, %v)", list, err)
	}

	// self claims the device manager.
	utiltest.ClaimDevice(t, selfCtx, "claimable", "dm", "alice", utiltest.NoPairingToken)

	ctx.VI(2).Info("Verify that associations start out empty.")
	deviceStub := device.DeviceClient("dm/device")
	listAndVerifyAssociations(t, selfCtx, deviceStub, []device.Association(nil))

	if err := deviceStub.AssociateAccount(selfCtx, []string{"root/self", "root/other"}, "alice_system_account"); err != nil {
		t.Fatalf("ListAssociations failed %v", err)
	}
	ctx.VI(2).Info("Added association should appear.")
	listAndVerifyAssociations(t, selfCtx, deviceStub, []device.Association{
		{
			"root/self",
			"alice_system_account",
		},
		{
			"root/other",
			"alice_system_account",
		},
	})

	if err := deviceStub.AssociateAccount(selfCtx, []string{"root/self", "root/other"}, "alice_other_account"); err != nil {
		t.Fatalf("AssociateAccount failed %v", err)
	}
	ctx.VI(2).Info("Change the associations and the change should appear.")
	listAndVerifyAssociations(t, selfCtx, deviceStub, []device.Association{
		{
			"root/self",
			"alice_other_account",
		},
		{
			"root/other",
			"alice_other_account",
		},
	})

	if err := deviceStub.AssociateAccount(selfCtx, []string{"root/other"}, ""); err != nil {
		t.Fatalf("AssociateAccount failed %v", err)
	}
	ctx.VI(2).Info("Verify that we can remove an association.")
	listAndVerifyAssociations(t, selfCtx, deviceStub, []device.Association{
		{
			"root/self",
			"alice_other_account",
		},
	})
}
