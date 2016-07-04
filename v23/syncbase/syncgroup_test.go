// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// Tests that Syncgroup.Create works as expected.
func TestCreateSyncgroup(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(tu.DefaultPerms(access.AllTypicalTags(), "root:o:app:client"))
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")

	// Check if create fails with empty spec.
	spec := wire.SyncgroupSpec{}
	sg1 := d.Syncgroup(ctx, "sg1").Id()

	createSyncgroup(t, ctx, d, sg1, spec, verror.ErrBadArg.ID)

	var wantGroups []wire.Id
	verifySyncgroups(t, ctx, d, wantGroups, verror.ID(""))

	// Prefill entries before creating a syncgroup to exercise the bootstrap
	// of a syncgroup through Snapshot operations to the watcher.
	c := tu.CreateCollection(t, ctx, d, "c")
	for _, k := range []string{"foo123", "foobar123", "xyz"} {
		if err := c.Put(ctx, k, "value@"+k); err != nil {
			t.Fatalf("c1.Put() of %s failed: %v", k, err)
		}
	}

	// Create successfully.
	spec = wire.SyncgroupSpec{
		Description: "test syncgroup sg1",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:o:app:client"),
		Collections: []wire.Id{c.Id()},
	}
	createSyncgroup(t, ctx, d, sg1, spec, verror.ID(""))

	// Verify syncgroup is created.
	wantGroups = []wire.Id{sg1}
	verifySyncgroups(t, ctx, d, wantGroups, verror.ID(""))
	verifySyncgroupInfo(t, ctx, d, sg1, spec, 1)

	// Check if creating an already existing syncgroup fails.
	createSyncgroup(t, ctx, d, sg1, spec, verror.ErrExist.ID)
	verifySyncgroups(t, ctx, d, wantGroups, verror.ID(""))

	// Create a peer syncgroup.
	spec.Description = "test syncgroup sg2"
	sg2 := d.Syncgroup(ctx, "sg2").Id()
	createSyncgroup(t, ctx, d, sg2, spec, verror.ID(""))

	wantGroups = []wire.Id{sg1, sg2}
	verifySyncgroups(t, ctx, d, wantGroups, verror.ID(""))
	verifySyncgroupInfo(t, ctx, d, sg2, spec, 1)

	// Check if creating a syncgroup on a non-existing collection fails.
	spec.Description = "test syncgroup sg3"
	spec.Collections = []wire.Id{wire.Id{"u", "c1"}}
	sg3 := d.Syncgroup(ctx, "sg3").Id()
	createSyncgroup(t, ctx, d, sg3, spec, verror.ErrNoExist.ID)
	verifySyncgroups(t, ctx, d, wantGroups, verror.ID(""))

	// Check that create fails if the perms disallow access.
	perms := tu.DefaultPerms(wire.AllDatabaseTags, "root:o:app:client")
	perms.Blacklist("root:o:app:client", string(access.Read))
	if err := d.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}
	spec.Description = "test syncgroup sg4"
	spec.Collections = []wire.Id{wire.Id{"u", "c"}}
	sg4 := d.Syncgroup(ctx, "sg4").Id()
	createSyncgroup(t, ctx, d, sg4, spec, verror.ErrNoAccess.ID)
	verifySyncgroups(t, ctx, d, nil, verror.ErrNoAccess.ID)
}

// Tests that Syncgroup.Join works as expected for the case with one Syncbase
// and 2 clients. One client creates the syncgroup, while the other attempts to
// join it.
func TestJoinSyncgroup(t *testing.T) {
	// Create client1-server pair.
	ctx, ctx1, sName, rootp, cleanup := tu.SetupOrDieCustom("o:app:client1", "server", tu.DefaultPerms(access.AllTypicalTags(), "root:o:app:client1"))
	defer cleanup()

	d1 := tu.CreateDatabase(t, ctx1, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx1, d1, "c")
	specA := wire.SyncgroupSpec{
		Description: "test syncgroup sgA",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:o:app:client1"),
		Collections: []wire.Id{c.Id()},
	}
	sgIdA := wire.Id{Name: "sgA", Blessing: "root:o:app:client1"}
	createSyncgroup(t, ctx1, d1, sgIdA, specA, verror.ID(""))

	// Check that creator can call join successfully.
	joinSyncgroup(t, ctx1, d1, sName, sgIdA, verror.ID(""))

	// Create client2.
	ctx2 := tu.NewCtx(ctx, rootp, "o:app:client2")
	d2 := syncbase.NewService(sName).Database(ctx2, "d", nil)

	// Check that client2's join fails if the perms disallow access.
	joinSyncgroup(t, ctx2, d2, sName, sgIdA, verror.ErrNoAccess.ID)

	verifySyncgroups(t, ctx2, d2, nil, verror.ErrNoAccess.ID)

	// Client1 gives access to client2.
	if err := d1.SetPermissions(ctx1, tu.DefaultPerms(wire.AllDatabaseTags, "root:o:app:client1", "root:o:app:client2"), ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}

	// Verify client2 has access.
	if err := d2.SetPermissions(ctx2, tu.DefaultPerms(wire.AllDatabaseTags, "root:o:app:client1", "root:o:app:client2"), ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}

	// Check that client2's join still fails since the SG ACL disallows access.
	joinSyncgroup(t, ctx2, d2, sName, sgIdA, verror.ErrNoAccess.ID)

	// Create a different syncgroup.
	specB := wire.SyncgroupSpec{
		Description: "test syncgroup sgB",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:o:app:client1", "root:o:app:client2"),
		Collections: []wire.Id{c.Id()},
	}
	sgIdB := wire.Id{Name: "sgB", Blessing: "root:o:app:client1"}
	createSyncgroup(t, ctx1, d1, sgIdB, specB, verror.ID(""))

	// Check that client2's join now succeeds.
	joinSyncgroup(t, ctx2, d2, sName, sgIdB, verror.ID(""))

	// Verify syncgroup state.
	wantGroups := []wire.Id{sgIdA, sgIdB}
	verifySyncgroups(t, ctx1, d1, wantGroups, verror.ID(""))
	verifySyncgroups(t, ctx2, d2, wantGroups, verror.ID(""))
	verifySyncgroupInfo(t, ctx1, d1, sgIdA, specA, 1)
	verifySyncgroupInfo(t, ctx1, d1, sgIdB, specB, 1)
	verifySyncgroupInfo(t, ctx2, d2, sgIdB, specB, 1)
}

// Tests that Syncgroup.SetSpec works as expected.
func TestSetSpecSyncgroup(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(tu.DefaultPerms(access.AllTypicalTags(), "root:o:app:client"))
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	// Create successfully.
	sgId := wire.Id{Name: "sg1", Blessing: "root:o:app:client"}
	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg1",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:o:app:client"),
		Collections: []wire.Id{c.Id()},
	}
	createSyncgroup(t, ctx, d, sgId, spec, verror.ID(""))

	// Verify syncgroup is created.
	wantNames := []wire.Id{sgId}
	verifySyncgroups(t, ctx, d, wantNames, verror.ID(""))
	verifySyncgroupInfo(t, ctx, d, sgId, spec, 1)

	spec.Description = "test syncgroup sg1 update"
	spec.Perms = tu.DefaultPerms(wire.AllSyncgroupTags, "root:o:app:client", "root:o:app:client1")

	sg := d.SyncgroupForId(sgId)
	if err := sg.SetSpec(ctx, spec, ""); err != nil {
		t.Fatalf("sg.SetSpec failed: %v", err)
	}
	verifySyncgroupInfo(t, ctx, d, sgId, spec, 1)
}

///////////////////
// Helpers.

func createSyncgroup(t *testing.T, ctx *context.T, d syncbase.Database, sgId wire.Id, spec wire.SyncgroupSpec, errID verror.ID) syncbase.Syncgroup {
	sg := d.SyncgroupForId(sgId)
	info := wire.SyncgroupMemberInfo{SyncPriority: 8}
	if err := sg.Create(ctx, spec, info); verror.ErrorID(err) != errID {
		tu.Fatalf(t, "Create SG %+v failed: %v", sgId, err)
	}
	return sg
}

func joinSyncgroup(t *testing.T, ctx *context.T, d syncbase.Database, sbName string, sgId wire.Id, wantErr verror.ID) syncbase.Syncgroup {
	sg := d.SyncgroupForId(sgId)
	info := wire.SyncgroupMemberInfo{SyncPriority: 10}
	if _, err := sg.Join(ctx, sbName, nil, info); verror.ErrorID(err) != wantErr {
		tu.Fatalf(t, "Join SG %v failed: %v", sgId, err)
	}
	return sg
}

func verifySyncgroups(t *testing.T, ctx *context.T, d syncbase.Database, wantGroups []wire.Id, wantErr verror.ID) {
	gotGroups, gotErr := d.ListSyncgroups(ctx)
	if verror.ErrorID(gotErr) != wantErr {
		t.Fatalf("d.GetSyncgroup() failed, want %+v, got err %v", wantGroups, gotErr)
	}
	if !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Fatalf("d.GetSyncgroup() failed, got %+v, want %+v", gotGroups, wantGroups)
	}
}

func verifySyncgroupInfo(t *testing.T, ctx *context.T, d syncbase.Database, sgId wire.Id, wantSpec wire.SyncgroupSpec, wantMembers int) {
	sg := d.SyncgroupForId(sgId)
	gotSpec, _, err := sg.GetSpec(ctx)
	if err != nil || !reflect.DeepEqual(gotSpec, wantSpec) {
		t.Fatalf("sg.GetSpec() failed, got %v, want %v, err %v", gotSpec, wantSpec, err)
	}

	members, err := sg.GetMembers(ctx)
	if err != nil || len(members) != wantMembers {
		t.Fatalf("sg.GetMembers() failed, got %v, want %v, err %v", members, wantMembers, err)
	}
}
