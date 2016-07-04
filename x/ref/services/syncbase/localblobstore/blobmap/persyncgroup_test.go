// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A test for the PerSyncgroup table in blobmap.

package blobmap_test

import "io/ioutil"
import "os"
import "testing"

import "v.io/v23/verror"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/localblobstore/blobmap"
import "v.io/x/ref/services/syncbase/server/interfaces"
import "v.io/x/ref/services/syncbase/store"
import "v.io/x/ref/test"
import _ "v.io/x/ref/runtime/factories/generic"

// perSyncgroupEqual() returns whether *a and *b are equal.
// We do not use reflect.DeepEqual(), because PerSyncgroup contains time.Time values,
// whose useless Location field is often different, even though the time is the same.
// Currently, it checks only the Priority.Distance field.
func perSyncgroupEqual(a *localblobstore.PerSyncgroup, b *localblobstore.PerSyncgroup) bool {
	return a.Priority.Distance == b.Priority.Distance
}

// TestAddRetrieveAndDeletePerSyncgroup() tests insertion, retrieval, and deletion
// of PerSyncgroup entries from a BlobMap.  It's all done in one test case, because
// one cannot retrieve or delete PerSyncgroup entries that have not been inserted.
func TestAddRetrieveAndDeletePerSyncgroup(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Make a temporary directory.
	var err error
	var testDirName string
	testDirName, err = ioutil.TempDir("", "blobmap_test")
	if err != nil {
		t.Fatalf("blobmap_test: can't make tmp directory: %v", err)
	}
	defer os.RemoveAll(testDirName)

	// Create a blobmap.
	var bm *blobmap.BlobMap
	bm, err = blobmap.New(ctx, store.EngineForTest, testDirName)
	if err != nil {
		t.Fatalf("blobmap_test: blobmap.New failed: %v", err)
	}

	// Two syncgroup IDs: sgs[0] and sgs[1].
	// Mimic actual syncgroup IDs: 43-byte strings.
	sgs := []interfaces.GroupId{"foofoofoofoofoofoofoofoofoofoofoofoofoofoo0", "barbarbarbarbarbarbarbarbarbarbarbarbarbar1"}

	var psg localblobstore.PerSyncgroup
	var psgList []localblobstore.PerSyncgroup
	for i := range sgs {
		err = bm.GetPerSyncgroup(ctx, sgs[i], &psg)
		if err == nil {
			t.Errorf("blobmap_test: blob: %v already has a PerSyncgroup", sgs[i])
		}
	}
	for i := range sgs {
		psg.Priority.Distance = 17
		psgList = append(psgList, psg)
		err = bm.SetPerSyncgroup(ctx, sgs[i], &psg)
		if err != nil {
			t.Errorf("blobmap_test: SetPerSyncgroup(%v, %v) got error: %v", sgs[i], psg, err)
		}
	}
	for i := range sgs {
		err = bm.GetPerSyncgroup(ctx, sgs[i], &psg)
		if err != nil {
			t.Errorf("blobmap_test: GetPerSyncgroup(%v) got error: %v", sgs[i], err)
		} else if !perSyncgroupEqual(&psg, &psgList[i]) {
			t.Errorf("blobmap_test: GetPerSyncgroup(%v) got wrong content: %v, want %v", sgs[i], psg, psgList[i])
		}
	}

	// Test iteration through the PerSyncgroup entries.
	count := 0
	psgs := bm.NewPerSyncgroupStream(ctx)
	for psgs.Advance() {
		sg := psgs.SyncgroupId()
		psg = psgs.PerSyncgroup()
		// Find the syncgroup in the list.
		var i int
		for i = 0; i != len(sgs) && sg != sgs[i]; i++ {
		}
		if i == len(sgs) {
			t.Errorf("blobmap_test: PerSyncgroupStream iteration %d, got syncgropup name %v, which is not in %v", count, sg, sgs)
		} else if !perSyncgroupEqual(&psg, &psgList[i]) {
			t.Errorf("blobmap_test: PerSyncgroupStream iteration %d, sgs[%d]=%v, got PerSyncgroup %v, want %v", count, i, sg, psg, psgList[i])
		}
		count++
	}
	if count != len(sgs) {
		t.Errorf("blobmap_test: PerSyncgroupStream got %d elements, wanted %d", count, len(sgs))
	}
	if err = psgs.Err(); err != nil {
		t.Errorf("blobmap_test: PerSyncgroupStream unexpectedly gave error: %v", err)
	}

	// Test cancellation after first element retrieved.
	count = 0
	psgs = bm.NewPerSyncgroupStream(ctx)
	for psgs.Advance() {
		psgs.Cancel()
		count++
	}
	if count != 1 {
		t.Errorf("blobmap_test: PerSyncgroupStream unexpectedly didn't give one element; got %d", count)
	}
	if err = psgs.Err(); verror.ErrorID(err) != verror.ErrCanceled.ID {
		t.Errorf("blobmap_test: PerSyncgroupStream unexpectedly wasn't cancelled: err is %v", err)
	}

	// Delete the first PerSyncgroup.
	if err = bm.DeletePerSyncgroup(ctx, sgs[0]); err != nil {
		t.Errorf("blobmap_test: DeletePerSyncgroup(%v) got error: %v", sgs[0], err)
	}

	// Check that we can no longer get the first element, and we can get the others.
	for i := range sgs {
		err = bm.GetPerSyncgroup(ctx, sgs[i], &psg)
		if i == 0 {
			if err == nil {
				t.Errorf("blobmap_test: GetPerSyncgroup(%v) unexpectedly failed to get error on deleted blob, returned %v", sgs[i], psg)
			}
		} else if err != nil {
			t.Errorf("blobmap_test: GetPerSyncgroup(%v) got error: %v", sgs[i], err)
		} else if !perSyncgroupEqual(&psg, &psgList[i]) {
			t.Errorf("blobmap_test: GetPerSyncgroup(%v) got wrong content: %v, want %v", sgs[i], psg, psgList[i])
		}
	}

	// Test iteration with first PerSyncgroup deleted.
	count = 0
	psgs = bm.NewPerSyncgroupStream(ctx)
	for psgs.Advance() {
		sg := psgs.SyncgroupId()
		psg = psgs.PerSyncgroup()
		// Find the blob in the list, ignoring the first.
		var i int
		for i = 1; i != len(sgs) && sg != sgs[i]; i++ {
		}
		if i == len(sgs) {
			t.Errorf("blobmap_test: PerSyncgroupStream iteration %d, got syncgroup name %v, which is not in %v", count, sg, sgs[1:])
		} else if !perSyncgroupEqual(&psg, &psgList[i]) {
			t.Errorf("blobmap_test: PerSyncgroupStream iteration %d, sgs[%d]=%v, got PerSyncgroup %v, want %v", count, i, sg, psg, psgList[i])
		}
		count++
	}
	if count != len(sgs)-1 {
		t.Errorf("blobmap_test: PerSyncgroupStream got %d elements, wanted %d", count, len(sgs)-1)
	}
	if err = psgs.Err(); err != nil {
		t.Errorf("blobmap_test: PerSyncgroupStream unexpectedly gave error: %v", err)
	}

	// Delete remaining PerSyncgroup entries.
	for i := 1; i != len(sgs); i++ {
		if err = bm.DeletePerSyncgroup(ctx, sgs[i]); err != nil {
			t.Errorf("blobmap_test: DeletePerSyncgroup(%v) got error: %v", sgs[i], err)
		}
	}

	// Check that all the elements are no longer there.
	for i := range sgs {
		err = bm.GetPerSyncgroup(ctx, sgs[i], &psg)
		if err == nil {
			t.Errorf("blobmap_test: GetPerSyncgroup(%v) unexpectedly failed to get error on deleted blob, returned %v", sgs[i], psg)
		}
	}

	// Check that iteration finds nothing.
	count = 0
	psgs = bm.NewPerSyncgroupStream(ctx)
	for psgs.Advance() {
		count++
	}
	if count != 0 {
		t.Errorf("blobmap_test: PerSyncgroupStream unexpectedly didn't give zero elements; got %d", count)
	}
	if err = psgs.Err(); err != nil {
		t.Errorf("blobmap_test: PerSyncgroupStream unexpectedly got error: %v", err)
	}

	err = bm.Close()
	if err != nil {
		t.Errorf("blobmap_test: unexpected error closing BlobMap: %v", err)
	}
}
