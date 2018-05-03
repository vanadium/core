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
	"bytes"
	"flag"
	"os"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"

	"v.io/x/lib/vlog"
	"v.io/x/ref"
	"v.io/x/ref/internal/logger"
	dfactory "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/pubsub"
	"v.io/x/ref/runtime/internal"
	"v.io/x/ref/runtime/internal/lib/appcycle"
	"v.io/x/ref/runtime/internal/rt"
	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp"
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
	"v.io/x/ref/services/debug/debuglib"
)

func defaultRoots() []string {
	_, envRoots := ref.EnvNamespaceRoots()
	if len(envRoots) > 0 {
		return envRoots
	}
	return []string{flags.DefaultNamespaceRoot()}
}

var (
	// LogToStderr is the equivalent of --logtostderr
	LogToStderr vlog.LogToStderr
	// AlsoLogToStderr is the equivalent of --alsologtostderr
	AlsoLogToStderr vlog.AlsoLogToStderr
	// LogDir is the equivalent of --log_dir
	LogDir vlog.LogDir
	// Level is the equivalent of --v
	Level vlog.Level
	// StderrThreshold is the equivalent of --stderrthreshold
	StderrThreshold = vlog.StderrThreshold(vlog.ErrorLog)
	// ModuleSpec is the equivalent of --vmodule
	ModuleSpec vlog.ModuleSpec
	// FilepathSpec is the equivalent of --vpath
	FilepathSpec vlog.FilepathSpec
	// TraceLocation is the equivalent of --log_backtrace_at
	TraceLocation vlog.TraceLocation
	// MaxStackBufSize is the equivalent of --max_stack_buf_size
	MaxStackBufSize vlog.MaxStackBufSize

	// RuntimeFlags is the equivalent of the following flags as per
	// v.io/x/ref/lib/flags:
	// --v23.namespace.root (which may be repeated to supply multiple values)
	// --v23.credentials
	// --v23.i18n-catalogue
	// --v23.vtrace.sample-rate
	// --v23.vtrace.dump-on-shutdown
	// --v23.vtrace.cache-size
	// --v23.vtrace.collect-regexp
	RuntimeFlags = flags.RuntimeFlags{
		NamespaceRoots: defaultRoots(),
		Credentials:    os.Getenv(ref.EnvCredentials),
		I18nCatalogue:  os.Getenv(ref.EnvI18nCatalogueFiles),
		Vtrace: flags.VtraceFlags{
			SampleRate:     0.0,
			DumpOnShutdown: true,
			CacheSize:      1024,
			LogLevel:       0,
			CollectRegexp:  "",
		},
	}

	// ListenAddrs is the equivalent of the following flags as per flags.ListenFlags
	// --v23.tcp.protocol
	// --v23.tcp.address
	ListenAddrs flags.ListenAddrs
	// is the equivalent of --v23.proxy
	ListenProxy string

	// RuntimePermissionsFile is the equivalent of
	// --v23.permissions.file=runtime:<file>
	RuntimePermissionsFile string
	// RuntimePermissionsLiteral is the equivalent of -v23.permissions.literal
	RuntimePermissionsLiteral string

	// Roam controls whether roaming is enabled.
	Roam bool
)

func init() {
	v23.RegisterRuntimeFactory(Init)
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
}

func newAuthorizer() (security.Authorizer, error) {
	if RuntimePermissionsFile == "" && RuntimePermissionsLiteral == "" {
		return nil, nil
	}
	var a security.Authorizer
	var err error
	if RuntimePermissionsLiteral == "" {
		a, err = access.PermissionsAuthorizerFromFile(RuntimePermissionsFile, access.TypicalTagType())
	} else {
		var perms access.Permissions
		if perms, err = access.ReadPermissions(bytes.NewBufferString(RuntimePermissionsLiteral)); err == nil {
			a = access.TypicalTagTypePermissionsAuthorizer(perms)
		}
	}
	if err != nil {
		return nil, err
	}
	return a, err
}

// AddAndParseFlags adds Vanadium-specific flags to the provided FlagSet, which
// for this factory has no effect.
func AddAndParseFlags(fs *flag.FlagSet) error { return nil }

func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {
	if err := logger.Manager(logger.Global()).ConfigureFromArgs(
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

	listenSpec := rpc.ListenSpec{
		Addrs:          rpc.ListenAddrs(ListenAddrs),
		Proxy:          ListenProxy,
		AddressChooser: internal.NewAddressChooser(logger.Global()),
	}

	authorizer, err := newAuthorizer()
	if err != nil {
		return nil, nil, nil, err
	}
	reservedDispatcher := debuglib.NewDispatcher(authorizer)

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
	var publisher *pubsub.Publisher
	if Roam {
		publisher = pubsub.NewPublisher()
	}

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
