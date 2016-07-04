// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the Signpost part of the localblobstore interface.
// It passes the calls through to blobmap.

package fs_cablobstore

import "v.io/v23/context"
import wire "v.io/v23/services/syncbase"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/server/interfaces"

// SetSignpost() sets the Signpost associated with a blob to *sp.
func (fscabs *FsCaBlobStore) SetSignpost(ctx *context.T, blobID wire.BlobRef, sp *interfaces.Signpost) error {
	return fscabs.bm.SetSignpost(ctx, blobID, sp)
}

// GetSignpost() yields in *sp the Signpost associated with a blob.
// If there is an error, *sp is set to a canonical empty Signpost.
// On return, it is guaranteed that any maps in *sp are non-nil.
func (fscabs *FsCaBlobStore) GetSignpost(ctx *context.T, blobID wire.BlobRef, sp *interfaces.Signpost) error {
	return fscabs.bm.GetSignpost(ctx, blobID, sp)
}

// DeleteSignpost() deletes the Signpost for the specified blob.
func (fscabs *FsCaBlobStore) DeleteSignpost(ctx *context.T, blobID wire.BlobRef) error {
	return fscabs.bm.DeleteSignpost(ctx, blobID)
}

// NewSignpostStream() returns a pointer to a SignpostStream
// that allows the client to iterate over each blob for which a Signpost
// has been specified.
func (fscabs *FsCaBlobStore) NewSignpostStream(ctx *context.T) localblobstore.SignpostStream {
	return fscabs.bm.NewSignpostStream(ctx)
}
