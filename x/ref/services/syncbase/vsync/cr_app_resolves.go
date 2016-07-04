// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"fmt"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// crSendStream is a named interface replicating the interface returned by
// wire.ConflictManagerStartConflictResolverServerStream.SendStream()
type crSendStream interface {
	Send(item wire.ConflictInfo) error
}

// crRecvStream is a named interface replicating the interface returned by
// wire.ConflictManagerStartConflictResolverServerStream.RecvStream()
type crRecvStream interface {
	Advance() bool
	Value() wire.ResolutionInfo
	Err() error
}

// resolveViaApp takes a groupedCrData object containing multiple disjoint
// closures of conflict batches and sends these closures for resolution.
// This method consists of the following steps:
// 1) Get CR connection stream object for this database. If none exists,
//    resolution cannot proceed; return error.
// For each group,
// 2) Stream each batch info within the group to the app.
// 3) Stream each conflict info row within the group to the app with the last
//    row containing continued=false.
// 4) Receive a batch of resolutions from app. This is a blocking step.
// 5) Verify if all conflicts sent within the group have a corresponding
//    resolution.
// 6) Process each resolution info object by assigning an appropriate
//    conflictResolution object to the Oid under conflict.
// TODO(jlodhia): Add a timeout for waiting on app's resolution.
func (iSt *initiationState) resolveViaApp(ctx *context.T, groupedConflicts *groupedCrData) error {
	db := iSt.config.db
	appConn := db.CrConnectionStream()
	if appConn == nil {
		// CR not possible now, delay conflict resolution
		vlog.VI(2).Infof("sync: resolveViaApp: No ConflictResolution stream available for db. App based resolution cannot move forward.")
		return interfaces.NewErrBrokenCrConnection(ctx)
	}
	sendStream := appConn.SendStream()
	recvStream := appConn.RecvStream()

	vlog.VI(2).Infof("sync: resolveViaApp: starting app based resolution on %d groups", len(groupedConflicts.groups))
	for i, group := range groupedConflicts.groups {
		vlog.VI(2).Infof("sync: resolveViaApp: sending conflict group %d to app", i)
		// Send all batches first
		if err := sendBatches(ctx, iSt, sendStream, db, group); err != nil {
			return err
		}

		// Send all conflict rows.
		if err := sendConflictRows(ctx, iSt, sendStream, db, group); err != nil {
			return err
		}

		// Receive resolutions from the app.
		results, err := receiveResolutions(ctx, recvStream)
		if err != nil {
			return err
		}

		for oid := range group.batchesByOid {
			_, present := results[oid]
			if !present {
				// CR not possible now, delay conflict resolution
				// TODO(jlodhia):[usability] send an error message to app.
				errStr := fmt.Sprintf("cr: resolveViaApp: Resolution for oid %s expected but not received", oid)
				vlog.VI(2).Infof(errStr)
				return verror.New(verror.ErrInternal, ctx, errStr)
			}
		}
		vlog.VI(2).Infof("sync: resolveViaApp: all resolutions received")

		// Process resolutions.
		conf := iSt.config

		// Empty str for Data that is not SyncGroup.
		sgoid := ""

		// reserve more than needed
		reserveCount := uint64(len(results))
		gen, pos := conf.sync.reserveGenAndPosInDbLog(ctx, conf.dbId, sgoid, reserveCount)
		processResInfos(ctx, iSt, results, gen, pos)
	}
	return nil
}

// sendBatches streams batch info row for each batch present within a cr group.
// In case of error while sending a conflict via the stream, the stream is
// deemed to be dead and is reset.
func sendBatches(ctx *context.T, iSt *initiationState, sendStream crSendStream, db interfaces.Database, group *crGroup) error {
	for batchId, source := range group.batchSource {
		ci := createBatchConflictInfo(batchId, source)
		if err := sendStream.Send(*ci); err != nil {
			// CR not possible now, delay conflict resolution
			// Remove the outdated cr connection object from database.
			db.ResetCrConnectionStream()
			vlog.VI(2).Infof("sync: resolveViaApp: Error while sending conflict over stream: %v", err)
			return interfaces.NewErrBrokenCrConnection(ctx)
		}
	}
	return nil
}

// sendConflictRows streams conflict info rows present within a cr group.
// The continued field is set to false only for the last row to mark end of the
// group. In case of error while sending a conflict via the stream, the stream
// is deemed to be dead and is reset.
func sendConflictRows(ctx *context.T, iSt *initiationState, sendStream crSendStream, db interfaces.Database, group *crGroup) error {
	numRows, count := len(group.batchesByOid), 0
	for oid, batches := range group.batchesByOid {
		count++
		ci := createRowConflictInfo(ctx, iSt, oid, batches, count < numRows)
		if err := sendStream.Send(*ci); err != nil {
			// CR not possible now, delay conflict resolution
			// Remove the outdated cr connection object from database.
			db.ResetCrConnectionStream()
			vlog.VI(2).Infof("sync: resolveViaApp: Error while sending conflict over stream: %v", err)
			return interfaces.NewErrBrokenCrConnection(ctx)
		}
	}
	return nil
}

// receiveResolutions is a blocking function that waits for the app to return
// all resolutions for the cr group. End of group is marked by field continued
// with value 'false'.
// TODO(jlodhia): Add a timeout to this function to protect against bad client
// code.
func receiveResolutions(ctx *context.T, recvStream crRecvStream) (map[string]*wire.ResolutionInfo, error) {
	results := map[string]*wire.ResolutionInfo{}
	for recvStream.Advance() {
		resp := recvStream.Value()
		// Key received as part of resolution info has struct <collection>:<row>
		// while oid is a complete row key r:<collection>:<row>
		results[toRowKey(resp.Key)] = &resp
		if !resp.Continued {
			return results, nil
		}
	}
	if err := recvStream.Err(); err != nil {
		vlog.Errorf("sync: resolveViaApp: Error while receiving resolutions from app over stream: %v", err)
		return nil, err
	}
	// Response stream ended midway. Advance returned false but the last
	// resolution info object had Continued field true.
	vlog.Errorf("sync: resolveViaApp: Resoponse stream ended midway")
	return nil, verror.NewErrInternal(ctx)
}

// processResInfos assigns appropriate conflictResolution objects for Oids in
// iSt.updObjects map based on ResolutionInfos received for a conflict group.
// Following are the main steps in this function
// 1) Create a new BatchId for this group. If there is a single Oid in the group
//    then the batch id is NoBatchId
// 2) For each resolution create a new conflictResolution object and assign
//    appropriate resolution type (pickLocal, pickRemote, createNew)
// 3) In case of createNew, add the new value to the conflictResolution object
//    and create a new localLogRecord for the new value. This record is not
//    written to db yet.
func processResInfos(ctx *context.T, iSt *initiationState, results map[string]*wire.ResolutionInfo, gen, pos uint64) {
	// Total count for batch includes new values and linked log records
	// (i.e. len(results)).
	batchCount := uint64(len(results))
	resBatchId := newBatch(ctx, iSt, batchCount)
	sId := iSt.config.sync.id
	for oid, rInfo := range results {
		conflictState := iSt.updObjects[oid]
		var res conflictResolution
		res.batchId = resBatchId
		res.batchCount = batchCount
		switch {
		case rInfo.Selection == wire.ValueSelectionLocal:
			if createsCycle(conflictState.oldHead, conflictState) {
				// The object had a remote version and since its not under
				// conflict, the local version is supposed to be its ancestor.
				// To avoid a dag cycle, we treat it as a createNew.
				res.ty = createNew
				now, _ := iSt.tx.St.Clock.Now()
				timestamp := now
				dagNode := getNodeOrFail(ctx, iSt.tx, oid, conflictState.oldHead)
				if !dagNode.Deleted {
					res.val = getObjectAtVer(ctx, iSt, oid, conflictState.oldHead)
				}
				parents := []string{conflictState.newHead}
				res.rec = createLocalLogRec(ctx, oid, parents, dagNode.Deleted, timestamp, sId, gen, pos, resBatchId, batchCount)
				gen++
				pos++
			} else {
				res.ty = pickLocal
			}
		case rInfo.Selection == wire.ValueSelectionRemote:
			res.ty = pickRemote
			if conflictState.oldHead == conflictState.newHead {
				// pickRemote gives the same version as pickLocal.
				// Converting to pickLocal for optimization.
				res.ty = pickLocal
			}
		case rInfo.Selection == wire.ValueSelectionOther:
			// TODO(jlodhia):[optimization] Do byte comparison to see if
			// the new value is equal to local or remote. If so dont use
			// createNew.
			res.ty = createNew
			// TODO(m3b): What should this routine do if there's an encoding error?
			res.val, _ = vom.Encode(rInfo.Result.Bytes)
			// TODO(jlodhia):[correctness] Use vclock to create the write
			// timestamp.
			timestamp := time.Now()
			parents := getResolutionParents(iSt, oid)
			res.rec = createLocalLogRec(ctx, oid, parents, isDeleted(rInfo), timestamp, sId, gen, pos, resBatchId, batchCount)
			gen++
			pos++
		}
		conflictState.res = &res
	}
}

// createsCycle detects whether selecting resVer as the resolution would create
// a cycle in the dag or not.
func createsCycle(resVer string, conflictState *objConflictState) bool {
	if !conflictState.isConflict &&
		(conflictState.newHead != NoVersion) && // object pulled in by CR
		(conflictState.oldHead != conflictState.newHead) {
		// This means old head is a parent of new head. If the resolution is
		// oldHead then it will create a cycle.
		return resVer == conflictState.oldHead
	}
	return false
}

func getNodeOrFail(ctx *context.T, st store.StoreReader, oid, version string) *DagNode {
	dagNode, err := getNode(ctx, st, oid, version)
	if err != nil {
		vlog.Fatalf("sync: resolveViaApp: error while fetching dag node: %v", err)
	}
	return dagNode
}

func getResolutionParents(iSt *initiationState, oid string) []string {
	parents := []string{}
	conflictState, present := iSt.updObjects[oid]
	if !present {
		vlog.Fatalf("sync: resolveViaApp: getResolutionParents was called for oid that was not a part of updObjects: %v", oid)
	}

	// For ease of readability and maintainability, the following code is kept
	// verbose.
	if conflictState.isConflict {
		parents = append(parents, conflictState.oldHead)
		parents = append(parents, conflictState.newHead)
		return parents
	}

	if iSt.isDirty(oid) {
		// There is no conflict. The object has a remote update and the local
		// head is an ancestor of the remote version. Add only the remote
		// version as parent.
		parents = append(parents, conflictState.newHead)
		return parents
	}

	// This object was added by conflict resolution and only has local version.
	// The remote version is unknown. Add only the local version as parent.
	parents = append(parents, conflictState.oldHead)
	return parents
}

// A delete in resolution is signified by a non nil Result field with nil Bytes.
func isDeleted(rInfo *wire.ResolutionInfo) bool {
	return rInfo.Result.Bytes == nil
}

func newBatch(ctx *context.T, iSt *initiationState, count uint64) uint64 {
	if count < 2 {
		return NoBatchId
	}
	resBatchId := iSt.config.sync.startBatch(ctx, iSt.tx, NoBatchId)
	if resBatchId == NoBatchId {
		vlog.Fatalf("sync: resolveViaApp: failed to generate batch ID")
	}
	if err := iSt.config.sync.endBatch(ctx, iSt.tx, resBatchId, count); err != nil {
		vlog.Fatalf("sync: resolveViaApp: failed end batch for id %d with error: %v", resBatchId, err)
	}
	return resBatchId
}

// Each batch info row has continued field set to true since the last batch
// info row will be followed by at least one conflict row.
func createBatchConflictInfo(batchId uint64, source wire.BatchSource) *wire.ConflictInfo {
	// TODO(jlodhia): add app's hint once transaction api and sync handles hint.
	batch := wire.BatchInfo{Id: batchId, Hint: "", Source: source}
	return &wire.ConflictInfo{
		Data:      wire.ConflictDataBatch{Value: batch},
		Continued: true,
	}
}

func createRowConflictInfo(ctx *context.T, iSt *initiationState, oid string, batches []uint64, contd bool) *wire.ConflictInfo {
	op := wire.RowOp{}
	op.Key = common.StripFirstKeyPartOrDie(oid)
	objSt := iSt.updObjects[oid]

	op.AncestorValue = createValueObj(ctx, iSt, oid, objSt.ancestor, true)
	op.LocalValue = createValueObj(ctx, iSt, oid, objSt.oldHead, false)
	op.RemoteValue = createValueObj(ctx, iSt, oid, objSt.newHead, false)

	row := wire.RowInfo{
		Op:       wire.OperationWrite{Value: op},
		BatchIds: batches,
	}
	return &wire.ConflictInfo{
		Data:      wire.ConflictDataRow{Value: row},
		Continued: contd,
	}
}

func createValueObj(ctx *context.T, iSt *initiationState, oid, ver string, isAncestor bool) *wire.Value {
	if ver == NoVersion {
		if isAncestor {
			return &wire.Value{State: wire.ValueStateNoExists}
		}
		return &wire.Value{State: wire.ValueStateUnknown}
	}
	dagNode, err := getNode(ctx, iSt.tx, oid, ver)
	if err != nil {
		vlog.Fatalf("sync: resolveViaApp: error while fetching dag node: %v", err)
	}
	var bytes []byte = nil
	valueState := wire.ValueStateDeleted

	var rawBytes *vom.RawBytes = nil
	if !dagNode.Deleted {
		bytes = getObjectAtVer(ctx, iSt, oid, ver)
		// TODO(m3b): What should this routine do if there's a decoding error?
		vom.Decode(bytes, &rawBytes)
		valueState = wire.ValueStateExists
	}
	return &wire.Value{
		State:   valueState,
		Bytes:   rawBytes,
		WriteTs: getLocalLogRec(ctx, iSt, oid, ver).Metadata.UpdTime,
	}
}

func getLocalLogRec(ctx *context.T, iSt *initiationState, oid, version string) *LocalLogRec {
	lrecs, err := iSt.getLogRecsBatch(ctx, oid, []string{version})
	if err != nil {
		vlog.Fatalf("sync: resolveViaApp: error while fetching LocalLogRec: %v", err)
	}
	if lrecs == nil || lrecs[0] == nil {
		vlog.Fatalf("sync: resolveViaApp: LocalLogRec found nil for oid %v, version: %v", oid, version)
	}
	return lrecs[0]
}

func getObjectAtVer(ctx *context.T, iSt *initiationState, oid, ver string) []byte {
	bytes, err := watchable.GetAtVersion(ctx, iSt.tx, []byte(oid), nil, []byte(ver))
	if err != nil {
		vlog.Fatalf("sync: resolveViaApp: error while fetching object: %v", err)
	}
	return bytes
}

// createLocalLogRec creates a local sync log record given its information.
func createLocalLogRec(ctx *context.T, oid string, parents []string, deleted bool, timestamp time.Time, sId, gen, pos, batchId, count uint64) *LocalLogRec {
	if len(parents) == 0 {
		vlog.Fatalf("sync: resolveViaApp: there must be atleast one parent")
	}
	rec := LocalLogRec{}
	rec.Metadata.ObjId = oid
	rec.Metadata.CurVers = string(watchable.NewVersion())
	rec.Metadata.Delete = deleted
	rec.Metadata.Parents = parents
	rec.Metadata.UpdTime = timestamp

	rec.Metadata.Id = sId
	rec.Metadata.Gen = gen
	rec.Metadata.RecType = interfaces.NodeRec

	rec.Metadata.BatchId = batchId
	rec.Metadata.BatchCount = count

	rec.Pos = pos

	return &rec
}
