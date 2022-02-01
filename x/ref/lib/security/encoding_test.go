// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"net/url"
	"testing"

	"v.io/v23/security"
)

func TestBlessingsBase64(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	tester := newStoreTester(p)

	for _, blessing := range []security.Blessings{
		tester.forAll, tester.forFoo, tester.forBar, tester.def, tester.other,
	} {
		enc, err := EncodeBlessingsBase64(blessing)
		if err != nil {
			t.Errorf("encode: %v: %v", blessing, err)
			continue
		}
		dec, err := DecodeBlessingsBase64(enc)
		if err != nil {
			t.Errorf("decode: %v: %v", blessing, err)
			continue
		}
		if got, want := dec.String(), blessing.String(); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := dec, blessing; !got.Equivalent(want) {
			t.Errorf("got %v not equivalent to want %v", got, want)
		}
		pq, err := url.ParseQuery("blessings=" + enc)
		if err != nil {
			t.Errorf("parseQuery: %v: %v", blessing, err)
			continue
		}
		dec, err = DecodeBlessingsBase64(pq.Get("blessings"))
		if err != nil {
			t.Errorf("decode query: %v: %v", blessing, err)
			continue
		}
		if got, want := dec.String(), blessing.String(); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := dec, blessing; !got.Equivalent(want) {
			t.Errorf("got %v not equivalent to want %v", got, want)
		}
	}

	all, err := security.UnionOfBlessings(tester.forAll, tester.forFoo, tester.forBar, tester.def)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := EncodeBlessingsBase64(all)
	if err != nil {
		t.Fatalf("encode: %v: %v", all, err)
	}
	dec, err := DecodeBlessingsBase64(enc)
	if err != nil {
		t.Fatalf("decode: %v: %v", all, err)

	}
	if got, want := dec, all; !got.Equivalent(want) {
		t.Errorf("got %v not equivalent to want %v", got, want)
	}
}
