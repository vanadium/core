// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/verror"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	appd "v.io/x/ref/services/application/applicationd"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/services/repository"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

var appRepository = gosh.RegisterFunc("appRepository", func(publishName, storedir string) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	defer fmt.Printf("%v terminating\n", publishName)
	defer ctx.VI(1).Infof("%v terminating", publishName)

	dispatcher, err := appd.NewDispatcher(storedir)
	if err != nil {
		ctx.Fatalf("Failed to create repository dispatcher: %v", err)
	}
	ctx, server, err := v23.WithNewDispatchingServer(ctx, publishName, dispatcher)
	if err != nil {
		ctx.Fatalf("NewDispatchingServer(%v) failed: %v", publishName, err)
	}
	ctx.VI(1).Infof("applicationd name: %v", server.Status().Endpoints[0].Name())

	fmt.Println("READY")
	<-signals.ShutdownOnSignals(ctx)
})

func TestApplicationUpdatePermissions(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// V23InitWithMounttable sets the context up with a self-signed principal,
	// whose blessing (test-blessing) will act as the root blessing for the test.
	const rootBlessing = test.TestBlessing
	idp := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	// Call ourselves test-blessing:self, distinct from test-blessing:other
	// which we'll give to the 'other' context.
	if err := idp.Bless(v23.GetPrincipal(ctx), "self"); err != nil {
		t.Fatal(err)
	}

	sh, deferFn := servicetest.CreateShell(t, ctx)
	defer deferFn()

	// setup mock up directory to put state in
	storedir, cleanup := servicetest.SetupRootDir(t, "application")
	defer cleanup()

	cmd := sh.FuncCmd(appRepository, "repo", storedir)
	cmd.Start()
	cmd.S.Expect("READY")

	otherCtx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if err := idp.Bless(v23.GetPrincipal(otherCtx), "other"); err != nil {
		t.Fatal(err)
	}

	v1stub := repository.ApplicationClient("repo/search/v1")
	repostub := repository.ApplicationClient("repo")

	// Create example envelopes.
	envelopeV1 := application.Envelope{
		Args:   []string{"--help"},
		Env:    []string{"DEBUG=1"},
		Binary: application.SignedFile{File: "/v23/name/of/binary"},
	}

	// Envelope putting as other should fail.
	if err := v1stub.Put(otherCtx, "base", envelopeV1, false); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Put() returned errorid=%v wanted errorid=%v [%v]", verror.ErrorID(err), verror.ErrNoAccess.ID, err)
	}

	// Envelope putting as global should succeed.
	if err := v1stub.Put(ctx, "base", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	ctx.VI(2).Infof("Accessing the Permission Lists of the root returns a (simulated) list providing default authorization.")
	perms, version, err := repostub.GetPermissions(ctx)
	if err != nil {
		t.Fatalf("GetPermissions should not have failed: %v", err)
	}
	if got, want := version, ""; got != want {
		t.Fatalf("GetPermissions got %v, want %v", got, want)
	}
	expectedInBps := []security.BlessingPattern{rootBlessing + ":$", rootBlessing + ":self:$", rootBlessing + ":self:child"}
	expected := access.Permissions{
		"Admin":   access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Read":    access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Write":   access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Debug":   access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Resolve": access.AccessList{In: expectedInBps, NotIn: []string(nil)},
	}
	if got := perms; !reflect.DeepEqual(expected.Normalize(), got.Normalize()) {
		t.Errorf("got %#v, expected %#v ", got, expected)
	}

	ctx.VI(2).Infof("self attempting to give other permission to update application")
	newPerms := make(access.Permissions)
	for _, tag := range access.AllTypicalTags() {
		newPerms.Add(rootBlessing+":self", string(tag))
		newPerms.Add(rootBlessing+":other", string(tag))
	}
	if err := repostub.SetPermissions(ctx, newPerms, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	perms, version, err = repostub.GetPermissions(ctx)
	if err != nil {
		t.Fatalf("GetPermissions should not have failed: %v", err)
	}
	expected = newPerms
	if got := perms; !reflect.DeepEqual(expected.Normalize(), got.Normalize()) {
		t.Errorf("got %#v, exected %#v ", got, expected)
	}

	// Envelope putting as other should now succeed.
	if err := v1stub.Put(otherCtx, "base", envelopeV1, true); err != nil {
		t.Fatalf("Put() wrongly failed: %v", err)
	}

	// Other takes control.
	perms, version, err = repostub.GetPermissions(otherCtx)
	if err != nil {
		t.Fatalf("GetPermissions 2 should not have failed: %v", err)
	}
	perms["Admin"] = access.AccessList{
		In:    []security.BlessingPattern{rootBlessing + ":other"},
		NotIn: []string{}}
	if err = repostub.SetPermissions(otherCtx, perms, version); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	// Self is now locked out but other isn't.
	if _, _, err = repostub.GetPermissions(ctx); err == nil {
		t.Fatalf("GetPermissions should not have succeeded")
	}
	perms, _, err = repostub.GetPermissions(otherCtx)
	if err != nil {
		t.Fatalf("GetPermissions should not have failed: %v", err)
	}
	expected = access.Permissions{
		"Admin":   access.AccessList{In: []security.BlessingPattern{rootBlessing + ":other"}, NotIn: []string{}},
		"Read":    access.AccessList{In: []security.BlessingPattern{rootBlessing + ":other", rootBlessing + ":self"}, NotIn: []string{}},
		"Write":   access.AccessList{In: []security.BlessingPattern{rootBlessing + ":other", rootBlessing + ":self"}, NotIn: []string{}},
		"Debug":   access.AccessList{In: []security.BlessingPattern{rootBlessing + ":other", rootBlessing + ":self"}, NotIn: []string{}},
		"Resolve": access.AccessList{In: []security.BlessingPattern{rootBlessing + ":other", rootBlessing + ":self"}, NotIn: []string{}}}

	if got := perms; !reflect.DeepEqual(expected.Normalize(), got.Normalize()) {
		t.Errorf("got %#v, exected %#v ", got, expected)
	}
}

func TestPerAppPermissions(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	// By default, all principals in this test will have blessings generated based
	// on the username/machine running this process. Give them recognizable names
	// ("root:self" etc.), so the Permissions can be set deterministically.
	idp := testutil.NewIDProvider("root")
	if err := idp.Bless(v23.GetPrincipal(ctx), "self"); err != nil {
		t.Fatal(err)
	}

	sh, deferFn := servicetest.CreateShellAndMountTable(t, ctx)
	defer deferFn()

	// setup mock up directory to put state in
	storedir, cleanup := servicetest.SetupRootDir(t, "application")
	defer cleanup()

	otherCtx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if err := idp.Bless(v23.GetPrincipal(otherCtx), "other"); err != nil {
		t.Fatal(err)
	}

	cmd := sh.FuncCmd(appRepository, "repo", storedir)
	cmd.Start()
	cmd.S.Expect("READY")

	// Create example envelope.
	envelopeV1 := application.Envelope{
		Args:   []string{"--help"},
		Env:    []string{"DEBUG=1"},
		Binary: application.SignedFile{File: "/v23/name/of/binary"},
	}

	ctx.VI(2).Info("Upload an envelope")
	v1stub := repository.ApplicationClient("repo/search/v1")
	if err := v1stub.Put(ctx, "base", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	v2stub := repository.ApplicationClient("repo/search/v2")
	if err := v2stub.Put(ctx, "base", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	v3stub := repository.ApplicationClient("repo/naps/v1")
	if err := v3stub.Put(ctx, "base", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	ctx.VI(2).Info("Self can access Permissions but other can't.")
	expectedSelfInBps := []security.BlessingPattern{"root:$", "root:self"}
	expectedSelfPermissions := access.Permissions{
		"Admin":   access.AccessList{In: expectedSelfInBps, NotIn: []string{}},
		"Read":    access.AccessList{In: expectedSelfInBps, NotIn: []string{}},
		"Write":   access.AccessList{In: expectedSelfInBps, NotIn: []string{}},
		"Debug":   access.AccessList{In: expectedSelfInBps, NotIn: []string{}},
		"Resolve": access.AccessList{In: expectedSelfInBps, NotIn: []string{}},
	}

	for _, path := range []string{"repo/search", "repo/search/v1", "repo/search/v2", "repo/naps", "repo/naps/v1"} {
		stub := repository.ApplicationClient(path)
		perms, _, err := stub.GetPermissions(ctx)
		if err != nil {
			t.Fatalf("Newly uploaded envelopes failed to receive permission lists: %v", err)
		}

		if got := perms; !reflect.DeepEqual(expectedSelfPermissions.Normalize(), got.Normalize()) {
			t.Errorf("got %#v, expected %#v ", got, expectedSelfPermissions)
		}

		// But otherCtx doesn't have admin permissions so has no access.
		if _, _, err := stub.GetPermissions(otherCtx); err == nil {
			t.Fatalf("GetPermissions didn't fail for other when it should have.")
		}
	}

	ctx.VI(2).Infof("Self sets root Permissions.")
	repostub := repository.ApplicationClient("repo")
	newPerms := make(access.Permissions)
	for _, tag := range access.AllTypicalTags() {
		newPerms.Add("root:self", string(tag))
	}
	if err := repostub.SetPermissions(ctx, newPerms, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	ctx.VI(2).Infof("Other still can't access anything.")
	if _, _, err = repostub.GetPermissions(otherCtx); err == nil {
		t.Fatalf("GetPermissions should have failed")
	}

	ctx.VI(2).Infof("Self gives other full access to repo/search/...")
	newPerms, version, err := v1stub.GetPermissions(ctx)
	if err != nil {
		t.Fatalf("GetPermissions should not have failed: %v", err)
	}
	for _, tag := range access.AllTypicalTags() {
		newPerms.Add("root:other", string(tag))
	}
	if err := v1stub.SetPermissions(ctx, newPerms, version); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	expectedInBps := []security.BlessingPattern{"root:$", "root:other", "root:self"}
	expected := access.Permissions{
		"Resolve": access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Admin":   access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Read":    access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Write":   access.AccessList{In: expectedInBps, NotIn: []string(nil)},
		"Debug":   access.AccessList{In: expectedInBps, NotIn: []string(nil)},
	}

	for _, path := range []string{"repo/search", "repo/search/v1", "repo/search/v2"} {
		stub := repository.ApplicationClient(path)
		ctx.VI(2).Infof("Other can now access this app independent of version.")
		perms, _, err := stub.GetPermissions(otherCtx)
		if err != nil {
			t.Fatalf("GetPermissions should not have failed: %v", err)
		}

		if got := perms; !reflect.DeepEqual(expected.Normalize(), got.Normalize()) {
			t.Errorf("got %#v, expected %#v ", got, expected)
		}
		ctx.VI(2).Infof("Self can also access thanks to hierarchical auth.")
		if _, _, err = stub.GetPermissions(ctx); err != nil {
			t.Fatalf("GetPermissions should not have failed: %v", err)
		}
	}

	ctx.VI(2).Infof("But other locations are unaffected and other cannot access.")
	for _, path := range []string{"repo/naps", "repo/naps/v1"} {
		stub := repository.ApplicationClient(path)
		if _, _, err := stub.GetPermissions(otherCtx); err == nil {
			t.Fatalf("GetPermissions didn't fail when it should have.")
		}
	}

	// Self gives other write perms on base.
	newPerms, version, err = repostub.GetPermissions(ctx)
	if err != nil {
		t.Fatalf("GetPermissions should not have failed: %v", err)
	}
	newPerms["Write"] = access.AccessList{In: []security.BlessingPattern{"root:other", "root:self"}}
	if err := repostub.SetPermissions(ctx, newPerms, version); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	// Other can now upload an envelope at both locations.
	for _, stub := range []repository.ApplicationClientStub{v1stub, v2stub} {
		if err := stub.Put(otherCtx, "base", envelopeV1, true); err != nil {
			t.Fatalf("Put() failed: %v", err)
		}
	}

	// But because application search already exists, the Permissions do not change.
	for _, path := range []string{"repo/search", "repo/search/v1", "repo/search/v2"} {
		stub := repository.ApplicationClient(path)
		perms, _, err := stub.GetPermissions(otherCtx)
		if err != nil {
			t.Fatalf("GetPermissions should not have failed: %v", err)
		}
		if got := perms; !reflect.DeepEqual(expected.Normalize(), got.Normalize()) {
			t.Errorf("got %#v, expected %#v ", got, expected)
		}
	}

	// But self didn't give other Permissions modification permissions.
	for _, path := range []string{"repo/search", "repo/search/v2"} {
		stub := repository.ApplicationClient(path)
		if _, _, err := stub.GetPermissions(otherCtx); err != nil {
			t.Fatalf("GetPermissions failed when it should not have for same application: %v", err)
		}
	}
}
