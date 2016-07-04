// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
)

// TestWatchLogBatch tests fetching a batch of log records.
func TestWatchLogBatch(t *testing.T) {
	runTest(t, []string{common.RowPrefix, common.CollectionPermsPrefix}, runWatchLogBatchTest)
}

// runWatchLogBatchTest tests fetching a batch of log records.
func runWatchLogBatchTest(t *testing.T, st store.Store) {
	// Create a set of batches to fill the log queue.
	numTx, numPut := 3, 4

	makeKeyVal := func(batchNum, recNum int) ([]byte, []byte) {
		key := common.JoinKeyParts(common.RowPrefix, fmt.Sprintf("foo-%d-%d", batchNum, recNum))
		val := fmt.Sprintf("val-%d-%d", batchNum, recNum)
		return []byte(key), []byte(val)
	}

	for i := 0; i < numTx; i++ {
		tx := st.NewTransaction()
		for j := 0; j < numPut; j++ {
			key, val := makeKeyVal(i, j)
			if err := tx.Put(key, val); err != nil {
				t.Errorf("cannot put %s (%s): %v", key, val, err)
			}
		}
		tx.Commit()
	}

	// Fetch the batches and a few more empty fetches and verify them.
	resmark := MakeResumeMarker(0)
	var seq uint64

	for i := 0; i < (numTx + 3); i++ {
		logs, newResmark, err := ReadBatchFromLog(st, resmark)
		if err != nil {
			t.Fatalf("can't get watch log batch: %v", err)
		}
		if i < numTx {
			if len(logs) != numPut {
				t.Errorf("log fetch (i=%d) wrong log seq: %d instead of %d",
					i, len(logs), numPut)
			}

			seq += uint64(len(logs))
			expResmark := MakeResumeMarker(seq)
			if !bytes.Equal(newResmark, expResmark) {
				t.Errorf("log fetch (i=%d) wrong resmark: %s instead of %s",
					i, newResmark, expResmark)
			}

			for j, log := range logs {
				var op PutOp
				if err := log.Op.ToValue(&op); err != nil {
					t.Fatalf("ToValue failed: %v", err)
				}
				expKey, expVal := makeKeyVal(i, j)
				key := op.Key
				if !bytes.Equal(key, expKey) {
					t.Errorf("log fetch (i=%d, j=%d) bad key: %s instead of %s",
						i, j, key, expKey)
				}
				tx := st.NewTransaction()
				var val []byte
				val, err := GetAtVersion(nil, tx, key, val, op.Version)
				if err != nil {
					t.Errorf("log fetch (i=%d, j=%d) cannot GetAtVersion(): %v", i, j, err)
				}
				if !bytes.Equal(val, expVal) {
					t.Errorf("log fetch (i=%d, j=%d) bad value: %s instead of %s",
						i, j, val, expVal)
				}
				tx.Abort()
			}
		} else {
			if logs != nil || !bytes.Equal(newResmark, resmark) {
				t.Errorf("NOP log fetch (i=%d) had changes: %d logs, resmask %s",
					i, len(logs), newResmark)
			}
		}
		resmark = newResmark
	}
}

// TestWatcher tests broadcasting updates to watch clients.
func TestWatcher(t *testing.T) {
	w := newWatcher()

	// Update broadcast should never block. It should be safe to call with no
	// clients registered.
	w.broadcastUpdates()
	w.broadcastUpdates()

	// Never-receiving client should not block watcher.
	_, cancel1 := w.watchUpdates()
	defer cancel1()

	// Cancelled client should not affect watcher.
	chan2, cancel2 := w.watchUpdates()
	cancel2()
	// Cancel should be idempotent.
	cancel2()

	// Channel should be closed when client is cancelled.
	select {
	case _, ok := <-chan2:
		if ok {
			t.Fatalf("cancel2 was called, chan2 should be drained and closed")
		}
	default:
		t.Fatalf("cancel2 was called, chan2 should be closed")
	}

	// Update broadcast should not block client registration or vice versa.
	timer1 := time.After(10 * time.Second)
	registerLoop1 := make(chan bool)
	go func() {
		for i := 0; i < 5000; i++ {
			_, canceli := w.watchUpdates()
			defer canceli()
		}
		registerLoop1 <- true
	}()

	chan3, cancel3 := w.watchUpdates()

	for i := 0; i < 5000; i++ {
		w.broadcastUpdates()
	}

	select {
	case <-registerLoop1:
		// ok
	case <-timer1:
		t.Fatalf("registerLoop1 didn't finish after 10s")
	}

	// Wait for broadcast to fully propagate out of watcherLoop.
	time.Sleep(1 * time.Second)

	// chan3 should have a single pending notification.
	select {
	case _, ok := <-chan3:
		if !ok {
			t.Fatalf("chan3 should not be closed")
		}
	default:
		t.Fatalf("chan3 should have a notification")
	}
	select {
	case <-chan3:
		t.Fatalf("chan3 should not have another notification")
	default:
		// ok
	}

	// After notification was read, chan3 still receives updates.
	go func() {
		time.Sleep(500 * time.Millisecond)
		w.broadcastUpdates()
	}()

	select {
	case _, ok := <-chan3:
		if !ok {
			t.Fatalf("chan3 should not be closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("chan3 didn't receive after 5s")
	}

	// Closing the watcher.
	w.close()
	// Close should be idempotent.
	w.close()

	// Client channels should be closed when watcher is closed.
	select {
	case _, ok := <-chan3:
		if ok {
			t.Fatalf("watcher was closed, chan3 should be drained and closed")
		}
	default:
		t.Fatalf("watcher was closed, chan3 should be closed")
	}

	// Cancel is safe to call after the store is closed.
	cancel3()

	// watchUpdates is safe to call after the store is closed, returning closed
	// channel.
	chan4, cancel4 := w.watchUpdates()

	select {
	case _, ok := <-chan4:
		if ok {
			t.Fatalf("watcher was closed, chan4 should be drained and closed")
		}
	default:
		t.Fatalf("watcher was closed, chan4 should be closed")
	}

	cancel4()

	// broadcastUpdates is safe to call after the store is closed, although it
	// logs an error message.
	w.broadcastUpdates()
}
