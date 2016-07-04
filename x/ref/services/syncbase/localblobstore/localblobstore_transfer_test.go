// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Example code for transferring a blob from one device to another.
// See the simulateResumption constant to choose whether to simulate a full
// transfer or a resumed one.
package localblobstore_test

import "bytes"
import "io"
import "io/ioutil"
import "math/rand"
import "os"
import "testing"

import "v.io/v23/context"
import "v.io/v23/verror"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/localblobstore/fs_cablobstore"
import "v.io/x/ref/services/syncbase/store"
import "v.io/x/ref/test"
import _ "v.io/x/ref/runtime/factories/generic"

// These constants affect the type of transfer simulated by blobTransfer()
const (
	simulateSimpleTransfer = 0 // the receiver has none of the blob.
	simulateResumption     = 1 // the receiver has an unfinalized prefix of the blob.
	simulatePartialOverlap = 2 // the receiver has some of the blob, but is missing a prefix and a suffix.
)

// createBlobStore() returns a new BlobStore, and the name of the directory
// used to implement it.
func createBlobStore(ctx *context.T) (bs localblobstore.BlobStore, dirName string) {
	var err error
	if dirName, err = ioutil.TempDir("", "localblobstore_transfer_test"); err != nil {
		panic(err)
	}
	if bs, err = fs_cablobstore.Create(ctx, store.EngineForTest, dirName); err != nil {
		panic(err)
	}
	return bs, dirName
}

// createBlob writes a blob to bs of "count" 32kByte blocks drawn from a determinstic
// but arbitrary random stream, starting at block offset within that stream.
// Returns its name, which is "blob" if non-empty, and chosen arbitrarily otherwise.
// The blob is finalized iff "complete" is true.
func createBlob(ctx *context.T, bs localblobstore.BlobStore, blob string, complete bool, offset int, count int) string {
	var bw localblobstore.BlobWriter
	var err error
	if bw, err = bs.NewBlobWriter(ctx, blob); err != nil {
		panic(err)
	}
	blob = bw.Name()
	var buffer [32 * 1024]byte
	block := localblobstore.BlockOrFile{Block: buffer[:]}
	r := rand.New(rand.NewSource(1)) // Always seed with 1 for repeatability.
	for i := 0; i != offset+count; i++ {
		for b := 0; b != len(buffer); b++ {
			buffer[b] = byte(r.Int31n(256))
		}
		if i >= offset {
			if err = bw.AppendBytes(block); err != nil {
				panic(err)
			}
		}
	}
	if complete {
		err = bw.Close()
	} else {
		err = bw.CloseWithoutFinalize()
	}
	if err != nil {
		panic(err)
	}
	return blob
}

// A channelChunkStream turns a channel of chunk hashes into a ChunkStream.
type channelChunkStream struct {
	channel <-chan []byte
	ok      bool
	value   []byte
}

// newChannelChunkStream returns a ChunkStream, given a channel containing the
// relevant chunk hashes.
func newChannelChunkStream(ch <-chan []byte) localblobstore.ChunkStream {
	return &channelChunkStream{channel: ch, ok: true}
}

// The following are the standard ChunkStream methods.
func (cs *channelChunkStream) Advance() bool {
	if cs.ok {
		cs.value, cs.ok = <-cs.channel
	}
	return cs.ok
}
func (cs *channelChunkStream) Value(buf []byte) []byte { return cs.value }
func (cs *channelChunkStream) Err() error              { return nil }
func (cs *channelChunkStream) Cancel()                 {}

// blobTransfer() demonstrates how to transfer a blob incrementally
// from one device's blob store to another.  In this code, the communication
// between sender and receiver is modelled with Go channels.
// simulate affects what bytes the receiver already has when performing a transfer, see the constants
// simulateSimpleTransfer, simulateResumption, simulatePartialOverlap, above.
// The number of chunks transferred is returned.
func blobTransfer(ctx *context.T, simulate int) int {
	// ----------------------------------------------
	// Channels used to send chunk hashes to receiver always end in
	// ToSender or ToReceiver.
	type blobData struct {
		name     string
		size     int64
		checksum []byte
	}
	blobDataToReceiver := make(chan blobData)  // indicate basic data for blob
	needChunksToSender := make(chan bool)      // indicate receiver does not have entire blob
	chunkHashesToReceiver := make(chan []byte) // for initial trasfer of chunk hashes
	chunkHashesToSender := make(chan []byte)   // to report which chunks receiver needs
	chunksToReceiver := make(chan []byte)      // to report which chunks receiver needs

	sDone := make(chan bool) // closed when sender done
	rDone := make(chan int)  // closed when receiver done; receiver sends number of chunks transferred

	// ----------------------------------------------
	// The sender.
	go func(ctx *context.T,
		blobDataToReceiver chan<- blobData,
		needChunksToSender <-chan bool,
		chunkHashesToReceiver chan<- []byte,
		chunkHashesToSender <-chan []byte,
		chunksToReceiver chan<- []byte,
		done chan<- bool) {

		defer close(done)
		var err error

		bsS, bsSDir := createBlobStore(ctx)
		defer os.RemoveAll(bsSDir)

		blob := createBlob(ctx, bsS, "", true, 0, 32) // Create a 1M blob at the sender.

		// 1. Send basic blob data to receiver.
		var br localblobstore.BlobReader
		if br, err = bsS.NewBlobReader(ctx, blob); err != nil {
			panic(err)
		}
		blobDataToReceiver <- blobData{name: blob, size: br.Size(), checksum: br.Hash()}
		br.Close()
		close(blobDataToReceiver)

		// 3. Get indication from receiver of whether it needs blob.
		needChunks := <-needChunksToSender

		if !needChunks { // Receiver has blob; done.
			return
		}

		// 4. Send the chunk hashes to the receiver.  This proceeds concurrently
		//    with the step below.
		go func(ctx *context.T, blob string, chunkHashesToReceiver chan<- []byte) {
			cs := bsS.BlobChunkStream(ctx, blob)
			for cs.Advance() {
				chunkHashesToReceiver <- cs.Value(nil)
			}
			if cs.Err() != nil {
				panic(cs.Err())
			}
			close(chunkHashesToReceiver)
		}(ctx, blob, chunkHashesToReceiver)

		// 7. Get needed chunk hashes from receiver, find the relevant
		//    data, and send it back to the receiver.
		var cbr localblobstore.BlobReader // Cached read handle on most-recent-read blob, or nil
		// Given chunk hash h from chunkHashesToSender, send chunk to chunksToReceiver.
		for h := range chunkHashesToSender {
			loc, err := bsS.LookupChunk(ctx, h)
			for err == nil && (cbr == nil || cbr.Name() != loc.BlobName) {
				if cbr != nil && cbr.Name() != loc.BlobName {
					cbr.Close()
					cbr = nil
				}
				if cbr == nil {
					if cbr, err = bsS.NewBlobReader(ctx, loc.BlobName); err != nil {
						bsS.GC(ctx) // A partially-deleted blob may be confusing things.
						loc, err = bsS.LookupChunk(ctx, h)
					}
				}
			}
			var i int = 1
			var n int64
			buffer := make([]byte, loc.Size) // buffer for current chunk
			for n = int64(0); n != loc.Size && i != 0 && err == nil; n += int64(i) {
				if i, err = cbr.ReadAt(buffer[n:loc.Size], n+loc.Offset); err == io.EOF {
					err = nil // EOF is expected
				}
			}
			if n == loc.Size { // Got chunk.
				chunksToReceiver <- buffer[:loc.Size]
			}
			if err != nil {
				break
			}
		}
		close(chunksToReceiver)
		if cbr != nil {
			cbr.Close()
		}

	}(ctx, blobDataToReceiver, needChunksToSender, chunkHashesToReceiver, chunkHashesToSender, chunksToReceiver, sDone)

	// ----------------------------------------------
	// The receiver.
	go func(ctx *context.T,
		blobDataToReceiver <-chan blobData,
		needChunksToSender chan<- bool,
		chunkHashesToReceiver <-chan []byte,
		chunkHashesToSender chan<- []byte,
		chunksToReceiver <-chan []byte,
		done chan<- int) {

		defer close(done)
		var err error

		bsR, bsRDir := createBlobStore(ctx)
		defer os.RemoveAll(bsRDir)

		// 2. Receive basic blob data from sender.
		blobInfo := <-blobDataToReceiver

		switch simulate {
		case simulateSimpleTransfer:
			// Nothing to do.
		case simulateResumption:
			// Write a fraction of the (unfinalized) blob on the receiving side
			// to check that the transfer process can resume a partial blob.
			createBlob(ctx, bsR, blobInfo.name, false, 0, 10)
		case simulatePartialOverlap:
			// Write a blob with a different name that contains some of the same data.
			// The prefix and suffix of the full blob will have to be transferred.
			createBlob(ctx, bsR, "", true, 1, 10)
		}

		// 3. Tell sender whether the recevier already has the complete
		//    blob.
		needChunks := true
		var br localblobstore.BlobReader
		if br, err = bsR.NewBlobReader(ctx, blobInfo.name); err == nil {
			if br.IsFinalized() {
				if len(br.Hash()) == len(blobInfo.checksum) && bytes.Compare(br.Hash(), blobInfo.checksum) != 0 {
					panic("receiver has a finalized blob with same name but different hash")
				}
				needChunks = false // The receiver already has the blob.
			}
			br.Close()
		}
		needChunksToSender <- needChunks
		close(needChunksToSender)

		if !needChunks { // Receiver has blob; done.
			return
		}

		// 5. Receive the chunk hashes from the sender, and turn them
		//    into a recipe.
		cs := newChannelChunkStream(chunkHashesToReceiver)
		rs := bsR.RecipeStreamFromChunkStream(ctx, cs)

		// 6. The following thread sends the chunk hashes that the
		//    receiver does not have to the sender.  It also makes
		//    a duplicate of the stream on the channel rsCopy.  The
		//    buffering in rsCopy allows the receiver to put several
		//    chunks into a fragment.
		rsCopy := make(chan localblobstore.RecipeStep, 100) // A buffered copy of the rs stream.
		go func(ctx *context.T, rs localblobstore.RecipeStream, rsCopy chan<- localblobstore.RecipeStep, chunkHashesToSender chan<- []byte) {
			for rs.Advance() {

				step := rs.Value()
				if step.Chunk != nil { // Data must be fetched from sender.
					chunkHashesToSender <- step.Chunk
				}
				rsCopy <- step
			}
			close(chunkHashesToSender)
			close(rsCopy)
		}(ctx, rs, rsCopy, chunkHashesToSender)

		// 8. The following thread splices the chunks from the sender
		//    (on chunksToReceiver) into the recipe stream copy
		//    (rsCopy) to generate a full recipe stream (rsFull) in
		//    which chunks are actual data, rather than just hashes.
		rsFull := make(chan localblobstore.RecipeStep) // A recipe stream containing chunk data, not just hashes.
		go func(ctx *context.T, rsCopy <-chan localblobstore.RecipeStep, chunksToReceiver <-chan []byte, rsFull chan<- localblobstore.RecipeStep) {
			var ok bool
			for step := range rsCopy {
				if step.Chunk != nil { // Data must be fetched from sender.
					if step.Chunk, ok = <-chunksToReceiver; !ok {
						break
					}
				}
				rsFull <- step
			}
			close(rsFull)
		}(ctx, rsCopy, chunksToReceiver, rsFull)

		// 9. Write the blob using the recipe.
		var chunksTransferred int
		const fragmentThreshold = 1024 * 1024 // Try to write on-disc fragments fragments at least this big.
		var ignoreBytes int64
		var bw localblobstore.BlobWriter
		if bw, err = bsR.ResumeBlobWriter(ctx, blobInfo.name); err != nil {
			bw, err = bsR.NewBlobWriter(ctx, blobInfo.name)
		} else {
			ignoreBytes = bw.Size()
		}
		if err == nil {
			var fragment []localblobstore.BlockOrFile
			var fragmentSize int64
			for step := range rsFull {
				if step.Chunk == nil { // Data can be obtained from local blob.
					if ignoreBytes >= step.Size { // Ignore chunks we already have.
						ignoreBytes -= step.Size
					} else {
						if err == nil && len(fragment) != 0 {
							err = bw.AppendBytes(fragment...)
							fragment = fragment[:0]
							fragmentSize = 0
						}
						err = bw.AppendBlob(step.Blob, step.Size-ignoreBytes, step.Offset+ignoreBytes)
						ignoreBytes = 0
					}
				} else if ignoreBytes >= int64(len(step.Chunk)) { // Ignore chunks we already have.
					ignoreBytes -= int64(len(step.Chunk))
				} else { // Data is from a chunk send by the sender.
					chunksTransferred++
					fragment = append(fragment, localblobstore.BlockOrFile{Block: step.Chunk[ignoreBytes:]})
					fragmentSize += int64(len(step.Chunk)) - ignoreBytes
					ignoreBytes = 0
					if fragmentSize > fragmentThreshold {
						err = bw.AppendBytes(fragment...)
						fragment = fragment[:0]
						fragmentSize = 0
					}
				}
				if err != nil {
					break
				}
			}
			if err == nil && len(fragment) != 0 {
				err = bw.AppendBytes(fragment...)
			}
			if err2 := bw.Close(); err == nil {
				err = err2
			}
			if err != nil {
				panic(verror.DebugString(err))
			}
		}

		// 10. Verify that the blob was written correctly.
		if br, err = bsR.NewBlobReader(ctx, blobInfo.name); err != nil {
			panic(err)
		}
		if br.Size() != blobInfo.size {
			panic("transferred blob has wrong size")
		}
		if len(br.Hash()) != len(blobInfo.checksum) || bytes.Compare(br.Hash(), blobInfo.checksum) != 0 {
			panic("transferred blob has wrong checksum")
		}
		if err = br.Close(); err != nil {
			panic(err)
		}
		done <- chunksTransferred
	}(ctx, blobDataToReceiver, needChunksToSender, chunkHashesToReceiver, chunkHashesToSender, chunksToReceiver, rDone)

	// ----------------------------------------------
	// Wait for sender and receiver to finish, and return the number of chunks the receiver transferred.
	_ = <-sDone
	return <-rDone
}

func TestSimpleTransfer(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	const expectedChunksTransferred = 936
	var chunksTransferred int = blobTransfer(ctx, simulateSimpleTransfer)
	if chunksTransferred != expectedChunksTransferred {
		t.Errorf("simple transfer, expected %d chunks transferred, got %d", expectedChunksTransferred, chunksTransferred)
	}
}

func TestResumption(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	const expectedChunksTransferred = 635
	var chunksTransferred int = blobTransfer(ctx, simulateResumption)
	if chunksTransferred != expectedChunksTransferred {
		t.Errorf("resumption transfer, expected %d chunks transferred, got %d", expectedChunksTransferred, chunksTransferred)
	}
}

func TestPartialOverlap(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	const expectedChunksTransferred = 647
	var chunksTransferred int = blobTransfer(ctx, simulatePartialOverlap)
	if chunksTransferred != expectedChunksTransferred {
		t.Errorf("partial overlap transfer, expected %d chunks transferred, got %d", expectedChunksTransferred, chunksTransferred)
	}
}
