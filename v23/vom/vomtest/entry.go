// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vomtest

import (
	"fmt"
	"reflect"

	"v.io/v23/vom"
)

// Entry represents a test entry, which contains a value and hex bytes.  The hex
// bytes represent the golden vom encoding of the value.  Encoding tests encode
// the value and expect to get the hex bytes, while decoding tests decode the
// hex bytes and expect to get the value.
type Entry struct {
	Label      string        // Label describes the entry
	ValueLabel string        // ValueLabel describes the Value
	Value      reflect.Value // Value for vom test
	Version    vom.Version   // Version of encoding
	HexType    string        // Hex bytes representing the type message(s).
	HexValue   string        // Hex bytes representing the value message.
}

// Name returns the name of the entry, which combines the entry and value
// labels.
func (e Entry) Name() string {
	return e.Label + " " + e.ValueLabel
}

// HexVersion returns the version as hex bytes.
func (e Entry) HexVersion() string {
	return fmt.Sprintf("%x", byte(e.Version))
}

// Hex returns the full hex bytes, including the version, types and value.
func (e Entry) Hex() string {
	return e.HexVersion() + e.HexType + e.HexValue
}

// Bytes returns the full binary bytes, including the version, types and value.
func (e Entry) Bytes() []byte {
	return hexToBytes(e.Hex())
}

// TypeBytes returns the binary bytes including the version and types.  Returns
// a nil slice if no type messages are encoded.
func (e Entry) TypeBytes() []byte {
	if e.HexType == "" {
		return nil
	}
	return hexToBytes(e.HexVersion() + e.HexType)
}

// ValueBytes returns the binary bytes including the version and value.
func (e Entry) ValueBytes() []byte {
	return hexToBytes(e.HexVersion() + e.HexValue)
}

func hexToBytes(hex string) []byte {
	var b []byte
	if _, err := fmt.Sscanf(hex, "%x", &b); err != nil {
		panic(fmt.Errorf("hex %s isn't valid: %v", hex, err))
	}
	return b
}
