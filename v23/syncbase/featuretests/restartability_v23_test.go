// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/syncbaselib"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23test"
)

const (
	acl = `{"Read": {"In":["root:o:app:client"]}, "Write": {"In":["root:o:app:client"]}, "Admin": {"In":["root:o:app:client"]}, "Resolve": {"In":["root:o:app:client"]}}`
)

func restartabilityInit(sh *v23test.Shell) (rootDir string, clientCtx *context.T, serverCreds *v23test.Credentials) {
	sh.StartRootMountTable()

	rootDir = sh.MakeTempDir()
	clientCtx = sh.ForkContext("o:app:client")
	serverCreds = sh.ForkCredentials("r:server")
	return
}

// TODO(ivanpi): Duplicate of setupAppA.
func createDbAndCollection(t *testing.T, clientCtx *context.T) syncbase.Database {
	d := syncbase.NewService(testSbName).DatabaseForId(testDb, nil)
	if err := d.Create(clientCtx, nil); err != nil {
		t.Fatalf("unable to create a database: %v", err)
	}
	if err := d.CollectionForId(testCx).Create(clientCtx, nil); err != nil {
		t.Fatalf("unable to create a collection: %v", err)
	}
	return d
}

func TestV23RestartabilityHierarchy(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Interrupt)

	_ = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)
	checkHierarchy(t, clientCtx)
}

// Same as TestV23RestartabilityHierarchy except the first syncbase is killed
// with SIGKILL instead of SIGINT.
func TestV23RestartabilityCrash(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	_ = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)
	checkHierarchy(t, clientCtx)
}

var (
	dbIds = []wire.Id{{"root", "d1"}, {"root", "d2"}, {"root:o:app", "d1"}, {"root:o:app", "d2"}}
	cxIds = []wire.Id{{"root:o:app:client", "c1"}, {"root:o:app:client", "c2"}}
)

// Creates dbs, collections, and rows.
func createHierarchy(t *testing.T, ctx *context.T) {
	s := syncbase.NewService(testSbName)
	for _, dbId := range dbIds {
		d := s.DatabaseForId(dbId, nil)
		if err := d.Create(ctx, nil); err != nil {
			tu.Fatalf(t, "d.Create() failed: %v", err)
		}
		for _, cxId := range cxIds {
			c := d.CollectionForId(cxId)
			if err := c.Create(ctx, nil); err != nil {
				tu.Fatalf(t, "c.Create() failed: %v", err)
			}
			for _, k := range []string{"foo", "bar"} {
				if err := c.Put(ctx, k, k); err != nil {
					tu.Fatalf(t, "c.Put() failed: %v", err)
				}
			}
		}
	}
}

// Checks for the dbs, collections, and rows created by runCreateHierarchy.
func checkHierarchy(t *testing.T, ctx *context.T) {
	s := syncbase.NewService(testSbName)
	var gotIds, wantIds []wire.Id = nil, dbIds
	var err error
	for len(gotIds) != len(wantIds) {
		if gotIds, err = s.ListDatabases(ctx); err != nil {
			tu.Fatalf(t, "s.ListDatabases() failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !reflect.DeepEqual(gotIds, wantIds) {
		tu.Fatalf(t, "Databases do not match: got %v, want %v", gotIds, wantIds)
	}
	for _, dbId := range wantIds {
		d := s.DatabaseForId(dbId, nil)
		var got, want []wire.Id = nil, cxIds
		if got, err = d.ListCollections(ctx); err != nil {
			tu.Fatalf(t, "d.ListCollections() failed: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			tu.Fatalf(t, "Collections do not match: got %v, want %v", got, want)
		}
		for _, cxId := range want {
			c := d.CollectionForId(cxId)
			if err := tu.ScanMatches(ctx, c, syncbase.Prefix(""), []string{"bar", "foo"}, []interface{}{"bar", "foo"}); err != nil {
				tu.Fatalf(t, "Scan does not match: %v", err)
			}
		}
	}
}

func TestV23RestartabilityQuiescent(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)
	d := createDbAndCollection(t, clientCtx)

	c := d.CollectionForId(testCx)

	// Do Put followed by Get on a row.
	r := c.Row("r")
	if err := r.Put(clientCtx, "testkey"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	cleanup(os.Kill)
	// Restart syncbase.
	_ = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}
}

// A read-only batch should fail if the server crashes in the middle.
func TestV23RestartabilityReadOnlyBatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)
	d := createDbAndCollection(t, clientCtx)

	// Add one row.
	if err := d.CollectionForId(testCx).Row("r").Put(clientCtx, "testkey"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}

	batch, err := d.BeginBatch(clientCtx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("unable to start batch: %v", err)
	}
	c := batch.CollectionForId(testCx)
	r := c.Row("r")

	var result string
	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	cleanup(os.Kill)
	expectedFailCtx, _ := context.WithTimeout(clientCtx, time.Second)
	// We get a variety of errors depending on how much of the network state of
	// syncbased has been reclaimed when this rpc goes out.
	if err := r.Get(expectedFailCtx, &result); err == nil {
		t.Fatalf("expected r.Get() to fail.")
	}

	// Restart syncbase.
	_ = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	if err := r.Get(clientCtx, &result); verror.ErrorID(err) != wire.ErrUnknownBatch.ID {
		t.Fatalf("expected r.Get() to fail because of ErrUnknownBatch.  got: %v", err)
	}
	if err := batch.Commit(clientCtx); verror.ErrorID(err) != wire.ErrUnknownBatch.ID {
		t.Fatalf("expected Commit() to fail because of ErrUnknownBatch.  got: %v", err)
	}

	// Try to get the row outside of a batch.  It should exist.
	if err := d.CollectionForId(testCx).Row("r").Get(clientCtx, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A read/write batch should fail if the server crashes in the middle.
func TestV23RestartabilityReadWriteBatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)
	d := createDbAndCollection(t, clientCtx)

	batch, err := d.BeginBatch(clientCtx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("unable to start batch: %v", err)
	}
	c := batch.CollectionForId(testCx)

	// Do Put followed by Get on a row.
	r := c.Row("r")
	if err := r.Put(clientCtx, "testkey"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	cleanup(os.Kill)
	expectedFailCtx, _ := context.WithTimeout(clientCtx, time.Second)
	// We get a variety of errors depending on how much of the network state of
	// syncbased has been reclaimed when this rpc goes out.
	if err := r.Get(expectedFailCtx, &result); err == nil {
		t.Fatalf("expected r.Get() to fail.")
	}

	// Restart syncbase.
	_ = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	if err := r.Get(clientCtx, &result); verror.ErrorID(err) != wire.ErrUnknownBatch.ID {
		t.Fatalf("expected r.Get() to fail because of ErrUnknownBatch.  got: %v", err)
	}
	if err := batch.Commit(clientCtx); verror.ErrorID(err) != wire.ErrUnknownBatch.ID {
		t.Fatalf("expected Commit() to fail because of ErrUnknownBatch.  got: %v", err)
	}

	// Try to get the row outside of a batch.  It should not exist.
	if err := d.CollectionForId(testCx).Row("r").Get(clientCtx, &result); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("expected r.Get() to fail because of ErrNoExist.  got: %v", err)
	}
}

func decodeString(t *testing.T, change syncbase.WatchChange) string {
	var ret string
	if err := change.Value(&ret); err != nil {
		t.Fatalf("unable to decode: %v", err)
	}
	return ret
}

func TestV23RestartabilityWatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)
	d := createDbAndCollection(t, clientCtx)

	// Put one row as well as get the initial ResumeMarker.
	batch, err := d.BeginBatch(clientCtx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("unable to start batch: %v", err)
	}
	marker, err := batch.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := batch.CollectionForId(testCx).Row("r")
	if err := r.Put(clientCtx, "testvalue1"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if err := batch.Commit(clientCtx); err != nil {
		t.Fatalf("could not commit: %v", err)
	}

	// Watch for the row change.
	timeout, _ := context.WithTimeout(clientCtx, time.Second)
	stream := d.Watch(timeout, marker, []wire.CollectionRowPattern{util.RowPrefixPattern(testCx, "r")})
	if !stream.Advance() {
		cleanup(os.Interrupt)
		t.Fatalf("expected to be able to Advance: %v", stream.Err())
	}
	change := stream.Change()
	val := decodeString(t, change)
	if change.Row != "r" || val != "testvalue1" {
		t.Fatalf("unexpected row: %s", change.Row)
	}
	marker = change.ResumeMarker

	// Kill syncbased.
	cleanup(os.Kill)

	// The stream should break when the server crashes.
	if stream.Advance() {
		t.Fatalf("unexpected Advance: %v", stream.Change())
	}

	// Restart syncbased.
	cleanup = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	// Put another row.
	r = d.CollectionForId(testCx).Row("r")
	if err := r.Put(clientCtx, "testvalue2"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}

	// Resume the watch from after the first Put.  We should see only the second
	// Put.
	stream = d.Watch(clientCtx, marker, []wire.CollectionRowPattern{util.RowPrefixPattern(testCx, "")})
	if !stream.Advance() {
		t.Fatalf("expected to be able to Advance: %v", stream.Err())
	}
	change = stream.Change()
	val = decodeString(t, change)
	if change.Row != "r" || val != "testvalue2" {
		t.Fatalf("unexpected row: %s, %s", change.Row, val)
	}

	cleanup(os.Kill)

	// The stream should break when the server crashes.
	if stream.Advance() {
		t.Fatalf("unexpected Advance: %v", stream.Change())
	}
}

func corruptFile(t *testing.T, rootDir, pathRegex string) {
	var fileToCorrupt string
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if fileToCorrupt != "" {
			return nil
		}
		if match, _ := regexp.MatchString(pathRegex, path); match {
			fileToCorrupt = path
			return errors.New("found match, stop walking")
		}
		return nil
	})
	if fileToCorrupt == "" {
		t.Fatalf("Could not find file")
	}
	fileBytes, err := ioutil.ReadFile(fileToCorrupt)
	if err != nil {
		t.Fatalf("Could not read log file: %v", err)
	}
	// Overwrite last 100 bytes.
	offset := len(fileBytes) - 100 - 1
	if offset < 0 {
		t.Fatalf("Expected bigger log file.  Found: %d", len(fileBytes))
	}
	for i := 0; i < 100; i++ {
		fileBytes[i+offset] = 0x80
	}
	if err := ioutil.WriteFile(fileToCorrupt, fileBytes, 0); err != nil {
		t.Fatalf("Could not corrupt file: %v", err)
	}
}

func TestV23RestartabilityServiceDBCorruption(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	corruptFile(t, rootDir, filepath.Join(rootDir, `leveldb/.*\.log`))

	// TODO(ivanpi): Repeated below, refactor into method.
	// Expect syncbase to fail to start.
	syncbasedPath := v23test.BuildGoPkg(sh, "v.io/x/ref/services/syncbase/syncbased")
	syncbased := sh.Cmd(syncbasedPath,
		"--alsologtostderr=true",
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal", acl,
		"--name="+testSbName,
		"--root-dir="+rootDir)
	syncbased = syncbased.WithCredentials(serverCreds)
	syncbased.ExitErrorIsOk = true
	stdout, stderr := syncbased.StdoutStderr()
	if syncbased.Err == nil {
		t.Fatal("Expected syncbased to fail to start.")
	}
	t.Logf("syncbased terminated\nstdout: %v\nstderr: %v\n", stdout, stderr)

	cleanup = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)
}

func TestV23RestartabilityServiceRootDirMoved(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	files, err := ioutil.ReadDir(rootDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	newRootDir := sh.MakeTempDir()
	for _, file := range files {
		sh.Cmd("mv", filepath.Join(rootDir, file.Name()), newRootDir).Run()
	}

	_ = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: newRootDir}, acl)
	checkHierarchy(t, clientCtx)
}

func TestV23RestartabilityAppDBCorruption(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	corruptFile(t, rootDir, `apps/[^/]*/dbs/[^/]*/leveldb/.*\.log`)

	// Expect syncbase to fail to start.
	syncbasedPath := v23test.BuildGoPkg(sh, "v.io/x/ref/services/syncbase/syncbased")
	syncbased := sh.Cmd(syncbasedPath,
		"--alsologtostderr=true",
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal", acl,
		"--name="+testSbName,
		"--root-dir="+rootDir)
	syncbased = syncbased.WithCredentials(serverCreds)
	syncbased.ExitErrorIsOk = true
	stdout, stderr := syncbased.StdoutStderr()
	if syncbased.Err == nil {
		t.Fatal("Expected syncbased to fail to start.")
	}
	t.Logf("syncbased terminated\nstdout: %v\nstderr: %v\n", stdout, stderr)

	cleanup = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	// Recreate root/d1 since that is the one that got corrupted.
	d := syncbase.NewService(testSbName).DatabaseForId(wire.Id{"root", "d1"}, nil)
	if err := d.Create(clientCtx, nil); err != nil {
		t.Fatalf("d.Create() failed: %v", err)
	}
	for _, cxId := range cxIds {
		c := d.CollectionForId(cxId)
		if err := c.Create(clientCtx, nil); err != nil {
			t.Fatalf("c.Create() failed: %v", err)
		}
		for _, k := range []string{"foo", "bar"} {
			if err := c.Put(clientCtx, k, k); err != nil {
				t.Fatalf("c.Put() failed: %v", err)
			}
		}
	}

	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)
}

func TestV23RestartabilityStoreGarbageCollect(t *testing.T) {
	// TODO(ivanpi): Fully testing store garbage collection requires fault
	// injection or mocking out the store.
	// NOTE: Test assumes that leveldb destroy is implemented as 'rm -r'.
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)

	const writeMask = 0220
	// Find a leveldb directory for one of the database stores.
	var leveldbDir string
	var leveldbMode os.FileMode
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if leveldbDir != "" {
			return nil
		}
		if match, _ := regexp.MatchString(`apps/[^/]*/dbs/[^/]*/leveldb`, path); match && info.IsDir() {
			leveldbDir = path
			leveldbMode = info.Mode()
			return errors.New("found match, stop walking")
		}
		return nil
	})
	if leveldbDir == "" {
		t.Fatalf("Could not find file")
	}

	// Create a subdirectory in the leveldb directory and populate it.
	anchorDir := filepath.Join(leveldbDir, "test-anchor")
	if err := os.Mkdir(anchorDir, leveldbMode); err != nil {
		t.Fatalf("Mkdir() failed: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(anchorDir, "a"), []byte("a"), 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}
	// Remove write permission from the subdir so that store destroy fails.
	if err := os.Chmod(anchorDir, leveldbMode&^writeMask); err != nil {
		t.Fatalf("Chmod() failed: %v", err)
	}

	s := syncbase.NewService(testSbName)

	// Destroy all databases. Destroy() should not fail even though the database
	// store destruction should fail for leveldbDir picked above.
	if dbIds, err := s.ListDatabases(clientCtx); err != nil {
		t.Fatalf("s.ListDatabases() failed: %v", err)
	} else {
		for _, dbId := range dbIds {
			if err := s.DatabaseForId(dbId, nil).Destroy(clientCtx); err != nil {
				t.Fatalf("db Destroy() failed: %v", err)
			}
		}
	}
	// TODO(ivanpi): Add Exists() checks.

	// leveldbDir should still exist.
	// TODO(ivanpi): Check that other stores have been destroyed.
	if _, err := os.Stat(leveldbDir); err != nil {
		t.Errorf("os.Stat() for old store failed: %v", err)
	}

	// Recreate the hierarchy. This should not be affected by the old store.
	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)

	cleanup(os.Kill)

	// leveldbDir should still exist.
	if _, err := os.Stat(leveldbDir); err != nil {
		t.Errorf("os.Stat() for old store failed: %v", err)
	}

	// Restarting syncbased should not affect the hierarchy. Garbage collection
	// should again fail to destroy leveldbDir.
	cleanup = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	// leveldbDir should still exist.
	if _, err := os.Stat(leveldbDir); err != nil {
		t.Errorf("os.Stat() for old store failed: %v", err)
	}

	// Reinstate write permission for the anchor subdir in leveldbDir.
	if err := os.Chmod(anchorDir, leveldbMode|writeMask); err != nil {
		t.Fatalf("Chmod() failed: %v", err)
	}

	// Restart syncbased. Garbage collection should now succeed.
	_ = sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName, RootDir: rootDir}, acl)

	// leveldbDir should not exist anymore.
	if _, err := os.Stat(leveldbDir); !os.IsNotExist(err) {
		t.Errorf("os.Stat() for old store should have failed with ErrNotExist, got: %v", err)
	}

	// The hierarchy should not have been affected.
	checkHierarchy(t, clientCtx)
}
