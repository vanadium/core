// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"testing"
)

/*
Test setup:

iSt.updObjects contains oids: x, y, z, d
x, y, z are data keys ("r")
d is a collection perms key ("c")
z is not under conflict.

Result:
x, y and d should get resolved based on timestamp
z will be skipped as it is not under conflict.
*/

var (
	updObjectsLastWins = map[string]*objConflictState{
		x: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		y: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
		z: createObjConflictState(false /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, false /*hasAncestor*/),
		d: createObjConflictState(true /*isConflict*/, true /*hasLocal*/, true /*hasRemote*/, true /*hasAncestor*/),
	}
)

func createAndSaveNodeAndLogRecData(iSt *initiationState) {
	saveNodeAndLogRec(iSt.tx, x, updObjectsLastWins[x].oldHead, 24, false)
	saveNodeAndLogRec(iSt.tx, x, updObjectsLastWins[x].newHead, 25, false)

	saveNodeAndLogRec(iSt.tx, y, updObjectsLastWins[y].oldHead, 56, false)
	saveNodeAndLogRec(iSt.tx, y, updObjectsLastWins[y].newHead, 23, false)

	saveNodeAndLogRec(iSt.tx, z, updObjectsLastWins[z].newHead, 12, false)

	saveNodeAndLogRec(iSt.tx, d, updObjectsLastWins[d].oldHead, 24, false)
	saveNodeAndLogRec(iSt.tx, d, updObjectsLastWins[d].newHead, 24, false)
}

func TestLastWinsResolveViaTimestamp(t *testing.T) {
	service := createService(t)
	defer destroyService(t, service)

	iSt := &initiationState{updObjects: updObjectsLastWins, tx: createDatabase(t, service).St().NewWatchableTransaction()}
	createAndSaveNodeAndLogRecData(iSt)

	if err := iSt.resolveViaTimestamp(nil, updObjectsLastWins); err != nil {
		t.Errorf("resolveViaTimestamp failed with error: %v", err)
	}
	verifyUpdObjectsResolution(t)
}

func verifyUpdObjectsResolution(t *testing.T) {
	verifyResolution(t, updObjectsLastWins, x, pickRemote)
	verifyResolution(t, updObjectsLastWins, y, pickLocal)
	if updObjectsLastWins[z].res != nil {
		t.Errorf("oid z should have been skipped")
	}
	if updObjectsLastWins[d].oldHead > updObjectsLastWins[d].newHead {
		verifyResolution(t, updObjectsLastWins, d, pickLocal)
	} else {
		verifyResolution(t, updObjectsLastWins, d, pickRemote)
	}
	verifyBatchId(t, updObjectsLastWins, NoBatchId, x, y, d)
}
