// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/vsync"
)

////////////////////////////////////////////////////////////////////////////////
// RPCs for managing blobs between Syncbase and its clients.

func (d *database) CreateBlob(ctx *context.T, call rpc.ServerCall) (wire.BlobRef, error) {
	if !d.exists {
		return wire.NullBlobRef, verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.CreateBlob(ctx, call)
}

func (d *database) PutBlob(ctx *context.T, call wire.BlobManagerPutBlobServerCall, br wire.BlobRef) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.PutBlob(ctx, call, br)
}

func (d *database) CommitBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.CommitBlob(ctx, call, br)
}

func (d *database) GetBlobSize(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) (int64, error) {
	if !d.exists {
		return 0, verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.GetBlobSize(ctx, call, br)
}

func (d *database) DeleteBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.DeleteBlob(ctx, call, br)
}

func (d *database) GetBlob(ctx *context.T, call wire.BlobManagerGetBlobServerCall, br wire.BlobRef, offset int64) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.GetBlob(ctx, call, br, offset)
}

func (d *database) FetchBlob(ctx *context.T, call wire.BlobManagerFetchBlobServerCall, br wire.BlobRef, priority uint64) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.FetchBlob(ctx, call, br, priority)
}

func (d *database) PinBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.PinBlob(ctx, call, br)
}

func (d *database) UnpinBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.UnpinBlob(ctx, call, br)
}

func (d *database) KeepBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef, rank uint64) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	sd := vsync.NewSyncDatabase(d)
	return sd.KeepBlob(ctx, call, br, rank)
}
