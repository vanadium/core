// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(sadovsky): Once the storage engine layer is made public, implement a
// Syncbase-based storage engine so that we can run all the storage engine tests
// against Syncbase itself.

package syncbase_test

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/query/syncql"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/roaming"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// Tests various Name and FullName methods.
func TestName(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")

	b, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	if d.Id() != (wire.Id{"root:o:app", "d"}) {
		t.Errorf("Wrong id: %q", d.Id())
	}
	if d.FullName() != naming.Join(sName, util.EncodeId(d.Id())) {
		t.Errorf("Wrong full name: %q", d.FullName())
	}
	if b.Id() != d.Id() {
		t.Errorf("Ids should match: %q, %q", b.Id(), d.Id())
	}
	if b.FullName() != d.FullName() {
		t.Errorf("Full names should match: %q", b.FullName())
	}
}

// Test that a batch cannot add a permission, make a change, and remove a permission
// since that change isn't able to be validated by remote syncbases.
func TestPermsChangeInBatch(test *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	service := syncbase.NewService(serverName)
	db := tu.CreateDatabase(test, ctx, service, "d")

	// Create the collection outside of batch, ensuring that the initial perms
	// do not have write permission. (Had the collection been created inside the
	// batch, initial perms in batch verification would have been the implicit
	// perms, which allow the creator to write, so would not trigger the failure.
	// Creating before the batch results in initial batch perms being ones passed
	// in to collection Create.)
	// TODO(ivanpi): Test that the other case doesn't fail.
	perms := tu.DefaultPerms(wire.AllCollectionTags, "root:o:app:client")
	perms.Clear("root:o:app:client", "Write")
	if err := db.Collection(ctx, "newname").Create(ctx, perms); err != nil {
		test.Fatalf("db.Collection().Create() failed, %v", err)
	}

	batch, err := db.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		test.Fatalf("db.BeginBatch() failed: %v", err)
	}
	batchCollection := batch.Collection(ctx, "newname")

	// Add the Write permission.
	perms, err = batchCollection.GetPermissions(ctx)
	if err != nil {
		test.Fatalf("bc.GetPermissions() failed: %v", err)
	}
	perms.Add("root:o:app:client", "Write")
	if err := batchCollection.SetPermissions(ctx, perms); err != nil {
		test.Fatalf("bc.SetPermissions() failed: %v", err)
	}

	// Attempt a Put.
	if err := batchCollection.Put(ctx, "fooKey", "fooValue"); err != nil {
		test.Fatalf("bc.Put() failed: %v", err)
	}

	// Remove the Write permission.
	perms, err = batchCollection.GetPermissions(ctx)
	perms.Clear("root:o:app:client", "Write")
	if err := batchCollection.SetPermissions(ctx, perms); err != nil {
		test.Fatalf("bc.SetPermissions() failed: %v", err)
	}

	// Commit the batch.
	if err := batch.Commit(ctx); verror.ErrorID(err) != wire.ErrInvalidPermissionsChange.ID {
		test.Fatalf("b.Commit() should have failed with ErrInvalidPermissionsChange, got: %v", err)
	}

	row := db.Collection(ctx, "newname").Row("fooKey")
	if exists, err := row.Exists(ctx); err != nil {
		test.Fatalf("r.Exists() failed: %v", err)
	} else if exists {
		test.Fatalf("the row should not exist since the commit failed")
	}
}

// Tests basic functionality of BatchDatabase.
func TestBatchBasics(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})

	var b1, b2 syncbase.BatchDatabase
	var b1c, b2c syncbase.Collection
	var err error

	// Test that the effects of a transaction are not visible until commit.
	b1, err = d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1c = b1.Collection(ctx, "c")

	if err := b1c.Put(ctx, "fooKey", "fooValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Check that foo is visible inside of this transaction.
	tu.CheckScan(t, ctx, b1c, syncbase.Prefix(""), []string{"fooKey"}, []interface{}{"fooValue"})

	// Check that foo is not yet visible outside of this transaction.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})

	// Start a scan in b1, advance the scan one row, put a new value that would
	// occur later in the scan (if it were visible) and then advance the scan to see
	// that it doesn't show (since we snapshot uncommiteed changes at the start).
	// Ditto for Exec.
	// start the scan and exec
	scanIt := b1c.Scan(ctx, syncbase.Prefix(""))
	if !scanIt.Advance() {
		t.Fatal("scanIt.Advance() returned false")
	}
	_, execIt, err := b1.Exec(ctx, "select k from c")
	if err != nil {
		t.Fatalf("b1.Exec() failed: %v", err)
	}
	if !execIt.Advance() {
		t.Fatal("execIt.Advance() returned false")
	}
	// put "zzzKey"
	if err := b1c.Put(ctx, "zzzKey", "zzzValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	// make sure Scan's Advance doesn't return a "zzzKey"
	for scanIt.Advance() {
		if string(scanIt.Key()) == "zzzKey" {
			t.Fatal("scanIt.Advance() found zzzKey")
		}
	}
	if scanIt.Err() != nil {
		t.Fatalf("scanIt.Advance() failed: %v", scanIt.Err())
	}
	// make sure Exec's Advance doesn't return a "zzzKey"
	for execIt.Advance() {
		var str string
		if err := execIt.Result(0, &str); err != nil {
			t.Fatal("execIt.Result()[0] is not a string: %v", err)
		}
		if str == "zzzKey" {
			t.Fatal("execIt.Advance() found zzzKey")
		}
	}
	if execIt.Err() != nil {
		t.Fatalf("execIt.Advance() failed: %v", err)
	}

	if err := b1.Commit(ctx); err != nil {
		t.Fatalf("b1.Commit() failed: %v", err)
	}

	// Check that foo is now visible.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"fooKey", "zzzKey"}, []interface{}{"fooValue", "zzzValue"})

	// Test that concurrent transactions are isolated.
	if b1, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	if b2, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1c, b2c = b1.Collection(ctx, "c"), b2.Collection(ctx, "c")

	if err := b1c.Put(ctx, "barKey", "barValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := b1c.Put(ctx, "bazKey", "bazValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	var got string
	if err := b2c.Get(ctx, "barKey", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("Get() should have failed: %v", err)
	}
	if err := b2c.Put(ctx, "rabKey", "rabValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	if err := b1.Commit(ctx); err != nil {
		t.Fatalf("b1.Commit() failed: %v", err)
	}
	if err := b2.Commit(ctx); err == nil {
		t.Fatalf("b2.Commit() should have failed: %v", err)
	}

	// Check that foo, bar, baz and zzz (but not rab) are now visible.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"barKey", "bazKey", "fooKey", "zzzKey"}, []interface{}{"barValue", "bazValue", "fooValue", "zzzValue"})
}

// Tests that BatchDatabase.ListCollections does not see the effect of
// concurrent collection creation.

// Note, this test fails if Database.ListCollections is implemented using glob,
// because b.ListCollections() does not see "c". The glob client library issues
// glob on each point along the path to check for Resolve access. Glob("a")
// returns "a/d" but not "a/d%%batchInfo", so the glob client library does not
// recurse further.
func TestBatchListCollections(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.CreateCollection(t, ctx, d, "c")
	user := "root:o:app:client"
	b, err := d.BeginBatch(ctx, wire.BatchOptions{})

	got, err := d.ListCollections(ctx)
	want := []wire.Id{{user, "c"}}
	if err != nil {
		t.Fatalf("self.ListCollections() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	tu.CreateCollection(t, ctx, d, "c_nonbatch")

	// Non-batch should see c_nonbatch; batch should only see c.
	got, err = d.ListCollections(ctx)
	want = []wire.Id{{user, "c"}, {user, "c_nonbatch"}}
	if err != nil {
		t.Fatalf("self.ListCollections() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	got, err = b.ListCollections(ctx)
	want = []wire.Id{{user, "c"}}
	if err != nil {
		t.Fatalf("self.ListCollections() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	// Create and destroy collections within a batch.
	if err := b.Collection(ctx, "c_batch").Create(ctx, nil); err != nil {
		t.Fatalf("b.c_batch.Create() failed: %v", err)
	}
	if err := b.Collection(ctx, "c").Destroy(ctx); err != nil {
		t.Fatalf("b.c.Destroy() failed: %v", err)
	}

	// Non-batch should see c and c_nonbatch; batch should only see c_batch.
	got, err = d.ListCollections(ctx)
	want = []wire.Id{{user, "c"}, {user, "c_nonbatch"}}
	if err != nil {
		t.Fatalf("self.ListCollections() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	got, err = b.ListCollections(ctx)
	want = []wire.Id{{user, "c_batch"}}
	if err != nil {
		t.Fatalf("self.ListCollections() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	// Commit should fail because ListCollections conflicts with Create/Destroy.
	if err := b.Commit(ctx); verror.ErrorID(err) != wire.ErrConcurrentBatch.ID {
		t.Fatalf("b.Commit() should have failed with ErrConcurrentBatch, got: %v", err)
	}
}

// Tests that BatchDatabase.Exec doesn't see changes committed outside the
// batch.
// 1. Create a read only batch.
// 2. query all rows in the collection
// 3. commit a new row outside of the batch
// 4. confirm new row not seen when querying all rows in the collection
// 5. abort the batch and create a new readonly batch
// 6. confirm new row NOW seen when querying all rows in the collection
func TestBatchExecIsolation(t *testing.T) {
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

	// Begin a readonly batch.
	roBatch, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	// fetch all rows
	tu.CheckExec(t, ctx, roBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	// Add a row outside this batch
	newRow := Baz{Name: "Alice Wonderland", Active: false}
	if err := c.Put(ctx, "newRow", newRow); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	// confirm fetching all rows doesn't get the new row
	tu.CheckExec(t, ctx, roBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	// start a new batch
	roBatch.Abort(ctx)
	roBatch, err = d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	defer roBatch.Abort(ctx)

	// confirm fetching all rows NOW gets the new row
	tu.CheckExec(t, ctx, roBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow)},
		})

	// test error condition on batch
	tu.CheckExecError(t, ctx, roBatch, "select k, v from foo", syncql.ErrTableCantAccess.ID)
}

// Test exec of delete statement in readonly batch (it should fail).
func TestBatchReadonlyExecDelete(t *testing.T) {
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

	// Begin a readonly batch.
	roBatch, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	// Attempt to delete "foo" k/v pair with a syncQL delete.
	tu.CheckExecError(t, ctx, roBatch, "delete from c where k = \"foo\"", syncql.ErrTableCantAccess.ID)

	// start a new batch
	roBatch.Abort(ctx)
}

// Tests that BatchDatabase.Exec DOES see changes made inside the transaction
// but before Exec is called.
func TestBatchExec(t *testing.T) {
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

	// Begin a readwrite batch.
	rwBatch, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: false})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	// fetch all rows
	tu.CheckExec(t, ctx, rwBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	rwBatchTb := rwBatch.Collection(ctx, "c")

	// Add a row in this batch
	newRow := Baz{Name: "Snow White", Active: true}
	if err := rwBatchTb.Put(ctx, "newRow", newRow); err != nil {
		t.Fatalf("rwBatchTb.Put() failed: %v", err)
	}

	// confirm fetching all rows DOES get the new row
	tu.CheckExec(t, ctx, rwBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow)},
		})

	// Delete the first row (bar) and the last row (newRow).
	// Change the baz row.  Confirm these rows are no longer fetched and that
	// the change to baz is seen.
	tu.CheckExec(t, ctx, rwBatch, "delete from c where k = \"bar\" or k = \"newRow\"",
		[]string{"Count"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf(2)},
		})
	baz2 := Baz{Name: "Batman", Active: false}
	if err := rwBatchTb.Put(ctx, "baz", baz2); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	tu.CheckExec(t, ctx, rwBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz2)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	// Add the 2 rows (we just deleted) back again.
	// Delete the other two rows (baz, foo).
	// Confirm we just see the three rows we added back.
	// Add a row in this batch
	bar2 := Baz{Name: "Tom Thumb", Active: true}
	if err := rwBatchTb.Put(ctx, "bar", bar2); err != nil {
		t.Fatalf("rwBatchTb.Put() failed: %v", err)
	}
	newRow2 := Baz{Name: "Snow White", Active: false}
	if err := rwBatchTb.Put(ctx, "newRow", newRow2); err != nil {
		t.Fatalf("rwBatchTb.Put() failed: %v", err)
	}
	tu.CheckExec(t, ctx, rwBatch, "delete from c where k = \"baz\" or k = \"foo\"",
		[]string{"Count"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf(2)},
		})
	tu.CheckExec(t, ctx, rwBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar2)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow2)},
		})

	// commit rw batch
	rwBatch.Commit(ctx)

	// start a new (ro) batch
	roBatch, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: false})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	defer roBatch.Abort(ctx)

	// confirm fetching all rows gets the rows committed above
	tu.CheckExec(t, ctx, roBatch, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar2)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow2)},
		})
}

// Tests enforcement of BatchOptions.ReadOnly.
func TestReadOnlyBatch(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	if err := c.Put(ctx, "fooKey", "fooValue"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	b1, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1c := b1.Collection(ctx, "c")

	if err := b1c.Put(ctx, "barKey", "barValue"); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
		t.Fatalf("Put() should have failed: %v", err)
	}
	if err := b1c.DeleteRange(ctx, syncbase.Prefix("fooKey")); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
		t.Fatalf("Collection.DeleteRange() should have failed: %v", err)
	}
	if err := b1c.Row("fooKey").Delete(ctx); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
		t.Fatalf("Row.Delete() should have failed: %v", err)
	}
}

// Tests that all ops fail after attempted commit or abort.
func TestOpAfterFinalize(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	// TODO(sadovsky): Add some sort of "op after finalize" error type and check
	// for it specifically below.
	checkOpsFail := func(b syncbase.BatchDatabase) {
		bc := b.Collection(ctx, "c")
		var got string
		if err := bc.Get(ctx, "fooKey", &got); err == nil {
			tu.Fatal(t, "Get() should have failed")
		}
		it := bc.Scan(ctx, syncbase.Prefix(""))
		it.Advance()
		if it.Err() == nil {
			tu.Fatal(t, "Scan() should have failed")
		}
		if err := bc.Put(ctx, "barKey", "barValue"); err == nil {
			tu.Fatal(t, "Put() should have failed")
		}
		if err := bc.DeleteRange(ctx, syncbase.Prefix("fooKey")); err == nil {
			tu.Fatal(t, "Collection.DeleteRange() should have failed: %v", err)
		}
		if err := bc.Row("fooKey").Delete(ctx); err == nil {
			tu.Fatal(t, "Row.Delete() should have failed: %v", err)
		}
		if err := b.Commit(ctx); err == nil {
			tu.Fatal(t, "Commit() should have failed")
		}
	}

	// Commit a transaction, check that subsequent ops fail.
	b1, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1c := b1.Collection(ctx, "c")

	if err := b1c.Put(ctx, "fooKey", "fooValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := b1.Commit(ctx); err != nil {
		t.Fatalf("b1.Commit() failed: %v", err)
	}
	checkOpsFail(b1)

	// Create a transaction with a conflict, check that the commit fails, then
	// check that subsequent ops fail.
	if b1, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1c = b1.Collection(ctx, "c")

	// Conflicts with future b1c.Get().
	if err := c.Put(ctx, "fooKey", "v2"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	var got string
	if err := b1c.Get(ctx, "fooKey", &got); err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	want := "fooValue"
	if got != want {
		t.Fatalf("Get() returned unexpected value: got %q, want %q", got, want)
	}
	if err := b1.Commit(ctx); err == nil {
		t.Fatalf("b1.Commit() should have failed: %v", err)
	}
	checkOpsFail(b1)

	// Create a transaction and immediately abort it, then check that subsequent
	// ops fail.
	if b1, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1c = b1.Collection(ctx, "c")
	b1.Abort(ctx)
	checkOpsFail(b1)
}

// Tests that batch methods called on non-batch return ErrNotBoundToBatch.
func TestDisallowedMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")

	dc := wire.DatabaseClient(d.FullName())
	if err := dc.Commit(ctx, ""); verror.ErrorID(err) != wire.ErrNotBoundToBatch.ID {
		t.Fatalf("dc.Commit() should have failed: %v", err)
	}
	if err := dc.Abort(ctx, ""); verror.ErrorID(err) != wire.ErrNotBoundToBatch.ID {
		t.Fatalf("dc.Abort() should have failed: %v", err)
	}
}

// Tests that d.BeginBatch() fails gracefully if the database does not exist.
func TestBeginBatchWithNonexistentDatabase(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := syncbase.NewService(sName).DatabaseForId(wire.Id{"a", "d"}, nil)
	if _, err := d.BeginBatch(ctx, wire.BatchOptions{}); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("d.BeginBatch() should have failed: %v", err)
	}
}

// tryWithConcurrentWrites is a RunInBatch() test helper that causes the first
// failTimes attempts to fail with a concurrent write before succeeding.
func tryWithConcurrentWrites(t *testing.T, ctx *context.T, d syncbase.Database, failTimes int, returnErr error) error {
	var value string
	retries := 0
	return syncbase.RunInBatch(ctx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		retries++
		// Read foo.
		if err := b.Collection(ctx, "c").Get(ctx, fmt.Sprintf("foo-%d", retries), &value); verror.ErrorID(err) != verror.ErrNoExist.ID {
			t.Errorf("b.Get() should have failed with ErrNoExist, got: %v", err)
		}
		// If we need to fail, write to foo in a separate concurrent batch. This
		// is always written on every attempt.
		if retries < failTimes {
			if err := d.Collection(ctx, "c").Put(ctx, fmt.Sprintf("foo-%d", retries), "foo"); err != nil {
				t.Errorf("d.Put() failed: %v", err)
			}
		}
		// Write to bar. This is only committed on a successful attempt.
		if err := b.Collection(ctx, "c").Put(ctx, fmt.Sprintf("bar-%d", retries), "bar"); err != nil {
			t.Errorf("b.Put() failed: %v", err)
		}
		// Return user defined error.
		return returnErr
	})
}

// Tests that RunInBatch() properly retries on Commit failure.
func TestRunInBatchRetry(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	// Succeed (no conflict) on second try.
	if err := tryWithConcurrentWrites(t, ctx, d, 2, nil); err != nil {
		t.Errorf("RunInBatch() failed: %v", err)
	}
	// First try failed, second succeeded.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""),
		[]string{"bar-2", "foo-1"},
		[]interface{}{"bar", "foo"})
}

// Tests that RunInBatch() gives up after too many Commit failures.
func TestRunInBatchMaxRetries(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	// Succeed (no conflict) on 10th try. RunInBatch will retry 3 times and give
	// up with ErrConcurrentBatch.
	if err := tryWithConcurrentWrites(t, ctx, d, 10, nil); verror.ErrorID(err) != wire.ErrConcurrentBatch.ID {
		t.Errorf("RunInBatch() should have failed with ErrConcurrentBatch, got: %v", err)
	}
	// Three failed tries.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""),
		[]string{"foo-1", "foo-2", "foo-3"},
		[]interface{}{"foo", "foo", "foo"})
}

// Tests that RunInBatch() passes through errors without retrying.
func TestRunInBatchError(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	// Return error from fn. Errors other than ErrConcurrentTransaction are not
	// retried.
	dummyError := fmt.Errorf("dummyError")
	if err := tryWithConcurrentWrites(t, ctx, d, 10, dummyError); err != dummyError {
		t.Errorf("RunInBatch() should have failed with %v, got: %v", dummyError, err)
	}
	// Single failed try.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""),
		[]string{"foo-1"},
		[]interface{}{"foo"})
}

// Tests that RunInBatch() works with readonly batches without trying to Commit.
func TestRunInBatchReadOnly(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	// Test readonly batch.
	if err := c.Put(ctx, "foo", 1); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := syncbase.RunInBatch(ctx, d, wire.BatchOptions{ReadOnly: true}, func(b syncbase.BatchDatabase) error {
		var value int32
		// Read foo.
		if err := b.Collection(ctx, "c").Get(ctx, "foo", &value); err != nil {
			t.Fatalf("b.Get() failed: %v", err)
		}
		newValue := value + 1
		// Write to foo in a separate concurrent batch. This is always written on
		// every iteration. It should not cause a retry since readonly batches are
		// not committed.
		if err := d.Collection(ctx, "c").Put(ctx, "foo", newValue); err != nil {
			t.Errorf("d.Put() failed: %v", err)
		}
		// Read foo again. Batch should not see the incremented value.
		var rereadValue int32
		if err := b.Collection(ctx, "c").Get(ctx, "foo", &rereadValue); err != nil {
			t.Fatalf("b.Get() failed: %v", err)
		}
		if value != rereadValue {
			t.Fatal("batch should not see value change outside batch")
		}
		// Try writing to bar. This should fail since the batch is readonly.
		if err := b.Collection(ctx, "c").Put(ctx, "bar", value); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
			t.Errorf("b.Put() should have failed with ErrReadOnlyBatch, got: %v", err)
		}
		return nil
	}); err != nil {
		t.Errorf("RunInBatch() failed: %v", err)
	}
	// Single uncommitted iteration.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""),
		[]string{"foo"},
		[]interface{}{int32(2)})
}
