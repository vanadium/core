// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"testing"

	wire "v.io/v23/services/syncbase"
)

func TestStKey(t *testing.T) {
	tests := []struct {
		dbId  wire.Id
		stKey string
	}{
		{wire.Id{Blessing: "app1", Name: "db1"}, "i\xfeapp1,db1"},
	}
	for _, test := range tests {
		got, want := dbInfoStKey(test.dbId), test.stKey
		if got != want {
			t.Errorf("wrong stKey: got %q, want %q", got, want)
		}
	}
}
