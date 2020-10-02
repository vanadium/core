// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mounttablelib implements utilities for mounttable implementations.
package mounttablelib

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/mounttable"
	"v.io/v23/verror"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/timekeeper"
)

const defaultMaxNodesPerUser = 1000
const maxNameElementLen = 512

var (
	traverseTags = []mounttable.Tag{mounttable.Read, mounttable.Resolve, mounttable.Create, mounttable.Admin}
	createTags   = []mounttable.Tag{mounttable.Create, mounttable.Admin}
	removeTags   = []mounttable.Tag{mounttable.Admin}
	mountTags    = []mounttable.Tag{mounttable.Mount, mounttable.Admin}
	resolveTags  = []mounttable.Tag{mounttable.Read, mounttable.Resolve, mounttable.Admin}
	globTags     = []mounttable.Tag{mounttable.Read, mounttable.Admin}
	setTags      = []mounttable.Tag{mounttable.Admin}
	getTags      = []mounttable.Tag{mounttable.Admin, mounttable.Read}
	allTags      = []mounttable.Tag{mounttable.Read, mounttable.Resolve, mounttable.Admin, mounttable.Mount, mounttable.Create}
)

type persistence interface {
	persistPerms(name, creator string, perm *VersionedPermissions) error
	persistDelete(name string) error
	close()
}

// mountTable represents a namespace.  One exists per server instance.
type mountTable struct {
	sync.Mutex
	root               *node
	superUsers         access.AccessList
	persisting         bool
	persist            persistence
	nodeCounter        *stats.Integer
	serverCounter      *stats.Integer
	perUserNodeCounter *stats.Map
	perUserRPCCounter  *stats.Map
	maxNodesPerUser    int64
	slm                *serverListManager
}

var _ rpc.Dispatcher = (*mountTable)(nil)

// mountContext represents a client bind.  The name is the name that was bound to.
type mountContext struct {
	name  string
	elems []string // parsed elements of name
	mt    *mountTable
}

// mount represents a single mount point.  It contains the rooted names of all servers mounted
// here.  The servers are considered equivalent, i.e., RPCs to a name below this
// point can be sent to any of these servers.
type mount struct {
	servers *serverList
	mt      bool
	leaf    bool
}

// node is a single point in the tree representing the mount table.
type node struct {
	sync.RWMutex
	parent              *node
	mount               *mount
	children            map[string]*node
	vPerms              *VersionedPermissions
	permsTemplate       access.Permissions
	explicitPermissions bool
	creator             string
}

type callContext struct {
	ctx          *context.T
	call         security.Call
	self         bool                        // true if client and server are the same.
	rbn          []string                    // remote blessing names to avoid authenticating on every check.
	rejected     []security.RejectedBlessing // rejected remote blessing names.
	create       bool                        // true if we are to create traversed nodes.
	creator      string
	ignorePerms  bool
	ignoreLimits bool
}

const createMissingNodes = true

const templateVar = "%%"

// NewMountTableDispatcher creates a new server that uses the AccessLists specified in
// permissions file for authorization.
//
// permsFile is a JSON-encoded mapping from paths in the mounttable to the
// access.Permissions for that path. The tags used in the map are the typical
// access tags (the Tag type defined in v.io/v23/security/access).
//
// persistDir is the directory for persisting Permissions.
//
// statsPrefix is the prefix for for exported statistics objects.
func NewMountTableDispatcher(ctx *context.T, permsFile, persistDir, statsPrefix string) (rpc.Dispatcher, error) {
	return NewMountTableDispatcherWithClock(ctx, permsFile, persistDir, statsPrefix, timekeeper.RealTime())
}
func NewMountTableDispatcherWithClock(ctx *context.T, permsFile, persistDir, statsPrefix string, clock timekeeper.TimeKeeper) (rpc.Dispatcher, error) {
	mt := &mountTable{
		root:               new(node),
		nodeCounter:        stats.NewInteger(naming.Join(statsPrefix, "num-nodes")),
		serverCounter:      stats.NewInteger(naming.Join(statsPrefix, "num-mounted-servers")),
		perUserNodeCounter: stats.NewMap(naming.Join(statsPrefix, "num-nodes-per-user")),
		perUserRPCCounter:  stats.NewMap(naming.Join(statsPrefix, "num-rpcs-per-user")),
		maxNodesPerUser:    defaultMaxNodesPerUser,
		slm:                newServerListManager(clock),
	}
	mt.root.parent = mt.newNode() // just for its lock
	if persistDir != "" {
		mt.persist = newPersistentStore(ctx, mt, persistDir)
		mt.persisting = mt.persist != nil
	}
	if err := mt.parsePermFile(ctx, permsFile); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("perms file %v invalid: %v", permsFile, err)
	}
	return mt, nil
}

// newNode creates a new node, and updates the number of nodes.
func (mt *mountTable) newNode() *node {
	mt.nodeCounter.Incr(1)
	return new(node)
}

// deleteNode deletes a node and all its children, and updates the number of
// nodes.
func (mt *mountTable) deleteNode(parent *node, child string) {
	// Assumes that parent and parent[child] are locked.

	// Walk the tree and count the number of nodes deleted.
	first := parent.children[child]
	if first == nil {
		return
	}
	delete(parent.children, child)
	mt.credit(first)
	nodeCount := int64(0)
	serverCount := int64(0)
	queue := []*node{first}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		nodeCount++
		serverCount += numServers(n)
		if n != first {
			n.Lock()
		}
		for k, ch := range n.children {
			queue = append(queue, ch)
			delete(n.children, k)
			mt.credit(ch)
		}
		if n != first {
			n.Unlock()
		}
	}

	mt.nodeCounter.Incr(-nodeCount)
	mt.serverCounter.Incr(-serverCount)
}

// Lookup implements rpc.Dispatcher.Lookup.
func (mt *mountTable) Lookup(ctx *context.T, name string) (interface{}, security.Authorizer, error) {
	ctx.VI(2).Infof("*********************Lookup %s", name)
	ms := &mountContext{
		name: name,
		mt:   mt,
	}
	if len(name) > 0 {
		ms.elems = strings.Split(name, "/")
	}
	return mounttable.MountTableServer(ms), ms, nil
}

// isActive returns true if a mount has unexpired servers attached.
func (m *mount) isActive(mt *mountTable) bool {
	if m == nil {
		return false
	}
	numLeft, numRemoved := m.servers.removeExpired()
	if numRemoved > 0 {
		mt.serverCounter.Incr(int64(-numRemoved))
	}
	return numLeft > 0
}

// satisfies returns no error if the ctx + n.vPerms satisfies the associated one of the required Tags.
func (n *node) satisfies(mt *mountTable, cc *callContext, tags []mounttable.Tag) error {
	// Nothing to check.
	if cc.ignorePerms || tags == nil || n.vPerms == nil {
		return nil
	}
	// Match client's blessings against the AccessLists.
	for _, tag := range tags {
		if al, exists := n.vPerms.AccessListForTag(string(tag)); exists && al.Includes(cc.rbn...) {
			return nil
		}
	}
	if mt.superUsers.Includes(cc.rbn...) {
		return nil
	}
	if len(cc.rejected) > 0 {
		return verror.ErrNoAccess.Errorf(cc.ctx, "access denied: %v: rejected blessings: %v", cc.rbn, cc.rejected)
	}
	return verror.ErrNoAccess.Errorf(cc.ctx, "access denied: %v", cc.rbn)
}

func expand(al access.AccessList, name string) *access.AccessList {
	newAccessList := new(access.AccessList)
	for _, bp := range al.In {
		newAccessList.In = append(newAccessList.In, security.BlessingPattern(strings.ReplaceAll(string(bp), templateVar, name)))
	}
	for _, bp := range al.NotIn {
		newAccessList.NotIn = append(newAccessList.NotIn, strings.ReplaceAll(bp, templateVar, name))
	}
	return newAccessList
}

// satisfiesTemplate returns no error if the ctx + n.permsTemplate satisfies the associated one of
// the required Tags.
func (n *node) satisfiesTemplate(cc *callContext, tags []mounttable.Tag, name string) error {
	// If no template, ignore.
	if cc.ignorePerms || n.permsTemplate == nil {
		return nil
	}
	// Match client's blessings against the AccessLists.
	for _, tag := range tags {
		if al, exists := n.permsTemplate[string(tag)]; exists && expand(al, name).Includes(cc.rbn...) {
			return nil
		}
	}
	return verror.ErrNoAccess.Errorf(cc.ctx, "access denied: %v: rejected blessings: %v", cc.rbn, cc.rejected)
}

// CopyPermissions copies one node's permissions to another and adds the clients blessings as
// patterns to the Admin tag.
func CopyPermissions(cc *callContext, cur *node) *VersionedPermissions {
	if cur.vPerms == nil {
		return nil
	}
	vPerms := cur.vPerms.Copy()
	if cc.rbn == nil {
		return vPerms
	}
	for _, b := range cc.rbn {
		vPerms.Add(security.BlessingPattern(b), string(mounttable.Admin))
	}
	vPerms.P.Normalize()
	return vPerms
}

// createVersionedPermissionsFromTemplate creates a new VersionedPermissions from the template subsituting name for %% everywhere.
func createVersionedPermissionsFromTemplate(perms access.Permissions, name string) *VersionedPermissions {
	vPerms := NewVersionedPermissions()
	for tag, al := range perms {
		vPerms.P[tag] = *expand(al, name)
	}
	return vPerms
}

// traverse returns the node for the path represented by elems.  If none exists and create is false, return nil.
// Otherwise create the path and return a pointer to the terminal node.  If a mount point is encountered
// while following the path, return that node and any remaining elems.
//
// If it returns a node, both the node and its parent are locked.
func (mt *mountTable) traverse(cc *callContext, elems []string) (*node, []string, error) {
	// Invariant is that the current node and its parent are both locked.
	cur := mt.root
	cur.parent.Lock()
	cur.Lock()
	for i, e := range elems {
		cc.ctx.VI(2).Infof("satisfying %v %v", elems[0:i], cur)
		if err := cur.satisfies(mt, cc, traverseTags); err != nil {
			cur.parent.Unlock()
			cur.Unlock()
			return nil, nil, err
		}
		// If we hit another mount table, we're done.
		if cur.mount.isActive(mt) {
			return cur, elems[i:], nil
		}
		// Walk the children looking for a match.
		c, ok := cur.children[e]
		if ok {
			cur.parent.Unlock()
			cur = c
			cur.Lock()
			continue
		}
		if !cc.create {
			cur.parent.Unlock()
			cur.Unlock()
			return nil, nil, nil
		}
		// Create a new node and keep recursing.
		cur.parent.Unlock()
		if err := cur.satisfies(mt, cc, createTags); err != nil {
			cur.Unlock()
			return nil, nil, err
		}
		if err := cur.satisfiesTemplate(cc, createTags, e); err != nil {
			cur.Unlock()
			return nil, nil, err
		}
		// Obey account limits.
		var err error
		if err = mt.debit(cc); err != nil {
			cur.Unlock()
			return nil, nil, err
		}
		// At this point cur is still locked, OK to use and change it.
		next := mt.newNode()
		next.creator = cc.creator
		next.parent = cur
		if cur.permsTemplate != nil {
			next.vPerms = createVersionedPermissionsFromTemplate(cur.permsTemplate, e)
		} else {
			next.vPerms = CopyPermissions(cc, cur)
		}
		if cur.children == nil {
			cur.children = make(map[string]*node)
		}
		cur.children[e] = next
		cur = next
		cur.Lock()
	}
	// Only way out of the loop is via a return or exhausting all elements.  In
	// the latter case both cur and cur.parent are locked.
	return cur, nil, nil
}

// findNode finds a node in the table and optionally creates a path to it.
//
// If a node is found, on return it and its parent are locked.
func (mt *mountTable) findNode(cc *callContext, elems []string, tags, ptags []mounttable.Tag) (*node, error) {
	n, nelems, err := mt.traverse(cc, elems)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, nil
	}
	if len(nelems) > 0 {
		n.parent.Unlock()
		n.Unlock()
		return nil, nil
	}
	// Either the node has to satisfy tags or the parent has to satisfy ptags.
	if err := n.satisfies(mt, cc, tags); err != nil {
		if ptags == nil {
			n.parent.Unlock()
			n.Unlock()
			return nil, err
		}
		if err := n.parent.satisfies(mt, cc, ptags); err != nil {
			n.parent.Unlock()
			n.Unlock()
			return nil, err
		}
	}
	return n, nil
}

// findMountPoint returns the first mount point encountered in the path and
// any elements remaining of the path.
//
// If a mountpoint is found, on return it and its parent are locked.
func (mt *mountTable) findMountPoint(cc *callContext, elems []string) (*node, []string, error) {
	n, nelems, err := mt.traverse(cc, elems)
	if err != nil {
		return nil, nil, err
	}
	if n == nil {
		return nil, nil, nil
	}
	// If we can't resolve it, we can't use it.
	if err := n.satisfies(mt, cc, resolveTags); err != nil {
		n.parent.Unlock()
		n.Unlock()
		return nil, nil, err
	}
	if !n.mount.isActive(mt) {
		removed := n.removeUseless(mt)
		n.parent.Unlock()
		n.Unlock()
		// If we removed the node, see if we can remove any of its
		// ascendants.
		if removed && len(elems) > 0 {
			mt.removeUselessRecursive(cc, elems[:len(elems)-1])
		}
		return nil, nil, nil
	}
	return n, nelems, nil
}

// Authorize verifies that the client has access to the requested node.
// Since we do the check at the time of access, we always return OK here.
func (ms *mountContext) Authorize(*context.T, security.Call) error {
	return nil
}

// ResolveStep returns the next server in a resolution in the form of a MountEntry.  The name
// in the mount entry is the name relative to the server's root.
func (ms *mountContext) ResolveStep(ctx *context.T, call rpc.ServerCall) (entry naming.MountEntry, err error) {
	ctx.VI(2).Infof("ResolveStep %q", ms.name)
	mt, cc := ms.newCallContext(ctx, call.Security(), !createMissingNodes)
	// Find the next mount point for the name.
	n, elems, werr := mt.findMountPoint(cc, ms.elems)
	if werr != nil {
		err = werr
		return
	}
	if n == nil {
		entry.Name = ms.name
		if len(ms.elems) == 0 {
			err = naming.ErrNoSuchNameRoot.Errorf(ctx, "namespace root name %s doesn't exist", ms.name)
		} else {
			err = naming.ErrNoSuchName.Errorf(ctx, "name %s doesn't exist", ms.name)
		}
		return
	}
	n.parent.Unlock()
	defer n.Unlock()
	entry.Servers = n.mount.servers.copyToSlice()
	entry.Name = strings.Join(elems, "/")
	entry.ServesMountTable = n.mount.mt
	entry.IsLeaf = n.mount.leaf
	return
}

func hasMTFlag(flags naming.MountFlag) bool {
	return (flags & naming.MT) == naming.MT
}

func hasLeafFlag(flags naming.MountFlag) bool {
	return (flags & naming.Leaf) == naming.Leaf
}

func hasReplaceFlag(flags naming.MountFlag) bool {
	return (flags & naming.Replace) == naming.Replace
}

func numServers(n *node) int64 {
	if n == nil || n.mount == nil || n.mount.servers == nil {
		return 0
	}
	return int64(n.mount.servers.len())
}

// This isn't a storage system.
func checkElementLengths(ctx *context.T, elems []string) error {
	for _, e := range elems {
		if len(e) > maxNameElementLen {
			return fmt.Errorf("path element %v...: too long (max len is %v)", e[:32], maxNameElementLen)
		}
	}
	return nil
}

// Mount a server onto the name in the receiver.
func (ms *mountContext) Mount(ctx *context.T, call rpc.ServerCall, server string, ttlsecs uint32, flags naming.MountFlag) error {
	ctx.VI(2).Infof("*********************Mount %q -> %s", ms.name, server)
	if err := checkElementLengths(ctx, ms.elems); err != nil {
		return err
	}
	mt, cc := ms.newCallContext(ctx, call.Security(), createMissingNodes)
	if ttlsecs == 0 {
		ttlsecs = 10 * 365 * 24 * 60 * 60 // a really long time
	}

	// Make sure the server address is reasonable.
	epString := server
	if naming.Rooted(server) {
		epString, _ = naming.SplitAddressName(server)
	}
	_, err := naming.ParseEndpoint(epString)
	if err != nil {
		return fmt.Errorf("malformed address %v for mounted server %v", epString, server)
	}

	// Find/create node in namespace and add the mount.
	n, werr := mt.findNode(cc, ms.elems, mountTags, nil)
	if werr != nil {
		return werr
	}
	if n == nil {
		return naming.ErrNoSuchNameRoot.Errorf(ctx, "namespace root name %s doesn't exist", ms.name)
	}
	// We don't need the parent lock
	n.parent.Unlock()
	defer n.Unlock()

	wantMT := hasMTFlag(flags)
	wantLeaf := hasLeafFlag(flags)
	if n.mount != nil {
		if wantMT != n.mount.mt {
			return fmt.Errorf("mount table doesn't match")
		}
		if wantLeaf != n.mount.leaf {
			return fmt.Errorf("leaf doesn't match")
		}
	}
	// Remove any existing children.
	for child := range n.children {
		mt.deleteNode(n, child)
	}

	nServersBefore := numServers(n)
	if hasReplaceFlag(flags) {
		n.mount = nil
	}
	if n.mount == nil {
		n.mount = &mount{servers: mt.slm.newServerList(), mt: wantMT, leaf: wantLeaf}
	}
	n.mount.servers.add(server, time.Duration(ttlsecs)*time.Second)
	mt.serverCounter.Incr(numServers(n) - nServersBefore)
	return nil
}

// removeUseless removes a node and all of its ascendants that are not useful.
//
// We assume both n and n.parent are locked.
func (n *node) removeUseless(mt *mountTable) bool {
	if len(n.children) > 0 || n.mount.isActive(mt) || n.explicitPermissions {
		return false
	}
	for k, c := range n.parent.children {
		if c == n {
			mt.deleteNode(n.parent, k)
			break
		}
	}
	return true
}

// removeUselessRecursive removes any useless nodes on the tail of the path.
func (mt *mountTable) removeUselessRecursive(cc *callContext, elems []string) {
	for i := len(elems); i > 0; i-- {
		n, nelems, _ := mt.traverse(cc, elems[:i])
		if n == nil {
			break
		}
		if nelems != nil {
			n.parent.Unlock()
			n.Unlock()
			break
		}
		removed := n.removeUseless(mt)
		n.parent.Unlock()
		n.Unlock()
		if !removed {
			break
		}
	}
}

// Unmount removes servers from the name in the receiver. If server is specified, only that
// server is removed.
func (ms *mountContext) Unmount(ctx *context.T, call rpc.ServerCall, server string) error {
	ctx.VI(2).Infof("*********************Unmount %q, %s", ms.name, server)
	mt, cc := ms.newCallContext(ctx, call.Security(), !createMissingNodes)
	n, err := mt.findNode(cc, ms.elems, mountTags, nil)
	if err != nil {
		return err
	}
	if n == nil {
		return nil
	}
	nServersBefore := numServers(n)
	if server == "" {
		n.mount = nil
	} else if n.mount != nil && n.mount.servers.remove(server) == 0 {
		n.mount = nil
	}
	mt.serverCounter.Incr(numServers(n) - nServersBefore)
	removed := n.removeUseless(mt)
	n.parent.Unlock()
	n.Unlock()
	if removed {
		// If we removed the node, see if we can also remove
		// any of its ascendants.
		mt.removeUselessRecursive(cc, ms.elems[:len(ms.elems)-1])
	}
	return nil
}

// Delete removes the receiver.  If all is true, any subtree is also removed.
func (ms *mountContext) Delete(ctx *context.T, call rpc.ServerCall, deleteSubTree bool) error {
	ctx.VI(2).Infof("*********************Delete %q, %v", ms.name, deleteSubTree)
	mt, cc := ms.newCallContext(ctx, call.Security(), !createMissingNodes)
	if len(ms.elems) == 0 {
		// We can't delete the root.
		return fmt.Errorf("cannot delete root node")
	}
	// Find and lock the parent node and parent node.  Either the node or its parent has
	// to satisfy removeTags.
	n, err := mt.findNode(cc, ms.elems, removeTags, removeTags)
	if err != nil {
		return err
	}
	if n == nil {
		return nil
	}
	defer n.parent.Unlock()
	defer n.Unlock()
	if !deleteSubTree && len(n.children) > 0 {
		return fmt.Errorf("cannot delete %v: has children", ms.name)
	}
	mt.deleteNode(n.parent, ms.elems[len(ms.elems)-1])
	if mt.persisting {
		mt.persist.persistDelete(ms.name) //nolint:errcheck
	}
	return nil
}

// globStep is called with n and n.parent locked.  Returns with both unlocked.
func (mt *mountTable) globStep(cc *callContext, n *node, name string, pattern *glob.Glob, gCall rpc.GlobServerCall) { //nolint:gocyclo
	if shouldAbort(cc) {
		n.parent.Unlock()
		n.Unlock()
		return
	}
	cc.ctx.VI(2).Infof("globStep(%s, %s)", name, pattern)

	// Globing is the lowest priority so we give up the cpu often.
	runtime.Gosched()

	// If this is a mount point, we're done.
	if m := n.mount; m != nil {
		removed := n.removeUseless(mt)
		if removed {
			n.parent.Unlock()
			n.Unlock()
			return
		}
		// Don't need the parent lock anymore.
		n.parent.Unlock()
		me := naming.MountEntry{
			Name: name,
		}
		// Only fill in the mount info if we can resolve this name.
		if err := n.satisfies(mt, cc, resolveTags); err == nil {
			me.Servers = m.servers.copyToSlice()
			me.ServesMountTable = n.mount.mt
			me.IsLeaf = n.mount.leaf
		} else {
			me.Servers = []naming.MountedServer{}
		}
		// Hold no locks while we are sending on the channel to avoid livelock.
		n.Unlock()
		gCall.SendStream().Send(naming.GlobReplyEntry{Value: me}) //nolint:errcheck
		return
	}

	if !pattern.Empty() {
		// We can only list children to whom we have some access AND either
		// - we have Read or Admin access to the directory or
		// - we have Resolve or Create access to the directory and the
		//    next element in the pattern is a fixed string.
		if err := n.satisfies(mt, cc, globTags); err != nil {
			if err := n.satisfies(mt, cc, traverseTags); err != nil {
				goto out
			}
			fixed, _ := pattern.SplitFixedElements()
			if len(fixed) == 0 {
				goto out
			}
		}

		// Since we will be unlocking the node,
		// we need to grab the list of children before any unlocking.
		children := make(map[string]*node, len(n.children))
		for k, c := range n.children {
			children[k] = c
		}
		n.parent.Unlock()

		// Recurse through the children.
		matcher, suffix := pattern.Head(), pattern.Tail()
		for k, c := range children {
			if shouldAbort(cc) {
				n.Unlock()
				return
			}
			// At this point, n lock is held.
			if matcher.Match(k) {
				c.Lock()
				// If child allows any access show it.  Otherwise, skip.
				if err := c.satisfies(mt, cc, allTags); err != nil {
					c.Unlock()
					continue
				}
				mt.globStep(cc, c, naming.Join(name, k), suffix, gCall)
				n.Lock()
			}
		}
		// Relock the node and its parent in the correct order to avoid deadlock.
		// Safe to access n.parent when its unlocked because it never changes.
		n.Unlock()
		n.parent.Lock()
		n.Lock()
	}

out:
	// Remove if no longer useful.
	if n.removeUseless(mt) || pattern.Len() != 0 {
		n.parent.Unlock()
		n.Unlock()
		return
	}

	// To see anything, one has to have some access to the node.  Don't need the parent lock anymore.
	n.parent.Unlock()
	if err := n.satisfies(mt, cc, allTags); err != nil {
		n.Unlock()
		return
	}
	// Hold no locks while we are sending on the channel to avoid livelock.
	n.Unlock()
	// Intermediate nodes are marked as serving a mounttable since they answer the mounttable methods.
	gCall.SendStream().Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: name, ServesMountTable: true}}) //nolint:errcheck
}

// Glob finds matches in the namespace.  If we reach a mount point before matching the
// whole pattern, return that mount point.
//
// pattern is a glob pattern as defined by the v.io/x/ref/lib/glob package.
//
// To avoid livelocking an application, Glob grabs and releases locks as it descends the tree
// and holds no locks while writing to the channel.  As such a glob can interleave with other
// operations that add or remove nodes.  The result returned by glob may, therefore, represent
// a state that never existed in the mounttable.  For example, if someone removes c/d and later
// adds a/b while a Glob is in progress, the Glob may return a set of nodes that includes both
// c/d and a/b.
//nolint:golint // API change required.
func (ms *mountContext) Glob__(ctx *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	ctx.VI(2).Infof("mt.Glob %v", ms.elems)
	mt, cc := ms.newCallContext(ctx, call.Security(), !createMissingNodes)
	// If there was an access error, just ignore the entry, i.e., make it invisible.
	n, err := mt.findNode(cc, ms.elems, nil, nil)
	if err != nil {
		return nil
	}
	// If the current name is not fully resolvable on this nameserver we
	// don't need to evaluate the glob expression. Send a partially resolved
	// name back to the client.
	if n == nil {
		ms.linkToLeaf(cc, call)
		return nil
	}
	mt.globStep(cc, n, "", g, call)
	return nil
}

func (ms *mountContext) linkToLeaf(cc *callContext, gCall rpc.GlobServerCall) {
	n, elems, err := ms.mt.findMountPoint(cc, ms.elems)
	if err != nil || n == nil {
		return
	}
	n.parent.Unlock()
	servers := n.mount.servers.copyToSlice()
	for i, s := range servers {
		servers[i].Server = naming.Join(s.Server, strings.Join(elems, "/"))
	}
	n.Unlock()
	gCall.SendStream().Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: "", Servers: servers}}) //nolint:errcheck
}

func (ms *mountContext) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	ctx.VI(2).Infof("SetPermissions %q", ms.name)
	if err := checkElementLengths(ctx, ms.elems); err != nil {
		return err
	}
	mt, cc := ms.newCallContext(ctx, call.Security(), createMissingNodes)

	// Find/create node in namespace and add the mount.
	n, err := mt.findNode(cc, ms.elems, setTags, nil)
	if err != nil {
		return err
	}
	if n == nil {
		// TODO(p): can this even happen?
		return naming.ErrNoSuchName.Errorf(ctx, "name %s doesn't exist", ms.name)
	}
	n.parent.Unlock()
	defer n.Unlock()

	// If the caller is trying to add a Permission that they are no longer Admin in,
	// retain the caller's blessings that were in Admin to prevent them from locking themselves out.
	if al, ok := perms[string(mounttable.Admin)]; !ok || !al.Includes(cc.rbn...) {
		_, oldPerms := n.vPerms.Get()
		if oldPerms == nil {
			for _, bname := range cc.rbn {
				perms.Add(security.BlessingPattern(bname), string(mounttable.Admin))
			}
		} else {
			oldAl := oldPerms[string(mounttable.Admin)]
			for _, bname := range cc.rbn {
				if oldAl.Includes(bname) {
					perms.Add(security.BlessingPattern(bname), string(mounttable.Admin))
				}
			}
		}
	}
	perms.Normalize()

	n.vPerms, err = n.vPerms.Set(ctx, version, perms)
	if err == nil {
		if mt.persisting {
			mt.persist.persistPerms(ms.name, n.creator, n.vPerms) //nolint:errcheck
		}
		n.explicitPermissions = true
	}
	return err
}

func (ms *mountContext) GetPermissions(ctx *context.T, call rpc.ServerCall) (access.Permissions, string, error) {
	ctx.VI(2).Infof("GetPermissions %q", ms.name)
	mt, cc := ms.newCallContext(ctx, call.Security(), !createMissingNodes)

	// Find node in namespace and add the mount.
	n, err := mt.findNode(cc, ms.elems, getTags, nil)
	if err != nil {
		return nil, "", err
	}
	if n == nil {
		return nil, "", naming.ErrNoSuchName.Errorf(ctx, "name %s doesn't exist", ms.name)
	}
	n.parent.Unlock()
	defer n.Unlock()
	version, perms := n.vPerms.Get()
	return perms, version, nil
}

// credit user for node deletion.
func (mt *mountTable) credit(n *node) {
	mt.perUserNodeCounter.Incr(n.creator, -1)
}

// debit user for node creation.
func (mt *mountTable) debit(cc *callContext) error {
	count, ok := mt.perUserNodeCounter.Incr(cc.creator, 1).(int64)
	if !ok {
		return fmt.Errorf("user has exceeded their node limit")
	}
	if count > mt.maxNodesPerUser && !cc.ignoreLimits {
		mt.perUserNodeCounter.Incr(cc.creator, -1)
		return fmt.Errorf("user has exceeded their node limit")
	}
	return nil
}

func shouldAbort(cc *callContext) bool {
	select {
	case <-cc.ctx.Done():
		return true
	default:
		return false
	}
}

func (ms *mountContext) newCallContext(ctx *context.T, call security.Call, create bool) (*mountTable, *callContext) {
	cc := &callContext{ctx: ctx, call: call, create: create}
	if call != nil {
		if l, r := cc.call.LocalBlessings().PublicKey(), cc.call.RemoteBlessings().PublicKey(); l != nil && reflect.DeepEqual(l, r) {
			cc.self = true
		}
		cc.rbn, cc.rejected = security.RemoteBlessingNames(ctx, call)
	}
	cc.creator = ms.mt.pickCreator(cc.ctx, cc.call)
	ms.mt.perUserRPCCounter.Incr(cc.creator, 1)
	return ms.mt, cc
}
