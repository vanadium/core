// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
)

// rowReq is a per-request object that handles Row RPCs.
type rowReq struct {
	key string
	c   *collectionReq
}

var (
	_ wire.RowServerMethods = (*rowReq)(nil)
)

////////////////////////////////////////
// RPC methods

func (r *rowReq) Exists(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) (bool, error) {
	_, err := r.Get(ctx, call, bh)
	return util.ErrorToExists(err)
}

func (r *rowReq) Get(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) (*vom.RawBytes, error) {
	var res *vom.RawBytes
	impl := func(sntx store.SnapshotOrTransaction) (err error) {
		res, err = r.get(ctx, call, sntx)
		return err
	}
	if err := r.c.d.runWithExistingBatchOrNewSnapshot(ctx, bh, impl); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *rowReq) Put(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle, value *vom.RawBytes) error {
	impl := func(ts *transactionState) error {
		return r.put(ctx, call, ts, value)
	}
	return r.c.d.runInExistingBatchOrNewTransaction(ctx, bh, impl)
}

func (r *rowReq) Delete(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) error {
	impl := func(ts *transactionState) error {
		return r.delete(ctx, call, ts)
	}
	return r.c.d.runInExistingBatchOrNewTransaction(ctx, bh, impl)
}

////////////////////////////////////////
// Internal helpers

func (r *rowReq) stKey() string {
	return common.JoinKeyParts(common.RowPrefix, r.stKeyPart())
}

func (r *rowReq) stKeyPart() string {
	return common.JoinKeyParts(r.c.stKeyPart(), r.key)
}

// get reads data from the storage engine.
// Performs authorization check.
func (r *rowReq) get(ctx *context.T, call rpc.ServerCall, sntx store.SnapshotOrTransaction) (*vom.RawBytes, error) {
	if _, err := r.c.checkAccess(ctx, call, sntx); err != nil {
		return nil, err
	}
	value, err := sntx.Get([]byte(r.stKey()), nil)
	var valueAsRawBytes vom.RawBytes
	if err == nil {
		err = vom.Decode(value, &valueAsRawBytes)
	}
	if err != nil {
		if verror.ErrorID(err) == store.ErrUnknownKey.ID {
			return nil, verror.New(verror.ErrNoExist, ctx, r.stKey())
		}
		return nil, verror.New(verror.ErrInternal, ctx, err)
	}
	return &valueAsRawBytes, nil
}

// put writes data to the storage engine.
// Performs authorization check.
func (r *rowReq) put(ctx *context.T, call rpc.ServerCall, ts *transactionState, value *vom.RawBytes) error {
	tx := ts.tx
	currentPerms, err := r.c.checkAccess(ctx, call, tx)
	if err != nil {
		return err
	}
	ts.MarkDataChanged(r.c.id, currentPerms)
	valueAsBytes, err := vom.Encode(value)
	if err != nil {
		return err
	}
	if err = tx.Put([]byte(r.stKey()), valueAsBytes); err != nil {
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return nil
}

// delete deletes data from the storage engine.
// Performs authorization check.
func (r *rowReq) delete(ctx *context.T, call rpc.ServerCall, ts *transactionState) error {
	currentPerms, err := r.c.checkAccess(ctx, call, ts.tx)
	if err != nil {
		return err
	}
	ts.MarkDataChanged(r.c.id, currentPerms)
	if err := ts.tx.Delete([]byte(r.stKey())); err != nil {
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return nil
}
