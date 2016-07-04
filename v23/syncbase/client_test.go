// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/query/syncql"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/roaming"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// TODO(rogulenko): Test perms checking for Glob and Exec.

// Tests various Id, Name, FullName, and Key methods.
func TestIdNameAndKey(t *testing.T) {
	s := syncbase.NewService("s")
	d := s.DatabaseForId(wire.Id{"a", "d"}, nil)
	c := d.CollectionForId(wire.Id{"u", "c"})
	r := c.Row("r")

	if s.FullName() != "s" {
		t.Errorf("Wrong full name: %q", s.FullName())
	}
	if d.Id() != (wire.Id{"a", "d"}) {
		t.Errorf("Wrong id: %v", d.Id())
	}
	if d.FullName() != naming.Join("s", "a,d") {
		t.Errorf("Wrong full name: %q", d.FullName())
	}
	if c.Id() != (wire.Id{"u", "c"}) {
		t.Errorf("Wrong id: %v", c.Id())
	}
	if c.FullName() != naming.Join("s", "a,d", "u,c") {
		t.Errorf("Wrong full name: %q", c.FullName())
	}
	if r.Key() != "r" {
		t.Errorf("Wrong key: %q", r.Key())
	}
	if r.FullName() != naming.Join("s", "a,d", "u,c", "r") {
		t.Errorf("Wrong full name: %q", r.FullName())
	}
}

// Tests that Service.ListDatabases works as expected.
func TestListDatabases(t *testing.T) {
	_, ctx, sName, rootp, cleanup := tu.SetupOrDieCustom("u:client", "server", tu.DefaultPerms(access.AllTypicalTags(), "root"))
	defer cleanup()
	tu.TestListChildIds(t, ctx, rootp, syncbase.NewService(sName), tu.OkAppUserBlessings, tu.OkDbCxNames)
}

// Tests that Service.{Set,Get}Permissions work as expected.
func TestServicePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestPerms(t, ctx, syncbase.NewService(sName))
}

// Tests that Database.Create works as expected.
func TestDatabaseCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestCreate(t, ctx, syncbase.NewService(sName))
}

// Tests name-checking on database creation.
// TODO(sadovsky): Also test blessing validation. We should rewrite some of
// these tests. Let's do this after we update collections to use ids.
func TestDatabaseCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestCreateNameValidation(t, ctx, syncbase.NewService(sName), tu.OkDbCxNames, tu.NotOkDbCxNames)
}

// Tests that Database.Destroy works as expected.
func TestDatabaseDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestDestroy(t, ctx, syncbase.NewService(sName))
}

// Tests that Database.Exec works as expected.
// Note: The Exec method is tested more thoroughly in exec_test.go.
// Also, the query package is tested in its entirety in v23/query/...
func TestExec(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	foo := Foo{I: 4, S: "f"}
	if err := c.Put(ctx, "foo", foo); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	bar := Bar{F: 0.5, S: "b"}
	// NOTE: not best practice, but store bar as
	// optional (by passing the address of bar to Put).
	// This tests auto-dereferencing.
	if err := c.Put(ctx, "bar", &bar); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	baz := Baz{Name: "John Doe", Active: true}
	if err := c.Put(ctx, "baz", baz); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	tu.CheckExec(t, ctx, d, "select k, v.Name from c where Type(v) like \"%.Baz\"",
		[]string{"k", "v.Name"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz.Name)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where k like \"ba%\"",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where v.Active = true",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where Type(v) like \"%.Bar\"",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where v.F = 0.5",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where Type(v) like \"%.Baz\"",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
		})

	// Delete baz.
	tu.CheckExec(t, ctx, d, "delete from c where Type(v) like \"%.Baz\"",
		[]string{"Count"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf(1)},
		})

	// Check that baz is no longer in the collection.
	tu.CheckExec(t, ctx, d, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	tu.CheckExecError(t, ctx, d, "select k, v from foo", syncql.ErrTableCantAccess.ID)
}

// Tests that Database.ListCollections works as expected.
func TestListCollections(t *testing.T) {
	_, ctx, sName, rootp, cleanup := tu.SetupOrDieCustom("u:client", "server", tu.DefaultPerms(access.AllTypicalTags(), "root"))
	defer cleanup()
	d := syncbase.NewService(sName).DatabaseForId(wire.Id{"root", "d"}, nil)
	if err := d.Create(ctx, tu.DefaultPerms(wire.AllDatabaseTags, "root")); err != nil {
		t.Fatalf("d.Create() failed: %v", err)
	}
	tu.TestListChildIds(t, ctx, rootp, d, tu.OkAppUserBlessings, tu.OkDbCxNames)
}

// Tests that Database.{Set,Get}Permissions work as expected.
func TestDatabasePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestPerms(t, ctx, d)
}

// Tests that Collection.Create works as expected.
func TestCollectionCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestCreate(t, ctx, d)
}

// Tests name-checking on collection creation.
// TODO(sadovsky): Also test blessing validation. We should rewrite some of
// these tests. Let's do this after we update collections to use ids.
func TestCollectionCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestCreateNameValidation(t, ctx, d, tu.OkDbCxNames, tu.NotOkDbCxNames)
}

// Tests that Collection.Destroy works as expected.
func TestCollectionDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestDestroy(t, ctx, d)
}

// Tests that Collection.Destroy deletes all rows in the collection.
func TestCollectionDestroyAndRecreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	// Write some data.
	if err := c.Put(ctx, "bar/baz", "A"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Put(ctx, "foo", "B"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	/*
		// Remove write permissions from collection.
		fullPerms := tu.DefaultPerms(wire.AllCollectionTags, "root:u:client")
		readPerms := fullPerms.Copy().Clear("root:u:client", string(access.Write))
		if err := c.SetPermissions(ctx, readPerms); err != nil {
			t.Fatalf("c.SetPermissions() failed: %v", err)
		}
		// Verify we have no write access to "bar" anymore.
		if err := c.Put(ctx, "bar/bat", "C"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
			t.Fatalf("c.Put() should have failed with ErrNoAccess, got: %v", err)
		}
	*/
	// Destroy collection. Destroy needs only admin permissions on the collection,
	// so it shouldn't be affected by the lack of Write access.
	// TODO(ivanpi): Destroy currently needs Write, reenable code above when fixed.
	if err := c.Destroy(ctx); err != nil {
		t.Fatalf("c.Destroy() failed: %v", err)
	}
	// Recreate the collection.
	if err := c.Create(ctx, nil); err != nil {
		t.Fatalf("c.Create() (recreate) failed: %v", err)
	}
	// Verify collection is empty.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})
	// Verify we again have write access to "bar".
	if err := c.Put(ctx, "bar/bat", "C"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
}

////////////////////////////////////////
// Tests involving rows

type Foo struct {
	I int
	S string
}

type Bar struct {
	F float32
	S string
}

type Baz struct {
	Name   string
	Active bool
}

// Tests that Collection.Scan works as expected.
func TestCollectionScan(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})

	fooWant := Foo{I: 4, S: "f"}
	if err := c.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	barWant := Bar{F: 0.5, S: "b"}
	if err := c.Put(ctx, "bar", &barWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	// Match all keys.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("a", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("a", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Match "bar" only.
	tu.CheckScan(t, ctx, c, syncbase.Prefix("b"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Prefix("bar"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("bar", "baz"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("bar", "foo"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("", "foo"), []string{"bar"}, []interface{}{&barWant})

	// Match "foo" only.
	tu.CheckScan(t, ctx, c, syncbase.Prefix("f"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Prefix("foo"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("foo", "fox"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("foo", ""), []string{"foo"}, []interface{}{&fooWant})

	// Match nothing.
	tu.CheckScan(t, ctx, c, syncbase.Range("a", "bar"), []string{}, []interface{}{})
	tu.CheckScan(t, ctx, c, syncbase.Range("bar", "bar"), []string{}, []interface{}{})
	tu.CheckScan(t, ctx, c, syncbase.Prefix("z"), []string{}, []interface{}{})
}

// Tests that Collection.DeleteRange works as expected.
func TestCollectionDeleteRange(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})

	// Put foo and bar.
	fooWant := Foo{I: 4, S: "f"}
	if err := c.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	barWant := Bar{F: 0.5, S: "b"}
	if err := c.Put(ctx, "bar", &barWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Delete foo.
	if err := c.DeleteRange(ctx, syncbase.Prefix("f")); err != nil {
		t.Fatalf("c.DeleteRange() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar"}, []interface{}{&barWant})

	// Restore foo.
	if err := c.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Delete everything.
	if err := c.DeleteRange(ctx, syncbase.Prefix("")); err != nil {
		t.Fatalf("c.DeleteRange() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})
}

// Tests that Collection.{Get,Put,Delete} work as expected.
func TestCollectionRowMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	got, want := Foo{}, Foo{I: 4, S: "foo"}
	if err := c.Get(ctx, "f", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("c.Get() should have failed: %v", err)
	}
	if err := c.Put(ctx, "f", &want); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Get(ctx, "f", &got); err != nil {
		t.Fatalf("c.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	// Overwrite value.
	want.I = 6
	if err := c.Put(ctx, "f", &want); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Get(ctx, "f", &got); err != nil {
		t.Fatalf("c.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	if err := c.Delete(ctx, "f"); err != nil {
		t.Fatalf("c.Delete() failed: %v", err)
	}
	if err := c.Get(ctx, "f", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("r.Get() should have failed: %v", err)
	}
}

// Tests that Row.{Get,Put,Delete} work as expected.
func TestRowMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	r := c.Row("f")
	got, want := Foo{}, Foo{I: 4, S: "foo"}
	if err := r.Get(ctx, &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("r.Get() should have failed: %v", err)
	}
	if err := r.Put(ctx, &want); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if err := r.Get(ctx, &got); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	// Overwrite value.
	want.I = 6
	if err := r.Put(ctx, &want); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if err := r.Get(ctx, &got); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	if err := r.Delete(ctx); err != nil {
		t.Fatalf("r.Delete() failed: %v", err)
	}
	if err := r.Get(ctx, &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("r.Get() should have failed: %v", err)
	}
}

// Tests name-checking on row creation.
func TestRowKeyValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	tu.TestCreateNameValidation(t, ctx, c, tu.OkRowKeys, tu.NotOkRowKeys)
}

// Test permission checking in Row.{Get,Put,Delete} and
// Collection.{Scan, DeleteRange}.
// TODO(ivanpi): Redundant with permissions_test?
func TestRowPermissions(t *testing.T) {
	_, clientACtx, sName, _, cleanup := tu.SetupOrDieCustom("u:clientA", "server", nil)
	defer cleanup()
	d := tu.CreateDatabase(t, clientACtx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Add some key-value pairs.
	r := c.Row("foo")
	if err := r.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}

	// Lock A out of c.
	bOnly := tu.DefaultPerms(wire.AllCollectionTags, "root:u:clientB")
	if err := c.SetPermissions(clientACtx, bOnly); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}

	// Check A doesn't have access.
	if err := r.Get(clientACtx, &Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Get() should have failed: %v", err)
	}
	if err := r.Put(clientACtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Put() should have failed: %v", err)
	}
	if err := r.Delete(clientACtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Delete() should have failed: %v", err)
	}
	// Test Collection.DeleteRange and Scan.
	if err := c.DeleteRange(clientACtx, syncbase.Prefix("")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.DeleteRange should have failed: %v", err)
	}
	s := c.Scan(clientACtx, syncbase.Prefix(""))
	if s.Advance() {
		t.Fatalf("Stream advanced unexpectedly")
	}
	if err := s.Err(); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Unexpected stream error: %v", err)
	}
}

// Tests collection perms where get is allowed but put is not.
// TODO(ivanpi): Redundant with permissions_test?
func TestMixedCollectionPerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("u:clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "u:clientB")
	d := tu.CreateDatabase(t, clientACtx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Set permissions.
	aAllBRead := tu.DefaultPerms(wire.AllCollectionTags, "root:u:clientA")
	aAllBRead.Add(security.BlessingPattern("root:u:clientB"), string(access.Read))
	if err := c.SetPermissions(clientACtx, aAllBRead); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}

	// Test GetPermissions.
	if got, err := c.GetPermissions(clientACtx); err != nil {
		t.Fatalf("c.GetPermissions() failed: %v", err)
	} else if want := aAllBRead; !reflect.DeepEqual(got, want) {
		t.Fatalf("Unexpected collection permissions: got %v, want %v", got, want)
	}

	// Both A and B can read row key "a", but only A can write it.
	r := c.Row("a")
	if err := r.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("client A r.Put() failed: %v", err)
	}
	if err := r.Put(clientBCtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("client B r.Put() should have failed: %v", err)
	}
	if err := r.Get(clientACtx, &Foo{}); err != nil {
		t.Fatalf("client A r.Get() failed: %v", err)
	}
	if err := r.Get(clientBCtx, &Foo{}); err != nil {
		t.Fatalf("client B r.Get() failed: %v", err)
	}
}

// TestWatchBasic tests the basic client watch functionality: no perms,
// no batches.
func TestWatchBasic(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	var resumeMarkers []watch.ResumeMarker

	// Generate the data and resume markers.
	// Initial state.
	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)
	// Put "abc%".
	r := c.Row("abc%")
	if err := r.Put(ctx, "value"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)
	// Delete "abc%".
	if err := r.Delete(ctx); err != nil {
		t.Fatalf("r.Delete() failed: %v", err)
	}
	if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)
	// Put "a".
	r = c.Row("a")
	if err := r.Put(ctx, "value"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)

	allChanges := []tu.WatchChangeTest{
		tu.WatchChangeTestRowPut(c.Id(), "abc%", "value", resumeMarkers[1]),
		tu.WatchChangeTestRowDelete(c.Id(), "abc%", resumeMarkers[2]),
		tu.WatchChangeTestRowPut(c.Id(), "a", "value", resumeMarkers[3]),
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream := d.Watch(ctxWithTimeout, resumeMarkers[0], []wire.CollectionRowPattern{util.RowPrefixPattern(c.Id(), "a")})
	tu.CheckWatch(t, wstream, allChanges)
	wstream.Cancel()
	wstream = d.Watch(ctxWithTimeout, resumeMarkers[1], []wire.CollectionRowPattern{util.RowPrefixPattern(c.Id(), "a")})
	tu.CheckWatch(t, wstream, allChanges[1:])
	wstream.Cancel()
	wstream = d.Watch(ctxWithTimeout, resumeMarkers[2], []wire.CollectionRowPattern{util.RowPrefixPattern(c.Id(), "a")})
	tu.CheckWatch(t, wstream, allChanges[2:])
	wstream.Cancel()

	wstream = d.Watch(ctxWithTimeout, resumeMarkers[0], []wire.CollectionRowPattern{util.RowPrefixPattern(c.Id(), "abc")})
	tu.CheckWatch(t, wstream, allChanges[:2])
	wstream.Cancel()
	wstream = d.Watch(ctxWithTimeout, resumeMarkers[1], []wire.CollectionRowPattern{util.RowPrefixPattern(c.Id(), "abc")})
	tu.CheckWatch(t, wstream, allChanges[1:2])
	wstream.Cancel()
}

// TestWatchWithBatchAndInitialState tests that the client watch correctly
// handles batches, perms, and fetching initial state on empty resume marker.
func TestWatchWithBatchAndInitialState(t *testing.T) {
	ctx, adminCtx, sName, rootp, cleanup := tu.SetupOrDieCustom("u:admin", "server", nil)
	defer cleanup()
	clientCtx := tu.NewCtx(ctx, rootp, "u:client")
	d := tu.CreateDatabase(t, adminCtx, syncbase.NewService(sName), "d")
	cp := tu.CreateCollection(t, adminCtx, d, "cpublic")
	ch := tu.CreateCollection(t, adminCtx, d, "chidden")

	// Set permissions. Lock client out of ch.
	openAcl := tu.DefaultPerms(wire.AllCollectionTags, "root:u:admin", "root:u:client")
	adminAcl := tu.DefaultPerms(wire.AllCollectionTags, "root:u:admin")
	if err := cp.SetPermissions(adminCtx, openAcl); err != nil {
		t.Fatalf("cp.SetPermissions() failed: %v", err)
	}
	if err := ch.SetPermissions(adminCtx, adminAcl); err != nil {
		t.Fatalf("ch.SetPermissions() failed: %v", err)
	}

	// === 1) Initial changes ===
	// -> Put cp:"a/1" and ch:"b/1" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		cp := b.Collection(adminCtx, "cpublic")
		if err := cp.Put(adminCtx, "a/1", "value"); err != nil {
			return err
		}
		ch := b.Collection(adminCtx, "chidden")
		return ch.Put(adminCtx, "b/1", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	// -> Put cp:"c/1".
	if err := cp.Put(adminCtx, "c/1", "value"); err != nil {
		t.Fatalf("cp.Put() failed: %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(clientCtx, 10*time.Second)
	defer cancel()
	adminCtxWithTimeout, cancelAdmin := context.WithTimeout(adminCtx, 10*time.Second)
	defer cancelAdmin()
	// Start watches with empty resume marker.
	wstreamAll := d.Watch(ctxWithTimeout, nil, []wire.CollectionRowPattern{
		util.RowPrefixPattern(cp.Id(), ""),
		util.RowPrefixPattern(ch.Id(), ""),
	})
	wstreamD := d.Watch(ctxWithTimeout, nil, []wire.CollectionRowPattern{
		util.RowPrefixPattern(cp.Id(), "d"),
	})
	wstreamAllAdmin := d.Watch(adminCtxWithTimeout, nil, []wire.CollectionRowPattern{
		util.RowPrefixPattern(cp.Id(), ""),
		util.RowPrefixPattern(ch.Id(), ""),
	})

	resumeMarkerInitial, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	// Check 1) Initial changes
	// Admin watch with empty prefix should have seen the full initial state as
	// one batch.
	tu.CheckWatch(t, wstreamAllAdmin, []tu.WatchChangeTest{
		tu.WatchChangeTestRootPut(nil),
		tu.WatchChangeTestCollectionPut(ch.Id(), wire.AllCollectionTags, adminAcl, nil),
		tu.WatchChangeTestRowPut(ch.Id(), "b/1", "value", nil),
		tu.WatchChangeTestCollectionPut(cp.Id(), wire.AllCollectionTags, openAcl, nil),
		tu.WatchChangeTestRowPut(cp.Id(), "a/1", "value", nil),
		tu.WatchChangeTestRowPut(cp.Id(), "c/1", "value", resumeMarkerInitial),
	})
	// Watch with empty prefix should have seen the initial state as one batch,
	// omitting the row in the hidden collection and seeing no perms on it.
	tu.CheckWatch(t, wstreamAll, []tu.WatchChangeTest{
		tu.WatchChangeTestRootPut(nil),
		tu.WatchChangeTestCollectionPut(ch.Id(), nil, nil, nil),
		// Skip row "b/1" in hidden collection.
		tu.WatchChangeTestCollectionPut(cp.Id(), wire.AllCollectionTags, openAcl, nil),
		tu.WatchChangeTestRowPut(cp.Id(), "a/1", "value", nil),
		tu.WatchChangeTestRowPut(cp.Id(), "c/1", "value", resumeMarkerInitial),
	})
	// Watch with prefix "d" should have seen only the initial root update and
	// public collection change.
	tu.CheckWatch(t, wstreamD, []tu.WatchChangeTest{
		tu.WatchChangeTestRootPut(nil),
		tu.WatchChangeTestCollectionPut(cp.Id(), wire.AllCollectionTags, openAcl, resumeMarkerInitial),
	})

	// === 2) More writes ===
	// -> Put cp:"a/2" and ch:"b/2" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		cp := b.Collection(adminCtx, "cpublic")
		if err := cp.Put(adminCtx, "a/2", "value"); err != nil {
			return err
		}
		ch := b.Collection(adminCtx, "chidden")
		return ch.Put(adminCtx, "b/2", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterB2A2, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	// -> Put cp:"d/1" and cp:"a/1" (overwrite) in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		cp := b.Collection(adminCtx, "cpublic")
		if err := cp.Put(adminCtx, "d/1", "value"); err != nil {
			return err
		}
		return cp.Put(adminCtx, "a/1", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterA1rD1, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	// Check 2) More writes
	// Admin watch with empty prefix should have seen all the continued changes as
	// separate batches.
	tu.CheckWatch(t, wstreamAllAdmin, []tu.WatchChangeTest{
		tu.WatchChangeTestRowPut(cp.Id(), "a/2", "value", nil),
		tu.WatchChangeTestRowPut(ch.Id(), "b/2", "value", resumeMarkerAfterB2A2),
		tu.WatchChangeTestRowPut(cp.Id(), "d/1", "value", nil),
		tu.WatchChangeTestRowPut(cp.Id(), "a/1", "value", resumeMarkerAfterA1rD1),
	})
	// Watch with empty prefix should have seen the continued changes as separate
	// batches, omitting rows in the hidden collection. Batches should be properly
	// terminated.
	tu.CheckWatch(t, wstreamAll, []tu.WatchChangeTest{
		tu.WatchChangeTestRowPut(cp.Id(), "a/2", "value", resumeMarkerAfterB2A2),
		tu.WatchChangeTestRowPut(cp.Id(), "d/1", "value", nil),
		tu.WatchChangeTestRowPut(cp.Id(), "a/1", "value", resumeMarkerAfterA1rD1),
	})
	// Watch with prefix "d" should have seen only the "d/1" change. Batch should
	// be properly terminated.
	tu.CheckWatch(t, wstreamD, []tu.WatchChangeTest{
		tu.WatchChangeTestRowPut(cp.Id(), "d/1", "value", resumeMarkerAfterA1rD1),
	})

	// === 3) Writes and permissions change ===
	// -> Put ch:"b/3", set permissions on ch, put ch:"b/4" in a batch.
	newPerms := access.Permissions{}.
		Add("root:u:admin", access.TagStrings(access.Write, access.Admin)...).
		Add("root:u:client", access.TagStrings(access.Read, access.Admin)...)
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		ch := b.Collection(adminCtx, "chidden")
		if err := ch.Put(adminCtx, "b/3", "value"); err != nil {
			return err
		}
		if err := ch.SetPermissions(adminCtx, newPerms); err != nil {
			return err
		}
		return ch.Put(adminCtx, "b/4", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterB3BspB4, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	// Check 3) Writes and permissions change
	// Admin batch no longer has Read access on the hidden collection, so it sees
	// only the ACL update. The ACL is checked when scanning the log after commit,
	// so the new ACL applies to the entire batch.
	tu.CheckWatch(t, wstreamAllAdmin, []tu.WatchChangeTest{
		tu.WatchChangeTestCollectionPut(ch.Id(), []access.Tag{access.Write, access.Admin}, newPerms, resumeMarkerAfterB3BspB4),
	})
	// Watch with empty prefix should have seen the continued changes since it has
	// Read access on the hidden collection now. The ACL is checked when scanning
	// the log after commit, so the new ACL applies to the entire batch.
	tu.CheckWatch(t, wstreamAll, []tu.WatchChangeTest{
		tu.WatchChangeTestRowPut(ch.Id(), "b/3", "value", nil),
		tu.WatchChangeTestCollectionPut(ch.Id(), []access.Tag{access.Read, access.Admin}, newPerms, nil),
		tu.WatchChangeTestRowPut(ch.Id(), "b/4", "value", resumeMarkerAfterB3BspB4),
	})

	// === 4) Writes and collection destroy ===
	// -> Put ch:"b/5", destroy ch, put cp:"a/3" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		ch := b.Collection(adminCtx, "chidden")
		if err := ch.Put(adminCtx, "b/5", "value"); err != nil {
			return err
		}
		if err := ch.Destroy(adminCtx); err != nil {
			return err
		}
		cp := b.Collection(adminCtx, "cpublic")
		return cp.Put(adminCtx, "a/3", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterB5BdA3, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	// -> Delete cp:"a/3".
	if err := cp.Delete(adminCtx, "a/3"); err != nil {
		t.Fatalf("cp.Delete() failed: %v", err)
	}
	resumeMarkerAfterA3d, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	// Check 4) Writes and collection destroy
	// Both watches do not see any of the hidden collection rows since the
	// collection has been destroyed and there is no ACL to check against. They
	// see only the public collection rows and the hidden collection destroy.
	finalChangesBoth := []tu.WatchChangeTest{
		tu.WatchChangeTestCollectionDelete(ch.Id(), nil),
		tu.WatchChangeTestRowPut(cp.Id(), "a/3", "value", resumeMarkerAfterB5BdA3),
		tu.WatchChangeTestRowDelete(cp.Id(), "a/3", resumeMarkerAfterA3d),
	}
	tu.CheckWatch(t, wstreamAllAdmin, finalChangesBoth)
	tu.CheckWatch(t, wstreamAll, finalChangesBoth)

	wstreamAllAdmin.Cancel()
	wstreamD.Cancel()
	wstreamAll.Cancel()
}

// TestBlockingWatch tests that the server side of the client watch correctly
// blocks until new updates to the database arrive.
func TestBlockingWatch(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream := d.Watch(ctxWithTimeout, resumeMarker, []wire.CollectionRowPattern{
		util.RowPrefixPattern(c.Id(), "a"),
	})
	for i := 0; i < 10; i++ {
		// Put "abc".
		r := c.Row("abc")
		if err := r.Put(ctx, "value"); err != nil {
			t.Fatalf("r.Put() failed: %v", err)
		}
		if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
			t.Fatalf("d.GetResumeMarker() failed: %v", err)
		}
		if !wstream.Advance() {
			t.Fatalf("wstream.Advance() reached the end: %v", wstream.Err())
		}
		want := tu.WatchChangeTestRowPut(c.Id(), "abc", "value", resumeMarker)
		if got := wstream.Change(); !tu.WatchChangeEq(&got, &want) {
			t.Fatalf("Unexpected watch change: got %v, want %v", got, want)
		}
	}
	wstream.Cancel()
}

// TestBlockedWatchCancel tests that the watch call blocked on the server side
// can be successfully canceled from the client.
func TestBlockedWatchCancel(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")

	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream := d.Watch(ctxWithTimeout, resumeMarker, []wire.CollectionRowPattern{
		util.RowPrefixPattern(wire.Id{"root:o:app:client", "c"}, "a"),
	})
	if err := wstream.Err(); err != nil {
		t.Fatalf("d.Watch() failed: %v", err)
	}
	time.AfterFunc(500*time.Millisecond, wstream.Cancel)
	if wstream.Advance() {
		t.Fatal("wstream should not have advanced")
	}
	if got, want := verror.ErrorID(wstream.Err()), verror.ErrCanceled.ID; got != want {
		t.Errorf("Unexpected wstream error ID: got %v, want %v", got, want)
	}
}

// TestWatchMulti tests that watch properly filters collections and includes
// matching rows only once per update.
func TestWatchMulti(t *testing.T) {
	_, ctx, sName, rootp, cleanup := tu.SetupOrDieCustom("x", "server", nil)
	defer cleanup()
	d := syncbase.NewService(sName).DatabaseForId(wire.Id{"root:x", "d"}, nil)
	if err := d.Create(ctx, tu.DefaultPerms(wire.AllDatabaseTags, "root:x")); err != nil {
		t.Fatalf("d.Create() failed: %v", err)
	}
	// Create three collections.
	cxPerms := tu.DefaultPerms(wire.AllCollectionTags, "root:x")
	cAFoo := d.CollectionForId(wire.Id{"root:x:alice", "foo"})
	if err := cAFoo.Create(tu.NewCtx(ctx, rootp, "x:alice"), cxPerms); err != nil {
		t.Fatalf("cAFoo.Create() failed: %v", err)
	}
	cAFoobar := d.CollectionForId(wire.Id{"root:x:alice", "foobar"})
	if err := cAFoobar.Create(tu.NewCtx(ctx, rootp, "x:alice"), cxPerms); err != nil {
		t.Fatalf("cAFoobar.Create() failed: %v", err)
	}
	cBFoo := d.CollectionForId(wire.Id{"root:x:%", "foo"})
	if err := cBFoo.Create(tu.NewCtx(ctx, rootp, "x:%"), cxPerms); err != nil {
		t.Fatalf("cBFoo.Create() failed: %v", err)
	}

	// Prepopulate data.
	for _, key := range []string{"a", "abc", "abcd", "cd", "ef", "zcd_"} {
		if err := cAFoo.Put(ctx, key, "value"); err != nil {
			t.Fatalf("cAFoo.Put() failed: %v", err)
		}
	}
	for _, key := range []string{"a", "abc", "cd", "ef%", "xv"} {
		if err := cAFoobar.Put(ctx, key, "value"); err != nil {
			t.Fatalf("cAFoobar.Put() failed: %v", err)
		}
	}
	for _, key := range []string{"ab", "ef", "efg", "pq", "x\\yz"} {
		if err := cBFoo.Put(ctx, key, "value"); err != nil {
			t.Fatalf("cBFoo.Put() failed: %v", err)
		}
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Start watches with empty resume marker.
	// non-overlapping - collections 'foo' and '%bar'
	wstream1 := d.Watch(ctxWithTimeout, nil, []wire.CollectionRowPattern{
		{"%:alice", "foo", "%c_"},
		{"%", "%bar", "a%"},
	})
	// prefix pattern - only literal 'root:\%'
	wstream2 := d.Watch(ctxWithTimeout, nil, []wire.CollectionRowPattern{
		util.RowPrefixPattern(wire.Id{"root:x:%", "foo"}, "e"),
	})
	// partially overlapping - '%:alice' and 'root:x:%', '%cd' and 'ab%'
	wstream3 := d.Watch(ctxWithTimeout, nil, []wire.CollectionRowPattern{
		{"%:alice", "%", "%cd"},
		{"root:x:%", "%", "ab%"},
		{"root:x:\\%", "%", "x%"},
	})

	resumeMarkerInitial, err := d.GetResumeMarker(ctxWithTimeout)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	tu.CheckWatch(t, wstream1, []tu.WatchChangeTest{
		tu.WatchChangeTestRootPut(nil),
		tu.WatchChangeTestCollectionPut(cAFoobar.Id(), wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cAFoobar.Id(), "a", "value", nil),
		tu.WatchChangeTestRowPut(cAFoobar.Id(), "abc", "value", nil),
		tu.WatchChangeTestCollectionPut(cAFoo.Id(), wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cAFoo.Id(), "abcd", "value", nil),
		tu.WatchChangeTestRowPut(cAFoo.Id(), "cd", "value", resumeMarkerInitial),
	})
	tu.CheckWatch(t, wstream2, []tu.WatchChangeTest{
		tu.WatchChangeTestRootPut(nil),
		tu.WatchChangeTestCollectionPut(cBFoo.Id(), wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cBFoo.Id(), "ef", "value", nil),
		tu.WatchChangeTestRowPut(cBFoo.Id(), "efg", "value", resumeMarkerInitial),
	})
	tu.CheckWatch(t, wstream3, []tu.WatchChangeTest{
		tu.WatchChangeTestRootPut(nil),
		tu.WatchChangeTestCollectionPut(cBFoo.Id(), wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cBFoo.Id(), "ab", "value", nil),
		tu.WatchChangeTestRowPut(cBFoo.Id(), "x\\yz", "value", nil),
		tu.WatchChangeTestCollectionPut(cAFoobar.Id(), wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cAFoobar.Id(), "abc", "value", nil),
		tu.WatchChangeTestRowPut(cAFoobar.Id(), "cd", "value", nil),
		tu.WatchChangeTestCollectionPut(cAFoo.Id(), wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cAFoo.Id(), "abc", "value", nil),
		tu.WatchChangeTestRowPut(cAFoo.Id(), "abcd", "value", nil),
		tu.WatchChangeTestRowPut(cAFoo.Id(), "cd", "value", resumeMarkerInitial),
	})

	// More writes.
	// Put root:x:alice,foo:"abcd" and root:x:alice,foobar:"abcd", delete root:x:%,foo:"ef%" in a batch.
	if err := syncbase.RunInBatch(ctxWithTimeout, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		if err := b.CollectionForId(wire.Id{"root:x:alice", "foo"}).Put(ctxWithTimeout, "abcd", "value"); err != nil {
			return err
		}
		if err := b.CollectionForId(wire.Id{"root:x:alice", "foobar"}).Put(ctxWithTimeout, "abcd", "value"); err != nil {
			return err
		}
		return b.CollectionForId(wire.Id{"root:x:%", "foo"}).Delete(ctxWithTimeout, "ef%")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterBatch1, err := d.GetResumeMarker(ctxWithTimeout)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	// Create collection root:x:bob,foobar and put "xyz", "acd", "abcd", root:x:alice,foo:"bcd" in a batch.
	ctxBob := tu.NewCtx(ctxWithTimeout, rootp, "x:bob")
	cNewId := wire.Id{"root:x:bob", "foobar"}
	if err := syncbase.RunInBatch(ctxBob, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		cNew := b.CollectionForId(cNewId)
		if err := cNew.Create(ctxBob, cxPerms); err != nil {
			return err
		}
		if err := cNew.Put(ctxBob, "xyz", "value"); err != nil {
			return err
		}
		if err := cNew.Put(ctxBob, "acd", "value"); err != nil {
			return err
		}
		if err := cNew.Put(ctxBob, "abcd", "value"); err != nil {
			return err
		}
		return b.CollectionForId(wire.Id{"root:x:alice", "foo"}).Put(ctxBob, "bcd", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterBatch2, err := d.GetResumeMarker(ctxWithTimeout)
	_ = resumeMarkerAfterBatch2
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	tu.CheckWatch(t, wstream1, []tu.WatchChangeTest{
		tu.WatchChangeTestRowPut(cAFoo.Id(), "abcd", "value", nil),
		tu.WatchChangeTestRowPut(cAFoobar.Id(), "abcd", "value", resumeMarkerAfterBatch1),
		tu.WatchChangeTestCollectionPut(cNewId, wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cNewId, "acd", "value", nil),
		tu.WatchChangeTestRowPut(cNewId, "abcd", "value", nil),
		tu.WatchChangeTestRowPut(cAFoo.Id(), "bcd", "value", resumeMarkerAfterBatch2),
	})
	tu.CheckWatch(t, wstream2, []tu.WatchChangeTest{
		tu.WatchChangeTestRowDelete(cBFoo.Id(), "ef%", resumeMarkerAfterBatch1),
	})
	tu.CheckWatch(t, wstream3, []tu.WatchChangeTest{
		tu.WatchChangeTestRowPut(cAFoo.Id(), "abcd", "value", nil),
		tu.WatchChangeTestRowPut(cAFoobar.Id(), "abcd", "value", resumeMarkerAfterBatch1),
		tu.WatchChangeTestCollectionPut(cNewId, wire.AllCollectionTags, cxPerms, nil),
		tu.WatchChangeTestRowPut(cNewId, "abcd", "value", nil),
		tu.WatchChangeTestRowPut(cAFoo.Id(), "bcd", "value", resumeMarkerAfterBatch2),
	})

	wstream1.Cancel()
	wstream2.Cancel()
	wstream3.Cancel()
}
