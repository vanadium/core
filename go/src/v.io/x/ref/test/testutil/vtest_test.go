// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil_test

import (
	"testing"

	"v.io/x/ref/test/testutil"
)

func TestCallAndRecover(t *testing.T) {
	tests := []struct {
		f      func()
		expect interface{}
	}{
		{func() {}, nil},
		{func() { panic(nil) }, nil},
		{func() { panic(123) }, 123},
		{func() { panic("abc") }, "abc"},
	}
	for _, test := range tests {
		got := testutil.CallAndRecover(test.f)
		if got != test.expect {
			t.Errorf(`CallAndRecover got "%v", want "%v"`, got, test.expect)
		}
	}
}
