// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security/access"
)

// SetPermissions implements Namespace.SetPermissions.
func (ns *namespace) SetPermissions(ctx *context.T, name string, perms access.Permissions, version string, opts ...naming.NamespaceOpt) error {

	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{Resolution: me})
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

	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{Resolution: me})
		timeoutCtx, cancel := withTimeout(ctx)
		defer cancel()
		err = v23.GetClient(ctx).Call(timeoutCtx, name, "GetPermissions", []interface{}{}, []interface{}{&perms, &version}, copts...)
		ns.forget(ctx, me)
	}
	ctx.VI(1).Infof("GetPermissions(%s) -> (%v, %v, %v)", name, perms, version, err)
	return
}
