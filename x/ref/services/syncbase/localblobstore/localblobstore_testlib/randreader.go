// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package localblobstore_testlib

import "math/rand"

// A RandReader contains a pointer to a rand.Read, and a size limit.  Its
// pointers implement the Read() method from io.Reader, which yields bytes
// obtained from the random number generator.
type RandReader struct {
	rand           *rand.Rand // Source of random bytes.
	pos            int        // Number of bytes read.
	limit          int        // Max number of bytes that may be read.
	insertInterval int        // If non-zero, number of bytes between insertions of zero bytes.
	eofErr         error      // error to be returned at the end of the stream
}

// NewRandReader() returns a new RandReader with the specified seed and size limit.
// It yields eofErr when the end of the stream is reached.
// If insertInterval is non-zero, a zero byte is inserted into the stream every
// insertInterval bytes, before resuming getting bytes from the random number
// generator.
func NewRandReader(seed int64, limit int, insertInterval int, eofErr error) *RandReader {
	r := new(RandReader)
	r.rand = rand.New(rand.NewSource(seed))
	r.limit = limit
	r.insertInterval = insertInterval
	r.eofErr = eofErr
	return r
}

// Read() implements the io.Reader Read() method for *RandReader.
func (r *RandReader) Read(buf []byte) (n int, err error) {
	// Generate bytes up to the end of the stream, or the end of the buffer.
	max := r.limit - r.pos
	if len(buf) < max {
		max = len(buf)
	}
	for ; n != max; n++ {
		if r.insertInterval == 0 || (r.pos%r.insertInterval) != 0 {
			buf[n] = byte(r.rand.Int31n(256))
		} else {
			buf[n] = 0
		}
		r.pos++
	}
	if r.pos == r.limit {
		err = r.eofErr
	}
	return n, err
}
