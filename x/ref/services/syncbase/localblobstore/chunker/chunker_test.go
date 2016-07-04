// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A test for the chunker package.
package chunker_test

import "bytes"
import "crypto/md5"
import "fmt"
import "io"
import "testing"

import "v.io/x/ref/services/syncbase/localblobstore/chunker"
import "v.io/x/ref/services/syncbase/localblobstore/localblobstore_testlib"
import "v.io/v23/context"
import "v.io/x/ref/test"
import _ "v.io/x/ref/runtime/factories/generic"

// TestChunksPartitionStream() tests that the chunker partitions its input
// stream into reasonable sized chunks, which when concatenated form the
// original stream.
func TestChunksPartitionStream(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var err error
	totalLength := 1024 * 1024

	// Compute the md5 of an arbiotrary stream.  We will later compare this
	// with the md5 of the concanenation of chunks from an equivalent
	// stream.
	r := localblobstore_testlib.NewRandReader(1, totalLength, 0, io.EOF)
	hStream := md5.New()
	buf := make([]byte, 8192)
	for err == nil {
		var n int
		n, err = r.Read(buf)
		hStream.Write(buf[0:n])
	}
	checksumStream := hStream.Sum(nil)

	// Using an equivalent stream, break it into chunks.
	r = localblobstore_testlib.NewRandReader(1, totalLength, 0, io.EOF)
	param := &chunker.DefaultParam
	hChunked := md5.New()

	length := 0
	s := chunker.NewStream(ctx, param, r)
	for s.Advance() {
		chunk := s.Value()
		length += len(chunk)
		// The last chunk is permitted to be short, hence the second
		// conjunct in the following predicate.
		if int64(len(chunk)) < param.MinChunk && length != totalLength {
			t.Errorf("chunker_test: chunk length %d below minimum %d", len(chunk), param.MinChunk)
		}
		if int64(len(chunk)) > param.MaxChunk {
			t.Errorf("chunker_test: chunk length %d above maximum %d", len(chunk), param.MaxChunk)
		}
		hChunked.Write(chunk)
	}
	if s.Err() != nil {
		t.Errorf("chunker_test: got error from chunker: %v\n", err)
	}

	if length != totalLength {
		t.Errorf("chunker_test: chunk lengths summed to %d, expected %d", length, totalLength)
	}

	checksumChunked := hChunked.Sum(nil)
	if bytes.Compare(checksumStream, checksumChunked) != 0 {
		t.Errorf("chunker_test: md5 of stream is %v, but md5 of chunks is %v", checksumStream, checksumChunked)
	}
}

// TestPosStream() tests that a PosStream leads to the same chunks as an Stream.
func TestPosStream(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	totalLength := 1024 * 1024

	s := chunker.NewStream(ctx, &chunker.DefaultParam,
		localblobstore_testlib.NewRandReader(1, totalLength, 0, io.EOF))
	ps := chunker.NewPosStream(ctx, &chunker.DefaultParam,
		localblobstore_testlib.NewRandReader(1, totalLength, 0, io.EOF))

	itReady := s.Advance()
	pitReady := ps.Advance()
	it_pos := 0
	chunk_count := 0
	for itReady && pitReady {
		it_pos += len(s.Value())
		if int64(it_pos) != ps.Value() {
			t.Fatalf("chunker_test: Stream and PosStream positions diverged at chunk %d: %d vs %d", chunk_count, it_pos, ps.Value())
		}
		chunk_count++
		itReady = s.Advance()
		pitReady = ps.Advance()
	}
	if itReady {
		t.Error("chunker_test: Stream ended before PosStream")
	}
	if pitReady {
		t.Error("chunker_test: PosStream ended before Stream")
	}
	if s.Err() != nil {
		t.Errorf("chunker_test: Stream got unexpected error: %v", s.Err())
	}
	if ps.Err() != nil {
		t.Errorf("chunker_test: PosStream got unexpected error: %v", ps.Err())
	}
}

// chunkSums() returns a vector of md5 checksums for the chunks of the
// specified Reader, using the default chunking parameters.
func chunkSums(ctx *context.T, r io.Reader) (sums [][md5.Size]byte) {
	s := chunker.NewStream(ctx, &chunker.DefaultParam, r)
	for s.Advance() {
		sums = append(sums, md5.Sum(s.Value()))
	}
	return sums
}

// TestInsertions() tests the how chunk sequences differ when bytes are
// periodically inserted into a stream.
func TestInsertions(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	totalLength := 1024 * 1024
	insertionInterval := 20 * 1024
	bytesInserted := totalLength / insertionInterval

	// Get the md5 sums of the chunks of two similar streams, where the
	// second has an extra bytes every 20k bytes.
	sums0 := chunkSums(ctx, localblobstore_testlib.NewRandReader(1, totalLength, 0, io.EOF))
	sums1 := chunkSums(ctx, localblobstore_testlib.NewRandReader(1, totalLength, insertionInterval, io.EOF))

	// Iterate over chunks of second stream, counting which are in common
	// with first stream.  We expect to find common chunks within 10 of the
	// last chunk in common, since insertions are single bytes, widely
	// separated.
	same := 0 // Number of chunks in sums1 that are the same as chunks in sums0.
	i0 := 0   // Where to search for a match in sums0.
	for i1 := 0; i1 != len(sums1); i1++ {
		// Be prepared to search up to the next 10 elements of sums0 from the most recent match.
		limit := len(sums0) - i0
		if limit > 10 {
			limit = 10
		}
		var d int
		for d = 0; d != limit && bytes.Compare(sums0[i0+d][:], sums1[i1][:]) != 0; d++ {
		}
		if d != limit { // found
			same++
			i0 += d // Advance i0 to the most recent match.
		}
	}
	// The number of chunks that aren't the same as one in the original stream should be at least as large
	// as the number of bytes inserted, and not too many more.
	different := len(sums1) - same
	if different < bytesInserted {
		t.Errorf("chunker_test: saw %d different chunks, but expected at least %d", different, bytesInserted)
	}
	if bytesInserted+(bytesInserted/2) < different {
		t.Errorf("chunker_test: saw %d different chunks, but expected at most %d", different, bytesInserted+(bytesInserted/2))
	}
	// Require that most chunks are the same, by a substantial margin.
	if same < 5*different {
		t.Errorf("chunker_test: saw %d different chunks, and %d same, but expected at least a factor of 5 more same than different", different, same)
	}
}

// TestError() tests the behaviour of a chunker when given an error by its
// reader.
func TestError(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	notEOF := fmt.Errorf("not EOF")
	totalLength := 50 * 1024
	r := localblobstore_testlib.NewRandReader(1, totalLength, 0, notEOF)
	s := chunker.NewStream(ctx, &chunker.DefaultParam, r)
	length := 0
	for s.Advance() {
		chunk := s.Value()
		length += len(chunk)
	}
	if s.Err() != notEOF {
		t.Errorf("chunker_test: chunk stream ended with error %v, expected %v", s.Err(), notEOF)
	}
	if length != totalLength {
		t.Errorf("chunker_test: chunk lengths summed to %d, expected %d", length, totalLength)
	}
}
