// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

// Package roaming implements a RuntimeFactory suitable for a variety of network
// configurations, including 1-1 NATs, dhcp auto-configuration, and Google
// Compute Engine.
//
// The pubsub.Publisher mechanism is used for communicating networking
// settings to the rpc.Server implementation of the runtime and publishes
// the Settings it expects.
package roaming

import (
	"flag"

	"v.io/x/ref/runtime/factories/library"

	"v.io/v23/flow"

	"v.io/x/ref/lib/flags"
	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp" // Initialize tcp, ws and wsh.
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
)

func init() {
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
	library.Roam = true
	library.CloudVM = true
	library.ConfigureLoggingFromFlags = true
	library.ReservedNameDispatcher = true
	flags.RegisterListenFlags(flag.CommandLine, &library.ListenFlags)
	flags.RegisterRuntimeFlags(flag.CommandLine, &library.RuntimeFlags)
	flags.RegisterPermissionsFlags(flag.CommandLine, &library.PermissionsFlags)
}

/*
func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {
	if err := internal.ParseFlagsAndConfigureGlobalLogger(commonFlags); err != nil {
		return nil, nil, nil, err
	}

	cancelCloud, err := internal.InitCloudVM()
	if err != nil {
		return nil, nil, nil, err
	}

	ac := appcycle.New()
	discoveryFactory, err := dfactory.New(ctx)
	if err != nil {
		ac.Shutdown()
		cancelCloud()
		return nil, nil, nil, err
	}

	lf := commonFlags.ListenFlags()
	listenSpec := rpc.ListenSpec{
		Addrs:          rpc.ListenAddrs(lf.Addrs),
		Proxy:          lf.Proxy,
		AddressChooser: internal.NewAddressChooser(logger.Global()),
	}
	reservedDispatcher := debuglib.NewDispatcher(securityflag.NewAuthorizerOrDie())

	ishutdown := func() {
		ac.Shutdown()
		cancelCloud()
		discoveryFactory.Shutdown()
	}

	// TODO(ashankar): As of April 2016, the only purpose this non-nil
	// publisher was serving was to enable roaming in RPC servers (see
	// runtime/internal/flow/manager/manager.go).  Once
	// https://vanadium-review.googlesource.com/#/c/21954/ has been merged,
	// I will try to remove the use of the publisher from here downstream
	// completely (and enable "roaming" for all servers by default).
	publisher := pubsub.NewPublisher()

	runtime, ctx, shutdown, err := rt.Init(ctx, ac, discoveryFactory, nil, nil, &listenSpec, publisher, commonFlags.RuntimeFlags(), reservedDispatcher, 0)
	if err != nil {
		ishutdown()
		return nil, nil, nil, err
	}

	runtimeFactoryShutdown := func() {
		ishutdown()
		shutdown()
	}
	return runtime, ctx, runtimeFactoryShutdown, nil
}
*/
