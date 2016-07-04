// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !mojo

// Package syncbaselib defines a Main function that takes syncbased flags as
// arguments, for use in syncbased/v23_main.go as well as gosh. It also defines
// a Serve function, called from Main as well as syncbased/mojo_main.go.
package syncbaselib

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/x/ref/lib/signals"
)

func Main(opts Opts) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	MainWithCtx(ctx, opts)
}

func MainWithCtx(ctx *context.T, opts Opts) {
	_, _, cleanup := Serve(ctx, opts)
	defer cleanup()
	ctx.Info("Received signal ", <-signals.ShutdownOnSignals(ctx))
}
