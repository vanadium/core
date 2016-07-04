// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package control starts a number of syncbase instances and configures their
// connectivity based on a model.
//
// Usage:
//
//  // Create a new universe model.
//  u := model.GenerateUniverse(universeOpts)
//
//  // Run the universe model on a new controller.
//  c := NewController(controllerOpts)
//  err := c.Run(u)
//
//  // Then update universe however you want, and run it on the controller.
//  u = ...
//  err := c.Run(u)
//
//  // Pause all clients.
//  err := c.Pause()
//
//  // Run checker.
//  err := c.RunChecker(Checker)
//
//  // Resume all clients.
//  err := c.Resume()
//
//  // Tear it all down.
//  err := c.TearDown()

package control

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/runtime/protocols/vine"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/services/syncbase/longevity_tests/checker"
	"v.io/x/ref/services/syncbase/longevity_tests/client"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/longevity_tests/mounttabled"
	"v.io/x/ref/test/testutil"
)

var (
	mounttabledMain = gosh.RegisterFunc("mounttabledMain", mounttabled.Main)
)

// Controller handles starting and stopping syncbase instances according to
// model universes.
type Controller struct {
	// Context used to send RPCs to vine servers.
	ctx *context.T

	// Context used by Checkers to verify syncbase data.
	checkerCtx *context.T

	// identityProvider used to bless mounttable and all syncbase instances.
	identityProvider *testutil.IDProvider

	// Map of syncbase instance names to instances.
	instances   map[string]*instance
	instancesMu sync.Mutex // Locks instances.

	// Gosh command for mounttabled process.  Will be nil if mounttabled is not
	// running.
	mtCmd *gosh.Cmd

	// Namespace root of all instances.
	namespaceRoot string

	// Directory containing all instance working directories.
	rootDir string

	// Gosh shell used to spawn all processes.
	sh *gosh.Shell

	// Name of the controller's vine server.  Even though the controller only
	// acts as a client, it needs to run a vine server so that other servers
	// can be configured to allow RPCs from the controller.
	vineName string

	// Universe model that the controller is currently simulating.
	universe *model.Universe

	// vineShutdown shutdown the vine server and vine plugin.
	vineShutdown func()
}

// Opts is used to configure the Controller.
type Opts struct {
	// Directory that will contain all instance output.
	RootDir string

	// Send all child process output to stdout/stderr.  Usefull for tests.
	DebugOutput bool

	// Tests should pass their testing.TB instance, non-tests should pass nil.
	TB gosh.TB
}

// NewController returns a new Controller configured by opts.
func NewController(ctx *context.T, opts Opts) (*Controller, error) {
	sh := gosh.NewShell(opts.TB)
	if opts.DebugOutput {
		sh.PropagateChildOutput = true
	}
	c := &Controller{
		instances: make(map[string]*instance),
		rootDir:   opts.RootDir,
		sh:        sh,
		vineName:  "vine-controller",
	}
	if err := c.init(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// init initializes the controller.
func (c *Controller) init(ctx *context.T) error {
	// Initialize rootDir.
	if c.rootDir == "" {
		return fmt.Errorf("controller rootDir must not be empty")
	}
	if err := os.MkdirAll(c.rootDir, 0755); err != nil {
		return err
	}

	// Capture output of "vmstat".
	if err := c.startVmstat(); err != nil {
		return err
	}

	// Initialize identity provider.
	c.identityProvider = testutil.NewIDProvider("root")

	// Start mounttabled.
	if err := c.startMounttabled(); err != nil {
		return err
	}

	// Configure context for controller.
	ctx, err := c.setContextBlessings(ctx, "controller")
	if err != nil {
		return err
	}

	// Set proper namespace root.
	ctx, _, err = v23.WithNewNamespace(ctx, c.namespaceRoot)
	if err != nil {
		return err
	}

	// Modify ctx to use vine protocol.
	ctx, c.vineShutdown, err = vine.Init(ctx, c.vineName, security.AllowEveryone(), c.vineName, 0)
	if err != nil {
		return err
	}
	c.ctx = ctx

	// Configure context for checker.
	if checkerCtx, err := c.setContextBlessings(ctx, "checker"); err != nil {
		return err
	} else {
		c.checkerCtx = checkerCtx
	}

	return nil
}

// TearDown stops all instances and the root mounttable.
func (c *Controller) TearDown() error {
	var retErr error
	c.instancesMu.Lock()

	// Stop all instance clients.
	for _, inst := range c.instances {
		err := inst.stopClients()
		if err != nil && retErr == nil {
			retErr = err
		}
	}

	// Give syncbases a few seconds to finish processing any operations (like
	// syncing).
	time.Sleep(2 * time.Second)

	// Stop all instance syncbases.
	for _, inst := range c.instances {
		err := inst.stopSyncbase()
		if err != nil && retErr == nil {
			retErr = err
		}
	}
	c.instancesMu.Unlock()

	c.vineShutdown()
	c.sh.Cleanup()
	return retErr
}

// Run runs a mounttable and a number of syncbased instances with connectivity
// defined by the given model.
// Run can be called multiple times, and will update the running syncbases to
// reflect the model.  Any running instances not contained in the model will be
// stopped.  Any non-running instances contained in the model will be started.
func (c *Controller) Run(u *model.Universe) error {
	// allDevices keeps track of all devices in the model.
	allDevices := model.DeviceSet{}

	// Update all devices owned by each user.  If a device is not currently
	// running, it will be started.
	for _, user := range u.Users {
		allDevices = append(allDevices, user.Devices...)
		if err := c.updateInstancesForUser(user); err != nil {
			return err
		}
	}

	// Return an error if there are running devices that are not in the model.
	// Universes are not allowed to shrink.
	// TODO(nlacasse): Re-evaluate this rule once we have more examples of
	// longevity tests.
	c.instancesMu.Lock()
	for _, inst := range c.instances {
		inModel := false
		for _, d := range allDevices {
			if inst.name == d.Name {
				inModel = true
				break
			}
		}
		if !inModel {
			c.instancesMu.Unlock()
			return fmt.Errorf("device %s is running but not in given model %v", inst.name, u)
		}
	}
	c.instancesMu.Unlock()

	if err := c.updateTopology(u.Topology); err != nil {
		return err
	}

	c.universe = u
	return nil
}

// RunChecker runs the given Checker.
func (c *Controller) RunChecker(checker checker.Checker) error {
	return checker.Run(c.checkerCtx, *c.universe)
}

// PauseUniverse stops all clients but leaves syncbases running.
func (c *Controller) PauseUniverse() error {
	var retErr error
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()
	for _, inst := range c.instances {
		err := inst.stopClients()
		if err != nil && retErr == nil {
			retErr = err
		}
	}
	return retErr
}

// ResumeUniverse restarts all clients.
func (c *Controller) ResumeUniverse() error {
	var retErr error
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()
	for _, inst := range c.instances {
		err := inst.startClients()
		if err != nil && retErr == nil {
			retErr = err
		}
	}
	return retErr
}

func (c *Controller) updateInstancesForUser(user *model.User) error {
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()

	for _, d := range user.Devices {
		// If instance is already running, update it to match model.
		if inst, ok := c.instances[d.Name]; ok {
			inst.update(d)
			continue
		}
		// Otherwise start a new instance for the model.
		inst, err := c.newInstance(user, d)
		if err != nil {
			return err
		}
		if err := inst.start(c.ctx); err != nil {
			return err
		}
		c.instances[inst.name] = inst
	}
	return nil
}

func (c *Controller) updateTopology(top model.Topology) error {
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()

	// Start with empty topology map.
	behaviorMap := map[vine.PeerKey]vine.PeerBehavior{}

	// Set the reachable pairs based on the given topology.
	for clientDevice, serviceDevices := range top {
		clientInst, ok := c.instances[clientDevice.Name]
		if !ok {
			return fmt.Errorf("no instance associated with device %v", clientDevice)
		}

		for _, serviceDevice := range serviceDevices {
			serviceInst, ok := c.instances[serviceDevice.Name]
			if !ok {
				return fmt.Errorf("no instance associated with device %v", clientDevice)
			}
			peerKey := vine.PeerKey{
				Dialer:   clientInst.vineName,
				Acceptor: serviceInst.vineName,
			}
			behaviorMap[peerKey] = vine.PeerBehavior{Reachable: true}
		}
	}

	// Allow the controller to access every device.
	for _, inst := range c.instances {
		pk1 := vine.PeerKey{
			Dialer:   c.vineName,
			Acceptor: inst.vineName,
		}
		behaviorMap[pk1] = vine.PeerBehavior{Reachable: true}
		pk2 := vine.PeerKey{
			Dialer:   inst.vineName,
			Acceptor: c.vineName,
		}
		behaviorMap[pk2] = vine.PeerBehavior{Reachable: true}
	}

	// Set the behavior on each instance.
	for _, inst := range c.instances {
		vineClient := vine.VineClient(inst.vineName)
		if err := vineClient.SetBehaviors(c.ctx, behaviorMap); err != nil {
			return err
		}
	}

	// Set the controller's behavior.
	vineClient := vine.VineClient(c.vineName)
	if err := vineClient.SetBehaviors(c.ctx, behaviorMap); err != nil {
		return err
	}

	return nil
}

// newInstance creates a new instance corresponding to the given device model.
func (c *Controller) newInstance(user *model.User, d *model.Device) (*instance, error) {
	wd := filepath.Join(c.rootDir, "instances", d.Name)
	if err := os.MkdirAll(wd, 0755); err != nil {
		return nil, err
	}

	// Create persistent principal for syncbase instance on device.
	credsDir := filepath.Join(wd, "credentials")
	instancePrincipal, err := vsecurity.CreatePersistentPrincipal(credsDir, nil)
	if err != nil {
		return nil, err
	}

	// Bless instance's principal with name root:u:<user>:<device>.
	blessingName := strings.Join([]string{"u", user.Name, d.Name}, security.ChainSeparator)
	if err := c.identityProvider.Bless(instancePrincipal, blessingName); err != nil {
		return nil, err
	}

	clients := make([]client.Client, len(d.Clients))
	for i, clientType := range d.Clients {
		f := LookupClient(clientType)
		if f == nil {
			return nil, fmt.Errorf("no client for type %q", clientType)
		}
		clients[i] = f()
	}

	vineName := "vine-" + d.Name
	inst := &instance{
		clients:       clients,
		credsDir:      credsDir,
		databases:     d.Databases,
		name:          d.Name,
		namespaceRoot: c.namespaceRoot,
		principal:     instancePrincipal,
		vineName:      vineName,
		sh:            c.sh,
		wd:            wd,
	}
	return inst, nil
}

// startMounttabled starts mounttabled and sets c.namespaceRoot with
// mounttabled's endpoint.
func (c *Controller) startMounttabled() error {
	mtDir := filepath.Join(c.rootDir, "mounttabled")
	mtCreds := filepath.Join(mtDir, "credentials")
	if err := os.MkdirAll(mtCreds, 0755); err != nil {
		return err
	}

	// Create blessings for mounttable.
	p, err := vsecurity.CreatePersistentPrincipal(mtCreds, nil)
	if err != nil {
		return err
	}
	if err = c.identityProvider.Bless(p, "mounttable"); err != nil {
		return err
	}

	// Start mounttable and set namespace root.
	opts := mounttablelib.Opts{}
	c.mtCmd = c.sh.FuncCmd(mounttabledMain, opts)
	c.mtCmd.Args = append(c.mtCmd.Args,
		"--log_dir="+mtDir,
		"--v23.credentials="+mtCreds,
		"--vpath=mounttable=3,namespace=3",
	)
	c.mtCmd.Start()
	vars := c.mtCmd.AwaitVars("NAME")
	name := vars["NAME"]
	if name == "" {
		return fmt.Errorf("empty NAME variable returned by mounttable")
	}
	c.namespaceRoot = name
	return nil
}

// setContextBlessings returns a new context based off the given one, with a new
// principal that is blessed by the controller's identity provider.
func (c *Controller) setContextBlessings(ctx *context.T, blessingName string) (*context.T, error) {
	// Create a principal blessed by the identity provider.
	p := testutil.NewPrincipal()
	if err := c.identityProvider.Bless(p, blessingName); err != nil {
		return nil, err
	}
	newCtx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		return nil, err
	}

	return newCtx, nil
}

// startVmstat writes a timestamp and vmstat output to a log file every second.
func (c *Controller) startVmstat() error {
	logFilePath := filepath.Join(c.rootDir, "vmstat.log")
	// Open logfile in append-mode.
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	cmd := c.sh.Cmd("bash", "-c", "vmstat -n -w 1 | awk '{ print strftime(\"%c, %Z\"), $0; fflush(); }'")
	outPipe := cmd.StdoutPipe()

	go func() {
		io.Copy(logFile, outPipe)
		logFile.Close()
	}()

	cmd.Start()
	return nil
}
