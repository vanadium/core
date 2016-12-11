// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

// TODO(toddw): Add more exhaustive tests of decbuf SetLimit and fill.

import (
	"io"
	"strings"
	"testing"
)

func expectEncbufBytes(t *testing.T, b *encbuf, expect string) {
	if got, want := b.Len(), len(expect); got != want {
		t.Errorf("len got %d, want %d", got, want)
	}
	if got, want := string(b.Bytes()), expect; got != want {
		t.Errorf("bytes got %q, want %q", b.Bytes(), expect)
	}
}

func TestEncbuf(t *testing.T) {
	b := newEncbuf()
	expectEncbufBytes(t, b, "")
	copy(b.Grow(5), "abcde*****")
	expectEncbufBytes(t, b, "abcde")
	b.WriteOneByte('1')
	expectEncbufBytes(t, b, "abcde1")
	b.Write([]byte("def"))
	expectEncbufBytes(t, b, "abcde1def")
	b.Reset()
	expectEncbufBytes(t, b, "")
	b.Write([]byte("123"))
	expectEncbufBytes(t, b, "123")
}

func testEncbufReserve(t *testing.T, base int) {
	for _, size := range []int{base - 1, base, base + 1} {
		str := strings.Repeat("x", size)
		// Test starting empty and writing size.
		b := newEncbuf()
		b.Write([]byte(str))
		expectEncbufBytes(t, b, str)
		b.Write([]byte(str))
		expectEncbufBytes(t, b, str+str)
		// Test starting with one byte and writing size.
		b = newEncbuf()
		b.WriteOneByte('A')
		expectEncbufBytes(t, b, "A")
		b.Write([]byte(str))
		expectEncbufBytes(t, b, "A"+str)
		b.Write([]byte(str))
		expectEncbufBytes(t, b, "A"+str+str)
	}
}

func TestEncbufReserve(t *testing.T) {
	testEncbufReserve(t, minBufFree)
	testEncbufReserve(t, minBufFree*2)
	testEncbufReserve(t, minBufFree*3)
	testEncbufReserve(t, minBufFree*4)
}

func expectReadAvailable(t *testing.T, mode string, b *decbuf, n int, expect string, expectErr error) {
	var err error
	if !b.IsAvailable(n) {
		err = b.Fill(n)
	}
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadAvailable err got %v, want %v", mode, got, want)
	}
	if err != nil {
		return
	}
	buf := b.ReadAvailable(n)
	if got, want := string(buf), expect; got != want {
		t.Errorf("%s ReadAvailable buf got %q, want %q", mode, got, want)
	}
}

func expectSkip(t *testing.T, mode string, b *decbuf, n int, expectErr error) {
	err := b.Skip(n)
	if got, want := err, expectErr; got != want {
		t.Errorf("%s Skip err got %v, want %v", mode, got, want)
	}
}

func expectPeekAvailable(t *testing.T, mode string, b *decbuf, n int, expect string, expectErr error) {
	var err error
	if !b.IsAvailable(n) {
		err = b.Fill(n)
	}
	if got, want := err, expectErr; got != want {
		t.Errorf("%s PeekAvailable err got %v, want %v", mode, got, want)
		return
	}
	if err != nil {
		return
	}
	buf := b.PeekAvailable(n)
	if got, want := string(buf), expect; got != want {
		t.Errorf("%s PeekAvailable buf got %q, want %q", mode, got, want)
	}
}

func expectReadAvailableByte(t *testing.T, mode string, b *decbuf, expect byte, expectErr error) {
	var err error
	if !b.IsAvailable(1) {
		err = b.Fill(1)
	}
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadAvailableByte err got %v, want %v", mode, got, want)
	}
	if err != nil {
		return
	}
	if got, want := b.ReadAvailableByte(), expect; got != want {
		t.Errorf("%s ReadAvailableByte buf got %q, want %q", mode, got, want)
	}
}

func expectPeekAvailableByte(t *testing.T, mode string, b *decbuf, expect byte, expectErr error) {
	var err error
	if !b.IsAvailable(1) {
		err = b.Fill(1)
	}
	if got, want := err, expectErr; got != want {
		t.Errorf("%s PeekAvailableByte err got %v, want %v", mode, got, want)
	}
	if err != nil {
		return
	}
	if got, want := b.PeekAvailableByte(), expect; got != want {
		t.Errorf("%s PeekAvailableByte buf got %q, want %q", mode, got, want)
	}
}

func expectReadIntoBuf(t *testing.T, mode string, b *decbuf, n int, expect string, expectErr error) {
	buf := make([]byte, n)
	err := b.ReadIntoBuf(buf)
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadIntoBuf err got %v, want %v", mode, got, want)
	}
	if err == nil {
		if got, want := string(buf), expect; got != want {
			t.Errorf("%s ReadIntoBuf buf got %q, want %q", mode, got, want)
		}
	}
}

func TestDecbufReadAvailable(t *testing.T) {
	fn := func(mode string, b *decbuf) {
		expectReadAvailable(t, mode, b, 1, "a", nil)
		expectReadAvailable(t, mode, b, 2, "bc", nil)
		expectReadAvailable(t, mode, b, 3, "def", nil)
		expectReadAvailable(t, mode, b, 4, "ghij", nil)
		expectReadAvailable(t, mode, b, 1, "", io.EOF)
		expectReadAvailable(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range AllReadModes {
		fn(mode.String(), newDecbuf(mode.TestReader(ABCReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(ABCBytes(10)))
}

func TestDecbufSkip(t *testing.T) {
	fn := func(mode string, b *decbuf) {
		expectSkip(t, mode, b, 1, nil)
		expectReadAvailable(t, mode, b, 2, "bc", nil)
		expectSkip(t, mode, b, 3, nil)
		expectReadAvailable(t, mode, b, 2, "gh", nil)
		expectSkip(t, mode, b, 2, nil)
		expectSkip(t, mode, b, 1, io.EOF)
		expectSkip(t, mode, b, 1, io.EOF)
		expectReadAvailable(t, mode, b, 1, "", io.EOF)
		expectReadAvailable(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range AllReadModes {
		fn(mode.String(), newDecbuf(mode.TestReader(ABCReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(ABCBytes(10)))
}

func TestDecbufPeekAvailable(t *testing.T) {
	fn := func(mode string, b *decbuf) {
		expectPeekAvailable(t, mode, b, 1, "a", nil)
		expectPeekAvailable(t, mode, b, 2, "ab", nil)
		expectPeekAvailable(t, mode, b, 3, "abc", nil)
		expectPeekAvailable(t, mode, b, 4, "", io.EOF)
		expectPeekAvailable(t, mode, b, 1, "a", nil)
		expectPeekAvailable(t, mode, b, 2, "ab", nil)
		expectPeekAvailable(t, mode, b, 3, "abc", nil)
		expectPeekAvailable(t, mode, b, 4, "", io.EOF)

		expectReadAvailable(t, mode, b, 1, "a", nil)
		expectPeekAvailable(t, mode, b, 1, "b", nil)
		expectPeekAvailable(t, mode, b, 2, "bc", nil)
		expectPeekAvailable(t, mode, b, 3, "", io.EOF)
		expectPeekAvailable(t, mode, b, 1, "b", nil)
		expectPeekAvailable(t, mode, b, 2, "bc", nil)
		expectPeekAvailable(t, mode, b, 3, "", io.EOF)

		expectReadAvailable(t, mode, b, 1, "b", nil)
		expectPeekAvailable(t, mode, b, 1, "c", nil)
		expectPeekAvailable(t, mode, b, 2, "", io.EOF)
		expectPeekAvailable(t, mode, b, 1, "c", nil)
		expectPeekAvailable(t, mode, b, 2, "", io.EOF)

		expectReadAvailable(t, mode, b, 1, "c", nil)
		expectPeekAvailable(t, mode, b, 1, "", io.EOF)
		expectPeekAvailable(t, mode, b, 1, "", io.EOF)

		expectReadAvailable(t, mode, b, 1, "", io.EOF)
		expectPeekAvailable(t, mode, b, 1, "", io.EOF)
		expectPeekAvailable(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range AllReadModes {
		fn(mode.String(), newDecbuf(mode.TestReader(ABCReader(3))))
	}
	fn("fromBytes", newDecbufFromBytes(ABCBytes(3)))
}

func TestDecbufReadAvailableByte(t *testing.T) {
	fn := func(mode string, b *decbuf) {
		expectReadAvailableByte(t, mode, b, 'a', nil)
		expectReadAvailableByte(t, mode, b, 'b', nil)
		expectReadAvailableByte(t, mode, b, 'c', nil)
		expectReadAvailableByte(t, mode, b, 0, io.EOF)
		expectReadAvailableByte(t, mode, b, 0, io.EOF)
	}
	for _, mode := range AllReadModes {
		fn(mode.String(), newDecbuf(mode.TestReader(ABCReader(3))))
	}
	fn("fromBytes", newDecbufFromBytes(ABCBytes(3)))
}

func TestDecbufPeekAvailableByte(t *testing.T) {
	fn := func(mode string, b *decbuf) {
		expectPeekAvailableByte(t, mode, b, 'a', nil)
		expectPeekAvailableByte(t, mode, b, 'a', nil)
		expectReadAvailableByte(t, mode, b, 'a', nil)

		expectPeekAvailableByte(t, mode, b, 'b', nil)
		expectPeekAvailableByte(t, mode, b, 'b', nil)
		expectReadAvailableByte(t, mode, b, 'b', nil)

		expectPeekAvailableByte(t, mode, b, 'c', nil)
		expectPeekAvailableByte(t, mode, b, 'c', nil)
		expectReadAvailableByte(t, mode, b, 'c', nil)

		expectPeekAvailableByte(t, mode, b, 0, io.EOF)
		expectPeekAvailableByte(t, mode, b, 0, io.EOF)
		expectReadAvailableByte(t, mode, b, 0, io.EOF)
	}
	for _, mode := range AllReadModes {
		fn(mode.String(), newDecbuf(mode.TestReader(ABCReader(3))))
	}
	fn("fromBytes", newDecbufFromBytes(ABCBytes(3)))
}

func TestDecbufReadIntoBuf(t *testing.T) {
	fn1 := func(mode string, b *decbuf) {
		// Start ReadFull from beginning.
		expectReadIntoBuf(t, mode, b, 3, "abc", nil)
		expectReadIntoBuf(t, mode, b, 3, "def", nil)
		expectReadIntoBuf(t, mode, b, 1, "", io.EOF)
		expectReadIntoBuf(t, mode, b, 1, "", io.EOF)
	}
	fn2 := func(mode string, b *decbuf) {
		// Start ReadFull after reading 1 byte, which fills the buffer.
		expectReadAvailable(t, mode, b, 1, "a", nil)
		expectReadIntoBuf(t, mode, b, 2, "bc", nil)
		expectReadIntoBuf(t, mode, b, 3, "def", nil)
		expectReadIntoBuf(t, mode, b, 1, "", io.EOF)
		expectReadIntoBuf(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range AllReadModes {
		fn1(mode.String(), newDecbuf(mode.TestReader(ABCReader(6))))
		fn2(mode.String(), newDecbuf(mode.TestReader(ABCReader(6))))
	}
	fn1("fromBytes", newDecbufFromBytes(ABCBytes(6)))
	fn2("fromBytes", newDecbufFromBytes(ABCBytes(6)))
}
