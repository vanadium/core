// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command suidhelper runs the provided command as the specified user identity.
// It should be installed setuid root.
package main

// suidhelper deliberately attempts to be as simple as possible to
// simplify reviewing it for security concerns.

import (
	"flag"
	"fmt"
	"os"

	"v.io/x/ref/services/device/internal/suid"
)

func main() {
	flag.Parse()
	fmt.Fprintln(os.Stderr, os.Args)
	if err := suid.Run(os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "Failed with:", err)
		// TODO(rjkroege): We should really only print the usage message
		// if the error is related to interpreting flags.
		flag.Usage()
	}
}
