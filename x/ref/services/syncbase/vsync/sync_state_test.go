// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"reflect"
	"testing"
	"time"

	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
)

// Tests for sync state management and storage in Syncbase.

// TestReserveGenAndPos tests reserving generation numbers and log positions in
// a Database log.
func TestReserveGenAndPos(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)
	s := svc.sync

	sgids := []string{"", "100", "200"}
	for _, sgid := range sgids {
		var wantGen, wantPos uint64 = 1, 0
		for i := 0; i < 5; i++ {
			gotGen, gotPos := s.reserveGenAndPosInternal(mockDbId, sgid, 5, 10)
			if gotGen != wantGen || gotPos != wantPos {
				t.Fatalf("reserveGenAndPosInternal failed, gotGen %v wantGen %v, gotPos %v wantPos %v", gotGen, wantGen, gotPos, wantPos)
			}
			wantGen += 5
			wantPos += 10

			var info *localGenInfoInMem
			if sgid == "" {
				info = s.syncState[mockDbId].data
			} else {
				info = s.syncState[mockDbId].sgs[sgid]
			}
			if info.gen != wantGen || info.pos != wantPos {
				t.Fatalf("reserveGenAndPosInternal failed, gotGen %v wantGen %v, gotPos %v wantPos %v", info.gen, wantGen, info.pos, wantPos)
			}
		}
	}
}

// TestPutGetDbSyncState tests setting and getting sync metadata.
func TestPutGetDbSyncState(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)
	st := svc.St()

	checkDbSyncState(t, st, false, nil)

	gv := interfaces.Knowledge{
		"mockcn/foo": interfaces.GenVector{
			1: 2, 3: 4, 5: 6,
		},
	}
	sggv := interfaces.Knowledge{
		"mocksg1": interfaces.GenVector{
			10: 20, 30: 40, 50: 60,
		},
		"mocksg2": interfaces.GenVector{
			100: 200, 300: 400, 500: 600,
		},
	}

	tx := st.NewTransaction()
	wantSt := &DbSyncState{
		GenVecs:   gv,
		SgGenVecs: sggv,
		IsPaused:  true,
	}
	if err := putDbSyncState(nil, tx, wantSt); err != nil {
		t.Fatalf("putDbSyncState failed, err %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("cannot commit putting db sync state, err %v", err)
	}

	checkDbSyncState(t, st, true, wantSt)
}

// TestPutGetDelLogRec tests setting, getting, and deleting a log record.
func TestPutGetDelLogRec(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)
	st := svc.St()

	var id uint64 = 10
	var gen uint64 = 100

	checkLogRec(t, st, id, gen, false, nil)

	tx := st.NewTransaction()
	wantRec := &LocalLogRec{
		Metadata: interfaces.LogRecMetadata{
			Id:         id,
			Gen:        gen,
			RecType:    interfaces.NodeRec,
			ObjId:      "foo",
			CurVers:    "3",
			Parents:    []string{"1", "2"},
			UpdTime:    time.Now().UTC(),
			Delete:     false,
			BatchId:    10000,
			BatchCount: 1,
		},
		Pos: 10,
	}
	if err := putLogRec(nil, tx, logDataPrefix, wantRec); err != nil {
		t.Fatalf("putLogRec(%s:%d:%d) failed err %v", logDataPrefix, id, gen, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("cannot commit putting log rec, err %v", err)
	}

	checkLogRec(t, st, id, gen, true, wantRec)

	tx = st.NewTransaction()
	if err := delLogRec(nil, tx, logDataPrefix, id, gen); err != nil {
		t.Fatalf("delLogRec(%s:%d:%d) failed err %v", logDataPrefix, id, gen, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("cannot commit deleting log rec, err %v", err)
	}

	checkLogRec(t, st, id, gen, false, nil)
}

func TestSyncPauseAndResumeInCache(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)

	s := svc.sync
	if !s.isDbSyncable(nil, mockDbId) {
		t.Errorf("isDbSyncable returned false")
	}

	// Test Pause.
	s.updateInMemoryPauseSt(nil, mockDbId, true)
	if s.isDbSyncable(nil, mockDbId) {
		t.Errorf("isDbSyncable expected to return false")
	}

	// Test Resume.
	s.updateInMemoryPauseSt(nil, mockDbId, false)
	if !s.isDbSyncable(nil, mockDbId) {
		t.Errorf("isDbSyncable returned false")
	}
}

func TestSyncPauseAndResumeInDb(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)

	s := svc.sync
	st := svc.St()
	if !s.isDbSyncable(nil, mockDbId) {
		t.Errorf("isDbSyncable returned false")
	}

	// Test Pause.
	commitAndVerifySyncStateChange(t, s, st, true)
	// Test Resume.
	commitAndVerifySyncStateChange(t, s, st, false)
}

func commitAndVerifySyncStateChange(t *testing.T, s *syncService, st store.Store, expected bool) {
	tx := st.NewTransaction()
	if err := s.updateDbPauseSt(nil, tx, mockDbId, expected); err != nil {
		t.Errorf("Error while updating with isPaused=%t : %v", expected, err)
	}
	if err := tx.Commit(); err != nil {
		t.Errorf("Commit error: %v", err)
	}

	// Check disk state.
	ss, err := getDbSyncState(nil, st)
	if err != nil {
		t.Errorf("Lookup error %v", err)
	}
	if ss.IsPaused != expected {
		t.Errorf("Expected pause to be true but found false")
	}
}

//////////////////////////////
// Helpers

// TODO(hpucha): Look into using v.io/x/ref/services/syncbase/testutil.Fatalf()
// for getting the stack trace. Right now cannot import the package due to a
// cycle.

func checkDbSyncState(t *testing.T, st store.StoreReader, exists bool, wantSt *DbSyncState) {
	gotSt, err := getDbSyncState(nil, st)

	if (!exists && err == nil) || (exists && err != nil) {
		t.Fatalf("getDbSyncState failed, exists %v err %v", exists, err)
	}

	if !reflect.DeepEqual(gotSt, wantSt) {
		t.Fatalf("getDbSyncState() failed, got %v, want %v", gotSt, wantSt)
	}
}

func checkLogRec(t *testing.T, st store.StoreReader, id, gen uint64, exists bool, wantRec *LocalLogRec) {
	gotRec, err := getLogRec(nil, st, logDataPrefix, id, gen)

	if (!exists && err == nil) || (exists && err != nil) {
		t.Fatalf("getLogRec(%d:%d) failed, exists %v err %v", id, gen, exists, err)
	}

	if !reflect.DeepEqual(gotRec, wantRec) {
		t.Fatalf("getLogRec(%d:%d) failed, got %v, want %v", id, gen, gotRec, wantRec)
	}

	if ok, err := hasLogRec(st, logDataPrefix, id, gen); err != nil || ok != exists {
		t.Fatalf("hasLogRec(%d:%d) failed, want %v", id, gen, exists)
	}
}
