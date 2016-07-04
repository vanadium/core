// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package blobmap implements a persistent map from blob identifiers to blob
// meta-data and vice versa.  Meta-data includes locations of chunks within
// blobs, used for incremental blob transfer over the network, hints for which
// device might have blobs (called signposts), and metadata about blob
// ownership and usage.  blobmap is implemented using a store.Store (currently,
// one implemented with leveldb).
package blobmap

import "encoding/binary"
import "sync"

import "v.io/v23/context"
import "v.io/v23/verror"
import "v.io/x/ref/services/syncbase/store/util"
import "v.io/x/ref/services/syncbase/store"

const pkgPath = "v.io/syncbase/x/ref/services/syncbase/localblobstore/blobmap"

var (
	errBadBlobIDLen               = verror.Register(pkgPath+".errBadBlobIDLen", verror.NoRetry, "{1:}{2:} blobmap {3}: bad blob length {4} should be {5}{:_}")
	errBadChunkHashLen            = verror.Register(pkgPath+".errBadChunkHashLen", verror.NoRetry, "{1:}{2:} blobmap {3}: bad chunk hash length {4} should be {5}{:_}")
	errNoSuchBlob                 = verror.Register(pkgPath+".errNoSuchBlob", verror.NoRetry, "{1:}{2:} blobmap {3}: no such blob{:_}")
	errMalformedChunkEntry        = verror.Register(pkgPath+".errMalformedChunkEntry", verror.NoRetry, "{1:}{2:} blobmap {3}: malformed chunk entry{:_}")
	errNoSuchChunk                = verror.Register(pkgPath+".errNoSuchChunk", verror.NoRetry, "{1:}{2:} blobmap {3}: no such chunk{:_}")
	errMalformedBlobEntry         = verror.Register(pkgPath+".errMalformedBlobEntry", verror.NoRetry, "{1:}{2:} blobmap {3}: malformed blob entry{:_}")
	errMalformedSignpostEntry     = verror.Register(pkgPath+".errMalformedSignpostEntry", verror.NoRetry, "{1:}{2:} blobmap {3}: malformed Signpost entry{:_}")
	errMalformedBlobMetadataEntry = verror.Register(pkgPath+".errMalformedBlobMetadataEntry", verror.NoRetry, "{1:}{2:} blobmap {3}: malformed BlobMetadata entry{:_}")
	errMalformedPerSyncgroupEntry = verror.Register(pkgPath+".errMalformedPerSyncgroupEntry", verror.NoRetry, "{1:}{2:} key {3}: malformed PerSyncgroup entry{:_}")
)

// There are five tables:
//   0) chunk-to-location
//   1) blob-to-chunk
//   2) blob-to-signpost
//   3) blob-to-metadata
//   4) syncgroup-to-per-syncgroup
//
// Chunks represent sequences of bytes that occur in blobs.  Chunks are the
// unit of network transfer; chunks that the recipient already has are not
// transferred---they are instead copied from a related blob.
// Each chunk is represented by one entry in each of tables (0) and (1).
// On deletion, the latter is used to find the former, so the latter is added
// first, and deleted last.
//
// chunk-to-location:
//    Key:    1-byte containing chunkPrefix, 16-byte chunk hash, 16-byte blob ID
//    Value:  Varint offset, Varint length.
// The chunk with the specified 16-byte hash had the specified length, and is
// (or was) found at the specified offset in the blob.
//
// blob-to-chunk:
//    Key:    1-byte containing blobPrefix, 16-byte blob ID, 8-byte bigendian offset
//    Value:  16-byte chunk hash, Varint length.
//
// blob-to-signpost:  (even if the blob is not stored locally)
//    Key:   1-byte containing signpostPrefix, 16-byte blob ID
//    Value: vom-encoded BlobSignpost
//
// blob-to-metadata:
//    Key:   1-byte containing metadataPrefix, 16-byte blob ID
//    Value: vom-encoded BlobMetaData
//
// syncgroup-to-per-syncgroup
//    Key:   1-byte containing perSyncgroupPrefix, 43-byte syncgroup id (GroupId)
//    Value: vom-encoded PerSyncgroup
//
// The varint encoded fields are written/read with
// encoding/binary.{Put,Read}Varint.  The blob-to-chunk keys encode the offset
// as raw big-endian (encoding/binary.{Put,}Uint64) so that it will sort in
// increasing offset order.

const chunkHashLen = 16 // length of chunk hash
const blobIDLen = 16    // length of blob ID
const offsetLen = 8     // length of offset in blob-to-chunk key

const maxKeyLen = 64            // conservative maximum key length
const maxValLen = 64            // conservative maximum value length
const maxSignpostLen = 1024     // conservative maximum Signpost length
const maxBlobMetadataLen = 1024 // conservative maximum BlobMetadata length
const maxPerSyncgroupLen = 1024 // conservative maximum PerSyncgroup length

var chunkPrefix []byte = []byte{0}        // key prefix for chunk-to-location
var blobPrefix []byte = []byte{1}         // key prefix for blob-to-chunk
var signpostPrefix string = "\u0002"      // key prefix for blob-to-signpost
var metadataPrefix string = "\u0003"      // key prefix for blob-to-metadata
var perSyncgroupPrefix []byte = []byte{4} // key prefix for per-syncgroup

// offsetLimit is an offset that's greater than, and one byte longer than, any
// real offset.
var offsetLimit []byte = []byte{
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff,
}

// blobLimit is a blobID that's greater than, and one byte longer than, any
// real blob ID
var blobLimit []byte = []byte{
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff,
}

// A Location describes chunk's location within a blob.
type Location struct {
	BlobID []byte // ID of blob
	Offset int64  // byte offset of chunk within blob
	Size   int64  // size of chunk
}

// A BlobMap maps chunk checksums to Locations, and vice versa.
type BlobMap struct {
	dir string      // the directory where the store is held
	st  store.Store // private store that holds the mapping.
}

// New() returns a pointer to a BlobMap, backed by storage in directory dir.
func New(ctx *context.T, stEngine, dir string) (bm *BlobMap, err error) {
	bm = new(BlobMap)
	bm.dir = dir
	bm.st, err = util.OpenStore(stEngine, dir, util.OpenOptions{CreateIfMissing: true, ErrorIfExists: false})
	return bm, err
}

// Open() returns a pointer to a BlobMap, backed by storage in directory dir, which must already exist.
func Open(ctx *context.T, stEngine, dir string) (bm *BlobMap, err error) {
	bm = new(BlobMap)
	bm.dir = dir
	bm.st, err = util.OpenStore(stEngine, dir, util.OpenOptions{CreateIfMissing: false, ErrorIfExists: false})
	return bm, err
}

// Close() closes any files or other resources associated with *bm.
// No other methods on bm may be called after Close().
func (bm *BlobMap) Close() error {
	return bm.st.Close()
}

// AssociateChunkWithLocation() remembers that the specified chunk hash is
// associated with the specified Location.
func (bm *BlobMap) AssociateChunkWithLocation(ctx *context.T, chunk []byte, loc Location) (err error) {
	// Check of expected lengths explicitly in routines that modify the database.
	if len(loc.BlobID) != blobIDLen {
		err = verror.New(errBadBlobIDLen, ctx, bm.dir, len(loc.BlobID), blobIDLen)
	} else if len(chunk) != chunkHashLen {
		err = verror.New(errBadChunkHashLen, ctx, bm.dir, len(chunk), chunkHashLen)
	} else {
		var key [maxKeyLen]byte
		var val [maxValLen]byte

		// Put the blob-to-chunk entry first, since it's used
		// to garbage collect the other.
		keyLen := copy(key[:], blobPrefix)
		keyLen += copy(key[keyLen:], loc.BlobID)
		binary.BigEndian.PutUint64(key[keyLen:], uint64(loc.Offset))
		keyLen += offsetLen

		valLen := copy(val[:], chunk)
		valLen += binary.PutVarint(val[valLen:], loc.Size)
		err = bm.st.Put(key[:keyLen], val[:valLen])

		if err == nil {
			keyLen = copy(key[:], chunkPrefix)
			keyLen += copy(key[keyLen:], chunk)
			keyLen += copy(key[keyLen:], loc.BlobID)

			valLen = binary.PutVarint(val[:], loc.Offset)
			valLen += binary.PutVarint(val[valLen:], loc.Size)

			err = bm.st.Put(key[:keyLen], val[:valLen])
		}
	}

	return err
}

// DeleteBlob() deletes any of the chunk associations previously added with
// AssociateChunkWithLocation(..., chunk, ...).
func (bm *BlobMap) DeleteBlob(ctx *context.T, blob []byte) (err error) {
	// Check of expected lengths explicitly in routines that modify the database.
	if len(blob) != blobIDLen {
		err = verror.New(errBadBlobIDLen, ctx, bm.dir, len(blob), blobIDLen)
	} else {
		var start [maxKeyLen]byte
		var limit [maxKeyLen]byte

		startLen := copy(start[:], blobPrefix)
		startLen += copy(start[startLen:], blob)

		limitLen := copy(limit[:], start[:startLen])
		limitLen += copy(limit[limitLen:], offsetLimit)

		var keyBuf [maxKeyLen]byte    // buffer for keys returned by stream
		var valBuf [maxValLen]byte    // buffer for values returned by stream
		var deleteKey [maxKeyLen]byte // buffer to construct chunk-to-location keys to delete

		deletePrefixLen := copy(deleteKey[:], chunkPrefix)

		seenAValue := false

		s := bm.st.Scan(start[:startLen], limit[:limitLen])
		for s.Advance() && err == nil {
			seenAValue = true

			key := s.Key(keyBuf[:])
			value := s.Value(valBuf[:])

			if len(value) >= chunkHashLen {
				deleteKeyLen := deletePrefixLen
				deleteKeyLen += copy(deleteKey[deleteKeyLen:], value[:chunkHashLen])
				deleteKeyLen += copy(deleteKey[deleteKeyLen:], blob)
				err = bm.st.Delete(deleteKey[:deleteKeyLen])
			}

			if err == nil {
				// Delete the blob-to-chunk entry last, as it's
				// used to find the chunk-to-location entry.
				err = bm.st.Delete(key)
			}
		}

		if err != nil {
			s.Cancel()
		} else {
			err = s.Err()
			if err == nil && !seenAValue {
				err = verror.New(errNoSuchBlob, ctx, bm.dir, blob)
			}
		}
	}

	return err
}

// LookupChunk() returns a Location for the specified chunk.  Only one Location
// is returned, even if several are available in the database.  If the client
// finds that the Location is not available, perhaps because its blob has
// been deleted, the client should remove the blob from the BlobMap using
// DeleteBlob(loc.Blob), and try again.  (The client may also wish to
// arrange at some point to call GC() on the blob store.)
func (bm *BlobMap) LookupChunk(ctx *context.T, chunkHash []byte) (loc Location, err error) {
	var start [maxKeyLen]byte
	var limit [maxKeyLen]byte

	startLen := copy(start[:], chunkPrefix)
	startLen += copy(start[startLen:], chunkHash)

	limitLen := copy(limit[:], start[:startLen])
	limitLen += copy(limit[limitLen:], blobLimit)

	var keyBuf [maxKeyLen]byte // buffer for keys returned by stream
	var valBuf [maxValLen]byte // buffer for values returned by stream

	s := bm.st.Scan(start[:startLen], limit[:limitLen])
	if s.Advance() {
		var n int
		key := s.Key(keyBuf[:])
		value := s.Value(valBuf[:])
		loc.BlobID = key[len(chunkPrefix)+chunkHashLen:]
		loc.Offset, n = binary.Varint(value)
		if n > 0 {
			loc.Size, n = binary.Varint(value[n:])
		}
		if n <= 0 {
			err = verror.New(errMalformedChunkEntry, ctx, bm.dir, chunkHash, key, value)
		}
		s.Cancel()
	} else {
		if err == nil {
			err = s.Err()
		}
		if err == nil {
			err = verror.New(errNoSuchChunk, ctx, bm.dir, chunkHash)
		}
	}

	return loc, err
}

// A ChunkStream allows the client to iterate over the chunks in a blob:
//	cs := bm.NewChunkStream(ctx, blob)
//	for cs.Advance() {
//		chunkHash := cs.Value()
//		...process chunkHash...
//	}
//	if cs.Err() != nil {
//		...there was an error...
//	}
type ChunkStream struct {
	bm     *BlobMap
	ctx    *context.T
	stream store.Stream

	keyBuf [maxKeyLen]byte // buffer for keys
	valBuf [maxValLen]byte // buffer for values
	key    []byte          // key for current element
	value  []byte          // value of current element
	loc    Location        // location of current element
	err    error           // error encountered.
	more   bool            // whether stream may be consulted again
}

// NewChunkStream() returns a pointer to a new ChunkStream that allows the client
// to enumerate the chunk hashes in a blob, in order.
func (bm *BlobMap) NewChunkStream(ctx *context.T, blob []byte) *ChunkStream {
	var start [maxKeyLen]byte
	var limit [maxKeyLen]byte

	startLen := copy(start[:], blobPrefix)
	startLen += copy(start[startLen:], blob)

	limitLen := copy(limit[:], start[:startLen])
	limitLen += copy(limit[limitLen:], offsetLimit)

	cs := new(ChunkStream)
	cs.bm = bm
	cs.ctx = ctx
	cs.stream = bm.st.Scan(start[:startLen], limit[:limitLen])
	cs.more = true

	return cs
}

// Advance() stages an element so the client can retrieve the chunk hash with
// Value(), or its Location with Location().  Advance() returns true iff there
// is an element to retrieve.  The client must call Advance() before calling
// Value() or Location() The client must call Cancel if it does not iterate
// through all elements (i.e. until Advance() returns false).  Advance() may
// block if an element is not immediately available.
func (cs *ChunkStream) Advance() (ok bool) {
	if cs.more && cs.err == nil {
		if !cs.stream.Advance() {
			cs.err = cs.stream.Err()
			cs.more = false // no more stream, even if no error
		} else {
			cs.key = cs.stream.Key(cs.keyBuf[:])
			cs.value = cs.stream.Value(cs.valBuf[:])
			ok = (len(cs.value) >= chunkHashLen) &&
				(len(cs.key) == len(blobPrefix)+blobIDLen+offsetLen)
			if ok {
				var n int
				cs.loc.BlobID = make([]byte, blobIDLen)
				copy(cs.loc.BlobID, cs.key[len(blobPrefix):len(blobPrefix)+blobIDLen])
				cs.loc.Offset = int64(binary.BigEndian.Uint64(cs.key[len(blobPrefix)+blobIDLen:]))
				cs.loc.Size, n = binary.Varint(cs.value[chunkHashLen:])
				ok = (n > 0)
			}
			if !ok {
				cs.err = verror.New(errMalformedBlobEntry, cs.ctx, cs.bm.dir, cs.key, cs.value)
				cs.stream.Cancel()
			}
		}
	}
	return ok
}

// Value() returns the content hash of the chunk staged by
// Advance().  The returned slice may be a sub-slice of buf if buf is large
// enough to hold the entire value.  Otherwise, a newly allocated slice will be
// returned.  It is valid to pass a nil buf.  Value() may panic if Advance()
// returned false or was not called at all.  Value() does not block.
func (cs *ChunkStream) Value(buf []byte) (result []byte) {
	if len(buf) < chunkHashLen {
		buf = make([]byte, chunkHashLen)
	}
	copy(buf, cs.value[:chunkHashLen])
	return buf[:chunkHashLen]
}

// Location() returns the Location associated with the chunk staged by
// Advance().  Location() may panic if Advance() returned false or was not
// called at all.  Location() does not block.
func (cs *ChunkStream) Location() Location {
	return cs.loc
}

// Err() returns a non-nil error iff the stream encountered any errors.  Err()
// does not block.
func (cs *ChunkStream) Err() error {
	return cs.err
}

// Cancel() notifies the stream provider that it can stop producing elements.
// The client must call Cancel() if it does not iterate through all elements
// (i.e. until Advance() returns false).  Cancel() is idempotent and can be
// called concurrently with a goroutine that is iterating via Advance() and
// Value().  Cancel() causes Advance() to subsequently return false.
// Cancel() does not block.
func (cs *ChunkStream) Cancel() {
	cs.stream.Cancel()
}

// A BlobStream allows the client to iterate over the blobs in BlobMap:
//	bs := bm.NewBlobStream(ctx)
//	for bs.Advance() {
//		blobID := bs.Value()
//		...process blobID...
//	}
//	if bs.Err() != nil {
//		...there was an error...
//	}
type BlobStream struct {
	bm  *BlobMap
	ctx *context.T

	key    []byte          // key for current element
	keyBuf [maxKeyLen]byte // buffer for keys
	err    error           // error encountered.
	mu     sync.Mutex      // protects "more", which may be written in Cancel()
	more   bool            // whether stream may be consulted again
}

// blobStreamKeyLimit is the key for limit in store.Scan() calls within a BlobStream.
var blobStreamKeyLimit []byte

// signpostStreamKeyLimit is the key for limit in store.Scan() calls within a
// SignpostStream.
var signpostStreamKeyLimit []byte

// metadataStreamKeyLimit is the key for limit in store.Scan() calls within a
// MetadataStream.
var metadataStreamKeyLimit []byte

// perSyncgroupStreamKeyLimit is the key for limit in store.Scan() calls within a
// PerSyncgroupStream.
var perSyncgroupStreamKeyLimit []byte

func init() {
	// The blobStreamKeyLimit key is the maximum length key, all ones after the blobPrefix.
	blobStreamKeyLimit = make([]byte, maxKeyLen)
	for i := copy(blobStreamKeyLimit, blobPrefix); i != len(blobStreamKeyLimit); i++ {
		blobStreamKeyLimit[i] = 0xff
	}

	// The signpostStreamKeyLimit key is the maximum length key, all ones after the signpostPrefix.
	signpostStreamKeyLimit = make([]byte, maxKeyLen)
	for i := copy(signpostStreamKeyLimit, signpostPrefix); i != len(signpostStreamKeyLimit); i++ {
		signpostStreamKeyLimit[i] = 0xff
	}

	// The metadataStreamKeyLimit key is the maximum length key, all ones after the metadataPrefix.
	metadataStreamKeyLimit = make([]byte, maxKeyLen)
	for i := copy(metadataStreamKeyLimit, metadataPrefix); i != len(metadataStreamKeyLimit); i++ {
		metadataStreamKeyLimit[i] = 0xff
	}

	// The perSyncgroupStreamKeyLimit key is the maximum length key, all
	// ones after the perSyncgroupPrefix.
	perSyncgroupStreamKeyLimit = make([]byte, maxKeyLen)
	for i := copy(perSyncgroupStreamKeyLimit, perSyncgroupPrefix); i != len(perSyncgroupStreamKeyLimit); i++ {
		perSyncgroupStreamKeyLimit[i] = 0xff
	}
}

// NewBlobStream() returns a pointer to a new BlobStream that allows the client
// to enumerate the blobs BlobMap, in lexicographic order.
func (bm *BlobMap) NewBlobStream(ctx *context.T) *BlobStream {
	bs := new(BlobStream)
	bs.bm = bm
	bs.ctx = ctx
	bs.more = true
	return bs
}

// Advance() stages an element so the client can retrieve the next blob ID with
// Value().  Advance() returns true iff there is an element to retrieve.  The
// client must call Advance() before calling Value().  The client must call
// Cancel if it does not iterate through all elements (i.e. until Advance()
// returns false).  Advance() may block if an element is not immediately
// available.
func (bs *BlobStream) Advance() (ok bool) {
	bs.mu.Lock()
	ok = bs.more
	bs.mu.Unlock()
	if ok {
		prefixAndKeyLen := len(blobPrefix) + blobIDLen
		// Compute the next key to search for.
		if len(bs.key) == 0 { // First time through: anything starting with blobPrefix.
			n := copy(bs.keyBuf[:], blobPrefix)
			bs.key = bs.keyBuf[:n]
		} else {
			// Increment the blobID to form the next possible key.
			i := prefixAndKeyLen - 1
			for ; i != len(blobPrefix)-1 && bs.keyBuf[i] == 0xff; i-- {
				bs.keyBuf[i] = 0
			}
			if i == len(blobPrefix)-1 { // End of database
				ok = false
			} else {
				bs.keyBuf[i]++
			}
			bs.key = bs.keyBuf[:prefixAndKeyLen]
		}
		if ok {
			stream := bs.bm.st.Scan(bs.key, blobStreamKeyLimit)
			if !stream.Advance() {
				bs.err = stream.Err()
				ok = false // no more stream, even if no error
			} else {
				bs.key = stream.Key(bs.keyBuf[:])
				if len(bs.key) < prefixAndKeyLen {
					bs.err = verror.New(errMalformedBlobEntry, bs.ctx, bs.bm.dir, bs.key, stream.Value(nil))
					ok = false
				}
				stream.Cancel() // We get at most one element from each stream.
			}
		}
		if !ok {
			bs.mu.Lock()
			bs.more = false
			bs.mu.Unlock()
		}
	}
	return ok
}

// Value() returns the blob ID staged by Advance().  The returned slice may be
// a sub-slice of buf if buf is large enough to hold the entire value.
// Otherwise, a newly allocated slice will be returned.  It is valid to pass a
// nil buf.  Value() may panic if Advance() returned false or was not called at
// all.  Value() does not block.
func (bs *BlobStream) Value(buf []byte) (result []byte) {
	if len(buf) < blobIDLen {
		buf = make([]byte, blobIDLen)
	}
	copy(buf, bs.key[len(blobPrefix):len(blobPrefix)+blobIDLen])
	return buf[:blobIDLen]
}

// Err() returns a non-nil error iff the stream encountered any errors.  Err()
// does not block.
func (bs *BlobStream) Err() error {
	return bs.err
}

// Cancel() notifies the stream provider that it can stop producing elements.
// The client must call Cancel() if it does not iterate through all elements
// (i.e. until Advance() returns false).  Cancel() is idempotent and can be
// called concurrently with a goroutine that is iterating via Advance() and
// Value().  Cancel() causes Advance() to subsequently return false.
// Cancel() does not block.
func (bs *BlobStream) Cancel() {
	bs.mu.Lock()
	bs.more = false
	bs.mu.Unlock()
}
