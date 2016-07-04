// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the BlobMetadata part of the localblobstore interface.
// It passes the calls through to blobmap.

package fs_cablobstore

import "v.io/v23/context"
import wire "v.io/v23/services/syncbase"
import "v.io/x/ref/services/syncbase/localblobstore"

// SetBlobMetadata() sets the BlobMetadata associated with a blob to *bmd.
func (fscabs *FsCaBlobStore) SetBlobMetadata(ctx *context.T, blobID wire.BlobRef, bmd *localblobstore.BlobMetadata) error {
	return fscabs.bm.SetBlobMetadata(ctx, blobID, bmd)
}

// GetBlobMetadata() yields in *bmd the BlobMetadata associated with a blob.
// If there is an error, *bmd is set to a canonical empty BlobMetadata.
// On return, it is guaranteed that any maps in *bmd are non-nil.
func (fscabs *FsCaBlobStore) GetBlobMetadata(ctx *context.T, blobID wire.BlobRef, bmd *localblobstore.BlobMetadata) (err error) {
	return fscabs.bm.GetBlobMetadata(ctx, blobID, bmd)
}

// DeleteBlobMetadata() deletes the BlobMetadata for the specified blob.
func (fscabs *FsCaBlobStore) DeleteBlobMetadata(ctx *context.T, blobID wire.BlobRef) error {
	return fscabs.bm.DeleteBlobMetadata(ctx, blobID)
}

// NewBlobMetadataStream() returns a pointer to a BlobMetadataStream
// that allows the client to iterate over each blob for which a BlobMetadata
// has been specified.
func (fscabs *FsCaBlobStore) NewBlobMetadataStream(ctx *context.T) localblobstore.BlobMetadataStream {
	return fscabs.bm.NewBlobMetadataStream(ctx)
}
