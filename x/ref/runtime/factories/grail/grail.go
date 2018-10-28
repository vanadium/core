// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin windows

// Package grail implements a RuntimeFactory suitable for a variety of network
// configurations appropriate for grail.
//
// The pubsub.Publisher mechanism is used for communicating networking
// settings to the rpc.Server implementation of the runtime and publishes
// the Settings it expects.
package grail

import (
	"flag"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/rpc"

	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/pubsub"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/runtime/internal"
	"v.io/x/ref/runtime/internal/lib/appcycle"
	"v.io/x/ref/runtime/internal/rt"
	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp"
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
	"v.io/x/ref/services/debug/debuglib"
)

var commonFlags *flags.Flags
var flagsParsed bool

func init() {
	v23.RegisterRuntimeFactory(Init)
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
	commonFlags = flags.CreateAndRegister(flag.CommandLine, flags.Runtime, flags.Listen)
}

// AddAndParseFlags adds Vanadium-specific flags to the provided FlagSet,
// and then parses them, including environment overrides.
//
// This allows the caller to control where and how these flags are parsed.
// AddAndParse panics if it is called after Vanadium initialization (i.e., v23.Init).
func AddAndParseFlags(fs *flag.FlagSet) error {
	if flagsParsed {
		panic("flags have already been parsed")
	}
	flagsParsed = true
	commonFlags = flags.CreateAndRegister(fs, flags.Runtime, flags.Listen)
	return internal.ParseFlagsAndConfigureGlobalLogger(commonFlags)
}

func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {
	if !flagsParsed {
		if err := internal.ParseFlagsAndConfigureGlobalLogger(commonFlags); err != nil {
			return nil, nil, nil, err
		}
		flagsParsed = true
	}

	cancelCloud, err := internal.InitCloudVM()
	if err != nil {
		return nil, nil, nil, err
	}

	ac := appcycle.New()

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
	}

	// TODO(ashankar): As of April 2016, the only purpose this non-nil
	// publisher was serving was to enable roaming in RPC servers (see
	// runtime/internal/flow/manager/manager.go).  Once
	// https://vanadium-review.googlesource.com/#/c/21954/ has been merged,
	// I will try to remove the use of the publisher from here downstream
	// completely (and enable "roaming" for all servers by default).
	publisher := pubsub.NewPublisher()

	runtime, ctx, shutdown, err := rt.Init(ctx, ac, nil, nil, nil, &listenSpec, publisher, commonFlags.RuntimeFlags(), reservedDispatcher, 0)
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
