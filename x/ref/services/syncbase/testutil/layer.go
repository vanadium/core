// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/test/testutil"
)

// TODO(sadovsky): This testutil library is starting to create more problems
// than it solves, now that our layers are less homogeneous in behavior. Perhaps
// eliminate this library entirely, and split up our unit tests into separate
// test implementations per layer of hierarchy.

// TestCreate tests that object creation works as expected.
func TestCreate(t *testing.T, ctx *context.T, i interface{}) {
	parent := makeLayer(i)
	self := parent.Child("self")
	child := self.Child("child")

	// child.Create should fail since self does not exist.
	if err := child.Create(ctx, nil); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("child.Create() should have failed: %v", err)
	}

	assertExists(t, ctx, self, "self", false)
	// TODO(ivanpi): Exists on child when parent does not exist currently fails
	// with an error instead of returning false.
	// assertExists(t, ctx, child, "child", false)

	// Create self.
	if err := self.Create(ctx, nil); err != nil {
		t.Fatalf("self.Create() failed: %v", err)
	}
	if gotPerms, wantPerms := getPermsOrDie(t, ctx, self), DefaultPerms(self.AllowedTags(), "root:o:app:client"); !reflect.DeepEqual(gotPerms, wantPerms) {
		t.Errorf("Perms do not match: got %v, want %v", gotPerms, wantPerms)
	}

	assertExists(t, ctx, self, "self", true)
	assertExists(t, ctx, child, "child", false)

	// child.Create should now succeed.
	if err := child.Create(ctx, nil); err != nil {
		t.Fatalf("child.Create() failed: %v", err)
	}

	assertExists(t, ctx, child, "child", true)

	// self.Create should fail since self already exists.
	if err := self.Create(ctx, nil); verror.ErrorID(err) != verror.ErrExist.ID {
		t.Fatalf("self.Create() should have failed: %v", err)
	}

	assertExists(t, ctx, self, "self", true)

	// Test create with non-default perms.
	self2 := parent.Child("self2")
	perms := access.Permissions{}
	perms.Add(security.BlessingPattern("root:o:app:client"), string(access.Admin))
	if err := self2.Create(ctx, perms); err != nil {
		t.Fatalf("self2.Create() failed: %v", err)
	}
	if gotPerms, wantPerms := getPermsOrDie(t, ctx, self2), perms; !reflect.DeepEqual(gotPerms, wantPerms) {
		t.Errorf("Perms do not match: got %v, want %v", gotPerms, wantPerms)
	}

	// Even though self2 exists, Exists returns false because Read access is
	// needed.
	assertExists(t, ctx, self2, "self2", false)

	// Test that create fails if the parent perms disallow access.
	perms = DefaultPerms(parent.AllowedTags(), "root:o:app:client")
	perms.Blacklist("root:o:app:client", string(access.Write))
	if err := parent.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("parent.SetPermissions() failed: %v", err)
	}
	self3 := parent.Child("self3")
	if err := self3.Create(ctx, nil); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("self3.Create() should have failed: %v", err)
	}

	assertExists(t, ctx, self, "self", true)
	assertExists(t, ctx, self3, "self3", false)
}

// Tests that Create() checks name validity.
func TestCreateNameValidation(t *testing.T, ctx *context.T, i interface{}, okNames, notOkNames []string) {
	parent := makeLayer(i)
	for _, name := range okNames {
		if err := parent.Child(name).Create(ctx, nil); err != nil {
			t.Fatalf("Create(%q) failed: %v", name, err)
		}
	}
	for _, name := range notOkNames {
		if err := parent.Child(name).Create(ctx, nil); err == nil {
			t.Fatalf("Create(%q) should have failed: %v", name, err)
		} else if name != "" && verror.ErrorID(err) != wire.ErrInvalidName.ID {
			// TODO(sadovsky): Currently for name "" we cannot check for
			// ErrInvalidName, since parent.Child("") dispatches on the parent
			// component. Create will still fail, but for a different reason. If/when
			// we add client-side name validation, we'll be able to check for
			// ErrInvalidName specifically.
			t.Fatalf("Create(%q) should have failed with ErrInvalidName: %v", name, err)
		}
	}
}

// TestDestroy tests that object destruction works as expected.
func TestDestroy(t *testing.T, ctx *context.T, i interface{}) {
	parent := makeLayer(i)
	self := parent.Child("self")
	child := self.Child("child")

	// Create self.
	if err := self.Create(ctx, nil); err != nil {
		t.Fatalf("self.Create() failed: %v", err)
	}

	assertExists(t, ctx, self, "self", true)

	// self.Create should fail, since self already exists.
	if err := self.Create(ctx, nil); verror.ErrorID(err) != verror.ErrExist.ID {
		t.Fatalf("self.Create() should have failed: %v", err)
	}

	assertExists(t, ctx, self, "self", true)

	// By default, perms allow the creator, so self.Destroy should succeed.
	if err := self.Destroy(ctx); err != nil {
		t.Fatalf("self.Destroy() failed: %v", err)
	}

	assertExists(t, ctx, self, "self", false)

	// child.Create should fail, since self does not exist.
	if err := child.Create(ctx, nil); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("child.Create() should have failed: %v", err)
	}

	assertExists(t, ctx, self, "self", false)
	// TODO(ivanpi): Exists on child when parent does not exist currently fails
	// with an error instead of returning false.
	// assertExists(t, ctx, child, "child", false)

	// self.Create should succeed, since self was destroyed.
	if err := self.Create(ctx, nil); err != nil {
		t.Fatalf("self.Create() failed: %v", err)
	}

	assertExists(t, ctx, self, "self", true)
	assertExists(t, ctx, child, "child", false)

	// Test that destroy fails if the perms disallow access.
	self2 := parent.Child("self2")
	if err := self2.Create(ctx, nil); err != nil {
		t.Fatalf("self2.Create() failed: %v", err)
	}
	perms := DefaultPerms(self2.AllowedTags(), "root:o:app:client")
	perms.Clear("root:o:app:client", string(access.Write), string(access.Admin))
	perms.Add("nobody", string(access.Admin))
	if err := self2.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("self2.SetPermissions() failed: %v", err)
	}
	if err := self2.Destroy(ctx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("self2.Destroy() should have failed: %v", err)
	}

	assertExists(t, ctx, self2, "self2", true)

	// Create child.
	if err := child.Create(ctx, nil); err != nil {
		t.Fatalf("child.Create() failed: %v", err)
	}

	assertExists(t, ctx, self, "self", true)
	assertExists(t, ctx, child, "child", true)

	// Test that destroy succeeds even if the parent and child perms disallow
	// access.
	perms = DefaultPerms(parent.AllowedTags(), "root:o:app:client")
	perms.Clear("root:o:app:client", string(access.Write), string(access.Admin))
	perms.Add("nobody", string(access.Admin))
	if err := parent.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("parent.SetPermissions() failed: %v", err)
	}
	perms = DefaultPerms(child.AllowedTags(), "root:o:app:client")
	if err := child.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("child.SetPermissions() failed: %v", err)
	}
	if err := self.Destroy(ctx); err != nil {
		t.Fatalf("self.Destroy() failed: %v", err)
	}

	assertExists(t, ctx, self, "self", false)
	// TODO(ivanpi): Reenable when Exists() is fixed to treat nonexistent parent
	// same as nonexistent self.
	//assertExists(t, ctx, child, "child", false)

	// Test that destroy is idempotent.
	if err := self.Destroy(ctx); err != nil {
		t.Fatalf("self.Destroy() failed: %v", err)
	}

	assertExists(t, ctx, self, "self", false)
}

func copyAndSortStrings(strs []string) []string {
	sortedStrs := make([]string, len(strs))
	copy(sortedStrs, strs)
	sort.Strings(sortedStrs)
	return sortedStrs
}

func TestListChildIds(t *testing.T, ctx *context.T, rootp security.Principal, i interface{}, blessings, names []string) {
	self := makeLayer(i)

	var got, want []wire.Id
	var err error

	ids := []wire.Id{}
	for _, blessing := range copyAndSortStrings(blessings) {
		blessing = "root" + security.ChainSeparator + blessing
		if len(blessing) > 1024 {
			// Since we prepend "root:" to the blessing to pass implicit perms in
			// Create, some otherwise valid blessings can exceed the 1024 byte limit
			// for an Id blessing when the prefix is added, so we skip them.
			continue
		}
		for _, name := range copyAndSortStrings(names) {
			ids = append(ids, wire.Id{blessing, name})
		}
	}

	for i := 0; i <= len(ids); i++ {
		got, err = self.ListChildIds(ctx)
		want = ids[:i]
		if err != nil {
			t.Fatalf("self.ListChildIds() failed: %v", err)
		}
		if got == nil {
			got = []wire.Id{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Lists do not match: got %v, want %v", got, want)
		}
		if i == len(ids) {
			break
		}
		id := ids[i]
		creatorCtx := NewCtx(ctx, rootp, strings.TrimPrefix(id.Blessing, "root"+security.ChainSeparator))
		if err := self.ChildForId(id).Create(creatorCtx, access.Permissions{}.Add("nobody", string(access.Admin))); err != nil {
			t.Fatalf("Create(%v) failed: %v", id, err)
		}
	}
}

// TestPerms tests that {Set,Get}Permissions work as expected.
// TODO(sadovsky): All Vanadium {Set,Get}Permissions tests ought to share this
// test implementation. :)
// Mirrors v.io/groups/x/ref/services/groups/internal/server/server_test.go.
func TestPerms(t *testing.T, ctx *context.T, ac util.AccessController) {
	myperms := access.Permissions{}
	myperms.Add(security.BlessingPattern("root:o:app:client"), string(access.Admin))
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
	if err := ac.SetPermissions(ctx, myperms, "20"); verror.ErrorID(err) != verror.ErrBadVersion.ID {
		t.Fatal("SetPermissions should have failed with version error")
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
	permsBefore, versionBefore = permsAfter, versionAfter
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
	permsBefore, versionBefore = permsAfter, versionAfter
	myperms.Add(security.BlessingPattern("root:o:app:client"), string(access.Read))
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
	if err := ac.SetPermissions(ctx, access.Permissions{}.Add("nobody", string(access.Admin)), ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	if _, _, err := ac.GetPermissions(ctx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("GetPermissions should have failed with access error. Instead got error: %v", err)
	}
	if err := ac.SetPermissions(ctx, myperms, ""); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("SetPermissions should have failed with access error. Instead got error: %v", err)
	}
}

////////////////////////////////////////
// Internal helpers

const notAvailable = "not available"

type layer interface {
	util.AccessController
	AllowedTags() []access.Tag
	FullName() string
	Create(ctx *context.T, perms access.Permissions) error
	Destroy(ctx *context.T) error
	Exists(ctx *context.T) (bool, error)
	ListChildIds(ctx *context.T) ([]wire.Id, error)
	Child(childName string) layer
	ChildForId(childId wire.Id) layer
}

type service struct {
	syncbase.Service
}

func (_ *service) AllowedTags() []access.Tag {
	return access.AllTypicalTags()
}
func (s *service) Create(ctx *context.T, perms access.Permissions) error {
	panic(notAvailable)
}
func (s *service) Destroy(ctx *context.T) error {
	panic(notAvailable)
}
func (s *service) Exists(ctx *context.T) (bool, error) {
	panic(notAvailable)
}
func (s *service) ListChildIds(ctx *context.T) ([]wire.Id, error) {
	return s.ListDatabases(ctx)
}
func (s *service) Child(childName string) layer {
	return makeLayer(s.DatabaseForId(wire.Id{"root:o:app", childName}, nil))
}
func (s *service) ChildForId(childId wire.Id) layer {
	return makeLayer(s.DatabaseForId(childId, nil))
}

type database struct {
	syncbase.Database
}

func (_ *database) AllowedTags() []access.Tag {
	return wire.AllDatabaseTags
}
func (d *database) ListChildIds(ctx *context.T) ([]wire.Id, error) {
	return d.ListCollections(ctx)
}
func (d *database) Child(childName string) layer {
	return makeLayer(d.CollectionForId(wire.Id{"root:o:app:client", childName}))
}
func (d *database) ChildForId(childId wire.Id) layer {
	return makeLayer(d.CollectionForId(childId))
}

type collection struct {
	syncbase.Collection
}

func (_ *collection) AllowedTags() []access.Tag {
	return wire.AllCollectionTags
}
func (c *collection) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return c.Collection.SetPermissions(ctx, perms)
}
func (c *collection) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	perms, err = c.Collection.GetPermissions(ctx)
	return perms, "", err
}
func (c *collection) ListChildIds(ctx *context.T) ([]wire.Id, error) {
	panic(notAvailable)
}
func (c *collection) Child(childName string) layer {
	return &row{Row: c.Row(childName), c: c.Collection}
}
func (c *collection) ChildForId(childId wire.Id) layer {
	panic(notAvailable)
}

type row struct {
	syncbase.Row
	c syncbase.Collection
}

func (_ *row) AllowedTags() []access.Tag {
	return nil
}
func (r *row) Create(ctx *context.T, perms access.Permissions) error {
	if perms != nil {
		panic(fmt.Sprintf("bad perms: %v", perms))
	}
	return r.Put(ctx, true)
}
func (r *row) Destroy(ctx *context.T) error {
	return r.Delete(ctx)
}
func (r *row) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	// no-op (cannot panic since it is called from collection layer tests)
	return nil
}
func (r *row) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	panic(notAvailable)
}
func (r *row) ListChildIds(ctx *context.T) ([]wire.Id, error) {
	panic(notAvailable)
}
func (r *row) Child(childName string) layer {
	panic(notAvailable)
}
func (r *row) ChildForId(childId wire.Id) layer {
	panic(notAvailable)
}

func makeLayer(i interface{}) layer {
	switch t := i.(type) {
	case syncbase.Service:
		return &service{t}
	case syncbase.Database:
		return &database{t}
	case syncbase.Collection:
		return &collection{t}
	default:
		vlog.Fatalf("unexpected type: %T", t)
	}
	return nil
}

func assertExists(t *testing.T, ctx *context.T, l layer, name string, want bool) {
	if got, err := l.Exists(ctx); err != nil {
		t.Fatal(testutil.FormatLogLine(2, "%s.Exists() failed: %v", name, err))
	} else if got != want {
		t.Error(testutil.FormatLogLine(2, "%s.Exists() got %v, want %v", name, got, want))
	}
}
