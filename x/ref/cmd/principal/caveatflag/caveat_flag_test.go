// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package caveatflag_test

import (
	"testing"

	"v.io/v23/security"
	"v.io/x/ref/cmd/principal/caveatflag"
)

func TestCaveatFlag(t *testing.T) {
	flset := func(v string) []security.Caveat {
		fl := &caveatflag.Flag{}
		if err := fl.Set(v); err != nil {
			t.Errorf("flag setting: %q: %v", v, err)
		}
		cavs, err := fl.Compile()
		if err != nil {
			t.Errorf("flag setting: %q: %v", v, err)
		}
		return cavs
	}
	for _, val := range []string{
		`{{0x54,0xa6,0x76,0x39,0x81,0x37,0x18,0x7e,0xcd,0xb2,0x6d,0x2d,0x69,0xba,0x0,0x3},typeobject([]string)}={"method"}`,
		`"v.io/v23/security".MethodCaveat={"method"}`,
	} {
		cavs := flset(val)
		if got, want := len(cavs), 1; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
		mc, _ := security.NewMethodCaveat("method")
		if got, want := cavs[0].String(), mc.String(); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
