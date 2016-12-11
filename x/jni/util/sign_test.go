// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import "testing"

func TestSigns(t *testing.T) {
	tests := []struct {
		input  Sign
		output string
	}{
		{ClassSign("java.lang.Object"), "Ljava/lang/Object;"},
		{ClassSign("java.lang.String"), "Ljava/lang/String;"},
		{ArraySign(ObjectSign), "[Ljava/lang/Object;"},
		{ArraySign(StringSign), "[Ljava/lang/String;"},
		{ArraySign(IntSign), "[I"},
		{FuncSign(nil, IntSign), "()I"},
		{FuncSign([]Sign{}, IntSign), "()I"},
		{FuncSign([]Sign{BoolSign}, VoidSign), "(Z)V"},
		{FuncSign([]Sign{CharSign, ByteSign, ShortSign}, FloatSign), "(CBS)F"},
		{FuncSign([]Sign{ClassSign("io.v.core.vanadium.testing.misc")}, ClassSign("io.v.core.vanadium.ret")), "(Lio/v/core/vanadium/testing/misc;)Lio/v/core/vanadium/ret;"},
		{FuncSign([]Sign{ClassSign("io.v.core.vanadium.testing.misc"), ClassSign("other")}, ClassSign("io.v.core.vanadium.ret")), "(Lio/v/core/vanadium/testing/misc;Lother;)Lio/v/core/vanadium/ret;"},
	}
	for _, test := range tests {
		output := string(test.input)
		if output != test.output {
			t.Errorf("expected %v, got %v", test.output, output)
		}
	}
}
