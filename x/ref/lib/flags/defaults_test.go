// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"strings"
	"testing"
)

func TestIsInitialized(t *testing.T) {
	intFlag := new(int)
	protocolFlagA := new(TCPProtocolFlag)
	for _, ifc := range []interface{}{intFlag, protocolFlagA} {
		if got, want := hasNewDefault(ifc), false; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		markAsNewDefault(ifc)
		if got, want := hasNewDefault(ifc), true; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	defer func() {
		e := recover()
		if e == nil || !strings.Contains(e.(string), "not a pointer") {
			t.Fatalf("unexpected or missing error: %v", e)
		}

	}()
	var notAPtr int
	hasNewDefault(notAPtr)
	t.FailNow() // fail if there is no panic.
}
