// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs_test

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/v23/naming"
	"v.io/v23/services/application"
	"v.io/v23/verror"
	"v.io/x/ref/services/internal/fs"
	_ "v.io/x/ref/services/profile"
)

func tempFile(t *testing.T) string {
	tmpfile, err := ioutil.TempFile("", "simplestore-test-")
	if err != nil {
		t.Fatalf("ioutil.TempFile() failed: %v", err)
	}
	defer tmpfile.Close()
	return tmpfile.Name()
}

func TestNewMemstore(t *testing.T) {
	memstore, err := fs.NewMemstore("")

	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}

	if _, err = os.Stat(memstore.PersistedFile()); err != nil {
		t.Fatalf("Stat(%v) failed: %v", memstore.PersistedFile(), err)
	}
	os.Remove(memstore.PersistedFile())
}

func TestNewNamedMemstore(t *testing.T) {
	path := tempFile(t)
	defer os.Remove(path)
	memstore, err := fs.NewMemstore(path)
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}

	if _, err = os.Stat(memstore.PersistedFile()); err != nil {
		t.Fatalf("Stat(%v) failed: %v", path, err)
	}
}

// Verify that all of the listed paths Exists().
// Caller is responsible for setting up any transaction state necessary.
func allPathsExist(ts *fs.Memstore, paths []string) error {
	for _, p := range paths {
		exists, err := ts.BindObject(p).Exists(nil)
		if err != nil {
			return fmt.Errorf("Exists(%s) expected to succeed but failed: %v", p, err)
		}
		if !exists {
			return fmt.Errorf("Exists(%s) expected to be true but is false", p)
		}
	}
	return nil
}

// Verify that all of the listed paths !Exists().
// Caller is responsible for setting up any transaction state necessary.
func allPathsDontExist(ts *fs.Memstore, paths []string) error {
	for _, p := range paths {
		exists, err := ts.BindObject(p).Exists(nil)
		if err != nil {
			return fmt.Errorf("Exists(%s) expected to succeed but failed: %v", p, err)
		}
		if exists {
			return fmt.Errorf("Exists(%s) expected to be false but is true", p)
		}
	}
	return nil
}

type PathValue struct {
	Path     string
	Expected interface{}
}

// getEquals tests that every provided path is equal to the specified value.
func allPathsEqual(ts *fs.Memstore, pvs []PathValue) error {
	for _, p := range pvs {
		v, err := ts.BindObject(p.Path).Get(nil)
		if err != nil {
			return fmt.Errorf("Get(%s) expected to succeed but failed: %v", p, err)
		}
		if !reflect.DeepEqual(p.Expected, v.Value) {
			return fmt.Errorf("Unexpected non-equality for %s: got %v, expected %v", p.Path, v.Value, p.Expected)
		}
	}
	return nil
}

func TestSerializeDeserialize(t *testing.T) {
	path := tempFile(t)
	defer os.Remove(path)
	memstoreOriginal, err := fs.NewMemstore(path)
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}

	// Create example data.
	envelope := application.Envelope{
		Args:   []string{"--help"},
		Env:    []string{"DEBUG=1"},
		Binary: application.SignedFile{File: "/v23/name/of/binary"},
	}
	secondEnvelope := application.Envelope{
		Args:   []string{"--save"},
		Env:    []string{"VEYRON=42"},
		Binary: application.SignedFile{File: "/v23/name/of/binary/is/memstored"},
	}

	// TRANSACTION BEGIN
	// Insert a value into the fs.Memstore at /test/a
	memstoreOriginal.Lock()
	tname, err := memstoreOriginal.BindTransactionRoot("ignored").CreateTransaction(nil)
	if err != nil {
		t.Fatalf("CreateTransaction() failed: %v", err)
	}
	if _, err := memstoreOriginal.BindObject(fs.TP("/test/a")).Put(nil, envelope); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	if err := allPathsExist(memstoreOriginal, []string{fs.TP("/test/a"), fs.TP("/test")}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{{fs.TP("/test/a"), envelope}}); err != nil {
		t.Fatalf("%v", err)
	}

	if err := memstoreOriginal.BindTransaction(tname).Commit(nil); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}
	memstoreOriginal.Unlock()
	// TRANSACTION END

	// Validate persisted state.
	if err := allPathsExist(memstoreOriginal, []string{"/test/a", "/test"}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{{"/test/a", envelope}}); err != nil {
		t.Fatalf("%v", err)
	}

	// TRANSACTION BEGIN Write a value to /test/b as well.
	memstoreOriginal.Lock()
	tname, err = memstoreOriginal.BindTransactionRoot("also ignored").CreateTransaction(nil)
	bindingTnameTestB := memstoreOriginal.BindObject(fs.TP("/test/b"))
	if _, err := bindingTnameTestB.Put(nil, envelope); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Validate persisted state during transaction
	if err := allPathsExist(memstoreOriginal, []string{"/test/a", "/test"}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{{"/test/a", envelope}}); err != nil {
		t.Fatalf("%v", err)
	}
	// Validate pending state during transaction
	if err := allPathsExist(memstoreOriginal, []string{fs.TP("/test/a"), fs.TP("/test"), fs.TP("/test/b")}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{
		{fs.TP("/test/a"), envelope},
		{fs.TP("/test/b"), envelope}}); err != nil {
		t.Fatalf("%v", err)
	}

	// Commit the <tname>/test/b to /test/b
	if err := memstoreOriginal.Commit(nil); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}
	memstoreOriginal.Unlock()
	// TODO(rjkroege): Consider ensuring that Get() on  <tname>/test/b should now fail.
	// TRANSACTION END

	// Validate persisted state after transaction
	if err := allPathsExist(memstoreOriginal, []string{"/test/a", "/test", "/test/b"}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{
		{"/test/a", envelope},
		{"/test/b", envelope}}); err != nil {
		t.Fatalf("%v", err)
	}

	// TRANSACTION BEGIN (to be abandonned)
	memstoreOriginal.Lock()
	tname, err = memstoreOriginal.BindTransactionRoot("").CreateTransaction(nil)

	// Exists is true before doing anything.
	if err := allPathsExist(memstoreOriginal, []string{fs.TP("/test")}); err != nil {
		t.Fatalf("%v", err)
	}

	if _, err := memstoreOriginal.BindObject(fs.TP("/test/b")).Put(nil, secondEnvelope); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Validate persisted state during transaction
	if err := allPathsExist(memstoreOriginal, []string{
		"/test/a",
		"/test/b",
		"/test",
		fs.TP("/test"),
		fs.TP("/test/a"),
		fs.TP("/test/b"),
	}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{
		{"/test/a", envelope},
		{"/test/b", envelope},
		{fs.TP("/test/b"), secondEnvelope},
		{fs.TP("/test/a"), envelope},
	}); err != nil {
		t.Fatalf("%v", err)
	}

	// Pending Remove() of /test
	if err := memstoreOriginal.BindObject(fs.TP("/test")).Remove(nil); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	// Verify that all paths are successfully removed from the in-progress transaction.
	if err := allPathsDontExist(memstoreOriginal, []string{fs.TP("/test/a"), fs.TP("/test"), fs.TP("/test/b")}); err != nil {
		t.Fatalf("%v", err)
	}
	// But all paths remain in the persisted version.
	if err := allPathsExist(memstoreOriginal, []string{"/test/a", "/test", "/test/b"}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{
		{"/test/a", envelope},
		{"/test/b", envelope},
	}); err != nil {
		t.Fatalf("%v", err)
	}

	// At which point, Get() on the transaction won't find anything.
	if _, err := memstoreOriginal.BindObject(fs.TP("/test/a")).Get(nil); verror.ErrorID(err) != fs.ErrNotInMemStore.ID {
		t.Fatalf("Get() should have failed: got %v, expected %v", err, verror.New(fs.ErrNotInMemStore, nil, tname+"/test/a"))
	}

	// Attempting to Remove() it over again will fail.
	if err := memstoreOriginal.BindObject(fs.TP("/test/a")).Remove(nil); verror.ErrorID(err) != fs.ErrNotInMemStore.ID {
		t.Fatalf("Remove() should have failed: got %v, expected %v", err, verror.New(fs.ErrNotInMemStore, nil, tname+"/test/a"))
	}

	// Attempting to Remove() a non-existing path will fail.
	if err := memstoreOriginal.BindObject(fs.TP("/foo")).Remove(nil); verror.ErrorID(err) != fs.ErrNotInMemStore.ID {
		t.Fatalf("Remove() should have failed: got %v, expected %v", err, verror.New(fs.ErrNotInMemStore, nil, tname+"/foo"))
	}

	// Exists() a non-existing path will fail.
	if present, _ := memstoreOriginal.BindObject(fs.TP("/foo")).Exists(nil); present {
		t.Fatalf("Exists() should have failed for non-existing path %s", tname+"/foo")
	}

	// Abort the transaction without committing it.
	memstoreOriginal.Abort(nil)
	memstoreOriginal.Unlock()
	// TRANSACTION END (ABORTED)

	// Validate that persisted state after abandonned transaction has not changed.
	if err := allPathsExist(memstoreOriginal, []string{"/test/a", "/test", "/test/b"}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreOriginal, []PathValue{
		{"/test/a", envelope},
		{"/test/b", envelope}}); err != nil {
		t.Fatalf("%v", err)
	}

	// Validate that Get will fail on a non-existent path.
	if _, err := memstoreOriginal.BindObject("/test/c").Get(nil); verror.ErrorID(err) != fs.ErrNotInMemStore.ID {
		t.Fatalf("Get() should have failed: got %v, expected %v", err, verror.New(fs.ErrNotInMemStore, nil, tname+"/test/c"))
	}

	// Verify that the previous Commit() operations have persisted to
	// disk by creating a new Memstore from the contents on disk.
	memstoreCopy, err := fs.NewMemstore(path)
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}
	// Verify that memstoreCopy is an exact copy of memstoreOriginal.
	if err := allPathsExist(memstoreCopy, []string{"/test/a", "/test", "/test/b"}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreCopy, []PathValue{
		{"/test/a", envelope},
		{"/test/b", envelope}}); err != nil {
		t.Fatalf("%v", err)
	}

	// TRANSACTION BEGIN
	memstoreCopy.Lock()
	tname, err = memstoreCopy.BindTransactionRoot("also ignored").CreateTransaction(nil)

	// Add a pending object c to test that pending objects are deleted.
	if _, err := memstoreCopy.BindObject(fs.TP("/test/c")).Put(nil, secondEnvelope); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := allPathsExist(memstoreCopy, []string{
		fs.TP("/test/a"),
		"/test/a",
		fs.TP("/test"),
		"/test",
		fs.TP("/test/b"),
		"/test/b",
		fs.TP("/test/c"),
	}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsEqual(memstoreCopy, []PathValue{
		{fs.TP("/test/a"), envelope},
		{fs.TP("/test/b"), envelope},
		{fs.TP("/test/c"), secondEnvelope},
		{"/test/a", envelope},
		{"/test/b", envelope},
	}); err != nil {
		t.Fatalf("%v", err)
	}

	// Remove /test/a /test/b /test/c /test
	if err := memstoreCopy.BindObject(fs.TP("/test")).Remove(nil); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	// Verify that all paths are successfully removed from the in-progress transaction.
	if err := allPathsDontExist(memstoreCopy, []string{
		fs.TP("/test/a"),
		fs.TP("/test"),
		fs.TP("/test/b"),
		fs.TP("/test/c"),
	}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := allPathsExist(memstoreCopy, []string{
		"/test/a",
		"/test",
		"/test/b",
	}); err != nil {
		t.Fatalf("%v", err)
	}
	// Commit the change.
	if err = memstoreCopy.Commit(nil); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}
	memstoreCopy.Unlock()
	// TRANSACTION END

	// Create a new Memstore from file to see if Remove operates are
	// persisted.
	memstoreRemovedCopy, err := fs.NewMemstore(path)
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed for removed copy: %v", err)
	}
	if err := allPathsDontExist(memstoreRemovedCopy, []string{
		"/test/a",
		"/test",
		"/test/b",
		"/test/c",
	}); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestOperationsNeedValidBinding(t *testing.T) {
	path := tempFile(t)
	defer os.Remove(path)
	memstoreOriginal, err := fs.NewMemstore(path)
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}

	// Create example data.
	envelope := application.Envelope{
		Args:   []string{"--help"},
		Env:    []string{"DEBUG=1"},
		Binary: application.SignedFile{File: "/v23/name/of/binary"},
	}

	// TRANSACTION BEGIN
	// Attempt inserting a value at /test/a.
	memstoreOriginal.Lock()
	tname, err := memstoreOriginal.BindTransactionRoot("").CreateTransaction(nil)
	if err != nil {
		t.Fatalf("CreateTransaction() failed: %v", err)
	}

	if err := memstoreOriginal.BindTransaction(tname).Commit(nil); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}
	memstoreOriginal.Unlock()
	// TRANSACTION END

	// Put outside ot a transaction should fail.
	bindingTnameTestA := memstoreOriginal.BindObject(naming.Join("fooey", "/test/a"))
	if _, err := bindingTnameTestA.Put(nil, envelope); verror.ErrorID(err) != fs.ErrWithoutTransaction.ID {
		t.Fatalf("Put() failed: got %v, expected %v", err, verror.New(fs.ErrWithoutTransaction, nil, "Put()"))
	}

	// Remove outside of a transaction should fail
	if err := bindingTnameTestA.Remove(nil); verror.ErrorID(err) != fs.ErrWithoutTransaction.ID {
		t.Fatalf("Put() failed: got %v, expected %v", err, verror.New(fs.ErrWithoutTransaction, nil, "Remove()"))
	}

	// Commit outside of a transaction should fail
	if err := memstoreOriginal.BindTransaction(tname).Commit(nil); verror.ErrorID(err) != fs.ErrDoubleCommit.ID {
		t.Fatalf("Commit() failed: got %v, expected %v", err, verror.New(fs.ErrDoubleCommit, nil))
	}

	// Attempt inserting a value at /test/b
	memstoreOriginal.Lock()
	tname, err = memstoreOriginal.BindTransactionRoot("").CreateTransaction(nil)
	if err != nil {
		t.Fatalf("CreateTransaction() failed: %v", err)
	}

	bindingTnameTestB := memstoreOriginal.BindObject(fs.TP("/test/b"))
	if _, err := bindingTnameTestB.Put(nil, envelope); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	// Abandon transaction.
	memstoreOriginal.Unlock()

	// Remove should definitely fail on an abndonned transaction.
	if err := bindingTnameTestB.Remove(nil); verror.ErrorID(err) != fs.ErrWithoutTransaction.ID {
		t.Fatalf("Remove() failed: got %v, expected %v", err, verror.New(fs.ErrWithoutTransaction, nil, "Remove()"))
	}
}

func TestOpenEmptyMemstore(t *testing.T) {
	path := tempFile(t)
	defer os.Remove(path)

	// Create a brand new memstore persisted to namedms. This will
	// have the side-effect of creating an empty backing file.
	if _, err := fs.NewMemstore(path); err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}

	// Create another memstore that will attempt to deserialize the empty
	// backing file.
	if _, err := fs.NewMemstore(path); err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}
}

func TestChildren(t *testing.T) {
	memstore, err := fs.NewMemstore("")
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}
	defer os.Remove(memstore.PersistedFile())

	// Create example data.
	envelope := application.Envelope{
		Args:   []string{"--help"},
		Env:    []string{"DEBUG=1"},
		Binary: application.SignedFile{File: "/v23/name/of/binary"},
	}

	// TRANSACTION BEGIN
	memstore.Lock()
	tname, err := memstore.BindTransactionRoot("ignored").CreateTransaction(nil)
	if err != nil {
		t.Fatalf("CreateTransaction() failed: %v", err)
	}
	// Insert a few values
	names := []string{"/test/a", "/test/b", "/test/a/x", "/test/a/y", "/test/b/fooooo/bar"}
	for _, n := range names {
		if _, err := memstore.BindObject(fs.TP(n)).Put(nil, envelope); err != nil {
			t.Fatalf("Put() failed: %v", err)
		}
	}
	if err := memstore.BindTransaction(tname).Commit(nil); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}
	memstore.Unlock()
	// TRANSACTION END

	memstore.Lock()
	testcases := []struct {
		name     string
		children []string
	}{
		{"/", []string{"test"}},
		{"/test", []string{"a", "b"}},
		{"/test/a", []string{"x", "y"}},
		{"/test/b", []string{"fooooo"}},
		{"/test/b/fooooo", []string{"bar"}},
		{"/test/a/x", nil},
		{"/test/a/y", nil},
	}
	for _, tc := range testcases {
		children, err := memstore.BindObject(tc.name).Children()
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tc.name, err)
			continue
		}
		if !reflect.DeepEqual(children, tc.children) {
			t.Errorf("unexpected result for %q: got %q, expected %q", tc.name, children, tc.children)
		}
	}

	for _, notthere := range []string{"/doesnt-exist", "/tes"} {
		if _, err := memstore.BindObject(notthere).Children(); err == nil {
			t.Errorf("unexpected success for: %q", notthere)
		}
	}
	memstore.Unlock()
}

func TestFormatConversion(t *testing.T) {
	path := tempFile(t)
	defer os.Remove(path)
	originalMemstore, err := fs.NewMemstore(path)
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}

	// Create example data.
	envelope := application.Envelope{
		Args:   []string{"--help"},
		Env:    []string{"DEBUG=1"},
		Binary: application.SignedFile{File: "/v23/name/of/binary"},
	}

	// TRANSACTION BEGIN
	// Insert a value into the legacy Memstore at /test/a
	originalMemstore.Lock()
	tname, err := originalMemstore.BindTransactionRoot("ignored").CreateTransaction(nil)
	if err != nil {
		t.Fatalf("CreateTransaction() failed: %v", err)
	}
	if _, err := originalMemstore.BindObject(fs.TP("/test/a")).Put(nil, envelope); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := originalMemstore.BindTransaction(tname).Commit(nil); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}
	originalMemstore.Unlock()

	// Write the original memstore to a GOB file.
	if err := gobPersist(t, originalMemstore); err != nil {
		t.Fatalf("gobPersist() failed: %v", err)
	}

	// Open the GOB format file.
	memstore, err := fs.NewMemstore(path)
	if err != nil {
		t.Fatalf("fs.NewMemstore() failed: %v", err)
	}

	// Verify the state.
	if err := allPathsEqual(memstore, []PathValue{{"/test/a", envelope}}); err != nil {
		t.Fatalf("%v", err)
	}
}

// gobPersist writes Memstore ms to its backing file.
func gobPersist(t *testing.T, ms *fs.Memstore) error {
	// Convert this memstore to the legacy GOM format.
	data := ms.GetGOBConvertedMemstore()

	// Persist this file to a GOB format file.
	fname := ms.PersistedFile()
	file, err := os.Create(fname)
	if err != nil {
		t.Fatalf("os.Create(%s) failed: %v", fname, err)
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	err = enc.Encode(data)
	if err := enc.Encode(data); err != nil {
		t.Fatalf("enc.Encode() failed: %v", err)
	}
	return nil
}
