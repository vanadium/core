// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"container/heap"
	"sync"
)

const (
	// TODO(cnicolaou): what are good initial values? Large servers want
	// large values, most won't.
	initialResults           = 1000
	initialOutOfOrderResults = 100
)

type results []interface{}

// Implement heap.Interface to maintain an ordered min-heap of uint64s.
type intHeap []uint64

func (h intHeap) Len() int           { return len(h) }
func (h intHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h intHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *intHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(uint64))
}

func (h *intHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// resultStore is used to store the results of previously exited RPCs
// until the client indicates that it has received those results and hence
// the server no longer needs to store them. Store entries are added
// one at a time, but the client indicates that it has received entries up to
// given value and that all entries with key lower than that can be deleted.
// Retrieving values is complicated by the fact that requests may arrive
// out of order and hence one RPC may have to wait for another to complete
// in order to access its stored results. A separate map of channels is
// used to implement this synchronization.
// TODO(cnicolaou): Servers protect themselves from badly behaved clients by
// refusing to allocate beyond a certain number of results.
type resultsStore struct {
	sync.Mutex
	store map[uint64]results
	chans map[uint64]chan struct{}
	keys  intHeap
	// TODO(cnicolaou): Should addEntry/waitForEntry return an error when
	// the calls do not match the frontier?
	frontier uint64 // results with index less than this have been removed.
}

func newStore() *resultsStore {
	r := &resultsStore{
		store: make(map[uint64]results, initialResults),
		chans: make(map[uint64]chan struct{}, initialOutOfOrderResults),
	}
	heap.Init(&r.keys)
	return r
}

func (rs *resultsStore) addEntry(key uint64, data results) {
	rs.Lock()
	if _, present := rs.store[key]; !present && rs.frontier <= key {
		rs.store[key] = data
		heap.Push(&rs.keys, key)
	}
	if ch, present := rs.chans[key]; present {
		close(ch)
		delete(rs.chans, key)
	}
	rs.Unlock()
}

func (rs *resultsStore) removeEntriesTo(to uint64) {
	rs.Lock()
	if rs.frontier > to {
		rs.Unlock()
		return
	}
	rs.frontier = to + 1
	for rs.keys.Len() > 0 && to >= rs.keys[0] {
		k := heap.Pop(&rs.keys).(uint64)
		delete(rs.store, k)
		if ch, present := rs.chans[k]; present {
			close(ch)
			delete(rs.chans, k)
		}
	}
	rs.Unlock()
}

func (rs *resultsStore) waitForEntry(key uint64) results {
	rs.Lock()
	if r, present := rs.store[key]; present {
		rs.Unlock()
		return r
	}
	if key < rs.frontier {
		rs.Unlock()
		return nil
	}
	// entry is not present, need to wait for it.
	ch, present := rs.chans[key]
	if !present {
		heap.Push(&rs.keys, key)
		ch = make(chan struct{}, 1)
		rs.chans[key] = ch
	}
	rs.Unlock()
	<-ch
	rs.Lock()
	defer rs.Unlock()
	delete(rs.chans, key) // Allow the channel to be GC'ed.
	return rs.store[key]  // This may be nil if the entry has been removed
}
