// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slang_test

import (
	"fmt"
	"testing"

	"v.io/x/ref/lib/slang"
)

func registerFunctionErr(t *testing.T, fn interface{}) (err error) {
	defer func() {
		e := recover()
		t.Log(e)
	}()
	slang.RegisterFunction(fn, "regtest", "")
	return fmt.Errorf("register function succeeded")
}

func null() {}

func noErr(rt slang.Runtime) {}

func noErr1(rt slang.Runtime) int {
	return -1
}

func noRuntime() error {
	return nil
}

func noRuntime1(a int) error {
	return nil
}
func TestRegisterErrors(t *testing.T) {
	for i, fn := range []interface{}{
		null, noErr, noErr1, noRuntime, noRuntime1,
	} {
		err := registerFunctionErr(t, fn)
		if err != nil {
			t.Errorf("%v: should have returned an error: %v", i, err)
		}
	}

}
