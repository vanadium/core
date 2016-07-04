// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"testing"

	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

//	Test setup:
//
//	iSt.updObjects contains oids: x, y, b, c, a, p, q
//	There are 5 batches:
//	xzb   : local
//	xy    : remote
//	bc    : remote
//	pq(L) : local
//	pq(R) : remote
//
//	a is not a part of any batch.
//	z does not have any remote changes (hence not a part of iSt.updObjects)
//
//	Result:
//	The following closures will be created:
//	group1: {x, y, z, b, c}
//	group2: {a}
//	group3: {p, q}

var (
	updObjects map[string]*objConflictState
	zVer       = string(watchable.NewVersion())

	batchxzbId = rand64()
	batchxzb   = createBatch(true /*local*/, x, z, b)

	batchxyId = rand64()
	batchxy   = createBatch(false /*remote*/, x, y)

	batchbcId = rand64()
	batchbc   = createBatch(false /*remote*/, b, c)

	batchpqLocalId = rand64()
	batchpqLocal   = createBatch(true /*local*/, p, q)

	batchpqRemoteId = rand64()
	batchpqRemote   = createBatch(false /*remote*/, p, q)
)

func createUpdObjectsMap() map[string]*objConflictState {
	return map[string]*objConflictState{
		//Group1
		x: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		y: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),
		// z has local change only and is not present in updObjects
		b: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		c: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),

		// Group2
		a: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),

		// Group3
		p: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		q: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
	}
}

func createAndSaveNodeAndBatchData(iSt *initiationState) {
	createAndSaveNode(iSt.tx, batchxzbId, x, updObjects[x].oldHead)
	createAndSaveNode(iSt.tx, batchxzbId, z, zVer)
	//watchable.PutVersion(nil, iSt.tx, []byte(z), []byte(zVer))
	setHead(nil, iSt.tx, z, zVer)
	createAndSaveNode(iSt.tx, batchxzbId, b, updObjects[b].oldHead)
	setBatch(nil, iSt.tx, batchxzbId, batchxzb)

	createAndSaveNode(iSt.tx, batchxyId, x, updObjects[x].newHead)
	createAndSaveNode(iSt.tx, batchxyId, y, updObjects[y].newHead)
	setBatch(nil, iSt.tx, batchxyId, batchxy)

	createAndSaveNode(iSt.tx, batchbcId, b, updObjects[b].newHead)
	createAndSaveNode(iSt.tx, batchbcId, c, updObjects[c].newHead)
	setBatch(nil, iSt.tx, batchbcId, batchbc)

	createAndSaveNode(iSt.tx, NoBatchId, a, updObjects[a].newHead)

	createAndSaveNode(iSt.tx, batchpqLocalId, p, updObjects[p].oldHead)
	createAndSaveNode(iSt.tx, batchpqLocalId, q, updObjects[q].oldHead)
	setBatch(nil, iSt.tx, batchpqLocalId, batchpqLocal)

	createAndSaveNode(iSt.tx, batchpqRemoteId, p, updObjects[p].newHead)
	createAndSaveNode(iSt.tx, batchpqRemoteId, q, updObjects[q].newHead)
	setBatch(nil, iSt.tx, batchpqRemoteId, batchpqRemote)
}

func TestGroupFor(t *testing.T) {
	service := createService(t)
	defer destroyService(t, service)

	updObjects = createUpdObjectsMap()
	iSt := &initiationState{updObjects: updObjects, tx: createDatabase(t, service).St().NewWatchableTransaction()}
	createAndSaveNodeAndBatchData(iSt)

	// Group1 is a closure of batches xzb, xy, bc containing oids x, y, z, b, c.
	// Following tests run groupFor() method on each of the oids and expects
	// the same group (Group1) to be returned regardless of which oid the
	// closure was innitiated with.
	group := initCrGroup()
	iSt.groupFor(nil, group, x)
	verifyGroup1(t, group)

	group = initCrGroup()
	iSt.groupFor(nil, group, y)
	verifyGroup1(t, group)

	group = initCrGroup()
	iSt.groupFor(nil, group, b)
	verifyGroup1(t, group)

	group = initCrGroup()
	iSt.groupFor(nil, group, c)
	verifyGroup1(t, group)

	// Group2 is a closure of oid "a" only. Since a is not part of any batches,
	// Group2 does not have any batches in it.
	group = initCrGroup()
	iSt.groupFor(nil, group, a)
	verifyGroup2(t, group)

	// Group3 is a closure of batches pqLocal and pqRemote containing oids p, q.
	// Following tests run groupFor() method on each of the oids and expects
	// the same group (Group3) to be returned regardless of which oid the
	// closure was innitiated with.
	group = initCrGroup()
	iSt.groupFor(nil, group, p)
	verifyGroup3(t, group)

	group = initCrGroup()
	iSt.groupFor(nil, group, q)
	verifyGroup3(t, group)
}

func TestGroupConflicts(t *testing.T) {
	service := createService(t)
	defer destroyService(t, service)

	updObjects = createUpdObjectsMap()
	iSt := &initiationState{updObjects: updObjects, tx: createDatabase(t, service).St().NewWatchableTransaction()}
	createAndSaveNodeAndBatchData(iSt)

	// Assuming that all objects in updObjects are to be resolved by app.
	crGroups := iSt.groupConflicts(nil, updObjects)

	for _, group := range crGroups.groups {
		batchCount := len(group.batchSource)
		if batchCount == 3 {
			verifyGroup1(t, group)
		} else if batchCount == 0 {
			verifyGroup2(t, group)
		} else if batchCount == 2 {
			verifyGroup3(t, group)
		} else {
			t.Errorf("Encountered unexpected group: %v", group)
		}
	}
	if len(crGroups.oids) != 8 {
		t.Errorf("Size of groupedCrData.oids expected: %d actual: %d", 8, len(crGroups.oids))
	}
	verifyOidList(t, crGroups, x, y, z, a, b, c, p, q)
}

func verifyOidList(t *testing.T, crGroups *groupedCrData, oids ...string) {
	for _, oid := range oids {
		if !crGroups.oids[oid] {
			t.Errorf("Oid %s missing in groupedCrData.oids map", oid)
		}
	}
}

func verifyGroup1(t *testing.T, group *crGroup) {
	verifyBatchSource(t, group, batchxzbId, "xzb", wire.BatchSourceLocal)
	verifyBatchSource(t, group, batchxyId, "xy", wire.BatchSourceRemote)
	verifyBatchSource(t, group, batchbcId, "bc", wire.BatchSourceRemote)

	verifyBatchesByOid(t, group, x, batchxzbId, batchxyId)
	verifyBatchesByOid(t, group, z, batchxzbId)
	verifyBatchesByOid(t, group, b, batchxzbId, batchbcId)
	verifyBatchesByOid(t, group, y, batchxyId)
	verifyBatchesByOid(t, group, c, batchbcId)

	objSt := updObjects[z]
	if (objSt == nil) || (objSt.oldHead != zVer) || objSt.isConflict || !objSt.isAddedByCr {
		t.Errorf("Unexpected value of objConflictState for z: %#v", objSt)
	}
}

func verifyGroup2(t *testing.T, group *crGroup) {
	if len(group.batchSource) != 0 {
		t.Errorf("Non empty batchSource map found for Group2: %v", group.batchSource)
	}
	_, present := group.batchesByOid[a]
	if len(group.batchesByOid) != 1 || !present {
		t.Errorf("Unexpected batchesByOid map found for Group2: %v", group.batchesByOid)
	}
}

func verifyGroup3(t *testing.T, group *crGroup) {
	verifyBatchSource(t, group, batchpqLocalId, "pqLocal", wire.BatchSourceLocal)
	verifyBatchSource(t, group, batchpqRemoteId, "pqRemote", wire.BatchSourceRemote)

	verifyBatchesByOid(t, group, p, batchpqLocalId, batchpqRemoteId)
	verifyBatchesByOid(t, group, q, batchpqLocalId, batchpqRemoteId)
}

func verifyBatchSource(t *testing.T, group *crGroup, batchId uint64, batchName string, source wire.BatchSource) {
	if v, ok := group.batchSource[batchId]; !ok || (v != source) {
		t.Errorf("Missing or incorrect source for batchId: %s", batchName)
	}
}

func verifyBatchesByOid(t *testing.T, group *crGroup, oid string, batchIds ...uint64) {
	batchesInOid := group.batchesByOid[oid]
	batchMap := map[uint64]bool{}
	for _, batchId := range batchesInOid {
		batchMap[batchId] = true
	}
	for _, batchId := range batchIds {
		if !batchMap[batchId] {
			t.Errorf("Batch %v expected to be associated with oid %v but missing.", batchId, oid)
		}
	}
}

func createBatch(isLocal bool, oids ...string) *BatchInfo {
	batch := &BatchInfo{
		Count:   uint64(len(oids)),
		Objects: map[string]string{},
	}
	for _, oid := range oids {
		if _, present := batch.Objects[oid]; !present {
			batch.Objects[oid] = zVer
			continue
		}
		if isLocal {
			batch.Objects[oid] = updObjects[oid].oldHead
		} else {
			batch.Objects[oid] = updObjects[oid].newHead
		}
	}
	return batch
}

func createAndSaveNode(tx store.Transaction, batchId uint64, oid, version string) {
	node := &DagNode{
		BatchId: batchId,
	}
	setNode(nil, tx, oid, version, node)
}
