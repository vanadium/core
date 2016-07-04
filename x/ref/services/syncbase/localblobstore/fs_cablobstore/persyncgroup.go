// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the PerSyncgroup part of the localblobstore interface.
// It passes the calls through to blobmap.

package fs_cablobstore

import "v.io/v23/context"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/server/interfaces"

// SetPerSyncgroup() sets the PerSyncgroup associated with a syncgroup to *psg.
func (fscabs *FsCaBlobStore) SetPerSyncgroup(ctx *context.T, sgId interfaces.GroupId, psg *localblobstore.PerSyncgroup) error {
	return fscabs.bm.SetPerSyncgroup(ctx, sgId, psg)
}

// GetPerSyncgroup() yields in *psg the PerSyncgroup associated with a syncgroup.
// If there is an error, *psg is set to a canonical empty PerSyncgroup.
// On return, it is guaranteed that any maps in *psg are non-nil.
func (fscabs *FsCaBlobStore) GetPerSyncgroup(ctx *context.T, sgId interfaces.GroupId, psg *localblobstore.PerSyncgroup) error {
	return fscabs.bm.GetPerSyncgroup(ctx, sgId, psg)
}

// DeletePerSyncgroup() deletes the PerSyncgroup for the specified blob.
func (fscabs *FsCaBlobStore) DeletePerSyncgroup(ctx *context.T, sgId interfaces.GroupId) error {
	return fscabs.bm.DeletePerSyncgroup(ctx, sgId)
}

// NewPerSyncgroupStream() returns a pointer to a PerSyncgroupStream
// that allows the client to iterate over each syncgroup for which a PerSyncgroup
// has been specified.
func (fscabs *FsCaBlobStore) NewPerSyncgroupStream(ctx *context.T) localblobstore.PerSyncgroupStream {
	return fscabs.bm.NewPerSyncgroupStream(ctx)
}
