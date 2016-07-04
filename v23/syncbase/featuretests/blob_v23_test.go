// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/x/ref/test/v23test"
)

func TestV23BlobWholeTransfer(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	sbs := setupSyncbases(t, sh, 2, false)

	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createCollection(sbs[0].clientCtx, sbs[0].sbName, testCx.Name))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", 0, 10))
	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "", nil, clBlessings(sbs)))
	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbs[0].sbName, sgId))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "", 0, 10))

	// FetchBlob first.
	ok(t, generateBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("foobarbaz")))
	ok(t, fetchBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, 9, false))
	ok(t, getBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, []byte("foobarbaz"), 0))

	// GetBlob directly.
	ok(t, generateBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, []byte("abcdefghijklmn")))
	// Sleep so that the update to key "foo0" makes it to the other side.
	time.Sleep(10 * time.Second)
	ok(t, getBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("fghijklmn"), 5))
	ok(t, fetchBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, 14, true))

	// Test with a big blob (1 MB).
	ok(t, generateBigBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 1))
	ok(t, getBigBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1))
}

////////////////////////////////////
// Helpers.

// TODO(sadovsky): Noticed while refactoring: there is a lot of duplicated code
// below, e.g. for generating blobs, getting testStruct values, etc. We should
// aim to avoid such duplication as it makes it more difficult to change APIs,
// fix bugs, etc.

type testStruct struct {
	Val  string
	Blob wire.BlobRef
}

func generateBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int, data []byte) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	b, err := d.CreateBlob(ctx)
	if err != nil {
		return fmt.Errorf("CreateBlob failed, err %v", err)
	}
	bw, err := b.Put(ctx)
	if err != nil {
		return fmt.Errorf("PutBlob RPC failed, err %v", err)
	}

	if err := bw.Send(data); err != nil {
		return fmt.Errorf("Sending blob data failed, err %v", err)
	}
	if err := bw.Close(); err != nil {
		return fmt.Errorf("Closing blob writer failed, err %v", err)
	}

	// Commit the blob.
	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("Committing a blob failed, err %v", err)
	}

	// Put the BlobRef in a key.
	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)
	s := testStruct{Val: "testkey" + key, Blob: b.Ref()}
	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}

	return nil
}

func fetchBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int, wantSize int64, skipIncStatus bool) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)
	var s testStruct

	// Try for 10 seconds to get the new value.
	var err error
	for i := 0; i < 10; i++ {
		// Note: the error is a decode error since the old value is a
		// string, and the new value is testStruct.
		if err = r.Get(ctx, &s); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("r.Get() failed: %v", err)
	}

	b := d.Blob(s.Blob)
	bs, err := b.Fetch(ctx, 100)
	if err != nil {
		return fmt.Errorf("Fetch RPC failed, err %v", err)
	}

	status := []wire.BlobFetchStatus{
		wire.BlobFetchStatus{State: wire.BlobFetchStatePending},
		wire.BlobFetchStatus{State: wire.BlobFetchStateLocating},
		wire.BlobFetchStatus{State: wire.BlobFetchStateFetching},
		wire.BlobFetchStatus{State: wire.BlobFetchStateDone}}

	var gotStatus wire.BlobFetchStatus
	i := 0
	for bs.Advance() {
		gotStatus = bs.Value()

		if !skipIncStatus {
			if i <= 1 {
				if !reflect.DeepEqual(gotStatus, status[i]) {
					return fmt.Errorf("Fetch blob failed, got status %v want status %v", gotStatus, status[i])
				}
				i++
			} else if !(gotStatus.State == status[2].State || reflect.DeepEqual(gotStatus, status[3])) {
				return fmt.Errorf("Fetch blob failed, got status %v", gotStatus)
			}
		}
	}

	if !reflect.DeepEqual(gotStatus, status[3]) {
		return fmt.Errorf("Fetch blob failed, got status %v want status %v", gotStatus, status[3])
	}

	if bs.Err() != nil {
		return fmt.Errorf("Fetch blob failed, err %v", err)
	}

	gotSize, err := b.Size(ctx)
	if err != nil || gotSize != wantSize {
		return fmt.Errorf("Blob size incorrect, got %v want %v, err %v", gotSize, wantSize, err)
	}

	return nil
}

func getBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int, wantVal []byte, offset int64) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)
	var s testStruct

	// Try for 10 seconds to get the new value.
	var err error
	for i := 0; i < 10; i++ {
		// Note: the error is a decode error since the old value is a
		// string, and the new value is testStruct.
		if err = r.Get(ctx, &s); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("r.Get() failed: %v", err)
	}

	b := d.Blob(s.Blob)
	br, err := b.Get(ctx, offset)
	if err != nil {
		return fmt.Errorf("GetBlob RPC failed, err %v", err)
	}
	var gotVal []byte
	for br.Advance() {
		gotVal = append(gotVal, br.Value()...)
	}
	if br.Err() != nil {
		return fmt.Errorf("Getting a blob failed, err %v", br.Err())
	}
	if !reflect.DeepEqual(gotVal, wantVal) {
		return fmt.Errorf("Getting a blob failed, got %v want %v", gotVal, wantVal)
	}

	return nil
}

func generateBigBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	b, err := d.CreateBlob(ctx)
	if err != nil {
		return fmt.Errorf("CreateBlob failed, err %v", err)
	}
	bw, err := b.Put(ctx)
	if err != nil {
		return fmt.Errorf("PutBlob RPC failed, err %v", err)
	}

	hasher := md5.New()

	chunkSize := 8192
	content := make([]byte, chunkSize)
	// Send 1 MB blob.
	for i := 0; i < 128; i++ {
		if n, err := rand.Read(content); err != nil || n != chunkSize {
			return fmt.Errorf("Creating blob data failed, n %v err %v", n, err)
		}
		if err := bw.Send(content); err != nil {
			return fmt.Errorf("Sending blob data failed, err %v", err)
		}
		hasher.Write(content)
	}
	if err := bw.Close(); err != nil {
		return fmt.Errorf("Closing blob writer failed, err %v", err)
	}

	// Commit the blob.
	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("Committing a blob failed, err %v", err)
	}

	// Put the BlobRef in a key.
	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)

	// Blob hash is transferred via structured store.
	s := testStruct{Val: hashToString(hasher.Sum(nil)), Blob: b.Ref()}

	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}

	return nil
}

func getBigBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)
	var s testStruct

	// Try for 10 seconds to get the new value.
	var err error
	for i := 0; i < 10; i++ {
		// Note: the error is a decode error since the old value is a
		// string, and the new value is testStruct.
		if err = r.Get(ctx, &s); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("r.Get() failed: %v", err)
	}

	b := d.Blob(s.Blob)
	br, err := b.Get(ctx, 0)
	if err != nil {
		return fmt.Errorf("GetBlob RPC failed, err %v", err)
	}
	hasher := md5.New()
	for br.Advance() {
		content := br.Value()
		hasher.Write(content)
	}
	if br.Err() != nil {
		return fmt.Errorf("Getting a blob failed, err %v", br.Err())
	}

	gotHash := hashToString(hasher.Sum(nil))
	if !reflect.DeepEqual(gotHash, s.Val) {
		return fmt.Errorf("Getting a blob failed, got %v want %v", gotHash, s.Val)
	}

	return nil
}

// Copied from localblobstore/fs_cablobstore/fs_cablobstore.go.
//
// hashToString() returns a string representation of the hash.
// Requires len(hash)==16.  An md5 hash is suitable.
func hashToString(hash []byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
		hash[0], hash[1], hash[2], hash[3],
		hash[4], hash[5], hash[6], hash[7],
		hash[8], hash[9], hash[10], hash[11],
		hash[12], hash[13], hash[14], hash[15])
}
