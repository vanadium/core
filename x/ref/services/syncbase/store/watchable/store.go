// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package watchable provides a Syncbase-specific store.Store wrapper that
// provides versioned storage for specified prefixes and maintains a watchable
// log of operations performed on versioned records. This log forms the basis
// for the implementation of client-facing watch as well as the sync module's
// internal watching of store updates.
//
// LogEntry records are stored chronologically, using keys of the form
// "$log:<seq>". Sequence numbers are zero-padded to ensure that the
// lexicographic order matches the numeric order.
//
// Version number records are stored using keys of the form "$version:<key>",
// where <key> is the client-specified key.
package watchable

import (
	"fmt"
	"strings"
	"sync"
	"time"

	pubutil "v.io/v23/syncbase/util"
	"v.io/x/ref/services/syncbase/store"
)

// Options configures a Store.
type Options struct {
	// Key prefixes to version and log. If nil, all keys are managed.
	ManagedPrefixes []string
}

// Clock is an interface to a generic clock.
type Clock interface {
	// Now returns the current time.
	Now() (time.Time, error)
}

// Wrap returns a *Store that wraps the given store.Store.
func Wrap(st store.Store, clock Clock, opts *Options) (*Store, error) {
	seq, err := getNextLogSeq(st)
	if err != nil {
		return nil, err
	}
	return &Store{
		ist:     st,
		watcher: newWatcher(),
		opts:    opts,
		seq:     seq,
		Clock:   clock,
	}, nil
}

type Store struct {
	ist     store.Store
	watcher *watcher
	opts    *Options
	mu      sync.Mutex // held during transaction commits; protects seq
	seq     uint64     // the next sequence number to be used for a new commit
	// TODO(razvanm): make the clock private. The clock is used only by the
	// addSyncgroupLogRec function from the vsync package.
	Clock Clock // used to provide write timestamps
}

var _ store.Store = (*Store)(nil)

// Close implements the store.Store interface.
func (st *Store) Close() error {
	st.watcher.close()
	return st.ist.Close()
}

// Get implements the store.StoreReader interface.
func (st *Store) Get(key, valbuf []byte) ([]byte, error) {
	if !st.managesKey(key) {
		return st.ist.Get(key, valbuf)
	}
	sn := newSnapshot(st)
	defer sn.Abort()
	return sn.Get(key, valbuf)
}

// Scan implements the store.StoreReader interface.
func (st *Store) Scan(start, limit []byte) store.Stream {
	if !st.managesRange(start, limit) {
		return st.ist.Scan(start, limit)
	}
	// TODO(sadovsky): Close snapshot once stream is finished or canceled.
	return newSnapshot(st).Scan(start, limit)
}

// Put implements the store.StoreWriter interface.
func (st *Store) Put(key, value []byte) error {
	// Use watchable.Store transaction so this op gets logged.
	return store.RunInTransaction(st, func(tx store.Transaction) error {
		return tx.Put(key, value)
	})
}

// Delete implements the store.StoreWriter interface.
func (st *Store) Delete(key []byte) error {
	// Use watchable.Store transaction so this op gets logged.
	return store.RunInTransaction(st, func(tx store.Transaction) error {
		return tx.Delete(key)
	})
}

// NewTransaction implements the store.Store interface.
func (st *Store) NewTransaction() store.Transaction {
	return newTransaction(st)
}

// NewWatchableTransaction implements the Store interface.
func (st *Store) NewWatchableTransaction() *Transaction {
	return newTransaction(st)
}

// NewSnapshot implements the store.Store interface.
func (st *Store) NewSnapshot() store.Snapshot {
	return newSnapshot(st)
}

// GetOptions returns the options configured on a watchable.Store.
// TODO(rdaoud): expose watchable store through an interface and change this
// function to be a method on the store.
func GetOptions(st store.Store) (*Options, error) {
	wst := st.(*Store)
	return wst.opts, nil
}

////////////////////////////////////////
// Internal helpers

func (st *Store) managesKey(key []byte) bool {
	if st.opts.ManagedPrefixes == nil {
		return true
	}
	ikey := string(key)
	// TODO(sadovsky): Optimize, e.g. use binary search (here and below).
	for _, p := range st.opts.ManagedPrefixes {
		if strings.HasPrefix(ikey, p) {
			return true
		}
	}
	return false
}

func (st *Store) managesRange(start, limit []byte) bool {
	if st.opts.ManagedPrefixes == nil {
		return true
	}
	istart, ilimit := string(start), string(limit)
	for _, p := range st.opts.ManagedPrefixes {
		pstart, plimit := pubutil.PrefixRangeStart(p), pubutil.PrefixRangeLimit(p)
		if pstart <= istart && ilimit <= plimit {
			return true
		}
		if !(plimit <= istart || ilimit <= pstart) {
			// If this happens, there's a bug in the Syncbase server implementation.
			panic(fmt.Sprintf("partial overlap: %q %q %q", p, start, limit))
		}
	}
	return false
}
