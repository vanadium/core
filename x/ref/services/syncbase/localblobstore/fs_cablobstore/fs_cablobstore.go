// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fs_cablobstore implements a content addressable blob store
// on top of a file system.  It assumes that either os.Link() or
// os.Rename() is available.
package fs_cablobstore

// Internals:
// Blobs are partitioned into two types of unit: "fragments" and "chunks".
// A fragment is stored in a single file on disc.  A chunk is a unit of network
// transmission.
//
//   The blobstore consists of a directory with "blob", "cas", "chunk", and
//   "tmp" subdirectories.
//   - "tmp" is used for temporary files that are moved into place via
//     link()/unlink() or rename(), depending on what's available.
//   - "cas" contains files whose names are content hashes of the files being
//     named.  A few slashes are thrown into the name near the front so that no
//     single directory gets too large.  These files are called "fragments".
//   - "blob" contains files whose names are random numbers.  These names are
//     visible externally as "blob names".  Again, a few slashes are thrown
//     into the name near the front so that no single directory gets too large.
//     Each of these files contains a series of lines of the form:
//        d <size> <offset> <cas-fragment>
//     followed optionally by a line of the form:
//        f <md5-hash>
//     Each "d" line indicates that the next <size> bytes of the blob appear at
//     <offset> bytes into <cas-fragment>, which is in the "cas" subtree.  The
//     "f" line indicates that the blob is "finalized" and gives its complete
//     md5 hash.  No fragments may be appended to a finalized blob.
//   - "chunk" contains a store (currently implemented with leveldb) that
//     maps chunks of blobs to content hashes and vice versa.

import "bufio"
import "bytes"
import "crypto/md5"
import "fmt"
import "hash"
import "io"
import "io/ioutil"
import "math"
import "math/rand"
import "os"
import "path/filepath"
import "strconv"
import "strings"
import "sync"
import "time"

import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/localblobstore/chunker"
import "v.io/x/ref/services/syncbase/localblobstore/blobmap"
import "v.io/v23/context"
import "v.io/v23/verror"

const pkgPath = "v.io/x/ref/services/syncbase/localblobstore/fs_cablobstore"

var (
	errNotADir                = verror.Register(pkgPath+".errNotADir", verror.NoRetry, "{1:}{2:} Not a directory{:_}")
	errAppendFailed           = verror.Register(pkgPath+".errAppendFailed", verror.NoRetry, "{1:}{2:} fs_cablobstore.Append failed{:_}")
	errMalformedField         = verror.Register(pkgPath+".errMalformedField", verror.NoRetry, "{1:}{2:} Malformed field in blob specification{:_}")
	errAlreadyClosed          = verror.Register(pkgPath+".errAlreadyClosed", verror.NoRetry, "{1:}{2:} BlobWriter is already closed{:_}")
	errBlobAlreadyFinalized   = verror.Register(pkgPath+".errBlobAlreadyFinalized", verror.NoRetry, "{1:}{2:} Blob is already finalized{:_}")
	errIllegalPositionForRead = verror.Register(pkgPath+".errIllegalPositionForRead", verror.NoRetry, "{1:}{2:} BlobReader: illegal position {3} on Blob of size {4}{:_}")
	errBadSeekWhence          = verror.Register(pkgPath+".errBadSeekWhence", verror.NoRetry, "{1:}{2:} BlobReader: Bad value for 'whence' in Seek{:_}")
	errNegativeSeekPosition   = verror.Register(pkgPath+".errNegativeSeekPosition", verror.NoRetry, "{1:}{2:} BlobReader: negative position for Seek: offset {3}, whence {4}{:_}")
	errBadSizeOrOffset        = verror.Register(pkgPath+".errBadSizeOrOffset", verror.NoRetry, "{1:}{2:} Bad size ({3}) or offset ({4}) in blob {5} (size {6}){:_}")
	errMalformedBlobHash      = verror.Register(pkgPath+".errMalformedBlobHash", verror.NoRetry, "{1:}{2:} Blob {3} hash malformed hash{:_}")
	errInvalidBlobName        = verror.Register(pkgPath+".errInvalidBlobName", verror.NoRetry, "{1:}{2:} Invalid blob name {3}{:_}")
	errCantDeleteBlob         = verror.Register(pkgPath+".errCantDeleteBlob", verror.NoRetry, "{1:}{2:} Can't delete blob {3}{:_}")
	errBlobDeleted            = verror.Register(pkgPath+".errBlobDeleted", verror.NoRetry, "{1:}{2:} Blob is deleted{:_}")
	errSizeTooBigForFragment  = verror.Register(pkgPath+".errSizeTooBigForFragment", verror.NoRetry, "{1:}{2:} writing blob {1}, size too big for fragment{:1}")
	errStreamCancelled        = verror.Register(pkgPath+".errStreamCancelled", verror.NoRetry, "{1:}{2:} Advance() called on cancelled stream{:_}")
)

// For the moment, we disallow others from accessing the tree where blobs are
// stored.  We could in the future relax this to 0711/0755, and 0644.
const dirPermissions = 0700
const filePermissions = 0600

// Subdirectories of the blobstore's tree
const (
	blobDir  = "blob"  // Subdirectory where blobs are indexed by blob id.
	casDir   = "cas"   // Subdirectory where fragments are indexed by content hash.
	chunkDir = "chunk" // Subdirectory where chunks are indexed by content hash.
	tmpDir   = "tmp"   // Subdirectory where temporary files are created.
)

// An FsCaBlobStore represents a simple, content-addressable store.
type FsCaBlobStore struct {
	rootName string           // The name of the root of the store.
	bm       *blobmap.BlobMap // Mapping from chunks to blob locations and vice versa.

	// mu protects fields below, plus most fields in each blobDesc when used from a BlobWriter.
	mu         sync.Mutex
	activeDesc []*blobDesc        // The blob descriptors in use by active BlobReaders and BlobWriters.
	toDelete   []*map[string]bool // Sets of items that active GC threads are about to delete. (Pointers to maps, to allow pointer comparison.)
}

// hashToFileName() returns the name of the binary ID with the specified
// prefix.  Requires len(id)==16.  An md5 hash is suitable.
func hashToFileName(prefix string, hash []byte) string {
	return filepath.Join(prefix,
		fmt.Sprintf("%02x", hash[0]),
		fmt.Sprintf("%02x", hash[1]),
		fmt.Sprintf("%02x", hash[2]),
		fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
			hash[3],
			hash[4], hash[5], hash[6], hash[7],
			hash[8], hash[9], hash[10], hash[11],
			hash[12], hash[13], hash[14], hash[15]))
}

// fileNameToHash() converts a file name in the format generated by
// hashToFileName(prefix, ...) to a vector of 16 bytes.  If the string is
// malformed, the nil slice is returned.
func fileNameToHash(prefix string, s string) []byte {
	idStr := strings.TrimPrefix(filepath.ToSlash(s), prefix+"/")
	hash := make([]byte, 16, 16)
	n, err := fmt.Sscanf(idStr, "%02x/%02x/%02x/%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
		&hash[0], &hash[1], &hash[2], &hash[3],
		&hash[4], &hash[5], &hash[6], &hash[7],
		&hash[8], &hash[9], &hash[10], &hash[11],
		&hash[12], &hash[13], &hash[14], &hash[15])
	if n != 16 || err != nil {
		hash = nil
	}
	return hash
}

// newBlobName() returns a new random name for a blob.
func newBlobName() string {
	return filepath.Join(blobDir,
		fmt.Sprintf("%02x", rand.Int31n(256)),
		fmt.Sprintf("%02x", rand.Int31n(256)),
		fmt.Sprintf("%02x", rand.Int31n(256)),
		fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
			rand.Int31n(256),
			rand.Int31n(256), rand.Int31n(256), rand.Int31n(256), rand.Int31n(256),
			rand.Int31n(256), rand.Int31n(256), rand.Int31n(256), rand.Int31n(256),
			rand.Int31n(256), rand.Int31n(256), rand.Int31n(256), rand.Int31n(256)))
}

// hashToString() returns a string representation of the hash.
// Requires len(hash)==16.  An md5 hash is suitable.
func hashToString(hash []byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
		hash[0], hash[1], hash[2], hash[3],
		hash[4], hash[5], hash[6], hash[7],
		hash[8], hash[9], hash[10], hash[11],
		hash[12], hash[13], hash[14], hash[15])
}

// stringToHash() converts a string in the format generated by hashToString()
// to a vector of 16 bytes.  If the string is malformed, the nil slice is
// returned.
func stringToHash(s string) []byte {
	hash := make([]byte, 16, 16)
	n, err := fmt.Sscanf(s, "%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
		&hash[0], &hash[1], &hash[2], &hash[3],
		&hash[4], &hash[5], &hash[6], &hash[7],
		&hash[8], &hash[9], &hash[10], &hash[11],
		&hash[12], &hash[13], &hash[14], &hash[15])
	if n != 16 || err != nil {
		hash = nil
	}
	return hash
}

// Create() returns a pointer to an FsCaBlobStore stored in the file system at
// "rootName".  If the directory rootName does not exist, it is created.
func Create(ctx *context.T, stEngine, rootName string) (fscabs *FsCaBlobStore, err error) {
	dir := []string{tmpDir, casDir, chunkDir, blobDir}
	for i := 0; i != len(dir) && err == nil; i++ {
		fullName := filepath.Join(rootName, dir[i])
		os.MkdirAll(fullName, dirPermissions)
		var fi os.FileInfo
		fi, err = os.Stat(fullName)
		if err == nil && !fi.IsDir() {
			err = verror.New(errNotADir, ctx, fullName)
		}
	}
	var bm *blobmap.BlobMap
	if err == nil {
		bm, err = blobmap.New(ctx, stEngine, filepath.Join(rootName, chunkDir))
	}
	if err == nil {
		fscabs = new(FsCaBlobStore)
		fscabs.rootName = rootName
		fscabs.bm = bm
	}
	return fscabs, err
}

// Close() closes the FsCaBlobStore. {
func (fscabs *FsCaBlobStore) Close() error {
	return fscabs.bm.Close()
}

// Root() returns the name of the root directory where *fscabs is stored.
func (fscabs *FsCaBlobStore) Root() string {
	return fscabs.rootName
}

// DeleteBlob() deletes the named blob from *fscabs.
func (fscabs *FsCaBlobStore) DeleteBlob(ctx *context.T, blobName string) (err error) {
	// Disallow deletions of things outside the blob tree, or that may contain "..".
	// For simplicity, the code currently disallows '.'.
	blobID := fileNameToHash(blobDir, blobName)
	if blobID == nil || strings.IndexByte(blobName, '.') != -1 {
		err = verror.New(errInvalidBlobName, ctx, blobName)
	} else {
		err = os.Remove(filepath.Join(fscabs.rootName, blobName))
		if err != nil {
			err = verror.New(errCantDeleteBlob, ctx, blobName, err)
		} else {
			err = fscabs.bm.DeleteBlob(ctx, blobID)
		}
	}
	return err
}

// -----------------------------------------------------------

// A file encapsulates both an os.File and a bufio.Writer on that file.
type file struct {
	fh     *os.File
	writer *bufio.Writer
}

// newFile() returns a *file containing fh and a bufio.Writer on that file, if
// err is nil.
func newFile(fh *os.File, err error) (*file, error) {
	var f *file
	if err == nil {
		f = new(file)
		f.fh = fh
		f.writer = bufio.NewWriter(f.fh)
	}
	return f, err
}

// newTempFile() returns a *file on a new temporary file created in the
// directory dir.
func newTempFile(ctx *context.T, dir string) (*file, error) {
	return newFile(ioutil.TempFile(dir, "newfile"))
}

// close() flushes buffers (if err==nil initially) and closes the file,
// returning its name.
func (f *file) close(ctx *context.T, err error) (string, error) {
	name := f.fh.Name()
	// Flush the data out to disc and close the file.
	if err == nil {
		err = f.writer.Flush()
	}
	if err == nil {
		err = f.fh.Sync()
	}
	err2 := f.fh.Close()
	if err == nil {
		err = err2
	}
	return name, err
}

// closeAndRename() calls f.close(), and if err==nil initially and no new
// errors are seen, renames the file to newName.
func (f *file) closeAndRename(ctx *context.T, newName string, err error) error {
	var oldName string
	oldName, err = f.close(ctx, err)
	if err == nil { // if temp file written successfully...
		// Link or rename the file into place, hoping at least one is
		// supported on this file system.
		os.MkdirAll(filepath.Dir(newName), dirPermissions)
		err = os.Link(oldName, newName)
		if err == nil {
			os.Remove(oldName)
		} else {
			err = os.Rename(oldName, newName)
		}
	}
	if err != nil {
		os.Remove(oldName)
	}
	return err
}

// -----------------------------------------------------------

// A blobFragment represents a vector of bytes and its position within a blob.
type blobFragment struct {
	pos      int64  // position of this fragment within its containing blob.
	size     int64  // size of this fragment.
	offset   int64  // offset within fileName.
	fileName string // name of file describing this fragment.
}

// A blobDesc is the in-memory representation of a blob.
type blobDesc struct {
	activeDescIndex int // Index into fscabs.activeDesc if refCount>0; under fscabs.mu.
	refCount        int // Reference count; under fscabs.mu.

	name string // Name of the blob.

	// The following fields are modified under fscabs.mu and in BlobWriter
	// owner's thread; they may be read by GC (when obtained from
	// fscabs.activeDesc) and the chunk writer under fscabs.mu.  In the
	// BlobWriter owner's thread, reading does not require a lock, but
	// writing does.  In other contexts (BlobReader, or a desc that has
	// just been allocated by getBlob()), no locking is needed.

	fragment  []blobFragment // All the fragments in this blob
	size      int64          // Total size of the blob.
	finalized bool           // Whether the blob has been finalized.
	// A finalized blob has a valid hash field, and no new bytes may be added
	// to it.  A well-formed hash has 16 bytes.
	hash []byte

	openWriter bool       // Whether this descriptor is being written by an open BlobWriter.
	cv         *sync.Cond // signalled when a BlobWriter writes or closes.
}

// isBeingDeleted() returns whether fragment fragName is about to be deleted
// by the garbage collector.   Requires fscabs.mu held.
func (fscabs *FsCaBlobStore) isBeingDeleted(fragName string) (beingDeleted bool) {
	for i := 0; i != len(fscabs.toDelete) && !beingDeleted; i++ {
		_, beingDeleted = (*(fscabs.toDelete[i]))[fragName]
	}
	return beingDeleted
}

// descRef() increments the reference count of *desc and returns whether
// successful.  It may fail if the fragments referenced by the descriptor are
// being deleted by the garbage collector.
func (fscabs *FsCaBlobStore) descRef(desc *blobDesc) bool {
	beingDeleted := false
	fscabs.mu.Lock()
	if desc.refCount == 0 {
		// On the first reference, check whether the fragments are
		// being deleted, and if not, add *desc to the
		// fscabs.activeDesc vector.
		for i := 0; i != len(desc.fragment) && !beingDeleted; i++ {
			beingDeleted = fscabs.isBeingDeleted(desc.fragment[i].fileName)
		}
		if !beingDeleted {
			desc.activeDescIndex = len(fscabs.activeDesc)
			fscabs.activeDesc = append(fscabs.activeDesc, desc)
		}
	}
	if !beingDeleted {
		desc.refCount++
	}
	fscabs.mu.Unlock()
	return !beingDeleted
}

// descUnref() decrements the reference count of *desc if desc!=nil; if that
// removes the last reference, *desc is removed from the fscabs.activeDesc
// vector.
func (fscabs *FsCaBlobStore) descUnref(desc *blobDesc) {
	if desc != nil {
		fscabs.mu.Lock()
		desc.refCount--
		if desc.refCount < 0 {
			panic("negative reference count")
		} else if desc.refCount == 0 {
			// Remove desc from fscabs.activeDesc by moving the
			// last entry in fscabs.activeDesc to desc's slot.
			n := len(fscabs.activeDesc)
			lastDesc := fscabs.activeDesc[n-1]
			lastDesc.activeDescIndex = desc.activeDescIndex
			fscabs.activeDesc[desc.activeDescIndex] = lastDesc
			fscabs.activeDesc = fscabs.activeDesc[0 : n-1]
			desc.activeDescIndex = -1
		}
		fscabs.mu.Unlock()
	}
}

// getBlob() returns the in-memory blob descriptor for the named blob.
func (fscabs *FsCaBlobStore) getBlob(ctx *context.T, blobName string) (desc *blobDesc, err error) {
	slashBlobName := filepath.ToSlash(blobName)
	if !strings.HasPrefix(slashBlobName, blobDir+"/") || strings.IndexByte(blobName, '.') != -1 {
		err = verror.New(errInvalidBlobName, ctx, blobName)
	} else {
		absBlobName := filepath.Join(fscabs.rootName, blobName)
		var fh *os.File
		fh, err = os.Open(absBlobName)
		if err == nil {
			var line string
			desc = new(blobDesc)
			desc.activeDescIndex = -1
			desc.name = blobName
			desc.cv = sync.NewCond(&fscabs.mu)
			scanner := bufio.NewScanner(fh)
			for scanner.Scan() {
				field := strings.Split(scanner.Text(), " ")
				if len(field) == 4 && field[0] == "d" {
					var fragSize int64
					var fragOffset int64
					fragSize, err = strconv.ParseInt(field[1], 0, 64)
					if err == nil {
						fragOffset, err = strconv.ParseInt(field[2], 0, 64)
					}
					if err == nil {
						// No locking needed here because desc
						// is newly allocated and not yet passed to descRef().
						desc.fragment = append(desc.fragment,
							blobFragment{
								fileName: field[3],
								pos:      desc.size,
								size:     fragSize,
								offset:   fragOffset})
					}
					desc.size += fragSize
				} else if len(field) == 2 && field[0] == "f" {
					desc.hash = stringToHash(field[1])
					desc.finalized = true
					if desc.hash == nil {
						err = verror.New(errMalformedBlobHash, ctx, blobName, field[1])
					}
				} else if len(field) > 0 && len(field[0]) == 1 && "a" <= field[0] && field[0] <= "z" {
					// unrecognized line, reserved for extensions: ignore.
				} else {
					err = verror.New(errMalformedField, ctx, line)
				}
			}
			err = scanner.Err()
			fh.Close()
		}
	}
	// Ensure that we return either a properly referenced desc, or nil.
	if err != nil {
		desc = nil
	} else if !fscabs.descRef(desc) {
		err = verror.New(errBlobDeleted, ctx, blobName)
		desc = nil
	}
	return desc, err
}

// -----------------------------------------------------------

// A BlobWriter allows a blob to be written.  If a blob has not yet been
// finalized, it also allows that blob to be extended.  A BlobWriter may be
// created with NewBlobWriter(), and should be closed with Close() or
// CloseWithoutFinalize().
type BlobWriter struct {
	// The BlobWriter exists within a particular FsCaBlobStore and context.T
	fscabs *FsCaBlobStore
	ctx    *context.T

	desc   *blobDesc // Description of the blob being written.
	f      *file     // The file being written.
	hasher hash.Hash // Running hash of blob.

	// The following three fields represent the state of
	// a temporary file that, when complete, will become a fragment.
	// These fields are manipulated by commitBytes() and writeToTempFragment().
	fragFile *file     // The current (temporary) fragment file being appended to, or nil if none.
	fragSize int64     // Bytes written to fragment.
	fragHash hash.Hash // Running hash of fragment.

	// Fields to allow the BlobMap to be written.
	csBr  *BlobReader     // Reader over the blob that's currently being written.
	cs    *chunker.Stream // Stream of chunks derived from csBr
	csErr chan error      // writeBlobMap() sends its result here; Close/CloseWithoutFinalize receives it.
}

// NewBlobWriter() returns a pointer to a newly allocated BlobWriter on
// a newly created blob.  If "name" is non-empty, it is used to name
// the blob, and it must be in the format of a name returned by this
// interface (probably by another instance on another device).
// Otherwise, a new name is created, which can be found using
// the Name() method.  It is an error to attempt to overwrite a blob
// that already exists in this blob store.  BlobWriters should not be
// used concurrently by multiple threads.  The returned handle should
// be closed with either the Close() or CloseWithoutFinalize() method
// to avoid leaking file handles.
func (fscabs *FsCaBlobStore) NewBlobWriter(ctx *context.T, name string) (localblobstore.BlobWriter, error) {
	var bw *BlobWriter
	if name == "" {
		name = newBlobName()
	}
	fileName := filepath.Join(fscabs.rootName, name)
	os.MkdirAll(filepath.Dir(fileName), dirPermissions)
	f, err := newFile(os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, filePermissions))
	if err == nil {
		bw = new(BlobWriter)
		bw.fscabs = fscabs
		bw.ctx = ctx
		bw.desc = new(blobDesc)
		bw.desc.activeDescIndex = -1
		bw.desc.name = name
		bw.desc.cv = sync.NewCond(&fscabs.mu)
		bw.desc.openWriter = true
		bw.f = f
		bw.hasher = md5.New()
		if !fscabs.descRef(bw.desc) {
			// Can't happen; descriptor refers to no fragments.
			panic(verror.New(errBlobDeleted, ctx, bw.desc.name))
		}
		// Write the chunks of this blob into the BlobMap, as they are
		// written by this writer.
		bw.forkWriteBlobMap()
	}
	return bw, err
}

// ResumeBlobWriter() returns a pointer to a newly allocated BlobWriter on an
// old, but unfinalized blob name.
func (fscabs *FsCaBlobStore) ResumeBlobWriter(ctx *context.T, blobName string) (localblobstore.BlobWriter, error) {
	var err error
	var bw *BlobWriter
	var desc *blobDesc
	desc, err = fscabs.getBlob(ctx, blobName)
	if err == nil && desc.finalized {
		err = verror.New(errBlobAlreadyFinalized, ctx, blobName)
	} else if err == nil {
		bw = new(BlobWriter)
		bw.fscabs = fscabs
		bw.ctx = ctx
		bw.desc = desc
		bw.desc.openWriter = true
		fileName := filepath.Join(fscabs.rootName, bw.desc.name)
		bw.f, err = newFile(os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND, 0666))
		bw.hasher = md5.New()
		// Add the existing fragments to the running hash.
		// The descRef's ref count is incremented here to compensate
		// for the decrement it will receive in br.Close(), below.
		if !fscabs.descRef(bw.desc) {
			// Can't happen; descriptor's ref count was already
			// non-zero.
			panic(verror.New(errBlobDeleted, ctx, fileName))
		}
		br := fscabs.blobReaderFromDesc(ctx, bw.desc, dontWaitForWriter)
		buf := make([]byte, 8192, 8192)
		for err == nil {
			var n int
			n, err = br.Read(buf)
			bw.hasher.Write(buf[0:n])
		}
		br.Close()
		if err == io.EOF { // EOF is expected.
			err = nil
		}
		if err == nil {
			// Write the chunks of this blob into the BlobMap, as
			// they are written by this writer.
			bw.forkWriteBlobMap()
		}
	}
	return bw, err
}

// commitBytes() commits bytes added by AppendBytes(), if any.
// If any bytes are committed, they are added to *bw's fragment list.
func (bw *BlobWriter) commitBytes() (err error) {
	if bw.fragFile != nil {
		hash := bw.fragHash.Sum(nil)
		relFileName := hashToFileName(casDir, hash)
		absFileName := filepath.Join(bw.fscabs.rootName, relFileName)

		// Add the fragment's name to bw.desc's fragments so the garbage
		// collector will not delete it when it is renamed.  The temporary
		// file will not be deleted before renaming because the garbage
		// collector does not delete temporary files that have been
		// written in the last several hours.
		bw.fscabs.mu.Lock()
		bw.desc.fragment = append(bw.desc.fragment, blobFragment{
			pos:      bw.desc.size,
			size:     bw.fragSize,
			offset:   0,
			fileName: relFileName})
		bw.fscabs.mu.Unlock()

		if _, statErr := os.Stat(absFileName); statErr != nil && os.IsNotExist(statErr) {
			// Fragment file does not yet exist; rename the temporary file into place.
			err = bw.fragFile.closeAndRename(bw.ctx, absFileName, err)
		} else {
			// Fragment is already present; delete temporary file.
			var oldName string
			oldName, err = bw.fragFile.close(bw.ctx, err)
			os.Remove(oldName)
		}
		if err == nil {
			_, err = fmt.Fprintf(bw.f.writer, "d %d %d %s\n", bw.fragSize, 0 /*offset*/, relFileName)
		}
		if err == nil {
			err = bw.f.writer.Flush()
		}
		if err != nil {
			err = verror.New(errAppendFailed, bw.ctx, bw.fscabs.rootName, err)
			// Remove the entry added to fragment list above.
			bw.fscabs.mu.Lock()
			bw.desc.fragment = bw.desc.fragment[0 : len(bw.desc.fragment)-1]
			bw.fscabs.mu.Unlock()
		} else { // commit the change by updating the size; it's then visible to readers.
			bw.fscabs.mu.Lock()
			bw.desc.size += bw.fragSize
			bw.desc.cv.Broadcast() // Tell blobmap BlobReader there's more to read.
			bw.fscabs.mu.Unlock()
		}
		bw.fragFile = nil
		bw.fragSize = 0
		bw.fragHash = nil
	}
	return err
}

// writeToTempFragment() writes buf[] to a temporary file that is eventually to become a fragment of *bw.
// Bytes are committed as necessary when a temporary file becomes large.
func (bw *BlobWriter) writeToTempFragment(buf []byte) (err error) {
	for len(buf) > 0 && err == nil {
		if bw.fragSize >= maxFragmentSize {
			err = bw.commitBytes()
		}
		if err == nil && bw.fragFile == nil {
			var fragFile *file
			fragFile, err = newTempFile(bw.ctx, filepath.Join(bw.fscabs.rootName, tmpDir))
			if err == nil {
				bw.fragFile = fragFile
				bw.fragSize = 0
				bw.fragHash = md5.New()
			}
		}
		// Process a prefix of buf[] on this iteration that will not violate maxFragmentSize.
		consume := buf
		if int64(len(buf))+bw.fragSize > maxFragmentSize {
			consume = buf[:maxFragmentSize-bw.fragSize]
		}
		if err == nil {
			bw.fragSize += int64(len(consume))
			bw.fragHash.Write(consume)                 // Cannot fail; see Hash interface.
			bw.hasher.Write(consume)                   // Cannot fail; see Hash interface.
			_, err = bw.fragFile.writer.Write(consume) // Writes all bytes unless returned err != nil.
		}
		buf = buf[len(consume):] // process rest of buf[] on next iteration
	}
	return err
}

// AppendBytes() tentatively appends bytes to the blob being written by *bw,
// where the bytes are composed of the byte vectors described by the elements
// of item[].  The implementation may choose when to commit these bytes to disc,
// except that they will be committed before the return of a subsequent call to
// Close() or CloseWithoutFinalize().  Uncomitted bytes may be lost during a crash,
// and will not be returned by a concurrent reader until they are committed.
func (bw *BlobWriter) AppendBytes(item ...localblobstore.BlockOrFile) (err error) {
	if bw.f == nil {
		panic("fs_cablobstore.BlobWriter programming error: AppendBytes() after Close()")
	}

	var buf []byte
	for i := 0; i != len(item) && err == nil; i++ {
		if len(item[i].FileName) != 0 {
			if buf == nil {
				buf = make([]byte, 8192, 8192)
			}
			var fileHandle *os.File
			fileHandle, err = os.Open(filepath.Join(bw.fscabs.rootName, item[i].FileName))
			if err == nil {
				at := item[i].Offset
				toRead := item[i].Size
				var haveRead int64
				for err == nil && (toRead == -1 || haveRead < toRead) {
					var n int
					n, err = fileHandle.ReadAt(buf, at)
					if err == nil {
						if toRead != -1 && int64(n)+haveRead > toRead {
							n = int(toRead - haveRead)
						}
						haveRead += int64(n)
						at += int64(n)
						err = bw.writeToTempFragment(buf[0:n])
					}
				}
				if err == io.EOF {
					if toRead == -1 || haveRead == toRead {
						err = nil // The loop read all that was asked; EOF is a possible outcome.
					} else { // The loop read less than was asked; request must have been too big.
						err = verror.New(errSizeTooBigForFragment, bw.ctx, bw.desc.name, item[i].FileName)
					}
				}
				fileHandle.Close()
			}
		} else {
			err = bw.writeToTempFragment(item[i].Block)
		}
	}
	return err
}

// forkWriteBlobMap() creates a new thread to run writeBlobMap().  It adds
// the chunks written to *bw to the blob store's BlobMap.  The caller is
// expected to call joinWriteBlobMap() at some later point.
func (bw *BlobWriter) forkWriteBlobMap() {
	// The descRef's ref count is incremented here to compensate
	// for the decrement it will receive in br.Close() in joinWriteBlobMap.
	if !bw.fscabs.descRef(bw.desc) {
		// Can't happen; descriptor's ref count was already non-zero.
		panic(verror.New(errBlobDeleted, bw.ctx, bw.desc.name))
	}
	bw.csBr = bw.fscabs.blobReaderFromDesc(bw.ctx, bw.desc, waitForWriter)
	bw.cs = chunker.NewStream(bw.ctx, &chunker.DefaultParam, bw.csBr)
	bw.csErr = make(chan error)
	go bw.writeBlobMap()
}

// insertChunk() inserts chunk into the blob store's BlobMap, associating it
// with the specified byte offset in the blob blobID being written by *bw.  The byte
// offset of the next chunk is returned.
func (bw *BlobWriter) insertChunk(blobID []byte, chunkHash []byte, offset int64, size int64) (int64, error) {
	err := bw.fscabs.bm.AssociateChunkWithLocation(bw.ctx, chunkHash[:],
		blobmap.Location{BlobID: blobID, Offset: offset, Size: size})
	if err != nil {
		bw.cs.Cancel()
	}
	return offset + size, err
}

// writeBlobMap() iterates over the chunk in stream bw.cs, and associates each
// one with the blob being written.
func (bw *BlobWriter) writeBlobMap() {
	var err error
	var offset int64
	blobID := fileNameToHash(blobDir, bw.desc.name)
	// Associate each chunk only after the next chunk has been seen (or
	// the blob finalized), to avoid recording an artificially short chunk
	// at the end of a partial transfer.
	var chunkHash [md5.Size]byte
	var chunkLen int64
	if bw.cs.Advance() {
		chunk := bw.cs.Value()
		// Record the hash and size, since chunk's underlying buffer
		// may be reused by the next call to Advance().
		chunkHash = md5.Sum(chunk)
		chunkLen = int64(len(chunk))
		for bw.cs.Advance() {
			offset, err = bw.insertChunk(blobID, chunkHash[:], offset, chunkLen)
			chunk = bw.cs.Value()
			chunkHash = md5.Sum(chunk)
			chunkLen = int64(len(chunk))
		}
	}
	if err == nil {
		err = bw.cs.Err()
	}
	bw.fscabs.mu.Lock()
	if err == nil && chunkLen != 0 && bw.desc.finalized {
		offset, err = bw.insertChunk(blobID, chunkHash[:], offset, chunkLen)
	}
	bw.fscabs.mu.Unlock()
	bw.csErr <- err // wake joinWriteBlobMap()
}

// joinWriteBlobMap waits for the completion of the thread forked by forkWriteBlobMap().
// It returns when the chunks in the blob have been written to the blob store's BlobMap.
func (bw *BlobWriter) joinWriteBlobMap(err error) error {
	err2 := <-bw.csErr // read error from end of writeBlobMap()
	if err == nil {
		err = err2
	}
	bw.csBr.Close()
	return err
}

// Close() finalizes *bw, and indicates that the client will perform no further
// append operations on *bw.  Any internal open file handles are closed.
func (bw *BlobWriter) Close() (err error) {
	if bw.f == nil {
		err = verror.New(errAlreadyClosed, bw.ctx, bw.desc.name)
	} else if bw.desc.finalized {
		err = verror.New(errBlobAlreadyFinalized, bw.ctx, bw.desc.name)
	} else {
		err = bw.commitBytes()
		if err == nil {
			h := bw.hasher.Sum(nil)
			_, err = fmt.Fprintf(bw.f.writer, "f %s\n", hashToString(h)) // finalize
		}
		_, err = bw.f.close(bw.ctx, err)
		bw.f = nil
		bw.fscabs.mu.Lock()
		bw.desc.finalized = true
		bw.desc.openWriter = false
		bw.desc.cv.Broadcast() // Tell blobmap BlobReader that writing has ceased.
		bw.fscabs.mu.Unlock()
		err = bw.joinWriteBlobMap(err)
		bw.fscabs.descUnref(bw.desc)
	}
	return err
}

// CloseWithoutFinalize() indicates that the client will perform no further
// append operations on *bw, but does not finalize the blob.  Any internal open
// file handles are closed.  Clients are expected to need this operation
// infrequently.
func (bw *BlobWriter) CloseWithoutFinalize() (err error) {
	if bw.f == nil {
		err = verror.New(errAlreadyClosed, bw.ctx, bw.desc.name)
	} else {
		err = bw.commitBytes()
		bw.fscabs.mu.Lock()
		bw.desc.openWriter = false
		bw.desc.cv.Broadcast() // Tell blobmap BlobReader that writing has ceased.
		bw.fscabs.mu.Unlock()
		_, err = bw.f.close(bw.ctx, err)
		bw.f = nil
		err = bw.joinWriteBlobMap(err)
		bw.fscabs.descUnref(bw.desc)
	}
	return err
}

// AppendBlob() adds a (substring of a) pre-existing blob to the blob being
// written by *bw.  The fragments of the pre-existing blob are not physically
// copied; they are referenced by both blobs.
func (bw *BlobWriter) AppendBlob(blobName string, size int64, offset int64) (err error) {
	if bw.f == nil {
		panic("fs_cablobstore.BlobWriter programming error: AppendBlob() after Close()")
	}
	err = bw.commitBytes()
	var desc *blobDesc
	var origSize int64
	if err == nil {
		desc, err = bw.fscabs.getBlob(bw.ctx, blobName)
		origSize = bw.desc.size
	}
	if err == nil {
		if size == -1 {
			size = desc.size - offset
		}
		if offset < 0 || desc.size < offset+size {
			err = verror.New(errBadSizeOrOffset, bw.ctx, size, offset, blobName, desc.size)
		}
		for i := 0; i != len(desc.fragment) && err == nil && size > 0; i++ {
			if desc.fragment[i].size <= offset {
				offset -= desc.fragment[i].size
			} else {
				consume := desc.fragment[i].size - offset
				if size < consume {
					consume = size
				}
				_, err = fmt.Fprintf(bw.f.writer, "d %d %d %s\n",
					consume, offset+desc.fragment[i].offset, desc.fragment[i].fileName)
				if err == nil {
					// Add fragment so garbage collector can see it.
					// The garbage collector cannot be
					// about to delete the fragment, because
					// getBlob() already checked for that
					// above, and kept a reference.
					bw.fscabs.mu.Lock()
					bw.desc.fragment = append(bw.desc.fragment, blobFragment{
						pos:      bw.desc.size,
						size:     consume,
						offset:   offset + desc.fragment[i].offset,
						fileName: desc.fragment[i].fileName})
					bw.desc.size += consume
					bw.desc.cv.Broadcast() // Tell blobmap BlobReader there's more to read.
					bw.fscabs.mu.Unlock()
				}
				offset = 0
				size -= consume
			}
		}
		bw.fscabs.descUnref(desc)
		// Add the new fragments to the running hash.
		if !bw.fscabs.descRef(bw.desc) {
			// Can't happen; descriptor's ref count was already
			// non-zero.
			panic(verror.New(errBlobDeleted, bw.ctx, blobName))
		}
		br := bw.fscabs.blobReaderFromDesc(bw.ctx, bw.desc, dontWaitForWriter)
		if err == nil {
			_, err = br.Seek(origSize, 0)
		}
		buf := make([]byte, 8192, 8192)
		for err == nil {
			var n int
			n, err = br.Read(buf)
			bw.hasher.Write(buf[0:n]) // Cannot fail; see Hash interface.
		}
		br.Close()
		if err == io.EOF { // EOF is expected.
			err = nil
		}
		if err == nil {
			err = bw.f.writer.Flush()
		}
	}
	return err
}

// IsFinalized() returns whether *bw has been finalized.
func (bw *BlobWriter) IsFinalized() bool {
	return bw.desc.finalized
}

// Size() returns *bw's size.
func (bw *BlobWriter) Size() int64 {
	return bw.desc.size + bw.fragSize // Count uncommited bytes in size for writer; they aren't yet counted for readers.
}

// Name() returns *bw's name.
func (bw *BlobWriter) Name() string {
	return bw.desc.name
}

// Hash() returns *bw's hash, reflecting the bytes written so far.
func (bw *BlobWriter) Hash() []byte {
	return bw.hasher.Sum(nil)
}

// -----------------------------------------------------------

// A BlobReader allows a blob to be read using the standard ReadAt(), Read(),
// and Seek() calls.  A BlobReader can be created with NewBlobReader(), and
// should be closed with the Close() method to avoid leaking file handles.
type BlobReader struct {
	// The BlobReader exists within a particular FsCaBlobStore and context.T.
	fscabs *FsCaBlobStore
	ctx    *context.T

	desc          *blobDesc // A description of the blob being read.
	waitForWriter bool      // whether this reader should wait for a concurrent BlobWriter

	pos int64 // The next position we will read from (used by Read/Seek, not ReadAt).

	// The fields below represent a cached open fragment desc.fragment[fragmentIndex].
	fragmentIndex int      // -1 or  0 <= fragmentIndex < len(desc.fragment).
	fh            *os.File // non-nil iff fragmentIndex != -1.
}

// constants to make the calls to blobReaderFromDesc invocations more readable
const (
	dontWaitForWriter = false
	waitForWriter     = true
)

// blobReaderFromDesc() returns a pointer to a newly allocated BlobReader given
// a pre-existing blobDesc.  If waitForWriter is true, the reader will wait for
// any BlobWriter to finish writing the part of the blob the reader is trying
// to read.
func (fscabs *FsCaBlobStore) blobReaderFromDesc(ctx *context.T, desc *blobDesc, waitForWriter bool) *BlobReader {
	br := new(BlobReader)
	br.fscabs = fscabs
	br.ctx = ctx
	br.fragmentIndex = -1
	br.desc = desc
	br.waitForWriter = waitForWriter
	return br
}

// NewBlobReader() returns a pointer to a newly allocated BlobReader on the
// specified blobName.  BlobReaders should not be used concurrently by multiple
// threads.  Returned handles should be closed with Close().
func (fscabs *FsCaBlobStore) NewBlobReader(ctx *context.T, blobName string) (br localblobstore.BlobReader, err error) {
	var desc *blobDesc
	desc, err = fscabs.getBlob(ctx, blobName)
	if err == nil {
		br = fscabs.blobReaderFromDesc(ctx, desc, dontWaitForWriter)
	}
	return br, err
}

// closeInternal() closes any open file handles within *br.
func (br *BlobReader) closeInternal() {
	if br.fh != nil {
		br.fh.Close()
		br.fh = nil
	}
	br.fragmentIndex = -1
}

// Close() indicates that the client will perform no further operations on *br.
// It closes any open file handles within a BlobReader.
func (br *BlobReader) Close() error {
	br.closeInternal()
	br.fscabs.descUnref(br.desc)
	return nil
}

// findFragment() returns the index of the first element of fragment[] that may
// contain "offset", based on the "pos" fields of each element.
// Requires that fragment[] be sorted on the "pos" fields of the elements.
func findFragment(fragment []blobFragment, offset int64) int {
	lo := 0
	hi := len(fragment)
	for lo < hi {
		mid := (lo + hi) >> 1
		if offset < fragment[mid].pos {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if lo > 0 {
		lo--
	}
	return lo
}

// waitUntilAvailable() waits until position pos within *br is available for
// reading, if this reader is waiting for writers.  This may be because:
//  - *br is on an already written blob.
//  - *br is on a blob being written that has been closed, or whose writes have
//    passed position pos.
// The value pos==math.MaxInt64 can be used to mean "until the writer is closed".
// Requires br.fscabs.mu held.
func (br *BlobReader) waitUntilAvailable(pos int64) {
	for br.waitForWriter && br.desc.openWriter && br.desc.size < pos {
		br.desc.cv.Wait()
	}
}

// ReadAt() fills b[] with up to len(b) bytes of data starting at position "at"
// within the blob that *br indicates, and returns the number of bytes read.
func (br *BlobReader) ReadAt(b []byte, at int64) (n int, err error) {
	br.fscabs.mu.Lock()
	br.waitUntilAvailable(at + int64(len(b)))
	i := findFragment(br.desc.fragment, at)
	if i < len(br.desc.fragment) && at <= br.desc.size {
		fragmenti := br.desc.fragment[i] // copy fragment data to allow releasing lock
		br.fscabs.mu.Unlock()
		if i != br.fragmentIndex {
			br.closeInternal()
		}
		if br.fragmentIndex == -1 {
			br.fh, err = os.Open(filepath.Join(br.fscabs.rootName, fragmenti.fileName))
			if err == nil {
				br.fragmentIndex = i
			} else {
				br.closeInternal()
			}
		}
		var offset int64 = at - fragmenti.pos + fragmenti.offset
		consume := fragmenti.size - (at - fragmenti.pos)
		if int64(len(b)) < consume {
			consume = int64(len(b))
		}
		if br.fh != nil {
			n, err = br.fh.ReadAt(b[0:consume], offset)
		} else if err == nil {
			panic("failed to open blob fragment")
		}
		br.fscabs.mu.Lock()
		// Return io.EOF if the Read reached the end of the last
		// fragment, but not if it's merely the end of some interior
		// fragment or the blob is still being extended.
		if int64(n)+at >= br.desc.size && !(br.waitForWriter && br.desc.openWriter) {
			if err == nil {
				err = io.EOF
			}
		} else if err == io.EOF {
			err = nil
		}
	} else if at == br.desc.size { // Reading at the end of the file, past the last fragment.
		err = io.EOF
	} else {
		err = verror.New(errIllegalPositionForRead, br.ctx, br.pos, br.desc.size)
	}
	br.fscabs.mu.Unlock()
	return n, err
}

// Read() fills b[] with up to len(b) bytes of data starting at the current
// seek position of *br within the blob that *br indicates, and then both
// returns the number of bytes read and advances *br's seek position by that
// amount.
func (br *BlobReader) Read(b []byte) (n int, err error) {
	n, err = br.ReadAt(b, br.pos)
	if err == nil {
		br.pos += int64(n)
	}
	return n, err
}

// Seek() sets the seek position of *br to offset if whence==0,
// offset+current_seek_position if whence==1, and offset+end_of_blob if
// whence==2, and then returns the current seek position.
func (br *BlobReader) Seek(offset int64, whence int) (result int64, err error) {
	br.fscabs.mu.Lock()
	if whence == 0 {
		result = offset
	} else if whence == 1 {
		result = offset + br.pos
	} else if whence == 2 {
		br.waitUntilAvailable(math.MaxInt64)
		result = offset + br.desc.size
	} else {
		err = verror.New(errBadSeekWhence, br.ctx, whence)
		result = br.pos
	}
	if result < 0 {
		err = verror.New(errNegativeSeekPosition, br.ctx, offset, whence)
		result = br.pos
	} else if result > br.desc.size {
		err = verror.New(errIllegalPositionForRead, br.ctx, result, br.desc.size)
		result = br.pos
	} else if err == nil {
		br.pos = result
	}
	br.fscabs.mu.Unlock()
	return result, err
}

// IsFinalized() returns whether *br has been finalized.
func (br *BlobReader) IsFinalized() bool {
	br.fscabs.mu.Lock()
	br.waitUntilAvailable(math.MaxInt64)
	finalized := br.desc.finalized
	br.fscabs.mu.Unlock()
	return finalized
}

// Size() returns *br's size.
func (br *BlobReader) Size() int64 {
	br.fscabs.mu.Lock()
	br.waitUntilAvailable(math.MaxInt64)
	size := br.desc.size
	br.fscabs.mu.Unlock()
	return size
}

// Name() returns *br's name.
func (br *BlobReader) Name() string {
	return br.desc.name
}

// Hash() returns *br's hash.  It may be nil if the blob is not finalized.
func (br *BlobReader) Hash() []byte {
	br.fscabs.mu.Lock()
	br.waitUntilAvailable(math.MaxInt64)
	hash := br.desc.hash
	br.fscabs.mu.Unlock()
	return hash
}

// -----------------------------------------------------------

// A dirListing is a list of names in a directory, plus a position, which
// indexes the last item in nameList that has been processed.
type dirListing struct {
	pos      int      // Current position in nameList; may be -1 at the start of iteration.
	nameList []string // List of directory entries.
}

// An FsCasIter represents an iterator that allows the client to enumerate all
// the blobs or fragments in a FsCaBlobStore.
type FsCasIter struct {
	fscabs *FsCaBlobStore // The parent FsCaBlobStore.
	err    error          // If non-nil, the error that terminated iteration.
	stack  []dirListing   // The stack of dirListings leading to the current entry.
	ctx    *context.T     // context passed to ListBlobIds() or ListCAIds()

	mu        sync.Mutex // Protects cancelled.
	cancelled bool       // Whether Cancel() has been called.
}

// ListBlobIds() returns an iterator that can be used to enumerate the blobs in
// an FsCaBlobStore.  Expected use is:
//    fscabsi := fscabs.ListBlobIds(ctx)
//    for fscabsi.Advance() {
//      // Process fscabsi.Value() here.
//    }
//    if fscabsi.Err() != nil {
//      // The loop terminated early due to an error.
//    }
func (fscabs *FsCaBlobStore) ListBlobIds(ctx *context.T) localblobstore.Stream {
	stack := make([]dirListing, 1)
	stack[0] = dirListing{pos: -1, nameList: []string{blobDir}}
	return &FsCasIter{fscabs: fscabs, stack: stack, ctx: ctx}
}

// ListCAIds() returns an iterator that can be used to enumerate the
// content-addressable fragments in an FsCaBlobStore.
// Expected use is:
//    fscabsi := fscabs.ListCAIds(ctx)
//    for fscabsi.Advance() {
//      // Process fscabsi.Value() here.
//    }
//    if fscabsi.Err() != nil {
//      // The loop terminated early due to an error.
//    }
func (fscabs *FsCaBlobStore) ListCAIds(ctx *context.T) localblobstore.Stream {
	stack := make([]dirListing, 1)
	stack[0] = dirListing{pos: -1, nameList: []string{casDir}}
	return &FsCasIter{fscabs: fscabs, stack: stack, ctx: ctx}
}

// isCancelled() returns whether Cancel() has been called.
func (fscabsi *FsCasIter) isCancelled() bool {
	fscabsi.mu.Lock()
	cancelled := fscabsi.cancelled
	fscabsi.mu.Unlock()
	return cancelled
}

// Advance() stages an item so that it may be retrieved via Value.  Returns
// true iff there is an item to retrieve.  Advance must be called before Value
// is called.
func (fscabsi *FsCasIter) Advance() (advanced bool) {
	stack := fscabsi.stack
	err := fscabsi.err

	for err == nil && !advanced && len(stack) != 0 && !fscabsi.isCancelled() {
		last := len(stack) - 1
		stack[last].pos++
		if stack[last].pos == len(stack[last].nameList) {
			stack = stack[0:last]
			fscabsi.stack = stack
		} else {
			fullName := filepath.Join(fscabsi.fscabs.rootName, fscabsi.Value())
			var fi os.FileInfo
			fi, err = os.Lstat(fullName)
			if err != nil {
				// error: nothing to do
			} else if fi.IsDir() {
				var dirHandle *os.File
				dirHandle, err = os.Open(fullName)
				if err == nil {
					var nameList []string
					nameList, err = dirHandle.Readdirnames(0)
					dirHandle.Close()
					stack = append(stack, dirListing{pos: -1, nameList: nameList})
					fscabsi.stack = stack
					last = len(stack) - 1
				}
			} else {
				advanced = true
			}
		}
	}

	if fscabsi.isCancelled() {
		if err == nil {
			fscabsi.err = verror.New(errStreamCancelled, fscabsi.ctx)
		}
		advanced = false
	}

	fscabsi.err = err
	return advanced
}

// Value() returns the item that was staged by Advance.  May panic if Advance
// returned false or was not called.  Never blocks.
func (fscabsi *FsCasIter) Value() (name string) {
	stack := fscabsi.stack
	if fscabsi.err == nil && len(stack) != 0 && stack[0].pos >= 0 {
		name = stack[0].nameList[stack[0].pos]
		for i := 1; i != len(stack); i++ {
			name = filepath.Join(name, stack[i].nameList[stack[i].pos])
		}
	}
	return name
}

// Err() returns any error encountered by Advance.  Never blocks.
func (fscabsi *FsCasIter) Err() error {
	return fscabsi.err
}

// Cancel() indicates that the iteration stream should terminate early.
// Never blocks.  May be called concurrently with other methods on fscabsi.
func (fscabsi *FsCasIter) Cancel() {
	fscabsi.mu.Lock()
	fscabsi.cancelled = true
	fscabsi.mu.Unlock()
}

// -----------------------------------------------------------

// An errorChunkStream is a localblobstore.ChunkStream that yields an error.
type errorChunkStream struct {
	err error
}

func (*errorChunkStream) Advance() bool       { return false }
func (*errorChunkStream) Value([]byte) []byte { return nil }
func (ecs *errorChunkStream) Err() error      { return ecs.err }
func (*errorChunkStream) Cancel()             {}

// BlobChunkStream() returns a ChunkStream that can be used to read the ordered
// list of content hashes of chunks in blob blobName.  It is expected that this
// list will be presented to RecipeFromChunks() on another device, to create a
// recipe for transmitting the blob efficiently to that other device.
func (fscabs *FsCaBlobStore) BlobChunkStream(ctx *context.T, blobName string) (cs localblobstore.ChunkStream) {
	blobID := fileNameToHash(blobDir, blobName)
	if blobID == nil {
		cs = &errorChunkStream{err: verror.New(errInvalidBlobName, ctx, blobName)}
	} else {
		cs = fscabs.bm.NewChunkStream(ctx, blobID)
	}
	return cs
}

// -----------------------------------------------------------

// LookupChunk returns the location of a chunk with the specified chunk hash
// within the store.
func (fscabs *FsCaBlobStore) LookupChunk(ctx *context.T, chunkHash []byte) (loc localblobstore.Location, err error) {
	var chunkMapLoc blobmap.Location
	chunkMapLoc, err = fscabs.bm.LookupChunk(ctx, chunkHash)
	if err == nil {
		loc.BlobName = hashToFileName(blobDir, chunkMapLoc.BlobID)
		loc.Size = chunkMapLoc.Size
		loc.Offset = chunkMapLoc.Offset
	}
	return loc, err
}

// -----------------------------------------------------------

// A RecipeStream implements localblobstore.RecipeStream.  It allows the client
// to iterate over the recipe steps to recreate a blob identified by a stream
// of chunk hashes (from chunkStream), but using parts of blobs in the current
// blob store where possible.
type RecipeStream struct {
	fscabs *FsCaBlobStore
	ctx    *context.T

	chunkStream     localblobstore.ChunkStream // the stream of chunks in the blob
	pendingChunkBuf [16]byte                   // a buffer for pendingChunk
	pendingChunk    []byte                     // the last unprocessed chunk hash read chunkStream, or nil if none
	step            localblobstore.RecipeStep  // the recipe step to be returned by Value()
	mu              sync.Mutex                 // protects cancelled
	cancelled       bool                       // whether Cancel() has been called
}

// RecipeStreamFromChunkStream() returns a pointer to a RecipeStream that allows
// the client to iterate over each RecipeStep needed to create the blob formed
// by the chunks in chunkStream.
func (fscabs *FsCaBlobStore) RecipeStreamFromChunkStream(ctx *context.T, chunkStream localblobstore.ChunkStream) localblobstore.RecipeStream {
	rs := new(RecipeStream)
	rs.fscabs = fscabs
	rs.ctx = ctx
	rs.chunkStream = chunkStream
	return rs
}

// isCancelled() returns whether rs.Cancel() has been called.
func (rs *RecipeStream) isCancelled() bool {
	rs.mu.Lock()
	cancelled := rs.cancelled
	rs.mu.Unlock()
	return cancelled
}

// Advance() stages an item so that it may be retrieved via Value().
// Returns true iff there is an item to retrieve.  Advance() must be
// called before Value() is called.  The caller is expected to read
// until Advance() returns false, or to call Cancel().
func (rs *RecipeStream) Advance() (ok bool) {
	if rs.pendingChunk == nil && rs.chunkStream.Advance() {
		rs.pendingChunk = rs.chunkStream.Value(rs.pendingChunkBuf[:])
	}
	for !ok && rs.pendingChunk != nil && !rs.isCancelled() {
		var err error
		var loc0 blobmap.Location
		loc0, err = rs.fscabs.bm.LookupChunk(rs.ctx, rs.pendingChunk)
		if err == nil {
			blobName := hashToFileName(blobDir, loc0.BlobID)
			var blobDesc *blobDesc
			if blobDesc, err = rs.fscabs.getBlob(rs.ctx, blobName); err != nil {
				// The BlobMap contained a reference to a
				// deleted blob.  Delete the reference in the
				// BlobMap; the next loop iteration will
				// consider the chunk again.
				rs.fscabs.bm.DeleteBlob(rs.ctx, loc0.BlobID)
			} else {
				rs.fscabs.descUnref(blobDesc)
				// The chunk is in a known blob.  Combine
				// contiguous chunks into a single recipe
				// entry.
				rs.pendingChunk = nil // consumed
				for rs.pendingChunk == nil && rs.chunkStream.Advance() {
					rs.pendingChunk = rs.chunkStream.Value(rs.pendingChunkBuf[:])
					var loc blobmap.Location
					loc, err = rs.fscabs.bm.LookupChunk(rs.ctx, rs.pendingChunk)
					if err == nil && bytes.Compare(loc0.BlobID, loc.BlobID) == 0 && loc.Offset == loc0.Offset+loc0.Size {
						loc0.Size += loc.Size
						rs.pendingChunk = nil // consumed
					}
				}
				rs.step = localblobstore.RecipeStep{Blob: blobName, Offset: loc0.Offset, Size: loc0.Size}
				ok = true
			}
		} else { // The chunk is not in the BlobMap; yield a single chunk hash.
			rs.step = localblobstore.RecipeStep{Chunk: rs.pendingChunk}
			rs.pendingChunk = nil // consumed
			ok = true
		}
	}
	return ok && !rs.isCancelled()
}

// Value() returns the item that was staged by Advance().  May panic if
// Advance() returned false or was not called.  Never blocks.
func (rs *RecipeStream) Value() localblobstore.RecipeStep {
	return rs.step
}

// Err() returns any error encountered by Advance.  Never blocks.
func (rs *RecipeStream) Err() error {
	// There are no errors to return here.  The errors encountered in
	// Advance() are expected and recoverable.
	return nil
}

// Cancel() indicates that the client wishes to cease reading from the stream.
// It causes the next call to Advance() to return false.  Never blocks.
// It may be called concurrently with other calls on the stream.
func (rs *RecipeStream) Cancel() {
	rs.mu.Lock()
	rs.cancelled = true
	rs.mu.Unlock()
	rs.chunkStream.Cancel()
}

// -----------------------------------------------------------

// gcTemp() attempts to delete files in dirName older than threshold.
// Errors are ignored.
func gcTemp(dirName string, threshold time.Time) {
	fh, err := os.Open(dirName)
	if err == nil {
		fi, _ := fh.Readdir(0)
		fh.Close()
		for i := 0; i < len(fi); i++ {
			if fi[i].ModTime().Before(threshold) {
				os.Remove(filepath.Join(dirName, fi[i].Name()))
			}
		}
	}
}

// GC() removes old temp files and content-addressed blocks that are no longer
// referenced by any blob.  It may be called concurrently with other calls to
// GC(), and with uses of BlobReaders and BlobWriters.
func (fscabs *FsCaBlobStore) GC(ctx *context.T) (err error) {
	// Remove old temporary files.
	gcTemp(filepath.Join(fscabs.rootName, tmpDir), time.Now().Add(-10*time.Hour))

	// Add a key to caSet for each content-addressed fragment in *fscabs,
	caSet := make(map[string]bool)
	caIter := fscabs.ListCAIds(ctx)
	for caIter.Advance() {
		caSet[caIter.Value()] = true
	}
	err = caIter.Err()

	// cmBlobs maps the names of blobs found in the BlobMap to their IDs.
	// (The IDs can be derived from the names; the map is really being used
	// to record which blobs exist, and the value merely avoids repeated
	// conversions.)
	cmBlobs := make(map[string][]byte)
	if err == nil {
		// Record all the blobs known to the BlobMap;
		bs := fscabs.bm.NewBlobStream(ctx)
		for bs.Advance() {
			blobID := bs.Value(nil)
			cmBlobs[hashToFileName(blobDir, blobID)] = blobID
		}
	}

	if err == nil {
		// Remove from cmBlobs all extant blobs, and remove from
		// caSet all their fragments.
		blobIter := fscabs.ListBlobIds(ctx)
		for blobIter.Advance() {
			var blobDesc *blobDesc
			if blobDesc, err = fscabs.getBlob(ctx, blobIter.Value()); err == nil {
				delete(cmBlobs, blobDesc.name)
				for i := range blobDesc.fragment {
					delete(caSet, blobDesc.fragment[i].fileName)
				}
				fscabs.descUnref(blobDesc)
			}
		}
	}

	if err == nil {
		// Remove all blobs still mentioned in cmBlobs from the BlobMap;
		// these are the ones that no longer exist in the blobs directory.
		for _, blobID := range cmBlobs {
			err = fscabs.bm.DeleteBlob(ctx, blobID)
			if err != nil {
				break
			}
		}
	}

	if err == nil {
		// Remove from caSet all fragments referenced by open BlobReaders and
		// BlobWriters.  Advertise to new readers and writers which blobs are
		// about to be deleted.
		fscabs.mu.Lock()
		for _, desc := range fscabs.activeDesc {
			for i := range desc.fragment {
				delete(caSet, desc.fragment[i].fileName)
			}
		}
		fscabs.toDelete = append(fscabs.toDelete, &caSet)
		fscabs.mu.Unlock()

		// Delete the things that still remain in caSet; they are no longer
		// referenced.
		for caName := range caSet {
			os.Remove(filepath.Join(fscabs.rootName, caName))
		}

		// Stop advertising what's been deleted.
		fscabs.mu.Lock()
		n := len(fscabs.toDelete)
		var i int
		// We require that &caSet still be in the list.
		for i = 0; fscabs.toDelete[i] != &caSet; i++ {
		}
		fscabs.toDelete[i] = fscabs.toDelete[n-1]
		fscabs.toDelete = fscabs.toDelete[0 : n-1]
		fscabs.mu.Unlock()
	}
	return err
}
