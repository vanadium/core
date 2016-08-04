// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lru

import "testing"

func TestBasic(t *testing.T) {
	c := New(3)
	// Put (A, 1), (B, 2), (A, 3), (B, 4)
	// (A, 1) should be evicted when (B, 4) is inserted.
	if ek, ev, e := c.Put("A", 1); e || ek != "" || ev != nil {
		t.Errorf("Unexpected eviction (%q, %v)", ek, ev)
	}
	if ek, ev, e := c.Put("B", 2); e || ek != "" || ev != nil {
		t.Errorf("Unexpected eviction (%q, %v)", ek, ev)
	}
	if ek, ev, e := c.Put("A", 3); e || ek != "" || ev != nil {
		t.Errorf("Unexpected eviction (%q, %v)", ek, ev)
	}
	if ek, ev, e := c.Put("B", 4); !e || ek != "A" || ev.(int) != 1 {
		t.Errorf("Got eviction (%q, %v, %v), want (%q, %v, %v)", ek, ev, e, "A", 1, true)
	}

	// Get() removes from the cache
	if v, ok := c.Get("A"); !ok || v.(int) != 3 {
		t.Errorf("Got (%v, %v) want (3, true)", v, ok)
	}
	if v, ok := c.Get("A"); ok || v != nil {
		t.Errorf("Got (%v, %v) want (nil, false)", v, ok)
	}
	if v, ok := c.Get("B"); !ok || v.(int) != 4 {
		t.Errorf("Got (%v, %v) want (4, true)", v, ok)
	}
	if v, ok := c.Get("B"); !ok || v.(int) != 2 {
		t.Errorf("Got (%v, %v) want (2, true)", v, ok)
	}
	if v, ok := c.Get("B"); ok || v != nil {
		t.Errorf("Got (%v, %v) want (nil, false)", v, ok)
	}
}
