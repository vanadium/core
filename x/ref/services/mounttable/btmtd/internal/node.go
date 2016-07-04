// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/cloud/bigtable"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	vdltime "v.io/v23/vdlroot/time"
	"v.io/v23/verror"

	"v.io/x/ref/lib/timekeeper"
)

var gcGracePeriod = time.Minute

// SetGcGracePeriod sets the grace period for garbage collecting newly created
// nodes. Nodes are not eligible for garbage collection until after this time
// has passed. This function exists only for testing purposes.
func SetGcGracePeriod(p time.Duration) {
	gcGracePeriod = p
}

type mtNode struct {
	bt             *BigTable
	id             string
	name           string
	sticky         bool
	creationTime   bigtable.Timestamp
	permissions    access.Permissions
	version        string
	creator        string
	mountFlags     mtFlags
	servers        []naming.MountedServer
	expiredServers []string
	children       []child
}

type mtFlags struct {
	MT   bool `json:"mt,omitempty"`
	Leaf bool `json:"leaf,omitempty"`
}

type child string

func newChild(id, name string) (child, error) {
	if len(id) != 8 {
		return child(""), fmt.Errorf("expected id to be 8 characters: %q", id)
	}
	return child(id + name), nil
}

func childFromCol(col string) (child, error) {
	if len(col) <= 8 {
		return child(""), fmt.Errorf("expected col to be more than 8 characters: %q", col)
	}
	return child(col), nil
}

func (c child) id() string {
	return string(c)[:8]
}

func (c child) name() string {
	return string(c)[8:]
}

func longTimeout(ctx *context.T) (*context.T, func()) {
	return context.WithTimeout(v23.GetBackgroundContext(ctx), time.Hour)
}

func getNode(ctx *context.T, bt *BigTable, name string) (*mtNode, error) {
	row, err := bt.readRow(ctx, rowKey(name), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, err
	}
	return nodeFromRow(ctx, bt, row, clock), nil
}

func rowKey(name string) string {
	// The row key is a hash of the node name followed by the node name
	// itself.
	// This spreads the rows evenly across the tablet servers to avoid
	// traffic imbalance.
	name = naming.Clean("/" + name)
	h := fnv.New32()
	h.Write([]byte(name))
	return hex.EncodeToString(h.Sum(nil)) + name
}

func nodeFromRow(ctx *context.T, bt *BigTable, row bigtable.Row, clock timekeeper.TimeKeeper) *mtNode {
	const offset = 9 // 32-bit value in hex + '/'
	name := row.Key()
	if len(name) < offset {
		return nil
	}
	n := &mtNode{
		bt:   bt,
		name: name[offset:],
	}
	for _, i := range row[metadataFamily] {
		col := strings.TrimPrefix(i.Column, metadataFamily+":")
		switch col {
		case idColumn:
			n.id = string(i.Value)
		case stickyColumn:
			n.sticky = true
		case versionColumn:
			n.version = string(i.Value)
		case permissionsColumn:
			if err := json.Unmarshal(i.Value, &n.permissions); err != nil {
				ctx.Errorf("Failed to decode permissions for %s", name)
				return nil
			}
		case creatorColumn:
			n.creationTime = i.Timestamp
			n.creator = string(i.Value)
		}
	}
	n.servers = make([]naming.MountedServer, 0, len(row[serversFamily]))
	for _, i := range row[serversFamily] {
		deadline := i.Timestamp.Time()
		server := i.Column[2:]
		if deadline.Before(clock.Now()) {
			n.expiredServers = append(n.expiredServers, server)
			continue
		}
		if err := json.Unmarshal(i.Value, &n.mountFlags); err != nil {
			ctx.Errorf("Failed to decode mount flags for %s", name)
			return nil
		}
		n.servers = append(n.servers, naming.MountedServer{
			Server:   server,
			Deadline: vdltime.Deadline{deadline},
		})
	}
	n.children = make([]child, 0, len(row[childrenFamily]))
	for _, i := range row[childrenFamily] {
		childCol := strings.TrimPrefix(i.Column, childrenFamily+":")
		if child, err := childFromCol(childCol); err != nil {
			ctx.Errorf("childFromCol(%q) failed: %v", childCol, err)
		} else {
			n.children = append(n.children, child)
		}
	}
	return n
}

func (n *mtNode) createChild(ctx *context.T, childName string, perms access.Permissions, creator string, limit int64) (*mtNode, error) {
	child, err := newChild(n.version, childName)
	if err != nil {
		return nil, err
	}
	mut := bigtable.NewMutation()
	mut.Set(childrenFamily, string(child), bigtable.ServerTime, []byte{1})
	if err := n.mutate(ctx, mut, false); err != nil {
		return nil, err
	}

	// If the current process dies right here, it will leave the parent with
	// a reference to a child row that doesn't exist. This means that the
	// parent will never be seen as "empty" and will not be garbage
	// collected. This will be corrected when:
	//  - the child is created again, or
	//  - the parent is forcibly deleted with Delete().

	childFullName := naming.Join(n.name, childName)
	longCtx, cancel := longTimeout(ctx)
	defer cancel()
	if err := n.bt.createRow(longCtx, childFullName, perms, creator, child, limit); err != nil {
		mut = bigtable.NewMutation()
		mut.DeleteCellsInColumn(childrenFamily, string(child))
		if err := n.bt.apply(ctx, rowKey(n.name), mut); err != nil {
			ctx.Errorf("Failed to delete child reference. Parent=%q Col=%q Err=%v", n.name, string(child), err)
		}
		return nil, err
	}
	// Delete any stale references to the child that we just successfully
	// created.
	mut = nil
	for _, c := range n.children {
		if c.name() != childName {
			continue
		}
		if mut == nil {
			mut = bigtable.NewMutation()
		}
		mut.DeleteCellsInColumn(childrenFamily, string(c))
	}
	if mut != nil {
		if err := n.bt.apply(ctx, rowKey(n.name), mut); err != nil {
			ctx.Errorf("Failed to delete child reference. Parent=%q Err=%v", n.name, err)
		}
	}

	// Return the new child node.
	cn, err := getNode(ctx, n.bt, childFullName)
	if err != nil {
		return nil, err
	}
	if cn == nil {
		return nil, verror.New(errConcurrentAccess, ctx, childFullName)
	}
	return cn, nil
}

func (n *mtNode) mount(ctx *context.T, server string, deadline time.Time, flags naming.MountFlag, limit int64) error {
	delta := int64(1)
	mut := bigtable.NewMutation()
	for _, s := range n.servers {
		// Mount replaces an already mounted server with the same name,
		// or all servers if the Replace flag is set.
		if s.Server != server && flags&naming.Replace == 0 {
			continue
		}
		delta--
		mut.DeleteCellsInColumn(serversFamily, s.Server)
	}
	if err := incrementCreatorServerCount(ctx, n.bt, n.creator, delta, limit); err != nil {
		return err
	}
	defer func() {
		if err := incrementCreatorServerCount(ctx, n.bt, n.creator, -delta, 0); err != nil {
			ctx.Errorf("incrementCreatorServerCount failed: %v", err)
		}
	}()
	f := mtFlags{
		MT:   flags&naming.MT != 0,
		Leaf: flags&naming.Leaf != 0,
	}
	jsonValue, err := json.Marshal(f)
	if err != nil {
		return err
	}
	mut.Set(serversFamily, server, n.bt.time(deadline), jsonValue)
	if err := n.mutate(ctx, mut, false); err != nil {
		return err
	}
	delta = 0
	return nil
}

func (n *mtNode) unmount(ctx *context.T, server string) error {
	delta := int64(0)
	mut := bigtable.NewMutation()
	for _, s := range n.servers {
		// Unmount removes the specified server, or all servers if
		// server == "".
		if server != "" && s.Server != server {
			continue
		}
		delta--
		mut.DeleteCellsInColumn(serversFamily, s.Server)
	}
	if delta == 0 {
		return nil
	}
	if err := n.mutate(ctx, mut, false); err != nil {
		return err
	}
	if n, err := getNode(ctx, n.bt, n.name); err == nil {
		n.gc(ctx)
	}
	return incrementCreatorServerCount(ctx, n.bt, n.creator, delta, 0)
}

func (n *mtNode) gc(ctx *context.T) (deletedSomething bool, err error) {
	for n != nil && n.name != "" {
		if len(n.expiredServers) > 0 {
			mut := bigtable.NewMutation()
			for _, s := range n.expiredServers {
				mut.DeleteCellsInColumn(serversFamily, s)
			}
			if err = n.mutate(ctx, mut, false); err != nil {
				break
			}
			delta := -int64(len(n.expiredServers))
			if err = incrementCreatorServerCount(ctx, n.bt, n.creator, delta, 0); err != nil {
				// TODO(rthellend): Since counters are stored in different rows,
				// there is no way to update them atomically, e.g. if the server
				// dies here, or if incrementCreatorServerCount returns an error,
				// the server counter will be off.
				// The same thing could happen everywhere the counters are updated.
				// If/when we start using these counters for quota enforcement, we
				// should also come up with a way to make sure the counters aren't
				// too far off.
				break
			}
			deletedSomething = true
			break
		}
		if n.sticky || len(n.children) > 0 || len(n.servers) > 0 {
			break
		}
		if time.Since(n.creationTime.Time()) < gcGracePeriod {
			break
		}
		if err = n.delete(ctx, false); err != nil {
			break
		}
		ctx.VI(2).Infof("Deleted empty node %q", n.name)
		deletedSomething = true
		parent := path.Dir(n.name)
		if parent == "." {
			break
		}
		if n, err = getNode(ctx, n.bt, parent); err != nil {
			break
		}
	}
	return
}

func (n *mtNode) deleteAndGC(ctx *context.T, deleteSubtree bool) error {
	if err := n.delete(ctx, deleteSubtree); err != nil {
		return err
	}
	parentName, _ := path.Split(n.name)
	if parent, err := getNode(ctx, n.bt, parentName); err == nil {
		parent.gc(ctx)
	}
	return nil
}

func (n *mtNode) delete(ctx *context.T, deleteSubtree bool) error {
	if !deleteSubtree && len(n.children) > 0 {
		return verror.New(errNotEmpty, ctx, n.name)
	}

	// TODO(rthellend): This naive approach could be very expensive in
	// terms of memory. A smarter, but slower, approach would be to walk
	// the tree without holding on to all the node data.
	for _, c := range n.children {
		cn, err := getNode(ctx, n.bt, naming.Join(n.name, c.name()))
		if err != nil {
			return err
		}
		if cn == nil {
			// Node 'n' has a reference to a child that doesn't
			// exist. It could be that it is being created or
			// deleted concurrently. To be sure, we have to create
			// it before deleting it.
			if cn, err = n.createChild(ctx, c.name(), n.permissions, "", 0); err != nil {
				return err
			}
		}
		if err := cn.delete(ctx, true); err != nil {
			return err
		}
	}

	mut := bigtable.NewMutation()
	mut.DeleteRow()
	if err := n.mutate(ctx, mut, true); err != nil {
		return err
	}

	// If the current process dies right here, it will leave the parent with
	// a reference to a child row that no longer exists. This means that the
	// parent will never be seen as "empty" and will not be garbage
	// collected. This will be corrected when:
	//  - the child is re-created, or
	//  - the parent is forcibly deleted with Delete().

	// Delete from parent node.
	parent, child := path.Split(n.name)
	mut = bigtable.NewMutation()
	mut.DeleteCellsInColumn(childrenFamily, n.id+child)

	longCtx, cancel := longTimeout(ctx)
	defer cancel()
	if err := n.bt.apply(longCtx, rowKey(parent), mut); err != nil {
		return err
	}
	// Adjust the server count for the node creator. Since delete() can be
	// called directly without gc(), we need to include the expired servers
	// in the counter adjustment.
	delta := -int64(len(n.servers) + len(n.expiredServers))
	if err := incrementCreatorServerCount(ctx, n.bt, n.creator, delta, 0); err != nil {
		return err
	}
	return incrementCreatorNodeCount(ctx, n.bt, n.creator, -1, 0)
}

func (n *mtNode) setPermissions(ctx *context.T, perms access.Permissions) error {
	jsonPerms, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	mut := bigtable.NewMutation()
	mut.Set(metadataFamily, permissionsColumn, bigtable.ServerTime, jsonPerms)
	mut.Set(metadataFamily, stickyColumn, bigtable.ServerTime, []byte{1})
	if err := n.mutate(ctx, mut, false); err != nil {
		return err
	}
	return nil
}

func (n *mtNode) mutate(ctx *context.T, mut *bigtable.Mutation, delete bool) error {
	if !delete {
		v, err := strconv.ParseUint(n.version, 16, 32)
		if err != nil {
			return err
		}
		newVersion := fmt.Sprintf("%08x", uint32(v)+1)
		mut.Set(metadataFamily, versionColumn, bigtable.ServerTime, []byte(newVersion))
	}

	// The mutation will succeed iff the row already exists with the
	// expected version.
	filter := bigtable.ChainFilters(
		bigtable.FamilyFilter(metadataFamily),
		bigtable.ColumnFilter(versionColumn),
		bigtable.LatestNFilter(1),
		bigtable.ValueFilter(n.version),
	)
	condMut := bigtable.NewCondMutation(filter, mut, nil)
	var success bool
	if err := n.bt.apply(ctx, rowKey(n.name), condMut, bigtable.GetCondMutationResult(&success)); err != nil {
		return err
	}
	if !success {
		return verror.New(errConcurrentAccess, ctx, n.name)
	}
	return nil
}

func createNodesFromFile(ctx *context.T, bt *BigTable, fileName string) error {
	var nodes map[string]access.Permissions
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &nodes); err != nil {
		return err
	}
	// This loop adds backward compatibility with the older template format,
	// e.g. "a/b/%%" { "Admin": { "In": [ "%%" ] } }
	// With the new format, this is equivalent to:
	// "a/b" { "%%/Admin": { "In": [ "%%" ] } }
	for node, perms := range nodes {
		if strings.HasSuffix(node, "/%%") {
			delete(nodes, node)
			node = strings.TrimSuffix(node, "/%%")
			p := nodes[node]
			if p == nil {
				p = make(access.Permissions)
			}
			for tag := range perms {
				p["%%/"+tag] = perms[tag]
			}
			nodes[node] = p
		}
	}

	// Create the nodes in alphanumeric order so that children are
	// created after their parents.
	sortedNodes := []string{}
	for node := range nodes {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Strings(sortedNodes)

	for _, node := range sortedNodes {
		perms := nodes[node]
		if node == "" {
			child, err := newChild("ROOTNODE", "")
			if err != nil {
				return err
			}
			if err := bt.createRow(ctx, "", perms, "", child, 0); err != nil {
				return err
			}
			continue
		}
		parentName := ""
		for _, e := range strings.Split(node, "/") {
			n, err := getNode(ctx, bt, naming.Join(parentName, e))
			if err != nil {
				return err
			}
			if n == nil {
				parent, err := getNode(ctx, bt, parentName)
				if err != nil {
					return err
				}
				if n, err = parent.createChild(ctx, e, parent.permissions, "", 0); err != nil {
					return err
				}
			}
			if n.name == node {
				// setPermissions also makes the node sticky.
				if err := n.setPermissions(ctx, perms); err != nil {
					return err
				}
			}
			parentName = n.name
		}
	}
	return nil
}

func (n *mtNode) checkInvariants(ctx *context.T, fix bool) error {
	// Is this an orphan node, i.e. either the parent doesn't exist, or the
	// parent doesn't have a reference to it?
	if n.name != "" {
		parentName, childName := path.Split(n.name)
		pn, err := getNode(ctx, n.bt, parentName)
		if err != nil {
			return err
		}
		orphan := true
		if pn != nil {
			for _, c := range pn.children {
				if c.name() == childName && c.id() == n.id {
					orphan = false
					break
				}
			}
		}
		if orphan {
			if fix {
				if err := n.delete(ctx, true); err != nil {
					return err
				}
				return fmt.Errorf("deleted orphan node %q", n.name)
			}
			return fmt.Errorf("found orphan node %q", n.name)
		}
	}
	// Does this node have references to nodes that don't exis?
	var mut *bigtable.Mutation
	for _, c := range n.children {
		cn, err := getNode(ctx, n.bt, naming.Join(n.name, c.name()))
		if err != nil {
			return err
		}
		if cn != nil && cn.id == c.id() {
			continue
		}
		if mut == nil {
			mut = bigtable.NewMutation()
		}
		mut.DeleteCellsInColumn(childrenFamily, string(c))
	}
	if mut != nil {
		if fix {
			if err := n.mutate(ctx, mut, false); err != nil {
				return err
			}
			return fmt.Errorf("deleted dangling reference on %q", n.name)
		}
		return fmt.Errorf("found dangling reference on %q", n.name)
	}
	return nil
}
