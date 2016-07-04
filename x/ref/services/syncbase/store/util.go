// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

type SnapshotSpecImpl struct{}

func (s *SnapshotSpecImpl) __snapshotSpec() {}

// TODO(razvanm): Another copy of this is inside in store/watchable/.
// TODO(sadovsky): Move this to model.go and make it an argument to
// Store.NewTransaction.
type TransactionOptions struct {
	NumAttempts int // number of attempts; only used by RunInTransaction
}

// RunWithSnapshot runs the given fn with a snapshot that is aborted afterwards.
func RunWithSnapshot(st Store, fn func(sntx SnapshotOrTransaction) error) error {
	sn := st.NewSnapshot()
	defer sn.Abort()
	return fn(sn)
}

// RunInTransaction runs the given fn in a transaction, managing retries and
// commit/abort.
func RunInTransaction(st Store, fn func(tx Transaction) error) error {
	// TODO(rogulenko): Change the default number of attempts to 3. Currently,
	// some storage engine tests fail when the number of attempts is that low.
	return RunInTransactionWithOpts(st, &TransactionOptions{NumAttempts: 100}, fn)
}

// RunInTransactionWithOpts runs the given fn in a transaction, managing retries
// and commit/abort.
func RunInTransactionWithOpts(st Store, opts *TransactionOptions, fn func(tx Transaction) error) error {
	var err error
	for i := 0; i < opts.NumAttempts; i++ {
		// TODO(sadovsky): Should NewTransaction return an error? If not, how will
		// we deal with RPC errors when talking to remote storage engines? (Note,
		// client-side BeginBatch returns an error.)
		tx := st.NewTransaction()
		if err = fn(tx); err != nil {
			tx.Abort()
			return err
		}
		// TODO(sadovsky): Commit() can fail for a number of reasons, e.g. RPC
		// failure or ErrConcurrentTransaction. Depending on the cause of failure,
		// it may be desirable to retry the Commit() and/or to call Abort().
		if err = tx.Commit(); verror.ErrorID(err) != ErrConcurrentTransaction.ID {
			return err
		}
	}
	return err
}

// CopyBytes copies elements from a source slice into a destination slice.
// The returned slice may be a sub-slice of dst if dst was large enough to hold
// src. Otherwise, a newly allocated slice will be returned.
// TODO(rogulenko): add some tests.
func CopyBytes(dst, src []byte) []byte {
	if cap(dst) < len(src) {
		newlen := cap(dst)*2 + 2
		if newlen < len(src) {
			newlen = len(src)
		}
		dst = make([]byte, newlen)
	}
	dst = dst[:len(src)]
	copy(dst, src)
	return dst
}

// ConvertError returns a copy of the verror, appending the current stack to it.
func ConvertError(err error) error {
	return verror.Convert(verror.IDAction{}, nil, err)
}

// Get does st.Get(k, v) and wraps the returned error.
func Get(ctx *context.T, st StoreReader, k string, v interface{}) error {
	bytes, err := st.Get([]byte(k), nil)
	if err != nil {
		if verror.ErrorID(err) == ErrUnknownKey.ID {
			return verror.New(verror.ErrNoExist, ctx, k)
		}
		return verror.New(verror.ErrInternal, ctx, err)
	}
	if err = vom.Decode(bytes, v); err != nil {
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return nil
}

// Put does st.Put(k, v) and wraps the returned error.
func Put(ctx *context.T, st StoreWriter, k string, v interface{}) error {
	bytes, err := vom.Encode(v)
	if err != nil {
		return verror.New(verror.ErrInternal, ctx, err)
	}
	if err = st.Put([]byte(k), bytes); err != nil {
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return nil
}

// Delete does st.Delete(k) and wraps the returned error.
func Delete(ctx *context.T, st StoreWriter, k string) error {
	if err := st.Delete([]byte(k)); err != nil {
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return nil
}

// Exists returns true if the key exists in the store.
// TODO(rdaoud): for now it only bypasses the Get's VOM decode step.  It should
// be optimized further by adding a st.Exists(k) API and let each implementation
// do its best to reduce data fetching in its key lookup.
func Exists(ctx *context.T, st StoreReader, k string) (bool, error) {
	_, err := st.Get([]byte(k), nil)
	if err != nil {
		if verror.ErrorID(err) == ErrUnknownKey.ID {
			return false, nil
		}
		return false, verror.New(verror.ErrInternal, ctx, err)
	}
	return true, nil
}
