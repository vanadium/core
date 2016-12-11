// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iobuf

// Slice refers to an iobuf and the byte slice for the actual data.
type Slice struct {
	iobuf    *buf
	free     uint   // Free area before base, if any.
	base     uint   // Index into the underlying iobuf.
	Contents []byte // iobuf.Contents[base:bound]
}

// Size returns the number of bytes in the Slice.
func (slice *Slice) Size() int {
	return len(slice.Contents)
}

// FreeEntirePrefix sets the free index to zero.  Be careful when using this,
// you should ensure that no Slices are using the free region.
func (slice *Slice) FreeEntirePrefix() {
	slice.free = 0
}

// Release releases the slice, decrementing the reference count on the iobuf
// and destroying the slice.
func (slice *Slice) Release() {
	if slice.iobuf != nil {
		slice.iobuf.release()
		slice.iobuf = nil
	}
	slice.Contents = nil
}

// ReleasePrevious releases the <prev> slice, extending the free prefix of the
// target slice if possible.
func (slice *Slice) ReleasePrevious(prev *Slice) {
	if prev.iobuf == slice.iobuf && prev.base+uint(len(prev.Contents)) == slice.free {
		slice.free = prev.free
	}
	prev.Release()
}

// TruncateFront removes <bytes> from the front of the Slice.
func (slice *Slice) TruncateFront(bytes uint) {
	if bytes > uint(len(slice.Contents)) {
		bytes = uint(len(slice.Contents))
	}
	slice.base += bytes
	slice.Contents = slice.Contents[bytes:]
}

// ExpandFront tries to expand the Slice by <bytes> before the front of the Slice.
// Returns true if the Slice was expanded.
func (slice *Slice) ExpandFront(bytes uint) bool {
	if slice.free+bytes > slice.base || slice.iobuf == nil {
		return false
	}
	bound := slice.base + uint(len(slice.Contents))
	slice.base -= bytes
	slice.Contents = slice.iobuf.Contents[slice.base:bound]
	return true
}

// Coalesce a sequence of slices.  If two slices are adjacent, they are
// combined.  Takes ownership of the slices, caller takes ownership of the
// result.
func Coalesce(slices []*Slice, maxSize uint) []*Slice {
	if len(slices) <= 1 {
		return slices
	}
	var result []*Slice
	c := slices[0]
	for i := 1; i != len(slices); i++ {
		s := slices[i]
		if uint(len(c.Contents)+len(s.Contents)) <= maxSize &&
			c.iobuf != nil && s.iobuf == c.iobuf &&
			c.base+uint(len(c.Contents)) == s.base {
			// The two slices are adjacent.  Merge them.
			c.Contents = c.iobuf.Contents[c.base : s.base+uint(len(s.Contents))]
			s.Release()
		} else {
			result = append(result, c)
			c = s
		}
	}
	return append(result, c)
}

// NewSlice creates a Slice from a byte array.  The value is not copied into an
// iobuf, it continues to refer to the buffer that was passed in.
func NewSlice(buf []byte) *Slice {
	return &Slice{Contents: buf}
}
