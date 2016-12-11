// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security/access"
	"v.io/x/ref/lib/apilog"
)

// SetPermissions implements Namespace.SetPermissions.
func (ns *namespace) SetPermissions(ctx *context.T, name string, perms access.Permissions, version string, opts ...naming.NamespaceOpt) error {
	defer apilog.LogCallf(ctx, "name=%.10s...,perms=,version=%.10s...,opts...=%v", name, version, opts)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT

	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{me})
		timeoutCtx, cancel := withTimeout(ctx)
		defer cancel()
		err = v23.GetClient(ctx).Call(timeoutCtx, name, "SetPermissions", []interface{}{perms, version}, nil, copts...)
		ns.forget(ctx, me)
	}
	ctx.VI(1).Infof("SetPermissions(%s, %v, %s) -> %v", name, perms, version, err)
	return err
}

// GetPermissions implements Namespace.GetPermissions.
func (ns *namespace) GetPermissions(ctx *context.T, name string, opts ...naming.NamespaceOpt) (perms access.Permissions, version string, err error) {
	defer apilog.LogCallf(ctx, "name=%.10s...,opts...=%v", name, opts)(ctx, "perms=,version=%.10s...,err=%v", &version, &err) // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT

	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{me})
		timeoutCtx, cancel := withTimeout(ctx)
		defer cancel()
		err = v23.GetClient(ctx).Call(timeoutCtx, name, "GetPermissions", []interface{}{}, []interface{}{&perms, &version}, copts...)
		ns.forget(ctx, me)
	}
	ctx.VI(1).Infof("GetPermissions(%s) -> (%v, %v, %v)", name, perms, version, err)
	return
}
