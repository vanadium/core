// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package framer

import (
	"io"

	"v.io/v23/flow"
)

// framer is a wrapper of io.ReadWriter that adds framing to a net.Conn
// and implements flow.MsgReadWriteCloser.
type framer struct {
	io.ReadWriteCloser
	buf   []byte
	frame [3]byte
}

func New(c io.ReadWriteCloser) flow.MsgReadWriteCloser {
	return &framer{ReadWriteCloser: c}
}

func (f *framer) WriteMsg(data ...[]byte) (int, error) {
	// Compute the message size.
	msgSize := 0
	for _, b := range data {
		msgSize += len(b)
	}
	// Construct a buffer to write that has space for the 3 bytes of framing.
	// If a previous buffer is large enough, reuse it.
	bufSize := msgSize + 3
	if bufSize > len(f.buf) {
		f.buf = make([]byte, bufSize)
	}
	if err := write3ByteUint(f.buf[:3], msgSize); err != nil {
		return 0, err
	}
	head := 3
	for _, b := range data {
		l := len(b)
		copy(f.buf[head:head+l], b)
		head += l
	}
	// Write the buffer to the io.ReadWriter. Remove the frame size
	// from the returned number of bytes written.
	n, err := f.Write(f.buf[:bufSize])
	if err != nil {
		return n - 3, err
	}
	return n - 3, nil
}

func (f *framer) ReadMsg() ([]byte, error) {
	// Read the message size.
	if _, err := io.ReadFull(f, f.frame[:]); err != nil {
		return nil, err
	}
	msgSize := read3ByteUint(f.frame)

	// Read the message.
	msg := make([]byte, msgSize)
	if _, err := io.ReadFull(f, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

const maxPacketSize = 0xffffff

func write3ByteUint(dst []byte, n int) error {
	if n > maxPacketSize || n < 0 {
		return ErrLargerThan3ByteUInt.Errorf(nil, "integer too large to represent in 3 bytes")
	}
	n = maxPacketSize - n
	dst[0] = byte((n & 0xff0000) >> 16)
	dst[1] = byte((n & 0x00ff00) >> 8)
	dst[2] = byte(n & 0x0000ff)
	return nil
}

func read3ByteUint(src [3]byte) int {
	return maxPacketSize - (int(src[0])<<16 | int(src[1])<<8 | int(src[2]))
}
