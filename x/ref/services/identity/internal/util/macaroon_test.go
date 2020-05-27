// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"crypto/rand"
	"testing"

	"v.io/v23/vom"
	"v.io/x/ref/test/testutil"
)

func TestMacaroon(t *testing.T) {
	principal := testutil.NewPrincipal("correct")
	incorrectPrincipal := testutil.NewPrincipal("incorrect")
	input := randBytes(t)

	m, err := NewMacaroon(principal, input)
	if err != nil {
		t.Errorf("NewMacaroon failed: %v", err)
	}

	// Test incorrect key.
	decoded, err := m.Decode(incorrectPrincipal)
	if err == nil {
		t.Errorf("m.Decode should have failed")
	}
	if decoded != nil {
		t.Errorf("decoded value should be nil when decode fails")
	}

	// Test correct key.
	if decoded, err = m.Decode(principal); err != nil {
		t.Errorf("m.Decode should have succeeded")
	}
	if !bytes.Equal(decoded, input) {
		t.Errorf("decoded value should equal input")
	}
}

func TestBadMacaroon(t *testing.T) {
	p := testutil.NewPrincipal("correct")

	mValid, err := NewMacaroon(p, []byte("Hello"))
	if err != nil {
		t.Fatalf("NewMacaroon failed: %v", err)
	}

	// Modify the data, but keep the Signature.
	raw, _ := b64decode(string(mValid))
	var msg MacaroonMessage
	if err := vom.Decode(raw, &msg); err != nil {
		t.Fatalf("vom.Decode failed: %v", err)
	}
	msg.Data = []byte("World")
	v, err := vom.Encode(msg)
	if err != nil {
		t.Fatalf("vom.Encode failed: %v", err)
	}
	mBadData := Macaroon(b64encode(v))

	// Restore data, but change the Signature.
	msg.Data = []byte("Hello")
	msg.Sig.R[0]++

	if v, err = vom.Encode(msg); err != nil {
		t.Fatalf("vom.Encode failed: %v", err)
	}
	mBadSig := Macaroon(b64encode(v))

	tests := []Macaroon{
		mValid,         // valid
		mValid + "!!!", // invalid base64
		mValid[:5],     // truncated
		mBadData,       // modified data
		mBadSig,        // modified signature
		"",             // zero value
	}

	// Make sure that "valid" is indeed valid!
	if data, err := tests[0].Decode(p); err != nil || data == nil {
		t.Fatalf("Bad test data: Got (%v, %v), want (<non-empty>, nil)", data, err)
	}
	// And all others are not:
	for idx, test := range tests[1:] {
		if _, err := test.Decode(p); err == nil {
			t.Errorf("Should have failed to decode invalid macaroon #%d", idx)
		}
	}
}

func randBytes(t *testing.T) []byte {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("bytes creation failed: %v", err)
	}
	return b
}
