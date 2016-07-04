// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package watchable contains helper functions for the Syncbase-specific
// operations. The equivalent for the Syncbase-agnostic operations sit in
// the store/watchable package.
package watchable

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// AddSyncgroupOp injects a syncgroup operation notification in the log entries
// that the transaction writes when it is committed.  It allows the syncgroup
// operations (create, join, leave, destroy) to notify the sync watcher of the
// change at its proper position in the timeline (the transaction commit).
// Note: this is an internal function used by sync, not part of the interface.
func AddSyncgroupOp(ctx *context.T, tx *watchable.Transaction, gid interfaces.GroupId, prefixes []string, remove bool) error {
	op := &SyncgroupOp{
		SgId:     gid,
		Prefixes: append([]string{}, prefixes...),
		Remove:   remove,
	}
	return watchable.AddOp(tx, op, nil)
}

// AddSyncSnapshotOp injects a sync snapshot operation notification in the log
// entries that the transaction writes when it is committed.  It allows the
// syncgroup create or join operations to notify the sync watcher of the
// current keys and their versions to use when initializing the sync metadata
// at the point in the timeline when these keys become syncable (at commit).
// Note: this is an internal function used by sync, not part of the interface.
func AddSyncSnapshotOp(ctx *context.T, tx *watchable.Transaction, key, version []byte) error {
	op := &SyncSnapshotOp{
		Key:     append([]byte{}, key...),
		Version: append([]byte{}, version...),
	}
	precond := func() error {
		if !watchable.ManagesKey(tx, key) {
			return verror.New(verror.ErrInternal, ctx, fmt.Sprintf("cannot create SyncSnapshotOp on unmanaged key: %s", string(key)))
		}
		return nil
	}
	return watchable.AddOp(tx, op, precond)
}

// AddDbStateChangeRequestOp injects a database state change request in the log
// entries that the transaction writes when it is committed. The sync watcher
// receives the request at the proper position in the timeline (the transaction
// commit) and makes appropriate updates to the db state causing the request to
// take effect.
// Note: This is an internal function used by server.database.
func AddDbStateChangeRequestOp(ctx *context.T, tx *watchable.Transaction, stateChangeType StateChange) error {
	op := &DbStateChangeRequestOp{
		RequestType: stateChangeType,
	}
	return watchable.AddOp(tx, op, nil)
}
