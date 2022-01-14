// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command simpled is a server for the Simple service.
package main

// To allow anyone to connect use `-v23.permissions.literal='{"Read": {"In": ["..."]}}'`

import (
	"flag"
	"fmt"
	"os"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/static"
	"v.io/x/ref/test/compatibility/modules/simple"
)

var nameFlag string

func init() {
	flag.StringVar(&nameFlag, "name", os.ExpandEnv("users/${USER}/simpled"), "name for the server in default mount table")
}

type simpleImpl struct{}

func (s *simpleImpl) Ping(ctx *context.T, call rpc.ServerCall, msg string) (response string, err error) {
	response = fmt.Sprintf("%s: %v\n", time.Now(), msg)
	ctx.Infof("%v: %v", call.RemoteEndpoint(), msg)
	return
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()
	ctx, server, err := v23.WithNewServer(ctx, nameFlag, simple.SimpleServer(&simpleImpl{}), securityflag.NewAuthorizerOrDie(ctx))

	if err != nil {
		ctx.Fatalf("Failure creating server: %v", err)
	}
	ctx.Infof("Listening at: %q\n", naming.JoinAddressName(server.Status().Endpoints[0].Name(), ""))
	<-signals.ShutdownOnSignals(ctx)
	ctx.Infof("Done")
}
