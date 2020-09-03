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
	"sync"
	"syscall"
	"time"

	"v.io/v23/context"
)

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

const (
	DoubleStopExitCode = 1
)

// Default returns a set of platform-specific signals that applications are
// encouraged to listen on.
func Default() []os.Signal {
	return []os.Signal{syscall.SIGTERM, syscall.SIGINT}
}

// ShutdownOnSignals registers signal handlers for the specified signals, or, if
// none are specified, the default signals.  The first signal received will be
// made available on the returned channel; upon receiving a second signal, the
// process will exit if that signal differs from the first, or if the same it
// arrives more than a second after the first.
func ShutdownOnSignals(ctx *context.T, signals ...os.Signal) <-chan os.Signal {
	if len(signals) == 0 {
		signals = Default()
	}
	// At least a buffer of length two so that we don't drop the first two
	// signals we get on account of the channel being full.
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, signals...)

	// At least a buffer of length one so that we don't block on ret <- sig.
	ret := make(chan os.Signal, 1)
	go func() {
		// First signal received.
		var sig os.Signal
		var sigTime time.Time
		select {
		case sig = <-ch:
			sigTime = time.Now()
			ret <- sig
		case <-ctx.Done():
			ret <- ContextDoneSignal(ctx.Err().Error())
			return
		}
		// Wait for a second signal, and force an exit if the process is
		// still executing cleanup code.
		for {
			secondSig := <-ch
			// If signal de-duping is enabled, ignore the signal if
			// it's the same signal and has occurred within the
			// specified time window.
			if SameSignalTimeWindow <= 0 || secondSig.String() != sig.String() || sigTime.Add(SameSignalTimeWindow).Before(time.Now()) {
				os.Exit(DoubleStopExitCode)
			}
		}
	}()
	return ret
}

// Handler represents a signal handler that can be used to wait for signal
// reception or context cancelation as per NotifyWithCancel. In addition
// it can be used to register additional cancel functions to be invoked
// on signal reception or context cancelation.
type Handler struct {
	ctx        *context.T
	cancel     context.CancelFunc
	ch         <-chan os.Signal
	mu         sync.Mutex
	cancelList []func() // GUARDED_BY(mu)
}

// RegisterCancel registers one or more cancel functions to be invoked
// when a signal is received or the original context is canceled.
func (h *Handler) RegisterCancel(fns ...func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cancelList = append(h.cancelList, fns...)
}

func (h *Handler) cancelAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cancel()
	for _, cancel := range h.cancelList {
		cancel()
	}
}

// WaitForSignal will wait for a signal to be received. Context cancelation
// is translated into a ContextDoneSignal signal.
func (h *Handler) WaitForSignal() os.Signal {
	select {
	case sig := <-h.ch:
		h.cancelAll()
		return sig
	case <-h.ctx.Done():
		h.cancelAll()
		return ContextDoneSignal(h.ctx.Err().Error())
	}
}

// ShutdownOnSignalsWithCancel is like ShutdownOnSignals except it forks the
// supplied context to obtain a cancel function which is called by the returned
// function when a signal is received. The returned function can be called to
// wait for the signal to be received or for the context to be canceled.
// Typical usage would be:
//
//    func main() {
// 	    ctx, shutdown := v23.Init()
//      defer shutdown()
//      ctx, handler := ShutdownOnSignalsWithCancel(ctx)
//      defer handler.WaitForSignal()
//
//      _, srv, err := v23.WithNewServer(ctx, ...)
//
//    }
//
// waitForInterrupt will wait for a signal to be received at which point it
// will cancel the context and thus the server created by WithNewServer to
// initiate its internal shutdown. The deferred shutdown returned by v23.Init()
// will then wait for that the server to complete its shutdown.
// Canceling the context is treated as receipt of a custom signal,
// ContextDoneSignal, in terms of its returns value.
func ShutdownOnSignalsWithCancel(ctx *context.T, signals ...os.Signal) (*context.T, *Handler) {
	ctx, cancel := context.WithCancel(ctx)
	handler := &Handler{
		ctx:    ctx,
		cancel: cancel,
		ch:     ShutdownOnSignals(ctx, signals...),
	}
	return ctx, handler
}

type ContextDoneSignal string

func (ContextDoneSignal) Signal()          {}
func (s ContextDoneSignal) String() string { return string(s) }
