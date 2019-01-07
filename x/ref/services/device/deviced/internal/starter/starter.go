// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package starter provides a single function that starts up servers for a
// mounttable and a device manager that is mounted on it.
package starter

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	displib "v.io/x/ref/lib/dispatcher"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/debug/debuglib"
	"v.io/x/ref/services/device/deviced/internal/impl"
	"v.io/x/ref/services/device/deviced/internal/versioning"
	"v.io/x/ref/services/device/internal/claim"
	"v.io/x/ref/services/device/internal/config"
	"v.io/x/ref/services/internal/pathperms"
	"v.io/x/ref/services/mounttable/mounttablelib"
)

const pkgPath = "v.io/x/ref/services/device/deviced/internal/starter"

var (
	errCantSaveInfo = verror.Register(pkgPath+".errCantSaveInfo", verror.NoRetry, "{1:}{2:} failed to save info{:_}")
)

type NamespaceArgs struct {
	Name            string         // Name to publish the mounttable service under (after claiming).
	ListenSpec      rpc.ListenSpec // ListenSpec for the server.
	PermissionsFile string         // Path to the Permissions file used by the mounttable.
	PersistenceDir  string         // Path to the directory holding persistent acls.
	// Name in the local neighborhood on which to make the mounttable
	// visible. If empty, the mounttable will not be visible in the local
	// neighborhood.
	Neighborhood string
}

type DeviceArgs struct {
	Name            string         // Name to publish the device service under (after claiming).
	ListenSpec      rpc.ListenSpec // ListenSpec for the device server.
	ConfigState     *config.State  // Configuration for the device.
	TestMode        bool           // Whether the device is running in test mode or not.
	RestartCallback func()         // Callback invoked when the device service is restarted.
	PairingToken    string         // PairingToken that a claimer needs to provide.
}

func (d *DeviceArgs) name(mt string) string {
	if d.Name != "" {
		return d.Name
	}
	return naming.Join(mt, "devmgr")
}

type Args struct {
	Namespace NamespaceArgs
	Device    DeviceArgs

	// If true, the global namespace will be made available on the
	// mounttable server under "global/".
	MountGlobalNamespaceInLocalNamespace bool
}

// Start creates servers for the mounttable and device services and links them together.
//
// Returns the endpoints for the claimable service (empty if already claimed),
// a callback to be invoked to shutdown the services on success, or an error on
// failure.
func Start(ctx *context.T, args Args) ([]naming.Endpoint, func(), error) {
	// Is this binary compatible with the state on disk?
	if err := versioning.CheckCompatibility(ctx, args.Device.ConfigState.Root); err != nil {
		return nil, nil, err
	}
	// In test mode, we skip writing the info file to disk, and we skip
	// attempting to start the claimable service: the device must have been
	// claimed already to enable updates anyway, and checking for perms in
	// NewClaimableDispatcher needlessly prints a perms signature
	// verification error to the logs.
	if args.Device.TestMode {
		cleanup, err := startClaimedDevice(ctx, args)
		return nil, cleanup, err
	}

	// TODO(caprita): use some mechanism (a file lock or presence of entry
	// in mounttable) to ensure only one device manager is running in an
	// installation?
	mi := &impl.ManagerInfo{
		Pid: os.Getpid(),
	}
	if err := impl.SaveManagerInfo(filepath.Join(args.Device.ConfigState.Root, "device-manager"), mi); err != nil {
		return nil, nil, verror.New(errCantSaveInfo, ctx, err)
	}

	// If the device has not yet been claimed, start the mounttable and
	// claimable service and wait for it to be claimed.
	// Once a device is claimed, close any previously running servers and
	// start a new mounttable and device service.
	claimable, claimed := claim.NewClaimableDispatcher(ctx, impl.PermsDir(args.Device.ConfigState), args.Device.PairingToken, security.AllowEveryone())
	if claimable == nil {
		// Device has already been claimed, bypass claimable service
		// stage.
		cleanup, err := startClaimedDevice(ctx, args)
		return nil, cleanup, err
	}
	eps, stopClaimable, err := startClaimableDevice(ctx, claimable, args)
	if err != nil {
		return nil, nil, err
	}
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go waitToBeClaimedAndStartClaimedDevice(ctx, stopClaimable, claimed, stop, stopped, args)
	return eps, func() {
		close(stop)
		<-stopped
	}, nil
}

func startClaimableDevice(ctx *context.T, dispatcher rpc.Dispatcher, args Args) ([]naming.Endpoint, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	ctx = v23.WithListenSpec(ctx, args.Device.ListenSpec)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", dispatcher, options.LameDuckTimeout(30*time.Second))
	if err != nil {
		cancel()
		return nil, nil, err
	}
	shutdown := func() {
		cancel()
		ctx.Infof("Stopping claimable server...")
		<-server.Closed()
		ctx.Infof("Stopped claimable server.")
	}
	publicKey, err := v23.GetPrincipal(ctx).PublicKey().MarshalBinary()
	if err != nil {
		shutdown()
		return nil, nil, err
	}
	var eps []naming.Endpoint
	if proxy := args.Device.ListenSpec.Proxy; proxy != "" {
		for {
			status := server.Status()
			if err, ok := status.ProxyErrors[proxy]; ok && err == nil {
				eps = status.Endpoints
				break
			}
			ctx.Infof("Waiting for proxy address to appear...")
			<-status.Dirty
		}
	} else {
		eps = server.Status().Endpoints
	}
	ctx.Infof("Unclaimed device manager with public_key: %s", base64.URLEncoding.EncodeToString(publicKey))
	for _, ep := range eps {
		ctx.Infof("Unclaimed device manager endpoint: %v", ep.Name())
	}
	ctx.FlushLog()
	return eps, shutdown, nil
}

func waitToBeClaimedAndStartClaimedDevice(ctx *context.T, stopClaimable func(), claimed, stop <-chan struct{}, stopped chan<- struct{}, args Args) {
	// Wait for either the claimable service to complete, or be stopped
	defer close(stopped)
	select {
	case <-claimed:
		stopClaimable()
	case <-stop:
		stopClaimable()
		return
	}
	shutdown, err := startClaimedDevice(ctx, args)
	if err != nil {
		ctx.Errorf("Failed to start device service after it was claimed: %v", err)
		v23.GetAppCycle(ctx).Stop(ctx)
		return
	}
	defer shutdown()
	<-stop // Wait to be stopped
}

func startClaimedDevice(ctx *context.T, args Args) (func(), error) {
	ctx.Infof("Starting claimed device services...")
	permStore := pathperms.NewPathStore(ctx)
	permsDir := impl.PermsDir(args.Device.ConfigState)
	debugAuth, err := pathperms.NewHierarchicalAuthorizer(permsDir, permsDir, permStore)
	if err != nil {
		return nil, err
	}

	debugDisp := debuglib.NewDispatcher(debugAuth)

	ctx = v23.WithReservedNameDispatcher(ctx, debugDisp)

	ctx.Infof("Starting mount table...")
	mtName, stopMT, err := startMounttable(ctx, args.Namespace)
	if err != nil {
		ctx.Errorf("Failed to start mounttable service: %v", err)
		return nil, err
	} else {
		ctx.Infof("Started mount table.")
	}
	ctx.Infof("Starting device service...")
	stopDevice, err := startDeviceServer(ctx, args.Device, mtName, permStore)
	if err != nil {
		ctx.Errorf("Failed to start device service: %v", err)
		stopMT()
		return nil, err
	} else {
		ctx.Infof("Started device service.")
	}
	if args.MountGlobalNamespaceInLocalNamespace {
		ctx.Infof("Mounting %v ...", mtName)
		mountGlobalNamespaceInLocalNamespace(ctx, mtName)
		ctx.Infof("Mounted %v", mtName)
	}

	impl.InvokeCallback(ctx, args.Device.ConfigState.Name)

	ctx.Infof("Started claimed device services.")
	return func() {
		stopDevice()
		stopMT()
	}, nil
}

func startMounttable(ctx *context.T, n NamespaceArgs) (string, func(), error) {
	mtName, stopMT, err := mounttablelib.StartServers(ctx, n.ListenSpec, n.Name, n.Neighborhood, n.PermissionsFile, n.PersistenceDir, "mounttable")
	if err != nil {
		ctx.Errorf("mounttablelib.StartServers(%#v) failed: %v", n, err)
	} else {
		ctx.Infof("Local mounttable (%v) published as %q", mtName, n.Name)
	}
	return mtName, func() {
		ctx.Infof("Stopping mounttable...")
		stopMT()
		ctx.Infof("Stopped mounttable.")
	}, err
}

// startDeviceServer creates an rpc.Server and sets it up to server the Device service.
//
// ls: ListenSpec for the server
// configState: configuration for the Device service dispatcher
// mt: Object address of the mounttable
// dm: Name to publish the device service under
// testMode: whether the service is to be run in test mode
// restarted: callback invoked when the device manager is restarted.
//
// Returns:
// (1) Function to be called to force the service to shutdown
// (2) Any errors in starting the service (in which case, (1) will be nil)
func startDeviceServer(ctx *context.T, args DeviceArgs, mt string, permStore *pathperms.PathStore) (shutdown func(), err error) {
	ctx = v23.WithListenSpec(ctx, args.ListenSpec)
	wrapper := displib.NewDispatcherWrapper()
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, args.name(mt), wrapper)
	if err != nil {
		cancel()
		return nil, err
	}
	args.ConfigState.Name = server.Status().Endpoints[0].Name()

	dispatcher, dShutdown, err := impl.NewDispatcher(ctx, args.ConfigState, mt, args.TestMode, args.RestartCallback, permStore)
	if err != nil {
		cancel()
		<-server.Closed()
		return nil, err
	}

	shutdown = func() {
		// TODO(caprita): Capture the Dying state by feeding it back to
		// the dispatcher and exposing it in Status.
		ctx.Infof("Stopping device server...")
		cancel()
		<-server.Closed()
		dShutdown()
		ctx.Infof("Stopped device.")
	}
	wrapper.SetDispatcher(dispatcher)
	ctx.Infof("Device manager (%v) published as %v", args.ConfigState.Name, args.name(mt))
	return shutdown, nil
}

func mountGlobalNamespaceInLocalNamespace(ctx *context.T, localMT string) {
	ns := v23.GetNamespace(ctx)
	for _, root := range ns.Roots() {
		go func(r string) {
			for {
				err := ns.Mount(ctx, naming.Join(localMT, "global"), r, 0 /* forever */, naming.ServesMountTable(true))
				if err == nil {
					break
				}
				ctx.Infof("Failed to Mount global namespace: %v", err)
				time.Sleep(time.Second)
			}
		}(root)
	}
}
