// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command simpled is a server for the Simple service.
package main

// To allow anyone to connect use `-v23.permissions.literal='{"Read": {"In": ["..."]}}'`

import (
	"flag"
	"os"

	v23 "v.io/v23"
	_ "v.io/x/ref/runtime/factories/static"
	"v.io/x/ref/test/compatibility/modules/simple/impl"
)

var nameFlag string

func init() {
	flag.StringVar(&nameFlag, "name", os.ExpandEnv("users/${USER}/simpled"), "name for the server in default mount table")
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()
	if err := impl.RunServer(ctx, nameFlag); err != nil {
		panic(err)
	}
}
