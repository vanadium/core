// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"bytes"
	"testing"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store/watchable"
)

/*
Test setup for normal conflicts:

Group1:
Oid: x, isConflict = true, has local update, has remote update, has ancestor, resolution = pickLocal

Group2:
Oid: b, isConflict = true, has local update, remote deleted, has ancestor, resolution = pickRemote

Group3:
Oid: c, isConflict = true, local deleted, has remote update, has ancestor, resolution = pickRemote

Group4:
Oid: p, isConflict = true, has local update, has remote update, has ancestor, resolution = createNew

Group5:
Oid: y, isConflict = true, has local update, has remote update, has no ancestor, resolution = pickRemote
Oid: z, isConflict = false, no local update, has remote update, has unknown ancestor, resolution = pickRemote
Oid: e, isConflict = false, no local update, has remote update, has unknown ancestor, resolution = createNew
Oid: f, isConflict = false, no local update, has remote update, has unknown ancestor, resolution = pickLocal
Oid: g, isConflict = false, no local update, has remote update, has unknown ancestor, local head is deleted, resolution = pickLocal
Oid: a, local value rubberbanded in due to localBatch, resolution = createNew

localBatch1: {y, a}
remoteBatch1: {y, z, e, f}

Group 6:
Oid: la1, isConflict = true, has local update, has remote update, has ancestor, resolution = pickLocal
Oid: lb1, is a LinkLogRecord, isConflict = false, remote syncbase added a linklogrecord with local update as parent and remote update as the child, resolution = pickLocal
Oid: lc1, is a LinkLogRecord, isConflict = false, remote syncbase added a linklogrecord with local update as child and remote update as the parent, resolution = pickLocal
remoteBatch2: {la1, lb1, lc1}

Group 7:
Oid: la2, isConflict = true, has local update, has remote update, has ancestor, resolution = pickRemote
Oid: lb2, is a LinkLogRecord, isConflict = false, remote syncbase added a linklogrecord with local update as parent and remote update as the child, resolution = pickRemote
Oid: lc2, is a LinkLogRecord, isConflict = false, remote syncbase added a linklogrecord with local update as child and remote update as the parent, resolution = pickRemote
remoteBatch3: {la2, lb2, lc2}

Group 8:
Oid: la3, isConflict = true, has local update, has remote update, has ancestor, resolution = pickNew
Oid: lb3, is a LinkLogRecord, isConflict = false, remote syncbase added a linklogrecord with local update as parent and remote update as the child, resolution = pickNew
Oid: lc3, is a LinkLogRecord, isConflict = false, remote syncbase added a linklogrecord with local update as child and remote update as the parent, resolution = pickNew
remoteBatch4: {la3, lb3, lc3}

Group 9:
Oid: la4, isConflict = true, has local update, has remote update, has ancestor, resolution = pickLocal
Oid: lb4, isAddedByCr = true, isConflict = false, no remote update, no ancestor, resolution = pickLocal
Oid: lc4, is a LinkLogRecord, isConflict = false, remote syncbase added a linklogrecord with local update as child and remote update as the parent, resolution = pickLocal
localBatch5: {la4, lb4}
remoteBatch5: {la4, lc4}
*/

var (
	updObjectsAppResolves = map[string]*objConflictState{
		// group1
		x: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		// group2
		b: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		// group3
		c: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		// group4
		p: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),

		// group5
		y: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),
		z: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),
		a: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, false /*hasRemote*/, false /*hasAncestor*/),

		e: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),
		f: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),
		g: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),

		// group6
		la1: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		lb1: createObjLinkState(true /*isRemoteBlessed*/),
		lc1: createObjLinkState(false /*isRemoteBlessed*/),

		// group7
		la2: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		lb2: createObjLinkState(true /*isRemoteBlessed*/),
		lc2: createObjLinkState(false /*isRemoteBlessed*/),

		// group8
		la3: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		lb3: createObjLinkState(true /*isRemoteBlessed*/),
		lc3: createObjLinkState(false /*isRemoteBlessed*/),

		// group9
		la4: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		lb4: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, false /*hasRemote*/, false /*hasAncestor*/),
		lc4: createObjLinkState(false /*isRemoteBlessed*/),
	}

	localBatchId1  uint64 = 34
	remoteBatchId1 uint64 = 58

	remoteBatchId2 uint64 = 72
	remoteBatchId3 uint64 = 45
	remoteBatchId4 uint64 = 23

	localBatchId5  uint64 = 98
	remoteBatchId5 uint64 = 78
)

func createGroupedCrTestData() *groupedCrData {
	groupedCrTestData := &groupedCrData{oids: map[string]bool{}}
	var group *crGroup
	// group1
	group = newGroup()
	addToGroup(group, x, NoBatchId, -1)
	groupedCrTestData.oids[x] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group2
	group = newGroup()
	addToGroup(group, b, NoBatchId, -1)
	groupedCrTestData.oids[b] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group3
	group = newGroup()
	addToGroup(group, c, NoBatchId, -1)
	groupedCrTestData.oids[c] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group4
	group = newGroup()
	addToGroup(group, p, NoBatchId, -1)
	groupedCrTestData.oids[p] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group5
	group = newGroup()
	addToGroup(group, y, localBatchId1, wire.BatchSourceLocal)
	addToGroup(group, y, remoteBatchId1, wire.BatchSourceRemote)
	addToGroup(group, z, remoteBatchId1, wire.BatchSourceRemote)
	addToGroup(group, e, remoteBatchId1, wire.BatchSourceRemote)
	addToGroup(group, f, remoteBatchId1, wire.BatchSourceRemote)
	addToGroup(group, g, remoteBatchId1, wire.BatchSourceRemote)
	addToGroup(group, a, localBatchId1, wire.BatchSourceLocal)
	groupedCrTestData.oids[y] = true
	groupedCrTestData.oids[z] = true
	groupedCrTestData.oids[a] = true
	groupedCrTestData.oids[e] = true
	groupedCrTestData.oids[f] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group6
	group = newGroup()
	addToGroup(group, la1, remoteBatchId2, wire.BatchSourceRemote)
	addToGroup(group, lb1, remoteBatchId2, wire.BatchSourceRemote)
	addToGroup(group, lc1, remoteBatchId2, wire.BatchSourceRemote)
	groupedCrTestData.oids[la1] = true
	groupedCrTestData.oids[lb1] = true
	groupedCrTestData.oids[lc1] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group7
	group = newGroup()
	addToGroup(group, la2, remoteBatchId3, wire.BatchSourceRemote)
	addToGroup(group, lb2, remoteBatchId3, wire.BatchSourceRemote)
	addToGroup(group, lc2, remoteBatchId3, wire.BatchSourceRemote)
	groupedCrTestData.oids[la2] = true
	groupedCrTestData.oids[lb2] = true
	groupedCrTestData.oids[lc2] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group8
	group = newGroup()
	addToGroup(group, la3, remoteBatchId4, wire.BatchSourceRemote)
	addToGroup(group, lb3, remoteBatchId4, wire.BatchSourceRemote)
	addToGroup(group, lc3, remoteBatchId4, wire.BatchSourceRemote)
	groupedCrTestData.oids[la3] = true
	groupedCrTestData.oids[lb3] = true
	groupedCrTestData.oids[lc3] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	// group9
	group = newGroup()
	addToGroup(group, la4, remoteBatchId5, wire.BatchSourceRemote)
	addToGroup(group, la4, localBatchId5, wire.BatchSourceLocal)
	addToGroup(group, lb4, localBatchId5, wire.BatchSourceLocal)
	addToGroup(group, lc4, remoteBatchId5, wire.BatchSourceRemote)
	groupedCrTestData.oids[la4] = true
	groupedCrTestData.oids[lb4] = true
	groupedCrTestData.oids[lc4] = true
	groupedCrTestData.groups = append(groupedCrTestData.groups, group)

	return groupedCrTestData
}

func createAndSaveNodeAndLogRecDataAppResolves(iSt *initiationState) {
	// group1
	saveNodeAndLogRec(iSt.tx, x, updObjectsAppResolves[x].oldHead, 24, false)
	saveNodeAndLogRec(iSt.tx, x, updObjectsAppResolves[x].newHead, 25, false)
	saveNodeAndLogRec(iSt.tx, x, updObjectsAppResolves[x].ancestor, 20, false)

	// group2
	saveNodeAndLogRec(iSt.tx, b, updObjectsAppResolves[b].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, b, updObjectsAppResolves[b].newHead, 23, true)
	saveNodeAndLogRec(iSt.tx, b, updObjectsAppResolves[b].ancestor, 15, false)

	// group3
	saveNodeAndLogRec(iSt.tx, c, updObjectsAppResolves[c].oldHead, 56, true)
	saveNodeAndLogRec(iSt.tx, c, updObjectsAppResolves[c].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, c, updObjectsAppResolves[c].ancestor, 15, false)

	// group4
	saveNodeAndLogRec(iSt.tx, p, updObjectsAppResolves[p].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, p, updObjectsAppResolves[p].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, p, updObjectsAppResolves[p].ancestor, 15, false)

	//group5
	saveNodeAndLogRec(iSt.tx, y, updObjectsAppResolves[y].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, y, updObjectsAppResolves[y].newHead, 23, false)

	saveNodeAndLogRec(iSt.tx, z, updObjectsAppResolves[z].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, z, updObjectsAppResolves[z].newHead, 23, false)

	saveNodeAndLogRec(iSt.tx, e, updObjectsAppResolves[e].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, e, updObjectsAppResolves[e].newHead, 23, false)

	saveNodeAndLogRec(iSt.tx, f, updObjectsAppResolves[f].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, f, updObjectsAppResolves[f].newHead, 23, false)

	saveNodeAndLogRec(iSt.tx, g, updObjectsAppResolves[g].oldHead, 56, true)
	saveNodeAndLogRec(iSt.tx, g, updObjectsAppResolves[g].newHead, 23, false)

	saveNodeAndLogRec(iSt.tx, a, updObjectsAppResolves[a].oldHead, 56, false)

	// group6
	saveNodeAndLogRec(iSt.tx, la1, updObjectsAppResolves[la1].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, la1, updObjectsAppResolves[la1].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, la1, updObjectsAppResolves[la1].ancestor, 15, false)
	saveNodeAndLogRec(iSt.tx, lb1, updObjectsAppResolves[lb1].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, lb1, updObjectsAppResolves[lb1].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, lc1, updObjectsAppResolves[lc1].oldHead, 56, false)
	// lc1.newHead is same as lc1.oldHead.

	// group7
	saveNodeAndLogRec(iSt.tx, la2, updObjectsAppResolves[la2].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, la2, updObjectsAppResolves[la2].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, la2, updObjectsAppResolves[la2].ancestor, 15, false)
	saveNodeAndLogRec(iSt.tx, lb2, updObjectsAppResolves[lb2].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, lb2, updObjectsAppResolves[lb2].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, lc2, updObjectsAppResolves[lc2].oldHead, 56, false)
	// lc2.newHead is same as lc2.oldHead.

	// group8
	saveNodeAndLogRec(iSt.tx, la3, updObjectsAppResolves[la3].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, la3, updObjectsAppResolves[la3].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, la3, updObjectsAppResolves[la3].ancestor, 15, false)
	saveNodeAndLogRec(iSt.tx, lb3, updObjectsAppResolves[lb3].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, lb3, updObjectsAppResolves[lb3].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, lc3, updObjectsAppResolves[lc3].oldHead, 56, false)
	// lc3.newHead is same as lc3.oldHead.

	// group8
	saveNodeAndLogRec(iSt.tx, la4, updObjectsAppResolves[la4].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, la4, updObjectsAppResolves[la4].newHead, 23, false)
	saveNodeAndLogRec(iSt.tx, la4, updObjectsAppResolves[la4].ancestor, 15, false)
	saveNodeAndLogRec(iSt.tx, lb4, updObjectsAppResolves[lb4].oldHead, 56, false)
	// lb4.newHead is NoVersion
	saveNodeAndLogRec(iSt.tx, lc4, updObjectsAppResolves[lc4].oldHead, 56, false)
	// lc4.newHead is same as lc4.oldHead.
}

func writeVersionedValues(t *testing.T, iSt *initiationState) {
	// group1
	saveValue(t, iSt.tx, x, updObjectsAppResolves[x].oldHead)
	saveValue(t, iSt.tx, x, updObjectsAppResolves[x].newHead)
	saveValue(t, iSt.tx, x, updObjectsAppResolves[x].ancestor)

	// group2
	saveValue(t, iSt.tx, b, updObjectsAppResolves[b].oldHead)
	saveValue(t, iSt.tx, b, updObjectsAppResolves[b].ancestor)

	// group3
	saveValue(t, iSt.tx, c, updObjectsAppResolves[c].newHead)
	saveValue(t, iSt.tx, c, updObjectsAppResolves[c].ancestor)

	// group4
	saveValue(t, iSt.tx, p, updObjectsAppResolves[p].oldHead)
	saveValue(t, iSt.tx, p, updObjectsAppResolves[p].newHead)
	saveValue(t, iSt.tx, p, updObjectsAppResolves[p].ancestor)

	// group5
	saveValue(t, iSt.tx, y, updObjectsAppResolves[y].oldHead)
	saveValue(t, iSt.tx, y, updObjectsAppResolves[y].newHead)

	saveValue(t, iSt.tx, z, updObjectsAppResolves[z].oldHead)
	saveValue(t, iSt.tx, z, updObjectsAppResolves[z].newHead)

	saveValue(t, iSt.tx, e, updObjectsAppResolves[e].oldHead)
	saveValue(t, iSt.tx, e, updObjectsAppResolves[e].newHead)

	saveValue(t, iSt.tx, f, updObjectsAppResolves[f].oldHead)
	saveValue(t, iSt.tx, f, updObjectsAppResolves[f].newHead)

	// No value for oldHead for g as g was deleted on local.
	saveValue(t, iSt.tx, g, updObjectsAppResolves[g].newHead)

	saveValue(t, iSt.tx, a, updObjectsAppResolves[a].oldHead)

	// group6
	saveValue(t, iSt.tx, la1, updObjectsAppResolves[la1].oldHead)
	saveValue(t, iSt.tx, la1, updObjectsAppResolves[la1].newHead)
	saveValue(t, iSt.tx, la1, updObjectsAppResolves[la1].ancestor)
	saveValue(t, iSt.tx, lb1, updObjectsAppResolves[lb1].oldHead)
	saveValue(t, iSt.tx, lb1, updObjectsAppResolves[lb1].newHead)
	saveValue(t, iSt.tx, lc1, updObjectsAppResolves[lc1].oldHead)

	// group7
	saveValue(t, iSt.tx, la2, updObjectsAppResolves[la2].oldHead)
	saveValue(t, iSt.tx, la2, updObjectsAppResolves[la2].newHead)
	saveValue(t, iSt.tx, la2, updObjectsAppResolves[la2].ancestor)
	saveValue(t, iSt.tx, lb2, updObjectsAppResolves[lb2].oldHead)
	saveValue(t, iSt.tx, lb2, updObjectsAppResolves[lb2].newHead)
	saveValue(t, iSt.tx, lc2, updObjectsAppResolves[lc2].oldHead)

	// group8
	saveValue(t, iSt.tx, la3, updObjectsAppResolves[la3].oldHead)
	saveValue(t, iSt.tx, la3, updObjectsAppResolves[la3].newHead)
	saveValue(t, iSt.tx, la3, updObjectsAppResolves[la3].ancestor)
	saveValue(t, iSt.tx, lb3, updObjectsAppResolves[lb3].oldHead)
	saveValue(t, iSt.tx, lb3, updObjectsAppResolves[lb3].newHead)
	saveValue(t, iSt.tx, lc3, updObjectsAppResolves[lc3].oldHead)

	// group9
	saveValue(t, iSt.tx, la4, updObjectsAppResolves[la4].oldHead)
	saveValue(t, iSt.tx, la4, updObjectsAppResolves[la4].newHead)
	saveValue(t, iSt.tx, la4, updObjectsAppResolves[la4].ancestor)
	saveValue(t, iSt.tx, lb4, updObjectsAppResolves[lb4].oldHead)
	saveValue(t, iSt.tx, lc4, updObjectsAppResolves[lc4].oldHead)
}

func setResInfoData(mockCrs *conflictResolverStream) {
	var newValue *vom.RawBytes
	newValue, _ = vom.RawBytesFromValue("newValue")
	// group1
	addResInfo(mockCrs, x, wire.ValueSelectionLocal, nil, false)
	// group2
	addResInfo(mockCrs, b, wire.ValueSelectionRemote, nil, false)
	// group3
	addResInfo(mockCrs, c, wire.ValueSelectionRemote, nil, false)
	// group4
	addResInfo(mockCrs, p, wire.ValueSelectionOther, newValue, false)
	// group5
	addResInfo(mockCrs, y, wire.ValueSelectionRemote, nil, true)
	addResInfo(mockCrs, z, wire.ValueSelectionRemote, nil, true)
	addResInfo(mockCrs, e, wire.ValueSelectionOther, newValue, true)
	addResInfo(mockCrs, f, wire.ValueSelectionLocal, nil, true)
	addResInfo(mockCrs, g, wire.ValueSelectionLocal, nil, true)
	addResInfo(mockCrs, a, wire.ValueSelectionOther, newValue, false)
	// group6
	addResInfo(mockCrs, la1, wire.ValueSelectionLocal, nil, true)
	addResInfo(mockCrs, lb1, wire.ValueSelectionLocal, nil, true)
	addResInfo(mockCrs, lc1, wire.ValueSelectionLocal, nil, false)
	// group7
	addResInfo(mockCrs, la2, wire.ValueSelectionRemote, nil, true)
	addResInfo(mockCrs, lb2, wire.ValueSelectionRemote, nil, true)
	addResInfo(mockCrs, lc2, wire.ValueSelectionRemote, nil, false)
	// group8
	addResInfo(mockCrs, la3, wire.ValueSelectionOther, newValue, true)
	addResInfo(mockCrs, lb3, wire.ValueSelectionOther, newValue, true)
	addResInfo(mockCrs, lc3, wire.ValueSelectionOther, newValue, false)
	// group9
	addResInfo(mockCrs, la4, wire.ValueSelectionLocal, nil, true)
	addResInfo(mockCrs, lb4, wire.ValueSelectionLocal, nil, true)
	addResInfo(mockCrs, lc4, wire.ValueSelectionLocal, nil, false)

	setMockCRStream(mockCrs)
}

func TestResolveViaApp(t *testing.T) {
	service := createService(t)
	defer destroyService(t, service)

	db := createDatabase(t, service)

	iSt := &initiationState{
		updObjects: updObjectsAppResolves,
		tx:         db.St().NewWatchableTransaction(),
		config: &initiationConfig{
			sync: service.sync,
			db:   db,
		},
	}
	createAndSaveNodeAndLogRecDataAppResolves(iSt)
	writeVersionedValues(t, iSt)
	mockCrs := &conflictResolverStream{}
	setResInfoData(mockCrs)
	groupedCrTestData := createGroupedCrTestData()

	if err := iSt.resolveViaApp(nil, groupedCrTestData); err != nil {
		t.Errorf("Error returned by resolveViaApp: %v", err)
	}

	// verify
	verifyConflictInfos(t, mockCrs)
	// group1
	verifyResolution(t, iSt.updObjects, x, pickLocal)
	// group2
	verifyResolution(t, iSt.updObjects, b, pickRemote)
	// group3
	verifyResolution(t, iSt.updObjects, c, pickRemote)
	// group4
	verifyCreateNew(t, iSt.updObjects, p, false)
	// group5
	verifyResolution(t, iSt.updObjects, y, pickRemote)
	verifyResolution(t, iSt.updObjects, z, pickRemote)
	verifyCreateNew(t, iSt.updObjects, e, false)
	verifyCreateNew(t, iSt.updObjects, f, false)
	verifyCreateNew(t, iSt.updObjects, g, true)
	verifyCreateNew(t, iSt.updObjects, a, false)

	// group6
	verifyResolution(t, iSt.updObjects, la1, pickLocal)
	// to avoid circular loop in dag pickLocal is converted to createNew
	verifyCreateNew(t, iSt.updObjects, lb1, false)
	verifyResolution(t, iSt.updObjects, lc1, pickLocal)

	// group7
	verifyResolution(t, iSt.updObjects, la2, pickRemote)
	verifyResolution(t, iSt.updObjects, lb2, pickRemote)
	// since localHead == remoteHead, the response is optimized and converted to
	// pickLocal
	verifyResolution(t, iSt.updObjects, lc2, pickLocal)

	// group8
	verifyCreateNew(t, iSt.updObjects, la3, false)
	verifyCreateNew(t, iSt.updObjects, lb3, false)
	verifyCreateNew(t, iSt.updObjects, lc3, false)

	// group9
	verifyResolution(t, iSt.updObjects, la4, pickLocal)
	verifyResolution(t, iSt.updObjects, lb4, pickLocal)
	verifyResolution(t, iSt.updObjects, lc4, pickLocal)

	// verify batch ids
	verifyBatchId(t, iSt.updObjects, NoBatchId, x, b, c, p)
	bid := iSt.updObjects[y].res.batchId
	if bid == NoBatchId {
		t.Errorf("BatchId for group5 should not be NoBatchId")
	}
	verifyBatchId(t, iSt.updObjects, bid, y, z, e, f, a)

	bid = iSt.updObjects[la1].res.batchId
	if bid == NoBatchId {
		t.Errorf("BatchId for group6 should not be NoBatchId")
	}
	verifyBatchId(t, iSt.updObjects, bid, la1, lb1, lc1)

	bid = iSt.updObjects[la2].res.batchId
	if bid == NoBatchId {
		t.Errorf("BatchId for group7 should not be NoBatchId")
	}
	verifyBatchId(t, iSt.updObjects, bid, la2, lb2, lc2)

	bid = iSt.updObjects[la3].res.batchId
	if bid == NoBatchId {
		t.Errorf("BatchId for group8 should not be NoBatchId")
	}
	verifyBatchId(t, iSt.updObjects, bid, la3, lb3, lc3)

	bid = iSt.updObjects[la4].res.batchId
	if bid == NoBatchId {
		t.Errorf("BatchId for group9 should not be NoBatchId")
	}
	verifyBatchId(t, iSt.updObjects, bid, la4, lb4, lc4)
}

func verifyConflictInfos(t *testing.T, mockCrs *conflictResolverStream) {
	var ci wire.ConflictInfo
	if len(mockCrs.sendQ) != 29 {
		t.Errorf("ConflictInfo count expected: %v, actual: %v", 29, len(mockCrs.sendQ))
	}

	// group1
	ci = mockCrs.sendQ[0]
	checkConflictRow(t, x, ci, []uint64{}, false /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[0:1])

	// group2
	ci = mockCrs.sendQ[1]
	checkConflictRow(t, b, ci, []uint64{}, false /*localDeleted*/, true /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[1:2])

	// group3
	ci = mockCrs.sendQ[2]
	checkConflictRow(t, c, ci, []uint64{}, true /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[2:3])

	// group4
	ci = mockCrs.sendQ[3]
	checkConflictRow(t, p, ci, []uint64{}, false /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[3:4])

	// group5
	batchMap := map[uint64]wire.ConflictInfo{}
	batchMap[getBid(mockCrs.sendQ[4])] = mockCrs.sendQ[4]
	batchMap[getBid(mockCrs.sendQ[5])] = mockCrs.sendQ[5]
	ci = batchMap[localBatchId1]
	checkConflictBatch(t, localBatchId1, ci, wire.BatchSourceLocal)
	ci = batchMap[remoteBatchId1]
	checkConflictBatch(t, remoteBatchId1, ci, wire.BatchSourceRemote)

	rowMap := map[string]wire.ConflictInfo{}
	rowMap[getOid(mockCrs.sendQ[6])] = mockCrs.sendQ[6]
	rowMap[getOid(mockCrs.sendQ[7])] = mockCrs.sendQ[7]
	rowMap[getOid(mockCrs.sendQ[8])] = mockCrs.sendQ[8]
	rowMap[getOid(mockCrs.sendQ[9])] = mockCrs.sendQ[9]
	rowMap[getOid(mockCrs.sendQ[10])] = mockCrs.sendQ[10]
	rowMap[getOid(mockCrs.sendQ[11])] = mockCrs.sendQ[11]
	ci = rowMap[y]
	checkConflictRow(t, y, ci, []uint64{localBatchId1, remoteBatchId1}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[z]
	checkConflictRow(t, z, ci, []uint64{remoteBatchId1}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[a]
	checkConflictRow(t, a, ci, []uint64{localBatchId1}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[e]
	checkConflictRow(t, e, ci, []uint64{remoteBatchId1}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[f]
	checkConflictRow(t, f, ci, []uint64{remoteBatchId1}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[g]
	checkConflictRow(t, g, ci, []uint64{remoteBatchId1}, true /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[4:12])

	// group6
	ci = mockCrs.sendQ[12]
	checkConflictBatch(t, remoteBatchId2, ci, wire.BatchSourceRemote)
	rowMap[getOid(mockCrs.sendQ[13])] = mockCrs.sendQ[13]
	rowMap[getOid(mockCrs.sendQ[14])] = mockCrs.sendQ[14]
	rowMap[getOid(mockCrs.sendQ[15])] = mockCrs.sendQ[15]
	ci = rowMap[la1]
	checkConflictRow(t, la1, ci, []uint64{remoteBatchId2}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lb1]
	checkConflictRow(t, lb1, ci, []uint64{remoteBatchId2}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lc1]
	checkConflictRow(t, lc1, ci, []uint64{remoteBatchId2}, false /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[12:16])

	// group7
	ci = mockCrs.sendQ[16]
	checkConflictBatch(t, remoteBatchId3, ci, wire.BatchSourceRemote)
	rowMap[getOid(mockCrs.sendQ[17])] = mockCrs.sendQ[17]
	rowMap[getOid(mockCrs.sendQ[18])] = mockCrs.sendQ[18]
	rowMap[getOid(mockCrs.sendQ[19])] = mockCrs.sendQ[19]
	ci = rowMap[la2]
	checkConflictRow(t, la2, ci, []uint64{remoteBatchId3}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lb2]
	checkConflictRow(t, lb2, ci, []uint64{remoteBatchId3}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lc2]
	checkConflictRow(t, lc2, ci, []uint64{remoteBatchId3}, false /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[16:20])

	// group8
	ci = mockCrs.sendQ[20]
	checkConflictBatch(t, remoteBatchId4, ci, wire.BatchSourceRemote)
	rowMap[getOid(mockCrs.sendQ[21])] = mockCrs.sendQ[21]
	rowMap[getOid(mockCrs.sendQ[22])] = mockCrs.sendQ[22]
	rowMap[getOid(mockCrs.sendQ[23])] = mockCrs.sendQ[23]
	ci = rowMap[la3]
	checkConflictRow(t, la3, ci, []uint64{remoteBatchId4}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lb3]
	checkConflictRow(t, lb3, ci, []uint64{remoteBatchId4}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lc3]
	checkConflictRow(t, lc3, ci, []uint64{remoteBatchId4}, false /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[20:24])

	// group9
	batchMap = map[uint64]wire.ConflictInfo{}
	batchMap[getBid(mockCrs.sendQ[24])] = mockCrs.sendQ[24]
	batchMap[getBid(mockCrs.sendQ[25])] = mockCrs.sendQ[25]
	ci = batchMap[localBatchId5]
	checkConflictBatch(t, localBatchId5, ci, wire.BatchSourceLocal)
	ci = batchMap[remoteBatchId5]
	checkConflictBatch(t, remoteBatchId5, ci, wire.BatchSourceRemote)
	rowMap[getOid(mockCrs.sendQ[26])] = mockCrs.sendQ[26]
	rowMap[getOid(mockCrs.sendQ[27])] = mockCrs.sendQ[27]
	rowMap[getOid(mockCrs.sendQ[28])] = mockCrs.sendQ[28]
	ci = rowMap[la4]
	checkConflictRow(t, la4, ci, []uint64{localBatchId5, remoteBatchId5}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lb4]
	checkConflictRow(t, lb4, ci, []uint64{localBatchId5}, false /*localDeleted*/, false /*remoteDeleted*/)
	ci = rowMap[lc4]
	checkConflictRow(t, lc4, ci, []uint64{remoteBatchId5}, false /*localDeleted*/, false /*remoteDeleted*/)
	checkContinued(t, mockCrs.sendQ[24:])
}

func checkContinued(t *testing.T, infoGroup []wire.ConflictInfo) {
	lastIndex := len(infoGroup) - 1
	for i := range infoGroup {
		if infoGroup[i].Continued != (i != lastIndex) {
			t.Errorf("Wrong value for continued field in %#v", infoGroup[i])
		}
	}
}

func getOid(ci wire.ConflictInfo) string {
	ciData := ci.Data.(wire.ConflictDataRow).Value
	writeOp := ciData.Op.(wire.OperationWrite).Value
	return toRowKey(writeOp.Key)
}

func getBid(ci wire.ConflictInfo) uint64 {
	ciData := ci.Data.(wire.ConflictDataBatch).Value
	return ciData.Id
}

func checkConflictBatch(t *testing.T, bid uint64, ci wire.ConflictInfo, source wire.BatchSource) {
	ciData := ci.Data.(wire.ConflictDataBatch).Value
	if ciData.Source != source {
		t.Errorf("Source for batchid %v expected: %#v, actual: %#v", bid, source, ciData.Source)
	}
	if ci.Continued != true {
		t.Errorf("Bid: %v, Unexpected value for continued: %v", bid, ci.Continued)
	}
}

func checkConflictRow(t *testing.T, oid string, ci wire.ConflictInfo, batchIds []uint64, localDeleted, remoteDeleted bool) {
	ciData := ci.Data.(wire.ConflictDataRow).Value
	writeOp := ciData.Op.(wire.OperationWrite).Value
	st, _ := updObjectsAppResolves[oid]
	var err error
	var ancestorValueBytes []byte
	if writeOp.AncestorValue.Bytes != nil {
		if err = writeOp.AncestorValue.Bytes.ToValue(&ancestorValueBytes); err != nil {
			t.Errorf("Oid: %v: error decoding writeOp.AncestorValue.Bytes: %v", oid, err)
		}
	}
	if (st.ancestor != NoVersion) && !bytes.Equal(makeValue(oid, st.ancestor), ancestorValueBytes) {
		t.Errorf("Oid: %v, Ancestor value expected: %v, actual: %v", oid, string(makeValue(oid, st.ancestor)), string(ancestorValueBytes))
	}
	if remoteDeleted && writeOp.RemoteValue == nil {
		t.Errorf("Oid: %v, for remote deleted remote value is expected to have an instance with no bytes", oid)
	}
	var remoteValueBytes []byte
	if writeOp.RemoteValue.Bytes != nil {
		if err = writeOp.RemoteValue.Bytes.ToValue(&remoteValueBytes); err != nil {
			t.Errorf("Oid: %v: error decoding writeOp.RemoteValue.Bytes: %v", oid, err)
		}
	}
	if !remoteDeleted && (st.newHead != NoVersion) && !bytes.Equal(makeValue(oid, st.newHead), remoteValueBytes) {
		t.Errorf("Oid: %v, Remote value expected: %v, actual: %v", oid, string(makeValue(oid, st.newHead)), string(remoteValueBytes))
	}
	if localDeleted && writeOp.LocalValue == nil {
		t.Errorf("Oid: %v, for local deleted local value is expected to have an instance with no bytes", oid)
	}
	var localValueBytes []byte
	if writeOp.LocalValue.Bytes != nil {
		if err = writeOp.LocalValue.Bytes.ToValue(&localValueBytes); err != nil {
			t.Errorf("Oid: %v: error decoding writeOp.LocalValue.Bytes: %v", oid, err)
		}
	}
	if !localDeleted && (st.oldHead != NoVersion) && !bytes.Equal(makeValue(oid, st.oldHead), localValueBytes) {
		t.Errorf("Oid: %v, Local value expected: %v, actual: %v", oid, string(makeValue(oid, st.oldHead)), string(localValueBytes))
	}
	if !uint64ArrayEq(batchIds, ciData.BatchIds) {
		t.Errorf("Oid: %v, BatchIds expected: %v, actual: %v", oid, batchIds, ciData.BatchIds)
	}
}

func addResInfo(crs *conflictResolverStream, oid string, sel wire.ValueSelection, val *vom.RawBytes, cntd bool) {
	var valRes *wire.Value
	if val != nil {
		valRes = &wire.Value{
			Bytes: val,
		}
	}
	rInfo := wire.ResolutionInfo{
		Key:       common.StripFirstKeyPartOrDie(oid),
		Selection: sel,
		Result:    valRes,
		Continued: cntd,
	}
	crs.recvQ = append(crs.recvQ, rInfo)
}

func saveValue(t *testing.T, tx *watchable.Transaction, oid, version string) {
	encodedValue, err := vom.Encode(makeValue(oid, version))
	if err != nil {
		t.Errorf("Error encoding %s,%s: %v", oid, version, err)
	}
	if err := watchable.PutAtVersion(nil, tx, []byte(oid), encodedValue, []byte(version)); err != nil {
		t.Errorf("Failed to write versioned value for oid,ver: %s,%s", oid, version)
		t.FailNow()
	}
}

func makeValue(oid, ver string) []byte {
	return []byte(oid + ver)
}

func addToGroup(group *crGroup, oid string, bid uint64, source wire.BatchSource) {
	if bid != NoBatchId {
		group.batchSource[bid] = source
		group.batchesByOid[oid] = append(group.batchesByOid[oid], bid)
	} else {
		group.batchesByOid[oid] = []uint64{}
	}
}

func newGroup() *crGroup {
	return &crGroup{
		batchSource:  map[uint64]wire.BatchSource{},
		batchesByOid: map[string][]uint64{},
	}
}
