// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/store"
)

// crGroup is a closure of oids under conflict that got rubberbanded together
// due to conflicting batches that they belong to.
// NOTE: A closure can pull in an oid that was not under conflict originally.
type crGroup struct {
	// batchSource maps batchids to the source from which the batch comes
	// (Local/Remote).
	batchSource map[uint64]wire.BatchSource

	// batchesByOid maps oids to an array of batchIds to which the oids belong.
	batchesByOid map[string][]uint64
}

// groupedCrData contains an array of crGroups and a map of oids that are
// present in at least one of the crGroups.
type groupedCrData struct {
	groups []*crGroup
	oids   map[string]bool
}

// groupConflicts takes a map of oids to objConflictState objects and creates
// disjoint closures of oids. For example, consider input map containing oids
// a, b, c, d, e, f and g, with these objects belonging to batches as follows:
// {B1: a, b}, {B2: b, c}, {B3: d, e}, {B4: d, x, y}, f, g where x and y were
// not a part of the input conflict map but got pulled in due to B4. The result
// can be logically represented as: {a, b, c}, {d, e, x, y}, {f}, {g}.
func (iSt *initiationState) groupConflicts(ctx *context.T, conflictsForApp map[string]*objConflictState) *groupedCrData {
	crData := &groupedCrData{}
	crData.oids = map[string]bool{}
	for oid := range conflictsForApp {
		if crData.oids[oid] {
			// The oid has already been added to a previous group.
			continue
		}
		group := initCrGroup()
		iSt.groupFor(ctx, group, oid)
		crData.groups = append(crData.groups, group)
		for k := range group.batchesByOid {
			crData.oids[k] = true
		}
	}
	return crData
}

// groupFor is a recursive method (in conjunction with processBatchId()) that
// takes an oid and creates a closure for the oid by following the local and
// remote heads for the oid as present in iSt.updObjects and recursively
// including the batches that these heads belong to.
// If the given oid is not dirty (i.e. a part of iSt.updObjects) then it was
// rubberbanded in. A new conflict object for this oid is added to
// iSt.updObjects to mark inclusion of the oid into the conflict closure but
// its local and remote head batches are not followed.
func (iSt *initiationState) groupFor(ctx *context.T, group *crGroup, oid string) {
	if !iSt.isDirty(oid) {
		// This means that this object was rubberbanded into the conflict by
		// another object and we dont need to process its local & remote heads.
		// NOTE: we never expect to see this object in the groupConflicts()
		// loop without already having added it to crData.oids array which makes
		// sure that we never try to create a closure over this object resulting
		// in an empty group.
		iSt.addNewConflictObj(ctx, oid)
		return
	}

	// Handle dirty objects that are not under conflict. Since the object is not
	// under conflict the local version will in reality be the ancestor of the
	// remote version. Hence we dont need to include the batch related to this
	// version in our closure. This makes sure that the closure is identical to
	// what the other device would see if it pulled updates from this device
	// instead.
	localBatchId := NoBatchId
	if iSt.updObjects[oid].isConflict {
		localBatchId = getBatchId(ctx, iSt.tx, oid, iSt.getLocalVersion(oid))
	}
	remoteBatchId := getBatchId(ctx, iSt.tx, oid, iSt.getRemoteVersion(oid))

	if (localBatchId == NoBatchId) && (remoteBatchId == NoBatchId) {
		// This oid is not a part of any batch hence it was a root dirty node
		// and will form a group of single oid.
		group.batchesByOid[oid] = []uint64{}
		return
	}

	// Process localBatchId.
	iSt.processBatchId(ctx, group, localBatchId, wire.BatchSourceLocal)

	// Process remoteBatchId.
	iSt.processBatchId(ctx, group, remoteBatchId, wire.BatchSourceRemote)
}

// processBatchId takes a batch id and recursively adds a closure for each of
// the objects within this batch. If a batch has already been seen before then
// the method returns making it the termination condition for the recursion.
func (iSt *initiationState) processBatchId(ctx *context.T, group *crGroup, batchId uint64, source wire.BatchSource) {
	_, seen := group.batchSource[batchId]
	if (batchId == NoBatchId) || seen {
		// Don't process batchId which has already been processed earlier.
		return
	}
	elements := getBatchElements(ctx, iSt.tx, batchId)

	for k := range elements {
		group.batchesByOid[k] = append(group.batchesByOid[k], batchId)
	}
	group.batchSource[batchId] = source

	// Warning: the batchId processed in this call of the method must be added
	// to the batchSource map before a recursive call to groupFor() method is
	// made to avoid infinite recursion.
	for bOid := range elements {
		iSt.groupFor(ctx, group, bOid)
	}
}

func (iSt *initiationState) isDirty(oid string) bool {
	state, ok := iSt.updObjects[oid]
	return ok && !state.isAddedByCr
}

func (iSt *initiationState) getLocalVersion(oid string) string {
	return iSt.updObjects[oid].oldHead
}

func (iSt *initiationState) getRemoteVersion(oid string) string {
	return iSt.updObjects[oid].newHead
}

func (iSt *initiationState) addNewConflictObj(ctx *context.T, oid string) {
	headVer, err := getHead(ctx, iSt.tx, oid)
	if err != nil {
		vlog.Fatalf("sync: ConflictClosure: error while fetching head for oid %v : %v", oid, err)
	}
	iSt.updObjects[oid] = &objConflictState{
		isConflict:  false,
		isAddedByCr: true,
		oldHead:     headVer,
		newHead:     NoVersion, // TODO(jlodhia): figure out the remote version using gen vectors
		ancestor:    NoVersion,
	}
}

func getBatchId(ctx *context.T, st store.StoreReader, oid, version string) uint64 {
	if version == NoVersion {
		return NoBatchId
	}
	dagNode, err := getNode(ctx, st, oid, version)
	if err != nil {
		vlog.Fatalf("sync: ConflictClosure: error while fetching dag node: %v", err)
	}
	return dagNode.BatchId
}

func getBatchElements(ctx *context.T, st store.StoreReader, batchId uint64) map[string]string {
	bInfo, err := getBatch(ctx, st, batchId)
	if err != nil {
		vlog.Fatalf("sync: ConflictClosure: error while fetching batch info: %v", err)
	}
	// Add linked objects to the objects that belong to this batch.
	for k, v := range bInfo.LinkedObjects {
		bInfo.Objects[k] = v
	}
	return bInfo.Objects
}

func initCrGroup() *crGroup {
	return &crGroup{
		batchSource:  map[uint64]wire.BatchSource{},
		batchesByOid: map[string][]uint64{},
	}
}
