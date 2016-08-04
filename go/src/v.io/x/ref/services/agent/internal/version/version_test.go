// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package version

import "testing"

func TestString(t *testing.T) {
	for i, c := range []struct {
		v        T
		expected string
	}{
		{0, "0"},
		{1, "1"},
	} {
		if got := c.v.String(); got != c.expected {
			t.Errorf("test case #%d: expected \"%s\", got \"%s\"", i, c.expected, got)
		}
	}
}

func TestSet(t *testing.T) {
	for i, c := range []struct {
		s        string
		expected T
		fail     bool
	}{
		{"this is not the version you're looking for", 0, true},
		{"-1", 0, true},
		{"0", 0, false},
		{"1", 1, false},
	} {
		var v T
		if err := v.Set(c.s); v != c.expected || (err != nil) != c.fail {
			t.Errorf("test case #%d: expected (%v, fail: %t), got (%v, err: %v)", i, c.expected, c.fail, v, err)
		}
	}
}

func TestCommon(t *testing.T) {
	for i, c := range []struct {
		lMin, lMax, rMin, rMax, common T
		fail                           bool
	}{
		{1, 0, 1, 1, 0, true},
		{1, 1, 1, 0, 0, true},
		{1, 2, 3, 4, 0, true},
		{1, 1, 1, 1, 1, false},
		{1, 2, 1, 1, 1, false},
		{1, 2, 1, 2, 2, false},
	} {
		common, err := Common(nil, Range{c.lMin, c.lMax}, Range{c.rMin, c.rMax})
		if (err != nil) != c.fail {
			t.Errorf("test case #%d: err: %v, expected fail: %t", i, err, c.fail)
		}
		if common != c.common {
			t.Errorf("test case #%d: got %v, expected %v", i, common, c.common)
		}
	}
}

func TestContains(t *testing.T) {
	for i, c := range []struct {
		rMin, rMax, v T
		expect        bool
	}{
		{0, 0, 1, false},
		{1, 0, 1, false},
		{2, 1, 1, false},
		{0, 1, 1, true},
		{1, 1, 1, true},
		{1, 2, 1, true},
		{1, 2, 2, true},
	} {
		r := Range{c.rMin, c.rMax}
		if got := r.Contains(c.v); got != c.expect {
			t.Errorf("test case #%d: expected %t, got %t", i, c.expect, got)
		}
	}
}

func TestRangeFromString(t *testing.T) {
	for i, c := range []struct {
		min, max   string
		vMin, vMax T
		fail       bool
	}{
		{"1", "one little duck", 0, 0, true},
		{"one little duck", "1", 0, 0, true},
		{"1", "1", 1, 1, false},
		{"1", "2", 1, 2, false},
		{"2", "1", 2, 1, false},
	} {
		r, err := RangeFromString(nil, c.min, c.max)
		if (err != nil) != c.fail {
			t.Errorf("test case #%d: err: %v, expected fail: %t", i, err, c.fail)
		}
		if r.Min != c.vMin || r.Max != c.vMax {
			t.Errorf("test case #%d: got {%v, %v}, expected {%v, %v}", i, r.Min, r.Max, c.vMin, c.vMax)
		}
	}
}
