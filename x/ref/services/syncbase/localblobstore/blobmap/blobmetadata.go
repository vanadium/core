// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the BlobMetadata table of a blobmap.

package blobmap

import "v.io/v23/context"
import wire "v.io/v23/services/syncbase"
import "v.io/v23/vom"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/server/interfaces"
import "v.io/x/ref/services/syncbase/store"

// metadataKey() returns the key used in the store to reference the metadata
// for blobId.
func metadataKey(blobId wire.BlobRef) []byte {
	return []byte(metadataPrefix + string(blobId))
}

// SetBlobMetadata() sets the BlobMetadata associated with a blob to *bmd.
func (bm *BlobMap) SetBlobMetadata(ctx *context.T, blobId wire.BlobRef, bmd *localblobstore.BlobMetadata) (err error) {
	var val []byte

	val, err = vom.Encode(bmd)
	if err == nil {
		err = bm.st.Put(metadataKey(blobId), val)
	}
	return err
}

// GetBlobMetadata() yields in *bmd the BlobMetadata associated with a blob.
// If there is an error, *bmd is set to a canonical empty BlobMetadata.
// On return, it is guaranteed that any maps in *bmd are non-nil.
func (bm *BlobMap) GetBlobMetadata(ctx *context.T, blobId wire.BlobRef, bmd *localblobstore.BlobMetadata) (err error) {
	var valBuf [maxBlobMetadataLen]byte
	var val []byte

	val, err = bm.st.Get(metadataKey(blobId), valBuf[:])
	if err == nil {
		err = vom.Decode(val, bmd)
	}
	if err != nil {
		*bmd = localblobstore.BlobMetadata{}
	}
	if bmd.OwnerShares == nil {
		bmd.OwnerShares = make(interfaces.BlobSharesBySyncgroup)
	}
	return err
}

// DeleteBlobMetadata() deletes the BlobMetadata for the specified blob.
func (bm *BlobMap) DeleteBlobMetadata(ctx *context.T, blobId wire.BlobRef) error {
	return bm.st.Delete(metadataKey(blobId))
}

// A BlobMetadataStream allows the client to iterate over the BlobMetadatas associated with blob IDs.
//      bms := bm.NewBlobMetadataStream(ctx, blob)
//      for bms.Advance() {
//              blob := bms.BlobId()
//              metadata := bms.BlobMetadata()
//              ...process blob, metadata...
//      }
//      if bms.Err() != nil {
//              ...there was an error...
//      }
type BlobMetadataStream struct {
	bm     *BlobMap
	stream store.Stream

	keyBuf   [maxKeyLen]byte             // buffer for keys
	valBuf   [maxBlobMetadataLen]byte    // buffer for values
	blobId   wire.BlobRef                // blobId key for current element
	metadata localblobstore.BlobMetadata // BlobMetadata of current element
	err      error                       // error encountered.
	more     bool                        // whether stream may be consulted again
}

// NewBlobMetadataStream() returns, as a localblobstore.BlobMetadataStream
// interface, a pointer to a new blobmap.BlobMetadataStream that allows the
// client to enumerate the BlobMetadatas associated with blob IDs, in order.
func (bm *BlobMap) NewBlobMetadataStream(ctx *context.T) localblobstore.BlobMetadataStream {
	// ctx is currently unused.
	bms := new(BlobMetadataStream)
	bms.bm = bm
	bms.stream = bm.st.Scan([]byte(metadataPrefix), metadataStreamKeyLimit)
	bms.more = true
	return bms
}

// Advance() stages an element so the client can retrieve the blob ID hash with
// BlobId(), or its BlobMetadata with BlobMetadata().  Advance() returns true iff there
// is an element to retrieve.  The client must call Advance() before calling
// BlobId() or BlobMetadata() The client must call Cancel if it does not iterate
// through all elements (i.e. until Advance() returns false).  Advance() may
// block if an element is not immediately available.
func (bms *BlobMetadataStream) Advance() (ok bool) {
	if bms.more && bms.err == nil {
		if !bms.stream.Advance() {
			bms.err = bms.stream.Err()
			bms.more = false // no more stream, even if no error
		} else {
			bms.blobId = wire.BlobRef(bms.stream.Key(bms.keyBuf[:])[1:])
			var value []byte = bms.stream.Value(bms.valBuf[:])
			bms.err = vom.Decode(value, &bms.metadata)
			ok = (bms.err == nil)
			if !ok {
				bms.stream.Cancel()
			}
		}
	}
	return ok
}

// BlobId() returns the blob ID of the blob Id and BlobMetadata staged by
// Advance().  BlobId() may panic if Advance() returned false or was not called
// at all.  BlobId() does not block.
func (bms *BlobMetadataStream) BlobId() (result wire.BlobRef) {
	return bms.blobId
}

// BlobMetadata() returns the BlobMetadata associated with the blob ID staged by
// Advance().  BlobMetadata() may panic if Advance() returned false or was not
// called at all.  BlobMetadata() does not block.
func (bms *BlobMetadataStream) BlobMetadata() localblobstore.BlobMetadata {
	return bms.metadata
}

// Err() returns a non-nil error iff the stream encountered any errors.  Err()
// does not block.
func (bms *BlobMetadataStream) Err() error {
	return bms.err
}

// Cancel() notifies the stream provider that it can stop producing elements.
// The client must call Cancel() if it does not iterate through all elements
// (i.e. until Advance() returns false).  Cancel() is idempotent and can be
// called concurrently with a goroutine that is iterating via Advance() and
// BlobId().  Cancel() causes Advance() to subsequently return false.
// Cancel() does not block.
func (bms *BlobMetadataStream) Cancel() {
	bms.stream.Cancel()
}
