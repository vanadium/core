// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

// resolutionType represents how a conflict is resolved.
type resolutionType byte

const (
	pickLocal  resolutionType = iota // local update was chosen as the resolution.
	pickRemote                       // remote update was chosen as the resolution.
	createNew                        // new update was created as the resolution.
)

// conflictResolution represents the state of a conflict resolution.
type conflictResolution struct {
	batchId    uint64
	batchCount uint64
	ty         resolutionType
	rec        *LocalLogRec // Valid only if ty == createNew.
	val        []byte       // Valid only if ty == createNew.
}

// resolveConflicts resolves conflicts for updated objects. Conflicts may be
// resolved by adding new versions or picking either the local or the remote
// version.
func (iSt *initiationState) resolveConflicts(ctx *context.T) error {
	vlog.VI(2).Infof("sync: resolveConflicts: start")
	defer vlog.VI(2).Infof("sync: resolveConflicts: end")
	// Lookup schema for the database to figure out the CR policy set by the
	// application.
	schema, err := iSt.getDbSchema(ctx)
	if err != nil {
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			vlog.VI(2).Infof("sync: resolveConflicts: no schema found, resolving based on timestamp")
			return iSt.resolveViaTimestamp(ctx, iSt.updObjects)
		}
		vlog.Errorf("sync: resolveConflicts: error while fetching schema: %v", err)
		return err
	}

	vlog.VI(2).Infof("cr: resolveConflicts: schema found: %v", schema)

	// Categorize conflicts based on CR policy in schema.
	conflictsByType := iSt.groupConflictsByType(schema)

	// Handle type AppResolves.
	conflictsForApp := conflictsByType[wire.ResolverTypeAppResolves]
	groupedConflicts := iSt.groupConflicts(ctx, conflictsForApp)
	if err := iSt.resolveViaApp(ctx, groupedConflicts); err != nil {
		return err
	}

	// Handle rest of the conflicts of type LastWins.
	return iSt.resolveViaTimestamp(ctx, conflictsByType[wire.ResolverTypeLastWins])

	// TODO(jlodhia): special handling for conflicts of type 'Defer'
}

// getLogRecsBatch gets the log records for an array of versions for a given object.
func (iSt *initiationState) getLogRecsBatch(ctx *context.T, obj string, versions []string) ([]*LocalLogRec, error) {
	lrecs := make([]*LocalLogRec, len(versions))
	for p, v := range versions {
		if v == NoVersion {
			lrecs[p] = nil
			continue
		}

		logKey, err := getLogRecKey(ctx, iSt.tx, obj, v)
		if err != nil {
			return nil, err
		}
		lrecs[p], err = getLogRecByKey(ctx, iSt.tx, logKey)
		if err != nil {
			return nil, err
		}
	}
	return lrecs, nil
}
