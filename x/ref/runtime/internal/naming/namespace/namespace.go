// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	vnamespace "v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	vdltime "v.io/v23/vdlroot/time"
	"v.io/v23/verror"
	"v.io/x/ref"
)

const defaultMaxResolveDepth = 32
const defaultMaxRecursiveGlobDepth = 10

// namespace is an implementation of namespace.T.
type namespace struct {
	sync.RWMutex

	// the default root servers for resolutions in this namespace.
	roots []string

	// depth limits
	maxResolveDepth       int
	maxRecursiveGlobDepth int

	// cache for name resolutions
	resolutionCache cache
}

// Factory creates a new namespace given a default namespace and a set
// of roots.
type Factory func(*context.T, vnamespace.T, ...string) (vnamespace.T, error)

func rooted(names []string) bool {
	for _, n := range names {
		if a, _ := naming.SplitAddressName(n); len(a) == 0 {
			return false
		}
	}
	return true
}

func badRoots(roots []string) error {
	return fmt.Errorf("at least one root is not a rooted name: %v", strings.Join(roots, ", "))
}

// Create a new namespace.
//nolint:golint // API change required.
func New(roots ...string) (*namespace, error) {
	if !rooted(roots) {
		return nil, badRoots(roots)
	}
	// A namespace with no roots can still be used for lookups of rooted names.
	return &namespace{
		roots:                 roots,
		maxResolveDepth:       defaultMaxResolveDepth,
		maxRecursiveGlobDepth: defaultMaxRecursiveGlobDepth,
		resolutionCache:       newCache(os.Getenv(ref.EnvDisableNamespaceCache) != ""),
	}, nil
}

// SetRoots implements namespace.T.SetRoots
func (ns *namespace) SetRoots(roots ...string) error {
	// Allow roots to be cleared with a call of SetRoots()
	if len(roots) > 0 && !rooted(roots) {
		return badRoots(roots)
	}
	ns.Lock()
	defer ns.Unlock()
	// TODO(cnicolaou): filter out duplicate values.
	ns.roots = roots
	return nil
}

// SetDepthLimits overrides the default limits.
func (ns *namespace) SetDepthLimits(resolve, glob int) {
	if resolve >= 0 {
		ns.maxResolveDepth = resolve
	}
	if glob >= 0 {
		ns.maxRecursiveGlobDepth = glob
	}
}

// Roots implements namespace.T.Roots
func (ns *namespace) Roots() []string {
	ns.RLock()
	defer ns.RUnlock()
	roots := make([]string, len(ns.roots))
	for i, r := range ns.roots {
		roots[i] = r
	}
	return roots
}

// rootName 'roots' a name: if name is not a rooted name, it prepends the root
// mounttable's OA.
func (ns *namespace) rootName(name string) []string {
	name = naming.Clean(name)
	if address, _ := naming.SplitAddressName(name); len(address) == 0 {
		var ret []string
		ns.RLock()
		defer ns.RUnlock()
		for _, r := range ns.roots {
			ret = append(ret, naming.Join(r, name))
		}
		return ret
	}
	return []string{name}
}

// rootMountEntry 'roots' a name creating a mount entry for the name.
//
// Returns:
// (1) MountEntry
// (2) Whether "name" is a rooted name or not (if not, the namespace roots
//     configured in "ns" will be used).
func (ns *namespace) rootMountEntry(name string, opts ...naming.NamespaceOpt) (*naming.MountEntry, bool) {
	_, name = security.SplitPatternName(naming.Clean(name))
	e := new(naming.MountEntry)
	deadline := vdltime.Deadline{Time: time.Now().Add(time.Hour)} // plenty of time for a call
	address, suffix := naming.SplitAddressName(name)
	if len(address) == 0 {
		e.ServesMountTable = true
		e.Name = name
		ns.RLock()
		defer ns.RUnlock()
		for _, r := range ns.roots {
			e.Servers = append(e.Servers, naming.MountedServer{Server: r, Deadline: deadline})
		}
		return e, false
	}
	servesMT := true
	if ep, err := naming.ParseEndpoint(address); err == nil {
		servesMT = ep.ServesMountTable
	}
	e.ServesMountTable = servesMT
	e.Name = suffix
	e.Servers = []naming.MountedServer{{Server: naming.JoinAddressName(address, ""), Deadline: deadline}}
	return e, true
}

// notAnMT returns true if the error indicates this isn't a mounttable server.
func notAnMT(err error) bool {
	switch verror.ErrorID(err) {
	case verror.ErrBadArg.ID:
		// This should cover "rpc: wrong number of in-args".
		return true
	case verror.ErrNoExist.ID, verror.ErrUnknownMethod.ID, verror.ErrUnknownSuffix.ID, errNoServers.ID:
		// This should cover "rpc: unknown method", "rpc: dispatcher not
		// found", dispatcher Lookup not found, and "No servers found to resolve query "errors.
		return true
	case verror.ErrBadProtocol.ID:
		// This covers "rpc: response decoding failed: EOF".
		return true
	}
	return false
}

// All operations against the mount table service use this fixed timeout unless overridden.
const callTimeout = 30 * time.Second

func emptyFunc() {}

// withTimeout returns a new context if the orinal has no timeout set.
func withTimeout(ctx *context.T) (*context.T, func()) {
	var cancel func()
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, callTimeout)
	} else {
		cancel = emptyFunc
	}
	return ctx, cancel
}

// CacheCtl implements namespace.T.CacheCtl
func (ns *namespace) CacheCtl(ctls ...naming.CacheCtl) []naming.CacheCtl {
	for _, c := range ctls {
		if v, ok := c.(naming.DisableCache); ok {
			disableCache := bool(v)
			ns.Lock()
			if _, isDisabled := ns.resolutionCache.(nullCache); isDisabled != disableCache {
				ns.resolutionCache = newCache(disableCache)
			}
			ns.Unlock()
		}
	}
	ns.RLock()
	defer ns.RUnlock()
	if _, isDisabled := ns.resolutionCache.(nullCache); isDisabled {
		return []naming.CacheCtl{naming.DisableCache(true)}
	}
	return nil
}

func getCallOpts(opts []naming.NamespaceOpt) []rpc.CallOpt {
	var out []rpc.CallOpt
	for _, o := range opts {
		if co, ok := o.(rpc.CallOpt); ok {
			if auth, ok := co.(options.NameResolutionAuthorizer); ok {
				// This function is being called to determine
				// the options for the "Resolve" RPC. The end server of
				// this particular RPC is to be authorized by the policy
				// in NameResolutionAuthorizer.
				co = options.ServerAuthorizer(auth)
			}
			out = append(out, co)
		}
	}
	return out
}
