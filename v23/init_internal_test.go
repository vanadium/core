// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v23

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/flow"
	"v.io/v23/namespace"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
)

// Create a mock RuntimeFactory.
func MockFactory(ctx *context.T) (Runtime, *context.T, Shutdown, error) {
	return &mockRuntime{}, nil, func() {}, nil
}

type mockRuntime struct{}

func (*mockRuntime) Init(ctx *context.T) error { return nil }
func (*mockRuntime) WithPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	return nil, nil
}
func (*mockRuntime) GetPrincipal(ctx *context.T) security.Principal { return nil }
func (*mockRuntime) WithNewClient(ctx *context.T, opts ...rpc.ClientOpt) (*context.T, rpc.Client, error) {
	return nil, nil, nil
}
func (*mockRuntime) GetClient(ctx *context.T) rpc.Client { return nil }
func (*mockRuntime) WithNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	return nil, nil, nil
}
func (*mockRuntime) GetNamespace(ctx *context.T) namespace.T     { return nil }
func (*mockRuntime) GetListenSpec(ctx *context.T) rpc.ListenSpec { return rpc.ListenSpec{} }

func (*mockRuntime) GetPermissionsSpec(ctx *context.T) access.PermissionsSpec {
	return access.PermissionsSpec{}
}
func (*mockRuntime) GetRequestID(ctx *context.T) uuid.UUID {
	return uuid.UUID{}
}
func (*mockRuntime) WithBackgroundContext(ctx *context.T) *context.T { return nil }
func (*mockRuntime) GetBackgroundContext(ctx *context.T) *context.T  { return nil }

func (*mockRuntime) NewDiscovery(ctx *context.T) (discovery.T, error) { return nil, nil }

func (*mockRuntime) WithReservedNameDispatcher(ctx *context.T, d rpc.Dispatcher) *context.T {
	return nil
}

func (*mockRuntime) GetReservedNameDispatcher(ctx *context.T) rpc.Dispatcher    { return nil }
func (*mockRuntime) WithListenSpec(ctx *context.T, _ rpc.ListenSpec) *context.T { return ctx }
func (*mockRuntime) NewFlowManager(ctx *context.T, channelTimeout time.Duration) (flow.Manager, error) {
	return nil, nil
}
func (*mockRuntime) WithNewServer(ctx *context.T, name string, object interface{}, auth security.Authorizer, opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {
	return nil, nil, nil
}
func (*mockRuntime) WithNewDispatchingServer(ctx *context.T, name string, disp rpc.Dispatcher, opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {
	return nil, nil, nil
}

func TestPanicOnInitWithNoRuntimeFactory(t *testing.T) {
	clear()
	// Calling Init without a registered RuntimeFactory should panic.
	catcher := func() {
		r := recover()
		if r == nil {
			t.Fatalf("recover returned nil")
		}
		err := r.(error)
		str := err.Error()
		if !strings.Contains(str, "No RuntimeFactory has been registered") {
			t.Fatalf("unexpected error: %s", str)
		}
	}
	defer catcher()
	Init()
}

func TestInitWithRegisteredRuntimeFactory(t *testing.T) {
	clear()
	// Calling v23.Init with a RuntimeFactory should succeed.
	RegisterRuntimeFactory(MockFactory)
	Init()
}

func TestPanicOnSecondRuntimeFactoryRegistration(t *testing.T) {
	clear()
	// Registering multiple RuntimeFactory should fail.
	RegisterRuntimeFactory(MockFactory)
	catcher := func() {
		r := recover()
		if r == nil {
			t.Fatalf("recover returned nil")
		}
		str := r.(string)
		if !strings.Contains(str, "A RuntimeFactory has already been registered") {
			t.Fatalf("unexpected error: %s", str)
		}
	}
	defer catcher()
	RegisterRuntimeFactory(MockFactory)
}

func TestPanicOnSecondInit(t *testing.T) {
	clear()
	// Calling Init again before calling shutdown should fail.
	RegisterRuntimeFactory(MockFactory)
	Init()
	catcher := func() {
		r := recover()
		if r == nil {
			t.Fatalf("recover returned nil")
		}
		err := r.(error)
		str := err.Error()
		if !strings.Contains(str, "A runtime has already been initialized") {
			t.Fatalf("unexpected error: %s", str)
		}
	}
	defer catcher()
	Init()
}

func TestSecondInitAfterShutdown(t *testing.T) {
	clear()
	// Calling Init, shutting down, and then calling Init again should succeed.
	RegisterRuntimeFactory(MockFactory)
	_, shutdown := Init()
	shutdown()
	Init()
}

// for tests only
func clear() {
	initState.mu.Lock()
	initState.runtime = nil
	initState.runtimeStack = ""
	initState.runtimeFactory = nil
	initState.runtimeFactoryStack = ""
	initState.mu.Unlock()
}
