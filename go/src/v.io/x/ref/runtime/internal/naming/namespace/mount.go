// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/x/ref/lib/apilog"
)

func (ns *namespace) forget(ctx *context.T, me *naming.MountEntry) {
	var names []string
	for _, s := range me.Servers {
		names = append(names, naming.JoinAddressName(s.Server, me.Name))
	}
	ns.resolutionCache.forget(ctx, names)
}

// Mount implements Namespace.Mount.
func (ns *namespace) Mount(ctx *context.T, name, server string, ttl time.Duration, opts ...naming.NamespaceOpt) error {
	defer apilog.LogCallf(ctx, "name=%.10s...,server=%.10s...,ttl=%v,opts...=%v", name, server, ttl, opts)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	var flags naming.MountFlag
	for _, o := range opts {
		// NB: used a switch since we'll be adding more options.
		switch v := o.(type) {
		case naming.ReplaceMount:
			if v {
				flags |= naming.MountFlag(naming.Replace)
			}
		case naming.ServesMountTable:
			if v {
				flags |= naming.MountFlag(naming.MT)
			}
		case naming.IsLeaf:
			if v {
				flags |= naming.MountFlag(naming.Leaf)
			}
		}
	}

	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{me})
		timeoutCtx, cancel := withTimeout(ctx)
		defer cancel()
		err = v23.GetClient(ctx).Call(timeoutCtx, name, "Mount", []interface{}{server, uint32(ttl.Seconds()), flags}, nil, copts...)
		ns.forget(ctx, me)
	}
	ctx.VI(1).Infof("Mount(%s, %q) -> %v", name, server, err)
	return err
}

// Unmount implements Namespace.Unmount.
func (ns *namespace) Unmount(ctx *context.T, name, server string, opts ...naming.NamespaceOpt) error {
	defer apilog.LogCallf(ctx, "name=%.10s...,server=%.10s...,opts...=%v", name, server, opts)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{me})
		timeoutCtx, cancel := withTimeout(ctx)
		defer cancel()
		err = v23.GetClient(ctx).Call(timeoutCtx, name, "Unmount", []interface{}{server}, nil, copts...)
		ns.forget(ctx, me)
	}
	ctx.VI(1).Infof("Unmount(%s, %s) -> %v", name, server, err)
	return err
}

// Delete implements Namespace.Delete.
func (ns *namespace) Delete(ctx *context.T, name string, deleteSubtree bool, opts ...naming.NamespaceOpt) error {
	defer apilog.LogCallf(ctx, "name=%.10s...,deleteSubtree=%v,opts...=%v", name, deleteSubtree, opts)(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{me})
		timeoutCtx, cancel := withTimeout(ctx)
		defer cancel()
		err = v23.GetClient(ctx).Call(timeoutCtx, name, "Delete", []interface{}{deleteSubtree}, nil, copts...)
		ns.forget(ctx, me)
	}
	ctx.VI(1).Infof("Remove(%s, %v) -> %v", name, deleteSubtree, err)
	return err
}
