// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"v.io/v23/context"
	"v.io/v23/namespace"
)

func (r *Runtime) WithNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	panic("unimplemented")
}
func (r *Runtime) GetNamespace(ctx *context.T) namespace.T {
	// nologcall
	return r.ns
}
