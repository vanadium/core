// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"sort"
	"sync"
	"testing"

	"v.io/x/ref/test/testutil"
)

func randomKeys() []uint64 {
	n := (testutil.RandomIntn(256*10) / 10) + 256
	k := make([]uint64, n)
	for i := 0; i < n; i++ {
		k[i] = uint64(testutil.RandomInt63())
	}
	return k
}

type keySlice []uint64

func (p keySlice) Len() int           { return len(p) }
func (p keySlice) Less(i, j int) bool { return p[i] < p[j] }
func (p keySlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p keySlice) Sort()              { sort.Sort(p) }

func TestStoreRandom(t *testing.T) {
	testutil.InitRandGenerator(t.Logf)
	store := newStore()
	keys := randomKeys()

	for i := 0; i < len(keys); i++ {
		r := []interface{}{i}
		store.addEntry(keys[i], r)
	}
	if len(store.store) != len(keys) {
		t.Errorf("num stored entries: got %d, want %d", len(store.store), len(keys))
	}
	for i := 0; i < len(keys); i++ {
		// Each call to removeEntries will remove an unknown number of entries
		// depending on the original randomised value of the ints.
		store.removeEntriesTo(keys[i])
	}
	if len(store.store) != 0 {
		t.Errorf("store is not empty: %d", len(store.store))
	}
}

func TestStoreOrdered(t *testing.T) {
	testutil.InitRandGenerator(t.Logf)
	store := newStore()
	keys := randomKeys()

	for i := 0; i < len(keys); i++ {
		r := []interface{}{i}
		store.addEntry(keys[i], r)
	}
	if len(store.store) != len(keys) {
		t.Errorf("num stored entries: got %d, want %d", len(store.store), len(keys))
	}

	(keySlice(keys)).Sort()
	l := len(keys)
	for i := 0; i < len(keys); i++ {
		store.removeEntriesTo(keys[i])
		l--
		if len(store.store) != l {
			t.Errorf("failed to remove a single item(%d): %d != %d", keys[i], len(store.store), l)
		}
	}
	if len(store.store) != 0 {
		t.Errorf("store is not empty: %d", len(store.store))
	}
}

func TestStoreWaitForEntry(t *testing.T) {
	store := newStore()
	store.addEntry(1, []interface{}{"1"})
	r := store.waitForEntry(1)
	if r[0].(string) != "1" {
		t.Errorf("Got: %q, Want: %q", r[0], "1")
	}
	ch := make(chan string)
	go func(ch chan string) {
		r := store.waitForEntry(2)
		ch <- r[0].(string)
	}(ch)
	store.addEntry(2, []interface{}{"2"})
	if result := <-ch; result != "2" {
		t.Errorf("Got: %q, Want: %q", r[0], "2")
	}
}

func TestStoreWaitForEntryRandom(t *testing.T) {
	testutil.InitRandGenerator(t.Logf)
	store := newStore()
	keys := randomKeys()
	var wg sync.WaitGroup
	for _, k := range keys {
		wg.Add(1)
		go func(t *testing.T, id uint64) {
			r := store.waitForEntry(id)
			if r[0].(uint64) != id {
				t.Errorf("Got: %d, Want: %d", r[0].(uint64), id)
			}
			wg.Done()
		}(t, k)
	}
	(keySlice(keys)).Sort()
	for _, k := range keys {
		store.addEntry(k, []interface{}{k})
	}
	wg.Wait()
}

func TestStoreWaitForRemovedEntry(t *testing.T) {
	testutil.InitRandGenerator(t.Logf)
	store := newStore()
	keys := randomKeys()
	var wg sync.WaitGroup
	for _, k := range keys {
		wg.Add(1)
		go func(t *testing.T, id uint64) {
			if r := store.waitForEntry(id); r != nil {
				t.Errorf("Got %v, want nil", r)
			}
			wg.Done()
		}(t, k)
	}
	(keySlice(keys)).Sort()
	for _, k := range keys {
		store.removeEntriesTo(k)
	}
	wg.Wait()
}

func TestStoreWaitForOldEntry(t *testing.T) {
	store := newStore()
	store.addEntry(1, []interface{}{"result"})
	store.removeEntriesTo(1)
	if got := store.waitForEntry(1); got != nil {
		t.Errorf("Got %T=%v, want nil", got, got)
	}
}
