// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipher_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"runtime"
	"testing"

	"golang.org/x/crypto/nacl/box"
	"v.io/x/ref/runtime/internal/flow/cipher/aead"
	"v.io/x/ref/runtime/internal/flow/cipher/naclbox"
)

// cipherAPI provides an API for testing ciphers used for encrypting
// messages sent by message pipe implementations. This interface is used
// solely for testing and is intended to allow for a single set of tests to
// be used for multiple cipher implementations in the future.
type cipherAPI interface {
	// Seal returns an authenticated and encrypted version of the supplied
	// data. Seal appends the encrypted data to the supplied buf and the
	// the returned slice points to the newly encrypted data that can be used
	// as the argument to Open (it includes he MAC).
	Seal(buf, data []byte) ([]byte, error)

	// Open authenticates and decrypts a box produced by Seal and returns true
	// on success. The decrypted data is appended to buf and the returned slice
	// points to the newly decrypted data.
	Open(buf, data []byte) ([]byte, bool)

	// ChannelBinding Returns a byte slice that is unique for the the
	// particular cipher (and the parties between which it is operating).
	// Having both parties assert out of the band that they are indeed
	// participating in a connection with that channel binding value is
	// sufficient to authenticate the data received through the cipher.
	ChannelBinding() []byte

	// Overhead is the max difference between the plaintext and ciphertext sizes.
	Overhead() int
}

// Add space for a MAC.
func newMessage(c cipherAPI, s string) (buf, msg []byte) {
	return make([]byte, 0, len(s)+c.Overhead()), []byte(s)
}

func newCipherRPC11() (c1, c2 cipherAPI, err error) {
	pk1, sk1, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate key")
	}
	pk2, sk2, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate key")
	}
	c1, err = naclbox.NewCipherRPC11(pk1, sk1, pk2)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	c2, err = naclbox.NewCipherRPC11(pk2, sk2, pk1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	return
}

func newCipherRPC15() (c1, c2 cipherAPI, err error) {
	pk1, sk1, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate key")
	}
	pk2, sk2, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate key")
	}
	c1, err = aead.NewCipherRPC15(pk1, sk1, pk2)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	c2, err = aead.NewCipherRPC15(pk2, sk2, pk1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	return
}

func testCipherOpenSeal(t *testing.T, c1, c2 cipherAPI) {
	buf1, msg1 := newMessage(c1, "hello")
	ret1, err := c1.Seal(buf1, msg1)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	buf2, msg2 := newMessage(c1, "world")
	ret2, err := c1.Seal(buf2, msg2)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	buf3, msg3 := newMessage(c1, "hello")
	ret3, err := c1.Seal(buf3, msg3)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	differ := func(a, b []byte) {
		if bytes.Equal(a, b) {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line %v: message should differ: %v, %v", line, a, b)
		}
	}
	differ(ret1, msg1)
	differ(ret2, msg2)
	differ(ret3, msg3)
	differ(ret1, ret2)
	differ(ret1, ret3)

	// Check that the client does not encrypt the same.
	buf4, msg4 := newMessage(c1, "hello")
	ret4, err := c2.Seal(buf4, msg4)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if bytes.Equal(ret4, ret1) {
		t.Errorf("messages should differ %q vs. %q", msg4, msg1)
	}

	// Corrupted message should not decrypt.
	msg1[0] ^= 1
	if _, ok := c2.Open(buf1, msg1); ok {
		t.Errorf("expected error")
	}

	// Fix the message and try again.
	var buf [100]byte
	msg1[0] ^= 1
	rmsg1, ok := c2.Open(buf[:0], ret1)
	if !ok {
		t.Errorf("Open failed")
	}
	if !bytes.Equal(rmsg1, []byte("hello")) {
		t.Errorf("got %q, expected %q", rmsg1, "hello")
	}

	// msg3 should not decrypt.
	if _, ok := c2.Open(buf[:0], ret3); ok {
		t.Errorf("expected error")
	}

	// Resume.
	rmsg2, ok := c2.Open(buf[:0], ret2)
	if !ok {
		t.Errorf("Open failed")
	}
	if !bytes.Equal(rmsg2, []byte("world")) {
		t.Errorf("got %q, expected %q", rmsg2, "world")
	}
	rmsg3, ok := c2.Open(buf[:0], ret3)
	if !ok {
		t.Errorf("Open failed")
	}
	if !bytes.Equal(rmsg3, []byte("hello")) {
		t.Errorf("got %q, expected %q", rmsg3, "hello")
	}
}

func testCipherOpenSealRand(t *testing.T, c1, c2 cipherAPI, size int) {
	msg := make([]byte, size)
	io.ReadFull(rand.Reader, msg)
	encBuf := make([]byte, 0, size+c1.Overhead())
	decBuf := make([]byte, 0, size+c1.Overhead())
	enc, err := c1.Seal(encBuf, msg)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if bytes.Contains(enc, msg) {
		t.Fatalf("data not encrypted")
	}
	dec, ok := c2.Open(decBuf, enc)
	if !ok {
		t.Errorf("decrytion failed")
	}
	if got, want := dec, msg; !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCipherOpenSealRPC11(t *testing.T) {
	c1, c2, err := newCipherRPC11()
	if err != nil {
		t.Fatal(err)
	}
	testCipherOpenSeal(t, c1, c2)
	testCipherOpenSealRand(t, c1, c2, 1024)
}

func TestCipherOpenSealRPC15(t *testing.T) {
	c1, c2, err := newCipherRPC15()
	if err != nil {
		t.Fatal(err)
	}
	testCipherOpenSeal(t, c1, c2)
	testCipherOpenSealRand(t, c1, c2, 1024)
}

func TestCipherChannelBinding(t *testing.T) {
	values := make([][]byte, 100)
	for i := 0; i < len(values); i++ {
		c1, c2, err := newCipherRPC11()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(c1.ChannelBinding(), c2.ChannelBinding()) {
			t.Fatalf("Two ends of the crypter ended up with different channel bindings (iteration #%d)", i)
		}
		values[i] = c1.ChannelBinding()
	}
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if bytes.Equal(values[i], values[j]) {
				t.Fatalf("Same ChannelBinding seen on multiple channels (%d and %d)", i, j)
			}
		}
	}
}

func benchmarkRPC11(b *testing.B, size int) {
	c1, c2, err := newCipherRPC11()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkCipher(b, c1, c2, size)
}

func benchmarkRPC15(b *testing.B, size int) {
	c1, c2, err := newCipherRPC15()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkCipher(b, c1, c2, size)
}

func benchmarkCipher(b *testing.B, c1, c2 cipherAPI, size int) {
	msg := make([]byte, size)
	io.ReadFull(rand.Reader, msg)
	encbuf := make([]byte, 0, size+c1.Overhead())
	decbuf := make([]byte, 0, size+c1.Overhead())
	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc, err := c1.Seal(encbuf, msg)
		if err != nil {
			b.Errorf("unexpected error: %s", err)
		}
		_, ok := c2.Open(decbuf, enc)
		if !ok {
			b.Errorf("unexpected error")
		}
	}
}

func Benchmark_RPC11____1KB(b *testing.B) {
	benchmarkRPC11(b, 1000)
}

func Benchmark_RPC11___10KB(b *testing.B) {
	benchmarkRPC11(b, 10000)
}

func Benchmark_RPC11___1MBB(b *testing.B) {
	benchmarkRPC11(b, 1000000)
}

func Benchmark_RPC15____1KB(b *testing.B) {
	benchmarkRPC15(b, 1000)
}

func Benchmark_RPC15___10KB(b *testing.B) {
	benchmarkRPC15(b, 10000)
}

func Benchmark_RPC15___1MBB(b *testing.B) {
	benchmarkRPC15(b, 1000000)
}
