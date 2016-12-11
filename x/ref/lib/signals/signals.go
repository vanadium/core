// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package signals implements utilities for managing process shutdown with
// support for signal-handling.
package signals

// TODO(caprita): Rename the function to Shutdown() and the package to shutdown
// since it's not just signals anymore.

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"v.io/v23"
	"v.io/v23/context"
)

type stopSignal string

func (stopSignal) Signal()          {}
func (s stopSignal) String() string { return string(s) }

const (
	STOP               = stopSignal("")
	DoubleStopExitCode = 1
)

// TODO(caprita): Needless to say, this is a hack.  The motivator was getting
// the device manager (run by the security agent) to shut down cleanly when the
// process group containing both the agent and device manager receives a signal
// (and the agent propagates that signal to the child).  We should be able to
// finesse this by demonizing the device manager and/or being smarter about how
// and when the agent sends the signal to the child.

// SameSignalTimeWindow specifies the time window during which multiple
// deliveries of the same signal are counted as one signal.  If set to zero, no
// such de-duping occurs.  This is useful in situations where a process receives
// a signal explicitly sent by its parent when the parent receives the signal,
// but also receives it independently by virtue of being part of the same
// process group.
//
// This is a variable, so that it can be set appropriately.  Note, there is no
// locking around it, the assumption being that it's set during initialization
// and never reset afterwards.
var SameSignalTimeWindow time.Duration

// Default returns a set of platform-specific signals that applications are
// encouraged to listen on.
func Default() []os.Signal {
	return []os.Signal{syscall.SIGTERM, syscall.SIGINT, STOP}
}

// ShutdownOnSignals registers signal handlers for the specified signals, or, if
// none are specified, the default signals.  The first signal received will be
// made available on the returned channel; upon receiving a second signal, the
// process will exit.
func ShutdownOnSignals(ctx *context.T, signals ...os.Signal) <-chan os.Signal {
	if len(signals) == 0 {
		signals = Default()
	}
	// At least a buffer of length two so that we don't drop the first two
	// signals we get on account of the channel being full.
	ch := make(chan os.Signal, 2)
	sawStop := false
	var signalsNoStop []os.Signal
	for _, s := range signals {
		switch s {
		case STOP:
			if !sawStop {
				sawStop = true
				if ctx != nil {
					stopWaiter := make(chan string, 1)
					v23.GetAppCycle(ctx).WaitForStop(ctx, stopWaiter)
					go func() {
						for {
							ch <- stopSignal(<-stopWaiter)
						}
					}()
				}
			}
		default:
			signalsNoStop = append(signalsNoStop, s)
		}
	}
	if len(signalsNoStop) > 0 {
		signal.Notify(ch, signalsNoStop...)
	}
	// At least a buffer of length one so that we don't block on ret <- sig.
	ret := make(chan os.Signal, 1)
	go func() {
		// First signal received.
		sig := <-ch
		sigTime := time.Now()
		ret <- sig
		// Wait for a second signal, and force an exit if the process is
		// still executing cleanup code.
		for {
			secondSig := <-ch
			// If signal de-duping is enabled, ignore the signal if
			// it's the same signal and has occured within the
			// specified time window.
			if SameSignalTimeWindow <= 0 || secondSig.String() != sig.String() || sigTime.Add(SameSignalTimeWindow).Before(time.Now()) {
				os.Exit(DoubleStopExitCode)
			}
		}
	}()
	return ret
}
