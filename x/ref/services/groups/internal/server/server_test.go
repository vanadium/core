// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server_test

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/groups"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/groups/internal/server"
	"v.io/x/ref/services/groups/internal/store"
	"v.io/x/ref/services/groups/internal/store/mem"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type backend int

const (
	memstore backend = iota
)

func Fatalf(t *testing.T, format string, args ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, args...)
}

func getEntriesOrDie(t *testing.T, ctx *context.T, g groups.GroupClientStub) map[groups.BlessingPatternChunk]struct{} {
	res, _, err := g.Get(ctx, groups.GetRequest{}, "")
	if err != nil {
		Fatalf(t, "Get failed: %v", err)
	}
	return res.Entries
}

func getPermsOrDie(t *testing.T, ctx *context.T, g groups.GroupClientStub) access.Permissions {
	res, _, err := g.GetPermissions(ctx)
	if err != nil {
		Fatalf(t, "GetPermissions failed: %v", err)
	}
	return res
}

func getVersionOrDie(t *testing.T, ctx *context.T, g groups.GroupClientStub) string {
	_, version, err := g.Get(ctx, groups.GetRequest{}, "")
	if err != nil {
		Fatalf(t, "Get failed: %v", err)
	}
	return version
}

func bpc(chunk string) groups.BlessingPatternChunk {
	return groups.BlessingPatternChunk(chunk)
}

func bpcSet(chunks ...string) map[groups.BlessingPatternChunk]struct{} {
	res := map[groups.BlessingPatternChunk]struct{}{}
	for _, chunk := range chunks {
		res[bpc(chunk)] = struct{}{}
	}
	return res
}

func bpcSlice(chunks ...string) []groups.BlessingPatternChunk {
	res := []groups.BlessingPatternChunk{}
	for _, chunk := range chunks {
		res = append(res, bpc(chunk))
	}
	return res
}

func entriesEqual(a, b map[groups.BlessingPatternChunk]struct{}) bool {
	// Unlike DeepEqual, we treat nil and empty maps as equivalent.
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// security.Authorizer implementation that disallows any operations on a
// "reserved*" suffix. Used to test that the provided authorization policy
// is applied to group creation operations.
type reservedAuthorizer struct{}

func (reservedAuthorizer) Authorize(_ *context.T, call security.Call) error {
	if strings.HasPrefix(call.Suffix(), "reserved") {
		return fmt.Errorf("operations on %q are reserved and not authorized", call.Suffix())
	}
	return nil
}

func newServer(ctx *context.T, be backend) (string, func()) {
	var st store.Store
	var path string
	var err error

	switch be {
	case memstore:
		st = mem.New()
	default:
		ctx.Fatal("unknown backend: ", be)
	}

	m := server.NewManager(st, reservedAuthorizer{})

	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", m)
	if err != nil {
		ctx.Fatal("NewDispatchingServer() failed: ", err)
	}

	name := server.Status().Endpoints[0].Name()
	return name, func() {
		cancel()
		<-server.Closed()
		if path != "" {
			os.RemoveAll(path)
		}
	}
}

func setupOrDie(be backend) (clientCtx *context.T, serverName string, cleanup func()) {
	ctx, shutdown := test.V23Init()
	serverCtx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal())
	if err != nil {
		ctx.Fatal(err)
	}
	if clientCtx, err = v23.WithPrincipal(ctx, testutil.NewPrincipal()); err != nil {
		ctx.Fatal(err)
	}

	// Provide the client and server blessings from a common identity provider.
	idp := testutil.NewIDProvider("idp")
	if err := idp.Bless(v23.GetPrincipal(clientCtx), "client"); err != nil {
		ctx.Fatal(err)
	}
	if err := idp.Bless(v23.GetPrincipal(serverCtx), "server"); err != nil {
		ctx.Fatal(err)
	}
	serverName, stopServer := newServer(serverCtx, be)
	cleanup = func() {
		stopServer()
		shutdown()
	}
	return
}

// Test cases
// ==========

func TestCreateMemStore(t *testing.T) {
	testCreateHelper(t, memstore)
}

func testCreateHelper(t *testing.T, be backend) {
	ctx, serverName, cleanup := setupOrDie(be)
	defer cleanup()

	// Unauthorized creates should fail.
	// This fails because of the reservedAuthorizer used in setupOrDie.
	if err := groups.GroupClient(naming.JoinAddressName(serverName, "reservedGroup")).Create(ctx, nil, nil); err == nil {
		t.Errorf("Creation of reservedGroup succeeded")
	}
	// Create a group with a default perms and no entries.
	g := groups.GroupClient(naming.JoinAddressName(serverName, "grpA"))
	if err := g.Create(ctx, nil, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Verify perms of created group.
	perms := access.Permissions{}
	for _, tag := range access.AllTypicalTags() {
		perms.Add(security.BlessingPattern("idp:client"), string(tag))
	}
	gotPermissions, wantPermissions := getPermsOrDie(t, ctx, g), perms
	if !reflect.DeepEqual(gotPermissions, wantPermissions) {
		t.Errorf("Permissions do not match: got %v, want %v", gotPermissions, wantPermissions)
	}
	// Verify entries of created group.
	got, want := getEntriesOrDie(t, ctx, g), bpcSet()
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}

	// Creating same group again should fail, since the group already exists.
	g = groups.GroupClient(naming.JoinAddressName(serverName, "grpA"))
	if err := g.Create(ctx, nil, nil); !errors.Is(err, verror.ErrExist) {
		t.Fatalf("Create should have failed: %v", err)
	}

	// Create a group with perms and a few entries, including some redundant ones.
	g = groups.GroupClient(naming.JoinAddressName(serverName, "grpB"))
	perms = access.Permissions{}
	// Allow Admin and Read so that we can call GetPermissions and Get.
	for _, tag := range []access.Tag{access.Admin, access.Read} {
		perms.Add(security.BlessingPattern("idp:client"), string(tag))
	}
	if err := g.Create(ctx, perms, bpcSlice("foo", "bar", "foo")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Verify perms of created group.
	gotPermissions, wantPermissions = getPermsOrDie(t, ctx, g), perms
	if !reflect.DeepEqual(gotPermissions, wantPermissions) {
		t.Errorf("Permissions do not match: got %v, want %v", gotPermissions, wantPermissions)
	}
	// Verify entries of created group.
	got, want = getEntriesOrDie(t, ctx, g), bpcSet("foo", "bar")
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}
}

func TestDeleteMemStore(t *testing.T) {
	testDeleteHelper(t, memstore)
}

func testDeleteHelper(t *testing.T, be backend) {
	ctx, serverName, cleanup := setupOrDie(be)
	defer cleanup()

	// Create a group with a default perms and no entries, check that we can
	// delete it.
	g := groups.GroupClient(naming.JoinAddressName(serverName, "grpA"))
	if err := g.Create(ctx, nil, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Delete with bad version should fail.
	if err := g.Delete(ctx, "20"); !errors.Is(err, verror.ErrBadVersion) {
		t.Fatalf("Delete should have failed with version error: %v", err)
	}
	// Delete with correct version should succeed.
	version := getVersionOrDie(t, ctx, g)
	if err := g.Delete(ctx, version); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	// Check that the group was actually deleted.
	if _, _, err := g.Get(ctx, groups.GetRequest{}, ""); !errors.Is(err, verror.ErrNoExist) {
		t.Fatal("Group was not deleted")
	}

	// Create a group with several entries, check that we can delete it.
	g = groups.GroupClient(naming.JoinAddressName(serverName, "grpB"))
	if err := g.Create(ctx, nil, bpcSlice("foo", "bar", "foo")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Delete with empty version should succeed.
	if err := g.Delete(ctx, ""); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	// Check that the group was actually deleted.
	if _, _, err := g.Get(ctx, groups.GetRequest{}, ""); !errors.Is(err, verror.ErrNoExist) {
		t.Fatal("Group was not deleted")
	}
	// Check that Delete is idempotent.
	if err := g.Delete(ctx, ""); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	// Check that we can recreate a group that was deleted.
	if err := g.Create(ctx, nil, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create a group with perms that disallow Delete(), check that Delete()
	// fails.
	g = groups.GroupClient(naming.JoinAddressName(serverName, "grpC"))
	perms := access.Permissions{}
	perms.Add(security.BlessingPattern("idp:client"), string(access.Admin))
	if err := g.Create(ctx, perms, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Delete should fail (no access).
	if err := g.Delete(ctx, ""); !errors.Is(err, verror.ErrNoAccess) {
		t.Fatalf("Delete should have failed with access error: %v", err)
	}
}

func TestPermsMemStore(t *testing.T) {
	testPermsHelper(t, memstore)
}

func testPermsHelper(t *testing.T, be backend) { //nolint:gocyclo
	ctx, serverName, cleanup := setupOrDie(be)
	defer cleanup()

	// Create a group with a default perms.
	g := groups.GroupClient(naming.JoinAddressName(serverName, "grpA"))
	if err := g.Create(ctx, nil, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Use "ac" so the code below can exactly match code in syncbase.
	// TODO(sadovsky): All Vanadium {Set,Get}Permissions tests ought to share this
	// test implementation.
	ac := g

	// Mirrors syncbase/v23/syncbase/testutil/layer.go.
	myperms := access.Permissions{}
	myperms.Add(security.BlessingPattern("idp:client"), string(access.Admin))
	// Demonstrate that myperms differs from the current perms.
	if reflect.DeepEqual(myperms, getPermsOrDie(t, ctx, ac)) {
		t.Fatalf("Permissions should not match: %v", myperms)
	}

	var permsBefore, permsAfter access.Permissions
	var versionBefore, versionAfter string

	getPermsAndVersionOrDie := func() (access.Permissions, string) {
		perms, version, err := ac.GetPermissions(ctx)
		if err != nil {
			// Use Fatalf rather than t.Fatalf so we get a stack trace.
			Fatalf(t, "GetPermissions failed: %v", err)
		}
		return perms, version
	}

	// SetPermissions with bad version should fail.
	permsBefore, versionBefore = getPermsAndVersionOrDie()
	if err := ac.SetPermissions(ctx, myperms, "20"); !errors.Is(err, verror.ErrBadVersion) {
		t.Fatalf("SetPermissions should have failed with version error: %v", err)
	}
	// Since SetPermissions failed, perms and version should not have changed.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, permsBefore) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, permsBefore)
	}
	if versionAfter != versionBefore {
		t.Errorf("Versions do not match: got %v, want %v", versionAfter, versionBefore)
	}

	// SetPermissions with correct version should succeed.
	_, versionBefore = permsAfter, versionAfter
	if err := ac.SetPermissions(ctx, myperms, versionBefore); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	// Check that perms and version actually changed.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, myperms) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, myperms)
	}
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// SetPermissions with empty version should succeed.
	_, versionBefore = permsAfter, versionAfter
	myperms.Add(security.BlessingPattern("idp:client"), string(access.Read))
	if err := ac.SetPermissions(ctx, myperms, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	// Check that perms and version actually changed.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, myperms) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, myperms)
	}
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// SetPermissions with unchanged perms should succeed, and version should
	// still change.
	permsBefore, versionBefore = permsAfter, versionAfter
	if err := ac.SetPermissions(ctx, myperms, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	// Check that perms did not change and version did change.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, permsBefore) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, permsBefore)
	}
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Take away our access. SetPermissions and GetPermissions should fail.
	if err := ac.SetPermissions(ctx, access.Permissions{}, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	if _, _, err := ac.GetPermissions(ctx); !errors.Is(err, verror.ErrNoAccess) {
		t.Fatalf("GetPermissions should have failed with access error: %v", err)
	}
	if err := ac.SetPermissions(ctx, myperms, ""); !errors.Is(err, verror.ErrNoAccess) {
		t.Fatalf("SetPermissions should have failed with access error: %v", err)
	}
}

func TestAddMemStore(t *testing.T) {
	testAddHelper(t, memstore)
}

// testAddHelper tests mirror testRemoveHelper tests.
func testAddHelper(t *testing.T, be backend) { //nolint:gocyclo
	ctx, serverName, cleanup := setupOrDie(be)
	defer cleanup()

	// Create a group with a default perms and no entries.
	g := groups.GroupClient(naming.JoinAddressName(serverName, "grpA"))
	if err := g.Create(ctx, nil, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Verify entries of created group.
	got, want := getEntriesOrDie(t, ctx, g), bpcSet()
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}

	var versionBefore, versionAfter string
	versionBefore = getVersionOrDie(t, ctx, g)
	// Add with bad version should fail.
	if err := g.Add(ctx, bpc("foo"), "20"); !errors.Is(err, verror.ErrBadVersion) {
		t.Fatalf("Add should have failed with version error: %v", err)
	}
	// Version should not have changed.
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionAfter != versionBefore {
		t.Errorf("Versions do not match: got %v, want %v", versionAfter, versionBefore)
	}

	// Add an entry, verify it was added and the version changed.
	versionBefore = versionAfter
	if err := g.Add(ctx, bpc("foo"), versionBefore); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	got, want = getEntriesOrDie(t, ctx, g), bpcSet("foo")
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Add another entry, verify it was added and the version changed.
	versionBefore = versionAfter
	// Add with empty version should succeed.
	if err := g.Add(ctx, bpc("bar"), ""); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	got, want = getEntriesOrDie(t, ctx, g), bpcSet("foo", "bar")
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Add "bar" again, verify entries are still ["foo", "bar"] and the version
	// changed.
	versionBefore = versionAfter
	if err := g.Add(ctx, bpc("bar"), versionBefore); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	got, want = getEntriesOrDie(t, ctx, g), bpcSet("foo", "bar")
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Create a group with perms that disallow Add(), check that Add() fails.
	g = groups.GroupClient(naming.JoinAddressName(serverName, "grpB"))
	perms := access.Permissions{}
	perms.Add(security.BlessingPattern("idp:client"), string(access.Admin))
	if err := g.Create(ctx, perms, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Add should fail (no access).
	if err := g.Add(ctx, bpc("foo"), ""); !errors.Is(err, verror.ErrNoAccess) {
		t.Fatalf("Add should have failed with access error: %v", err)
	}
}

func TestRemoveMemStore(t *testing.T) {
	testRemoveHelper(t, memstore)
}

// testRemoveHelper tests mirror testAddHelper tests.
func testRemoveHelper(t *testing.T, be backend) { //nolint:gocyclo
	ctx, serverName, cleanup := setupOrDie(be)
	defer cleanup()

	// Create a group with a default perms and two entries.
	g := groups.GroupClient(naming.JoinAddressName(serverName, "grpA"))
	if err := g.Create(ctx, nil, bpcSlice("foo", "bar")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Verify entries of created group.
	got, want := getEntriesOrDie(t, ctx, g), bpcSet("foo", "bar")
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}

	var versionBefore, versionAfter string
	versionBefore = getVersionOrDie(t, ctx, g)
	// Remove with bad version should fail.
	if err := g.Remove(ctx, bpc("foo"), "20"); !errors.Is(err, verror.ErrBadVersion) {
		t.Fatalf("Remove should have failed with version error: %v", err)
	}
	// Version should not have changed.
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionAfter != versionBefore {
		t.Errorf("Versions do not match: got %v, want %v", versionAfter, versionBefore)
	}

	// Remove an entry, verify it was removed and the version changed.
	versionBefore = versionAfter
	if err := g.Remove(ctx, bpc("foo"), versionBefore); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	got, want = getEntriesOrDie(t, ctx, g), bpcSet("bar")
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Remove another entry, verify it was removed and the version changed.
	versionBefore = versionAfter
	// Remove with empty version should succeed.
	if err := g.Remove(ctx, bpc("bar"), ""); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	got, want = getEntriesOrDie(t, ctx, g), bpcSet()
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Remove "bar" again, verify entries are still [] and the version changed.
	versionBefore = versionAfter
	if err := g.Remove(ctx, bpc("bar"), versionBefore); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	got, want = getEntriesOrDie(t, ctx, g), bpcSet()
	if !entriesEqual(got, want) {
		t.Errorf("Entries do not match: got %v, want %v", got, want)
	}
	versionAfter = getVersionOrDie(t, ctx, g)
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Create a group with perms that disallow Remove(), check that Remove()
	// fails.
	g = groups.GroupClient(naming.JoinAddressName(serverName, "grpB"))
	perms := access.Permissions{}
	perms.Add(security.BlessingPattern("idp:client"), string(access.Admin))
	if err := g.Create(ctx, perms, bpcSlice("foo", "bar")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Remove should fail (no access).
	if err := g.Remove(ctx, bpc("foo"), ""); !errors.Is(err, verror.ErrNoAccess) {
		t.Fatalf("Remove should have failed with access error: %v", err)
	}
}

func TestGet(t *testing.T) {
	// TODO(sadovsky): Implement.
}

func TestRest(t *testing.T) {
	// TODO(sadovsky): Implement.
}
