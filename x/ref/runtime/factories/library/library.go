// Copyright 2017 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

// Package library implements a RuntimeFactory suitable for building a Vanadium
// library that is linked into other applications. It is configured via a set
// of exported variables rather than via command line flags. The variables
// are all instances of the flag groups define by v.io/x/ref/lib/flags
// which allows other factories and packages to assign command line
// flags to these variables. The intent being to reduce code duplication
// across factories and packages whilst maintaining consistency of
// flag definitions on the command line.
//
// The pubsub.Publisher mechanism is used for communicating networking
// settings to the rpc.Server implementation of the runtime and publishes
// the Settings it expects.
package library

import (
	"fmt"
	"os"
	"sync"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/rpc"
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

	// LoggingOpts are passed to the logging initialization functions.
	LoggingOpts = []vlog.LoggingOpts{}

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
		Credentials:   os.Getenv(ref.EnvCredentials),
		I18nCatalogue: os.Getenv(ref.EnvI18nCatalogueFiles),
		Vtrace: flags.VtraceFlags{
			SampleRate:     0.0,
			DumpOnShutdown: true,
			CacheSize:      1024,
			LogLevel:       0,
			CollectRegexp:  "",
		},
	}

	// ListenFlags is the equivalent of the following flags as per
	// v.io/x/ref/lib/flags:
	// --v23.tcp.protocol
	// --v23.tcp.address
	//-v23.proxy
	ListenFlags = flags.ListenFlags{}

	// PermissionsFlags is the equivalent of the following flags as per
	// v.io/x/ref/lib/flags:
	// --v23.permissions.file=runtime:<file>
	// --v23.permissions.literal=<json-permissions>
	PermissionsFlags flags.PermissionsFlags

	// Roam controls whether roaming is enabled.
	Roam = false

	// CloudVM controls whether cloud configuration is enabled.
	CloudVM = true

	// ReservedNameDispatcher controls whether a dispatcher is created
	// for the reserved names on an RPC dispatcher. If it is set then
	// its authorization will be determined by the PermissionsFlags and
	// in particular the 'runtime' file therein.
	ReservedNameDispatcher = false

	// ConfigureLoggingFromFlags controls whether the logging related variables
	// above are used for configuring logging, or if command line flags
	// are used instead.
	ConfigureLoggingFromFlags = false

	// AllowMultipleInitializations controls whether the runtime can
	// be initialized multiple times. The shutdown callback must be called
	// between multiple initializations.
	AllowMultipleInitializations = false

	state factoryState
)

type factoryState struct {
	sync.Mutex
	running     bool
	initialized bool
}

func (st *factoryState) setRunning(s bool) {
	st.Lock()
	st.running = s
	if s {
		st.initialized = true
	}
	st.Unlock()
}

func (st *factoryState) getState() (initialized, running bool) {
	st.Lock()
	defer st.Unlock()
	initialized, running = st.initialized, st.running
	return
}

func init() {
	v23.RegisterRuntimeFactory(Init)
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
}

// Init creates a new v23.Runtime.
func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {

	initialized, running := state.getState()
	if AllowMultipleInitializations && running {
		return nil, nil, nil, fmt.Errorf("Library.init called whilst a previous instance is still running, the shutdown callback has not bee called")
	}

	if !AllowMultipleInitializations && initialized {
		return nil, nil, nil, fmt.Errorf("Library.init incorrectly called multiple times")
	}

	var err error
	if ConfigureLoggingFromFlags {
		err = logger.Manager(logger.Global()).ConfigureFromFlags(LoggingOpts...)
	} else {
		opts := []vlog.LoggingOpts{
			LogToStderr,
			AlsoLogToStderr,
			LogDir,
			Level,
			StderrThreshold,
			ModuleSpec,
			FilepathSpec,
			TraceLocation,
			MaxStackBufSize,
		}
		err = logger.Manager(logger.Global()).ConfigureFromArgs(
			append(opts, LoggingOpts...)...)
	}
	if err != nil {
		if !AllowMultipleInitializations || !logger.IsAlreadyConfiguredError(err) {
			return nil, nil, nil, fmt.Errorf("libary.Init: %v", err)
		}
	}

	permissionsSpec := access.PermissionsSpec{
		Files:   PermissionsFlags.PermissionsNamesAndFiles(),
		Literal: PermissionsFlags.PermissionsLiteral(),
	}
	ishutdown := func(sf ...func()) {
		for _, f := range sf {
			if f != nil {
				f()
			}
		}
	}

	var cancelCloud func()
	if CloudVM {
		cancelCloud, err = internal.InitCloudVM()
		if err != nil {
			return nil, nil, nil, err
		}
	}

	ac := appcycle.New()
	discoveryFactory, err := dfactory.New(ctx)
	if err != nil {
		ishutdown(ac.Shutdown, cancelCloud)
		return nil, nil, nil, err
	}

	listenSpec := rpc.ListenSpec{
		Addrs:          rpc.ListenAddrs(ListenFlags.Addrs),
		Proxy:          ListenFlags.Proxy,
		AddressChooser: internal.NewAddressChooser(logger.Global()),
	}

	var reservedDispatcher rpc.Dispatcher
	if ReservedNameDispatcher {
		authorizer, err := access.AuthorizerFromSpec(
			permissionsSpec, true, "runtime", access.TypicalTagType())
		if err != nil {
			return nil, nil, nil, err
		}
		reservedDispatcher = debuglib.NewDispatcher(authorizer)
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

	// Read the default namespace roots since they may have been changed
	// by another package's init function.
	RuntimeFlags.NamespaceRoots = flags.DefaultNamespaceRoots()
	runtime, ctx, shutdown, err := rt.Init(ctx,
		ac,
		discoveryFactory,
		nil,
		nil,
		&listenSpec,
		publisher,
		RuntimeFlags,
		reservedDispatcher,
		permissionsSpec,
		0)
	if err != nil {
		ishutdown(ac.Shutdown, cancelCloud, discoveryFactory.Shutdown)
		return nil, nil, nil, err
	}

	runtimeFactoryShutdown := func() {
		ishutdown(ac.Shutdown, cancelCloud, discoveryFactory.Shutdown)
		shutdown()
		state.setRunning(false)
	}
	state.setRunning(true)
	return runtime, ctx, runtimeFactoryShutdown, nil
}
