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
	frameStorage    [6]byte
	readFrameSlice  []byte
	writeFrameSlice []byte
}

func New(c io.ReadWriteCloser) flow.MsgReadWriteCloser {
	f := &framer{ReadWriteCloser: c}
	f.readFrameSlice = f.frameStorage[:3]
	f.writeFrameSlice = f.frameStorage[3:]
	return f
}

func (f *framer) WriteMsg(data ...[]byte) (int, error) {
	// Compute the message size.
	msgSize := 0
	for _, b := range data {
		msgSize += len(b)
	}
	if err := write3ByteUint(f.writeFrameSlice, msgSize); err != nil {
		return 0, err
	}
	if _, err := f.Write(f.writeFrameSlice); err != nil {
		return 0, err
	}
	// Write the buffer to the io.ReadWriter. Remove the frame size
	// from the returned number of bytes written.
	written := 0
	for _, d := range data {
		n, err := f.Write(d)
		if err != nil {
			return written, err
		}
		written += n
	}
	return written, nil
}

func (f *framer) ReadMsg() ([]byte, error) {
	// Read the message size.
	if _, err := io.ReadAtLeast(f, f.readFrameSlice, 3); err != nil {
		return nil, err
	}
	msgSize := read3ByteUint(f.readFrameSlice)

	// Read the message.
	msg := make([]byte, msgSize)
	if _, err := io.ReadAtLeast(f, msg, msgSize); err != nil {
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

func read3ByteUint(src []byte) int {
	return maxPacketSize - (int(src[0])<<16 | int(src[1])<<8 | int(src[2]))
}
