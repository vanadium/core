// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ptrie

// mostSignificantBit stores the mapping from a byte to the position of its most
// significant bit set to 1.
var mostSignificantBit [256]uint32

func init() {
	for i := 2; i < 256; i++ {
		mostSignificantBit[i] = mostSignificantBit[i>>1] + 1
	}
}

// bitlen returns the length of a slice in bits, i.e. 8 * len(slice).
func bitlen(slice []byte) uint32 {
	return uint32(len(slice)) << 3
}

// getBit returns i-th lexicographically significant bit of a slice.
// In other words, if we think of a byte slice as a big integer, getBit returns
// the i-th bit of that integer counting from the highest to the lowest.
func getBit(slice []byte, i uint32) byte {
	return (slice[i>>3] >> (7 - (i & 7))) & 1
}

// sliceBitLCP returns the length of the longest common prefix between two
// slices. The slices are compared as two bit strings. The first skipbits of
// both slices are virtually removed and ignored.
// NOTE: sliceBitLCP assumes that 0 <= skipbits < 8.
func sliceBitLCP(a, b []byte, skipbits uint32) uint32 {
	// Check that the first byte (without the first skipbits) is the same in
	// both slices.
	if x := (a[0] ^ b[0]) << skipbits; x != 0 {
		return 7 - mostSignificantBit[x]
	}
	minlen := len(a)
	if len(b) < minlen {
		minlen = len(b)
	}
	for i := 1; i < minlen; i++ {
		// (uint32(i<<3) - skipbits) --- the number of already compared bits.
		if x := a[i] ^ b[i]; x != 0 {
			return (uint32(i<<3) - skipbits) + (7 - mostSignificantBit[x])
		}
	}
	return uint32(minlen<<3) - skipbits
}

// copyNode returns a copy of the provided pnode struct.
func copyNode(node *pnode) *pnode {
	if node == nil {
		return nil
	}
	return &pnode{
		value: node.value,
		child: node.child,
	}
}

// copyNode returns a copy of the provided child struct.
func copyChild(child *pchild) *pchild {
	if child == nil {
		return nil
	}
	return &pchild{
		node:   child.node,
		bitstr: child.bitstr,
		bitlen: child.bitlen,
	}
}

// bitLCP returns the length of the longest common bit-prefix between the Path
// of a child and a slice. The first skipbits of both slices are virtually
// removed and ignored. Returns 0 if the child is nil.
func bitLCP(child *pchild, slice []byte, skipbits uint32) uint32 {
	if child == nil {
		return 0
	}
	lcp := sliceBitLCP(child.bitstr, slice, skipbits)
	if lcp > child.bitlen {
		lcp = child.bitlen
	}
	return lcp
}

// appendBits appends 'b' to 'a', assuming that 'a' has the provided bit length.
// If the bit length is a multiple of 8, appendBits just appends 'b' to 'a'.
// Otherwise appendBits appends 'b' to 'a' overlapping the first byte of 'b'
// with the last byte of 'a' so that the first bitlen bits of 'a' are unchanged.
//
// If 'a' has not enough capacity to hold the result, appendBits creates a new
// slice to hold the result. Otherwise the result is stored in 'a'.
func appendBits(a []byte, bitlen uint32, b []byte) []byte {
	newlen := int(bitlen>>3) + len(b)
	if newlen > cap(a) {
		tmp := make([]byte, newlen)
		copy(tmp, a)
		a = tmp
	}
	a = a[:newlen]
	oldByte := a[bitlen>>3]
	copy(a[bitlen>>3:], b)
	var bitmask byte = (1 << (8 - (bitlen & 7))) - 1
	a[bitlen>>3] = (bitmask & a[bitlen>>3]) | (^bitmask & oldByte)
	return a
}

// copyBytes returns a copy of the provided slice.
func copyBytes(slice []byte) []byte {
	result := make([]byte, len(slice))
	copy(result, slice)
	return result
}
