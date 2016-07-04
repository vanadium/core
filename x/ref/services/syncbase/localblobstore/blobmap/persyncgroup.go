// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the PerSyncgroup table of a blobmap.

package blobmap

import "v.io/v23/context"
import "v.io/v23/verror"
import "v.io/v23/vom"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/server/interfaces"
import "v.io/x/ref/services/syncbase/store"

// perSyncgroupKey() places in keyBuf[] the key used in the store to reference
// the PerSyncgroup for sgId, and returns the correctly sized slice pointing
// into keyBuf.  The argument keyBuf is required to be large enough to hold the
// key.
func perSyncgroupKey(sgId interfaces.GroupId, keyBuf []byte) []byte {
	if len(keyBuf) < (len(perSyncgroupPrefix) + len(sgId)) {
		panic("keyBuf too short")
	}
	keyLen := copy(keyBuf[:], perSyncgroupPrefix)
	keyLen += copy(keyBuf[keyLen:], []byte(sgId))
	return keyBuf[:keyLen]
}

// SetPerSyncgroup() sets the PerSyncgroup associated with a syncgroup to *psg.
func (bm *BlobMap) SetPerSyncgroup(ctx *context.T, sgId interfaces.GroupId, psg *localblobstore.PerSyncgroup) (err error) {
	var val []byte

	val, err = vom.Encode(psg)
	if err == nil {
		var keyBuf [64]byte
		err = bm.st.Put(perSyncgroupKey(sgId, keyBuf[:]), val)
	}
	return err
}

// GetPerSyncgroup() yields in *psg the PerSyncgroup associated with a syncgroup.
// If there is an error, *psg is set to a canonical empty PerSyncgroup.
// On return, it is guaranteed that any maps in *psg are non-nil.
func (bm *BlobMap) GetPerSyncgroup(ctx *context.T, sgId interfaces.GroupId, psg *localblobstore.PerSyncgroup) (err error) {
	var keyBuf [64]byte
	var valBuf [maxPerSyncgroupLen]byte
	var val []byte

	val, err = bm.st.Get(perSyncgroupKey(sgId, keyBuf[:]), valBuf[:])
	if err == nil {
		err = vom.Decode(val, psg)
	}
	if err != nil {
		*psg = localblobstore.PerSyncgroup{}
	}
	return err
}

// DeletePerSyncgroup() deletes the PerSyncgroup for the specified syncgroup.
func (bm *BlobMap) DeletePerSyncgroup(ctx *context.T, sgId interfaces.GroupId) error {
	var keyBuf [64]byte
	return bm.st.Delete(perSyncgroupKey(sgId, keyBuf[:]))
}

// A PerSyncgroupStream allows the client to iterate over the PerSyncgroups associated with syncgroups.
//      psgs := bm.NewPerSyncgroupStream(ctx, sgId)
//      for psgs.Advance() {
//              syncgroup := psgs.SyncgroupName()
//              perSyncgroup := psgs.PerSyncgroup()
//              ...process blob, perSyncgroup...
//      }
//      if psgs.Err() != nil {
//              ...there was an error...
//      }
type PerSyncgroupStream struct {
	bm     *BlobMap
	ctx    *context.T
	stream store.Stream

	keyBuf       [maxKeyLen]byte             // buffer for keys
	valBuf       [maxPerSyncgroupLen]byte    // buffer for values
	sgId         interfaces.GroupId          // GroupId key for current element
	perSyncgroup localblobstore.PerSyncgroup // PerSyncgroup of current element
	err          error                       // error encountered.
	more         bool                        // whether stream may be consulted again
}

// NewPerSyncgroupStream() returns, as a localblobstore.BlobMetadataStream
// interface, a pointer to a new blobmap.PerSyncgroupStream that allows the
// client to enumerate the PerSyncgroups asscoaiedted with blob IDs, in order.
func (bm *BlobMap) NewPerSyncgroupStream(ctx *context.T) localblobstore.PerSyncgroupStream {
	psgs := new(PerSyncgroupStream)
	psgs.bm = bm
	psgs.ctx = ctx

	psgs.stream = bm.st.Scan([]byte(perSyncgroupPrefix), perSyncgroupStreamKeyLimit)
	psgs.more = true
	return psgs
}

// Advance() stages an element so the client can retrieve the blob ID hash with
// BlobID(), or its PerSyncgroup with PerSyncgroup().  Advance() returns true iff there
// is an element to retrieve.  The client must call Advance() before calling
// BlobID() or PerSyncgroup() The client must call Cancel if it does not iterate
// through all elements (i.e. until Advance() returns false).  Advance() may
// block if an element is not immediately available.
func (psgs *PerSyncgroupStream) Advance() (ok bool) {
	if psgs.more && psgs.err == nil {
		if !psgs.stream.Advance() {
			psgs.err = psgs.stream.Err()
			psgs.more = false // no more stream, even if no error
		} else {
			var key []byte = psgs.stream.Key(psgs.keyBuf[:])
			psgs.sgId = interfaces.GroupId(key[1:])
			var value []byte = psgs.stream.Value(psgs.valBuf[:])
			psgs.err = vom.Decode(value, &psgs.perSyncgroup)
			// The syncgroup ID is always a 43-byte string.
			if psgs.err == nil && len(key) != len(perSyncgroupPrefix)+43 {
				psgs.err = verror.New(errMalformedPerSyncgroupEntry, psgs.ctx, psgs.bm.dir, key, value)
			}
			ok = (psgs.err == nil)
			if !ok {
				psgs.stream.Cancel()
			}
		}
	}
	return ok
}

// SyncgroupID() returns the syncgroup Id staged by Advance().  SyncgroupID()
// may panic if Advance() returned false or was not called at all.
// SyncgroupID() does not block.
func (psgs *PerSyncgroupStream) SyncgroupId() interfaces.GroupId {
	return psgs.sgId
}

// PerSyncgroup() returns the PerSyncgroup associated with the blob ID staged by
// Advance().  PerSyncgroup() may panic if Advance() returned false or was not
// called at all.  PerSyncgroup() does not block.
func (psgs *PerSyncgroupStream) PerSyncgroup() localblobstore.PerSyncgroup {
	return psgs.perSyncgroup
}

// Err() returns a non-nil error iff the stream encountered any errors.  Err()
// does not block.
func (psgs *PerSyncgroupStream) Err() error {
	return psgs.err
}

// Cancel() notifies the stream provider that it can stop producing elements.
// The client must call Cancel() if it does not iterate through all elements
// (i.e. until Advance() returns false).  Cancel() is idempotent and can be
// called concurrently with a goroutine that is iterating via Advance() and
// BlobID().  Cancel() causes Advance() to subsequently return false.
// Cancel() does not block.
func (psgs *PerSyncgroupStream) Cancel() {
	psgs.stream.Cancel()
}
