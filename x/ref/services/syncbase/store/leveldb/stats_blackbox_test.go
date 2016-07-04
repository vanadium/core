// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb_test

import (
	"crypto/rand"
	"fmt"
	"log"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/x/ref/lib/stats"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// In this file we do integration tests of the leveldb stats via the public
// Syncbase API.  For lower level tests see the stats_test.go file.

func TestEmpty(t *testing.T) {
	_, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()

	since := time.Now()
	syncbase.NewService(serverName)

	count := 0
	for got := range statsValues("service/*/*", since) {
		count++

		// All stats values should be zero (see stats_test.go)
		if got.(int64) != 0 {
			t.Errorf("Got %v, want 0", got)
		}
	}
	// Want 2 stats keys for the service
	if count != 2 {
		t.Errorf("Got %d service stats keys, want 2", count)
	}

	count = 0
	for got := range statsValues("blobmap/*/*", since) {
		count++

		// All stats values should be zero (see stats_test.go)
		if got.(int64) != 0 {
			t.Errorf("Got %v, want 0", got)
		}
	}
	// Want 2 stats keys for the blobmap
	if count != 2 {
		t.Errorf("Got %d blobmap stats keys, want 2", count)
	}

	for range statsValues("db/v_io_a_xyz/*/*/*", since) {
		t.Error("Expected no database-specific stats")
	}
}

func TestWithMultipleDbs(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()

	since := time.Now()
	service := syncbase.NewService(serverName)
	tu.CreateDatabase(t, ctx, service, "empty_db_0")
	tu.CreateDatabase(t, ctx, service, "empty_db_1")
	tu.CreateDatabase(t, ctx, service, "empty_db_2")

	count := 0
	for got := range statsValues("service/*/*", since) {
		count++

		// All stats values should be zero (see stats_test.go)
		if got.(int64) != 0 {
			t.Errorf("Got %v, want 0", got)
		}
	}
	// Want 2 stats keys for the service
	if count != 2 {
		t.Errorf("Got %d service stats keys, want 2", count)
	}

	count = 0
	for got := range statsValues("blobmap/*/*", since) {
		count++

		// All stats values should be zero (see stats_test.go)
		if got.(int64) != 0 {
			t.Errorf("Got %v, want 0", got)
		}
	}
	// Want 2 stats keys for the blobmap
	if count != 2 {
		t.Errorf("Got %d blobmap stats keys, want 2", count)
	}

	count = 0
	for got := range statsValues("db/root_o_app/*/*/*", since) {
		count++

		// All stats values should be zero (see stats_test.go)
		if got.(int64) != 0 {
			t.Errorf("Got %v, want 0", got)
		}
	}
	// Want 2 stats keys per each of 3 DBs
	if count != 6 {
		t.Errorf("Got %d DB stats keys, want 6", count)
	}
}

func TestWithLotsOfData(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()

	since := time.Now()
	service := syncbase.NewService(serverName)
	db := tu.CreateDatabase(t, ctx, service, "big_db")

	write100MB(ctx, db)

	// Expect at least 50 MB (see stats_test.go)
	fileBytes := <-statsValues("db/root_o_app/big_db/*/filesystem_bytes", since)
	if fileBytes.(int64) < 50*1024*1024 {
		t.Errorf("Got %v, want more than 50 MB", fileBytes.(int64))
	}
	// Expect 25 or more (see stats_test.go)
	fileCount := <-statsValues("db/root_o_app/big_db/*/file_count", since)
	if fileCount.(int64) < 25 {
		t.Errorf("Got %v, want 25 or more", fileCount.(int64))
	}
}

// statsValues returns a channel with all the values since the given time of the
// sync keys that match the given glob under the root "syncbase/leveldb/".
func statsValues(pattern string, since time.Time) <-chan interface{} {
	ch := make(chan interface{})
	go func() {
		iter := stats.Glob("syncbase/leveldb/", pattern, since, true)
		for iter.Advance() {
			if err := iter.Err(); err != nil {
				panic(err)
			}
			if iter.Value().Value == nil {
				log.Printf("Ignoring empty stats %q", iter.Value().Key)
			} else {
				ch <- iter.Value().Value
			}
		}
		close(ch)
	}()
	return ch
}

// Write 100 kB each to 1024 keys, returning keys written.
func write100MB(ctx *context.T, db syncbase.Database) {
	coll := db.Collection(ctx, "the_collection")
	err := coll.Create(ctx, cxPermissions)
	if err != nil {
		panic(err)
	}

	hundredKB := make([]byte, 100*1024)

	// Write 1024 X 100 KB  (100 MB).
	for i := 0; i < 1024; i++ {
		rand.Read(hundredKB) // Randomize so it won't compress.
		key := fmt.Sprintf("foo-%d", i)
		err := coll.Put(ctx, key, hundredKB)
		if err != nil {
			panic(err)
		}
	}
	return
}

var (
	cxPermissions = tu.DefaultPerms(wire.AllCollectionTags, string(security.AllPrincipals))
)
