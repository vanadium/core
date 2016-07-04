// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/x/ref/test/v23test"
)

const pingPongPairIterations = 500

var numSync *int = flag.Int("numSync", 2, "run test with 2 or more syncbases")
var numGroup *int = flag.Int("numGroup", 1, "run test with 1 or more syncgroups")

// BenchmarkPingPongPair measures the round trip sync latency between a pair of
// Syncbase instances that Ping Pong data to each other over a syncgroup.
//
// This benchmark performs the following operations:
// - Create two syncbase instances and have them join the same syncgroup.
// - Each watches the other syncbase's "section" of the collection.
// - A preliminary write by each syncbase ensures that clocks are synced.
// - During Ping Pong, each syncbase instance writes data to its own section of
//   the collection. The other syncbase watches that section for writes. Once a
//   write is received, it does the same.
//
// After the benchmark completes, the "ns/op" value refers to the average time
// for |pingPongPairIterations| of Ping Pong roundtrips completed.
func BenchmarkPingPongPair(b *testing.B) {
	flag.Parse()
	if *numSync < 2 {
		b.Fatalf("numSync should be at least 2, received %d\n", *numSync)
	}
	if *numGroup < 1 {
		b.Fatalf("numGroup should be at least 1, received %d\n", *numGroup)
	}
	sh := v23test.NewShell(b, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	b.ResetTimer()
	b.StopTimer()

	for iter := 0; iter < b.N; iter++ {
		// Setup *numSync Syncbases.
		sbs := setupSyncbases(b, sh, *numSync, false)

		// Setup *numGroup Syncgroups
		for g := 0; g < *numGroup; g++ {
			// Syncbase s0 is the creator.
			sgId := wire.Id{Name: fmt.Sprintf("SG%d", g+1), Blessing: testCx.Blessing}

			ok(b, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "", nil, clBlessings(sbs)))

			// The other syncbases will attempt to join the syncgroup.
			for i := 1; i < *numSync; i++ {
				ok(b, joinSyncgroup(sbs[i].clientCtx, sbs[i].sbName, sbs[0].sbName, sgId))
			}
		}

		// Obtain the handles to the databases.
		db0, _ := getDbAndCollection(sbs[0].sbName)
		db1, _ := getDbAndCollection(sbs[1].sbName)

		// Setup the remaining syncbases
		for i := 2; i < *numSync; i++ {
			getDbAndCollection(sbs[i].sbName)
		}

		// Set up the watch streams (watching the other syncbase's prefix).
		prefix0, prefix1 := "prefix0", "prefix1"
		w0 := db0.Watch(sbs[0].clientCtx, watch.ResumeMarker("now"), []wire.CollectionRowPattern{
			util.RowPrefixPattern(testCx, prefix1),
		})
		ok(b, w0.Err())
		w1 := db1.Watch(sbs[1].clientCtx, watch.ResumeMarker("now"), []wire.CollectionRowPattern{
			util.RowPrefixPattern(testCx, prefix0),
		})
		ok(b, w0.Err())

		// The join has succeeded, so make sure sync is initialized.
		// The strategy is: s0 sends to s1, and then s1 responds.
		sendInt32Sync(b, sbs[0], prefix0, w1)
		sendInt32Sync(b, sbs[1], prefix1, w0)

		// Sync is now active, so it is time to really start our watch streams.
		c0, c1 := make(chan int32), make(chan int32)
		go watchInt32s(b, w0, c0)
		go watchInt32s(b, w1, c1)

		// Perform and time |pingPongIterations| of ping pong between syncbases.
		b.StartTimer()
		lastTime := time.Now()
		var t0to1, t1to0 int64
		for i := 0; i < pingPongPairIterations; i++ {
			var delta int64
			value := int32(i)
			ok(b, writeInt32(sbs[0], prefix0, value))
			<-c1
			delta, lastTime = getDelta(lastTime)
			t0to1 += delta
			ok(b, writeInt32(sbs[1], prefix1, value))
			<-c0
			delta, lastTime = getDelta(lastTime)
			t1to0 += delta
		}
		b.StopTimer()

		// Clean up syncbases.
		for i, _ := range sbs {
			sbs[i].cleanup(os.Interrupt)
		}

		// Log intermediate information.
		b.Logf("Iteration %d of %d\n", iter, b.N)
		b.Logf("Avg Time from 0 to 1: %d ns\n", t0to1/int64(pingPongPairIterations))
		b.Logf("Avg Time from 1 to 0: %d ns\n", t1to0/int64(pingPongPairIterations))
		b.Logf("Avg Time per iteration: %d ns\n", (t1to0+t0to1)/int64(pingPongPairIterations))
	}
}

// getDelta computes the delta between the last time and now.
// It returns both that delta and the new time.
func getDelta(lastTime time.Time) (delta int64, nextTime time.Time) {
	nextTime = time.Now()
	delta = nextTime.Sub(lastTime).Nanoseconds()
	return
}

// sendInt32Sync sends data from 1 syncbase to another.
// Be sure that the receiving watch stream sees the data and is still ok.
func sendInt32Sync(b *testing.B, ts *testSyncbase, senderPrefix string, w syncbase.WatchStream) {
	ok(b, writeInt32(ts, senderPrefix, -1))
	if w.Advance() {
		w.Change() // grab the change, but ignore the value.
	}
	watchStreamOk(b, w)
}

// watchInt32s sends the value of each put through to the channel.
func watchInt32s(b *testing.B, w syncbase.WatchStream, c chan int32) {
	var count int
	for count < pingPongPairIterations && w.Advance() {
		var t int32
		change := w.Change()
		if change.ChangeType == syncbase.DeleteChange {
			b.Error("Received a delete change")
		}
		err := change.Value(&t)
		if err != nil {
			b.Error(err)
		}
		c <- t
		watchStreamOk(b, w)
		count++
	}

	w.Cancel() // The stream can be canceled since we've seen enough iterations.
}

// getDbAndCollection obtains the database and collection handles for a syncbase
// name.
func getDbAndCollection(syncbaseName string) (d syncbase.Database, c syncbase.Collection) {
	d = syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c = d.CollectionForId(testCx)
	return
}

// writeInt32 writes keyValue into syncbase at keyPrefix/keyValue.
func writeInt32(ts *testSyncbase, keyPrefix string, keyValue int32) error {
	ctx := ts.clientCtx
	syncbaseName := ts.sbName
	_, c := getDbAndCollection(syncbaseName)

	key := fmt.Sprintf("%s/%d", keyPrefix, keyValue)

	if err := c.Put(ctx, key, keyValue); err != nil {
		return fmt.Errorf("c.Put() failed: %v", err)
	}
	return nil
}

// watchStreamOk emits an error if the watch stream has an error.
func watchStreamOk(b *testing.B, w syncbase.WatchStream) {
	if w.Err() != nil {
		b.Errorf("stream error: %v", w.Err())
	}
}
