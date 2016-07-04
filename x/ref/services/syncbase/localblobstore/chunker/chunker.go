// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package chunker breaks a stream of bytes into context-defined chunks whose
// boundaries are chosen based on content checksums of a window that slides
// over the data.  An edited sequence with insertions and removals can share
// many chunks with the original sequence.
//
// The intent is that when a sequence of bytes is to be transmitted to a
// recipient that may have much of the data, the sequence can be broken down
// into chunks.  The checksums of the resulting chunks may then be transmitted
// to the recipient, which can then discover which of the chunks it has, and
// which it needs.
//
// Example:
//      var s *chunker.Stream = chunker.New(&chunker.DefaultParam, anIOReader)
//      for s.Advance() {
//		var chunk []byte := s.Value()
//              // process chunk
//	}
//	if s.Err() != nil {
//		// anIOReader generated an error.
//	}
package chunker

// The design is from:
// "A Framework for Analyzing and Improving Content-Based Chunking Algorithms";
// Kave Eshghi, Hsiu Khuern Tang; HPL-2005-30(R.1); Sep, 2005;
// http://www.hpl.hp.com/techreports/2005/HPL-2005-30R1.pdf

import "io"
import "sync"

import "v.io/x/ref/services/syncbase/localblobstore/crc64window"
import "v.io/v23/context"
import "v.io/v23/verror"

const pkgPath = "v.io/x/ref/services/syncbase/localblobstore/chunker"

var (
	errStreamCancelled = verror.Register(pkgPath+".errStreamCancelled", verror.NoRetry, "{1:}{2:} Advance() called on cancelled stream{:_}")
)

// A Param contains the parameters for chunking.
//
// Chunks are broken based on a hash of a sliding window of width WindowWidth
// bytes.
// Each chunk is at most MaxChunk bytes long, and, unless end-of-file or an
// error is reached, at least MinChunk bytes long.
//
// Subject to those constaints, a chunk boundary introduced at the first point
// where the hash of the sliding window is 1 mod Primary, or if that doesn't
// occur before MaxChunk bytes, at the last position where the hash is 1 mod
// Secondary, or if that does not occur, after MaxChunk bytes.
// Normally, MinChunk < Primary < MaxChunk.
// Primary is the expected chunk size.
// The Secondary divisor exists to make it more likely that a chunk boundary is
// selected based on the local data when the Primary divisor by chance does not
// find a match for a long distance.  It should be a few times smaller than
// Primary.
//
// Using primes for Primary and Secondary is not essential, but recommended
// because it guarantees mixing of the checksum bits should their distribution
// be non-uniform.
type Param struct {
	WindowWidth int    // the window size to use when looking for chunk boundaries
	MinChunk    int64  // minimum chunk size
	MaxChunk    int64  // maximum chunk size
	Primary     uint64 // primary divisor; the expected chunk size
	Secondary   uint64 // secondary divisor
}

// DefaultParam contains default chunking parameters.
var DefaultParam Param = Param{WindowWidth: 48, MinChunk: 512, MaxChunk: 3072, Primary: 601, Secondary: 307}

// A Stream allows a client to iterate over the chunks within an io.Reader byte
// stream.
type Stream struct {
	param        Param               // chunking parameters
	ctx          *context.T          // context of creator
	window       *crc64window.Window // sliding window for computing the hash
	buf          []byte              // buffer of data
	rd           io.Reader           // source of data
	err          error               // error from rd
	mu           sync.Mutex          // protects cancelled
	cancelled    bool                // whether the stream has been cancelled
	bufferChunks bool                // whether to buffer entire chunks
	// Invariant:  bufStart <= chunkStart <= chunkEnd <= bufEnd
	bufStart   int64  // offset in rd of first byte in buf[]
	bufEnd     int64  // offset in rd of next byte after those in buf[]
	chunkStart int64  // offset in rd of first byte of current chunk
	chunkEnd   int64  // offset in rd of next byte after current chunk
	windowEnd  int64  // offset in rd of next byte to be given to window
	hash       uint64 // hash of sliding window
}

// newStream() returns a pointer to a new Stream instance, with the
// parameters in *param.  This internal version of NewStream() allows the caller
// to specify via bufferChunks whether entire chunks should be buffered.
func newStream(ctx *context.T, param *Param, rd io.Reader, bufferChunks bool) *Stream {
	s := new(Stream)
	s.param = *param
	s.ctx = ctx
	s.window = crc64window.New(crc64window.ECMA, s.param.WindowWidth)
	bufSize := int64(8192)
	if bufferChunks {
		// If we must buffer entire chunks, arrange that the buffer
		// size is considerably larger than the max chunk size to avoid
		// copying data repeatedly.
		for bufSize < 4*s.param.MaxChunk {
			bufSize *= 2
		}
	}
	s.buf = make([]byte, bufSize)
	s.rd = rd
	s.bufferChunks = bufferChunks
	return s
}

// NewStream() returns a pointer to a new Stream instance, with the
// parameters in *param.
func NewStream(ctx *context.T, param *Param, rd io.Reader) *Stream {
	return newStream(ctx, param, rd, true)
}

// isCancelled() returns whether s.Cancel() has been called.
func (s *Stream) isCancelled() (cancelled bool) {
	s.mu.Lock()
	cancelled = s.cancelled
	s.mu.Unlock()
	return cancelled
}

// Advance() stages the next chunk so that it may be retrieved via Value().
// Returns true iff there is an item to retrieve.  Advance() must be called
// before Value() is called.
func (s *Stream) Advance() bool {
	// Remember that s.{bufStart,bufEnd,chunkStart,chunkEnd,windowEnd}
	// are all relative to the offset in it.rd, not it.buf.
	// Therefore, these starts and ends can easily be compared
	// with each other, but we must subtract bufStart when
	// indexing into buf.  (Other schemes were considered, but
	// nothing seems uniformly better.)

	// If buffering entire chunks, ensure there's enough data in the buffer
	// for the next chunk.
	if s.bufferChunks && s.bufEnd < s.chunkEnd+s.param.MaxChunk && s.err == nil {
		// Next chunk might need more data.
		if s.bufStart < s.chunkEnd {
			// Move any remaining buffered data to start of buffer.
			copy(s.buf, s.buf[s.chunkEnd-s.bufStart:s.bufEnd-s.bufStart])
			s.bufStart = s.chunkEnd
		}
		// Fill buffer with data, unless error/EOF.
		for s.err == nil && s.bufEnd < s.bufStart+int64(len(s.buf)) && !s.isCancelled() {
			var n int
			n, s.err = s.rd.Read(s.buf[s.bufEnd-s.bufStart:])
			s.bufEnd += int64(n)
		}
	}

	// Make the next chunk current.
	s.chunkStart = s.chunkEnd
	minChunk := s.chunkStart + s.param.MinChunk
	maxChunk := s.chunkStart + s.param.MaxChunk
	lastSecondaryBreak := maxChunk

	// While not end of chunk...
	for s.windowEnd != maxChunk &&
		(s.windowEnd < minChunk || (s.hash%s.param.Primary) != 1) &&
		(s.windowEnd != s.bufEnd || s.err == nil) && !s.isCancelled() {

		// Fill the buffer if empty, and there's more data to read.
		if s.windowEnd == s.bufEnd && s.err == nil {
			if s.bufferChunks {
				panic("chunker.Advance had to fill buffer in bufferChunks mode")
			}
			s.bufStart = s.bufEnd
			var n int
			n, s.err = s.rd.Read(s.buf)
			s.bufEnd += int64(n)
		}

		// bufLimit is the minimum of the maximum possible chunk size and the buffer length.
		bufLimit := maxChunk
		if s.bufEnd < bufLimit {
			bufLimit = s.bufEnd
		}
		// Advance window until both MinChunk reached and primary boundary found.
		for s.windowEnd != bufLimit &&
			(s.windowEnd < minChunk || (s.hash%s.param.Primary) != 1) &&
			!s.isCancelled() {

			// Advance the window by one byte.
			s.hash = s.window.Advance(s.buf[s.windowEnd-s.bufStart])
			s.windowEnd++
			if (s.hash % s.param.Secondary) == 1 {
				lastSecondaryBreak = s.windowEnd
			}
		}
	}

	if s.windowEnd == maxChunk && (s.hash%s.param.Primary) != 1 && lastSecondaryBreak != maxChunk {
		// The primary break point was not found in the maximum chunk
		// size, and a secondary break point was found; use it.
		s.chunkEnd = lastSecondaryBreak
	} else {
		s.chunkEnd = s.windowEnd
	}

	return !s.isCancelled() && s.chunkStart != s.chunkEnd // We have a non-empty chunk to return.
}

// Value() returns the chunk that was staged by Advance().  May panic if
// Advance() returned false or was not called.  Never blocks.
func (s *Stream) Value() []byte {
	return s.buf[s.chunkStart-s.bufStart : s.chunkEnd-s.bufStart]
}

// Err() returns any error encountered by Advance().  Never blocks.
func (s *Stream) Err() (err error) {
	s.mu.Lock()
	if s.cancelled && (s.err == nil || s.err == io.EOF) {
		s.err = verror.New(errStreamCancelled, s.ctx)
	}
	s.mu.Unlock()
	if s.err != io.EOF { // Do not consider EOF to be an error.
		err = s.err
	}
	return err
}

// Cancel() causes the next call to Advance() to return false.
// It should be used when the client does not wish to iterate to the end of the stream.
// Never blocks.  May be called concurrently with other method calls on s.
func (s *Stream) Cancel() {
	s.mu.Lock()
	s.cancelled = true
	s.mu.Unlock()
}

// ----------------------------------

// A PosStream is just like a Stream, except that the Value() method returns only
// the byte offsets of the ends of chunks, rather than the chunks themselves.
// It can be used when chunks are too large to buffer a small number
// comfortably in memory.
type PosStream struct {
	s *Stream
}

// NewPosStream() returns a pointer to a new PosStream instance, with the
// parameters in *param.
func NewPosStream(ctx *context.T, param *Param, rd io.Reader) *PosStream {
	ps := new(PosStream)
	ps.s = newStream(ctx, param, rd, false)
	return ps
}

// Advance() stages the offset of the end of the next chunk so that it may be
// retrieved via Value().  Returns true iff there is an item to retrieve.
// Advance() must be called before Value() is called.
func (ps *PosStream) Advance() bool {
	return ps.s.Advance()
}

// Value() returns the chunk that was staged by Advance().  May panic if
// Advance() returned false or was not called.  Never blocks.
func (ps *PosStream) Value() int64 {
	return ps.s.chunkEnd
}

// Err() returns any error encountered by Advance().  Never blocks.
func (ps *PosStream) Err() error {
	return ps.s.Err()
}

// Cancel() causes the next call to Advance() to return false.
// It should be used when the client does not wish to iterate to the end of the stream.
// Never blocks.  May be called concurrently with other method calls on ps.
func (ps *PosStream) Cancel() {
	ps.s.Cancel()
}
