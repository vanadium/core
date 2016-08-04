// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import "testing"

func TestAdIdBasic(t *testing.T) {
	ids := make(map[AdId]struct{})
	for x := 0; x < 100; x++ {
		id, err := NewAdId()
		if err != nil {
			t.Fatal(err)
		}
		if !id.IsValid() {
			t.Errorf("got invalid id %q", id)
		}

		if _, ok := ids[id]; ok {
			t.Errorf("NewAdId returned duplicated id %q", id)
		}

		decoded, err := ParseAdId(id.String())
		if err != nil {
			t.Error(err)
		}
		if decoded != id {
			t.Errorf("decoded into %q, but want %q", decoded, id)
		}
	}
}

func TestIdString(t *testing.T) {
	id := AdId{8, 9, 10, 11, 12, 13, 14, 15, 0, 1, 2, 3, 4, 5, 6, 7}
	if got, want := id.String(), "08090a0b0c0d0e0f0001020304050607"; got != want {
		t.Errorf("got %q, but want %q", got, want)
	}
}
