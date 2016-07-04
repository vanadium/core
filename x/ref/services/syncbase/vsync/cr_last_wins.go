// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"v.io/v23/context"
	"v.io/x/lib/vlog"
)

// resolveViaTimestamp takes a map of updated objects and resolves each object
// independently based on write timestamps.
// NOTE: if an object is a part of the input map but not under conflict, it is
// ignored.
func (iSt *initiationState) resolveViaTimestamp(ctx *context.T, objConfMap map[string]*objConflictState) error {
	for obj, conflictState := range objConfMap {
		if !conflictState.isConflict || conflictState.res != nil {
			continue
		}
		var err error
		conflictState.res, err = resolveObjConflict(ctx, iSt, obj, conflictState.oldHead, conflictState.newHead, conflictState.ancestor)
		if err != nil {
			return err
		}
	}
	return nil
}

// resolveObjConflict resolves a conflict for an object given its ID and the 3
// versions that express the conflict: the object's local version, its remote
// version (from the device contacted), and the closest common ancestor (see
// dag.go on how the ancestor is chosen). The function resolves conflicts using
// the timestamps of the conflicting mutations. It picks a mutation with the
// larger timestamp, i.e. the most recent update.  If the timestamps are equal,
// mutation version numbers as a tie-breaker, picking the mutation with the
// it uses the larger version.  Instead of creating a new version that resolves
// the conflict, we pick an existing version as the conflict resolution.
func resolveObjConflict(ctx *context.T, iSt *initiationState, oid, local, remote, ancestor string) (*conflictResolution, error) {
	// Fetch the log records of the 3 object versions.
	versions := []string{local, remote}
	lrecs, err := iSt.getLogRecsBatch(ctx, oid, versions)
	if err != nil {
		return nil, err
	}

	// The local and remote records must exist.
	locRec, remRec := lrecs[0], lrecs[1]
	if locRec == nil || remRec == nil {
		vlog.Fatalf("sync: resolveObjConflict: oid %s: invalid local (%s: %v) or remote recs (%s: %v)",
			oid, local, locRec, remote, remRec)
	}

	var res conflictResolution
	res.batchId = NoBatchId
	switch {
	case locRec.Metadata.UpdTime.After(remRec.Metadata.UpdTime):
		res.ty = pickLocal
	case locRec.Metadata.UpdTime.Before(remRec.Metadata.UpdTime):
		res.ty = pickRemote
	case locRec.Metadata.CurVers > remRec.Metadata.CurVers:
		res.ty = pickLocal
	case locRec.Metadata.CurVers < remRec.Metadata.CurVers:
		res.ty = pickRemote
	default:
		vlog.Fatalf("sync: resolveObjConflictByTime: local and remote update times and versions are the same, local %v remote %v", local, remote)
	}
	return &res, nil
}
