// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test the device manager and related services and tools.
//
// By default, this script tests the device manager in a fashion amenable
// to automatic testing: the --single_user is passed to the device
// manager so that all device manager components run as the same user and
// no user input (such as an agent pass phrase) is needed.
//
// This script can exercise the device manager in two different modes. It
// can be executed like so:
//
//   jiri go test -v . --v23.tests
//
// This will exercise the device manager's single user mode where all
// processes run as the same invoking user.
//
// Alternatively, the device manager can be executed in multiple account
// mode by providing the --deviceuser <deviceuser> and --appuser
// <appuser> flags. In this case, the device manager will run as user
// <devicemgr> and the test will run applications as user <appuser>. If
// executed in this fashion, root permissions will be required to install
// and it may require configuring an agent passphrase. For example:
//
//   jiri go test -v . --v23.tests --deviceuser devicemanager --appuser  vana
//
// NB: the accounts provided as arguments to this test must already exist.
// Also, the --v23.tests.shell-on-fail flag is useful to enable debugging
// output. Note that this flag does not work for some shells. Set
// $SHELL in that case.

package device_test

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"v.io/x/ref"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/internal/dirprinter"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

var (
	appUserFlag    string
	deviceUserFlag string
	hostname       string
	errTimeout     = errors.New("timeout")
)

func init() {
	flag.StringVar(&appUserFlag, "appuser", "", "launch apps as the specified user")
	flag.StringVar(&deviceUserFlag, "deviceuser", "", "run the device manager as the specified user")
	name, err := os.Hostname()
	if err != nil {
		panic(fmt.Sprintf("Hostname() failed: %v", err))
	}
	hostname = name
}

func TestV23DeviceManagerSingleUser(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	u, err := user.Current()
	if err != nil {
		t.Fatalf("couldn't get the current user: %v", err)
	}
	testCore(t, sh, u.Username, "", false)
}

func TestV23DeviceManagerMultiUser(t *testing.T) {
	t.Skip("Permissions need to be configured properly on credentials.")
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	u, err := user.Current()
	if err != nil {
		t.Fatalf("couldn't get the current user: %v", err)
	}

	if u.Username == "veyron" && runTestOnThisPlatform {
		// We are running on the builder so run the multiuser
		// test with default user names. These will be created as
		// required.
		makeTestAccounts(t, sh)
		testCore(t, sh, "vana", "devicemanager", true)
		return
	}

	if len(deviceUserFlag) > 0 && len(appUserFlag) > 0 {
		testCore(t, sh, appUserFlag, deviceUserFlag, true)
	} else {
		t.Logf("Test skipped because running in multiuser mode requires --appuser and --deviceuser flags")
	}
}

func testCore(t *testing.T, sh *v23test.Shell, appUser, deviceUser string, withSuid bool) {
	defer fmt.Fprintf(os.Stderr, "--------------- SHUTDOWN ---------------\n")

	// Call sh.StartRootMountTable() first, since it updates sh.Vars, which is
	// copied by various Cmds at Cmd creation time.
	sh.StartRootMountTable()

	// When running with --with_suid, TMPDIR must grant the invoking user rwx
	// permissions and world x permissions for all parent directories back to /.
	// Otherwise, the with_suid user will not be able to use absolute paths. On
	// Darwin, TMPDIR defaults to a directory hierararchy in /var that is 0700.
	// This is unworkable, so force TMPDIR to /tmp in this case.
	//
	// In addition, even when running without --with_suid, on Darwin the default
	// TMPDIR results in "socket path (...) exceeds maximum allowed socket path
	// length" errors, so we always set TMPDIR to /tmp.
	oldTmpdir := os.Getenv("TMPDIR")
	os.Setenv("TMPDIR", "/tmp")
	defer os.Setenv("TMPDIR", oldTmpdir)

	var (
		workDir       = sh.MakeTempDir()
		binStagingDir = mkSubdir(t, workDir, "bin")
		dmInstallDir  = filepath.Join(workDir, "dm")

		// Most vanadium command-line utilities will be run by a
		// principal that has "root:u:alice" as its blessing.
		// (Where "root" comes from i.Principal().BlessingStore().Default()).
		// Create those credentials and options to use to setup the
		// binaries with them.
		aliceCreds = sh.ForkCredentials("u:alice")

		// Build all the command-line tools and set them up to run as alice.
		// applicationd/binaryd servers will be run by alice too.
		// TODO: applicationd/binaryd should run as a separate "service" role, as
		// alice is just a user.
		namespaceBin    = sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/namespace")).WithCredentials(aliceCreds)
		deviceBin       = sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/device/device")).WithCredentials(aliceCreds)
		binarydBin      = sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/binary/binaryd")).WithCredentials(aliceCreds)
		applicationdBin = sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/application/applicationd")).WithCredentials(aliceCreds)

		// The devicex script is not provided with any credentials, it
		// will generate its own.  This means that on "devicex start"
		// the device will have no useful credentials and until "device
		// claim" is invoked (as alice), it will just sit around
		// waiting to be claimed.
		//
		// Other binaries, like applicationd and binaryd will be run by alice.
		deviceScript = sh.Cmd("./devicex")

		mtName = "devices/" + hostname // Name under which the device manager will publish itself.
	)

	defer func() {
		if !t.Failed() {
			return
		}
		fmt.Fprintf(os.Stderr, "--------------- START DUMP %s ---------------\n", workDir)
		if err := dirprinter.DumpDir(os.Stderr, workDir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed: %v\n", err)
		}
		fmt.Fprintf(os.Stderr, "--------------- END DUMP %s ---------------\n", workDir)
	}()
	deviceScript.Vars["V23_DEVICE_DIR"] = dmInstallDir
	// Make sure the devicex command is not provided with credentials. Note, this
	// is analogous to what's done in
	// v.io/x/ref/services/device/deviced/internal/impl/utiltest.RunDeviceManager.
	delete(deviceScript.Vars, ref.EnvCredentials)
	delete(deviceScript.Vars, ref.EnvAgentPath)

	// We also need some tools running with different sets of credentials...

	// Administration tasks will be performed with a blessing that represents a corporate
	// adminstrator (which is usually a role account)
	adminCreds := sh.ForkCredentials("r:admin")
	adminDeviceBin := deviceBin.WithCredentials(adminCreds)
	debugBin := sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")).WithCredentials(adminCreds)

	// A special set of credentials will be used to give two blessings to the device manager
	// when claiming it -- one blessing will be from the corporate administrator role who owns
	// the machine, and the other will be a manufacturer blessing. (This is a hack until
	// there's a way to separately supply a manufacturer blessing. Eventually, the claim
	// would really be done by the administator, and the adminstrator's blessing would get
	// added to the manufacturer's blessing, which would already be present.)
	claimCreds := sh.ForkCredentials("r:admin", "m:orange:zphone5:ime-i007")
	claimDeviceBin := deviceBin.WithCredentials(claimCreds)

	// Another set of credentials be used to represent the application publisher, who
	// signs and pushes binaries
	pubCreds := sh.ForkCredentials("a:rovio")
	pubDeviceBin := deviceBin.WithCredentials(pubCreds)
	applicationBin := sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/application/application")).WithCredentials(pubCreds)
	binaryBin := sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/binary/binary")).WithCredentials(pubCreds)

	if withSuid {
		// In multiuser mode, deviceUserFlag needs execute access to
		// tempDir.
		if err := os.Chmod(workDir, 0711); err != nil {
			t.Fatalf("os.Chmod() failed: %v", err)
		}
	}

	buildAndCopyBinaries(
		t, sh, binStagingDir,
		"v.io/x/ref/services/device/deviced",
		"v.io/x/ref/services/device/restarter",
		"v.io/x/ref/services/agent/v23agentd",
		"v.io/x/ref/services/device/suidhelper",
		"v.io/x/ref/services/device/inithelper")

	appDName := "applications"
	devicedAppName := filepath.Join(appDName, "deviced", "test")

	deviceScriptArguments := []string{
		"install",
		binStagingDir,
	}

	if withSuid {
		deviceScriptArguments = append(deviceScriptArguments, "--devuser="+deviceUser)
	} else {
		deviceScriptArguments = append(deviceScriptArguments, "--single_user")
	}

	deviceScriptArguments = append(deviceScriptArguments, []string{
		"--origin=" + devicedAppName,
		"--",
		"--v23.tcp.address=127.0.0.1:0",
		"--neighborhood-name=" + fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), rand.Int()),
	}...)

	withArgs(deviceScript, deviceScriptArguments...).Run()
	withArgs(deviceScript, "start").Run()
	dmLog := filepath.Join(dmInstallDir, "dmroot/device-manager/logs/deviced.INFO")
	stopDevMgr := func() {
		withArgs(deviceScript, "stop").Run()
		if dmLogF, err := os.Open(dmLog); err != nil {
			t.Errorf("Failed to read dm log: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "--------------- START DM LOG ---------------\n")
			defer dmLogF.Close()
			if _, err := io.Copy(os.Stderr, dmLogF); err != nil {
				t.Errorf("Error dumping dm log: %v", err)
			}
			fmt.Fprintf(os.Stderr, "--------------- END DM LOG ---------------\n")
		}
	}
	var stopDevMgrOnce sync.Once
	defer stopDevMgrOnce.Do(stopDevMgr)
	// Grab the endpoint for the claimable service from the device manager's
	// log.
	var claimableEP string
	expiry := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(expiry) {
			t.Fatalf("Timed out looking for claimable endpoint in %v", dmLog)
		}
		startLog, err := ioutil.ReadFile(dmLog)
		if err != nil {
			t.Logf("Couldn't read log %v: %v", dmLog, err)
			time.Sleep(time.Second)
			continue
		}
		re := regexp.MustCompile(`Unclaimed device manager endpoint: (.*)`)
		matches := re.FindSubmatch(startLog)
		if len(matches) == 0 {
			t.Logf("Couldn't find match in %v [%s]", dmLog, startLog)
			time.Sleep(time.Second)
			continue
		}
		if len(matches) < 2 {
			t.Fatalf("Wrong match in %v (%d) %v", dmLog, len(matches), string(matches[0]))
		}
		claimableEP = string(matches[len(matches)-1])
		break
	}
	// Claim the device as "root:u:alice:myworkstation".
	claimCmd := withArgs(claimDeviceBin, "claim", claimableEP, "myworkstation")
	// TODO(caprita): Sometimes, the claim RPC fails to complete, even
	// though the server-side claiming seems to succeed.  See
	// https://github.com/vanadium/build/issues/58
	claimCmd.ExitErrorIsOk = true
	claimCmd.Run()

	resolve := func(name string) string {
		res := ""
		if err := testutil.RetryFor(10*time.Second, func() error {
			// Set ExitErrorIsOk to true since we expect "namespace resolve" to fail
			// if the name doesn't exist.
			c := withArgs(namespaceBin, "resolve", "-s", name)
			c.ExitErrorIsOk = true
			c.AddStderrWriter(os.Stderr)
			if res = tr(c.Stdout()); len(res) > 0 {
				return nil
			}
			return testutil.TryAgain(errors.New("resolve returned nothing"))
		}); err != nil {
			t.Fatal(err)
		}
		return res
	}

	// Wait for the device manager to publish its mount table entry.
	mtEP := resolve(mtName)
	withArgs(adminDeviceBin, "acl", "set", mtName+"/devmgr/device", "root:u:alice", "Read,Resolve,Write").Run()

	if withSuid {
		withArgs(adminDeviceBin, "associate", "add", mtName+"/devmgr/device", appUser, "root:u:alice").Run()
		associations := withArgs(adminDeviceBin, "associate", "list", mtName+"/devmgr/device").Stdout()
		if got, expected := strings.Trim(associations, "\n "), "root:u:alice "+appUser; got != expected {
			t.Fatalf("association test, got %v, expected %v", got, expected)
		}
	}

	// Verify the device's default blessing is as expected.
	mfrBlessing := "root:m:orange:zphone5:ime-i007:myworkstation"
	ownerBlessing := "root:r:admin:myworkstation"
	c := withArgs(debugBin, "stats", "read", mtName+"/devmgr/__debug/stats/security/principal/*/blessingstore/*")
	c.Run()
	c.S.ExpectSetEventuallyRE(".*Default Blessings[ ]+" + mfrBlessing + "," + ownerBlessing)

	// Get the device's profile, which should be set to non-empty string
	c = withArgs(adminDeviceBin, "describe", mtName+"/devmgr/device")
	c.Run()
	parts := c.S.ExpectRE(`{Profiles:map\[(.*):{}\]}`, 1)
	expectOneMatch := func(parts [][]string) string {
		if len(parts) != 1 || len(parts[0]) != 2 {
			t.Fatalf("%s: failed to match profile: %#v", caller(1), parts)
		}
		return parts[0][1]
	}
	deviceProfile := expectOneMatch(parts)
	if len(deviceProfile) == 0 {
		t.Fatalf("failed to get profile")
	}

	// Start a binaryd server that will serve the binary for the test
	// application to be installed on the device.
	binarydName := "binaries"
	withArgs(binarydBin,
		"--name="+binarydName,
		"--root-dir="+filepath.Join(workDir, "binstore"),
		"--v23.tcp.address=127.0.0.1:0",
		"--http=127.0.0.1:0").Start()
	// Allow publishers to update binaries
	withArgs(deviceBin, "acl", "set", binarydName, "root:a", "Write").Run()

	// We are also going to use the binaryd binary as our test app binary. Once our test app
	// binary is published to the binaryd server started above, this (augmented with a
	// timestamp) is the name the test app binary will have.
	sampleAppBinName := binarydName + "/binaryd"

	// Start an applicationd server that will serve the application
	// envelope for the test application to be installed on the device.
	withArgs(applicationdBin,
		"--name="+appDName,
		"--store="+mkSubdir(t, workDir, "appstore"),
		"--v23.tcp.address=127.0.0.1:0").Start()
	// Allow publishers to create and update envelopes
	withArgs(deviceBin, "acl", "set", appDName, "root:a", "Read,Write,Resolve").Run()

	sampleAppName := appDName + "/testapp"
	appPubName := "testbinaryd"
	appEnvelopeFilename := filepath.Join(workDir, "app.envelope")
	appEnvelope := fmt.Sprintf("{\"Title\":\"BINARYD\", \"Args\":[\"--name=%s\", \"--root-dir=./binstore\", \"--v23.tcp.address=127.0.0.1:0\", \"--http=127.0.0.1:0\"], \"Binary\":{\"File\":%q}, \"Env\":[ \"%s=1\", \"PATH=%s\"]}", appPubName, sampleAppBinName, ref.EnvCredentialsNoAgent, os.Getenv("PATH"))
	ioutil.WriteFile(appEnvelopeFilename, []byte(appEnvelope), 0666)
	defer os.Remove(appEnvelopeFilename)

	output := withArgs(applicationBin, "put", sampleAppName+"/0", deviceProfile, appEnvelopeFilename).Stdout()
	if got, want := tr(output), fmt.Sprintf("Application envelope added for profile %s.", deviceProfile); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Verify that the envelope we uploaded shows up with glob.
	c = withArgs(applicationBin, "match", sampleAppName, deviceProfile)
	c.Run()
	parts = c.S.ExpectSetEventuallyRE(`"Title": "(.*)",`, `"File": "(.*)",`)
	if got, want := len(parts), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	for line, want := range []string{"BINARYD", sampleAppBinName} {
		if got := parts[line][1]; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	// Publish the app (This uses the binarydBin binary and the testapp envelope from above)
	withArgs(pubDeviceBin, "publish", "-from", filepath.Dir(binarydBin.Path), "-readers", "root:r:admin", filepath.Base(binarydBin.Path)+":testapp").Run()
	if got := withArgs(namespaceBin, "glob", sampleAppBinName).Stdout(); len(got) == 0 {
		t.Fatalf("glob failed for %q", sampleAppBinName)
	}

	// Install the app on the device.
	c = withArgs(deviceBin, "install", mtName+"/devmgr/apps", sampleAppName)
	c.Run()
	installationName := c.S.ReadLine()
	if installationName == "" {
		t.Fatalf("got empty installation name from install")
	}

	// Verify that the installation shows up when globbing the device manager.
	output = withArgs(namespaceBin, "glob", mtName+"/devmgr/apps/BINARYD/*").Stdout()
	if got, want := tr(output), installationName; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Start an instance of the app, granting it blessing extension myapp.
	c = withArgs(deviceBin, "instantiate", installationName, "myapp")
	c.Run()
	instanceName := c.S.ReadLine()
	if instanceName == "" {
		t.Fatalf("got empty instance name from new")
	}
	withArgs(deviceBin, "run", instanceName).Run()

	resolve(mtName + "/" + appPubName)

	// Verify that the instance shows up when globbing the device manager.
	output = withArgs(namespaceBin, "glob", mtName+"/devmgr/apps/BINARYD/*/*").Stdout()
	if got, want := tr(output), instanceName; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	c = withArgs(debugBin, "stats", "read", instanceName+"/stats/system/pid")
	c.Run()
	pid := c.S.ExpectRE("[0-9]+$", 1)[0][0]
	uname, err := getUserForPid(sh, pid)
	if err != nil {
		t.Errorf("getUserForPid could not determine the user running pid %v", pid)
	} else if uname != appUser {
		t.Errorf("app expected to be running as %v but is running as %v", appUser, uname)
	}

	// Verify the app's blessings. We check the default blessing, as well as the
	// "..." blessing, which should be the default blessing plus a publisher blessing.
	userBlessing := "root:u:alice:myapp"
	pubBlessing := "root:a:rovio:apps:published:binaryd"
	appBlessing := mfrBlessing + ":a:" + pubBlessing + "," + ownerBlessing + ":a:" + pubBlessing
	c = withArgs(debugBin, "stats", "read", instanceName+"/stats/security/principal/*/blessingstore/*")
	c.Run()
	c.S.ExpectSetEventuallyRE(".*Default Blessings[ ]+"+userBlessing+"$", "[.][.][.][ ]+"+userBlessing+","+appBlessing)

	// Kill and delete the instance.
	withArgs(deviceBin, "kill", instanceName).Run()
	withArgs(deviceBin, "delete", instanceName).Run()

	// Verify that logs, but not stats, show up when globbing the
	// not-running instance.
	if output = withArgs(namespaceBin, "glob", instanceName+"/stats/...").Stdout(); len(output) > 0 {
		t.Fatalf("no output expected for glob %s/stats/..., got %q", output, instanceName)
	}
	if output = withArgs(namespaceBin, "glob", instanceName+"/logs/...").Stdout(); len(output) == 0 {
		t.Fatalf("output expected for glob %s/logs/..., but got none", instanceName)
	}

	// TODO: The deviced binary should probably be published by someone other than rovio :-)
	// Maybe publishing the deviced binary should eventually use "device publish" too?
	// For now, it uses the "application" and "binary" tools directly to ensure that those work

	// Upload a deviced binary
	devicedAppBinName := binarydName + "/deviced"
	withArgs(binaryBin, "upload", devicedAppBinName, v23test.BuildGoPkg(sh, "v.io/x/ref/services/device/deviced")).Run()
	// Allow root:r:admin and its devices to read the binary
	withArgs(deviceBin, "acl", "set", devicedAppBinName, "root:r:admin", "Read").Run()

	// Upload a device manager envelope.
	devicedEnvelopeFilename := filepath.Join(workDir, "deviced.envelope")
	devicedEnvelope := fmt.Sprintf("{\"Title\":\"device manager\", \"Binary\":{\"File\":%q}, \"Env\":[ \"%s=1\", \"PATH=%s\"]}", devicedAppBinName, ref.EnvCredentialsNoAgent, os.Getenv("PATH"))
	ioutil.WriteFile(devicedEnvelopeFilename, []byte(devicedEnvelope), 0666)
	defer os.Remove(devicedEnvelopeFilename)
	withArgs(applicationBin, "put", devicedAppName, deviceProfile, devicedEnvelopeFilename).Run()
	// Allow root:r:admin and its devices to read the envelope
	withArgs(deviceBin, "acl", "set", devicedAppName, "root:r:admin", "Read").Run()

	// Update the device manager.
	withArgs(adminDeviceBin, "update", mtName+"/devmgr/device").Run()
	resolveChange := func(name, old string) string {
		res := ""
		if err := testutil.RetryFor(10*time.Second, func() error {
			// Set ExitErrorIsOk to true since we expect "namespace resolve" to fail
			// if the name doesn't exist.
			c := withArgs(namespaceBin, "resolve", "-s", name)
			c.ExitErrorIsOk = true
			c.AddStderrWriter(os.Stderr)
			switch res = tr(c.Stdout()); {
			case res == "":
				return testutil.TryAgain(errors.New("resolve returned nothing"))
			case res == old:
				return testutil.TryAgain(errors.New("no change"))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return res
	}
	mtEP = resolveChange(mtName, mtEP)

	// Verify that device manager's mounttable is still published under the
	// expected name (hostname).
	if withArgs(namespaceBin, "glob", mtName).Stdout() == "" {
		t.Fatalf("failed to glob %s", mtName)
	}

	// Revert the device manager
	// The argument to "device revert" is a glob pattern. So we need to
	// wait for devmgr to be mounted before running the command.
	resolve(mtEP + "/devmgr")
	withArgs(adminDeviceBin, "revert", mtName+"/devmgr/device").Run()
	mtEP = resolveChange(mtName, mtEP)

	// Verify that device manager's mounttable is still published under the
	// expected name (hostname).
	if withArgs(namespaceBin, "glob", mtName).Stdout() == "" {
		t.Fatalf("failed to glob %s", mtName)
	}

	// Verify that the local mounttable exists, and that the device manager,
	// the global namespace, and the neighborhood are mounted on it.
	resolve(mtEP + "/devmgr")
	resolve(mtEP + "/nh")
	resolve(mtEP + "/global")

	namespaceRoot := sh.Vars[ref.EnvNamespacePrefix]
	output = withArgs(namespaceBin, "resolve", "-s", mtEP+"/global").Stdout()
	if got, want := tr(output), namespaceRoot; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Kill the device manager (which causes it to be restarted), wait for
	// the endpoint to change.
	withArgs(deviceBin, "kill", mtName+"/devmgr/device").Run()
	mtEP = resolveChange(mtName, mtEP)

	// Shut down the device manager.
	stopDevMgrOnce.Do(stopDevMgr)

	// Wait for the mounttable entry to go away.
	resolveGone := func(name string) string {
		res := ""
		if err := testutil.RetryFor(10*time.Second, func() error {
			// Set ExitErrorIsOk to true since we expect "namespace resolve" to fail
			// if the name doesn't exist.
			c := withArgs(namespaceBin, "resolve", "-s", name)
			c.ExitErrorIsOk = true
			c.AddStderrWriter(os.Stderr)
			if res = tr(c.Stdout()); len(res) == 0 {
				return nil
			}
			return testutil.TryAgain(errors.New("mount table entry still exists"))
		}); err != nil {
			t.Fatal(err)
		}
		return res
	}
	resolveGone(mtName)

	var fi []os.FileInfo

	// This doesn't work in multiuser mode because dmInstallDir is
	// owned by the device manager user and unreadable by the user
	// running this test.
	if !withSuid {
		fi, err = ioutil.ReadDir(dmInstallDir)
		if err != nil {
			t.Fatalf("failed to readdir for %q: %v", dmInstallDir, err)
		}
	}

	withArgs(deviceScript, "uninstall").Run()

	fi, err = ioutil.ReadDir(dmInstallDir)
	if err == nil || len(fi) > 0 {
		t.Fatalf("managed to read %d entries from %q", len(fi), dmInstallDir)
	}
	if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("wrong error: %v", err)
	}
}

func withArgs(cmd *v23test.Cmd, args ...string) *v23test.Cmd {
	res := cmd.Clone()
	res.Args = append(res.Args, args...)
	return res
}

func buildAndCopyBinaries(t *testing.T, sh *v23test.Shell, destinationDir string, packages ...string) {
	var args []string
	for _, pkg := range packages {
		args = append(args, v23test.BuildGoPkg(sh, pkg))
	}
	args = append(args, destinationDir)
	sh.Cmd("/bin/cp", args...).Run()
}

func mkSubdir(t *testing.T, parent, child string) string {
	dir := filepath.Join(parent, child)
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("failed to create %q: %v", dir, err)
	}
	return dir
}

var re = regexp.MustCompile("[ \t]+")

// getUserForPid determines the username running process pid.
func getUserForPid(sh *v23test.Shell, pid string) (string, error) {
	pidString := sh.Cmd("/bin/ps", psFlags).Stdout()
	for _, line := range strings.Split(pidString, "\n") {
		fields := re.Split(line, -1)
		if len(fields) > 1 && pid == fields[1] {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("Couldn't determine the user for pid %s", pid)
}

// tr trims off trailing newline characters.
func tr(s string) string {
	return strings.TrimRight(s, "\n")
}

// caller returns a string of the form <filename>:<lineno>.
func caller(skip int) string {
	_, file, line, _ := runtime.Caller(skip + 1)
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
