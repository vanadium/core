// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A test for the Signpost table in blobmap.

package blobmap_test

import "fmt"
import "io/ioutil"
import "os"
import "testing"
import "time"

import wire "v.io/v23/services/syncbase"
import "v.io/v23/verror"
import "v.io/x/ref/services/syncbase/localblobstore/blobmap"
import "v.io/x/ref/services/syncbase/server/interfaces"
import "v.io/x/ref/services/syncbase/store"
import "v.io/x/ref/test"
import _ "v.io/x/ref/runtime/factories/generic"

// signpostEq() returns whether Signpost objects *a and *b are equal.
func signpostEq(a *interfaces.Signpost, b *interfaces.Signpost) bool {
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
		if bLocData, exists := b.Locations[peer]; !exists || !aLocData.WhenSeen.Equal(bLocData.WhenSeen) {
			return false
		}
	}
	return true
}

// TestAddRetrieveAndDeleteSignpost() tests insertion, retrieval, and deletion
// of Signpost entries from a BlobMap.  It's all done in one test case, because
// one cannot retrieve or delete Signpost entries that have not been inserted.
func TestAddRetrieveAndDeleteSignpost(t *testing.T) {
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

	// Two blob Ids: b[0] and b[1].
	b := []wire.BlobRef{"foo", "bar"}

	var spList []interfaces.Signpost
	for i := range b {
		var sp interfaces.Signpost
		err = bm.GetSignpost(ctx, b[i], &sp)
		if err == nil {
			t.Errorf("blobmap_test: blob: %v already has a Signpost", b[i])
		}
	}
	locationData := interfaces.LocationData{WhenSeen: time.Now()}
	for i := range b {
		var sp interfaces.Signpost
		sp.Locations = make(interfaces.PeerToLocationDataMap)
		sp.Locations[fmt.Sprintf("peer of blob %d", i)] = locationData
		sp.Locations[fmt.Sprintf("source of blob %d", i)] = locationData
		spList = append(spList, sp)
		err = bm.SetSignpost(ctx, b[i], &sp)
		if err != nil {
			t.Errorf("blobmap_test: SetSignpost(%v, %v) got error: %v", b[i], sp, err)
		}
	}
	for i := range b {
		var sp interfaces.Signpost
		err = bm.GetSignpost(ctx, b[i], &sp)
		if err != nil {
			t.Errorf("blobmap_test: GetSignpost(%v) got error: %v", b[i], err)
		} else if !signpostEq(&sp, &spList[i]) {
			t.Errorf("blobmap_test: GetSignpost(%v) got wrong content: %#v, want %#v", b[i], sp, spList[i])
		}
	}

	// Test iteration through the Signpost entries.
	count := 0
	ss := bm.NewSignpostStream(ctx)
	for ss.Advance() {
		blob := ss.BlobId()
		var sp interfaces.Signpost = ss.Signpost()
		// Find the blob in the list.
		var i int
		for i = 0; i != len(b) && blob != b[i]; i++ {
		}
		if i == len(b) {
			t.Errorf("blobmap_test: SignpostStream iteration %d, got blob Id %v, which is not in %v", count, blob, b)
		} else if !signpostEq(&sp, &spList[i]) {
			t.Errorf("blobmap_test: SignpostStream iteration %d, blob[%d]=%v, got Signpost %v, want %v", count, i, blob, sp, spList[i])
		}
		count++
	}
	if count != len(b) {
		t.Errorf("blobmap_test: SignpostStream got %d elements, wanted %d", count, len(b))
	}
	if err = ss.Err(); err != nil {
		t.Errorf("blobmap_test: SignpostStream unexpectedly gave error: %v", err)
	}

	// Test cancellation after first element retrieved.
	count = 0
	ss = bm.NewSignpostStream(ctx)
	for ss.Advance() {
		ss.Cancel()
		count++
	}
	if count != 1 {
		t.Errorf("blobmap_test: SignpostStream unexpectedly didn't give one element; got %d", count)
	}
	if err = ss.Err(); verror.ErrorID(err) != verror.ErrCanceled.ID {
		t.Errorf("blobmap_test: SignpostStream unexpectedly wasn't cancelled: err is %v", err)
	}

	// Delete the first Signpost.
	if err = bm.DeleteSignpost(ctx, b[0]); err != nil {
		t.Errorf("blobmap_test: DeleteSignpost(%v) got error: %v", b[0], err)
	}

	// Check that we can no longer get the first element, and we can get the others.
	for i := range b {
		var sp interfaces.Signpost
		err = bm.GetSignpost(ctx, b[i], &sp)
		if i == 0 {
			if err == nil {
				t.Errorf("blobmap_test: GetSignpost(%v) unexpectedly failed to get error on deleted blob, returned %v", b[i], sp)
			}
		} else if err != nil {
			t.Errorf("blobmap_test: GetSignpost(%v) got error: %v", b[i], err)
		} else if !signpostEq(&sp, &spList[i]) {
			t.Errorf("blobmap_test: GetSignpost(%v) got wrong content: %v, want %v", b[i], sp, spList[i])
		}
	}

	// Test iteration with first Signpost deleted.
	count = 0
	ss = bm.NewSignpostStream(ctx)
	for ss.Advance() {
		blob := ss.BlobId()
		var sp interfaces.Signpost = ss.Signpost()
		// Find the blob in the list, ignoring the first.
		var i int
		for i = 1; i != len(b) && blob != b[i]; i++ {
		}
		if i == len(b) {
			t.Errorf("blobmap_test: SignpostStream iteration %d, got blob Id %v, which is not in %v", count, blob, b[1:])
		} else if !signpostEq(&sp, &spList[i]) {
			t.Errorf("blobmap_test: SignpostStream iteration %d, blob[%d]=%v, got Signpost %v, want %v", count, i, blob, sp, spList[i])
		}
		count++
	}
	if count != len(b)-1 {
		t.Errorf("blobmap_test: SignpostStream got %d elements, wanted %d", count, len(b)-1)
	}
	if err = ss.Err(); err != nil {
		t.Errorf("blobmap_test: SignpostStream unexpectedly gave error: %v", err)
	}

	// Delete remaining Signpost entries.
	for i := 1; i != len(b); i++ {
		if err = bm.DeleteSignpost(ctx, b[i]); err != nil {
			t.Errorf("blobmap_test: DeleteSignpost(%v) got error: %v", b[i], err)
		}
	}

	// Check that all the elements are no longer there.
	for i := range b {
		var sp interfaces.Signpost
		err = bm.GetSignpost(ctx, b[i], &sp)
		if err == nil {
			t.Errorf("blobmap_test: GetSignpost(%v) unexpectedly failed to get error on deleted blob, returned %v", b[i], sp)
		}
	}

	// Check that iteration finds nothing.
	count = 0
	ss = bm.NewSignpostStream(ctx)
	for ss.Advance() {
		count++
	}
	if count != 0 {
		t.Errorf("blobmap_test: SignpostStream unexpectedly didn't give zero elements; got %d", count)
	}
	if err = ss.Err(); err != nil {
		t.Errorf("blobmap_test: SignpostStream unexpectedly got error: %v", err)
	}

	err = bm.Close()
	if err != nil {
		t.Errorf("blobmap_test: unexpected error closing BlobMap: %v", err)
	}
}
