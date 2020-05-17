// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"io"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/v23/verror"
)

var (
	errCantUpgradeServerCall = verror.Register(pkgPath+".errCantUpgradeServerCall", verror.NoRetry, "{1:}{2:} couldn't upgrade rpc.ServerCall to rpc.StreamServerCall{:_}")
	errBadNumberOfResults    = verror.Register(pkgPath+".errBadNumberOfResults", verror.NoRetry, "{1:}{2:} unexpected number of result values. Got {3}, want 2.{:_}")
	errBadErrorType          = verror.Register(pkgPath+".errBadErrorType", verror.NoRetry, "{1:}{2:} unexpected error type. Got {3}, want error.{:_}")
	errWantSigInterfaceSlice = verror.Register(pkgPath+".errWantSigInterfaceSlice", verror.NoRetry, "{1:}{2:} unexpected result value type. Got {3}, want []signature.Interface.{:_}")
	errWantSigMethod         = verror.Register(pkgPath+".errWantSigMethod", verror.NoRetry, "{1:}{2:} unexpected result value type. Got {3}, want signature.Method.{:_}")
	errUnknownMethod         = verror.Register(pkgPath+".errUnknownMethod", verror.NoRetry, "{1:}{2:} unknown method{:_}")
)

// proxyInvoker is an rpc.Invoker implementation that proxies all requests
// to a remote object, i.e. requests to <suffix> are forwarded to
// <remote> transparently.
//
// remote is the name of the remote object.
// access is the access tag require to access the object.
// desc is used to determine the number of results for a given method.
func newProxyInvoker(remote string, access access.Tag, desc []rpc.InterfaceDesc) *proxyInvoker {
	methodNumResults := make(map[string]int)
	for _, iface := range desc {
		for _, method := range iface.Methods {
			methodNumResults[method.Name] = len(method.OutArgs)
		}
	}
	return &proxyInvoker{remote, access, methodNumResults}
}

type proxyInvoker struct {
	remote           string
	access           access.Tag
	methodNumResults map[string]int
}

var _ rpc.Invoker = (*proxyInvoker)(nil)

func (p *proxyInvoker) Prepare(_ *context.T, method string, numArgs int) (argptrs []interface{}, tags []*vdl.Value, _ error) {
	// TODO(toddw): Change argptrs to be filled in with *vdl.Value, to avoid
	// unnecessary type lookups.
	argptrs = make([]interface{}, numArgs)
	for i := range argptrs {
		var x interface{}
		argptrs[i] = &x
	}
	tags = []*vdl.Value{vdl.ValueOf(p.access)}
	return
}

func (p *proxyInvoker) Invoke(ctx *context.T, inCall rpc.StreamServerCall, method string, argptrs []interface{}) (results []interface{}, err error) {
	// We accept any values as argument and pass them through to the remote
	// server.
	args := make([]interface{}, len(argptrs))
	for i, ap := range argptrs {
		args[i] = ap
	}
	client := v23.GetClient(ctx)

	outCall, err := client.StartCall(ctx, p.remote, method, args)
	if err != nil {
		return nil, err
	}

	// Each RPC has a bi-directional stream, and there is no way to know in
	// advance how much data will be sent in either direction, if any.
	//
	// This method (Invoke) must return when the remote server is done with
	// the RPC, which is when outCall.Recv() returns EOF. When that happens,
	// we need to call outCall.Finish() to get the return values, and then
	// return these values to the client.
	//
	// While we are forwarding data from the server to the client, we must
	// also forward data from the client to the server. This happens in a
	// separate goroutine. This goroutine may return after Invoke has
	// returned if the client doesn't call CloseSend() explicitly.
	//
	// Any error, other than EOF, will be returned to the client, if
	// possible. The only situation where it is not possible to send an
	// error to the client is when the error comes from forwarding data from
	// the client to the server and Invoke has already returned or is about
	// to return. In this case, the error is lost. So, it is possible that
	// the client could successfully Send() data that the server doesn't
	// actually receive if the server terminates the RPC while the data is
	// in the proxy.
	fwd := func(src, dst rpc.Stream, errors chan<- error) {
		for {
			var obj interface{}
			switch err := src.Recv(&obj); err {
			case io.EOF:
				if call, ok := src.(rpc.ClientCall); ok {
					if err := call.CloseSend(); err != nil {
						errors <- err
					}
				}
				return
			case nil:
				break
			default:
				errors <- err
				return
			}
			if err := dst.Send(obj); err != nil {
				errors <- err
				return
			}
		}
	}
	errors := make(chan error, 2)
	go fwd(inCall, outCall, errors)
	fwd(outCall, inCall, errors)
	select {
	case err := <-errors:
		return nil, err
	default:
	}

	nResults, err := p.numResults(ctx, method)
	if err != nil {
		return nil, err
	}

	// We accept any return values, without type checking, and return them
	// to the client.
	res := make([]interface{}, nResults)
	for i := 0; i < len(res); i++ {
		var foo interface{}
		res[i] = &foo
	}
	err = outCall.Finish(res...)
	results = make([]interface{}, len(res))
	for i, r := range res {
		results[i] = *r.(*interface{})
	}
	return results, err
}

// TODO(toddw): Expose a helper function that performs all error checking based
// on reflection, to simplify the repeated logic processing results.
func (p *proxyInvoker) Signature(ctx *context.T, call rpc.ServerCall) ([]signature.Interface, error) {
	streamCall, ok := call.(rpc.StreamServerCall)
	if !ok {
		return nil, verror.New(errCantUpgradeServerCall, ctx)
	}
	results, err := p.Invoke(ctx, streamCall, rpc.ReservedSignature, nil)
	if err != nil {
		return nil, err
	}
	if len(results) != 2 {
		return nil, verror.New(errBadNumberOfResults, ctx, len(results))
	}
	if results[1] != nil {
		err, ok := results[1].(error)
		if !ok {
			return nil, verror.New(errBadErrorType, ctx, fmt.Sprintf("%T", err))
		}
		return nil, err
	}
	var res []signature.Interface
	if results[0] != nil {
		sig, ok := results[0].([]signature.Interface)
		if !ok {
			return nil, verror.New(errWantSigInterfaceSlice, ctx, fmt.Sprintf("%T", sig))
		}
	}
	return res, nil
}

func (p *proxyInvoker) MethodSignature(ctx *context.T, call rpc.ServerCall, method string) (signature.Method, error) {
	empty := signature.Method{}
	streamCall, ok := call.(rpc.StreamServerCall)
	if !ok {
		return empty, verror.New(errCantUpgradeServerCall, ctx)
	}
	results, err := p.Invoke(ctx, streamCall, rpc.ReservedMethodSignature, []interface{}{&method})
	if err != nil {
		return empty, err
	}
	if len(results) != 2 {
		return empty, verror.New(errBadNumberOfResults, ctx, len(results))
	}
	if results[1] != nil {
		err, ok := results[1].(error)
		if !ok {
			return empty, verror.New(errBadErrorType, ctx, fmt.Sprintf("%T", err))
		}
		return empty, err
	}
	var res signature.Method
	if results[0] != nil {
		sig, ok := results[0].(signature.Method)
		if !ok {
			return empty, verror.New(errWantSigMethod, ctx, fmt.Sprintf("%T", sig))
		}
	}
	return res, nil
}

func (p *proxyInvoker) Globber() *rpc.GlobState {
	return &rpc.GlobState{AllGlobber: p}
}

type call struct {
	rpc.GlobServerCall
}

func (c *call) Recv(v interface{}) error {
	return io.EOF
}

func (c *call) Send(v interface{}) error {
	return c.SendStream().Send(v.(naming.GlobReply))
}

func (p *proxyInvoker) Glob__(ctx *context.T, serverCall rpc.GlobServerCall, g *glob.Glob) error {
	pattern := g.String()
	p.Invoke(ctx, &call{serverCall}, rpc.GlobMethod, []interface{}{&pattern})
	return nil
}

// numResults returns the number of result values for the given method.
func (p *proxyInvoker) numResults(ctx *context.T, method string) (int, error) {
	switch method {
	case rpc.GlobMethod:
		return 1, nil
	case rpc.ReservedSignature, rpc.ReservedMethodSignature:
		return 2, nil
	}
	num, ok := p.methodNumResults[method]
	if !ok {
		return 0, verror.New(errUnknownMethod, ctx, method)
	}
	return num, nil
}
