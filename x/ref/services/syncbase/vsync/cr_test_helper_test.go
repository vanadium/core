// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"math/rand"
	"testing"

	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

var (
	x = makeRowKeyFromParts("u", "collection1", "x")
	y = makeRowKeyFromParts("u", "collection1", "y")
	z = makeRowKeyFromParts("u", "collection1", "z")
	a = makeRowKeyFromParts("u", "collection2", "a")
	b = makeRowKeyFromParts("u", "collection2", "b")
	c = makeRowKeyFromParts("u", "collection2", "c")
	d = makeCollectionPermsKey("u", "collection1")

	e = makeRowKeyFromParts("u", "collection1", "e")
	f = makeRowKeyFromParts("u", "collection1", "f")
	g = makeRowKeyFromParts("u", "collection1", "g")

	// Oids for batches containing linked objects
	la1 = makeRowKeyFromParts("u", "collection1", "la1")
	lb1 = makeRowKeyFromParts("u", "collection1", "lb1")
	lc1 = makeRowKeyFromParts("u", "collection1", "lc1")

	la2 = makeRowKeyFromParts("u", "collection1", "la2")
	lb2 = makeRowKeyFromParts("u", "collection1", "lb2")
	lc2 = makeRowKeyFromParts("u", "collection1", "lc2")

	la3 = makeRowKeyFromParts("u", "collection1", "la3")
	lb3 = makeRowKeyFromParts("u", "collection1", "lb3")
	lc3 = makeRowKeyFromParts("u", "collection1", "lc3")

	la4 = makeRowKeyFromParts("u", "collection1", "la4")
	lb4 = makeRowKeyFromParts("u", "collection1", "lb4")
	lc4 = makeRowKeyFromParts("u", "collection1", "lc4")

	p = makeRowKeyFromParts("u", "collection3", "p")
	q = makeRowKeyFromParts("u", "collection3", "q")
)

func rand64() uint64 {
	return uint64(rand.Int63())
}

func createObjConflictState(isConflict, hasLocal, hasRemote, hasAncestor bool) *objConflictState {
	remote := NoVersion
	local := NoVersion
	ancestor := NoVersion
	if hasRemote {
		remote = string(watchable.NewVersion())
	}
	if hasLocal {
		local = string(watchable.NewVersion())
	}
	if hasAncestor {
		ancestor = string(watchable.NewVersion())
	}
	return &objConflictState{
		isAddedByCr: hasLocal && !hasRemote,
		isConflict:  isConflict,
		newHead:     remote,
		oldHead:     local,
		ancestor:    ancestor,
	}
}

func createObjLinkState(isRemoteBlessed bool) *objConflictState {
	local := string(watchable.NewVersion())
	remote := local
	if isRemoteBlessed {
		remote = string(watchable.NewVersion())
	}
	return &objConflictState{
		isAddedByCr: false,
		isConflict:  false,
		newHead:     remote,
		oldHead:     local,
		ancestor:    NoVersion,
	}
}

func verifyResolution(t *testing.T, updMap map[string]*objConflictState, oid string, resolution resolutionType) {
	st, ok := updMap[oid]
	if !ok {
		t.Errorf("st not found for oid %v", oid)
	}
	if st.res == nil {
		t.Errorf("st.res found nil for oid %v", oid)
		return
	}
	if st.res.ty != resolution {
		t.Errorf("st.res.ty value for oid %v expected: %v, actual: %v", oid, resolution, st.res.ty)
	}
}

func verifyBatchId(t *testing.T, updMap map[string]*objConflictState, batchId uint64, oids ...string) {
	for _, oid := range oids {
		if updMap[oid].res.batchId != batchId {
			t.Errorf("BatchId for Oid %v expected: %#v, actual: %#v", oid, batchId, updMap[oid].res.batchId)
		}
	}
}

func verifyCreateNew(t *testing.T, updMap map[string]*objConflictState, oid string, isDeleted bool) {
	st, ok := updMap[oid]
	if !ok {
		t.Errorf("st not found for oid %v", oid)
	}
	if st.res == nil {
		t.Errorf("st.res found nil for oid %v", oid)
		return
	}
	if st.res.ty != createNew {
		t.Errorf("st.res.ty value for oid %v expected: %v, actual: %v", oid, createNew, st.res.ty)
	}
	if updObjectsAppResolves[oid].res.rec == nil {
		t.Errorf("No log record found for newly created value of oid: %v", oid)
	}
	if isDeleted {
		if updObjectsAppResolves[oid].res.val != nil {
			t.Errorf("Resolution for oid %v has missing new value", oid)
		}
	} else if updObjectsAppResolves[oid].res.val == nil {
		t.Errorf("Resolution for oid %v is not expected to have a val", oid)
	}
}

func saveNodeAndLogRec(tx store.Transaction, oid, version string, ts int64, isDeleted bool) {
	devId, gen := rand64(), rand64()
	node := &DagNode{
		Deleted: isDeleted,
		Logrec:  logRecKey(logDataPrefix, devId, gen),
	}
	setNode(nil, tx, oid, version, node)

	logRec := &LocalLogRec{
		Metadata: interfaces.LogRecMetadata{
			Id:      devId,
			Gen:     gen,
			UpdTime: unixNanoToTime(ts),
			CurVers: version,
		},
	}
	putLogRec(nil, tx, logDataPrefix, logRec)
}

func uint64ArrayEq(expected, actual []uint64) bool {
	if len(actual) != len(expected) {
		return false
	}
	batchMap := map[uint64]int8{}
	for _, bid := range expected {
		batchMap[bid] = batchMap[bid] + 1
	}
	for _, bid := range actual {
		count := batchMap[bid]
		if count <= 0 {
			return false
		} else if count == 1 {
			delete(batchMap, bid)
		} else {
			batchMap[bid] = count - 1
		}
	}
	return len(batchMap) == 0
}
