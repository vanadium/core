// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/lib/signals"

	_ "v.io/x/ref/runtime/factories/static"
)

// This is as a simple server to test configuration of cloud/virtualized
// servers.

type service struct{}

func (service) Ping(ctx *context.T, call rpc.ServerCall, msg string) (string, error) {
	return "ping: " + msg, nil
}

var name string

func init() {
	flag.StringVar(&name, "name", "", "name of server")
}

func main() {
	flag.Parse()
	ctx, shutdown := v23.Init()
	defer shutdown()
	ctx, handler := signals.ShutdownOnSignalsWithCancel(ctx)
	defer handler.WaitForSignal()
	_, srv, err := v23.WithNewServer(ctx, name, service{}, security.AllowEveryone())
	if err != nil {
		fmt.Printf("v23.WithNewServerFailed: %v", err)
		return
	}
	fmt.Printf("NAME=%s\n", srv.Status().Endpoints[0])
	for i, ep := range srv.Status().Endpoints {
		fmt.Printf("%v: %v\n", i, ep)
	}
}
