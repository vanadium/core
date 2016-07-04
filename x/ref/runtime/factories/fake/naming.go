// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/x/ref/lib/apilog"
)

func (r *Runtime) WithNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	defer apilog.LogCallf(ctx, "roots...=%v", roots)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	panic("unimplemented")
}
func (r *Runtime) GetNamespace(ctx *context.T) namespace.T {
	// nologcall
	return r.ns
}
