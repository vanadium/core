// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake_test

import (
	"testing"

	v23 "v.io/v23"

	_ "v.io/x/ref/runtime/factories/fake"
)

// Ensure that the fake RuntimeFactory can be used to initialize a fake runtime.
func TestInit(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	if !ctx.Initialized() {
		t.Errorf("Got uninitialized context from Init.")
	}
}
