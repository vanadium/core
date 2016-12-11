// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"reflect"
	"testing"
)

func TestParseQueryAndString(t *testing.T) {
	for _, test := range []struct {
		q    string
		want Query
	}{
		{"", Query{}},
		{"BenchmarkSign", Query{Name: "BenchmarkSign"}},
		{"v.io/v23/security.BenchmarkSign", Query{Name: "v.io/v23/security.BenchmarkSign"}},
		{"os:Linux", Query{OS: "Linux"}},
		{"cpu:AMD64 os:darwin Verify", Query{Name: "Verify", OS: "darwin", CPU: "AMD64"}},
		{"Verify cpu:AMD64 os:darwin", Query{Name: "Verify", OS: "darwin", CPU: "AMD64"}},
		{`security/BenchmarkVerify cpu:"Intel(R) Core(TM) i7-5557U CPU @ 3.10GHz" os:"OS X 10.11.3 (15D21)"`, Query{Name: "security/BenchmarkVerify", OS: "OS X 10.11.3 (15D21)", CPU: "Intel(R) Core(TM) i7-5557U CPU @ 3.10GHz"}},
		{"label:mytest uploader:janeDoe OS:darWin Verify", Query{Name: "Verify", OS: "darWin", Uploader: "janeDoe", Label: "mytest"}},
	} {
		got, err := ParseQuery(test.q)
		if err != nil {
			t.Errorf("Failed to parse query [%v]: %v", test.q, err)
			continue
		}
		if !reflect.DeepEqual(*got, test.want) {
			t.Errorf("[%v] Got %#v, want %#v", test.q, *got, test.want)
			continue
		}
		// Parsing the string representation of the query should yield the same results.
		if got, err = ParseQuery(test.want.String()); err != nil {
			t.Errorf("Failed to parse stringified query [%v]: %v", test.want.String(), err)
			continue
		}
		if !reflect.DeepEqual(*got, test.want) {
			t.Errorf("[%v] Got %#v, want %#v", test.q, *got, test.want)
			continue
		}
	}
}

func TestParseQueryErrors(t *testing.T) {
	for _, test := range []string{
		// Same operator twice: Don't know whether to AND or OR them,
		// so for now they are outlawed.
		"os:linux os:darwin",
		"cpu:foo cpu:bar",
	} {
		got, err := ParseQuery(test)
		if err == nil {
			t.Errorf("[%v]: Unexpectedly succeeded parsing into %#v", test, got)
			continue
		}
	}
}
