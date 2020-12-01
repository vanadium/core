// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"github.com/google/uuid"
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/namespace"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/ref/internal/logger"
	tnaming "v.io/x/ref/runtime/internal/testing/mocks/naming"
	"v.io/x/ref/test/testutil"
)

type contextKey int

const (
	clientKey = contextKey(iota)
	principalKey
	backgroundKey
	listenSpecKey

	clientFactoryKey
	flowFactoryKey
	serverFactoryKey
)

type Runtime struct {
	ns namespace.T
}

func new(ctx *context.T) (*Runtime, *context.T, v23.Shutdown, error) {
	ctx = context.WithValue(ctx, principalKey, testutil.NewPrincipal("fake"))
	ctx = context.WithLogger(ctx, logger.Global())
	return &Runtime{ns: tnaming.NewSimpleNamespace()}, ctx, func() {}, nil
}

func (r *Runtime) Init(ctx *context.T) error {
	return nil
}

func (r *Runtime) WithPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	return context.WithValue(ctx, principalKey, principal), nil
}

func (r *Runtime) GetPrincipal(ctx *context.T) security.Principal {
	// nologcall
	p, _ := ctx.Value(principalKey).(security.Principal)
	return p
}

func (r *Runtime) WithBackgroundContext(ctx *context.T) *context.T {
	// Note we add an extra context with a nil value here.
	// This prevents users from travelling back through the
	// chain of background contexts.
	ctx = context.WithValue(ctx, backgroundKey, nil)
	return context.WithValue(ctx, backgroundKey, ctx)
}

func (r *Runtime) GetBackgroundContext(ctx *context.T) *context.T {
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

func (r *Runtime) GetPermissionsSpec(ctx *context.T) access.PermissionsSpec {
	return access.PermissionsSpec{}
}

func (r *Runtime) GetRequestID(ctx *context.T) uuid.UUID {
	return uuid.UUID{}
}

func (r *Runtime) NewDiscovery(ctx *context.T) (discovery.T, error) {
	// nologcall
	panic("unimplemented")
}

func (*Runtime) WithReservedNameDispatcher(ctx *context.T, d rpc.Dispatcher) *context.T {
	panic("unimplemented")
}

func (*Runtime) GetReservedNameDispatcher(ctx *context.T) rpc.Dispatcher {
	// nologcall
	panic("unimplmeneted")
}
