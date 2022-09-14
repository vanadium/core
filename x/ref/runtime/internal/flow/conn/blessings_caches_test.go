// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"testing"

	"v.io/v23/security"
)

func TestBlessingsIncomingCache(t *testing.T) {
	var ic inCache

	testHasBlessings := func(bkey uint64, present bool) {
		_, o := ic.hasBlessings(bkey)
		if got, want := o, present; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	testHasDischarges := func(bkey, dkey uint64, present bool) {
		d, dk, o := ic.hasDischarges(bkey)
		if got, want := o, present; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := dk, dkey; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if present && d == nil {
			t.Errorf("should not be nil")
		}
		if !present && d != nil {
			t.Errorf("should be nil")
		}
	}

	testHasBlessings(0, false)
	ic.addBlessings(2, security.Blessings{})
	testHasBlessings(1, false)
	testHasBlessings(2, true)

	if got, want := len(ic.blessings), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	testHasDischarges(0, 0, false)
	ic.addDischarges(2, 1, []security.Discharge{})
	testHasDischarges(1, 2, true)

	ic.addDischarges(3, 2, []security.Discharge{})

	if got, want := len(ic.discharges), 2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBlessingsOutgoingCache(t *testing.T) {
	var oc outCache

	testHasBlessings := func(uid string, bkey uint64, present bool) {
		k, o := oc.hasBlessings(uid)
		if got, want := o, present; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := k, bkey; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	testHasDischarges := func(bkey, dkey uint64, present bool) {
		d, dk, o := oc.hasDischarges(bkey)
		if got, want := o, present; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := dk, dkey; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if present && d == nil {
			t.Errorf("should not be nil")
		}
		if !present && d != nil {
			t.Errorf("should be nil")
		}
	}

	testHasBlessings("x", 0, false)
	oc.addBlessings("yy", 1, security.Blessings{})
	testHasBlessings("yy", 1, true)
	if got, want := len(oc.blessings), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	testHasDischarges(1, 0, false)
	oc.addDischarges(1, 2, []security.Discharge{})
	testHasDischarges(1, 2, true)
	testHasDischarges(3, 0, false)
	if got, want := len(oc.discharges), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
