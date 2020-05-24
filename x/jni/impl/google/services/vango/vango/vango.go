// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command vango is a companion testing command-line tool for the vango android
// test application. It is meant for internal testing and will be deleted soon.
package main

import (
	"fmt"
	"os"

	v23 "v.io/v23"
	"v.io/x/jni/impl/google/services/vango"
	_ "v.io/x/ref/runtime/factories/roaming"
)

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()
	fmt.Println(vango.AllFunc(ctx, os.Stdout))
}
