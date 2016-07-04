// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"testing"

	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/test/v23test"
)

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
