// Copyright 2017 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

// Package library implements a RuntimeFactory suitable for building a Vanadium
// library that is linked into other applications. It is configured via a set
// of exported variables as well as optionally via the v.io/x/ref/lib/flags
// package which allows for both internal configuration as well as via the
// command line or environment.
// Client packages (factories) that wish to be configured via the command line
// must call EnableCommadLineFlags, all other users should use the
// flags package's 'SetDefault...' methods.
// This scheme allows this package to be broadly used to create 'factories'
// and other initialization mechanisms whilst reducing code duplication.
//
// The pubsub.Publisher mechanism is used for communicating networking
// settings to the rpc.Server implementation of the runtime and publishes
// the Settings it expects.
package library

import (
	"flag"
	"fmt"
	"sync"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/rpc"
	"v.io/v23/security/access"

	"v.io/x/lib/vlog"
	"v.io/x/ref/internal/logger"
	dfactory "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/pubsub"
	"v.io/x/ref/runtime/internal"
	"v.io/x/ref/runtime/internal/lib/appcycle"
	"v.io/x/ref/runtime/internal/rt"
	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp" // Initialize tcp, ws and wsh.
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

	// FlagSet represents the set of flags that can be interpreted as variables.
	// Client packages that wish to configure this package via command line
	// flags should call EnableCommandlineFlags.
	flagSet *flags.Flags

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

	// ConnectionExpiryDuration sets the rate at which cached connections
	// will be considered for eviction from the cache.
	ConnectionExpiryDuration = 10 * time.Minute

	state factoryState
)

type factoryState struct {
	sync.Mutex
	running        bool
	initialized    bool
	logFlagsParsed bool
	rtFlagsParsed  bool
}

func (st *factoryState) setRunning(s bool) {
	st.Lock()
	st.running = s
	if s {
		st.initialized = true
	}
	st.Unlock()
}

func (st *factoryState) logsParsed() {
	st.Lock()
	st.logFlagsParsed = true
	st.Unlock()
}

func (st *factoryState) rtParsed() {
	st.Lock()
	st.rtFlagsParsed = true
	st.Unlock()
}

func (st *factoryState) getState() (initialized, running bool) {
	st.Lock()
	defer st.Unlock()
	initialized, running = st.initialized, st.running
	return
}

func (st *factoryState) getParsingState() (logsParsed, runtimeParsed bool) {
	st.Lock()
	defer st.Unlock()
	logsParsed, runtimeParsed = st.logFlagsParsed, st.rtFlagsParsed
	return
}

func init() {
	v23.RegisterRuntimeFactory(Init)
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
}

// EnableCommandlineFlags enables use of command line flags.
func EnableCommandlineFlags() {
	EnableFlags(flag.CommandLine)
}

// EnableFlags enables the use of flags on the specified flag set.
func EnableFlags(fs *flag.FlagSet) {
	flagSet = flags.CreateAndRegister(fs, flags.Runtime, flags.Listen, flags.Permissions)
}

// LogFlagsParsedExternally prevents the initialization code from re-parsing
// the log related flags.
func LogFlagsParsedExternally() {
	state.logsParsed()
}

// RuntimeFlagsParsedExternally prevents the initialization code from re-parsing
// the runtime related flags.
func RuntimeFlagsParsedExternally() {
	state.rtParsed()
}

func configureLogging() error {
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
			return fmt.Errorf("libary.Init: %v", err)
		}
	}
	return err
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

	logsParsed, rtParsed := state.getParsingState()
	if !logsParsed {
		if err := configureLogging(); err != nil {
			return nil, nil, nil, err
		}
	}

	previousFlagSet := flagSet
	if !rtParsed {
		if flagSet == nil {
			dummy := &flag.FlagSet{}
			flagSet = flags.CreateAndRegister(dummy,
				flags.Runtime, flags.Listen, flags.Permissions)
		} else {
			// Only parse flags if EnableFlags has been called.
			if err := internal.ParseFlagsIncV23Env(flagSet); err != nil {
				return nil, nil, nil, fmt.Errorf("library.Init: %v", err)
			}
		}
	}

	runtimeFlags := flagSet.RuntimeFlags()
	listenFlags := flagSet.ListenFlags()
	permissionsFlags := flagSet.PermissionsFlags()

	permissionsSpec := access.PermissionsSpec{
		Files:   permissionsFlags.PermissionsNamesAndFiles(),
		Literal: permissionsFlags.PermissionsLiteral(),
	}

	ishutdown := func(sf ...func()) {
		for _, f := range sf {
			if f != nil {
				f()
			}
		}
	}

	var cancelCloud func()
	var err error
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
		Addrs:          rpc.ListenAddrs(listenFlags.Addrs),
		Proxy:          listenFlags.Proxy,
		AddressChooser: internal.NewAddressChooser(logger.Global()),
	}

	var reservedDispatcher rpc.Dispatcher
	if ReservedNameDispatcher {
		authorizer, err := access.AuthorizerFromSpec(
			permissionsSpec, "runtime", access.TypicalTagType())
		if err != nil {
			return nil, nil, nil, err
		}
		reservedDispatcher = debuglib.NewDispatcher(authorizer)
	}

	var publisher *pubsub.Publisher
	if Roam {
		publisher = pubsub.NewPublisher()
	}

	runtime, ctx, shutdown, err := rt.Init(ctx,
		ac,
		discoveryFactory,
		nil,
		nil,
		&listenSpec,
		publisher,
		runtimeFlags,
		reservedDispatcher,
		permissionsSpec,
		ConnectionExpiryDuration)
	if err != nil {
		ishutdown(ac.Shutdown, cancelCloud, discoveryFactory.Shutdown)
		return nil, nil, nil, err
	}

	runtimeFactoryShutdown := func() {
		ishutdown(ac.Shutdown, cancelCloud, discoveryFactory.Shutdown)
		shutdown()
		flagSet = previousFlagSet
		state.setRunning(false)
	}
	state.setRunning(true)
	return runtime, ctx, runtimeFactoryShutdown, nil
}
