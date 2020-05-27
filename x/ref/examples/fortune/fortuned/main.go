// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command fortuned runs a daemon that implements the Fortune interface.
package main

import (
	"flag"
	"log"

	v23 "v.io/v23"
	"v.io/v23/security"
	"v.io/x/ref/examples/fortune"
	"v.io/x/ref/examples/fortune/fortuned/internal"
	"v.io/x/ref/lib/signals"

	// The v23.Init call below will use the roaming runtime factory.
	_ "v.io/x/ref/runtime/factories/roaming"
)

var (
	name = flag.String("name", "", "Name for fortuned in default mount table")
)

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()

	authorizer := security.DefaultAuthorizer()
	impl := internal.NewImpl()
	service := fortune.FortuneServer(impl)

	ctx, server, err := v23.WithNewServer(ctx, *name, service, authorizer)
	if err != nil {
		log.Panic("Failure creating server: ", err)
	}
	log.Printf("Listening at: %v\n", server.Status().Endpoints[0])

	<-signals.ShutdownOnSignals(ctx)
}
