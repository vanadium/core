// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

// Package library implements a RuntimeFactory suitable for building a Vanadium
// library that is linked into other applications. It is configured via a set
// of exported variables rather than via command line flags.
//
// The pubsub.Publisher mechanism is used for communicating networking
// settings to the rpc.Server implementation of the runtime and publishes
// the Settings it expects.
package library

import (
	"flag"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"

	"v.io/x/lib/vlog"
	"v.io/x/ref/internal/logger"
	dfactory "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/pubsub"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/runtime/internal"
	"v.io/x/ref/runtime/internal/lib/appcycle"
	"v.io/x/ref/runtime/internal/rt"
	_ "v.io/x/ref/runtime/protocols/tcp"
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
	"v.io/x/ref/services/debug/debuglib"
)

var (
	LogToStderr     vlog.LogToStderr
	AlsoLogToStderr vlog.AlsoLogToStderr
	LogDir          vlog.LogDir
	Level           vlog.Level
	StderrThreshold = vlog.StderrThreshold(vlog.ErrorLog)
	ModuleSpec      vlog.ModuleSpec
	FilepathSpec    vlog.FilepathSpec
	TraceLocation   vlog.TraceLocation
	MaxStackBufSize vlog.MaxStackBufSize

	ListenFlags flags.ListenFlags

	RuntimeFlags flags.RuntimeFlags
)

func init() {
	v23.RegisterRuntimeFactory(Init)
}

// AddAndParseFlags adds Vanadium-specific flags to the provided FlagSet, which
// for this factory has no effect.
func AddAndParseFlags(fs *flag.FlagSet) error { return nil }

func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {
	l := logger.Manager(logger.Global()).(*vlog.Logger)
	if err := l.Configure(
		LogToStderr,
		AlsoLogToStderr,
		LogDir,
		Level,
		StderrThreshold,
		ModuleSpec,
		FilepathSpec,
		TraceLocation,
		MaxStackBufSize,
	); err != nil {
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

	lf := ListenFlags
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

	runtime, ctx, shutdown, err := rt.Init(ctx, ac, discoveryFactory, nil, nil, &listenSpec, publisher, RuntimeFlags, reservedDispatcher, 0)
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
