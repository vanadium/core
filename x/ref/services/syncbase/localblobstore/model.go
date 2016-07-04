// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package localblobstore is the interface to a local blob store.
// Implementations include fs_cablobstore.
//
// Expected use
// ============
// These examples assume that bs, bsS (sender) and bsR (receiver) are blobstores.
//
// Writing blobs
//      bw, err := bs.NewBlobWriter(ctx, "")  // For a new blob, implementation picks blob name.
//      if err == nil {
//		blobName := bw.Name()  // Get name the implementation picked.
//   	  	... use bw.AppendBytes() to append data to the blob...
//   	  	... and/or bw.AppendBlob() to append data that's in another existing blob...
//        	err = bw.Close()
//   	}
//
// Resume writing a blob that was partially written due to a crash (not yet finalized).
//	bw, err := bs.ResumeBlobWriter(ctx, name)
//	if err == nil {
//		size := bw.Size() // The store has this many bytes from the blob.
//		... write the remaining data using bw.AppendBytes() and/or bw.AppendBlob()...
//		err = bw.Close()
//	}
//
// Reading blobs
//	br, err := bs.NewBlobReader(ctx, name)
//	if err == nil {
//		... read bytes with br.ReadAt() or br.Read(), perhas with br.Seek()...
//		err = br.Close()
//	}
//
// Transferring blobs from one store to another:
// See example in localblobstore_transfer_test.go
// Summary:
// - The sender sends the chunksum of the blob from BlobReader's Hash().
// - The receiver checks whether it already has the blob, with the same
//   checksum.
// - If the receiver does not have the blob, the sender sends the list of chunk
//   hashes in the blob using BlobChunkStream().
// - The receiver uses RecipeStreamFromChunkStream() with the chunk hash stream
//   from the sender, and tells the sender the chunk hashes of the chunks it
//   needs.
// - The sender uses LookupChunk() to find the data for each chunk the receiver
//   needs, and sends it to the receiver.
// - The receiver applies the recipe steps, with the actual chunk data from
//   the sender and its own local data.
package localblobstore

import "v.io/v23/context"
import wire "v.io/v23/services/syncbase"
import "v.io/x/ref/services/syncbase/server/interfaces"

// A BlobStore represents a simple, content-addressable store.
type BlobStore interface {
	// NewBlobReader() returns a pointer to a newly allocated BlobReader on
	// the specified blobName.  BlobReaders should not be used concurrently
	// by multiple threads.  Returned handles should be closed with
	// Close().
	NewBlobReader(ctx *context.T, blobName string) (br BlobReader, err error)

	// NewBlobWriter() returns a pointer to a newly allocated BlobWriter on
	// a newly created blob.  If "name" is non-empty, its is used to name
	// the blob, and it must be in the format of a name returned by this
	// interface (probably by another instance on another device).
	// Otherwise, otherwise a new name is created, which can be found using
	// the Name() method.  It is an error to attempt to overwrite a blob
	// that already exists in this blob store.  BlobWriters should not be
	// used concurrently by multiple threads.  The returned handle should
	// be closed with either the Close() or CloseWithoutFinalize() method
	// to avoid leaking file handles.
	NewBlobWriter(ctx *context.T, name string) (bw BlobWriter, err error)

	// ResumeBlobWriter() returns a pointer to a newly allocated BlobWriter on
	// an old, but unfinalized blob name.
	ResumeBlobWriter(ctx *context.T, blobName string) (bw BlobWriter, err error)

	// DeleteBlob() deletes the named blob from the BlobStore.
	DeleteBlob(ctx *context.T, blobName string) (err error)

	// GC() removes old temp files and content-addressed blocks that are no
	// longer referenced by any blob.  It may be called concurrently with
	// other calls to GC(), and with uses of BlobReaders and BlobWriters.
	GC(ctx *context.T) error

	// BlobChunkStream() returns a ChunkStream that can be used to read the
	// ordered list of content hashes of chunks in blob blobName.  It is
	// expected that this list will be presented to
	// RecipeStreamFromChunkStream() on another device, to create a recipe
	// for transmitting the blob efficiently to that other device.
	BlobChunkStream(ctx *context.T, blobName string) ChunkStream

	// RecipeStreamFromChunkStream() returns a pointer to a RecipeStream
	// that allows the client to iterate over each RecipeStep needed to
	// create the blob formed by the chunks in chunkStream.  It is expected
	// that this will be called on a receiving device, and be given a
	// ChunkStream from a sending device, to yield a recipe for efficient
	// chunk transfer.  RecipeStep values with non-nil Chunk fields need
	// the chunk from the sender; once the data is returned it can be
	// written with BlobWriter.AppendBytes().  Those with blob
	// references can be written locally with BlobWriter.AppendBlob().
	RecipeStreamFromChunkStream(ctx *context.T, chunkStream ChunkStream) RecipeStream

	// LookupChunk() returns the location of a chunk with the specified chunk
	// hash within the store.  It is expected that chunk hashes from
	// RecipeStep entries from RecipeStreamFromChunkStream() will be mapped
	// to blob Location values on the sender for transmission to the
	// receiver.
	LookupChunk(ctx *context.T, chunkHash []byte) (loc Location, err error)

	// ListBlobIds() returns an iterator that can be used to enumerate the
	// blobs in a BlobStore.  Expected use is:
	//
	//	iter := bs.ListBlobIds(ctx)
	//	for iter.Advance() {
	//	  // Process iter.Value() here.
	//	}
	//	if iter.Err() != nil {
	//	  // The loop terminated early due to an error.
	//	}
	ListBlobIds(ctx *context.T) (iter Stream)

	// ListCAIds() returns an iterator that can be used to enumerate the
	// content-addressable fragments in a BlobStore.  Expected use is:
	//
	//	iter := bs.ListCAIds(ctx)
	//	for iter.Advance() {
	//	  // Process iter.Value() here.
	//	}
	//	if iter.Err() != nil {
	//	  // The loop terminated early due to an error.
	//	}
	ListCAIds(ctx *context.T) (iter Stream)

	// Root() returns the name of the root directory where the BlobStore is stored.
	Root() string

	// ------------------------------------------------

	// SetBlobMetadata() sets the BlobMetadata associated with a blob to *bmd.
	SetBlobMetadata(ctx *context.T, blobID wire.BlobRef, bmd *BlobMetadata) error

	// GetBlobMetadata() yields in *bmd the BlobMetadata associated with a blob.
	// If there is an error, *bmd is set to a canonical empty BlobMetadata.
	// On return, it is guaranteed that any maps in *bmd are non-nil.
	GetBlobMetadata(ctx *context.T, blobID wire.BlobRef, bmd *BlobMetadata) error

	// DeleteBlobMetadata() deletes the BlobMetadata for the specified blob.
	DeleteBlobMetadata(ctx *context.T, blobID wire.BlobRef) error

	// NewBlobMetadataStream() returns a pointer to a BlobMetadataStream
	// that allows the client to iterate over each blob for which BlobMetadata
	// has been specified.
	NewBlobMetadataStream(ctx *context.T) BlobMetadataStream

	// ------------------------------------------------

	// SetSignpost() sets the Signpost associated with a blob to *sp.
	SetSignpost(ctx *context.T, blobID wire.BlobRef, sp *interfaces.Signpost) error

	// GetSignpost() yields in *sp the Signpost associated with a blob.
	// If there is an error, *sp is set to a canonical empty Signpost.
	// On return, it is guaranteed that any maps in *sp are non-nil.
	GetSignpost(ctx *context.T, blobID wire.BlobRef, sp *interfaces.Signpost) error

	// DeleteSignpost() deletes the Signpost for the specified blob.
	DeleteSignpost(ctx *context.T, blobID wire.BlobRef) error

	// NewSignpostStream() returns a pointer to a SignpostStream
	// that allows the client to iterate over each blob for which a Signpost
	// has been specified.
	NewSignpostStream(ctx *context.T) SignpostStream

	// ------------------------------------------------

	// SetPerSyncgroup() sets the PerSyncgroup associated with a syncgroup to *psg.
	SetPerSyncgroup(ctx *context.T, sgId interfaces.GroupId, psg *PerSyncgroup) error

	// GetPerSyncgroup() yields in *psg the PerSyncgroup associated with a syncgroup.
	// If there is an error, *psg is set to a canonical empty PerSyncgroup.
	// On return, it is guaranteed that any maps in *psg are non-nil.
	GetPerSyncgroup(ctx *context.T, sgId interfaces.GroupId, psg *PerSyncgroup) error

	// DeletePerSyncgroup() deletes the PerSyncgroup for the specified syncgroup.
	DeletePerSyncgroup(ctx *context.T, sgId interfaces.GroupId) error

	// NewPerSyncgroupStream() returns a pointer to a PerSyncgroupStream
	// that allows the client to iterate over each syncgroup for which PerSyncgroup
	// has been specified.
	NewPerSyncgroupStream(ctx *context.T) PerSyncgroupStream

	// ------------------------------------------------

	// Close() closes the BlobStore.
	Close() error
}

// A Location describes chunk's location within a blob.  It is returned by
// BlobStore.LookupChunk().
type Location struct {
	BlobName string // name of blob
	Offset   int64  // byte offset of chunk within blob
	Size     int64  // size of chunk
}

// A BlobReader allows a blob to be read using the standard ReadAt(), Read(),
// and Seek() calls.  A BlobReader can be created with NewBlobReader(), and
// should be closed with the Close() method to avoid leaking file handles.
type BlobReader interface {
	// ReadAt() fills b[] with up to len(b) bytes of data starting at
	// position "at" within the blob that the BlobReader indicates, and
	// returns the number of bytes read.
	ReadAt(b []byte, at int64) (n int, err error)

	// Read() fills b[] with up to len(b) bytes of data starting at the
	// current seek position of the BlobReader within the blob that the
	// BlobReader indicates, and then both returns the number of bytes read
	// and advances the BlobReader's seek position by that amount.
	Read(b []byte) (n int, err error)

	// Seek() sets the seek position of the BlobReader to offset if
	// whence==0, offset+current_seek_position if whence==1, and
	// offset+end_of_blob if whence==2, and then returns the current seek
	// position.
	Seek(offset int64, whence int) (result int64, err error)

	// Close() indicates that the client will perform no further operations
	// on the BlobReader.  It releases any resources held by the
	// BlobReader.
	Close() error

	// Name() returns the BlobReader's name.
	Name() string

	// Size() returns the BlobReader's size.
	Size() int64

	// IsFinalized() returns whether the BlobReader has been finalized.
	IsFinalized() bool

	// Hash() returns the BlobReader's hash.  It may be nil if the blob is
	// not finalized.
	Hash() []byte
}

// A BlockOrFile represents a vector of bytes, and contains either a data
// block (as a []byte), or a (file name, size, offset) triple.
type BlockOrFile struct {
	Block    []byte // If FileName is empty, the bytes represented.
	FileName string // If non-empty, the name of the file containing the bytes.
	Size     int64  // If FileName is non-empty, the number of bytes (or -1 for "all")
	Offset   int64  // If FileName is non-empty, the offset of the relevant bytes within the file.
}

// A BlobWriter allows a blob to be written.  If a blob has not yet been
// finalized, it also allows that blob to be extended.  A BlobWriter may be
// created with NewBlobWriter(), and should be closed with Close() or
// CloseWithoutFinalize().
type BlobWriter interface {
	// AppendBlob() adds a (substring of a) pre-existing blob to the blob
	// being written by the BlobWriter.  The fragments of the pre-existing
	// blob are not physically copied; they are referenced by both blobs.
	AppendBlob(blobName string, size int64, offset int64) (err error)

	// AppendBytes() appends bytes from byte vectors or local files to the
	// blob being written by the BlobWriter.  On return from this call, the
	// bytes are not necessarily guaranteed to be committed to disc or
	// available to a concurrent BlobReader.  They will certainly be
	// committed after a subsequent call to Close() or
	// CloseWithoutFinalize().
	AppendBytes(item ...BlockOrFile) (err error)

	// Close() finalizes the BlobWriter, and indicates that the client will
	// perform no further append operations on the BlobWriter.  Any
	// internal open file handles are closed.
	Close() (err error)

	// CloseWithoutFinalize() indicates that the client will perform no
	// further append operations on the BlobWriter, but does not finalize
	// the blob.  Any internal open file handles are closed.  Clients are
	// expected to need this operation infrequently.
	CloseWithoutFinalize() (err error)

	// Name() returns the BlobWriter's name.
	Name() string

	// Size() returns the BlobWriter's size.
	Size() int64

	// IsFinalized() returns whether the BlobWriter has been finalized.
	IsFinalized() bool

	// Hash() returns the BlobWriter's hash, reflecting the bytes written so far.
	Hash() []byte
}

// A Stream represents an iterator that allows the client to enumerate
// all the blobs or fragments in a BlobStore.
//
// The interfaces Stream, ChunkStream, RecipeStream all have four calls,
// and differ only in the Value() call.
type Stream interface {
	// Advance() stages an item so that it may be retrieved via Value().
	// Returns true iff there is an item to retrieve.  Advance() must be
	// called before Value() is called.  The caller is expected to read
	// until Advance() returns false, or to call Cancel().
	Advance() bool

	// Value() returns the item that was staged by Advance().  May panic if
	// Advance() returned false or was not called.  Never blocks.
	Value() (name string)

	// Err() returns any error encountered by Advance.  Never blocks.
	Err() error

	// Cancel() indicates that the client wishes to cease reading from the stream.
	// It causes the next call to Advance() to return false.  Never blocks.
	// It may be called concurrently with other calls on the stream.
	Cancel()
}

// A ChunkStream represents an iterator that allows the client to enumerate
// the chunks in a blob.   See the comments for Stream for usage.
type ChunkStream interface {
	Advance() bool

	// Value() returns the chunkHash that was staged by Advance().  May
	// panic if Advance() returned false or was not called.  Never blocks.
	// The result may share storage with buf[] if it is large enough;
	// otherwise, a new buffer is allocated.  It is legal to call with
	// buf==nil.
	Value(buf []byte) (chunkHash []byte)

	Err() error
	Cancel()
}

// A RecipeStep describes one piece of a recipe for making a blob.
// The step consists either of appending the chunk with content hash Chunk and size Size,
// or (if Chunk==nil) the Size bytes from Blob, starting at Offset.
type RecipeStep struct {
	Chunk  []byte
	Blob   string
	Size   int64
	Offset int64
}

// A RecipeStream represents an iterator that allows the client to obtain the
// steps needed to construct a blob with a given ChunkStream, attempting to
// reuse data in existing blobs.  See the comments for Stream for usage.
type RecipeStream interface {
	Advance() bool

	// Value() returns the RecipeStep that was staged by Advance().  May panic if
	// Advance() returned false or was not called.  Never blocks.
	Value() RecipeStep

	Err() error
	Cancel()
}

// -----------------------------------------------------

// A BlobMetadataStream represents an iterator that allows the client to obtain
// the BlobMetadata that has been associated with each blob ID.  See the
// comments for Stream for usage.
type BlobMetadataStream interface {
	Advance() bool

	// BlobId() returns the blob ID that was staged by Advance().  May
	// panic if Advance() returned false or was not called.  Never blocks.
	BlobId() wire.BlobRef

	// BlobMetaData() returns the BlobMetedata that was staged by
	// Advance().  May panic if Advance() returned false or was not called.
	// Never blocks.
	BlobMetadata() BlobMetadata

	Err() error
	Cancel()
}

// -----------------------------------------------------

// A SignpostStream represents an iterator that allows the client to obtain the
// Signpost information that has been associated with each blob ID.  See the
// comments for Stream for usage.
type SignpostStream interface {
	Advance() bool

	// BlobId() returns the blob ID that was staged by Advance().  May
	// panic if Advance() returned false or was not called.  Never blocks.
	BlobId() wire.BlobRef

	// BlobMetaData() returns the BlobMetedata that was staged by
	// Advance().  May panic if Advance() returned false or was not called.
	// Never blocks.
	Signpost() interfaces.Signpost

	Err() error
	Cancel()
}

// -----------------------------------------------------

// A PerSyncgroupStream represents an iterator that allows the client to obtain
// the PerSyncgroup that has been associated with each blob.  See the comments
// for Stream for usage.
type PerSyncgroupStream interface {
	Advance() bool

	// SyncgroupId() returns the blob ID that was staged by Advance().  May
	// panic if Advance() returned false or was not called.  Never blocks.
	SyncgroupId() interfaces.GroupId

	// PerSyncgroup() returns the PerSyncgroupt was staged by
	// Advance().  May panic if Advance() returned false or was not called.
	// Never blocks.
	PerSyncgroup() PerSyncgroup

	Err() error
	Cancel()
}
