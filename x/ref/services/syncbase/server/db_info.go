// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

// This file defines internal methods for manipulating dbInfo. None of these
// methods perform authorization checks.
//
// These methods are needed because information about a database is spread
// across two storage engines: existence of a database is tracked in the
// service-level storage engine, while permissions for a database are tracked in
// the database's storage engine.

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
)

func dbInfoStKey(dbId wire.Id) string {
	return common.JoinKeyParts(common.DbInfoPrefix, pubutil.EncodeId(dbId))
}

// getDbInfo reads data from the storage engine.
func getDbInfo(ctx *context.T, sntx store.SnapshotOrTransaction, dbId wire.Id) (*DbInfo, error) {
	info := &DbInfo{}
	if err := store.Get(ctx, sntx, dbInfoStKey(dbId), info); err != nil {
		return nil, err
	}
	return info, nil
}

// putDbInfo writes data to the storage engine.
func putDbInfo(ctx *context.T, tx store.Transaction, info *DbInfo) error {
	return store.Put(ctx, tx, dbInfoStKey(info.Id), info)
}

// delDbInfo deletes data from the storage engine.
func delDbInfo(ctx *context.T, stw store.StoreWriter, dbId wire.Id) error {
	return store.Delete(ctx, stw, dbInfoStKey(dbId))
}
