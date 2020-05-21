// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/rpc"
	"v.io/v23/security"
)

// SetClientFactory can be used to inject a mock Client implementation
// into the context.  When v23.WithNewClient is called passed function
// will be invoked.
func SetClientFactory(ctx *context.T, factory func(ctx *context.T, opts ...rpc.ClientOpt) rpc.Client) *context.T {
	client := factory(ctx)
	ctx = context.WithValue(ctx, clientFactoryKey, factory)
	return context.WithValue(ctx, clientKey, client)
}
func (r *Runtime) WithNewClient(ctx *context.T, opts ...rpc.ClientOpt) (*context.T, rpc.Client, error) {
	factory, ok := ctx.Value(clientFactoryKey).(func(ctx *context.T, opts ...rpc.ClientOpt) rpc.Client)
	if !ok {
		panic("Calling WithNewClient on the fake runtime, but no factory has been set.")
	}
	client := factory(ctx, opts...)
	return context.WithValue(ctx, clientKey, client), client, nil
}
func (r *Runtime) GetClient(ctx *context.T) rpc.Client {
	c, _ := ctx.Value(clientKey).(rpc.Client)
	return c
}

func (r *Runtime) GetListenSpec(ctx *context.T) rpc.ListenSpec {
	ls, _ := ctx.Value(listenSpecKey).(rpc.ListenSpec)
	return ls
}

func (r *Runtime) WithListenSpec(ctx *context.T, ls rpc.ListenSpec) *context.T {
	return context.WithValue(ctx, listenSpecKey, ls)
}

// SetFlowManagerFactory can be used to inject a mock FlowManager
// implementation into the context.  When v23.NewFlowManager is called
// passed function will be invoked.
func SetFlowManagerFactory(ctx *context.T, factory func(ctx *context.T, channelTimeout time.Duration) flow.Manager) *context.T {
	return context.WithValue(ctx, flowFactoryKey, factory)
}

func (r *Runtime) NewFlowManager(ctx *context.T, channelTimeout time.Duration) (flow.Manager, error) {

	factory, ok := ctx.Value(flowFactoryKey).(func(ctx *context.T, channelTimeout time.Duration) flow.Manager)
	if !ok {
		panic("Calling NewFlowManager on the fake runtime, but no factory has been set.")
	}
	return factory(ctx, channelTimeout), nil
}

// SetServerFactory can be used to inject a mock Server implementation into the context.
// When v23.WithNewServer is called the passed function will be invoked.
func SetServerFactory(ctx *context.T, factory func(*context.T, string, interface{}, security.Authorizer, ...rpc.ServerOpt) (*context.T, rpc.Server)) *context.T {
	return context.WithValue(ctx, serverFactoryKey, factory)
}

func (r *Runtime) WithNewServer(ctx *context.T, name string, object interface{}, auth security.Authorizer, opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {

	factory, ok := ctx.Value(serverFactoryKey).(func(*context.T, string, interface{}, security.Authorizer, ...rpc.ServerOpt) (*context.T, rpc.Server))
	if !ok {
		panic("Calling WithNewServer on the fake runtime, but no factory has been set.")
	}
	ctx, server := factory(ctx, name, object, auth, opts...)
	return ctx, server, nil
}

func (r *Runtime) WithNewDispatchingServer(ctx *context.T, name string, disp rpc.Dispatcher, opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {
	panic("unimplemented")
}
