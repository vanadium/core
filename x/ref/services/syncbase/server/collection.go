// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
)

// collectionReq is a per-request object that handles Collection RPCs.
type collectionReq struct {
	id wire.Id
	d  *database
}

var (
	_ wire.CollectionServerMethods = (*collectionReq)(nil)
)

////////////////////////////////////////
// RPC methods

func (c *collectionReq) Create(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle, perms access.Permissions) error {
	if err := common.ValidatePerms(ctx, perms, wire.AllCollectionTags); err != nil {
		return err
	}
	impl := func(ts *transactionState) error {
		tx := ts.tx
		// Check DatabaseData perms.
		dData := &DatabaseData{}
		if err := util.GetWithAuth(ctx, call, tx, c.d.stKey(), dData); err != nil {
			return err
		}
		// Check implicit perms derived from blessing pattern in id.
		implicitPerms, err := common.CheckImplicitPerms(ctx, call, c.id, wire.AllCollectionTags)
		if err != nil {
			return err
		}
		// Check for "collection already exists".
		if err := store.Get(ctx, tx, c.permsKey(), &interfaces.CollectionPerms{}); verror.ErrorID(err) != verror.ErrNoExist.ID {
			if err != nil {
				return err
			}
			return verror.New(verror.ErrExist, ctx, c.id)
		}
		// Collection Create is equivalent to changing permissions from implicit to
		// explicit. Note, the creator implicitly has all permissions (RWA), so it
		// is legal to write data and drop the write permission in the same batch.
		ts.MarkPermsChanged(c.id, implicitPerms, perms)
		// Write new CollectionPerms.
		storedPerms := interfaces.CollectionPerms(perms)
		return store.Put(ctx, tx, c.permsKey(), &storedPerms)
	}
	return c.d.runInExistingBatchOrNewTransaction(ctx, bh, impl)
}

// TODO(ivanpi): Decouple collection key prefix from collection id to allow
// collection data deletion to be deferred, making deletion faster (reference
// removal).
func (c *collectionReq) Destroy(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) error {
	impl := func(ts *transactionState) error {
		tx := ts.tx
		// Read CollectionPerms.
		if err := util.GetWithAuth(ctx, call, tx, c.permsKey(), &interfaces.CollectionPerms{}); err != nil {
			if verror.ErrorID(err) == verror.ErrNoExist.ID {
				return nil // delete is idempotent
			}
			return err
		}

		// TODO(ivanpi): Check that no syncgroup includes the collection being
		// destroyed. Also check that all collections exist when creating a
		// syncgroup. Refactor with common part of DeleteRange.

		// Reset all tracked changes to the collection. See comment on the method
		// for more details.
		ts.ResetCollectionChanges(c.id)
		// Delete all data rows.
		it := tx.Scan(common.ScanPrefixArgs(common.JoinKeyParts(common.RowPrefix, c.stKeyPart()), ""))
		var key []byte
		for it.Advance() {
			key = it.Key(key)
			if err := tx.Delete(key); err != nil {
				return verror.New(verror.ErrInternal, ctx, err)
			}
		}
		if err := it.Err(); err != nil {
			return verror.New(verror.ErrInternal, ctx, err)
		}
		// Delete CollectionPerms.
		return store.Delete(ctx, tx, c.permsKey())
	}
	return c.d.runInExistingBatchOrNewTransaction(ctx, bh, impl)
}

func (c *collectionReq) Exists(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) (bool, error) {
	impl := func(sntx store.SnapshotOrTransaction) error {
		return util.GetWithAuth(ctx, call, c.d.st, c.permsKey(), &interfaces.CollectionPerms{})
	}
	return util.ErrorToExists(c.d.runWithExistingBatchOrNewSnapshot(ctx, bh, impl))
}

func (c *collectionReq) GetPermissions(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) (perms access.Permissions, err error) {
	var res interfaces.CollectionPerms
	impl := func(sntx store.SnapshotOrTransaction) error {
		return util.GetWithAuth(ctx, call, sntx, c.permsKey(), &res)
	}
	if err := c.d.runWithExistingBatchOrNewSnapshot(ctx, bh, impl); err != nil {
		return nil, err
	}
	return access.Permissions(res), nil
}

func (c *collectionReq) SetPermissions(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle, newPerms access.Permissions) error {
	if err := common.ValidatePerms(ctx, newPerms, wire.AllCollectionTags); err != nil {
		return err
	}
	impl := func(ts *transactionState) error {
		tx := ts.tx
		currentPerms, err := c.checkAccess(ctx, call, tx)
		if err != nil {
			return err
		}
		ts.MarkPermsChanged(c.id, currentPerms, newPerms)
		storedPerms := interfaces.CollectionPerms(newPerms)
		return store.Put(ctx, tx, c.permsKey(), &storedPerms)
	}
	return c.d.runInExistingBatchOrNewTransaction(ctx, bh, impl)
}

func (c *collectionReq) DeleteRange(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle, start, limit []byte) error {
	impl := func(ts *transactionState) error {
		tx := ts.tx
		// Check for collection-level access before doing a scan.
		currentPerms, err := c.checkAccess(ctx, call, tx)
		if err != nil {
			return err
		}
		ts.MarkDataChanged(c.id, currentPerms)
		it := tx.Scan(common.ScanRangeArgs(common.JoinKeyParts(common.RowPrefix, c.stKeyPart()), string(start), string(limit)))
		key := []byte{}
		for it.Advance() {
			key = it.Key(key)
			// Delete the key-value pair.
			if err := tx.Delete(key); err != nil {
				return verror.New(verror.ErrInternal, ctx, err)
			}
		}
		if err := it.Err(); err != nil {
			return verror.New(verror.ErrInternal, ctx, err)
		}
		return nil
	}
	return c.d.runInExistingBatchOrNewTransaction(ctx, bh, impl)
}

func (c *collectionReq) Scan(ctx *context.T, call wire.CollectionScanServerCall, bh wire.BatchHandle, start, limit []byte) error {
	impl := func(sntx store.SnapshotOrTransaction) error {
		// Check for collection-level access before doing a scan.
		if _, err := c.checkAccess(ctx, call, sntx); err != nil {
			return err
		}
		it := sntx.Scan(common.ScanRangeArgs(common.JoinKeyParts(common.RowPrefix, c.stKeyPart()), string(start), string(limit)))
		sender := call.SendStream()
		key, value := []byte{}, []byte{}
		for it.Advance() {
			key, value = it.Key(key), it.Value(value)
			// See comment in util/constants.go for why we use SplitNKeyParts.
			parts := common.SplitNKeyParts(string(key), 3)
			externalKey := parts[2]
			var rawBytes *vom.RawBytes
			if err := vom.Decode(value, &rawBytes); err != nil {
				// TODO(m3b): Is this the correct behaviour on an encoding error here?
				it.Cancel()
				return err
			}
			if err := sender.Send(wire.KeyValue{Key: externalKey, Value: rawBytes}); err != nil {
				it.Cancel()
				return err
			}
		}
		if err := it.Err(); err != nil {
			return verror.New(verror.ErrInternal, ctx, err)
		}
		return nil
	}
	return c.d.runWithExistingBatchOrNewSnapshot(ctx, bh, impl)
}

func (c *collectionReq) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, matcher *glob.Element) error {
	impl := func(sntx store.SnapshotOrTransaction) error {
		// Check perms.
		if _, err := c.checkAccess(ctx, call, sntx); err != nil {
			return err
		}
		return util.GlobChildren(ctx, call, matcher, sntx, common.JoinKeyParts(common.RowPrefix, c.stKeyPart()))
	}
	return store.RunWithSnapshot(c.d.st, impl)
}

////////////////////////////////////////
// Internal helpers

func (c *collectionReq) permsKey() string {
	// TODO(rdaoud,ivanpi): The third empty key component is added to ensure a
	// sync prefix matches only the exact collection id. Make sync handling of
	// collections more explicit and clean up this hack.
	return common.JoinKeyParts(common.CollectionPermsPrefix, c.stKeyPart(), "")
}

func (c *collectionReq) stKeyPart() string {
	return pubutil.EncodeId(c.id)
}

// checkAccess checks that this collection exists in the database, and performs
// an authorization check on the collection ACL. It should be called in the same
// transaction as any store modification to ensure that concurrent ACL changes
// invalidate the modification.
// TODO(rogulenko): Revisit this behavior. Eventually we'll want the
// collection-level access check to be a check for "Resolve", i.e. also check
// access to service and database.
func (c *collectionReq) checkAccess(ctx *context.T, call rpc.ServerCall, sntx store.SnapshotOrTransaction) (access.Permissions, error) {
	collectionPerms := &interfaces.CollectionPerms{}
	if err := util.GetWithAuth(ctx, call, sntx, c.permsKey(), collectionPerms); err != nil {
		return nil, err
	}
	return collectionPerms.GetPerms(), nil
}
