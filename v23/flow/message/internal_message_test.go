// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/rpc/version"
)

func TestVarInt(t *testing.T) {
	cases := []uint64{
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
	ctx, cancel := context.RootContext()
	defer cancel()
	for _, want := range cases {
		got, b, valid := readVarUint64(ctx, writeVarUint64(want, []byte{}))
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

func TestUninterpretedSetupOpts(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	ma := &Setup{Versions: version.RPCVersionRange{Min: 3, Max: 5},
		PeerNaClPublicKey: &[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
			14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
	}
	mb := &Setup{Versions: version.RPCVersionRange{Min: 3, Max: 5},
		PeerNaClPublicKey: &[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
			14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
		uninterpretedOptions: []option{{opt: 7777, payload: []byte("wat")}},
	}
	encma, err := Append(ctx, ma, nil)
	if err != nil {
		t.Fatal(err)
	}
	encmb, err := Append(ctx, mb, nil)
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
