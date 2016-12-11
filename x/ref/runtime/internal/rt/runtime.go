// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"v.io/x/lib/metadata"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/flow"
	"v.io/v23/i18n"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vtrace"

	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/apilog"
	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/pubsub"
	"v.io/x/ref/lib/stats"
	_ "v.io/x/ref/lib/stats/sysstats"
	"v.io/x/ref/runtime/internal/flow/manager"
	"v.io/x/ref/runtime/internal/lib/dependency"
	inamespace "v.io/x/ref/runtime/internal/naming/namespace"
	irpc "v.io/x/ref/runtime/internal/rpc"
	ivtrace "v.io/x/ref/runtime/internal/vtrace"
)

type contextKey int

const (
	clientKey = contextKey(iota)
	namespaceKey
	principalKey
	backgroundKey
	reservedNameKey
	listenKey

	// initKey is used to store values that are only set at init time.
	initKey
)

func init() {
	metadata.Insert("v23.RPCEndpointVersion", fmt.Sprint(naming.DefaultEndpointVersion))
}

var (
	errDiscoveryNotInitialized = verror.Register(pkgPath+".errDiscoveryNotInitialized", verror.NoRetry, "{1:}{2:} discovery not initialized")
)

var setPrincipalCounter int32 = -1

type initData struct {
	appCycle          v23.AppCycle
	discoveryFactory  idiscovery.Factory
	namespaceFactory  inamespace.Factory
	protocols         []string
	settingsPublisher *pubsub.Publisher
	connIdleExpiry    time.Duration
}

type vtraceDependency struct{}

// Runtime implements the v23.Runtime interface.
// Please see the interface definition for documentation of the
// individiual methods.
type Runtime struct {
	ctx  *context.T
	deps *dependency.Graph
}

func Init(
	ctx *context.T,
	appCycle v23.AppCycle,
	discoveryFactory idiscovery.Factory,
	namespaceFactory inamespace.Factory,
	protocols []string,
	listenSpec *rpc.ListenSpec,
	settingsPublisher *pubsub.Publisher,
	flags flags.RuntimeFlags,
	reservedDispatcher rpc.Dispatcher,
	connIdleExpiry time.Duration) (*Runtime, *context.T, v23.Shutdown, error) {
	r := &Runtime{deps: dependency.NewGraph()}

	ctx = context.WithValue(ctx, initKey, &initData{
		appCycle:          appCycle,
		discoveryFactory:  discoveryFactory,
		namespaceFactory:  namespaceFactory,
		protocols:         protocols,
		settingsPublisher: settingsPublisher,
		connIdleExpiry:    connIdleExpiry,
	})

	if listenSpec != nil {
		ctx = context.WithValue(ctx, listenKey, listenSpec.Copy())
	}

	if reservedDispatcher != nil {
		ctx = context.WithValue(ctx, reservedNameKey, reservedDispatcher)
	}

	// Configure the context to use the global logger.
	ctx = context.WithLogger(ctx, logger.Global())

	// We want to print out metadata only into the log files, to avoid
	// spamming stderr, see #1246.
	//
	// TODO(caprita): We should add it to the log file header information;
	// since that requires changes to the llog and vlog packages, for now we
	// condition printing of metadata on having specified an explicit
	// log_dir for the program.  It's a hack, but it gets us the metadata
	// to device manager-run apps and avoids it for command-lines, which is
	// a good enough approximation.
	if logger.Manager(ctx).LogDir() != os.TempDir() {
		ctx.Infof(metadata.ToXML())
	}

	// Setup the initial trace.
	ctx, err := ivtrace.Init(ctx, flags.Vtrace)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, _ = vtrace.WithNewTrace(ctx)
	r.addChild(ctx, vtraceDependency{}, func() {
		vtrace.FormatTraces(os.Stderr, vtrace.GetStore(ctx).TraceRecords(), nil)
	})

	ctx = context.WithContextLogger(ctx, &ivtrace.VTraceLogger{})

	// Setup i18n.
	ctx = i18n.WithLangID(ctx, i18n.LangIDFromEnv())
	if len(flags.I18nCatalogue) != 0 {
		cat := i18n.Cat()
		for _, filename := range strings.Split(flags.I18nCatalogue, ",") {
			err := cat.MergeFromFile(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: i18n: error reading i18n catalogue file %q: %s\n", os.Args[0], filename, err)
			}
		}
	}

	// Setup the program name.
	ctx = verror.WithComponentName(ctx, filepath.Base(os.Args[0]))

	// Enable signal handling.
	r.initSignalHandling(ctx)

	// Set the initial namespace.
	ctx, _, err = r.setNewNamespace(ctx, flags.NamespaceRoots...)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create and set the principal
	principal, shutdown, err := r.initPrincipal(ctx, flags.Credentials)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, err = r.setPrincipal(ctx, principal, shutdown)
	if err != nil {
		return nil, nil, nil, err
	}

	// Add the Client to the context.
	ctx, _, err = r.WithNewClient(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	r.ctx = ctx
	return r, r.WithBackgroundContext(ctx), r.shutdown, nil
}

func (r *Runtime) addChild(ctx *context.T, me interface{}, stop func(), dependsOn ...interface{}) error {
	if err := r.deps.Depend(me, dependsOn...); err != nil {
		stop()
		return err
	} else if done := ctx.Done(); done != nil {
		go func() {
			<-done
			finish := r.deps.CloseAndWait(me)
			stop()
			finish()
		}()
	}
	return nil
}

func (r *Runtime) Init(ctx *context.T) error {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	return r.initMgmt(ctx)
}

func (r *Runtime) shutdown() {
	r.deps.CloseAndWaitForAll()
	r.ctx.FlushLog()
}

func (r *Runtime) initSignalHandling(ctx *context.T) {
	// TODO(caprita): Given that our device manager implementation is to
	// kill all child apps when the device manager dies, we should
	// enable SIGHUP on apps by default.

	// Automatically handle SIGHUP to prevent applications started as
	// daemons from being killed.  The developer can choose to still listen
	// on SIGHUP and take a different action if desired.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)
	go func() {
		for {
			sig, ok := <-signals
			if !ok {
				break
			}
			ctx.Infof("Received signal %v", sig)
		}
	}()
	r.addChild(ctx, signals, func() {
		signal.Stop(signals)
		close(signals)
	})
}

func (r *Runtime) setPrincipal(ctx *context.T, principal security.Principal, shutdown func()) (*context.T, error) {
	stop := shutdown
	if principal != nil {
		// Uniquely identify blessingstore and blessingroots with
		// security/principal/<publicKey>/(blessingstore|blessingroots)/<counter>.
		// Make sure to stop exporting the stats when the context dies.
		var (
			counter = atomic.AddInt32(&setPrincipalCounter, 1)
			prefix  = "security/principal/" + principal.PublicKey().String()
			store   = fmt.Sprintf("%s/blessingstore/%d", prefix, counter)
			roots   = fmt.Sprintf("%s/blessingroots/%d", prefix, counter)
		)
		stats.NewStringFunc(store, principal.BlessingStore().DebugString)
		stats.NewStringFunc(roots, principal.Roots().DebugString)
		stop = func() {
			if shutdown != nil {
				shutdown()
			}
			stats.Delete(store)
			stats.Delete(roots)
		}
	}
	ctx = context.WithValue(ctx, principalKey, principal)
	return ctx, r.addChild(ctx, principal, stop)
}

func (r *Runtime) WithPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	defer apilog.LogCallf(ctx, "principal=%v", principal)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	var err error
	newctx := ctx

	// TODO(mattr, suharshs): If there user gives us some principal that has dependencies
	// we don't know about, we will not honour those dependencies during shutdown.
	// For example if they create an agent principal with some client, we don't know
	// about that, so servers based of this new principal will not prevent the client
	// from terminating early.
	if newctx, err = r.setPrincipal(ctx, principal, func() {}); err != nil {
		return ctx, err
	}
	if newctx, _, err = r.setNewNamespace(newctx, r.GetNamespace(ctx).Roots()...); err != nil {
		return ctx, err
	}
	if newctx, _, err = r.WithNewClient(newctx); err != nil {
		return ctx, err
	}

	return newctx, nil
}

func (*Runtime) GetPrincipal(ctx *context.T) security.Principal {
	// nologcall
	p, _ := ctx.Value(principalKey).(security.Principal)
	return p
}

func (r *Runtime) WithNewClient(ctx *context.T, opts ...rpc.ClientOpt) (*context.T, rpc.Client, error) {
	defer apilog.LogCallf(ctx, "opts...=%v", opts)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	otherOpts := append([]rpc.ClientOpt{}, opts...)

	p, _ := ctx.Value(principalKey).(security.Principal)
	id, _ := ctx.Value(initKey).(*initData)
	if id.protocols != nil {
		otherOpts = append(otherOpts, irpc.PreferredProtocols(id.protocols))
	}
	if id.connIdleExpiry > 0 {
		otherOpts = append(otherOpts, irpc.IdleConnectionExpiry(id.connIdleExpiry))
	}
	deps := []interface{}{vtraceDependency{}}
	client := irpc.NewClient(ctx, otherOpts...)
	newctx := context.WithValue(ctx, clientKey, client)
	if p != nil {
		deps = append(deps, p)
	}
	if err := r.addChild(ctx, client, client.Close, deps...); err != nil {
		return ctx, nil, err
	}
	return newctx, client, nil
}

func (*Runtime) GetClient(ctx *context.T) rpc.Client {
	// nologcall
	cl, _ := ctx.Value(clientKey).(rpc.Client)
	return cl
}

func (r *Runtime) setNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	id, _ := ctx.Value(initKey).(*initData)
	var ns namespace.T
	var err error
	if ns, err = inamespace.New(roots...); err != nil {
		return nil, nil, err
	}
	if id.namespaceFactory != nil {
		if ns, err = id.namespaceFactory(ctx, ns, roots...); err != nil {
			return nil, nil, err
		}
	}
	if oldNS := r.GetNamespace(ctx); oldNS != nil {
		ns.CacheCtl(oldNS.CacheCtl()...)
	}
	ctx = context.WithValue(ctx, namespaceKey, ns)
	return ctx, ns, err
}

func (r *Runtime) WithNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	defer apilog.LogCallf(ctx, "roots...=%v", roots)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	newctx, ns, err := r.setNewNamespace(ctx, roots...)
	if err != nil {
		return ctx, nil, err
	}

	// Replace the client since it depends on the namespace.
	newctx, _, err = r.WithNewClient(newctx)
	if err != nil {
		return ctx, nil, err
	}

	return newctx, ns, err
}

func (*Runtime) GetNamespace(ctx *context.T) namespace.T {
	// nologcall
	ns, _ := ctx.Value(namespaceKey).(namespace.T)
	return ns
}

func (*Runtime) GetAppCycle(ctx *context.T) v23.AppCycle {
	// nologcall
	id, _ := ctx.Value(initKey).(*initData)
	return id.appCycle
}

func (*Runtime) GetListenSpec(ctx *context.T) rpc.ListenSpec {
	// nologcall
	ls, _ := ctx.Value(listenKey).(rpc.ListenSpec)
	return ls
}

func (*Runtime) WithListenSpec(ctx *context.T, ls rpc.ListenSpec) *context.T {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	return context.WithValue(ctx, listenKey, ls.Copy())
}

func (*Runtime) WithBackgroundContext(ctx *context.T) *context.T {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	// Note we add an extra context with a nil value here.
	// This prevents users from travelling back through the
	// chain of background contexts.
	ctx = context.WithValue(ctx, backgroundKey, nil)
	return context.WithValue(ctx, backgroundKey, ctx)
}

func (*Runtime) GetBackgroundContext(ctx *context.T) *context.T {
	// nologcall
	bctx, _ := ctx.Value(backgroundKey).(*context.T)
	if bctx == nil {
		// There should always be a background context.  If we don't find
		// it, that means that the user passed us the background context
		// in hopes of following the chain.  Instead we just give them
		// back what they sent in, which is correct.
		return ctx
	}
	return bctx
}

func (*Runtime) NewDiscovery(ctx *context.T) (discovery.T, error) {
	// nologcall
	id, _ := ctx.Value(initKey).(*initData)
	if id.discoveryFactory != nil {
		return id.discoveryFactory.New(ctx)
	}
	return nil, verror.New(errDiscoveryNotInitialized, ctx)
}

func (*Runtime) WithReservedNameDispatcher(ctx *context.T, d rpc.Dispatcher) *context.T {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	return context.WithValue(ctx, reservedNameKey, d)
}

func (*Runtime) GetReservedNameDispatcher(ctx *context.T) rpc.Dispatcher {
	// nologcall
	if d, ok := ctx.Value(reservedNameKey).(rpc.Dispatcher); ok {
		return d
	}
	return nil
}

func (r *Runtime) NewFlowManager(ctx *context.T, channelTimeout time.Duration) (flow.Manager, error) {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	rid, err := naming.NewRoutingID()
	if err != nil {
		return nil, err
	}
	id, _ := ctx.Value(initKey).(*initData)
	return manager.New(ctx, rid, id.settingsPublisher, channelTimeout, id.connIdleExpiry, nil), nil
}

func (r *Runtime) commonServerInit(ctx *context.T, opts ...rpc.ServerOpt) (*pubsub.Publisher, []rpc.ServerOpt, error) {
	otherOpts := append([]rpc.ServerOpt{}, opts...)
	if reservedDispatcher := r.GetReservedNameDispatcher(ctx); reservedDispatcher != nil {
		otherOpts = append(otherOpts, irpc.ReservedNameDispatcher{
			Dispatcher: reservedDispatcher,
		})
	}
	id, _ := ctx.Value(initKey).(*initData)
	if id.protocols != nil {
		otherOpts = append(otherOpts, irpc.PreferredServerResolveProtocols(id.protocols))
	}
	if id.connIdleExpiry > 0 {
		otherOpts = append(otherOpts, irpc.IdleConnectionExpiry(id.connIdleExpiry))
	}
	return id.settingsPublisher, otherOpts, nil
}

func (r *Runtime) WithNewServer(ctx *context.T, name string, object interface{}, auth security.Authorizer, opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	spub, opts, err := r.commonServerInit(ctx, opts...)
	if err != nil {
		return ctx, nil, err
	}
	newctx, s, err := irpc.WithNewServer(ctx, name, object, auth, spub, opts...)
	if err != nil {
		return ctx, nil, err
	}
	if err = r.addChild(ctx, s, func() { <-s.Closed() }); err != nil {
		return ctx, nil, err
	}
	return newctx, s, nil
}

func (r *Runtime) WithNewDispatchingServer(ctx *context.T, name string, disp rpc.Dispatcher, opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {
	defer apilog.LogCall(ctx)(ctx) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	spub, opts, err := r.commonServerInit(ctx, opts...)
	if err != nil {
		return ctx, nil, err
	}
	newctx, s, err := irpc.WithNewDispatchingServer(ctx, name, disp, spub, opts...)
	if err != nil {
		return ctx, nil, err
	}
	if err = r.addChild(ctx, s, func() { <-s.Closed() }); err != nil {
		return ctx, nil, err
	}
	return newctx, s, nil
}
