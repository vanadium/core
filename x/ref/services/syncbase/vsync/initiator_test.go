// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The initiator tests below are driven by replaying the state from the log
// files (in testdata directory). These log files may mimic watching the
// Database locally (addl commands in the log file) or obtaining log records and
// generation vector from a remote peer (addr, genvec commands). The log files
// contain the metadata of log records. The log files are only used to set up
// the state. The tests verify that given a particular local state and a stream
// of remote deltas, the initiator behaves as expected.

package vsync

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/vom"
	"v.io/x/lib/set"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// TestLogStreamRemoteOnly tests processing of a remote log stream. Commands are
// in file testdata/remote-init-00.log.sync.
func TestLogStreamRemoteOnly(t *testing.T) {
	svc, iSt, cleanup := testInit(t, "", "remote-init-00.log.sync", false)
	defer cleanup()

	// Check all log records.
	objid := common.JoinKeyParts(common.RowPrefix, "c", "foo1")
	var gen uint64
	var parents []string
	for gen = 1; gen < 4; gen++ {
		gotRec, err := getLogRec(nil, svc.St(), logDataPrefix, 11, gen)
		if err != nil || gotRec == nil {
			t.Fatalf("getLogRec can not find object %s 11 %d, err %v", logDataPrefix, gen, err)
		}
		vers := fmt.Sprintf("%d", gen)
		wantRec := &LocalLogRec{
			Metadata: interfaces.LogRecMetadata{
				Id:         11,
				Gen:        gen,
				RecType:    interfaces.NodeRec,
				ObjId:      objid,
				CurVers:    vers,
				Parents:    parents,
				UpdTime:    constTime,
				BatchCount: 1,
			},
			Pos: gen - 1,
		}

		if !reflect.DeepEqual(gotRec, wantRec) {
			t.Fatalf("Data mismatch in log record got %v, want %v", gotRec, wantRec)
		}
		// Verify DAG state.
		if _, err := getNode(nil, svc.St(), objid, vers); err != nil {
			t.Fatalf("getNode can not find object %s vers %s in DAG, err %v", objid, vers, err)
		}
		// Verify Database state.
		tx := createDatabase(t, svc).St().NewWatchableTransaction()
		if _, err := watchable.GetAtVersion(nil, tx, []byte(objid), nil, []byte(vers)); err != nil {
			t.Fatalf("GetAtVersion can not find object %s vers %s in Database, err %v", objid, vers, err)
		}
		tx.Abort()
		parents = []string{vers}
	}

	// Verify conflict state.
	if len(iSt.updObjects) != 1 {
		t.Fatalf("Unexpected number of updated objects %d", len(iSt.updObjects))
	}
	st := iSt.updObjects[objid]
	if st.isConflict {
		t.Fatalf("Detected a conflict %v", st)
	}
	if st.newHead != "3" || st.oldHead != NoVersion {
		t.Fatalf("Conflict detection didn't succeed %v", st)
	}

	// Verify genvec state.
	wantVecs := interfaces.Knowledge{
		"c\xfefoo1": interfaces.GenVector{11: 3},
		"c\xfebar":  interfaces.GenVector{11: 0},
	}
	if !reflect.DeepEqual(iSt.updLocal, wantVecs) {
		t.Fatalf("Final local gen vec mismatch got %v, want %v", iSt.updLocal, wantVecs)
	}

	// Verify DAG state.
	if head, err := getHead(nil, svc.St(), objid); err != nil || head != "3" {
		t.Fatalf("Invalid object %s head in DAG %v, err %v", objid, head, err)
	}

	// Verify Database state.
	db := createDatabase(t, svc)
	valbuf, err := db.St().Get([]byte(objid), nil)
	var val string
	if err := vom.Decode(valbuf, &val); err != nil {
		t.Fatalf("Value decode failed, err %v: %+v %+v", err, valbuf, objid)
	}
	if err != nil || val != "abc" {
		t.Fatalf("Invalid object %s in Database %v, err %v", objid, val, err)
	}
	tx := db.St().NewWatchableTransaction()
	version, err := watchable.GetVersion(nil, tx, []byte(objid))
	if err != nil || string(version) != "3" {
		t.Fatalf("Invalid object %s head in Database %v, err %v", objid, string(version), err)
	}
	tx.Abort()
}

// TestLogStreamNoConflict tests that a local and a remote log stream can be
// correctly applied (when there are no conflicts). Commands are in files
// testdata/<local-init-00.log.sync,remote-noconf-00.log.sync>.
func TestLogStreamNoConflict(t *testing.T) {
	svc, iSt, cleanup := testInit(t, "local-init-00.log.sync", "remote-noconf-00.log.sync", false)
	defer cleanup()

	objid := common.JoinKeyParts(common.RowPrefix, "c\xfefoo1")

	// Check all log records.
	var version uint64 = 1
	var parents []string
	for _, devid := range []uint64{10, 11} {
		var gen uint64
		for gen = 1; gen < 4; gen++ {
			gotRec, err := getLogRec(nil, svc.St(), logDataPrefix, devid, gen)
			if err != nil || gotRec == nil {
				t.Fatalf("getLogRec can not find object %s:%d:%d, err %v",
					logDataPrefix, devid, gen, err)
			}
			vers := fmt.Sprintf("%d", version)
			wantRec := &LocalLogRec{
				Metadata: interfaces.LogRecMetadata{
					Id:         devid,
					Gen:        gen,
					RecType:    interfaces.NodeRec,
					ObjId:      objid,
					CurVers:    vers,
					Parents:    parents,
					UpdTime:    constTime,
					BatchCount: 1,
				},
				Pos: gen - 1,
			}

			if !reflect.DeepEqual(gotRec, wantRec) {
				t.Fatalf("Data mismatch in log record got %v, want %v", gotRec, wantRec)
			}

			// Verify DAG state.
			if _, err := getNode(nil, svc.St(), objid, vers); err != nil {
				t.Fatalf("getNode can not find object %s vers %s in DAG, err %v", objid, vers, err)
			}
			// Verify Database state.
			tx := createDatabase(t, svc).St().NewWatchableTransaction()
			if _, err := watchable.GetAtVersion(nil, tx, []byte(objid), nil, []byte(vers)); err != nil {
				t.Fatalf("GetAtVersion can not find object %s vers %s in Database, err %v", objid, vers, err)
			}
			tx.Abort()
			parents = []string{vers}
			version++
		}
	}

	// Verify conflict state.
	if len(iSt.updObjects) != 1 {
		t.Fatalf("Unexpected number of updated objects %d", len(iSt.updObjects))
	}
	st := iSt.updObjects[objid]
	if st.isConflict {
		t.Fatalf("Detected a conflict %v", st)
	}
	if st.newHead != "6" || st.oldHead != "3" {
		t.Fatalf("Conflict detection didn't succeed %v", st)
	}

	// Verify genvec state.
	wantVecs := interfaces.Knowledge{
		"c\xfefoo1": interfaces.GenVector{11: 3},
		"c\xfebar":  interfaces.GenVector{11: 0},
	}
	if !reflect.DeepEqual(iSt.updLocal, wantVecs) {
		t.Fatalf("Final local gen vec failed got %v, want %v", iSt.updLocal, wantVecs)
	}

	// Verify DAG state.
	if head, err := getHead(nil, svc.St(), objid); err != nil || head != "6" {
		t.Fatalf("Invalid object %s head in DAG %v, err %v", objid, head, err)
	}

	// Verify Database state.
	db := createDatabase(t, svc)
	valbuf, err := db.St().Get([]byte(objid), nil)
	var val string
	if err := vom.Decode(valbuf, &val); err != nil {
		t.Fatalf("Value decode failed, err %v", err)
	}
	if err != nil || val != "abc" {
		t.Fatalf("Invalid object %s in Database %v, err %v", objid, val, err)
	}
	tx := db.St().NewTransaction()
	versbuf, err := watchable.GetVersion(nil, tx, []byte(objid))
	if err != nil || string(versbuf) != "6" {
		t.Fatalf("Invalid object %s head in Database %v, err %v", objid, string(versbuf), err)
	}
	tx.Abort()
}

// TestLogStreamConflict tests that a local and a remote log stream can be
// correctly applied when there are conflicts. Commands are in files
// testdata/<local-init-00.log.sync,remote-conf-00.log.sync>.
func TestLogStreamConflict(t *testing.T) {
	svc, iSt, cleanup := testInit(t, "local-init-00.log.sync", "remote-conf-00.log.sync", false)
	defer cleanup()

	objid := common.JoinKeyParts(common.RowPrefix, "c\xfefoo1")

	// Verify conflict state.
	if len(iSt.updObjects) != 1 {
		t.Fatalf("Unexpected number of updated objects %d", len(iSt.updObjects))
	}
	st := iSt.updObjects[objid]
	if !st.isConflict {
		t.Fatalf("Didn't detect a conflict %v", st)
	}
	if st.newHead != "6" || st.oldHead != "3" || st.ancestor != "2" {
		t.Fatalf("Conflict detection didn't succeed %v", st)
	}
	if st.res.ty != pickRemote {
		t.Fatalf("Conflict resolution did not pick remote: %v", st.res.ty)
	}

	// Verify DAG state.
	if head, err := getHead(nil, svc.St(), objid); err != nil || head != "6" {
		t.Fatalf("Invalid object %s head in DAG %v, err %v", objid, head, err)
	}

	// Verify Database state.
	db := createDatabase(t, svc)
	valbuf, err := db.St().Get([]byte(objid), nil)
	var val string
	if err := vom.Decode(valbuf, &val); err != nil {
		t.Fatalf("Value decode failed, err %v", err)
	}
	if err != nil || val != "abc" {
		t.Fatalf("Invalid object %s in Database %v, err %v", objid, string(valbuf), err)
	}
	tx := db.St().NewTransaction()
	versbuf, err := watchable.GetVersion(nil, tx, []byte(objid))
	if err != nil || string(versbuf) != "6" {
		t.Fatalf("Invalid object %s head in Database %v, err %v", objid, string(versbuf), err)
	}
	tx.Abort()
}

// TestLogStreamConflictNoAncestor tests that a local and a remote log stream
// can be correctly applied when there are conflicts from the start where the
// two versions of an object have no common ancestor. Commands are in files
// testdata/<local-init-00.log.sync,remote-conf-03.log.sync>.
func TestLogStreamConflictNoAncestor(t *testing.T) {
	svc, iSt, cleanup := testInit(t, "local-init-00.log.sync", "remote-conf-03.log.sync", false)
	defer cleanup()

	objid := common.JoinKeyParts(common.RowPrefix, "c\xfefoo1")

	// Verify conflict state.
	if len(iSt.updObjects) != 1 {
		t.Fatalf("Unexpected number of updated objects %d", len(iSt.updObjects))
	}
	st := iSt.updObjects[objid]
	if !st.isConflict {
		t.Fatalf("Didn't detect a conflict %v", st)
	}
	if st.newHead != "6" || st.oldHead != "3" || st.ancestor != "" {
		t.Fatalf("Conflict detection didn't succeed %v", st)
	}
	if st.res.ty != pickRemote {
		t.Fatalf("Conflict resolution did not pick remote: %v", st.res.ty)
	}

	// Verify DAG state.
	if head, err := getHead(nil, svc.St(), objid); err != nil || head != "6" {
		t.Fatalf("Invalid object %s head in DAG %v, err %v", objid, head, err)
	}

	// Verify Database state.
	db := createDatabase(t, svc)
	valbuf, err := db.St().Get([]byte(objid), nil)
	var val string
	if err := vom.Decode(valbuf, &val); err != nil {
		t.Fatalf("Value decode failed, err %v", err)
	}
	if err != nil || val != "abc" {
		t.Fatalf("Invalid object %s in Database %v, err %v", objid, string(valbuf), err)
	}
	tx := db.St().NewTransaction()
	versbuf, err := watchable.GetVersion(nil, tx, []byte(objid))
	if err != nil || string(versbuf) != "6" {
		t.Fatalf("Invalid object %s head in Database %v, err %v", objid, string(versbuf), err)
	}
	tx.Abort()
}

//////////////////////////////
// Helpers.

func testInit(t *testing.T, lfile, rfile string, sg bool) (*mockService, *initiationState, func()) {
	// Set a large value to prevent the initiator from running.
	peerSyncInterval = 1 * time.Hour
	svc := createService(t)
	cleanup := func() {
		destroyService(t, svc)
	}

	// If there's an error during testInit, we bail out and should clean up after
	// ourselves.
	var err error
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	now, err := svc.vclock.Now()
	if err != nil {
		t.Fatalf("unable to get time: %v\n", err)
	}
	s := svc.sync
	s.id = 10 // initiator

	sgId1 := interfaces.GroupId("1234")
	nullInfo := wire.SyncgroupMemberInfo{}
	sgInfo := sgMember{
		sgId1: interfaces.SyncgroupMemberState{WhenUpdated: now.Unix(), MemberInfo: nullInfo},
	}
	info := &memberInfo{
		db2sg: map[wire.Id]sgMember{
			mockDbId: sgInfo,
		},
		mtTables: map[string]struct{}{
			"1/2/3/4": struct{}{},
			"5/6/7/8": struct{}{},
		},
	}

	sg1 := &interfaces.Syncgroup{
		Id:          wire.Id{Name: "sg1", Blessing: "b1"},
		DbId:        mockDbId,
		Creator:     "mockCreator",
		SpecVersion: "etag-0",
		Spec: wire.SyncgroupSpec{
			Collections: []wire.Id{makeCxId("foo"), makeCxId("bar")},
			Perms:       mockSgPerms,
			MountTables: []string{"1/2/3/4", "5/6/7/8"},
		},
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"a": interfaces.SyncgroupMemberState{WhenUpdated: now.Unix(), MemberInfo: nullInfo},
			"b": interfaces.SyncgroupMemberState{WhenUpdated: now.Unix(), MemberInfo: nullInfo},
		},
	}

	tx := createDatabase(t, svc).St().NewWatchableTransaction()
	if err = s.addSyncgroup(nil, tx, NoVersion, true, "", nil, s.id, 1, 1, sg1); err != nil {
		t.Fatalf("cannot add syncgroup %v, err %v", sg1.Id, err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("cannot commit adding syncgroup %v, err %v", sg1.Id, err)
	}

	if lfile != "" {
		replayLocalCommands(t, svc, lfile)
	}

	if rfile == "" {
		return svc, nil, cleanup
	}

	var c *initiationConfig
	if c, err = newInitiationConfig(nil, s, mockDbId, info.db2sg[mockDbId], connInfo{relName: "b", mtTbls: set.String.ToSlice(info.mtTables)}); err != nil {
		t.Fatalf("newInitiationConfig failed with err %v", err)
	}

	iSt := newInitiationState(nil, c, sg)

	sgs := make(sgSet)
	sgs[sgId1] = struct{}{}
	iSt.config.sharedSgPfxs = map[string]sgSet{
		toCollectionPrefixStr(sg1.Spec.Collections[0]): sgs,
		toCollectionPrefixStr(sg1.Spec.Collections[1]): sgs,
	}

	sort.Strings(iSt.config.peer.mtTbls)
	sort.Strings(sg1.Spec.MountTables)

	if !reflect.DeepEqual(iSt.config.peer.mtTbls, sg1.Spec.MountTables) {
		// Set err so that the deferred cleanup func runs.
		err = fmt.Errorf("Mount tables are not equal: config %v, spec %v", iSt.config.peer.mtTbls, sg1.Spec.MountTables)
		t.Fatal(err)
	}

	s.initSyncStateInMem(nil, mockDbId, sgOID(sgId1))

	iSt.stream = createReplayStream(t, rfile)

	var wantVecs interfaces.Knowledge
	if sg {
		if err = iSt.prepareSGDeltaReq(nil); err != nil {
			t.Fatalf("prepareSGDeltaReq failed with err %v", err)
		}
		sg := string(sgId1)
		wantVecs = interfaces.Knowledge{
			sg: interfaces.GenVector{10: 0},
		}
	} else {
		if err = iSt.prepareDataDeltaReq(nil); err != nil {
			t.Fatalf("prepareDataDeltaReq failed with err %v", err)
		}

		wantVecs = interfaces.Knowledge{
			"mockuser,foo\xfe": interfaces.GenVector{10: 0},
			"mockuser,bar\xfe": interfaces.GenVector{10: 0},
		}
	}

	if !reflect.DeepEqual(iSt.local, wantVecs) {
		// Set err so that the deferred cleanup func runs.
		err = fmt.Errorf("createLocalGenVec failed: got %v, want %v", iSt.local, wantVecs)
		t.Fatal(err)
	}

	if err = iSt.recvAndProcessDeltas(nil); err != nil {
		t.Fatalf("recvAndProcessDeltas failed with err %v", err)
	}

	if err = iSt.processUpdatedObjects(nil); err != nil {
		t.Fatalf("processUpdatedObjects failed with err %v", err)
	}
	return svc, iSt, cleanup
}
