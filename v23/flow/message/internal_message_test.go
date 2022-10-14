// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/rpc/version"
)

func randomTestCases() []uint64 {
	c := make([]uint64, 4096)
	for i := range c {
		c[i] = rand.Uint64()
	}
	return c
}

func randomMaxTestCases(limit int64) []uint64 {
	c := make([]uint64, 4096)
	for i := range c {
		c[i] = uint64(rand.Int63n(limit))
	}
	return c
}

func randomLargeTestCases() []uint64 {
	c := make([]uint64, 4096)
	for i := range c {
		for c[i] < math.MaxUint32 {
			c[i] = rand.Uint64()
		}
	}
	return c
}

var (
	varintBoundaryCases = []uint64{
		0x00, 0x01,
		0x7f, 0x80,
		0xff, 0x100,
		0xffff, 0x10000,
		0xffffff, 0x1000000,
		0xffffffff, 0x100000000,
		0xffffffffff, 0x10000000000,
		0xffffffffffff, 0x1000000000000,
		0xffffffffffffff, 0x100000000000000,
		0xffffffffffffffff,
	}

	varintRandomCases       = randomTestCases()
	varintSmallRandomCases  = randomMaxTestCases(math.MaxUint16)
	varintMediumRandomCases = randomMaxTestCases(math.MaxUint32)
	varintLargeRandomCases  = randomLargeTestCases()
)

func testVarInt(t *testing.T, cases []uint64) {
	for _, want := range cases {
		got, b, valid := readVarUint64(writeVarUint64(want, []byte{}))
		if !valid {
			t.Fatalf("error reading %x", want)
		}
		if len(b) != 0 {
			t.Errorf("unexpected buffer remaining for %x: %v", want, b)
		}
		if got != want {
			t.Errorf("got: %d want: %d", got, want)
		}
	}
}

func TestVarInt(t *testing.T) {
	testVarInt(t, varintBoundaryCases)
	testVarInt(t, varintRandomCases)
}

func TestVarIntBackwardsCompat(t *testing.T) {
	for _, tc := range append(varintBoundaryCases, varintRandomCases...) {
		prevWrite := writeVarUint64Orig(tc, nil)
		newWrite := writeVarUint64(tc, make([]byte, 0, 10))
		if got, want := newWrite, prevWrite; !bytes.Equal(got, want) {
			t.Errorf("%02x: got: %d want: %d", tc, got, want)
		}
		prevRead, prevBuf, prevOk := readVarUint64Orig(prevWrite)
		newRead, newBuf, newOk := readVarUint64(prevWrite)
		if got, want := newRead, prevRead; got != want {
			t.Errorf("%02x: got %v, want %v (%02x != %02x)", tc, got, want, got, want)
		}
		if got, want := newBuf, prevBuf; !bytes.Equal(got, want) {
			t.Errorf("%02x: got %v, want %v", tc, got, want)
		}
		if got, want := newOk, prevOk; got != want {
			t.Errorf("%02x: got %v, want %v", tc, got, want)
		}
	}
}

func TestUninterpretedSetupOpts(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	ma := Setup{Versions: version.RPCVersionRange{Min: 3, Max: 5},
		PeerNaClPublicKey: &[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
			14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
	}
	mb := Setup{Versions: version.RPCVersionRange{Min: 3, Max: 5},
		PeerNaClPublicKey: &[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
			14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
		uninterpretedOptions: []option{{opt: 7777, payload: []byte("wat")}},
	}
	encma, err := ma.Append(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	encmb, err := mb.Append(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encma, encmb) {
		t.Errorf("uninterpreted opts were not encoded")
	}
	mc, err := Read(ctx, encmb)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mc, mb) {
		t.Errorf("uninterpreted options were no decoded correctly, got %v, want %v", mc, mb)
	}
}

func TestErrInvalidMessage(t *testing.T) {
	serr := fmt.Errorf("my error")
	err := NewErrInvalidMsg(nil, 0x11, 33, 64, serr)
	if got, want := errors.Unwrap(err).Error(), serr.Error(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	_, _, _, ok := ParseErrInvalidMessage(serr)
	if got, want := ok, false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	typ, size, field, ok := ParseErrInvalidMessage(err)
	if got, want := ok, true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := typ, byte(0x11); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := size, uint64(33); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := field, uint64(64); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func ExposeSetAuthMessageType(m Auth, ecdsa, ed25519, rsa bool) Auth {
	switch {
	case ecdsa:
		m.signatureType = authType
	case ed25519:
		m.signatureType = authED25519Type
	case rsa:
		m.signatureType = authRSAType
	}
	return m
}

func Benchmark__VarIntPrev_______All(b *testing.B) {
	benchmarkVarIntPrev(b, varintRandomCases)
}

func Benchmark__VarIntPrev_____Small(b *testing.B) {
	benchmarkVarIntPrev(b, varintSmallRandomCases)
}

func Benchmark__VarIntPrev____Medium(b *testing.B) {
	benchmarkVarIntPrev(b, varintMediumRandomCases)
}

func Benchmark__VarIntPrev_____Large(b *testing.B) {
	benchmarkVarIntPrev(b, varintLargeRandomCases)
}

func Benchmark__VarIntNew________All(b *testing.B) {
	benchmarkVarIntNew(b, varintRandomCases)
}

func Benchmark__VarIntNew______Small(b *testing.B) {
	benchmarkVarIntNew(b, varintSmallRandomCases)
}

func Benchmark__VarIntNew_____Medium(b *testing.B) {
	benchmarkVarIntNew(b, varintMediumRandomCases)
}

func Benchmark__VarIntNew______Large(b *testing.B) {
	benchmarkVarIntNew(b, varintLargeRandomCases)
}

func Benchmark__VarIntStdlib_____All(b *testing.B) {
	benchmarkVarIntStdlib(b, varintRandomCases)
}

func Benchmark__VarIntStdlib___Small(b *testing.B) {
	benchmarkVarIntStdlib(b, varintSmallRandomCases)
}

func Benchmark__VarIntStdlib__Medium(b *testing.B) {
	benchmarkVarIntStdlib(b, varintMediumRandomCases)
}

func Benchmark__VarIntStdlib___Large(b *testing.B) {
	benchmarkVarIntStdlib(b, varintLargeRandomCases)
}

func benchmarkVarIntPrev(b *testing.B, varintCases []uint64) {
	b.ReportAllocs()
	b.ResetTimer()
	buf := make([]byte, 0, 10)
	for i := 0; i < b.N; i++ {
		want := varintCases[i%len(varintCases)]
		_, _, valid := readVarUint64Orig(writeVarUint64Orig(want, buf))
		if !valid {
			b.Fatalf("error reading %x", want)
		}
	}
}

func benchmarkVarIntNew(b *testing.B, varintCases []uint64) {
	b.ReportAllocs()
	b.ResetTimer()
	buf := make([]byte, 0, 10)
	for i := 0; i < b.N; i++ {
		want := varintCases[i%len(varintCases)]
		_, _, valid := readVarUint64(writeVarUint64(want, buf))
		if !valid {
			b.Fatalf("error reading %x", want)
		}
	}
}

func benchmarkVarIntStdlib(b *testing.B, varintCases []uint64) {
	b.ReportAllocs()
	b.ResetTimer()
	buf := make([]byte, 10)
	for i := 0; i < b.N; i++ {
		want := varintCases[i%len(varintCases)]
		n := binary.PutUvarint(buf, want)
		got, _ := binary.Uvarint(buf[:n])
		if got != want {
			b.Fatalf("got %02x, want %02x", got, want)
		}
	}
}

func writeVarUint64Orig(u uint64, buf []byte) []byte {
	if u <= 0x7f {
		return append(buf, byte(u))
	}
	shift, l := 56, byte(7)
	for ; shift >= 0 && (u>>uint(shift))&0xff == 0; shift, l = shift-8, l-1 {
	}
	buf = append(buf, 0xff-l)
	for ; shift >= 0; shift -= 8 {
		buf = append(buf, byte(u>>uint(shift))&0xff)
	}
	return buf
}

func readVarUint64Orig(data []byte) (uint64, []byte, bool) {
	if len(data) == 0 {
		return 0, data, false
	}
	l := data[0]
	if l <= 0x7f {
		return uint64(l), data[1:], true
	}
	l = 0xff - l + 1
	if l > 8 || len(data)-1 < int(l) {
		return 0, data, false
	}
	var out uint64
	for i := 1; i < int(l+1); i++ {
		out = out<<8 | uint64(data[i])
	}
	return out, data[l+1:], true
}
