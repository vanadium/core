// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common_test

import (
	"reflect"
	"testing"

	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/services/syncbase/common"
)

var keyPartTests = []struct {
	parts []string
	key   string
}{
	{[]string{"a", "b"}, "a\xfeb"},
	{[]string{"aa", "bb"}, "aa\xfebb"},
	{[]string{"a", "b", "c"}, "a\xfeb\xfec"},
}

func TestJoinKeyParts(t *testing.T) {
	for _, test := range keyPartTests {
		got, want := common.JoinKeyParts(test.parts...), test.key
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %q, want %q", test.parts, got, want)
		}
	}
}

func TestSplitKeyParts(t *testing.T) {
	for _, test := range keyPartTests {
		got, want := common.SplitKeyParts(test.key), test.parts
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.key, got, want)
		}
	}
}

func TestSplitNKeyParts(t *testing.T) {
	for _, test := range keyPartTests {
		got, want := common.SplitNKeyParts(test.key, 1), []string{test.key}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.key, got, want)
		}
	}
	for _, test := range keyPartTests {
		// Note, all test cases in keyPartTests have <= 3 parts.
		got, want := common.SplitNKeyParts(test.key, 3), test.parts
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.key, got, want)
		}
	}
}

func TestStripFirstKeyPartOrDie(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"a\xfe", ""},
		{"a\xfeb", "b"},
		{"a\xfe\xfe", "\xfe"},
		{"a\xfeb\xfe", "b\xfe"},
		{"a\xfeb\xfec", "b\xfec"},
	}
	for _, test := range tests {
		got, want := common.StripFirstKeyPartOrDie(test.in), test.out
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.in, got, want)
		}
	}
}

func TestFirstKeyPart(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"", ""},
		{"a", "a"},
		{"a\xfe", "a"},
		{"a\xfeb", "a"},
		{"\xfe", ""},
		{"\xfeb", ""},
	}
	for _, test := range tests {
		got, want := common.FirstKeyPart(test.in), test.out
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.in, got, want)
		}
	}
}

func TestIsRowKey(t *testing.T) {
	tests := []struct {
		in  string
		out bool
	}{
		{"", false},
		{"a", false},
		{"a\xfe", false},
		{"a\xfeb", false},
		{common.RowPrefix, true},
		{common.RowPrefix + "\xfe", true},
		{common.RowPrefix + "\xfeb", true},
		{common.CollectionPermsPrefix, false},
		{common.CollectionPermsPrefix + "\xfe", false},
		{common.CollectionPermsPrefix + "\xfeb", false},
	}
	for _, test := range tests {
		got, want := common.IsRowKey(test.in), test.out
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.in, got, want)
		}
	}
}

func TestParseRowKey(t *testing.T) {
	tests := []struct {
		key        string
		collection wire.Id
		row        string
		err        bool
	}{
		{common.RowPrefix + "\xfeu,c\xferow", wire.Id{"u", "c"}, "row", false},
		{common.RowPrefix + "\xfeu,c\xfe", wire.Id{"u", "c"}, "", false},
		{common.RowPrefix + "\xfe,c\xfe", wire.Id{"", "c"}, "", false},
		{common.RowPrefix + "\xfeu,\xfe", wire.Id{"u", ""}, "", false},
		{common.RowPrefix + "\xfe,\xferow", wire.Id{"", ""}, "row", false},
		{common.RowPrefix + "\xfe\xferow", wire.Id{}, "", true},
		{common.RowPrefix + "\xfeuc\xfe", wire.Id{}, "", true},
		{common.CollectionPermsPrefix + "\xfeu,c\xferow", wire.Id{}, "", true},
		{common.CollectionPermsPrefix + "\xfeu,c\xfe", wire.Id{}, "", true},
		{common.CollectionPermsPrefix + "\xfe\xferow", wire.Id{}, "", true},
		{common.CollectionPermsPrefix + "\xfe\xfe", wire.Id{}, "", true},
		{"pfx\xfeu,c\xferow", wire.Id{}, "", true},
		{"pfx\xfeu,c\xfe", wire.Id{}, "", true},
		{"pfx\xfe\xferow", wire.Id{}, "", true},
		{"pfx\xfe\xfe", wire.Id{}, "", true},
		{"\xfeu,c\xferow", wire.Id{}, "", true},
		{"\xfeu,c\xfe", wire.Id{}, "", true},
		{"\xfe\xferow", wire.Id{}, "", true},
		{"\xfe\xfe", wire.Id{}, "", true},
		{common.RowPrefix, wire.Id{}, "", true},
		{common.RowPrefix + "\xfeu,c", wire.Id{}, "", true},
		{common.RowPrefix + "\xfe", wire.Id{}, "", true},
	}
	for _, test := range tests {
		collection, row, err := common.ParseRowKey(test.key)
		if !reflect.DeepEqual(collection, test.collection) {
			t.Errorf("%q: got %v, want %v", test.key, collection, test.collection)
		}
		if !reflect.DeepEqual(row, test.row) {
			t.Errorf("%q: got %v, want %v", test.key, row, test.row)
		}
		if !reflect.DeepEqual(err != nil, test.err) {
			t.Errorf("%q: got %v, want %v", test.key, err != nil, test.err)
		}
	}
}

func TestParseCollectionPermsKey(t *testing.T) {
	tests := []struct {
		key        string
		collection wire.Id
		err        bool
	}{
		{common.CollectionPermsPrefix + "\xfeu,c\xfe", wire.Id{"u", "c"}, false},
		{common.CollectionPermsPrefix + "\xfeu,\xfe", wire.Id{"u", ""}, false},
		{common.CollectionPermsPrefix + "\xfe,c\xfe", wire.Id{"", "c"}, false},
		{common.CollectionPermsPrefix + "\xfe,\xfe", wire.Id{"", ""}, false},
		{"\xfeu,c\xfe", wire.Id{}, true},
		{"pfx\xfeu,c\xfe", wire.Id{}, true},
		{common.RowPrefix + "\xfeu,c\xfe", wire.Id{}, true},
		{common.CollectionPermsPrefix + "\xfe", wire.Id{}, true},
		{common.CollectionPermsPrefix + "\xfe\xfe", wire.Id{}, true},
		{common.CollectionPermsPrefix + "\xfeu,c\xferow", wire.Id{}, true},
		{common.CollectionPermsPrefix + "\xfeu,c", wire.Id{}, true},
		{common.CollectionPermsPrefix + "\xfe", wire.Id{}, true},
	}
	for _, test := range tests {
		collection, err := common.ParseCollectionPermsKey(test.key)
		if !reflect.DeepEqual(collection, test.collection) {
			t.Errorf("%q: got %v, want %v", test.key, collection, test.collection)
		}
		if !reflect.DeepEqual(err != nil, test.err) {
			t.Errorf("%q: got %v, want %v", test.key, err != nil, test.err)
		}
	}
}

func TestScanPrefixArgs(t *testing.T) {
	tests := []struct {
		stKeyPrefix, prefix, wantStart, wantLimit string
	}{
		{"x", "", "x\xfe", "x\xff"},
		{"x", "a", "x\xfea", "x\xfeb"},
		{"x", "a\xfe", "x\xfea\xfe", "x\xfea\xff"},
	}
	for _, test := range tests {
		start, limit := common.ScanPrefixArgs(test.stKeyPrefix, test.prefix)
		gotStart, gotLimit := string(start), string(limit)
		if gotStart != test.wantStart {
			t.Errorf("{%q, %q} start: got %q, want %q", test.stKeyPrefix, test.prefix, gotStart, test.wantStart)
		}
		if gotLimit != test.wantLimit {
			t.Errorf("{%q, %q} limit: got %q, want %q", test.stKeyPrefix, test.prefix, gotLimit, test.wantLimit)
		}
	}
}

func TestScanRangeArgs(t *testing.T) {
	tests := []struct {
		stKeyPrefix, start, limit, wantStart, wantLimit string
	}{
		{"x", "", "", "x\xfe", "x\xff"},   // limit "" means "no limit"
		{"x", "a", "", "x\xfea", "x\xff"}, // limit "" means "no limit"
		{"x", "a", "b", "x\xfea", "x\xfeb"},
		{"x", "a", "a", "x\xfea", "x\xfea"}, // empty range
		{"x", "b", "a", "x\xfeb", "x\xfea"}, // empty range
	}
	for _, test := range tests {
		start, limit := common.ScanRangeArgs(test.stKeyPrefix, test.start, test.limit)
		gotStart, gotLimit := string(start), string(limit)
		if gotStart != test.wantStart {
			t.Errorf("{%q, %q, %q} start: got %q, want %q", test.stKeyPrefix, test.start, test.limit, gotStart, test.wantStart)
		}
		if gotLimit != test.wantLimit {
			t.Errorf("{%q, %q, %q} limit: got %q, want %q", test.stKeyPrefix, test.start, test.limit, gotLimit, test.wantLimit)
		}
	}
}
