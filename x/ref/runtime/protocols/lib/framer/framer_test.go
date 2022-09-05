// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package framer

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

type readWriteCloser struct {
	rw    *bytes.Buffer
	wrops int
}

func (rwc *readWriteCloser) Write(data []byte) (int, error) {
	rwc.wrops++
	return rwc.rw.Write(data)
}
func (rwc *readWriteCloser) Read(buf []byte) (int, error) {
	return rwc.rw.Read(buf)
}

func (rwc *readWriteCloser) Close() error {
	return nil
}

func randomBuffer(size int) []byte {
	b := make([]byte, size)
	io.ReadFull(rand.Reader, b)
	return b
}

func TestFramer(t *testing.T) {
	rwc := &readWriteCloser{rw: &bytes.Buffer{}}
	f := New(rwc)

	writeAndRead := func(want []byte, bufs [][]byte, nops int) {
		rwc.wrops = 0
		l := len(want)
		if n, err := f.WriteMsg(bufs...); err != nil || n != l {
			t.Fatalf("got %v, %v, want %v, nil", n, err, l)
		}
		if got, want := rwc.wrops, nops; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		rwc.wrops = 0

		if got, err := f.ReadMsg(); err != nil || !bytes.Equal(got, want) {
			t.Errorf("got %v, %v, want %v, nil", got, err, want)
		}

		if n, err := f.WriteMsg(bufs...); err != nil || n != l {
			t.Fatalf("got %v, %v, want %v, nil", n, err, l)
		}

		rbuf := make([]byte, len(want)*2)
		got, err := f.ReadMsg2(rbuf)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("got %v, %v, want %v, nil", got, err, want)
		}
		rbuf[0] = 0xff
		if bytes.Equal(got, want) {
			t.Errorf("looks like ReadMsg2 did not use the supplied buffer")
		}
	}

	bufs := [][]byte{[]byte("read "), []byte("this "), []byte("please.")}
	want := []byte("read this please.")

	writeAndRead(want, bufs, 1)

	rnd := randomBuffer(1024)
	writeAndRead(rnd, [][]byte{rnd}, 1)

	rnd = randomBuffer(copyBound - 4)
	writeAndRead(rnd, [][]byte{rnd}, 1)

	rnd = randomBuffer(copyBound - 1)
	writeAndRead(rnd, [][]byte{rnd}, 2)

	rnd = randomBuffer(copyBound - 3)
	writeAndRead(rnd, [][]byte{rnd}, 2)

	rnd = randomBuffer(copyBound + 10)
	writeAndRead(rnd, [][]byte{rnd}, 2)
}

func Test3ByteUint(t *testing.T) {
	var b [3]byte
	write3ByteUint(b[:], 65555)
	if got := read3ByteUint(b); got != 65555 {
		t.Errorf("got %v, want %v", got, 65555)
	}
}
