// Copyright 2017 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// An example using the 'library' factory which is configured via exported
// variables rather than by command line flags.
package main

import (
	v23 "v.io/v23"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/runtime/factories/library"
)

func main() {
	// Ensure that logging goes to stderr and that the listen spec
	// contains the test-proxy setting. These settings must be made
	// before v23.Init() is called to have any effect.
	library.AlsoLogToStderr = true
	flags.SetDefaultProxy("test-proxy")
	ctx, shutdown := v23.Init()
	defer shutdown()
	ctx.Infof("Listen spec %v", v23.GetListenSpec(ctx))
	principal := v23.GetPrincipal(ctx)
	ctx.Infof("Principal %s", principal.PublicKey())
}
