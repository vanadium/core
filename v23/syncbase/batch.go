// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

func newBatch(parentFullName string, id wire.Id, batchHandle wire.BatchHandle) *batch {
	if batchHandle == "" {
		panic("batch must have non-empty handle")
	}
	return &batch{databaseBatch: *newDatabaseBatch(parentFullName, id, batchHandle)}
}

type batch struct {
	databaseBatch
}

var _ BatchDatabase = (*batch)(nil)

// Commit implements BatchDatabase.Commit.
func (b *batch) Commit(ctx *context.T) error {
	return b.c.Commit(ctx, b.bh)
}

// Abort implements BatchDatabase.Abort.
func (b *batch) Abort(ctx *context.T) error {
	return b.c.Abort(ctx, b.bh)
}

// RunInBatch runs the given fn in a batch, managing retries and commit/abort.
// Writable batches are committed, retrying if commit fails. Readonly batches
// are aborted.
func RunInBatch(ctx *context.T, d Database, opts wire.BatchOptions, fn func(b BatchDatabase) error) error {
	attemptInBatch := func() error {
		b, err := d.BeginBatch(ctx, opts)
		if err != nil {
			return err
		}
		// Use defer for Abort to make sure it gets called in case fn panics.
		commitCalled := false
		defer func() {
			if !commitCalled {
				b.Abort(ctx)
			}
		}()
		if err = fn(b); err != nil {
			return err
		}
		// A readonly batch should be Aborted; Commit would fail.
		if opts.ReadOnly {
			return nil
		}
		// Commit is about to be called, do not call Abort.
		commitCalled = true
		return b.Commit(ctx)
	}
	var err error
	// TODO(sadovsky): Make the number of attempts configurable.
	for i := 0; i < 3; i++ {
		// TODO(sadovsky): Commit() can fail for a number of reasons, e.g. RPC
		// failure or ErrConcurrentTransaction. Depending on the cause of failure,
		// it may be desirable to retry the Commit() and/or to call Abort().
		if err = attemptInBatch(); verror.ErrorID(err) != wire.ErrConcurrentBatch.ID {
			return err
		}
	}
	return err
}
