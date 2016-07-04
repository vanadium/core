// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl_test

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vdl/vdltest"
)

func TestValueReader(t *testing.T) {
	for _, entry := range vdltest.ToEntryValues(vdltest.AllPass()) {
		out := vdl.ZeroValue(entry.Target.Type())
		if err := out.VDLRead(entry.Source.Decoder()); err != nil {
			t.Errorf("%s: VDLRead failed: %v", entry.Name(), err)
			continue
		}
		if got, want := out, entry.Target; !vdl.EqualValue(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", entry.Name(), got, want)
		}
	}
}
