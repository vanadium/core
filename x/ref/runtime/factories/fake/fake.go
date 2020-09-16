// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fake implements a fake RuntimeFactory, useful in tests for mocking
// out certain components.
package fake

// TODO(mattr): Make a more complete, but still fake, implementation.

import (
	"sync"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/local" //nolint:golint
	_ "v.io/x/ref/runtime/protocols/tcp"   //nolint:golint
	_ "v.io/x/ref/runtime/protocols/ws"    //nolint:golint
	_ "v.io/x/ref/runtime/protocols/wsh"   //nolint:golint
)

var (
	runtimeInfo struct {
		mu       sync.Mutex
		runtime  v23.Runtime  // GUARDED_BY mu
		ctx      *context.T   // GUARDED_BY mu
		shutdown v23.Shutdown // GUARDED_BY mu
	}
)

func init() {
	v23.RegisterRuntimeFactory(Init)
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
}

func Init(ctx *context.T) (v23.Runtime, *context.T, v23.Shutdown, error) {
	err := logger.Global().ConfigureFromFlags()
	if err != nil && !logger.IsAlreadyConfiguredError(err) {
		return nil, nil, nil, err
	}

	runtimeInfo.mu.Lock()
	defer runtimeInfo.mu.Unlock()
	if runtimeInfo.runtime != nil {
		shutdown := func() {
			runtimeInfo.mu.Lock()
			runtimeInfo.shutdown()
			runtimeInfo.runtime = nil
			runtimeInfo.ctx = nil
			runtimeInfo.shutdown = nil
			runtimeInfo.mu.Unlock()
		}
		return runtimeInfo.runtime, runtimeInfo.ctx, shutdown, nil
	}
	return new(ctx)
}

// InjectRuntime allows packages to inject whichever runtime, ctx, and shutdown.
// This allows a package that needs different runtimes in tests to swap them as needed.
// The injected runtime will be valid until the shutdown returned from v23.Init is called.
func InjectRuntime(runtime v23.Runtime, ctx *context.T, shutdown v23.Shutdown) {
	runtimeInfo.mu.Lock()
	runtimeInfo.runtime = runtime
	runtimeInfo.ctx = ctx
	runtimeInfo.shutdown = shutdown
	runtimeInfo.mu.Unlock()
}
