// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"reflect"
	"testing"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// Tests local blob get following a put.
func TestLocalBlobPutGet(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")

	b, err := d.CreateBlob(ctx)
	if err != nil {
		t.Fatalf("Creating local blob failed, err %v", err)
	}

	// Verify that getting/fetching an uncommitted blob should fail.
	br, err := b.Get(ctx, 0)
	if err != nil {
		t.Fatalf("GetBlob RPC failed, err %v", err)
	}
	if br.Advance() {
		t.Fatalf("Getting uncommitted blob didn't fail")
	}
	if verror.ErrorID(br.Err()) != wire.ErrBlobNotCommitted.ID {
		t.Fatalf("Getting uncommitted blob didn't fail, err %v", err)
	}

	bs, err := b.Fetch(ctx, 100)
	if err != nil {
		t.Fatalf("FetchBlob RPC failed, err %v", err)
	}
	if bs.Advance() {
		t.Fatalf("Fetching uncommitted blob didn't fail")
	}
	if verror.ErrorID(bs.Err()) != wire.ErrBlobNotCommitted.ID {
		t.Fatalf("Fetching uncommitted blob didn't fail, err %v", err)
	}

	verifyBlobSize(t, ctx, b, 0)

	// Put some data in the blob.
	bw, err := b.Put(ctx)
	if err != nil {
		t.Fatalf("PutBlob RPC failed, err %v", err)
	}
	wantVal := "foobarbaz"
	wantSize := int64(len(wantVal))
	if err := bw.Send([]byte(wantVal)); err != nil {
		t.Fatalf("Sending blob data failed, err %v", err)
	}

	// Before the put is completed, the blob size can be anything between 0
	// and wantSize since the blob size is obtained from the persistent
	// storage before closing. Note: if Fatalf is used when there is an
	// error, the test hangs. Not sure why but looks like it is because
	// there is still a pending goroutine spawned for the Put rpc.
	gotSize, err := b.Size(ctx)
	if err != nil || gotSize > wantSize {
		t.Errorf("Getting blob size failed, got %v, want %v, err %v", gotSize, wantSize, err)
	}

	if err := bw.Close(); err != nil {
		t.Fatalf("Closing blob writer failed, err %v", err)
	}

	verifyBlobSize(t, ctx, b, wantSize)

	// Commit the blob.
	if err := b.Commit(ctx); err != nil {
		t.Fatalf("Committing a blob failed, err %v", err)
	}

	// Verify getting/fetching the committed blob.
	verifyBlobSize(t, ctx, b, wantSize)
	verifyGetBlob(t, ctx, b, 0, wantVal)
	verifyGetBlob(t, ctx, b, 3, "barbaz")

	bs, err = b.Fetch(ctx, 100)
	if err != nil {
		t.Fatalf("FetchBlob RPC failed, err %v", err)
	}
	wantStatus := wire.BlobFetchStatus{State: wire.BlobFetchStateDone}
	var gotStatus wire.BlobFetchStatus
	for bs.Advance() {
		gotStatus = bs.Value()
	}
	if bs.Err() != nil {
		t.Fatalf("Fetching a blob failed, err %v", br.Err())
	}
	if !reflect.DeepEqual(gotStatus, wantStatus) {
		t.Fatalf("Fetching a blob failed, got %v want %v", gotStatus, wantStatus)
	}
}

///////////////////
// Helpers.

func verifyBlobSize(t *testing.T, ctx *context.T, b syncbase.Blob, wantSize int64) {
	gotSize, err := b.Size(ctx)
	if err != nil || gotSize != wantSize {
		t.Fatalf("Getting blob size failed, got %v, want %v, err %v", gotSize, wantSize, err)
	}
}

func verifyGetBlob(t *testing.T, ctx *context.T, b syncbase.Blob, offset int64, wantVal string) {
	br, err := b.Get(ctx, offset)
	if err != nil {
		t.Fatalf("GetBlob RPC failed, err %v", err)
	}
	var gotVal []byte
	for br.Advance() {
		gotVal = append(gotVal, br.Value()...)
	}
	if br.Err() != nil {
		t.Fatalf("Getting a blob failed, err %v", br.Err())
	}
	if !reflect.DeepEqual(gotVal, []byte(wantVal)) {
		t.Fatalf("Getting a blob failed, got %v want %v", gotVal, []byte(wantVal))
	}
}
