// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package framer

import (
	"bytes"
	"net"
	"testing"
	"time"
)

type readWriteCloser struct {
	bytes.Buffer
}

func (*readWriteCloser) Close() error {
	return nil
}

func newTCPConn(address string) (dialed, accepted net.Conn, err error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, nil, err
	}
	d, err := net.DialTimeout(l.Addr().Network(), l.Addr().String(), time.Second)
	if err != nil {
		return nil, nil, err
	}
	a, err := l.Accept()
	if err != nil {
		return nil, nil, err
	}
	l.Close()
	return d, a, nil
}

func TestFramer(t *testing.T) {
	testFramer(t, &framer{ReadWriteCloser: &readWriteCloser{}})
	dialed, accepted, err := newTCPConn(":0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			buf := make([]byte, 4096)
			n, err := accepted.Read(buf)
			if err != nil {
				break
			}
			if _, err := accepted.Write(buf[:n]); err != nil {
				break
			}
		}
	}()
	defer dialed.Close()
	testFramer(t, &framer{ReadWriteCloser: dialed})
}

func testFramer(t *testing.T, f *framer) {
	bufs := [][]byte{[]byte("read "), []byte("this "), []byte("please.")}
	want := []byte("read this please.")
	l := len(want)
	if n, err := f.WriteMsg(bufs...); err != nil || n != l {
		t.Fatalf("got %v, %v, want %v, nil", n, err, l)
	}
	if got, err := f.ReadMsg(); err != nil || !bytes.Equal(got, want) {
		t.Errorf("got %v, %v, want %v, nil", got, err, want)
	}
	// Framing a smaller message afterwards should reuse the internal buffer
	// from the first sent message.
	bufs = [][]byte{[]byte("read "), []byte("this "), []byte("too.")}
	want = []byte("read this too.")
	l = len(want)
	if n, err := f.WriteMsg(bufs...); err != nil || n != l {
		t.Fatalf("got %v, %v, want %v, nil", n, err, l)
	}
	if got, err := f.ReadMsg(); err != nil || !bytes.Equal(got, want) {
		t.Errorf("got %v, %v, want %v, nil", got, err, want)
	}
	// Sending larger message afterwards should work as well.
	bufs = [][]byte{[]byte("read "), []byte("this "), []byte("way bigger message.")}
	want = []byte("read this way bigger message.")
	l = len(want)
	if n, err := f.WriteMsg(bufs...); err != nil || n != l {
		t.Fatalf("got %v, %v, want %v, nil", n, err, l)
	}
	if got, err := f.ReadMsg(); err != nil || !bytes.Equal(got, want) {
		t.Errorf("got %v, %v, want %v, nil", got, err, want)
	}
}

func Test3ByteUint(t *testing.T) {
	var b [3]byte
	if err := write3ByteUint(b[:], 65555); err != nil {
		t.Error(err)
	}
	if got := read3ByteUint(b); got != 65555 {
		t.Errorf("got %v, want %v", got, 65555)
	}
}
