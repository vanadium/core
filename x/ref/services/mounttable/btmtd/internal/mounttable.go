// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"math"
	"math/rand"
	"path"
	"reflect"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	v23mt "v.io/v23/services/mounttable"
	"v.io/v23/verror"
)

const pkgPath = "v.io/x/ref/services/mounttable/btmtd/internal"

var (
	errMalformedAddress = verror.Register(pkgPath+".errMalformedAddress", verror.NoRetry, "{1:}{2:} malformed address {3} for mounted server {4}{:_}")
	errNotEmpty         = verror.Register(pkgPath+".errNotEmpty", verror.NoRetry, "{1:}{2:} cannot delete {3}: has children{:_}")
	errConcurrentAccess = verror.Register(pkgPath+".errConcurrentAccess", verror.RetryRefetch, "{1:}{2:} concurrent access caused conflict{:_}")
)

func shouldAbort(ctx *context.T) error {
	select {
	case <-ctx.Done():
		return verror.NewErrAborted(ctx)
	default:
		return nil
	}
}

func op(ctx *context.T, f func() error) (err error) {
	for {
		if err = shouldAbort(ctx); err != nil {
			return
		}
		if err = f(); verror.ErrorID(err) == errConcurrentAccess.ID {
			ctx.VI(2).Infof("Concurrent access conflict detected: %v", err)
			time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
			continue
		}
		return
	}
}

type mounttable struct {
	suffix string
	config *Config
	bt     *BigTable

	rbn    []string
	rbnInv []security.RejectedBlessing
}

func (mt *mounttable) remoteBlessingNames(ctx *context.T, call security.Call) ([]string, []security.RejectedBlessing) {
	// This cache logic really should be in security.RemoteBlessingNames.
	// https://github.com/vanadium/issues/issues/579
	if mt.rbn == nil {
		mt.rbn, mt.rbnInv = security.RemoteBlessingNames(ctx, call)
	}
	return mt.rbn, mt.rbnInv
}

// Mount a server onto the name in the receiver.
func (mt *mounttable) Mount(ctx *context.T, call rpc.ServerCall, server string, ttl uint32, flags naming.MountFlag) error {
	ctx.VI(2).Infof("%q.Mount(%q, %d, %v)", mt.suffix, server, ttl, flags)
	if ttl == 0 {
		ttl = math.MaxUint32
	}
	// Make sure the server address is reasonable.
	ep := server
	if naming.Rooted(server) {
		ep, _ = naming.SplitAddressName(server)
	}
	if _, err := naming.ParseEndpoint(ep); err != nil {
		return verror.New(errMalformedAddress, ctx, ep, server)
	}

	return op(ctx, func() error {
		n, err := mt.accessNodeCreate(ctx, mt.suffix, call.Security())
		if err != nil {
			return err
		}
		if err := mt.authorize(ctx, call.Security(), n.permissions, v23mt.Mount, v23mt.Admin); err != nil {
			return err
		}
		return n.mount(ctx, server, clock.Now().Add(time.Duration(ttl)*time.Second), flags, mt.config.MaxServersPerUser)
	})
}

// Unmount removes servers from the name in the receiver. If server is specified, only that
// server is removed.
func (mt *mounttable) Unmount(ctx *context.T, call rpc.ServerCall, server string) error {
	ctx.VI(2).Infof("%q.Unmount(%q)", mt.suffix, server)

	return op(ctx, func() error {
		n, err := mt.accessNodeRead(ctx, mt.suffix, call.Security())
		if err != nil {
			return err
		}
		if err := mt.authorize(ctx, call.Security(), n.permissions, v23mt.Mount, v23mt.Admin); err != nil {
			return err
		}
		return n.unmount(ctx, server)
	})
}

// Delete removes the receiver.  If the receiver has children, it will not
// be removed unless deleteSubtree is true in which case the whole subtree is
// removed.
func (mt *mounttable) Delete(ctx *context.T, call rpc.ServerCall, deleteSubtree bool) error {
	ctx.VI(2).Infof("%q.Delete(%v)", mt.suffix, deleteSubtree)

	if mt.suffix == "" {
		return verror.New(verror.ErrNoAccess, ctx, "")
	}

	return op(ctx, func() error {
		n, err := mt.accessNodeRead(ctx, mt.suffix, call.Security())
		if verror.ErrorID(err) == naming.ErrNoSuchName.ID {
			return nil
		}
		if err != nil {
			return err
		}
		// Caller must have Admin access on either the node itself or
		// its parent.
		if err := mt.authorize(ctx, call.Security(), n.permissions, v23mt.Admin); err != nil {
			parent, _ := path.Split(mt.suffix)
			if p, _ := mt.accessNodeRead(ctx, parent, call.Security()); p == nil {
				return err
			} else if err2 := mt.authorize(ctx, call.Security(), p.permissions, v23mt.Admin); err2 != nil {
				return err
			}
		}
		return n.deleteAndGC(ctx, deleteSubtree)
	})
}

// ResolveStep takes the next step in resolving a name.  Returns the next
// servers to query and the suffix at those servers.
func (mt *mounttable) ResolveStep(ctx *context.T, call rpc.ServerCall) (entry naming.MountEntry, err error) {
	ctx.VI(2).Infof("%q.ResolveStep()", mt.suffix)

	n, remainder, err := mt.accessNodeWithServers(ctx, mt.suffix, call.Security())
	if err != nil {
		return entry, err
	}
	if n == nil {
		entry.Name = mt.suffix
		err = verror.New(naming.ErrNoSuchName, ctx, mt.suffix)
		return
	}
	entry.Name = remainder
	entry.Servers = make([]naming.MountedServer, len(n.servers))
	for i, s := range n.servers {
		entry.Servers[i] = s
		entry.ServesMountTable = n.mountFlags.MT
		entry.IsLeaf = n.mountFlags.Leaf
	}
	return entry, nil
}

// Glob__ finds matches in the namespace.  If we reach a mount point before
// matching the whole pattern, return that mount point.
func (mt *mounttable) Glob__(ctx *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	ctx.VI(2).Infof("%q.Glob__(%q)", mt.suffix, g)

	n, remainder, err := mt.accessNodeWithServers(ctx, mt.suffix, call.Security())
	if err != nil {
		return err
	}
	if n != nil {
		me := naming.MountEntry{
			Name:             "",
			Servers:          make([]naming.MountedServer, len(n.servers)),
			ServesMountTable: n.mountFlags.MT,
			IsLeaf:           n.mountFlags.Leaf,
		}
		for i, s := range n.servers {
			me.Servers[i].Server = naming.Join(s.Server, remainder)
			me.Servers[i].Deadline = s.Deadline
		}
		call.SendStream().Send(naming.GlobReplyEntry{Value: me})
		return nil
	}

	type gState struct {
		name string
		g    *glob.Glob
	}
	queue := []gState{gState{"", g}}
	for len(queue) != 0 {
		if err := shouldAbort(ctx); err != nil {
			return err
		}
		state := queue[0]
		queue = queue[1:]

		n, err := mt.accessNodeRead(ctx, naming.Join(mt.suffix, state.name), call.Security())
		if err != nil {
			ctx.VI(2).Infof("caller doesn't have access to %s", state.name)
			continue
		}
		if n == nil {
			continue
		}
		if len(n.servers) > 0 {
			me := naming.MountEntry{
				Name:             state.name,
				Servers:          make([]naming.MountedServer, len(n.servers)),
				ServesMountTable: n.mountFlags.MT,
				IsLeaf:           n.mountFlags.Leaf,
			}
			for i, s := range n.servers {
				me.Servers[i] = s
			}
			call.SendStream().Send(naming.GlobReplyEntry{Value: me})
			continue
		}
		if state.g.Len() == 0 {
			call.SendStream().Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: state.name}})
		}
		if state.g.Empty() {
			continue
		}
		matcher, left := state.g.Head(), state.g.Tail()
		for _, child := range n.children {
			if matcher.Match(child.name()) {
				// TODO(rthellend): We should protect against large query results
				// that could blow up memory usage.
				queue = append(queue, gState{naming.Join(state.name, child.name()), left})
			}
		}
	}
	return nil
}

// SetPermissions replaces the current Permissions for an object.  version
// allows for optional, optimistic concurrency control.  If non-empty,
// version's value must come from GetPermissions.  If any client has
// successfully called SetPermissions in the meantime, the version will be
// stale and SetPermissions will fail.  If empty, SetPermissions performs an
// unconditional update.
func (mt *mounttable) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	ctx.VI(2).Infof("%q.SetPermissions(%#v, %q)", mt.suffix, perms, version)

	// If the user is trying to set permissions on a node such that the
	// user will no longer have admin access, add the user as Admin.
	if err := mt.authorize(ctx, call.Security(), perms, v23mt.Admin); err != nil {
		blessings, _ := mt.remoteBlessingNames(ctx, call.Security())
		for _, b := range blessings {
			perms.Add(security.BlessingPattern(b), string(v23mt.Admin))
		}
	}
	perms.Normalize()

	return op(ctx, func() error {
		n, err := mt.accessNodeCreate(ctx, mt.suffix, call.Security())
		if err != nil {
			return err
		}
		if err := mt.authorize(ctx, call.Security(), n.permissions, v23mt.Admin); err != nil {
			return err
		}
		if version != "" && n.version != version {
			return verror.NewErrBadVersion(ctx)
		}
		return n.setPermissions(ctx, perms)
	})
}

// GetPermissions returns the complete, current Permissions for an object. The
// returned version can be passed to a subsequent call to SetPermissions for
// optimistic concurrency control. A successful call to SetPermissions will
// invalidate version, and the client must call GetPermissions again to get
// the current version.
func (mt *mounttable) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	ctx.VI(2).Infof("%q.GetPermissions()", mt.suffix)

	var n *mtNode
	if n, err = mt.accessNodeRead(ctx, mt.suffix, call.Security()); err != nil {
		return
	}
	return n.permissions, n.version, nil
}

func (mt *mounttable) authorize(ctx *context.T, call security.Call, perms access.Permissions, tags ...v23mt.Tag) error {
	if l, r := call.LocalBlessings().PublicKey(), call.RemoteBlessings().PublicKey(); l != nil && r != nil && reflect.DeepEqual(l, r) {
		return nil
	}
	blessings, invalid := mt.remoteBlessingNames(ctx, call)
	for _, tag := range tags {
		if acl, exists := perms[string(tag)]; exists && acl.Includes(blessings...) {
			return nil
		}
	}
	if mt.config.GlobalAcl.Includes(blessings...) {
		return nil
	}
	return access.NewErrNoPermissions(ctx, blessings, invalid, string(tags[0]))
}

// accessNode returns the requested mounttable node after verifying that the
// caller is authorized to access it. Missing nodes are created if 'create' is
// true and the caller has 'Create' permission on the parent.
//
// If withServers is true, returns the first node within 'name' that has
// mounted servers, and the remaining part is returned as a string.
//
// Example:
//   a -> b -> c
// If b has a mounted server,
//  accessNode(.., "a/b", false, true, ..) returns (b, "")
//  accessNode(.., "a/b/c", false, true, ..) returns (b, "c")
//  accessNode(.., "a/b", false, false, ..) returns (b, "")
//  accessNode(.., "a/b/c", false, false, ..) returns (c, "")
func (mt *mounttable) accessNode(ctx *context.T, name string, create, withServers bool, call security.Call) (*mtNode, string, error) {
start:
	var (
		n, parent *mtNode
		current   string
		elems     = append([]string{""}, strings.Split(name, "/")...)
	)

	for len(elems) > 0 {
		if err := shouldAbort(ctx); err != nil {
			return nil, "", err
		}

		elem := elems[0]
		elems = elems[1:]
		current = naming.Join(current, elem)

		var err error
		n, err = getNode(ctx, mt.bt, current)

		switch {
		case err != nil:
			return nil, "", err
		case n == nil:
			if parent == nil {
				return nil, "", verror.New(naming.ErrNoSuchNameRoot, ctx, current)
			}
			if !create {
				return nil, "", verror.New(naming.ErrNoSuchName, ctx, current)
			}
			if err := mt.authorize(ctx, call, parent.permissions, v23mt.Create, v23mt.Admin); err != nil {
				return nil, "", err
			}
			var perms access.Permissions
			if perms = templatePermissions(parent.permissions, elem); perms == nil {
				perms = parent.permissions.Copy()
				if current == name {
					names, _ := mt.remoteBlessingNames(ctx, call)
					for _, n := range names {
						perms.Add(security.BlessingPattern(n), string(v23mt.Admin))
					}
				}
			}
			perms.Normalize()
			n, err = parent.createChild(ctx, elem, perms, pickCreator(ctx, call), mt.config.MaxNodesPerUser)
			if err != nil {
				return nil, "", err
			}
		default:
			deleted, err := n.gc(ctx)
			if deleted || verror.ErrorID(err) == errConcurrentAccess.ID {
				goto start
			}
			if err != nil {
				return nil, "", err
			}
		}
		if err := mt.authorize(ctx, call, n.permissions, v23mt.Resolve, v23mt.Read, v23mt.Admin); err != nil {
			return nil, "", err
		}
		if withServers && len(n.servers) > 0 {
			return n, naming.Join(elems...), nil
		}
		parent = n
	}
	if withServers {
		return nil, "", nil
	}
	return n, "", nil
}

func (mt *mounttable) accessNodeRead(ctx *context.T, name string, call security.Call) (*mtNode, error) {
	n, _, err := mt.accessNode(ctx, name, false, false, call)
	return n, err
}

func (mt *mounttable) accessNodeCreate(ctx *context.T, name string, call security.Call) (*mtNode, error) {
	n, _, err := mt.accessNode(ctx, name, true, false, call)
	return n, err
}

func (mt *mounttable) accessNodeWithServers(ctx *context.T, name string, call security.Call) (*mtNode, string, error) {
	return mt.accessNode(ctx, name, false, true, call)
}

func templatePermissions(perms access.Permissions, child string) access.Permissions {
	var templatePerms access.Permissions
	for tag, acl := range perms {
		if !strings.HasPrefix(tag, "%%/") {
			continue
		}
		tmpAcl := access.AccessList{
			In:    make([]security.BlessingPattern, len(acl.In)),
			NotIn: make([]string, len(acl.NotIn)),
		}
		for i, b := range acl.In {
			tmpAcl.In[i] = security.BlessingPattern(strings.Replace(string(b), "%%", child, -1))
		}
		for i, b := range acl.NotIn {
			tmpAcl.NotIn[i] = strings.Replace(b, "%%", child, -1)
		}
		if templatePerms == nil {
			templatePerms = make(access.Permissions)
		}
		templatePerms[strings.TrimPrefix(tag, "%%/")] = tmpAcl
	}
	return templatePerms
}

func pickCreator(ctx *context.T, call security.Call) string {
	return conventions.GetClientUserIds(ctx, call)[0]
}
