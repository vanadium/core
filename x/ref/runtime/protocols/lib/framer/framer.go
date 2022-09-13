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
	io.ReadWriteCloser
	readFrame  [sizeBytes]byte
	writeFrame [sizeBytes]byte
	writeBuf   []byte
}

// T represents the interface to a framer, see New, for a more complete
// explanation.
type T interface {
	// MsgReadWriteCloser reads/writes frames to the underlying stream.
	flow.MsgReadWriteCloser
	// ReadWriter provides raw, unframed, access to the underlying stream,
	// that is messages are written/read from it without notion of a frame.
	io.ReadWriter
	// FrameHeaderSize returns the size, in bytes, of the frame header.
	FrameHeaderSize() int
	// PutSize encodes the message size into FrameHeaderSize bytes at
	// the head of dst (ie. dst[0:FrameHeaderSize()])
	PutSize(dst []byte, msgSize int) error
	// GetSize retrieves the encoded message size from src[0:FrameHeaderSize()]
	GetSize(src []byte) int
}

// New creates a new instance of T that implements 'framing' over a
// raw connection via the flow.MsgReadWriteCloser methods. It also
// provides direct access (without framing) via the io.ReadWriter methods,
// which in conjunction with the SizeBytes, PutSize and Size methods can
// be used to send pre-framed messages to reduce the number of system
// calls and copying that is otherwise required.
// The framer may issue multiple writes to the underlying connection
// for a single message. For smaller messages it will copy the data into
// a single buffer and issue a single write. This combinded approach ensures
// that the framer has a fixed and known memory overhead.
func New(c io.ReadWriteCloser) T {
	f := &framer{
		ReadWriteCloser: c,
		writeBuf:        make([]byte, copyBound),
	}
	return f
}

func (f *framer) FrameHeaderSize() int {
	return sizeBytes
}

func (f *framer) PutSize(dst []byte, msgSize int) error {
	if msgSize > maxPacketSize {
		return ErrLargerThan3ByteUInt.Errorf(nil, "integer too large to represent in %v bytes", sizeBytes)
	}
	write3ByteUint(dst, msgSize)
	return nil
}

func (f *framer) GetSize(src []byte) int {
	return read3ByteUint(src)
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
		n, err := f.Write(f.writeBuf[:head])
		return n - 3, err
	}
	if msgSize > maxPacketSize {
		return 0, ErrLargerThan3ByteUInt.Errorf(nil, "integer too large to represent in %v bytes", sizeBytes)
	}
	write3ByteUint(f.writeFrame[:], msgSize)
	if n, err := f.Write(f.writeFrame[:]); err != nil {
		return n, err
	}
	written := 0
	for _, d := range data {
		n, err := f.Write(d)
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

// ReadMsg2 implements flow.MsgReadWriteCloser and will use
// the supplied msg buffer if it is large enough.
func (f *framer) ReadMsg2(msg []byte) ([]byte, error) {
	// Read the message size.
	if _, err := io.ReadFull(f, f.readFrame[:]); err != nil {
		return nil, err
	}
	msgSize := read3ByteUint(f.readFrame[:])

	// Read the message.
	if msgSize > len(msg) {
		msg = make([]byte, msgSize)
	}
	used := msg[:msgSize]
	if _, err := io.ReadFull(f, used); err != nil {
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

func read3ByteUint(src []byte) int {
	return maxPacketSize - (int(src[0])<<16 | int(src[1])<<8 | int(src[2]))
}
