// Copyright 2018 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command echo is a server for the Echo service.
package main

// To allow anyone to connect use `-v23.permissions.literal='{"Read": {"In": ["..."]}}'`

import (
	"fmt"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/x/ref/examples/echo"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/generic"
)

type echod struct{}

// Echo responsds to an Echo request with the original message, the time of day
// and the pid of the server.
func (e *echod) Echo(ctx *context.T, call rpc.ServerCall, msg string) (response string, err error) {
	response = fmt.Sprintf("%s: %v", time.Now(), msg)
	ctx.Infof("%v: %v", call.RemoteEndpoint(), msg)
	return
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()
	ctx, server, err := v23.WithNewServer(ctx, "echo", echo.EchoServiceServer(&echod{}), access.RuntimeAuthorizer(v23.GetPermissionsSpec(ctx)))
	if err != nil {
		ctx.Fatalf("Failure creating server: %v", err)
	}
	ctx.Infof("Listening at: %q\n", server.Status().Endpoints)
	<-signals.ShutdownOnSignals(ctx)
	ctx.Infof("Done")
}
