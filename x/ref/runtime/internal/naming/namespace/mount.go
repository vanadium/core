// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
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
	var flags naming.MountFlag
	for _, o := range opts {
		// NB: used a switch since we'll be adding more options.
		switch v := o.(type) {
		case naming.ReplaceMount:
			if v {
				flags |= naming.Replace
			}
		case naming.ServesMountTable:
			if v {
				flags |= naming.MT
			}
		case naming.IsLeaf:
			if v {
				flags |= naming.Leaf
			}
		}
	}

	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		ns.forget(ctx, me)
		for i, mts := range me.Servers {
			mec := me
			mec.Servers = []naming.MountedServer{mts}
			ctx.VI(1).Infof("Mount(%s, %q)@%v: %v -> %v", name, server, i, mts, err)
			copts := append(getCallOpts(opts), options.Preresolved{Resolution: mec})
			timeoutCtx, cancel := withTimeout(ctx)
			defer cancel()
			err = v23.GetClient(ctx).Call(timeoutCtx, name, "Mount", []interface{}{server, uint32(ttl.Seconds()), flags}, nil, copts...)
			// Always attempt all mounts, but report the last error found.
		}
	}
	ctx.VI(1).Infof("Mount(%s, %q) -> %v", name, server, err)
	return err
}

// Unmount implements Namespace.Unmount.
func (ns *namespace) Unmount(ctx *context.T, name, server string, opts ...naming.NamespaceOpt) error {
	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{Resolution: me})
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
	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err == nil {
		copts := append(getCallOpts(opts), options.Preresolved{Resolution: me})
		timeoutCtx, cancel := withTimeout(ctx)
		defer cancel()
		err = v23.GetClient(ctx).Call(timeoutCtx, name, "Delete", []interface{}{deleteSubtree}, nil, copts...)
		ns.forget(ctx, me)
	}
	ctx.VI(1).Infof("Remove(%s, %v) -> %v", name, deleteSubtree, err)
	return err
}
