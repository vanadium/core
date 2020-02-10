// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package appcycle

import (
	"fmt"
	"os"
	"sync"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"

	public "v.io/v23/services/appcycle"
)

var (
	// We may want to be able to override these (e.g. for testing), hence
	// the variables.
	ForceStopExitCode     = v23.ForceStopExitCode
	UnhandledStopExitCode = v23.UnhandledStopExitCode
)

type AppCycle struct {
	sync.RWMutex
	waiters      []chan<- string
	taskTrackers []chan<- v23.Task
	task         v23.Task
	shutDown     bool
	disp         *invoker
}

type invoker struct {
	ac *AppCycle
}

func New() *AppCycle {
	ac := new(AppCycle)
	ac.disp = &invoker{ac}
	return ac
}

func (m *AppCycle) Shutdown() {
	m.Lock()
	defer m.Unlock()
	if m.shutDown {
		return
	}
	m.shutDown = true
	for _, t := range m.taskTrackers {
		close(t)
	}
	m.taskTrackers = nil
}

func (m *AppCycle) stop(ctx *context.T, msg string) {
	ctx.Infof("stop(%v)", msg)
	defer ctx.Infof("stop(%v) done", msg)
	m.RLock()
	defer m.RUnlock()
	if len(m.waiters) == 0 {
		ctx.Infof("Unhandled stop. Exiting.")
		os.Exit(UnhandledStopExitCode)
	}
	for _, w := range m.waiters {
		select {
		case w <- msg:
		default:
		}
	}
}

func (m *AppCycle) Stop(ctx *context.T) {
	m.stop(ctx, v23.LocalStop)
}

func (*AppCycle) ForceStop(ctx *context.T) {
	os.Exit(ForceStopExitCode)
}

func (m *AppCycle) WaitForStop(_ *context.T, ch chan<- string) {
	m.Lock()
	defer m.Unlock()
	m.waiters = append(m.waiters, ch)
}

func (m *AppCycle) TrackTask(ch chan<- v23.Task) {
	m.Lock()
	defer m.Unlock()
	if m.shutDown {
		close(ch)
		return
	}
	m.taskTrackers = append(m.taskTrackers, ch)
}

func (m *AppCycle) advanceTask(progress, goal int32) {
	m.Lock()
	defer m.Unlock()
	m.task.Goal += goal
	m.task.Progress += progress
	for _, t := range m.taskTrackers {
		select {
		case t <- m.task:
		default:
			// TODO(caprita): Make it such that the latest task
			// update is always added to the channel even if channel
			// is full.  One way is to pull an element from t and
			// then re-try the push.
		}
	}
}

func (m *AppCycle) AdvanceGoal(delta int32) {
	if delta <= 0 {
		return
	}
	m.advanceTask(0, delta)
}

func (m *AppCycle) AdvanceProgress(delta int32) {
	if delta <= 0 {
		return
	}
	m.advanceTask(delta, 0)
}

func (m *AppCycle) Remote() interface{} {
	return public.AppCycleServer(m.disp)
}

func (d *invoker) Stop(ctx *context.T, call public.AppCycleStopServerCall) error {
	blessings, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("AppCycle Stop request from %v", blessings)
	// The size of the channel should be reasonably sized to expect not to
	// miss updates while we're waiting for the stream to unblock.
	ch := make(chan v23.Task, 10)
	d.ac.TrackTask(ch)
	// TODO(caprita): Include identity of Stop issuer in message.
	d.ac.stop(ctx, v23.RemoteStop)
	for {
		task, ok := <-ch
		if !ok {
			// Channel closed, meaning process shutdown is imminent.
			break
		}
		actask := public.Task{Progress: task.Progress, Goal: task.Goal}
		ctx.Infof("AppCycle Stop progress %d/%d", task.Progress, task.Goal)
		call.SendStream().Send(actask)
	}
	ctx.Infof("AppCycle Stop done")
	return nil
}

func (d *invoker) ForceStop(ctx *context.T, _ rpc.ServerCall) error {
	d.ac.ForceStop(ctx)
	return fmt.Errorf("ForceStop should not reply as the process should be dead")
}
