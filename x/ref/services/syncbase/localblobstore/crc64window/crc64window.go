// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crc64window provides CRCs over fixed-sized, rolling windows of bytes.
//
// It uses the same polynomial representation and CRC conditioning as
// hash/crc64, so the results are the same as computing hash/crc64 over the
// window of the last sz bytes added, where sz is the window size.  Thus, in
// this code, rolling and nonRolling will receive the same value.
//     w := crc64window.New(crc64window.ECMA, 3)  // Window size is 3 bytes.
//     w.Advance(0x17)
//     w.Advance(0x92)
//     w.Advance(0x04)
//     rolling := w.Advance(0x28)   // Rolls 0x17 out, and 0x28 in.
//
//     nonRolling := crc64.Update(0, crc64.MakeTable(crc64.ECMA), []byte{0x92, 0x04, 0x28})
//
// Strangely, hash/crc64's specification does not mention which of the many
// possible bit representations and conditioning choices it uses.  We assume it
// will not change from the following, which was gleaned from the hash/crc64
// source code:
//
//  - All messages to be processed, CRC values, and CRC polynomials are
//    polynomials in x whose coefficients are in Z(2).
//  - CRC values are represented by uint64 values in which bit i of the integer
//    represents the coefficient of x**(63-i) in the polynomial.
//  - CRC polynomials are represented like CRC values, except that the x**64
//    coefficient of the CRC polynomial is implicitly 1, and not stored.
//  - Messages to be processed are represented by byte vectors in which the
//    lowest-order bit of the first byte is the highest-degree polynomial
//    coefficient.
//  - For a CRC polynomial p and a message m, the CRC value:
//        CRC(p, m) = c + ((c * (x**len(m)) + (m * x**64)) mod p)
//    where the conditioning constant c = x**63 + x**62 + x**61 + ... + x + 1,
//    and len(m) is the number of bits in m.
package crc64window

import "sync"

// The ECMA-64 polynomial, defined in ECMA 182.
// This polynomial is recommended for use with this package, though other
// polynomials found in hash/crc64 will also work.
const ECMA = 0xc96c5795d7870f42

// A Window contains the state needed to compute a CRC over a fixed-sized,
// rolling window of data.
type Window struct {
	crc     uint64   // CRC of window, unconditioned (i.e., just the window mod the CRC polynomial).
	window  []byte   // The bytes in the window.
	pos     int      // Index in window[] of first byte, which is the next byte to be overwritten.
	crcData *crcData // Pointer to the immutable CRC tables for the CRC.
}

// A crcData is immutable after initialization, and contains tables for
// computing a particular CRC over a particular window size.  Pre-computed
// copies of crcData are stored in tables[] so that CRC tables need be computed
// only once per (polynomial, window size) pair.
type crcData struct {
	conditioning  uint64
	crcTableFront [256]uint64
	crcTableRear  [256]uint64
}

var mu sync.Mutex // Protects "tables", the cache of CRC tables already computed.

// A polySize represents a pair of a CRC polynomial and a window size.
type polySize struct {
	poly uint64
	size int
}

// tables[] maps (polynomial,window size) pairs to computed tables, so tables
// are computed only once.  It's accessed only under mu.
var tables map[polySize]*crcData

// getCRCData() returns a pointer to a crcData for the given CRC polynomial
// and window size, either by cache lookup on by calculating it.  Requires
// size > 0.
func getCRCData(poly uint64, size int) *crcData {
	mu.Lock()
	// Use cached CRC tables if available.
	if tables == nil {
		tables = make(map[polySize]*crcData)
	}
	ps := polySize{poly: poly, size: size}
	c, found := tables[ps]
	if !found { // Compute and save the CRC tables.
		c = new(crcData)
		// Loop ensures:  c.crcTableFront[m & 0xff] ^ (m >> 8)==CRC(m * x**8)
		zeroOrPoly := []uint64{0, poly}
		for i := 1; i != 256; i <<= 1 {
			crc := uint64(i)
			for j := 0; j != 8; j++ {
				crc = (crc >> 1) ^ zeroOrPoly[crc&1]
			}
			for j := 0; j != i; j++ {
				c.crcTableFront[j+i] = crc ^ c.crcTableFront[j]
			}
		}
		// Loop ensures: c.crcTableRear[b] == CRC(b * x**(size*8))
		for i := 1; i != 256; i <<= 1 {
			crc := c.crcTableFront[i]
			for j := 1; j != size; j++ {
				crc = c.crcTableFront[byte(crc)] ^ (crc >> 8)
			}
			for j := 0; j != i; j++ {
				c.crcTableRear[j+i] = crc ^ c.crcTableRear[j]
			}
		}

		// Loop ensures: c.conditioning == CRC(all-ones * x**(size*8))
		conditioning := ^uint64(0)
		for i := 0; i != size; i++ {
			conditioning = c.crcTableFront[byte(conditioning)] ^ (conditioning >> 8)
		}
		c.conditioning = conditioning

		tables[ps] = c
	}
	mu.Unlock()
	return c
}

// New() returns a Window with the given size and CRC polynomial.
// Initially, all the bytes in the window are zero.  Requires size > 0.
func New(poly uint64, size int) *Window {
	if size <= 0 {
		panic("crc64window.New() called with size <= 0")
	}
	w := new(Window)
	w.window = make([]byte, size)
	w.crc = 0
	w.crcData = getCRCData(poly, size)
	return w
}

// Advance() removes the first byte from window *w, adds b as the new last
// byte, and returns the CRC of the window.
func (w *Window) Advance(b byte) uint64 {
	c := w.crcData
	pos := w.pos
	crc := w.crc
	crc ^= c.crcTableRear[w.window[pos]]
	w.crc = c.crcTableFront[byte(crc)^b] ^ (crc >> 8)
	w.window[pos] = b
	pos++
	if pos == len(w.window) {
		pos = 0
	}
	w.pos = pos
	return ^(c.conditioning ^ w.crc)
}
