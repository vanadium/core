// Copyright 2017 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// An example using the 'library' factory which is configured via exported
// variables rather than by command line flags.
package main

import (
	"fmt"

	v23 "v.io/v23"
	"v.io/x/ref/lib/flags"
	_ "v.io/x/ref/runtime/factories/library"
)

func main() {
	flags.SetDefaultProxy("test-proxy")
	ctx, shutdown := v23.Init()
	defer shutdown()
	fmt.Printf("Listen spec %v\n", v23.GetListenSpec(ctx))
	principal := v23.GetPrincipal(ctx)
	fmt.Printf("Principal %s\n", principal.PublicKey())
}
