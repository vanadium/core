// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil_test

import (
	"fmt"
	"testing"

	"v.io/x/ref/test/testutil"
)

func TestCallAndRecover(t *testing.T) {
	tests := []struct {
		f      func()
		expect any
	}{
		{func() {}, nil},
		{func() { panic(123) }, 123},
		{func() { panic("abc") }, "abc"},
	}
	for i, test := range tests {
		got := testutil.CallAndRecover(test.f)
		if got != test.expect {
			t.Errorf(`%v: CallAndRecover got "%v" (%T), want "%v" (%T )`, i, got, got, test.expect, test.expect)
		}
	}
	got := fmt.Sprintf("%v", testutil.CallAndRecover(func() { panic(nil) }))
	if got != "panic called with nil argument" && got != "<nil>" {
		// pre go1.22 returns and go1.22 returns "panic called with nil argument
		t.Errorf("CallAndRecover got %v, want nil", got)
	}
}
