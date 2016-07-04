// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A test for the BlobMetadata table in blobmap.

package blobmap_test

import "io/ioutil"
import "os"
import "testing"

import wire "v.io/v23/services/syncbase"
import "v.io/v23/verror"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/localblobstore/blobmap"
import "v.io/x/ref/services/syncbase/server/interfaces"
import "v.io/x/ref/services/syncbase/store"
import "v.io/x/ref/test"
import _ "v.io/x/ref/runtime/factories/generic"

// blobMetadataEqual() returns whether *a and *b are equal.
// We do not use reflect.DeepEqual(), because BlobMetadata contains time.Time values,
// whose useless Location field is often different, even though the time is the same.
// Currently, it checks only the OwnerShares field.
func blobMetadataEqual(a *localblobstore.BlobMetadata, b *localblobstore.BlobMetadata) bool {
	equal := len(a.OwnerShares) == len(b.OwnerShares)
	for ak, av := range a.OwnerShares {
		if !equal {
			break
		}
		if bv, ok := b.OwnerShares[ak]; ok {
			equal = av == bv
		} else {
			equal = false
		}
	}
	return equal
}

// TestAddRetrieveAndDeleteBlobMetadata() tests insertion, retrieval, and deletion
// of BlobMetadata entries from a BlobMap.  It's all done in one test case, because
// one cannot retrieve or delete BlobMetadata entries that have not been inserted.
func TestAddRetrieveAndDeleteBlobMetadata(t *testing.T) {
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

	var bmd localblobstore.BlobMetadata
	var bmdList []localblobstore.BlobMetadata
	for i := range b {
		err = bm.GetBlobMetadata(ctx, b[i], &bmd)
		if err == nil {
			t.Errorf("blobmap_test: blob: %v already has a BlobMetadata", b[i])
		}
	}
	for i := range b {
		bmd.OwnerShares = make(interfaces.BlobSharesBySyncgroup)
		bmd.OwnerShares["fakesyncgroupidshouldbe43byteslong123456789"] = 17 + int32(i)
		bmdList = append(bmdList, bmd)
		err = bm.SetBlobMetadata(ctx, b[i], &bmd)
		if err != nil {
			t.Errorf("blobmap_test: SetBlobMetadata(%v, %v) got error: %v", b[i], bmd, err)
		}
	}
	for i := range b {
		err = bm.GetBlobMetadata(ctx, b[i], &bmd)
		if err != nil {
			t.Errorf("blobmap_test: GetBlobMetadata(%v) got error: %v", b[i], err)
		} else if !blobMetadataEqual(&bmd, &bmdList[i]) {
			t.Errorf("blobmap_test: GetBlobMetadata(%v) got wrong content: %v, want %v", b[i], bmd, bmdList[i])
		}
	}

	// Test iteration through the BlobMetadata entries.
	count := 0
	ms := bm.NewBlobMetadataStream(ctx)
	for ms.Advance() {
		blob := ms.BlobId()
		bmd = ms.BlobMetadata()
		// Find the blob in the list.
		var i int
		for i = 0; i != len(b) && blob != b[i]; i++ {
		}
		if i == len(b) {
			t.Errorf("blobmap_test: BlobMetadataStream iteration %d, got blob Id %v, which is not in %v", count, blob, b)
		} else if !blobMetadataEqual(&bmd, &bmdList[i]) {
			t.Errorf("blobmap_test: BlobMetadataStream iteration %d, blob[%d]=%v, got BlobMetadata %v, want %v", count, i, blob, bmd, bmdList[i])
		}
		count++
	}
	if count != len(b) {
		t.Errorf("blobmap_test: BlobMetadataStream got %d elements, wanted %d", count, len(b))
	}
	if err = ms.Err(); err != nil {
		t.Errorf("blobmap_test: BlobMetadataStream unexpectedly gave error: %v", err)
	}

	// Test cancellation after first element retrieved.
	count = 0
	ms = bm.NewBlobMetadataStream(ctx)
	for ms.Advance() {
		ms.Cancel()
		count++
	}
	if count != 1 {
		t.Errorf("blobmap_test: BlobMetadataStream unexpectedly didn't give one element; got %d", count)
	}
	if err = ms.Err(); verror.ErrorID(err) != verror.ErrCanceled.ID {
		t.Errorf("blobmap_test: BlobMetadataStream unexpectedly wasn't cancelled: err is %v", err)
	}

	// Delete the first BlobMetadata.
	if err = bm.DeleteBlobMetadata(ctx, b[0]); err != nil {
		t.Errorf("blobmap_test: DeleteBlobMetadata(%v) got error: %v", b[0], err)
	}

	// Check that we can no longer get the first element, and we can get the others.
	for i := range b {
		err = bm.GetBlobMetadata(ctx, b[i], &bmd)
		if i == 0 {
			if err == nil {
				t.Errorf("blobmap_test: GetBlobMetadata(%v) unexpectedly failed to get error on deleted blob, returned %v", b[i], bmd)
			}
		} else if err != nil {
			t.Errorf("blobmap_test: GetBlobMetadata(%v) got error: %v", b[i], err)
		} else if !blobMetadataEqual(&bmd, &bmdList[i]) {
			t.Errorf("blobmap_test: GetBlobMetadata(%v) got wrong content: %v, want %v", b[i], bmd, bmdList[i])
		}
	}

	// Test iteration with first BlobMetadata deleted.
	count = 0
	ms = bm.NewBlobMetadataStream(ctx)
	for ms.Advance() {
		blob := ms.BlobId()
		bmd = ms.BlobMetadata()
		// Find the blob in the list, ignoring the first.
		var i int
		for i = 1; i != len(b) && blob != b[i]; i++ {
		}
		if i == len(b) {
			t.Errorf("blobmap_test: BlobMetadataStream iteration %d, got blob Id %v, which is not in %v", count, blob, b[1:])
		} else if !blobMetadataEqual(&bmd, &bmdList[i]) {
			t.Errorf("blobmap_test: BlobMetadataStream iteration %d, blob[%d]=%v, got BlobMetadata %v, want %v", count, i, blob, bmd, bmdList[i])
		}
		count++
	}
	if count != len(b)-1 {
		t.Errorf("blobmap_test: BlobMetadataStream got %d elements, wanted %d", count, len(b)-1)
	}
	if err = ms.Err(); err != nil {
		t.Errorf("blobmap_test: BlobMetadataStream unexpectedly gave error: %v", err)
	}

	// Delete remaining BlobMetadata entries.
	for i := 1; i != len(b); i++ {
		if err = bm.DeleteBlobMetadata(ctx, b[i]); err != nil {
			t.Errorf("blobmap_test: DeleteBlobMetadata(%v) got error: %v", b[i], err)
		}
	}

	// Check that all the elements are no longer there.
	for i := range b {
		err = bm.GetBlobMetadata(ctx, b[i], &bmd)
		if err == nil {
			t.Errorf("blobmap_test: GetBlobMetadata(%v) unexpectedly failed to get error on deleted blob, returned %v", b[i], bmd)
		}
	}

	// Check that iteration finds nothing.
	count = 0
	ms = bm.NewBlobMetadataStream(ctx)
	for ms.Advance() {
		count++
	}
	if count != 0 {
		t.Errorf("blobmap_test: BlobMetadataStream unexpectedly didn't give zero elements; got %d", count)
	}
	if err = ms.Err(); err != nil {
		t.Errorf("blobmap_test: BlobMetadataStream unexpectedly got error: %v", err)
	}

	err = bm.Close()
	if err != nil {
		t.Errorf("blobmap_test: unexpected error closing BlobMap: %v", err)
	}
}
