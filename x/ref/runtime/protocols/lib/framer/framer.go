// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package framer provides the ability to wrap a network connection
// (or any io.ReadWriteCloser) with framing that consists of a 3-byte
// length followed by data of that length.
package framer

import (
	"io"

	"v.io/v23/flow"
)

const (
	sizeBytes     = 3
	maxPacketSize = 0xffffff
	copyBound     = 8192
)

type framer struct {
	rwc        io.ReadWriteCloser
	readFrame  [sizeBytes]byte
	writeFrame [sizeBytes]byte
	writeBuf   []byte
}

// New creates a new framer instance that implements flow.MsgReadWriteCloser.
// The framer may issue multiple writes to the underlying connection
// for a single message. For smaller messages it will copy the data into
// a single buffer and issue a single write. This combinded approach ensures
// that the framer has a fixed and known memory overhead.
func New(c io.ReadWriteCloser) flow.MsgReadWriteCloser {
	f := &framer{
		rwc:      c,
		writeBuf: make([]byte, copyBound),
	}
	return f
}

// WriteMsg implements flow.MsgReadWriteCloser. The supplied data may be written
// to the underlying connection in one or more writes depending on the size
// of supplied data.
func (f *framer) WriteMsg(data ...[]byte) (int, error) {
	// Compute the message size.
	msgSize := 0
	for _, b := range data {
		msgSize += len(b)
	}
	if msgSize < (copyBound - sizeBytes) {
		write3ByteUint(f.writeBuf[:sizeBytes], msgSize)
		head := sizeBytes
		for _, d := range data {
			l := len(d)
			copy(f.writeBuf[head:head+l], d)
			head += l
		}
		n, err := f.rwc.Write(f.writeBuf[:head])
		return n - 3, err
	}
	if msgSize > maxPacketSize {
		return 0, ErrLargerThan3ByteUInt.Errorf(nil, "integer too large to represent in 3 bytes")
	}
	write3ByteUint(f.writeFrame[:], msgSize)
	if n, err := f.rwc.Write(f.writeFrame[:]); err != nil {
		return n, err
	}
	written := 0
	for _, d := range data {
		n, err := f.rwc.Write(d)
		if err != nil {
			return written + n, err
		}
		written += n
	}
	return written, nil
}

// ReadMsg implements flow.MsgReadWriteCloser.
func (f *framer) ReadMsg() ([]byte, error) {
	return f.ReadMsg2(nil)
}

// Close implements flow.MsgReadWriteCloser.
func (f *framer) Close() error {
	return f.rwc.Close()
}

// ReadMsg2 implements flow.MsgReadWriteCloser and will use
// the supplied msg buffer if it is large enough.
func (f *framer) ReadMsg2(msg []byte) ([]byte, error) {
	// Read the message size.
	if _, err := io.ReadFull(f.rwc, f.readFrame[:]); err != nil {
		return nil, err
	}
	msgSize := read3ByteUint(f.readFrame)

	// Read the message.
	if msgSize > len(msg) {
		msg = make([]byte, msgSize)
	}
	used := msg[:msgSize]
	if _, err := io.ReadFull(f.rwc, used); err != nil {
		return nil, err
	}
	return used, nil
}

func write3ByteUint(dst []byte, n int) {
	n = maxPacketSize - n
	dst[0] = byte((n & 0xff0000) >> 16)
	dst[1] = byte((n & 0x00ff00) >> 8)
	dst[2] = byte(n & 0x0000ff)
}

func read3ByteUint(src [sizeBytes]byte) int {
	return maxPacketSize - (int(src[0])<<16 | int(src[1])<<8 | int(src[2]))
}
