// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

// TODO(sadovsky): Avoid copying back and forth between []byte's and strings.
// We should probably convert incoming strings to []byte's as early as possible,
// and deal exclusively in []byte's internally.
// TODO(rdaoud): I propose we standardize on key and version being strings and
// the value being []byte within Syncbase.  We define invalid characters in the
// key space (and reserve "$" and ":").  The lower storage engine layers are
// free to map that to what they need internally ([]byte or string).

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
)

var (
	rng     *rand.Rand = rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	rngLock sync.Mutex
)

// NewVersion returns a new version for a store entry mutation.
func NewVersion() []byte {
	// TODO(rdaoud): revisit the number of bits: should we use 128 bits?
	// Note: the version has to be unique per object key, not on its own.
	// TODO(rdaoud): move sync's rand64() to a general Syncbase spot and
	// reuse it here.
	rngLock.Lock()
	num := rng.Int63()
	rngLock.Unlock()

	return []byte(fmt.Sprintf("%x", num))
}

func makeVersionKey(key []byte) []byte {
	return []byte(join(common.VersionPrefix, string(key)))
}

func makeAtVersionKey(key, version []byte) []byte {
	return []byte(join(string(key), string(version)))
}

func getVersion(st store.StoreReader, key []byte) ([]byte, error) {
	return st.Get(makeVersionKey(key), nil)
}

func getAtVersion(st store.StoreReader, key, valbuf, version []byte) ([]byte, error) {
	return st.Get(makeAtVersionKey(key, version), valbuf)
}

func getVersioned(sntx store.SnapshotOrTransaction, key, valbuf []byte) ([]byte, error) {
	version, err := getVersion(sntx, key)
	if err != nil {
		return valbuf, err
	}
	return getAtVersion(sntx, key, valbuf, version)
}

func putVersioned(tx store.Transaction, key, value []byte) ([]byte, error) {
	version := NewVersion()
	if err := tx.Put(makeVersionKey(key), version); err != nil {
		return nil, err
	}
	if err := tx.Put(makeAtVersionKey(key, version), value); err != nil {
		return nil, err
	}
	return version, nil
}

func deleteVersioned(tx store.Transaction, key []byte) error {
	return tx.Delete(makeVersionKey(key))
}

func join(parts ...string) string {
	return common.JoinKeyParts(parts...)
}

func convertError(err error) error {
	return verror.Convert(verror.IDAction{}, nil, err)
}

// TODO(razvanm): This is copied from store/util.go.
// TODO(sadovsky): Move this to model.go and make it an argument to
// Store.NewTransaction.
type TransactionOptions struct {
	NumAttempts int // number of attempts; only used by RunInTransaction
}

// RunInTransaction runs the given fn in a transaction, managing retries and
// commit/abort.
func RunInTransaction(st *Store, fn func(tx *Transaction) error) error {
	// TODO(rogulenko): Change the default number of attempts to 3. Currently,
	// some storage engine tests fail when the number of attempts is that low.
	return runInTransactionWithOpts(st, &TransactionOptions{NumAttempts: 100}, fn)
}

// RunInTransactionWithOpts runs the given fn in a transaction, managing retries
// and commit/abort.
func runInTransactionWithOpts(st *Store, opts *TransactionOptions, fn func(tx *Transaction) error) error {
	var err error
	for i := 0; i < opts.NumAttempts; i++ {
		// TODO(sadovsky): Should NewTransaction return an error? If not, how will
		// we deal with RPC errors when talking to remote storage engines? (Note,
		// client-side BeginBatch returns an error.)
		tx := st.NewWatchableTransaction()
		if err = fn(tx); err != nil {
			tx.Abort()
			return err
		}
		// TODO(sadovsky): Commit() can fail for a number of reasons, e.g. RPC
		// failure or ErrConcurrentTransaction. Depending on the cause of failure,
		// it may be desirable to retry the Commit() and/or to call Abort().
		if err = tx.Commit(); verror.ErrorID(err) != store.ErrConcurrentTransaction.ID {
			return err
		}
	}
	return err
}
