// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"v.io/v23/context"
	"v.io/v23/services/device"
	"v.io/v23/verror"
)

var errPIDIsNotInteger = verror.Register(pkgPath+".errPIDIsNotInteger", verror.NoRetry, "{1:}{2:} __debug/stats/system/pid isn't an integer{:_}")

type pidInstanceDirPair struct {
	instanceDir string
	pid         int
}

type reaper struct {
	c       chan pidInstanceDirPair
	stopped chan struct{}
	ctx     *context.T
}

var stashedPidMap map[string]int

func newReaper(ctx *context.T, root string, appRunner *appRunner) (*reaper, error) {
	pidMap, err := findAllTheInstances(ctx, root)

	// Used only by the testing code that verifies that all processes
	// have been shutdown.
	stashedPidMap = pidMap
	if err != nil {
		return nil, err
	}

	r := &reaper{
		c:       make(chan pidInstanceDirPair),
		stopped: make(chan struct{}),
		ctx:     ctx,
	}
	// Restart daemon jobs if they're not running (say because the machine crashed.)
	go r.processStatusPolling(ctx, pidMap, appRunner)
	return r, nil
}

func markNotRunning(ctx *context.T, runner *appRunner, idir string) error {
	if err := runner.principalMgr.StopServing(idir); err != nil {
		return fmt.Errorf("StopServing(%v) failed: %v", idir, err)
	}

	if instanceStateIs(idir, device.InstanceStateNotRunning) {
		return nil
	}
	// If the app is not in state Running, it is likely in the process of
	// being launched or killed when the reaper poll finds the process dead.
	// Do not attempt a restart in this case.
	return transitionInstance(idir, device.InstanceStateRunning, device.InstanceStateNotRunning)
}

func isAlive(ctx *context.T, pid int) bool {
	switch err := syscall.Kill(pid, 0); err {
	case syscall.ESRCH:
		// No such PID.
		return false
	case nil, syscall.EPERM:
		return true
	default:
		// The kill system call manpage says that this can only happen
		// if the kernel claims that 0 is an invalid signal.  Only a
		// deeply confused kernel would say this so just give up.
		ctx.Panicf("processStatusPolling: unanticipated result from sys.Kill: %v", err)
		return true
	}
}

// processStatusPolling polls for the continued existence of a set of
// tracked pids. TODO(rjkroege): There are nicer ways to provide this
// functionality. For example, use the kevent facility in darwin or
// replace init. See http://www.incenp.org/dvlpt/wait4.html for
// inspiration.
func (r *reaper) processStatusPolling(ctx *context.T, trackedPids map[string]int, appRunner *appRunner) {
	poll := func(ctx *context.T) {
		for idir, pid := range trackedPids {
			if !isAlive(ctx, pid) {
				ctx.Infof("processStatusPolling discovered %v (pid %d) ended", idir, pid)
				if err := markNotRunning(ctx, appRunner, idir); err != nil {
					ctx.Errorf("markNotRunning failed: %v", err)
				} else {
					go appRunner.restartAppIfNecessary(ctx, idir)
				}
				delete(trackedPids, idir)
			} else {
				ctx.VI(2).Infof("processStatusPolling saw live pid: %d", pid)
				// The task exists and is running under the same uid as
				// the device manager or the task exists and is running
				// under a different uid as would be the case if invoked
				// via suidhelper. In this case do, nothing.

				// This implementation cannot detect if a process exited
				// and was replaced by an arbitrary non-Vanadium process
				// within the polling interval.
				// TODO(rjkroege): Probe the appcycle service of the app
				// to confirm that its pid is valid.

				// TODO(rjkroege): if we can't connect to the app here via
				// the appcycle manager, the app was probably started under
				// a different agent and cannot be managed. Perhaps we should
				// then kill the app and restart it?
			}
		}
	}

	for {
		select {
		case p := <-r.c:
			switch {
			case p.instanceDir == "":
				return // Shutdown.
			case p.pid == -1: // stop watching this instance
				delete(trackedPids, p.instanceDir)
				poll(ctx)
			case p.pid == -2: // kill the process
				info, err := loadInstanceInfo(ctx, p.instanceDir)
				if err != nil {
					ctx.Errorf("loadInstanceInfo(%v) failed: %v", p.instanceDir, err)
					continue
				}
				if info.Pid <= 0 {
					ctx.Errorf("invalid pid in %v: %v", p.instanceDir, info.Pid)
					continue
				}
				if err := suidHelper.terminatePid(ctx, info.Pid, nil, nil); err != nil {
					ctx.Errorf("Failure to kill pid %d: %v", info.Pid, err)
				}
			case p.pid < 0:
				ctx.Panicf("invalid pid %v", p.pid)
			default:
				trackedPids[p.instanceDir] = p.pid
				poll(ctx)
			}
		case <-time.After(time.Second):
			// Poll once / second.
			// TODO(caprita): Configure this to use timekeeper to
			// allow simulated time injection for testing.
			poll(ctx)
		}
	}
}

func (r *reaper) sendCmd(idir string, pid int) {
	select {
	case r.c <- pidInstanceDirPair{instanceDir: idir, pid: pid}:
	case <-r.stopped:
	}
}

// startWatching begins watching process pid's state. This routine
// assumes that pid already exists. Since pid is delivered to the device
// manager by RPC callback, this seems reasonable.
func (r *reaper) startWatching(idir string, pid int) {
	r.sendCmd(idir, pid)
}

// stopWatching stops watching process pid's state.
func (r *reaper) stopWatching(idir string) {
	r.sendCmd(idir, -1)
}

// forciblySuspend terminates the process pid.
func (r *reaper) forciblySuspend(idir string) {
	r.sendCmd(idir, -2)
}

// shutdown stops the reaper.
func (r *reaper) shutdown() {
	r.sendCmd("", 0)
	close(r.stopped)
}

type pidErrorTuple struct {
	ipath string
	pid   int
	err   error
}

// processStatusViaKill updates the status based on sending a kill signal
// to the process. This assumes that most processes on the system are
// likely to be managed by the device manager and a live process is not
// responsive because the agent has been restarted rather than being
// created through a different means.
func processStatusViaKill(ctx *context.T, c chan<- pidErrorTuple, instancePath string, info *instanceInfo, state device.InstanceState) {
	pid := info.Pid

	switch err := syscall.Kill(pid, 0); err {
	case syscall.ESRCH:
		// No such PID.
		if err := transitionInstance(instancePath, state, device.InstanceStateNotRunning); err != nil {
			ctx.Errorf("transitionInstance(%s,%s,%s) failed: %v", instancePath, state, device.InstanceStateNotRunning, err)
		}
		// We only want to restart apps that were Running or Launching.
		if state == device.InstanceStateLaunching || state == device.InstanceStateRunning {
			c <- pidErrorTuple{ipath: instancePath, pid: pid, err: err}
		}
	case nil, syscall.EPERM:
		// The instance was found to be running, so update its state.
		if err := transitionInstance(instancePath, state, device.InstanceStateRunning); err != nil {
			ctx.Errorf("transitionInstance(%s,%v, %v) failed: %v", instancePath, state, device.InstanceStateRunning, err)
		}
		ctx.VI(0).Infof("perInstance go routine for %v ending", instancePath)
		c <- pidErrorTuple{ipath: instancePath, pid: pid}
	}
}

func perInstance(ctx *context.T, instancePath string, c chan<- pidErrorTuple, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx.Infof("Instance: %v", instancePath)
	state, err := getInstanceState(instancePath)
	switch state {
	// Ignore apps already in deleted and not running states.
	case device.InstanceStateNotRunning:
		return
	case device.InstanceStateDeleted:
		return
	// If the app was updating, it means it was already not running, so just
	// update its state back to not running.
	case device.InstanceStateUpdating:
		if err := transitionInstance(instancePath, state, device.InstanceStateNotRunning); err != nil {
			ctx.Errorf("transitionInstance(%s,%s,%s) failed: %v", instancePath, state, device.InstanceStateNotRunning, err)
		}
		return
	}
	ctx.VI(2).Infof("perInstance firing up on %s", instancePath)

	// Read the instance data.
	info, err := loadInstanceInfo(ctx, instancePath)
	if err != nil {
		ctx.Errorf("loadInstanceInfo failed: %v", err)
		// Something has gone badly wrong.
		// TODO(rjkroege,caprita): Consider removing the instance or at
		// least set its state to something indicating error?
		c <- pidErrorTuple{err: err, ipath: instancePath}
		return
	}

	// Remaining states: Launching, Running, Dying. Of these,
	// daemon mode will restart Launching and Running if the process
	// is not alive.
	processStatusViaKill(ctx, c, instancePath, info, state)
}

// Digs through the directory hierarchy.
func findAllTheInstances(ctx *context.T, root string) (map[string]int, error) {
	paths, err := filepath.Glob(filepath.Join(root, "app*", "installation*", "instances", "instance*"))
	if err != nil {
		return nil, err
	}

	pidmap := make(map[string]int)
	pidchan := make(chan pidErrorTuple, len(paths))
	var wg sync.WaitGroup

	for _, pth := range paths {
		wg.Add(1)
		go perInstance(ctx, pth, pidchan, &wg)
	}
	wg.Wait()
	close(pidchan)

	for p := range pidchan {
		if p.err != nil {
			ctx.Errorf("instance at %s had an error: %v", p.ipath, p.err)
		}
		if p.pid > 0 {
			pidmap[p.ipath] = p.pid
		}
	}
	return pidmap, nil
}

// RunningChildrenProcesses uses the reaper to verify that a test has
// successfully shut down all processes.
func RunningChildrenProcesses() bool {
	return len(stashedPidMap) > 0
}
