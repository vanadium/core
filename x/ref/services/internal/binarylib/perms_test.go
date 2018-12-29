// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binarylib_test

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/repository"
	"v.io/v23/verror"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/services/internal/binarylib"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

var binaryd = gosh.RegisterFunc("binaryd", func(publishName, storedir string) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	defer fmt.Printf("%v terminating\n", publishName)
	defer ctx.VI(1).Infof("%v terminating", publishName)

	depth := 2
	state, err := binarylib.NewState(storedir, "", depth)
	if err != nil {
		ctx.Fatalf("NewState(%v, %v, %v) failed: %v", storedir, "", depth, err)
	}
	dispatcher, err := binarylib.NewDispatcher(ctx, state)
	if err != nil {
		ctx.Fatalf("Failed to create binaryd dispatcher: %v", err)
	}
	ctx, server, err := v23.WithNewDispatchingServer(ctx, publishName, dispatcher)
	if err != nil {
		ctx.Fatalf("NewDispatchingServer(%v) failed: %v", publishName, err)
	}
	ctx.VI(1).Infof("binaryd name: %v", server.Status().Endpoints[0].Name())

	fmt.Println("READY")
	<-signals.ShutdownOnSignals(ctx)
})

func b(name string) repository.BinaryClientStub {
	return repository.BinaryClient(name)
}

func ctxWithBlessedPrincipal(ctx *context.T, childExtension string) (*context.T, error) {
	child := testutil.NewPrincipal()
	if err := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx)).Bless(child, childExtension); err != nil {
		return nil, err
	}
	return v23.WithPrincipal(ctx, child)
}

func TestBinaryCreateAccessList(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	rg := testutil.NewRandGenerator(t.Logf)

	selfCtx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("self"))
	if err != nil {
		t.Fatalf("WithPrincipal failed: %v", err)
	}
	childCtx, err := ctxWithBlessedPrincipal(selfCtx, "child")
	if err != nil {
		t.Fatalf("WithPrincipal failed: %v", err)
	}

	sh, deferFn := servicetest.CreateShellAndMountTable(t, childCtx)
	defer deferFn()
	// make selfCtx and childCtx have the same Namespace Roots as set by
	// CreateShellAndMountTable
	v23.GetNamespace(selfCtx).SetRoots(v23.GetNamespace(childCtx).Roots()...)

	// setup mock up directory to put state in
	storedir, cleanup := servicetest.SetupRootDir(t, "bindir")
	defer cleanup()
	prepDirectory(t, storedir)

	nmh := sh.FuncCmd(binaryd, "bini", storedir)
	nmh.Start()
	nmh.S.Expect("READY")

	ctx.VI(2).Infof("Self uploads a shared and private binary.")
	if err := b("bini/private").Create(childCtx, 1, repository.MediaInfo{Type: "application/octet-stream"}); err != nil {
		t.Fatalf("Create() failed %v", err)
	}
	fakeDataPrivate := testData(rg)
	if streamErr, err := invokeUpload(t, childCtx, b("bini/private"), fakeDataPrivate, 0); streamErr != nil || err != nil {
		t.Fatalf("invokeUpload() failed %v, %v", err, streamErr)
	}

	ctx.VI(2).Infof("Validate that the AccessList also allows Self")
	perms, _, err := b("bini/private").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	expectedInBps := []security.BlessingPattern{"self:$", "self:child"}
	expected := access.Permissions{
		"Admin":   access.AccessList{In: expectedInBps},
		"Read":    access.AccessList{In: expectedInBps},
		"Write":   access.AccessList{In: expectedInBps},
		"Debug":   access.AccessList{In: expectedInBps},
		"Resolve": access.AccessList{In: expectedInBps},
	}
	if got, want := perms.Normalize(), expected.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}
}

func TestBinaryRootAccessList(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	rg := testutil.NewRandGenerator(t.Logf)

	selfPrincipal := testutil.NewPrincipal("self")
	selfBlessings, _ := selfPrincipal.BlessingStore().Default()
	selfCtx, err := v23.WithPrincipal(ctx, selfPrincipal)
	if err != nil {
		t.Fatalf("WithPrincipal failed: %v", err)
	}
	sh, deferFn := servicetest.CreateShellAndMountTable(t, selfCtx)
	defer deferFn()

	// setup mock up directory to put state in
	storedir, cleanup := servicetest.SetupRootDir(t, "bindir")
	defer cleanup()
	prepDirectory(t, storedir)

	otherPrincipal := testutil.NewPrincipal("other")
	if err := security.AddToRoots(otherPrincipal, selfBlessings); err != nil {
		t.Fatalf("AddToRoots() failed: %v", err)
	}
	otherCtx, err := v23.WithPrincipal(selfCtx, otherPrincipal)
	if err != nil {
		t.Fatalf("WithPrincipal() failed: %v", err)
	}

	nmh := sh.FuncCmd(binaryd, "bini", storedir)
	nmh.Start()
	nmh.S.Expect("READY")

	ctx.VI(2).Infof("Self uploads a shared and private binary.")
	if err := b("bini/private").Create(selfCtx, 1, repository.MediaInfo{Type: "application/octet-stream"}); err != nil {
		t.Fatalf("Create() failed %v", err)
	}
	fakeDataPrivate := testData(rg)
	if streamErr, err := invokeUpload(t, selfCtx, b("bini/private"), fakeDataPrivate, 0); streamErr != nil || err != nil {
		t.Fatalf("invokeUpload() failed %v, %v", err, streamErr)
	}

	if err := b("bini/shared").Create(selfCtx, 1, repository.MediaInfo{Type: "application/octet-stream"}); err != nil {
		t.Fatalf("Create() failed %v", err)
	}
	fakeDataShared := testData(rg)
	if streamErr, err := invokeUpload(t, selfCtx, b("bini/shared"), fakeDataShared, 0); streamErr != nil || err != nil {
		t.Fatalf("invokeUpload() failed %v, %v", err, streamErr)
	}

	ctx.VI(2).Infof("Verify that in the beginning other can't access bini/private or bini/shared")
	if _, _, err := b("bini/private").Stat(otherCtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Stat() should have failed but didn't: %v", err)
	}
	if _, _, err := b("bini/shared").Stat(otherCtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Stat() should have failed but didn't: %v", err)
	}

	ctx.VI(2).Infof("Validate the AccessList file on bini/private.")
	perms, _, err := b("bini/private").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	expectedInBps := []security.BlessingPattern{"self"}
	expected := access.Permissions{
		"Admin":   access.AccessList{In: expectedInBps},
		"Read":    access.AccessList{In: expectedInBps},
		"Write":   access.AccessList{In: expectedInBps},
		"Debug":   access.AccessList{In: expectedInBps},
		"Resolve": access.AccessList{In: expectedInBps},
	}
	if got, want := perms.Normalize(), expected.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}

	ctx.VI(2).Infof("Validate the AccessList file on bini/private.")
	perms, version, err := b("bini/private").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	if got, want := perms.Normalize(), expected.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}

	ctx.VI(2).Infof("self blesses other as self/other and locks the bini/private binary to itself.")
	selfBlessing, _ := selfPrincipal.BlessingStore().Default()
	otherBlessing, err := selfPrincipal.Bless(otherPrincipal.PublicKey(), selfBlessing, "other", security.UnconstrainedUse())
	if err != nil {
		t.Fatalf("selfPrincipal.Bless() failed: %v", err)
	}
	if _, err := otherPrincipal.BlessingStore().Set(otherBlessing, security.AllPrincipals); err != nil {
		t.Fatalf("otherPrincipal.BlessingStore() failed: %v", err)
	}

	ctx.VI(2).Infof("Self modifies the AccessList file on bini/private.")
	for _, tag := range access.AllTypicalTags() {
		perms.Clear("self", string(tag))
		perms.Add("self:$", string(tag))
	}
	if err := b("bini/private").SetPermissions(selfCtx, perms, version); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	ctx.VI(2).Infof(" Verify that bini/private's perms are updated.")
	updatedInBps := []security.BlessingPattern{"self:$"}
	updated := access.Permissions{
		"Admin":   access.AccessList{In: updatedInBps},
		"Read":    access.AccessList{In: updatedInBps},
		"Write":   access.AccessList{In: updatedInBps},
		"Debug":   access.AccessList{In: updatedInBps},
		"Resolve": access.AccessList{In: updatedInBps},
	}
	perms, _, err = b("bini/private").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	if got, want := perms.Normalize(), updated.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}

	// Other still can't access bini/shared because there's no AccessList file at the
	// root level. Self has to set one explicitly to enable sharing. This way, self
	// can't accidentally expose the server without setting a root AccessList.
	ctx.VI(2).Infof(" Verify that other still can't access bini/shared.")
	if _, _, err := b("bini/shared").Stat(otherCtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Stat() should have failed but didn't: %v", err)
	}

	ctx.VI(2).Infof("Self sets a root AccessList.")
	newRootAccessList := make(access.Permissions)
	for _, tag := range access.AllTypicalTags() {
		newRootAccessList.Add("self:$", string(tag))
	}
	if err := b("bini").SetPermissions(selfCtx, newRootAccessList, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	ctx.VI(2).Infof("Verify that other can access bini/shared now but not access bini/private.")
	if _, _, err := b("bini/shared").Stat(otherCtx); err != nil {
		t.Fatalf("Stat() shouldn't have failed: %v", err)
	}
	if _, _, err := b("bini/private").Stat(otherCtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Stat() should have failed but didn't: %v", err)
	}

	ctx.VI(2).Infof("Other still can't create so Self gives Other right to Create.")
	perms, tag, err := b("bini").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions() failed: %v", err)
	}

	// More than one AccessList change will result in the same functional result in
	// this test: that self:other acquires the right to invoke Create at the
	// root. In particular:
	//
	// a. perms.Add("self", "Write ")
	// b. perms.Add("self:other", "Write")
	// c. perms.Add("self:other:$", "Write")
	//
	// will all give self:other the right to invoke Create but in the case of
	// (a) it will also extend this right to self's delegates (because of the
	// absence of the $) including other and in (b) will also extend the
	// Create right to all of other's delegates. Since (c) is the minimum
	// case, use that.
	perms.Add("self:other:$", string("Write"))
	err = b("bini").SetPermissions(selfCtx, perms, tag)
	if err != nil {
		t.Fatalf("SetPermissions() failed: %v", err)
	}

	ctx.VI(2).Infof("Other creates bini/otherbinary")
	if err := b("bini/otherbinary").Create(otherCtx, 1, repository.MediaInfo{Type: "application/octet-stream"}); err != nil {
		t.Fatalf("Create() failed %v", err)
	}
	fakeDataOther := testData(rg)
	if streamErr, err := invokeUpload(t, otherCtx, b("bini/otherbinary"), fakeDataOther, 0); streamErr != nil || err != nil {
		t.FailNow()
	}

	ctx.VI(2).Infof("Other can read perms for bini/otherbinary.")
	updatedInBps = []security.BlessingPattern{"self:$", "self:other"}
	updated = access.Permissions{
		"Admin":   access.AccessList{In: updatedInBps},
		"Read":    access.AccessList{In: updatedInBps},
		"Write":   access.AccessList{In: updatedInBps},
		"Debug":   access.AccessList{In: updatedInBps},
		"Resolve": access.AccessList{In: updatedInBps},
	}
	perms, _, err = b("bini/otherbinary").GetPermissions(otherCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	if got, want := perms.Normalize(), updated.Normalize(); !reflect.DeepEqual(want, got) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}

	ctx.VI(2).Infof("Other tries to exclude self by removing self from the AccessList set")
	perms, tag, err = b("bini/otherbinary").GetPermissions(otherCtx)
	if err != nil {
		t.Fatalf("GetPermissions() failed: %v", err)
	}
	perms.Clear("self:$")
	err = b("bini/otherbinary").SetPermissions(otherCtx, perms, tag)
	if err != nil {
		t.Fatalf("SetPermissions() failed: %v", err)
	}

	ctx.VI(2).Infof("Verify that other can make this change.")
	updatedInBps = []security.BlessingPattern{"self:other"}
	updated = access.Permissions{
		"Admin":   access.AccessList{In: updatedInBps},
		"Read":    access.AccessList{In: updatedInBps},
		"Write":   access.AccessList{In: updatedInBps},
		"Debug":   access.AccessList{In: updatedInBps},
		"Resolve": access.AccessList{In: updatedInBps},
	}
	perms, _, err = b("bini/otherbinary").GetPermissions(otherCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	if got, want := perms.Normalize(), updated.Normalize(); !reflect.DeepEqual(want, got) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}

	ctx.VI(2).Infof("But self's rights are inherited from root so self can still access despite this.")
	if _, _, err := b("bini/otherbinary").Stat(selfCtx); err != nil {
		t.Fatalf("Stat() shouldn't have failed: %v", err)
	}

	ctx.VI(2).Infof("Self petulantly blacklists other back.")
	perms, tag, err = b("bini").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions() failed: %v", err)
	}
	for _, tag := range access.AllTypicalTags() {
		perms.Blacklist("self:other", string(tag))
	}
	err = b("bini").SetPermissions(selfCtx, perms, tag)
	if err != nil {
		t.Fatalf("SetPermissions() failed: %v", err)
	}

	ctx.VI(2).Infof("And now other can do nothing at affecting the root. Other should be penitent.")
	if err := b("bini/nototherbinary").Create(otherCtx, 1, repository.MediaInfo{Type: "application/octet-stream"}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Create() should have failed %v", err)
	}

	ctx.VI(2).Infof("But other can still access shared.")
	if _, _, err := b("bini/shared").Stat(otherCtx); err != nil {
		t.Fatalf("Stat() should not have failed but did: %v", err)
	}

	ctx.VI(2).Infof("Self petulantly blacklists other's binary too.")
	perms, tag, err = b("bini/shared").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions() failed: %v", err)
	}
	for _, tag := range access.AllTypicalTags() {
		perms.Blacklist("self:other", string(tag))
	}
	err = b("bini/shared").SetPermissions(selfCtx, perms, tag)
	if err != nil {
		t.Fatalf("SetPermissions() failed: %v", err)
	}
	ctx.VI(2).Infof("And now other can't access shared either.")
	if _, _, err := b("bini/shared").Stat(otherCtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Stat() should have failed but didn't: %v", err)
	}
	// TODO(rjkroege): Extend the test with a third principal and verify that
	// new principals can be given Admin perimission at the root.

	ctx.VI(2).Infof("Self feels guilty for petulance and disempowers itself")
	// TODO(rjkroege,caprita): This is a one-way transition for self. Perhaps it
	// should not be. Consider adding a factory-reset facility.
	perms, tag, err = b("bini").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions() failed: %v", err)
	}
	perms.Clear("self:$", "Admin")
	err = b("bini").SetPermissions(selfCtx, perms, tag)
	if err != nil {
		t.Fatalf("SetPermissions() failed: %v", err)
	}

	ctx.VI(2).Info("Self can't access other's binary now")
	if _, _, err := b("bini/otherbinary").Stat(selfCtx); err == nil {
		t.Fatalf("Stat() should have failed but didn't")
	}
}

func TestBinaryRationalStartingValueForGetPermissions(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	selfPrincipal := testutil.NewPrincipal("self")
	selfBlessings, _ := selfPrincipal.BlessingStore().Default()
	selfCtx, err := v23.WithPrincipal(ctx, selfPrincipal)
	if err != nil {
		t.Fatalf("WithPrincipal failed: %v", err)
	}
	sh, deferFn := servicetest.CreateShellAndMountTable(t, selfCtx)
	defer deferFn()

	// setup mock up directory to put state in
	storedir, cleanup := servicetest.SetupRootDir(t, "bindir")
	defer cleanup()
	prepDirectory(t, storedir)

	otherPrincipal := testutil.NewPrincipal("other")
	if err := security.AddToRoots(otherPrincipal, selfBlessings); err != nil {
		t.Fatalf("AddToRoots() failed: %v", err)
	}

	nmh := sh.FuncCmd(binaryd, "bini", storedir)
	nmh.Start()
	nmh.S.Expect("READY")

	perms, tag, err := b("bini").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %#v", err)
	}
	expectedInBps := []security.BlessingPattern{"self:$", "self:child"}
	expected := access.Permissions{
		"Admin":   access.AccessList{In: expectedInBps, NotIn: []string{}},
		"Read":    access.AccessList{In: expectedInBps, NotIn: []string{}},
		"Write":   access.AccessList{In: expectedInBps, NotIn: []string{}},
		"Debug":   access.AccessList{In: expectedInBps, NotIn: []string{}},
		"Resolve": access.AccessList{In: expectedInBps, NotIn: []string{}},
	}
	if got, want := perms.Normalize(), expected.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}

	perms.Blacklist("self", string("Read"))
	err = b("bini").SetPermissions(selfCtx, perms, tag)
	if err != nil {
		t.Fatalf("SetPermissions() failed: %v", err)
	}

	perms, tag, err = b("bini").GetPermissions(selfCtx)
	if err != nil {
		t.Fatalf("GetPermissions failed: %#v", err)
	}
	expectedInBps = []security.BlessingPattern{"self:$", "self:child"}
	expected = access.Permissions{
		"Admin":   access.AccessList{In: expectedInBps, NotIn: []string{}},
		"Read":    access.AccessList{In: expectedInBps, NotIn: []string{"self"}},
		"Write":   access.AccessList{In: expectedInBps, NotIn: []string{}},
		"Debug":   access.AccessList{In: expectedInBps, NotIn: []string{}},
		"Resolve": access.AccessList{In: expectedInBps, NotIn: []string{}},
	}
	if got, want := perms.Normalize(), expected.Normalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, expected %#v ", got, want)
	}
}
