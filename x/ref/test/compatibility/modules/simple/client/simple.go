// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command simple is a client for the Simple service.
package main

import (
	"flag"
	"os"

	v23 "v.io/v23"
	_ "v.io/x/ref/runtime/factories/static"
	"v.io/x/ref/test/compatibility/modules/simple/impl"
)

var (
	nameFlag, msgFlag string
	numCallsFlag      int
)

func init() {
	flag.StringVar(&nameFlag, "name", os.ExpandEnv("users/${USER}/simpled"), "name of the server to connect to")
	flag.StringVar(&msgFlag, "message", "hello", "message to ping the server with")
	flag.IntVar(&numCallsFlag, "num-calls", 2, "number of RPC calls to make to the server")
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()
	if err := impl.RunClient(ctx, nameFlag, msgFlag, numCallsFlag); err != nil {
		panic(err)
	}
}
