// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the Signpost table of a blobmap.

package blobmap

import "v.io/v23/context"
import wire "v.io/v23/services/syncbase"
import "v.io/v23/vom"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/server/interfaces"
import "v.io/x/ref/services/syncbase/store"

// signpostKey() returns the key used in the store to reference the signpost
// for blobId.
func signpostKey(blobId wire.BlobRef) []byte {
	return []byte(signpostPrefix + string(blobId))
}

// SetSignpost() sets the Signpost associated with a blob to *sp.
func (bm *BlobMap) SetSignpost(ctx *context.T, blobId wire.BlobRef, sp *interfaces.Signpost) (err error) {
	var val []byte
	val, err = vom.Encode(sp)
	if err == nil {
		err = bm.st.Put(signpostKey(blobId), val)
	}
	return err
}

// GetSignpost() yields in *sp the Signpost associated with a blob.
// If there is an error, *sp is set to a canonical empty Signpost.
// On return, it is guaranteed that any maps in *sp are non-nil.
func (bm *BlobMap) GetSignpost(ctx *context.T, blobId wire.BlobRef, sp *interfaces.Signpost) (err error) {
	var valBuf [maxSignpostLen]byte
	var val []byte
	val, err = bm.st.Get(signpostKey(blobId), valBuf[:])
	if err == nil {
		err = vom.Decode(val, sp)
	}
	if err != nil {
		*sp = interfaces.Signpost{}
	}
	if sp.Locations == nil {
		sp.Locations = make(interfaces.PeerToLocationDataMap)
	}
	if sp.SgIds == nil {
		sp.SgIds = make(map[interfaces.GroupId]struct{})
	}
	return err
}

// DeleteSignpost() deletes the Signpost for the specified blob.
func (bm *BlobMap) DeleteSignpost(ctx *context.T, blobId wire.BlobRef) error {
	return bm.st.Delete(signpostKey(blobId))
}

// A SignpostStream allows the client to iterate over the Signposts associated with blob IDs.
//      ss := bm.NewSignpostStream(ctx, blob)
//      for ss.Advance() {
//              blob := ss.BlobId()
//              signpost := ss.Signpost()
//              ...process blob, signpost...
//      }
//      if ss.Err() != nil {
//              ...there was an error...
//      }
type SignpostStream struct {
	bm     *BlobMap
	stream store.Stream

	keyBuf   [maxKeyLen]byte      // buffer for keys
	valBuf   [maxSignpostLen]byte // buffer for values
	blobId   wire.BlobRef         // blobId key for current element
	signpost interfaces.Signpost  // Signpost of current element
	err      error                // error encountered.
	more     bool                 // whether stream may be consulted again
}

// NewSignpostStream() returns, as a localblobstore.SignpostStream interface, a
// pointer to a new blobmap.SignpostStream that allows the client to enumerate
// the Signposts associated with blob IDs, in order.
func (bm *BlobMap) NewSignpostStream(ctx *context.T) localblobstore.SignpostStream {
	// ctx is currently unused.
	ss := new(SignpostStream)
	ss.bm = bm
	ss.stream = bm.st.Scan([]byte(signpostPrefix), signpostStreamKeyLimit)
	ss.more = true
	return ss
}

// Advance() stages an element so the client can retrieve the blob ID hash with
// BlobId(), or its Signpost with Signpost().  Advance() returns true iff there
// is an element to retrieve.  The client must call Advance() before calling
// BlobId() or Signpost() The client must call Cancel if it does not iterate
// through all elements (i.e. until Advance() returns false).  Advance() may
// block if an element is not immediately available.
func (ss *SignpostStream) Advance() (ok bool) {
	if ss.more && ss.err == nil {
		if !ss.stream.Advance() {
			ss.err = ss.stream.Err()
			ss.more = false // no more stream, even if no error
		} else {
			ss.blobId = wire.BlobRef(ss.stream.Key(ss.keyBuf[:])[1:])
			var value []byte = ss.stream.Value(ss.valBuf[:])
			ss.err = vom.Decode(value, &ss.signpost)
			ok = (ss.err == nil)
			if !ok {
				ss.stream.Cancel()
			}
		}
	}
	return ok
}

// BlobId() returns the blob Id of the blob Id and Signpost staged by
// Advance().  BlobId() may panic if Advance() returned false or was not called
// at all.  BlobId() does not block.
func (ss *SignpostStream) BlobId() (result wire.BlobRef) {
	return ss.blobId
}

// Signpost() returns the Signpost associated with the blob ID staged by
// Advance().  Signpost() may panic if Advance() returned false or was not
// called at all.  Signpost() does not block.
func (ss *SignpostStream) Signpost() interfaces.Signpost {
	return ss.signpost
}

// Err() returns a non-nil error iff the stream encountered any errors.  Err()
// does not block.
func (ss *SignpostStream) Err() error {
	return ss.err
}

// Cancel() notifies the stream provider that it can stop producing elements.
// The client must call Cancel() if it does not iterate through all elements
// (i.e. until Advance() returns false).  Cancel() is idempotent and can be
// called concurrently with a goroutine that is iterating via Advance() and
// BlobId().  Cancel() causes Advance() to subsequently return false.
// Cancel() does not block.
func (ss *SignpostStream) Cancel() {
	ss.stream.Cancel()
}
