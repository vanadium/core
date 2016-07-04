// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bridge

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vtrace"
	slib "v.io/x/ref/lib/security"
)

// Bridge object, representing an in-process Syncbase singleton.
type Bridge struct {
	Ctx      *context.T
	Shutdown v23.Shutdown
	Srv      rpc.Server
	Disp     rpc.Dispatcher
	Cleanup  func()
}

func NewBridge(ctx *context.T, disp rpc.Dispatcher) *Bridge {
	return &Bridge{Ctx: ctx, Disp: disp}
}

func (b *Bridge) NewCtxCall(suffix string, method rpc.MethodDesc) (*context.T, rpc.ServerCall) {
	ctx, _ := vtrace.WithNewTrace(b.Ctx)
	return ctx, newFakeServerCall(ctx, b.Srv, suffix, method)
}

////////////////////////////////////////
// Dispatch-related methods

func (b *Bridge) lookupAndAuthorize(ctx *context.T, call rpc.ServerCall, suffix string) (interface{}, error) {
	resInt, auth, err := b.Disp.Lookup(ctx, suffix)
	if err != nil {
		return nil, err
	}
	if err := auth.Authorize(ctx, call.Security()); err != nil {
		return nil, verror.New(verror.ErrNoAccess, ctx, err)
	}
	return resInt, nil
}

func (b *Bridge) GetService(ctx *context.T, call rpc.ServerCall) (wire.ServiceServerStubMethods, error) {
	resInt, err := b.lookupAndAuthorize(ctx, call, "")
	if err != nil {
		return nil, err
	}
	if res, ok := resInt.(wire.ServiceServerStubMethods); !ok {
		return nil, verror.NewErrInternal(ctx)
	} else {
		return res, nil
	}
}

func (b *Bridge) GetDb(ctx *context.T, call rpc.ServerCall, name string) (wire.DatabaseServerStubMethods, error) {
	resInt, err := b.lookupAndAuthorize(ctx, call, name)
	if err != nil {
		return nil, err
	}
	if res, ok := resInt.(wire.DatabaseServerStubMethods); !ok {
		return nil, verror.NewErrInternal(ctx)
	} else {
		return res, nil
	}
}

func (b *Bridge) GetGlobber(ctx *context.T, call rpc.ServerCall, name string) (rpc.ChildrenGlobber, error) {
	resInt, err := b.lookupAndAuthorize(ctx, call, name)
	if err != nil {
		return nil, err
	}
	if res, ok := resInt.(rpc.Globber); !ok {
		return nil, verror.NewErrInternal(ctx)
	} else if res.Globber() == nil || res.Globber().ChildrenGlobber == nil {
		return nil, verror.NewErrInternal(ctx)
	} else {
		return res.Globber().ChildrenGlobber, nil
	}
}

func (b *Bridge) GetCollection(ctx *context.T, call rpc.ServerCall, name string) (wire.CollectionServerStubMethods, error) {
	resInt, err := b.lookupAndAuthorize(ctx, call, name)
	if err != nil {
		return nil, err
	}
	if res, ok := resInt.(wire.CollectionServerStubMethods); !ok {
		return nil, verror.NewErrInternal(ctx)
	} else {
		return res, nil
	}
}

func (b *Bridge) GetRow(ctx *context.T, call rpc.ServerCall, name string) (wire.RowServerStubMethods, error) {
	resInt, err := b.lookupAndAuthorize(ctx, call, name)
	if err != nil {
		return nil, err
	}
	if res, ok := resInt.(wire.RowServerStubMethods); !ok {
		return nil, verror.NewErrInternal(ctx)
	} else {
		return res, nil
	}
}

////////////////////////////////////////
// fakeServerCall

type fakeServerCall struct {
	sec    security.Call
	srv    rpc.Server
	suffix string
}

// TODO(sadovsky): Synthesize endpoints and discharges as needed.
func newFakeServerCall(ctx *context.T, srv rpc.Server, suffix string, method rpc.MethodDesc) rpc.ServerCall {
	p := v23.GetPrincipal(ctx)
	// HACK: For now, we set the "remote" (client, i.e. mobile app) blessing to be
	// the same as the "local" (server, i.e. Syncbase module) blessing.
	// TODO(sadovsky): Eliminate this hack.
	blessings, _ := p.BlessingStore().Default()
	discharges, _ := slib.PrepareDischarges(ctx, blessings, security.BlessingNames(p, blessings), method.Name, nil)
	return &fakeServerCall{
		sec: security.NewCall(&security.CallParams{
			Method:           method.Name,
			MethodTags:       method.Tags,
			Suffix:           suffix,
			LocalPrincipal:   p,
			LocalBlessings:   blessings,
			RemoteBlessings:  blessings,
			RemoteDischarges: discharges,
			LocalDischarges:  discharges,
		}),
		srv:    srv,
		suffix: suffix,
	}
}

var _ rpc.ServerCall = (*fakeServerCall)(nil)

func (call *fakeServerCall) Security() security.Call {
	return call.sec
}

func (call *fakeServerCall) Suffix() string {
	return call.suffix
}

func (call *fakeServerCall) LocalEndpoint() naming.Endpoint {
	return call.sec.LocalEndpoint()
}

func (call *fakeServerCall) RemoteEndpoint() naming.Endpoint {
	return call.sec.RemoteEndpoint()
}

func (call *fakeServerCall) GrantedBlessings() security.Blessings {
	return security.Blessings{}
}

func (call *fakeServerCall) Server() rpc.Server {
	return call.srv
}
