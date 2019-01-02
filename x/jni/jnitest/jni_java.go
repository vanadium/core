// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java

package jni

import (
	_ "v.io/x/jni/internal"
	library "v.io/x/ref/runtime/factories/library"
	_ "v.io/x/ref/runtime/factories/roaming"
)

func init() {
	library.AllowMultipleInitializations = true
}

func main() {}
