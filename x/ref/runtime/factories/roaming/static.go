// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin && linux && cgo

package roaming

import (
	"v.io/x/ref/runtime/factories/library"
)

func init() {
	library.Roam = false
}
