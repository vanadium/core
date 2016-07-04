// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Tests for the sync watcher in Syncbase.

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store/watchable"
	sbwatchable "v.io/x/ref/services/syncbase/watchable"
)

// signpostApproxEq() returns whether Signpost objects *a and *b are approximately equal,
// differing by no more than 60s in their timestamps.
func signpostApproxEq(a *interfaces.Signpost, b *interfaces.Signpost) bool {
	// We avoid reflect.DeepEqual() because LocationData contains a
	// time.Time which is routinely given a different time zone
	// (time.Location).
	if len(a.Locations) != len(b.Locations) || len(a.SgIds) != len(b.SgIds) {
		return false
	}
	for sg := range a.SgIds {
		if _, exists := b.SgIds[sg]; !exists {
			return false
		}
	}
	for peer, aLocData := range a.Locations {
		bLocData, exists := b.Locations[peer]
		if !exists ||
			aLocData.WhenSeen.After(bLocData.WhenSeen.Add(time.Minute)) ||
			bLocData.WhenSeen.After(aLocData.WhenSeen.Add(time.Minute)) {
			return false
		}
	}
	return true
}

// TestSetResmark tests setting and getting a resume marker.
func TestSetResmark(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)
	st := svc.St()

	resmark, err := getResMark(nil, st)
	if err == nil || resmark != nil {
		t.Errorf("found non-existent resume marker: %s, %v", resmark, err)
	}

	wantResmark := watchable.MakeResumeMarker(1234567890)
	tx := st.NewTransaction()
	if err := setResMark(nil, tx, wantResmark); err != nil {
		t.Errorf("cannot set resume marker: %v", err)
	}
	tx.Commit()

	resmark, err = getResMark(nil, st)
	if err != nil {
		t.Errorf("cannot get new resume marker: %v", err)
	}
	if !bytes.Equal(resmark, wantResmark) {
		t.Errorf("invalid new resume: got %s instead of %s", resmark, wantResmark)
	}
}

// TestWatchPrefixes tests setting and updating the watch prefixes map.
func TestWatchPrefixes(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)

	if len(watchPrefixes) != 0 {
		t.Errorf("watch prefixes not empty: %v", watchPrefixes)
	}

	gid1 := interfaces.GroupId(1234)
	gid2 := interfaces.GroupId(5678)

	watchPrefixOps := []struct {
		dbId wire.Id
		key  string
		add  bool
		gid  interfaces.GroupId
	}{
		{wire.Id{"app1", "db1"}, "foo", true, gid1},
		{wire.Id{"app1", "db1"}, "bar", true, gid1},
		{wire.Id{"app2", "db1"}, "xyz", true, gid1},
		{wire.Id{"app3", "db1"}, "haha", true, gid2},
		{wire.Id{"app1", "db1"}, "foo", true, gid2},
		{wire.Id{"app1", "db1"}, "foo", false, gid1},
		{wire.Id{"app2", "db1"}, "ttt", true, gid2},
		{wire.Id{"app2", "db1"}, "ttt", true, gid1},
		{wire.Id{"app2", "db1"}, "ttt", false, gid2},
		{wire.Id{"app2", "db1"}, "ttt", false, gid1},
		{wire.Id{"app2", "db2"}, "qwerty", true, gid1},
		{wire.Id{"app3", "db1"}, "haha", true, gid1},
		{wire.Id{"app2", "db2"}, "qwerty", false, gid1},
		{wire.Id{"app3", "db1"}, "haha", false, gid2},
	}

	for _, op := range watchPrefixOps {
		if op.add {
			addWatchPrefixSyncgroup(op.dbId, op.key, op.gid)
		} else {
			rmWatchPrefixSyncgroup(op.dbId, op.key, op.gid)
		}
	}

	expPrefixes := map[wire.Id]sgPrefixes{
		wire.Id{"app1", "db1"}: sgPrefixes{"foo": sgSet{gid2: struct{}{}}, "bar": sgSet{gid1: struct{}{}}},
		wire.Id{"app2", "db1"}: sgPrefixes{"xyz": sgSet{gid1: struct{}{}}},
		wire.Id{"app3", "db1"}: sgPrefixes{"haha": sgSet{gid1: struct{}{}}},
	}
	if !reflect.DeepEqual(watchPrefixes, expPrefixes) {
		t.Errorf("invalid watch prefixes: got %v instead of %v", watchPrefixes, expPrefixes)
	}

	checkSyncableTests := []struct {
		dbId   wire.Id
		key    string
		result bool
	}{
		{wire.Id{"app1", "db1"}, "foo", true},
		{wire.Id{"app1", "db1"}, "foobar", true},
		{wire.Id{"app1", "db1"}, "bar", true},
		{wire.Id{"app1", "db1"}, "bar123", true},
		{wire.Id{"app1", "db1"}, "f", false},
		{wire.Id{"app1", "db1"}, "ba", false},
		{wire.Id{"app1", "db1"}, "xyz", false},
		{wire.Id{"app1", "db555"}, "foo", false},
		{wire.Id{"app555", "db1"}, "foo", false},
		{wire.Id{"app2", "db1"}, "xyz123", true},
		{wire.Id{"app2", "db1"}, "ttt123", false},
		{wire.Id{"app2", "db2"}, "qwerty", false},
		{wire.Id{"app3", "db1"}, "hahahoho", true},
		{wire.Id{"app3", "db1"}, "hoho", false},
		{wire.Id{"app3", "db1"}, "h", false},
	}

	for _, test := range checkSyncableTests {
		res := syncable(test.dbId, makeRowKey(test.key))
		if res != test.result {
			t.Errorf("checkSyncable: invalid output: %v, %s: got %t instead of %t", test.dbId, test.key, res, test.result)
		}
	}
}

// newLog creates a Put or Delete watch log entry.
func newLog(key, version string, delete bool) *watchable.LogEntry {
	k, v := []byte(key), []byte(version)
	log := &watchable.LogEntry{}
	if delete {
		log.Op = vom.RawBytesOf(&watchable.DeleteOp{Key: k})
	} else {
		log.Op = vom.RawBytesOf(&watchable.PutOp{Key: k, Version: v})
	}
	return log
}

// newSGLog creates a syncgroup watch log entry.
func newSGLog(gid interfaces.GroupId, prefixes []string, remove bool) *watchable.LogEntry {
	return &watchable.LogEntry{
		// Note that the type information will be written in each log entry. This
		// is inefficient. Ideally we'd split the type information from the log
		// entries themselves (we only have 6 types of them). The issue tracking
		// this is http://v.io/i/1242.
		Op: vom.RawBytesOf(&sbwatchable.SyncgroupOp{SgId: gid, Prefixes: prefixes, Remove: remove}),
	}
}

// putBlobRefData inserts data containing blob refs for a key at a version.
func putBlobRefData(t *testing.T, tx *watchable.Transaction, key, version string) {
	data := struct {
		Msg  string
		Blob wire.BlobRef
	}{
		"hello world",
		wire.BlobRef("br_" + key + "_" + version),
	}

	buf, err := vom.Encode(&data)
	if err != nil {
		t.Errorf("putBlobRefData: cannot vom-encode data: key %s, version %s: %v", key, version, err)
	}
	if err := watchable.PutAtVersion(nil, tx, []byte(key), buf, []byte(version)); err != nil {
		t.Errorf("putBlobRefData: cannot PutAtVersion: key %s, version %s: %v", key, version, err)
	}
}

// TestProcessWatchLogBatch tests the processing of a batch of log records.
func TestProcessWatchLogBatch(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)
	st := createDatabase(t, svc).St()
	s := svc.sync

	fooKey := makeRowKey("foo")
	barKey := makeRowKey("bar")
	fooxyzKey := makeRowKey("fooxyz")

	// Empty logs does not fail.
	s.processWatchLogBatch(nil, mockDbId, st, nil, nil)

	// Non-syncable logs.
	batch := []*watchable.LogEntry{
		newLog(fooKey, "123", false),
		newLog(barKey, "555", false),
	}

	resmark := watchable.MakeResumeMarker(1234)
	s.processWatchLogBatch(nil, mockDbId, st, batch, resmark)

	if res, err := getResMark(nil, st); err != nil && !bytes.Equal(res, resmark) {
		t.Errorf("invalid resmark batch processing: got %s instead of %s", res, resmark)
	}
	if ok, err := hasNode(nil, st, fooKey, "123"); err != nil || ok {
		t.Error("hasNode() found DAG entry for non-syncable log on foo")
	}
	if ok, err := hasNode(nil, st, barKey, "555"); err != nil || ok {
		t.Error("hasNode() found DAG entry for non-syncable log on bar")
	}

	// Partially syncable logs.
	gid := interfaces.GroupId(1234)
	state := &SgLocalState{}
	tx := st.NewWatchableTransaction()
	if err := setSGIdEntry(nil, tx, gid, state); err != nil {
		t.Fatalf("setSGIdEntry() failed for gid %v", gid)
	}
	putBlobRefData(t, tx, fooKey, "333")
	putBlobRefData(t, tx, fooxyzKey, "444")
	if err := tx.Commit(); err != nil {
		t.Fatalf("cannot commit putting sg local state and some blob refs, err %v", err)
	}

	batch = []*watchable.LogEntry{
		newSGLog(gid, []string{"f", "x"}, false),
		newLog(fooKey, "333", false),
		newLog(fooxyzKey, "444", false),
		newLog(barKey, "222", false),
	}

	resmark = watchable.MakeResumeMarker(3456)
	s.processWatchLogBatch(nil, mockDbId, st, batch, resmark)

	if res, err := getResMark(nil, st); err != nil && !bytes.Equal(res, resmark) {
		t.Errorf("invalid resmark batch processing: got %s instead of %s", res, resmark)
	}
	if head, err := getHead(nil, st, fooKey); err != nil && head != "333" {
		t.Errorf("getHead() did not find foo: %s, %v", head, err)
	}
	node, err := getNode(nil, st, fooKey, "333")
	if err != nil {
		t.Errorf("getNode() did not find foo: %v", err)
	}
	if node.Level != 0 || node.Parents != nil || node.Logrec == "" || node.BatchId != NoBatchId {
		t.Errorf("invalid DAG node for foo: %v", node)
	}
	node2, err := getNode(nil, st, fooxyzKey, "444")
	if err != nil {
		t.Errorf("getNode() did not find fooxyz: %v", err)
	}
	if node2.Level != 0 || node2.Parents != nil || node2.Logrec == "" || node2.BatchId != NoBatchId {
		t.Errorf("invalid DAG node for fooxyz: %v", node2)
	}
	if ok, err := hasNode(nil, st, barKey, "222"); err != nil || ok {
		t.Error("hasNode() found DAG entry for non-syncable log on bar")
	}

	tx = st.NewWatchableTransaction()
	putBlobRefData(t, tx, fooKey, "1")
	if err := tx.Commit(); err != nil {
		t.Fatalf("cannot commit putting more blob refs, err %v", err)
	}

	// More partially syncable logs updating existing ones.
	batch = []*watchable.LogEntry{
		newLog(fooKey, "1", false),
		newLog(fooxyzKey, "", true),
		newLog(barKey, "7", false),
	}

	resmark = watchable.MakeResumeMarker(7890)
	s.processWatchLogBatch(nil, mockDbId, st, batch, resmark)

	if res, err := getResMark(nil, st); err != nil && !bytes.Equal(res, resmark) {
		t.Errorf("invalid resmark batch processing: got %s instead of %s", res, resmark)
	}
	if head, err := getHead(nil, st, fooKey); err != nil && head != "1" {
		t.Errorf("getHead() did not find foo: %s, %v", head, err)
	}
	node, err = getNode(nil, st, fooKey, "1")
	if err != nil {
		t.Errorf("getNode() did not find foo: %v", err)
	}
	expParents := []string{"333"}
	if node.Level != 1 || !reflect.DeepEqual(node.Parents, expParents) ||
		node.Logrec == "" || node.BatchId == NoBatchId {
		t.Errorf("invalid DAG node for foo: %v", node)
	}
	head2, err := getHead(nil, st, fooxyzKey)
	if err != nil {
		t.Errorf("getHead() did not find fooxyz: %v", err)
	}
	node2, err = getNode(nil, st, fooxyzKey, head2)
	if err != nil {
		t.Errorf("getNode() did not find fooxyz: %v", err)
	}
	expParents = []string{"444"}
	if node2.Level != 1 || !reflect.DeepEqual(node2.Parents, expParents) ||
		node2.Logrec == "" || node2.BatchId == NoBatchId {
		t.Errorf("invalid DAG node for fooxyz: %v", node2)
	}
	if ok, err := hasNode(nil, st, barKey, "7"); err != nil || ok {
		t.Error("hasNode() found DAG entry for non-syncable log on bar")
	}

	// Back to non-syncable logs (remove "f" prefix).
	batch = []*watchable.LogEntry{
		newSGLog(gid, []string{"f"}, true),
		newLog(fooKey, "99", false),
		newLog(fooxyzKey, "888", true),
		newLog(barKey, "007", false),
	}

	resmark = watchable.MakeResumeMarker(20212223)
	s.processWatchLogBatch(nil, mockDbId, st, batch, resmark)

	if res, err := getResMark(nil, st); err != nil && !bytes.Equal(res, resmark) {
		t.Errorf("invalid resmark batch processing: got %s instead of %s", res, resmark)
	}
	// No changes to "foo".
	if head, err := getHead(nil, st, fooKey); err != nil && head != "333" {
		t.Errorf("getHead() did not find foo: %s, %v", head, err)
	}
	if node, err := getNode(nil, st, fooKey, "99"); err == nil {
		t.Errorf("getNode() should not have found foo @ 99: %v", node)
	}
	if node, err := getNode(nil, st, fooxyzKey, "888"); err == nil {
		t.Errorf("getNode() should not have found fooxyz @ 888: %v", node)
	}
	if ok, err := hasNode(nil, st, barKey, "007"); err != nil || ok {
		t.Error("hasNode() found DAG entry for non-syncable log on bar")
	}

	// Verify the blob ref metadata.
	expBlobDir := make(map[wire.BlobRef]*interfaces.Signpost)
	expSp := &interfaces.Signpost{
		Locations: interfaces.PeerToLocationDataMap{s.name: interfaces.LocationData{WhenSeen: time.Now()}},
		SgIds:     sgSet{gid: struct{}{}},
	}
	expBlobDir[wire.BlobRef("br_"+fooKey+"_333")] = expSp
	expBlobDir[wire.BlobRef("br_"+fooxyzKey+"_444")] = expSp
	expBlobDir[wire.BlobRef("br_"+fooKey+"_1")] = expSp

	sps := s.bst.NewSignpostStream(svc.sync.ctx)
	spsCount := 0
	for sps.Advance() {
		spsCount++
		if expSp, ok := expBlobDir[sps.BlobId()]; !ok {
			t.Errorf("unexpectedly got unexpected signpost for blob %v", sps.BlobId())
		} else if sp := sps.Signpost(); !signpostApproxEq(&sp, expSp) {
			t.Errorf("signpost for blob %v: got %v, want %v", sps.BlobId(), sps.Signpost(), *expSp)
		}
	}
	if err := sps.Err(); err != nil {
		t.Errorf("NewSignpostStream yielded error  %v\n", err)
	}
	if spsCount != len(expBlobDir) {
		t.Errorf("got %d signposts, expected %d", spsCount, len(expBlobDir))
	}

	// Scan the batch records and verify that there is only 1 DAG batch
	// stored, with a total count of 3 and a map of 2 syncable entries.
	// This is because the 1st batch, while containing syncable keys, is a
	// syncgroup snapshot that does not get assigned a batch ID.  The 2nd
	// batch is an application batch with 3 keys of which 2 are syncable.
	// The 3rd batch is also a syncgroup snapshot.
	count := 0
	start, limit := common.ScanPrefixArgs(dagBatchPrefix, "")
	stream := st.Scan(start, limit)
	for stream.Advance() {
		count++
		key := string(stream.Key(nil))
		var info BatchInfo
		if err := vom.Decode(stream.Value(nil), &info); err != nil {
			t.Errorf("cannot decode batch %s: %v", key, err)
		}
		if info.Count != 3 {
			t.Errorf("wrong total count in batch %s: got %d instead of 3", key, info.Count)
		}
		if n := len(info.Objects); n != 2 {
			t.Errorf("wrong object count in batch %s: got %d instead of 2", key, n)
		}
	}
	if count != 1 {
		t.Errorf("wrong count of batches: got %d instead of 1", count)
	}
}
