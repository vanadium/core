// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"errors"
	"runtime"
	"strings"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/verror"
)

var (
	errNoServers = verror.NewIDAction("errNoServers", verror.NoRetry)
)

// resolveAgainstMountTable asks each server in e.Servers that might be a mounttable to resolve e.Name.  The requests
// are parallelized by the client rpc code.
func (ns *namespace) resolveAgainstMountTable(ctx *context.T, client rpc.Client, e *naming.MountEntry, opts ...rpc.CallOpt) (*naming.MountEntry, error) {
	// Run through the server list looking for answers in the cache or servers that aren't mounttables.
	change := false
	for _, s := range e.Servers {
		// If the server was not specified as an endpoint (perhaps as host:port)
		// then we really don't know if this is a mounttable or not.  Check the
		// cache to see if we've tried in the recent past and it came back as not
		// a mounttable.
		if ns.resolutionCache.isNotMT(s.Server) {
			change = true
			continue
		}

		// Check the cache.  If its there, we're done.
		n := naming.JoinAddressName(s.Server, e.Name)
		if ne, err := ns.resolutionCache.lookup(ctx, n); err == nil {
			ctx.VI(2).Infof("resolveAMT %s from cache -> %v", n, convertServersToStrings(ne.Servers, ne.Name))
			return &ne, nil
		}
	}
	// We had at least one server that wasn't a mount table.  Create a new mount entry without those servers.
	if change {
		ne := *e
		ne.Servers = nil
		for _, s := range e.Servers {
			if !ns.resolutionCache.isNotMT(s.Server) {
				ne.Servers = append(ne.Servers, s)
			}
		}
		e = &ne
	}
	// If we have no servers to query, give up.
	if len(e.Servers) == 0 {
		ctx.VI(2).Infof("resolveAMT %s -> No servers", e.Name)
		return nil, errNoServers.Errorf(ctx, "no servers found to resolve query")
	}
	// We have preresolved the servers.  Pass the mount entry to the call.
	opts = append(opts, options.Preresolved{Resolution: e})
	callCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		// Only set a per-call timeout if a deadline has not already
		// been set.
		var cancel func()
		callCtx, cancel = withTimeout(ctx)
		defer cancel()
	}
	entry := new(naming.MountEntry)
	if err := client.Call(callCtx, e.Name, "ResolveStep", nil, []interface{}{entry}, opts...); err != nil {
		// If it wasn't a mounttable remember that fact.  The check for the __ is for
		// the debugging hack in the local namespace of every server.  That part never
		// answers mounttable RPCs and shouldn't make us think this isn't a mounttable
		// server.
		if notAnMT(err) && !strings.HasPrefix(e.Name, "__") {
			for _, s := range e.Servers {
				ns.resolutionCache.setNotMT(s.Server)
			}
		}
		return nil, err
	}
	// Add result to cache for each server that may have returned it.
	for _, s := range e.Servers {
		n := naming.JoinAddressName(s.Server, e.Name)
		ns.resolutionCache.remember(ctx, n, entry)
	}
	ctx.VI(2).Infof("resolveAMT %s -> %v", e.Name, entry)
	return entry, nil
}

func terminal(e *naming.MountEntry) bool {
	return len(e.Name) == 0
}

func hasEndpointPrefix(name string) bool {
	if name == "" || naming.Rooted(name) {
		return false
	}
	elems := strings.SplitN(name, "/", 2)
	_, err := naming.ParseEndpoint(elems[0])
	return err == nil
}

// Resolve implements v.io/v23/naming.Namespace.
func (ns *namespace) Resolve(ctx *context.T, name string, opts ...naming.NamespaceOpt) (*naming.MountEntry, error) {

	e, err := ns.resolveInternal(ctx, name, opts...)
	// If the resolution failed and it appears that the name starts with an
	// endpoint but was mistakenly passed in un-rooted, add a sub error to
	// point that out to the client.

	// TODO(caprita): Consider doing this also for ResolveShallow and
	// ResolveToMountTable.
	if err != nil && hasEndpointPrefix(name) {
		verr := err
		if !verror.IsAny(verr) {
			verr = verror.ErrUnknown.Errorf(ctx, "%v", err)
		}
		err = verror.WithSubErrors(verr, verror.SubErr{Name: "Did you mean", Err: verror.E{Msg: "/" + name}, Options: verror.Print})
	}
	return e, err
}

func (ns *namespace) resolveInternal(ctx *context.T, name string, opts ...naming.NamespaceOpt) (*naming.MountEntry, error) {
	// If caller supplied a mount entry, use it.
	e, skipResolution := preresolved(opts)
	if e != nil {
		return e, nil
	}
	// Expand any relative name.
	e, _ = ns.rootMountEntry(name, opts...)
	if ctx.V(2) {
		_, file, line, _ := runtime.Caller(1)
		ctx.Infof("Resolve(%s) called from %s:%d", name, file, line)
		ctx.Infof("Resolve(%s) -> rootEntry %v", name, *e)
	}
	// If caller didn't want resolution, use expanded name.
	if skipResolution {
		return e, nil
	}
	if len(e.Servers) == 0 {
		return nil, naming.ErrNoSuchName.Errorf(ctx, "name %v doesn't exist", name)
	}
	client := v23.GetClient(ctx)
	callOpts := getCallOpts(opts)

	// Iterate walking through mount table servers.
	for remaining := ns.maxResolveDepth; remaining > 0; remaining-- {
		if ctx.V(2) {
			ctx.Infof("Resolve(%s) loop %v", name, *e)
		}
		if !e.ServesMountTable {
			if ctx.V(1) {
				ctx.Infof("Resolve(%s) -> %v", name, *e)
			}
			return e, nil
		}
		var err error
		curr := e
		if e, err = ns.resolveAgainstMountTable(ctx, client, curr, callOpts...); err != nil {
			// Lots of reasons why another error can happen.  We are trying
			// to single out "this isn't a mount table".
			if notAnMT(err) {
				ctx.VI(1).Infof("Resolve(%s) -> %v", name, curr)
				return curr, nil
			}
			if errors.Is(err, naming.ErrNoSuchNameRoot) {
				err = naming.ErrNoSuchName.Errorf(ctx, "name %v doesn't exist", name)
			}
			ctx.VI(1).Infof("Resolve(%s) -> (%s: %v)", err, name, curr)
			return nil, err
		}
	}
	return nil, naming.ErrResolutionDepthExceeded.Errorf(ctx, "resolution depth exceeded")
}

// ShallowResolve implements v.io/v23/naming.Namespace.
func (ns *namespace) ShallowResolve(ctx *context.T, name string, opts ...naming.NamespaceOpt) (*naming.MountEntry, error) {

	// Find the containing mount table.
	me, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err != nil {
		return nil, err
	}
	if terminal(me) {
		return me, nil
	}

	// Resolve the entry directly.
	client := v23.GetClient(ctx)
	entry := new(naming.MountEntry)
	err = client.Call(ctx, name, "ResolveStep", nil, []interface{}{entry}, append(getCallOpts(opts), options.Preresolved{Resolution: me})...)
	return entry, err
}

// ResolveToMountTable implements v.io/v23/naming.Namespace.
func (ns *namespace) ResolveToMountTable(ctx *context.T, name string, opts ...naming.NamespaceOpt) (*naming.MountEntry, error) {
	e, _ := ns.rootMountEntry(name, opts...)
	if ctx.V(2) {
		_, file, line, _ := runtime.Caller(1)
		ctx.Infof("ResolveToMountTable(%s) called from %s:%d", name, file, line)
		ctx.Infof("ResolveToMountTable(%s) -> rootNames %v", name, e)
	}
	if len(e.Servers) == 0 {
		return nil, naming.ErrNoMountTable.Errorf(ctx, "no mounttable")
	}
	callOpts := getCallOpts(opts)
	client := v23.GetClient(ctx)
	last := e
	for remaining := ns.maxResolveDepth; remaining > 0; remaining-- {
		ctx.VI(2).Infof("ResolveToMountTable(%s) loop %v", name, e)
		var err error
		curr := e
		// If the next name to resolve doesn't point to a mount table, we're done.
		if !e.ServesMountTable || terminal(e) {
			ctx.VI(1).Infof("ResolveToMountTable(%s) -> %v", name, last)
			return last, nil
		}
		if e, err = ns.resolveAgainstMountTable(ctx, client, e, callOpts...); err != nil {
			if errors.Is(err, naming.ErrNoSuchNameRoot) {
				ctx.VI(1).Infof("ResolveToMountTable(%s) -> %v (NoSuchRoot: %v)", name, last, curr)
				return last, nil
			}
			if errors.Is(err, naming.ErrNoSuchName) {
				ctx.VI(1).Infof("ResolveToMountTable(%s) -> %v (NoSuchName: %v)", name, curr, curr)
				return curr, nil
			}
			// Lots of reasons why another error can happen.  We are trying
			// to single out "this isn't a mount table".
			if notAnMT(err) {
				ctx.VI(1).Infof("ResolveToMountTable(%s) -> %v", name, last)
				return last, nil
			}
			// TODO(caprita): If the server is unreachable for
			// example, we may still want to return its parent
			// mounttable rather than an error.
			ctx.VI(1).Infof("ResolveToMountTable(%s) -> %v", name, err)
			return nil, err
		}
		last = curr
	}
	return nil, naming.ErrResolutionDepthExceeded.Errorf(ctx, "resolution depth exceeded")
}

// FlushCache flushes the most specific entry found for name.  It returns true if anything was
// actually flushed.
func (ns *namespace) FlushCacheEntry(ctx *context.T, name string) bool {
	flushed := false
	for _, n := range ns.rootName(name) {
		// Walk the cache as we would in a resolution.  Unlike a resolution, we have to follow
		// all branches since we want to flush all entries at which we might end up whereas in a resolution,
		// we stop with the first branch that works.
		if e, err := ns.resolutionCache.lookup(ctx, n); err == nil {
			// Recurse.
			for _, s := range e.Servers {
				flushed = flushed || ns.FlushCacheEntry(ctx, naming.Join(s.Server, e.Name))
			}
			if !flushed {
				// Forget the entry we just used.
				ns.resolutionCache.forget(ctx, []string{naming.TrimSuffix(n, e.Name)})
				flushed = true
			}
		}
	}
	return flushed
}

func preresolved(opts []naming.NamespaceOpt) (*naming.MountEntry, bool) {
	for _, o := range opts {
		if v, ok := o.(options.Preresolved); ok {
			return v.Resolution, true
		}
	}
	return nil, false
}
