// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Sync utility functions

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store/watchable"
)

const (
	nanoPerSec = int64(1000000000)
)

// forEachDatabaseStore iterates over all Databases within the service and
// invokes the callback function on each. The callback returns a "done" flag
// to make forEachDatabaseStore() stop the iteration earlier; otherwise the
// function loops across all databases.
func (s *syncService) forEachDatabaseStore(ctx *context.T, callback func(wire.Id, *watchable.Store) bool) {
	// Get the databases and iterate over them.
	// TODO(rdaoud): use a "privileged call" parameter instead of nil (here and
	// elsewhere).
	dbIds, err := s.sv.DatabaseIds(ctx, nil)
	if err != nil {
		vlog.Errorf("sync: forEachDatabaseStore: cannot get all db ids: %v", err)
		return
	}
	// For each database, get its Store and invoke the callback.
	for _, id := range dbIds {
		db, err := s.sv.Database(ctx, nil, id)
		if err != nil {
			vlog.Errorf("sync: forEachDatabaseStore: cannot get db %v: %v", id, err)
			continue
		}
		if callback(id, db.St()) {
			return // done, early exit
		}
	}
}

// getDbStore gets the store handle to the database.
func (s *syncService) getDbStore(ctx *context.T, call rpc.ServerCall, dbId wire.Id) (*watchable.Store, error) {
	db, err := s.sv.Database(ctx, call, dbId)
	if err != nil {
		return nil, err
	}
	return db.St(), nil
}

// unixNanoToTime converts a Unix timestamp in nanoseconds to a Time object.
func unixNanoToTime(timestamp int64) time.Time {
	if timestamp < 0 {
		vlog.Fatalf("sync: unixNanoToTime: invalid timestamp %d", timestamp)
	}
	return time.Unix(timestamp/nanoPerSec, timestamp%nanoPerSec)
}

// toCollectionPrefixStr converts a collection id to a string of the form used
// for storing perms and row data in the underlying storage engine.
func toCollectionPrefixStr(c wire.Id) string {
	return common.JoinKeyParts(pubutil.EncodeId(c), "")
}

// toRowKey prepends RowPrefix to what is presumably a "<collection>:<row>"
// string, yielding a storage engine key for a row.
// TODO(sadovsky): Only used by CR code. Should go away once CR stores
// collection id and row key as separate fields in a "CollectionRow" struct.
func toRowKey(collectionRow string) string {
	return common.JoinKeyParts(common.RowPrefix, collectionRow)
}

////////////////////////////////////////////////////////////////////////////////
// Internal helpers for authorization during the two sync rounds, and during
// blob transfers.

// namesAuthorizer authorizes the remote peer only if it presents all the names
// in the expNames set.
type namesAuthorizer struct {
	expNames []string
}

// Authorize allows namesAuthorizer to implement the
// security.Authorizer interface.  It tests that the peer server
// has at least the blessings namesAuthorizer.expNames.
func (na namesAuthorizer) Authorize(ctx *context.T, securityCall security.Call) (err error) {
	var peerBlessingNames []string
	peerBlessingNames, _ = security.RemoteBlessingNames(ctx, securityCall)
	vlog.VI(4).Infof("sync: Authorize: names %v", peerBlessingNames)
	// Put the peerBlessingNames in a set, to make it easy to test whether
	// na.expectedBlessingNames is a subset.
	peerBlessingNamesMap := make(map[string]bool)
	for i := 0; i != len(peerBlessingNames); i++ {
		peerBlessingNamesMap[peerBlessingNames[i]] = true
	}
	// isSubset = na.expectedBlessingNames is_subset_of peerBlessingNames.
	isSubset := true
	for i := 0; i != len(na.expNames) && isSubset; i++ {
		isSubset = peerBlessingNamesMap[na.expNames[i]]
	}
	if !isSubset {
		err = verror.New(verror.ErrInternal, ctx, "server blessings changed")
	} else {
		vlog.VI(4).Infof("sync: Authorize: remote peer allowed")
	}
	return err
}
