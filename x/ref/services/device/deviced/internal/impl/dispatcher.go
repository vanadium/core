// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/device"
	"v.io/v23/services/pprof"
	libstats "v.io/v23/services/stats"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/v23/verror"
	s_device "v.io/x/ref/services/device"
	"v.io/x/ref/services/device/internal/config"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/internal/logreaderlib"
	"v.io/x/ref/services/internal/pathperms"
)

// internalState wraps state shared between different device manager
// invocations.
type internalState struct {
	callback       *callbackState
	updating       *updatingState
	principalMgr   principalManager
	restartHandler func()
	stats          *stats
	testMode       bool
	// runner is responsible for running app instances.
	runner *appRunner
	// tidying is the automatic state tidying subsystem.
	tidying chan<- tidyRequests
}

// dispatcher holds the state of the device manager dispatcher.
type dispatcher struct {
	// internal holds the state that persists across RPC method invocations.
	internal *internalState
	// config holds the device manager's (immutable) configuration state.
	config *config.State
	// dispatcherMutex is a lock for coordinating concurrent access to some
	// dispatcher methods.
	mu sync.RWMutex
	// TODO(rjkroege): Consider moving this inside internal.
	uat        BlessingSystemAssociationStore
	permsStore *pathperms.PathStore
	// Namespace
	mtAddress string // The address of the local mounttable.
}

var _ rpc.Dispatcher = (*dispatcher)(nil)

const (
	appsSuffix   = "apps"
	deviceSuffix = "device"
	configSuffix = "cfg"

	// TODO(caprita): the value of pkgPath corresponds to the previous
	// package where the error ids were defined.  Updating error ids needs
	// to be carefully coordinated between clients and servers, so we should
	// do it when we settle on the final location for these error
	// definitions.
	pkgPath = "v.io/x/ref/services/device/internal/impl"
)

var (
	errInvalidConfig          = verror.Register(pkgPath+".errInvalidConfig", verror.NoRetry, "{1:}{2:} invalid config {3}{:_}")
	errCantCreateAccountStore = verror.Register(pkgPath+".errCantCreateAccountStore", verror.NoRetry, "{1:}{2:} cannot create persistent store for identity to system account associations{:_}")
	errCantCreateAppWatcher   = verror.Register(pkgPath+".errCantCreateAppWatcher", verror.NoRetry, "{1:}{2:} cannot create app status watcher{:_}")
	errNewAgentFailed         = verror.Register(pkgPath+".errNewAgentFailed", verror.NoRetry, "{1:}{2:} NewAgent() failed{:_}")
	errStoppedWithErrors      = verror.Register(pkgPath+".errStoppedWithErrors", verror.NoRetry, "{1:}{2:} instance killed uncleanly{:_}")
	errStopFailed             = verror.Register(pkgPath+".errStopFailed", verror.NoRetry, "{1:}{2:} instance couldn't be killed{:_}")
)

// NewDispatcher is the device manager dispatcher factory.  It returns a new
// dispatcher as well as a shutdown function, to be called when the dispatcher
// is no longer needed.
func NewDispatcher(ctx *context.T, config *config.State, mtAddress string, testMode bool, restartHandler func(), permStore *pathperms.PathStore) (rpc.Dispatcher, func(), error) {
	if err := config.Validate(); err != nil {
		return nil, nil, verror.New(errInvalidConfig, ctx, config, err)
	}
	uat, err := NewBlessingSystemAssociationStore(config.Root)
	if err != nil {
		return nil, nil, verror.New(errCantCreateAccountStore, ctx, err)
	}
	InitSuidHelper(ctx, config.Helper)
	d := &dispatcher{
		internal: &internalState{
			callback:       newCallbackState(config.Name),
			updating:       newUpdatingState(),
			restartHandler: restartHandler,
			stats:          newStats("device-manager"),
			testMode:       testMode,
			tidying:        newTidyingDaemon(ctx, config.Root),
			principalMgr:   newPrincipalManager(),
		},
		config:     config,
		uat:        uat,
		permsStore: permStore,
		mtAddress:  mtAddress,
	}
	runner := &appRunner{
		callback:       d.internal.callback,
		principalMgr:   d.internal.principalMgr,
		appServiceName: naming.Join(d.config.Name, appsSuffix),
		mtAddress:      d.mtAddress,
		stats:          d.internal.stats,
	}
	d.internal.runner = runner
	reap, err := newReaper(ctx, config.Root, runner)
	if err != nil {
		return nil, nil, verror.New(errCantCreateAppWatcher, ctx, err)
	}
	runner.reap = reap

	if testMode {
		return &testModeDispatcher{d}, reap.shutdown, nil
	}
	return d, reap.shutdown, nil
}

// Logging invoker that logs any error messages before returning.
func newLoggingInvoker(ctx *context.T, obj interface{}) (rpc.Invoker, error) {
	if invoker, ok := obj.(rpc.Invoker); ok {
		return &loggingInvoker{invoker: invoker}, nil
	}
	invoker, err := rpc.ReflectInvoker(obj)
	if err != nil {
		ctx.Errorf("rpc.ReflectInvoker returned error: %v", err)
		return nil, err
	}
	return &loggingInvoker{invoker: invoker}, nil
}

type loggingInvoker struct {
	invoker rpc.Invoker
}

func (l *loggingInvoker) Prepare(ctx *context.T, method string, numArgs int) (argptrs []interface{}, tags []*vdl.Value, err error) {
	argptrs, tags, err = l.invoker.Prepare(ctx, method, numArgs)
	if err != nil {
		ctx.Errorf("Prepare(%s %d) returned error: %v", method, numArgs, err)
	}
	return
}

func (l *loggingInvoker) Invoke(ctx *context.T, call rpc.StreamServerCall, method string, argptrs []interface{}) (results []interface{}, err error) {
	results, err = l.invoker.Invoke(ctx, call, method, argptrs)
	if err != nil {
		ctx.Errorf("Invoke(method:%s argptrs:%v) returned error: %v", method, argptrs, err)
	}
	return
}

func (l *loggingInvoker) Signature(ctx *context.T, call rpc.ServerCall) ([]signature.Interface, error) {
	sig, err := l.invoker.Signature(ctx, call)
	if err != nil {
		ctx.Errorf("Signature returned error: %v", err)
	}
	return sig, err
}

func (l *loggingInvoker) MethodSignature(ctx *context.T, call rpc.ServerCall, method string) (signature.Method, error) {
	methodSig, err := l.invoker.MethodSignature(ctx, call, method)
	if err != nil {
		ctx.Errorf("MethodSignature(%s) returned error: %v", method, err)
	}
	return methodSig, err
}

func (l *loggingInvoker) Globber() *rpc.GlobState {
	return l.invoker.Globber()
}

// DISPATCHER INTERFACE IMPLEMENTATION
func (d *dispatcher) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	invoker, auth, err := d.internalLookup(suffix)
	if err != nil {
		return nil, nil, err
	}
	loggingInvoker, err := newLoggingInvoker(ctx, invoker)
	if err != nil {
		return nil, nil, err
	}
	return loggingInvoker, auth, nil
}

func newTestableHierarchicalAuth(testMode bool, rootDir, childDir string, get pathperms.PermsGetter) (security.Authorizer, error) {
	if testMode {
		// In test mode, the device manager will not be able to read the
		// Permissions, because they were signed with the key of the real device
		// manager. It's not a problem because the testModeDispatcher overrides the
		// authorizer anyway.
		return nil, nil
	}
	return pathperms.NewHierarchicalAuthorizer(rootDir, childDir, get)
}

func (d *dispatcher) internalLookup(suffix string) (interface{}, security.Authorizer, error) {
	components := strings.Split(suffix, "/")
	for i := 0; i < len(components); i++ {
		if len(components[i]) == 0 {
			components = append(components[:i], components[i+1:]...)
			i--
		}
	}

	// TODO(rjkroege): Permit the root Permissions to diverge for the device and
	// app sub-namespaces of the device manager after claiming.
	auth, err := newTestableHierarchicalAuth(d.internal.testMode, PermsDir(d.config), PermsDir(d.config), d.permsStore)
	if err != nil {
		return nil, nil, err
	}

	if len(components) == 0 {
		return rpc.ChildrenGlobberInvoker(deviceSuffix, appsSuffix), auth, nil
	}
	// The implementation of the device manager is split up into several
	// invokers, which are instantiated depending on the receiver name
	// prefix.
	switch components[0] {
	case deviceSuffix:
		receiver := device.DeviceServer(&deviceService{
			callback:       d.internal.callback,
			updating:       d.internal.updating,
			restartHandler: d.internal.restartHandler,
			config:         d.config,
			disp:           d,
			uat:            d.uat,
			principalMgr:   d.internal.principalMgr,
			tidying:        d.internal.tidying,
		})
		return receiver, auth, nil
	case appsSuffix:
		// Requests to apps/*/*/*/logs are handled locally by LogFileService.
		// Requests to apps/*/*/*/pprof are proxied to the apps' __debug/pprof object.
		// Requests to apps/*/*/*/stats are proxied to the apps' __debug/stats object.
		// Everything else is handled by the Application server.
		if len(components) >= 5 {
			appInstanceDir, err := instanceDir(d.config.Root, components[1:4])
			if err != nil {
				return nil, nil, err
			}
			switch kind := components[4]; kind {
			case "logs":
				logsDir := filepath.Join(appInstanceDir, "logs")
				suffix := naming.Join(components[5:]...)
				appSpecificAuthorizer, err := newAppSpecificAuthorizer(auth, d.config, components[1:], d.permsStore)
				if err != nil {
					return nil, nil, err
				}
				return logreaderlib.NewLogFileService(logsDir, suffix), appSpecificAuthorizer, nil
			case "pprof", "stats":
				info, err := loadInstanceInfo(nil, appInstanceDir)
				if err != nil {
					return nil, nil, err
				}
				if !instanceStateIs(appInstanceDir, device.InstanceStateRunning) {
					return nil, nil, verror.New(errors.ErrInvalidSuffix, nil)
				}
				var desc []rpc.InterfaceDesc
				switch kind {
				case "pprof":
					desc = pprof.PProfServer(nil).Describe__()
				case "stats":
					desc = libstats.StatsServer(nil).Describe__()
				}
				suffix := naming.Join("__debug", naming.Join(components[4:]...))
				remote := naming.JoinAddressName(info.AppCycleMgrName, suffix)

				// Use hierarchical auth with debugacls under debug access.
				appSpecificAuthorizer, err := newAppSpecificAuthorizer(auth, d.config, components[1:], d.permsStore)
				if err != nil {
					return nil, nil, err
				}
				return newProxyInvoker(remote, access.Debug, desc), appSpecificAuthorizer, nil
			}
		}
		receiver := device.ApplicationServer(&appService{
			config:     d.config,
			suffix:     components[1:],
			uat:        d.uat,
			permsStore: d.permsStore,
			runner:     d.internal.runner,
			stats:      d.internal.stats,
		})
		appSpecificAuthorizer, err := newAppSpecificAuthorizer(auth, d.config, components[1:], d.permsStore)
		if err != nil {
			return nil, nil, err
		}
		return receiver, appSpecificAuthorizer, nil
	case configSuffix:
		if len(components) != 2 {
			return nil, nil, verror.New(errors.ErrInvalidSuffix, nil)
		}
		receiver := s_device.ConfigServer(&configService{
			callback: d.internal.callback,
			suffix:   components[1],
		})
		// The nil authorizer ensures that only principals blessed by
		// the device manager can talk back to it.  All apps started by
		// the device manager should fall in that category.
		//
		// TODO(caprita,rjkroege): We should further refine this, by
		// only allowing the app to update state referring to itself
		// (and not other apps).
		return receiver, nil, nil
	default:
		return nil, nil, verror.New(errors.ErrInvalidSuffix, nil)
	}
}

// testModeDispatcher is a wrapper around the real dispatcher. It returns the
// exact same object as the real dispatcher, but the authorizer only allows
// calls to "device".Delete().
type testModeDispatcher struct {
	realDispatcher rpc.Dispatcher
}

func (d *testModeDispatcher) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	obj, _, err := d.realDispatcher.Lookup(ctx, suffix)
	return obj, d, err
}

func (testModeDispatcher) Authorize(ctx *context.T, call security.Call) error {
	if call.Suffix() == deviceSuffix && call.Method() == "Delete" {
		ctx.Infof("testModeDispatcher.Authorize: Allow %q.%s()", call.Suffix(), call.Method())
		return nil
	}
	ctx.Infof("testModeDispatcher.Authorize: Reject %q.%s()", call.Suffix(), call.Method())
	return verror.New(errors.ErrInvalidSuffix, nil)
}

func newAppSpecificAuthorizer(sec security.Authorizer, config *config.State, suffix []string, getter pathperms.PermsGetter) (security.Authorizer, error) {
	// TODO(rjkroege): This does not support <appname>.Start() to start all
	// instances. Correct this.

	// If we are attempting a method invocation against "apps/", we use the root
	// Permissions.
	if len(suffix) == 0 || len(suffix) == 1 {
		return sec, nil
	}
	// Otherwise, we require a per-installation and per-instance Permissions file.
	if len(suffix) == 2 {
		p, err := installationDirCore(suffix, config.Root)
		if err != nil {
			return nil, verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("newAppSpecificAuthorizer failed: %v", err))
		}
		return pathperms.NewHierarchicalAuthorizer(PermsDir(config), path.Join(p, "acls"), getter)
	}
	// Use the special debugacls for instance/logs, instance/pprof, instance/stats.
	if len(suffix) > 3 && (suffix[3] == "logs" || suffix[3] == "pprof" || suffix[3] == "stats") {
		p, err := instanceDir(config.Root, suffix[0:3])
		if err != nil {
			return nil, verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("newAppSpecificAuthorizer failed: %v", err))
		}
		return pathperms.NewHierarchicalAuthorizer(PermsDir(config), path.Join(p, "debugacls"), getter)
	}

	p, err := instanceDir(config.Root, suffix[0:3])
	if err != nil {
		return nil, verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("newAppSpecificAuthorizer failed: %v", err))
	}
	return pathperms.NewHierarchicalAuthorizer(PermsDir(config), path.Join(p, "acls"), getter)
}
