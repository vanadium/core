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
	"net"
	"os"
	"sync"
	"time"

	v23 "v.io/v23"
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

	// CloudVM controls whether virtualization/cloud configuration is enabled.
	CloudVM = false

	// ReservedNameDispatcher controls whether a dispatcher is created
	// for the reserved names on an RPC dispatcher. If it is set then
	// its authorization will be determined by the PermissionsFlags and
	// in particular the 'runtime' file therein.
	ReservedNameDispatcher = false

	// ConfigureLoggingFromFlags controls whether the logging related variables
	// above are used for configuring logging, or if command line flags
	// are used instead.
	ConfigureLoggingFromFlags = false

	// PermissionsSpec specifies the permissions to be stored in the context
	// and thus used by default.
	PermissionsSpec access.PermissionsSpec

	// ConfigurePermissionsFromFlags controls whether the permissions
	// related variable above is used for configuring permissions, or if
	// command line flags are used instead.
	ConfigurePermissionsFromFlags = false

	// AllowMultipleInitializations controls whether the runtime can
	// be initialized multiple times. The shutdown callback must be called
	// between multiple initializations.
	AllowMultipleInitializations = false

	// ConnectionExpiryDuration sets the rate at which cached connections
	// will be considered for eviction from the cache.
	ConnectionExpiryDuration = 10 * time.Minute

	state factoryState
)

// VirtualizedFlags flags returns a copy of flags.VirtualizedFlags as
// obtained from the command line and/or environment via the flags
// package and an indication of whether the flag group was enabled.
func VirtualizedFlags() (flags.VirtualizedFlags, bool) {
	return flagSet.VirtualizedFlags(), flagSet.HasGroup(flags.Virtualized)
}

type factoryState struct {
	sync.Mutex
	running       bool
	initialized   bool
	rtFlagsParsed bool
}

func (st *factoryState) setRunning(s bool) {
	st.Lock()
	st.running = s
	if s {
		st.initialized = true
	}
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

func (st *factoryState) getParsingState() (runtimeParsed bool) {
	st.Lock()
	defer st.Unlock()
	runtimeParsed = st.rtFlagsParsed
	return
}

func init() {
	v23.RegisterRuntimeFactory(Init)
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
}

func configuredFlags() []flags.FlagGroup {
	configured := []flags.FlagGroup{}
	if CloudVM {
		configured = append(configured, flags.Virtualized)
	}
	if !ConfigurePermissionsFromFlags {
		return append(configured, flags.Runtime, flags.Listen)
	}
	return append(configured,
		flags.Runtime,
		flags.Listen,
		flags.Permissions,
	)
}

// EnableCommandlineFlags enables use of command line flags.
func EnableCommandlineFlags() {
	EnableFlags(flag.CommandLine, false) //nolint:errcheck
}

// EnableFlags enables the use of flags on the specified flag set and returns
// the newly created v.io/x/ref/lib/flags.Flags to allow for external
// parsing by the caller if need be. It will optionally parse the newly
// created and registered flags.
func EnableFlags(fs *flag.FlagSet, parse bool) error {
	fset, err := flags.CreateAndRegister(fs, configuredFlags()...)
	if err != nil {
		return nil
	}
	flagSet = fset
	if parse {
		if err := flagSet.Parse(os.Args[1:], nil); err != nil {
			return err
		}
		state.rtParsed()
	}
	return nil
}

func configureLogging() error {
	var err error
	if ConfigureLoggingFromFlags {
		// Ensure that command line flags are parsed before they are used.
		if !flag.Parsed() {
			flag.Parse()
		}
		err = logger.Global().ConfigureFromFlags(LoggingOpts...)
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
		err = logger.Global().ConfigureFromArgs(
			append(opts, LoggingOpts...)...)
	}
	if err != nil {
		if !AllowMultipleInitializations || !logger.IsAlreadyConfiguredError(err) {
			return fmt.Errorf("library.Init: %v", err)
		}
		return nil
	}
	return err
}

type passthroughAddressChooser struct{}

func (c *passthroughAddressChooser) ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	return candidates, nil
}

// Init creates a new v23.Runtime.
func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) { //nolint:gocyclo
	initialized, running := state.getState()
	if AllowMultipleInitializations && running {
		return nil, nil, nil, fmt.Errorf("library.Init called whilst a previous instance is still running, the shutdown callback has not bee called")
	}

	if !AllowMultipleInitializations && initialized {
		return nil, nil, nil, fmt.Errorf("library.Init incorrectly called multiple times")
	}

	if err := configureLogging(); err != nil {
		return nil, nil, nil, err
	}

	previousFlagSet := flagSet
	if flagSet == nil {
		dummy := &flag.FlagSet{}
		fset, err := flags.CreateAndRegister(dummy, configuredFlags()...)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("library.Init: flags: %w", err)
		}
		flagSet = fset
	} else {
		rtParsed := state.getParsingState()
		if !rtParsed {
			// Only parse flags if EnableFlags has been called.
			if err := flagSet.Parse(os.Args[1:], nil); err != nil {
				if err == flag.ErrHelp {
					return nil, nil, nil, err
				}
				return nil, nil, nil, fmt.Errorf("library.Init: runtime flags: %w", err)
			}
		}
	}

	runtimeFlags := flagSet.RuntimeFlags()
	listenFlags := flagSet.ListenFlags()

	if ConfigurePermissionsFromFlags {
		permissionsFlags := flagSet.PermissionsFlags()
		PermissionsSpec = access.PermissionsSpec{
			ExplicitlySpecified: permissionsFlags.ExplicitlySpecified(),
			Files:               permissionsFlags.PermissionsNamesAndFiles(),
			Literal:             permissionsFlags.PermissionsLiteral(),
		}
	}

	ishutdown := func(sf ...func()) {
		for _, f := range sf {
			if f != nil {
				f()
			}
		}
	}

	var chooser rpc.AddressChooser = &passthroughAddressChooser{}
	var err error

	if CloudVM {
		vf := flagSet.VirtualizedFlags()
		chooser = internal.AsyncCloudAddressChoser(ctx, logger.Global(), &vf)
	}

	discoveryFactory, err := dfactory.New(ctx)
	if err != nil {
		ishutdown()
		return nil, nil, nil, err
	}

	listenSpec := rpc.ListenSpec{
		Addrs:          rpc.ListenAddrs(listenFlags.Addrs),
		Proxy:          listenFlags.Proxy,
		ProxyPolicy:    listenFlags.ProxyPolicy.Value(),
		AddressChooser: chooser,
	}

	var reservedDispatcher rpc.Dispatcher
	if ReservedNameDispatcher {
		authorizer, err := access.AuthorizerFromSpec(
			PermissionsSpec, "runtime", access.TypicalTagType())
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
		discoveryFactory,
		nil,
		nil,
		&listenSpec,
		publisher,
		runtimeFlags,
		reservedDispatcher,
		&PermissionsSpec,
		ConnectionExpiryDuration)
	if err != nil {
		ishutdown(discoveryFactory.Shutdown)
		return nil, nil, nil, err
	}

	runtimeFactoryShutdown := func() {
		ishutdown(discoveryFactory.Shutdown)
		shutdown()
		flagSet = previousFlagSet
		state.setRunning(false)
	}
	state.setRunning(true)
	return runtime, ctx, runtimeFactoryShutdown, nil
}
