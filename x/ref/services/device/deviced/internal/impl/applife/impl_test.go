// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package applife_test

import (
	"testing"

	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
)

func TestMain(m *testing.M) {
	utiltest.TestMainImpl(m)
}

func TestSuidHelper(t *testing.T) {
	utiltest.TestSuidHelperImpl(t)
}
