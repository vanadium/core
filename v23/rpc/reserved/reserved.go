// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reserved implements client-side support for reserved RPC methods
// implemented by all servers.
package reserved

import (
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdlroot/signature"
)

// Signature invokes the reserved signature RPC on the given name, and returns
// the results.
func Signature(ctx *context.T, name string, opts ...rpc.CallOpt) ([]signature.Interface, error) {
	var sig []signature.Interface
	res := []interface{}{&sig}
	if err := v23.GetClient(ctx).Call(ctx, name, rpc.ReservedSignature, nil, res, opts...); err != nil {
		return nil, err
	}
	return sig, nil
}

// MethodSignature invokes the reserved method signature RPC on the given name,
// and returns the results.
func MethodSignature(ctx *context.T, name, method string, opts ...rpc.CallOpt) (signature.Method, error) {
	args := []interface{}{method}
	var sig signature.Method
	res := []interface{}{&sig}
	if err := v23.GetClient(ctx).Call(ctx, name, rpc.ReservedMethodSignature, args, res, opts...); err != nil {
		return signature.Method{}, err
	}
	return sig, nil
}
