// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"testing"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/x/ref/services/syncbase/syncbaselib"
	"v.io/x/ref/test/v23test"
)

func TestV23ClientPutGet(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Start syncbased.
	serverCreds := sh.ForkCredentials("server")
	// TODO(aghassemi): Resolve permission is currently needed for Watch.
	// See https://github.com/vanadium/issues/issues/1110
	sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName}, `{"Resolve": {"In":["root:server", "root:o:app:client"]}, "Read": {"In":["root:server", "root:o:app:client"]}, "Write": {"In":["root:server", "root:o:app:client"]}, "Admin": {"In":["root:server", "root:o:app:client"]}}`)

	// Create database and collection.
	// TODO(ivanpi): Use setupAppA.
	ctx := sh.ForkContext("o:app:client")
	d := syncbase.NewService(testSbName).DatabaseForId(testDb, nil)
	if err := d.Create(ctx, nil); err != nil {
		t.Fatalf("unable to create a database: %v", err)
	}
	c := d.CollectionForId(testCx)
	if err := c.Create(ctx, nil); err != nil {
		t.Fatalf("unable to create a collection: %v", err)
	}
	marker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("unable to get the resume marker: %v", err)
	}

	// Do a Put followed by a Get.
	r := c.Row("testkey")
	if err := r.Put(ctx, "testvalue"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(ctx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testvalue"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	// Do a watch from the resume marker before the put operation.
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stream := d.Watch(ctxWithTimeout, marker, []wire.CollectionRowPattern{util.RowPrefixPattern(testCx, "")})
	if !stream.Advance() {
		t.Fatalf("watch stream unexpectedly reached the end: %v", stream.Err())
	}
	change := stream.Change()
	if got, want := change.EntityType, syncbase.EntityRow; got != want {
		t.Fatalf("unexpected watch entity type: got %q, want %q", got, want)
	}
	if got, want := change.Collection, testCx; got != want {
		t.Fatalf("unexpected watch collection: got %q, want %q", got, want)
	}
	if got, want := change.Row, "testkey"; got != want {
		t.Fatalf("unexpected watch row: got %q, want %q", got, want)
	}
	if got, want := change.ChangeType, syncbase.PutChange; got != want {
		t.Fatalf("unexpected watch change type: got %q, want %q", got, want)
	}
	if got, want := change.FromSync, false; got != want {
		t.Fatalf("unexpected FromSync value: got %t, want %t", got, want)
	}
	if err := change.Value(&result); err != nil {
		t.Fatalf("couldn't decode watch value: %v", err)
	}
	if got, want := result, "testvalue"; got != want {
		t.Fatalf("unexpected watch value: got %q, want %q", got, want)
	}

}
