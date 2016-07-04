// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package globsuid_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"v.io/v23"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/application"
	"v.io/v23/services/device"
	"v.io/v23/services/repository"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/internal/binarylib"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func TestDownloadSignatureMatch(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	rg := testutil.NewRandGenerator(t.Logf)

	sh, deferFn := servicetest.CreateShellAndMountTable(t, ctx)
	defer deferFn()

	binaryVON := "binary"
	pkgVON := naming.Join(binaryVON, "testpkg")
	defer utiltest.StartRealBinaryRepository(t, ctx, binaryVON)()

	up := rg.RandomBytes(rg.RandomIntn(5 << 20))
	mediaInfo := repository.MediaInfo{Type: "application/octet-stream"}
	sig, err := binarylib.Upload(ctx, naming.Join(binaryVON, "testbinary"), up, mediaInfo)
	if err != nil {
		t.Fatalf("Upload(%v) failed:%v", binaryVON, err)
	}

	// Upload packages for this application
	tmpdir, err := ioutil.TempDir("", "test-package-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	pkgContents := rg.RandomBytes(rg.RandomIntn(5 << 20))
	if err := ioutil.WriteFile(filepath.Join(tmpdir, "pkg.txt"), pkgContents, 0600); err != nil {
		t.Fatalf("ioutil.WriteFile failed: %v", err)
	}
	pkgSig, err := binarylib.UploadFromDir(ctx, pkgVON, tmpdir)
	if err != nil {
		t.Fatalf("binarylib.UploadFromDir failed: %v", err)
	}

	// Start the application repository
	envelope, serverStop := utiltest.StartApplicationRepository(ctx)
	defer serverStop()

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

	p := v23.GetPrincipal(ctx)
	publisher, err := p.BlessSelf("publisher")
	if err != nil {
		t.Fatalf("Failed to generate publisher blessings:%v", err)
	}
	*envelope = application.Envelope{
		Binary: application.SignedFile{
			File:      naming.Join(binaryVON, "testbinary"),
			Signature: *sig,
		},
		Publisher: publisher,
		Packages: map[string]application.SignedFile{
			"pkg": application.SignedFile{
				File:      pkgVON,
				Signature: *pkgSig,
			},
		},
	}

	// Using the publisher should fail, because blessing "publisher" is not covered by the
	// trusted roots of the device manager's principal
	if _, err := utiltest.AppStub().Install(ctx, utiltest.MockApplicationRepoName, device.Config{}, nil); verror.ErrorID(err) != errors.ErrOperationFailed.ID {
		t.Fatalf("Unexpected error installing app:%v (expected ErrOperationFailed)", err)
	}

	// Changing the publisher blessing to one that is covered by the DM roots, should
	// allow the app installation to succeed.
	b, _ := p.BlessingStore().Default()
	envelope.Publisher, err = p.Bless(p.PublicKey(), b, "publisher", security.UnconstrainedUse())
	if err != nil {
		t.Fatalf("Failed to generate trusted publisher blessings: %v", err)
	}

	if _, err := utiltest.AppStub().Install(ctx, utiltest.MockApplicationRepoName, device.Config{}, nil); err != nil {
		t.Fatalf("Failed to Install app:%v", err)
	}

	// Verify that when the binary is corrupted, signature verification fails.
	up[0] = up[0] ^ 0xFF
	if err := binarylib.Delete(ctx, naming.Join(binaryVON, "testbinary")); err != nil {
		t.Fatalf("Delete(%v) failed:%v", binaryVON, err)
	}
	if _, err := binarylib.Upload(ctx, naming.Join(binaryVON, "testbinary"), up, mediaInfo); err != nil {
		t.Fatalf("Upload(%v) failed:%v", binaryVON, err)
	}
	if _, err := utiltest.AppStub().Install(ctx, utiltest.MockApplicationRepoName, device.Config{}, nil); verror.ErrorID(err) != errors.ErrOperationFailed.ID {
		t.Fatalf("Failed to verify signature mismatch for binary:%v. Got errorid=%v[%v], want errorid=%v", binaryVON, verror.ErrorID(err), err, errors.ErrOperationFailed.ID)
	}

	// Restore the binary and verify that installation succeeds.
	up[0] = up[0] ^ 0xFF
	if err := binarylib.Delete(ctx, naming.Join(binaryVON, "testbinary")); err != nil {
		t.Fatalf("Delete(%v) failed:%v", binaryVON, err)
	}
	if _, err := binarylib.Upload(ctx, naming.Join(binaryVON, "testbinary"), up, mediaInfo); err != nil {
		t.Fatalf("Upload(%v) failed:%v", binaryVON, err)
	}
	if _, err := utiltest.AppStub().Install(ctx, utiltest.MockApplicationRepoName, device.Config{}, nil); err != nil {
		t.Fatalf("Failed to Install app:%v", err)
	}

	// Verify that when the package contents are corrupted, signature verification fails.
	pkgContents[0] = pkgContents[0] ^ 0xFF
	if err := binarylib.Delete(ctx, pkgVON); err != nil {
		t.Fatalf("Delete(%v) failed:%v", pkgVON, err)
	}
	if err := os.Remove(filepath.Join(tmpdir, "pkg.txt")); err != nil {
		t.Fatalf("Remove(%v) failed:%v", filepath.Join(tmpdir, "pkg.txt"), err)
	}
	if err := ioutil.WriteFile(filepath.Join(tmpdir, "pkg.txt"), pkgContents, 0600); err != nil {
		t.Fatalf("ioutil.WriteFile failed: %v", err)
	}
	if _, err = binarylib.UploadFromDir(ctx, pkgVON, tmpdir); err != nil {
		t.Fatalf("binarylib.UploadFromDir failed: %v", err)
	}
	if _, err := utiltest.AppStub().Install(ctx, utiltest.MockApplicationRepoName, device.Config{}, nil); verror.ErrorID(err) != errors.ErrOperationFailed.ID {
		t.Fatalf("Failed to verify signature mismatch for package:%v", pkgVON)
	}
}
