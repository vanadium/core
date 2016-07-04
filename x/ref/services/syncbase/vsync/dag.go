// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Syncbase DAG (directed acyclic graph) utility functions.
//
// The DAG is used to track the version history of synced objects in order
// to detect and resolve conflicts (concurrent changes on different devices).
//
// Note: the sync code uses the words "object" and "object ID" (oid) as a
// generic way to refer to syncable entities, whether they are actual user data
// (collection row and its row key), collection headers (permissions entry and
// its internal key based on the collection id), or other metadata such as
// syncgroups (syncgroup value and its internal key based on the syncgroup ID).
//
// * Object IDs are globally unique across all devices.
// * Syncable objects have version numbers associated with their mutations.
// * For a given object ID, the version number is globally unique across all
//   devices, i.e. the (oid, version) tuple is globally unique.
// * Each (oid, version) tuple is represented by a node in the DAG.
// * The previous version of an object is its parent in the DAG, i.e. the
//   new version is derived from that parent.
// * DAG nodes have child-to-parent pointers.
// * When there are no conflicts, the parent node has a single child node
//   that points to it.
// * When a parent node has more than one child, this indicates concurrent
//   mutations which are treated as a conflict to be resolved.
// * When a conflict is resolved, the new version has pointers back to each of
//   the two parents to indicate that it is derived from both nodes.
// * During a sync operation from a source device to a target device, the
//   target receives a DAG fragment from the source.  That fragment has to
//   be incorporated (grafted) into the target device's DAG.  It may be a
//   continuation of the DAG of an object, with the attachment (graft) point
//   being the current head of DAG, in which case there are no conflicts.
//   Or the graft point(s) may be older nodes, which means the new fragment
//   is a divergence in the graph causing a conflict that must be resolved
//   in order to re-converge the two DAG fragments.
//
// In the diagrams below:
// (h) represents the head node in the local device.
// (nh) represents the new head node received from the remote device.
// (g) represents a graft node, where new nodes attach to the existing DAG.
// <- represents a derived-from mutation, i.e. a child-to-parent pointer
//
// a- No-conflict example: the new nodes (v4, v5) attach to the head node (v3).
//    In this case the new head becomes the head node, the new DAG fragment
//    being a continuation of the existing DAG.
//
//    Before:
//    v1 <- v2 <- v3(h)
//
//    Sync updates applied, no conflict detected:
//    v1 <- v2 <- v3(h,g) <- v4 <- v5 (nh)
//
//    After:
//    v1 <- v2 <- v3 <- v4 <- v5 (h)
//
// b- Conflict example: the new nodes (v4, v5) attach to an old node (v2).
//    The current head node (v3) and the new head node (v5) are divergent
//    (concurrent) mutations that need to be resolved.  The conflict
//    resolution function is passed the old head (v3), new head (v5), and
//    the common ancestor (v2).  It resolves the conflict with (v6) which
//    is represented in the DAG as derived from both v3 and v5 (2 parents).
//
//    Before:
//    v1 <- v2 <- v3(h)
//
//    Sync updates applied, conflict detected (v3 not a graft node):
//    v1 <- v2(g) <- v3(h)
//                <- v4 <- v5 (nh)
//
//    After: conflict resolver creates v6 having 2 parents (v3, v5):
//    v1 <- v2(g) <- v3 <------- v6(h)
//                <- v4 <- v5 <-
//
// The DAG does not grow indefinitely.  During a sync operation each device
// learns what the other device already knows -- where it's at in the version
// history for the objects.  When a device determines that all devices that
// sync an object (members of matching syncgroups) have moved past some version
// for that object, the DAG for that object can be pruned up to that common
// version, deleting all prior (ancestor) nodes.
//
// The DAG contains three tables persisted to disk (nodes, heads, batches):
//
//   * nodes:   one entry per (oid, version) with references to parent node(s)
//              it is derived from, a reference to the log record identifying
//              that mutation, a reference to its write batch (if any), and a
//              boolean to indicate whether this was an object deletion.
//
//   * heads:   one entry per object pointing to its most recent version.
//
//   * batches: one entry per batch ID containing the set of objects in the
//              write batch and their versions.

import (
	"container/list"
	"fmt"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
)

const (
	NoVersion = ""
	NoBatchId = uint64(0)
)

// batchSet holds information on a set of write batches.
type batchSet map[uint64]*BatchInfo

// graftMap holds the state of DAG node grafting (attaching) per object.  It
// holds a store handle to use when reading the object heads during grafting
// operations.  This avoids contaminating the transaction read-set of the
// caller (typically the Initiator storing newly received deltas).
type graftMap struct {
	objGraft map[string]*graftInfo
	st       store.Store
}

// graftInfo holds the state of an object's node grafting in the DAG.
// It is ephemeral (in-memory), used during a single sync operation to track
// where the new DAG fragments are attached to the existing DAG for the object:
// - newNodes:    the set of newly added nodes; used to detect the type of edges
//                between nodes (new-node to old-node or vice versa).
// - newHeads:    the set of new candidate head nodes; used to detect conflicts.
// - graftNodes:  the set of old nodes on which new nodes were added, and their
//                level in the DAG; used to find common ancestors for conflicts.
// - oldHeadSnap: snapshot of the current local head known by sync, used in
//                conflict detection, particularly when conflict detection needs
//                to be retried due to sync dag state being stale compared
//                to local store.
//
// After the received mutations are applied, if there are two heads in the
// newHeads set, there is a conflict to be resolved for the object.  Otherwise,
// if there is one head, no conflict was triggered and the new head becomes the
// current object version.  In case of conflict, the graftNodes set is used to
// select a common ancestor.
// TODO(rdaoud): support open DAGs to handle delayed conflict resolution by
// tracking multiple dangling remote heads in addition to the local head node.
type graftInfo struct {
	newNodes    map[string]bool
	newHeads    map[string]bool
	graftNodes  map[string]uint64
	oldHeadSnap string
}

// newBatchInfo allocates and initializes a batch info entry.
func newBatchInfo() *BatchInfo {
	return &BatchInfo{
		Objects:       make(map[string]string),
		LinkedObjects: make(map[string]string),
		Count:         0,
	}
}

// startBatch marks the start of a batch.  It generates a batch ID and returns
// it to the caller, if an ID is not given.  The batch ID is used to track DAG
// nodes that are part of the same batch and it is stored in the log records.
// If a batch ID is given by the caller, its information is accessed.
func (s *syncService) startBatch(ctx *context.T, st store.StoreReader, btid uint64) uint64 {
	s.batchesLock.Lock()
	defer s.batchesLock.Unlock()

	// If no batch ID is given, generate a new unused one.
	if btid == NoBatchId {
		for (btid == NoBatchId) || (s.batches[btid] != nil) {
			btid = s.rand64()
		}

		s.batches[btid] = newBatchInfo()
		return btid
	}

	// Use the given batch ID and, if needed, refetch its in-memory entry
	// from the store.  It is OK not to find it in the store; it means sync
	// is learning about this batch ID the first time from another sync.
	if s.batches[btid] == nil {
		info, err := getBatch(ctx, st, btid)
		if err != nil {
			info = newBatchInfo()
		}
		s.batches[btid] = info
	}

	return btid
}

// addNodeToBatch adds a node (oid, version) to a batch under construction.
func (s *syncService) addNodeToBatch(ctx *context.T, btid uint64, oid, version string) error {
	s.batchesLock.Lock()
	defer s.batchesLock.Unlock()

	if btid == NoBatchId {
		return verror.New(verror.ErrInternal, ctx, "invalid batch id", btid)
	}

	info := s.batches[btid]
	if info == nil {
		return verror.New(verror.ErrInternal, ctx, "unknown batch id", btid)
	}

	info.Objects[oid] = version
	return nil
}

// addLinkToBatch adds the child node (oid, version) in a link to a batch under
// construction.
func (s *syncService) addLinkToBatch(ctx *context.T, btid uint64, oid, version string) error {
	s.batchesLock.Lock()
	defer s.batchesLock.Unlock()

	if btid == NoBatchId {
		return verror.New(verror.ErrInternal, ctx, "invalid batch id", btid)
	}

	info := s.batches[btid]
	if info == nil {
		return verror.New(verror.ErrInternal, ctx, "unknown batch id", btid)
	}

	info.LinkedObjects[oid] = version
	return nil
}

// endBatch marks the end of a given batch.  The batch information is persisted
// to the store and removed from the temporary in-memory entry.
func (s *syncService) endBatch(ctx *context.T, tx store.Transaction, btid, count uint64) error {
	s.batchesLock.Lock()
	defer s.batchesLock.Unlock()

	if btid == NoBatchId || count == 0 {
		return verror.New(verror.ErrInternal, ctx, "invalid batch info", btid, count)
	}

	info := s.batches[btid]
	if info == nil {
		return verror.New(verror.ErrInternal, ctx, "unknown batch id", btid)
	}

	// The first time a batch is ended, info.Count is zero.  Subsequently,
	// if this batch ID is started and ended again, info.Count should be
	// the same as the "count" value given.
	if info.Count != 0 && info.Count != count {
		return verror.New(verror.ErrInternal, ctx, "wrong counts for batch", btid, info.Count, count)
	}

	// Only save non-empty batches.
	if len(info.Objects) > 0 {
		info.Count = count
		if err := setBatch(ctx, tx, btid, info); err != nil {
			return err
		}
	}

	delete(s.batches, btid)
	return nil
}

// addNode adds a new node for a DAG object, linking it to its parent nodes.
// It verifies that the parent nodes are valid.  If a batch ID is given, track
// the node membership in the batch.
//
// Note: an in-memory grafting structure is passed to track DAG attachments
// during a sync operation.  This is needed when nodes are being added due to
// remote changes fetched by the sync protocol.  The Initiator allocates a
// grafting structure at the start of a sync operation and passes it across
// calls to addNode() to update the DAG grafting state:
// - If a parent node is not new, mark it as a DAG graft point.
// - Mark this version as a new node.
// - Update the new head node pointer of the grafted DAG.
//
// The grafting structure is not needed when nodes are being added locally by
// the Watcher, passing a nil grafting structure.
func (s *syncService) addNode(ctx *context.T, tx store.Transaction, oid, version, logrec string,
	deleted bool, parents []string, btid uint64, graft *graftMap) error {
	if parents != nil {
		if len(parents) > 2 {
			return verror.New(verror.ErrInternal, ctx, "cannot have more than 2 parents")
		}
		if len(parents) == 0 {
			parents = nil // replace an empty array with a nil
		}
	}

	// Verify the parents, determine the node level.  Also save the levels
	// of the parent nodes for later in this function in graft updates.
	parentLevels := make(map[string]uint64)
	var level uint64
	for _, parent := range parents {
		pnode, err := getNode(ctx, tx, oid, parent)
		if err != nil {
			return err
		}
		parentLevels[parent] = pnode.Level
		if level <= pnode.Level {
			level = pnode.Level + 1
		}
	}

	// If a batch ID is given, add the node to that batch.
	if btid != NoBatchId {
		if err := s.addNodeToBatch(ctx, btid, oid, version); err != nil {
			return err
		}
	}

	// Add the node entry to the DAG.
	node := &DagNode{
		Level:   level,
		Parents: parents,
		Logrec:  logrec,
		BatchId: btid,
		Deleted: deleted,
	}
	if err := setNode(ctx, tx, oid, version, node); err != nil {
		return err
	}

	// We are done if grafting is not being tracked (a local node add).
	if graft == nil {
		return nil
	}

	// Get the object's graft info entry in order to update it.  It happens
	// when addNode() is called by the sync Initiator and the DAG is updated
	// with new nodes fetched from other devices.
	//
	// During a sync operation, each mutated object gets new nodes added in
	// its DAG.  These new nodes are either derived from nodes that were
	// previously known on this device (i.e. their parent nodes are pre-
	// existing, or they have no parents (new root nodes)), or they are
	// derived from other new DAG nodes being discovered during this sync
	// (i.e. their parent nodes were also just added to the DAG).
	//
	// To detect a conflict and find the most recent common ancestor to
	// pass to the conflict resolver, the DAG graft info keeps track of the
	// new nodes that have old parent nodes.  These old-to-new edges are
	// points where new DAG fragments are attached (grafted) onto the
	// existing DAG.  The old nodes are the graft nodes forming the set of
	// common ancestors to use in conflict resolution:
	//
	// 1- A conflict happens when the current "head node" for an object is
	//    not in the set of graft nodes.  It means the object mutations
	//    were not derived from what the device knows, but are divergent
	//    changes at a prior point.
	//
	// 2- The most recent common ancestor to use in resolving the conflict
	//    is the object graft node with the deepest level (furthest from
	//    the root node), representing the most up-to-date common knowledge
	//    between the devices.
	info := getObjectGraftInfo(ctx, tx, graft, oid)

	for _, parent := range parents {
		// If this parent is an old node, it's a graft point.
		if !info.newNodes[parent] {
			info.graftNodes[parent] = parentLevels[parent]
		}

		// A parent cannot be a candidate for a new head.
		delete(info.newHeads, parent)
	}

	// This new node is a candidate for new head version.
	info.newNodes[version] = true
	info.newHeads[version] = true
	return nil
}

// addParent adds to the DAG node (oid, version) linkage to this parent node.
//
// Note: as with the addNode() call, an in-memory grafting structure is passed
// to track DAG attachements during a sync operation.  It is not needed if the
// parent linkage is due to a local change (from conflict resolution selecting
// an existing version).
func (s *syncService) addParent(ctx *context.T, tx store.Transaction, oid, version, parent string, btid uint64, graft *graftMap) error {
	if version == parent {
		return verror.New(verror.ErrInternal, ctx, "object", oid, version, "cannot be its own parent")
	}

	node, err := getNode(ctx, tx, oid, version)
	if err != nil {
		return err
	}
	pnode, err := getNode(ctx, tx, oid, parent)
	if err != nil {
		return err
	}

	// If a batch ID is given, add the child node in the link to that batch.
	if btid != NoBatchId {
		if err := s.addLinkToBatch(ctx, btid, oid, version); err != nil {
			return err
		}
	}

	// Check if the parent is already linked to this node.
	found := false
	for _, p := range node.Parents {
		if p == parent {
			found = true
			break
		}
	}

	// Add the parent if it is not yet linked.
	if !found {
		// Make sure that adding the link does not create a DAG cycle.
		// Verify that the node is not an ancestor of the parent that
		// it is being linked to.
		err = forEachAncestor(ctx, tx, oid, pnode.Parents, func(v string, nd *DagNode) error {
			if v == version {
				return verror.New(verror.ErrInternal, ctx, "cycle on object",
					oid, ": node", version, "is ancestor of parent", parent)
			}
			return nil
		})
		if err != nil {
			return err
		}
		node.Parents = append(node.Parents, parent)
		if err = setNode(ctx, tx, oid, version, node); err != nil {
			return err
		}
	}

	// If no grafting structure is given (i.e. local changes), we are done.
	if graft == nil {
		return nil
	}

	// Update graft: if the node and its parent are new/old or old/new then
	// add the parent as a graft point (a potential common ancestor).
	info := getObjectGraftInfo(ctx, tx, graft, oid)

	_, nodeNew := info.newNodes[version]
	_, parentNew := info.newNodes[parent]
	if (nodeNew && !parentNew) || (!nodeNew && parentNew) {
		info.graftNodes[parent] = pnode.Level
	}

	// The parent node can no longer be a candidate for a new head version.
	delete(info.newHeads, parent)
	return nil
}

// moveHead moves the object head node in the DAG.
func moveHead(ctx *context.T, tx store.Transaction, oid, head string) error {
	// Verify that the node exists.
	if ok, err := hasNode(ctx, tx, oid, head); err != nil {
		return err
	} else if !ok {
		return verror.New(verror.ErrInternal, ctx, "node", oid, head, "does not exist")
	}

	return setHead(ctx, tx, oid, head)
}

// hasConflict determines if an object has a conflict between its new and old
// head nodes.
// - Yes: return (true, newHead, oldHead, ancestor)   -- from a common past
// - Yes: return (true, newHead, oldHead, NoVersion)  -- from disjoint pasts
// - No:  return (false, newHead, oldHead, NoVersion) -- no conflict
//
// A conflict exists when there are two new-head nodes in the graft structure.
// It means the newly added object versions are not derived in part from this
// device's current knowledge. A conflict also exists if the snapshotted local
// head is different from the current local head. If there is a single new-head
// and the snapshot head is the same as the current local head, the object
// changes were applied without triggering a conflict.
func hasConflict(ctx *context.T, st store.StoreReader, oid string, graft *graftMap) (isConflict bool, newHead, oldHead, ancestor string, err error) {
	isConflict = false
	oldHead = NoVersion
	newHead = NoVersion
	ancestor = NoVersion
	err = nil

	if graft == nil {
		err = verror.New(verror.ErrInternal, ctx, "no DAG graft map given")
		return
	}

	info := graft.objGraft[oid]
	if info == nil {
		err = verror.New(verror.ErrInternal, ctx, "node", oid, "has no DAG graft info")
		return
	}

	numHeads := len(info.newHeads)
	if numHeads < 1 || numHeads > 2 {
		err = verror.New(verror.ErrInternal, ctx, "node", oid, "invalid count of new heads", numHeads)
		return
	}

	// Fetch the current head for this object if it exists.  The error from
	// getHead() is ignored because a newly received object is not yet known
	// on this device and will not trigger a conflict.
	oldHead, _ = getHead(ctx, st, oid)

	// If there is only one new head node and the snapshotted old head is
	// still unchanged, there is no conflict. The new head is that single
	// one, even if it might also be the same old node.
	if numHeads == 1 {
		for head := range info.newHeads {
			newHead = head
		}
		if newHead == info.oldHeadSnap {
			// Only link log records could've been received.
			newHead = oldHead
			return
		} else if oldHead == info.oldHeadSnap {
			return
		}
	}

	// The new head is the non-old one.
	for head := range info.newHeads {
		if head != info.oldHeadSnap {
			newHead = head
			break
		}
	}

	// There wasn't a conflict at the old snapshot, but now there is. The
	// snapshotted head is the common ancestor.
	isConflict = true
	if numHeads == 1 {
		vlog.VI(4).Infof("sync: hasConflict: old graft snapshot %v, head %s", graft, oldHead)
		ancestor = info.oldHeadSnap
		return
	}

	// There is a conflict: the best choice ancestor is the graft node with
	// the largest level (farthest from the root).  It is possible in some
	// corner cases to have multiple graft nodes at the same level.  This
	// would still be a single conflict, but the multiple same-level graft
	// nodes representing equivalent conflict resolutions on different
	// devices that are now merging their resolutions.  In such a case it
	// does not matter which node is chosen as the ancestor because the
	// conflict resolver function is assumed to be convergent.  However it
	// is nicer to make that selection deterministic so all devices see the
	// same choice: the version number is used as a tie-breaker.
	// Note: for the case of a conflict from disjoint pasts, there are no
	// graft nodes (empty set) and thus no common ancestor because the two
	// DAG fragments were created from distinct root nodes.  The "NoVersion"
	// value is returned as the ancestor.
	var maxLevel uint64
	for node, level := range info.graftNodes {
		if maxLevel < level || (maxLevel == level && ancestor < node) {
			maxLevel = level
			ancestor = node
		}
	}
	return
}

// newGraft allocates a graftMap to track DAG node grafting during sync.  It is
// given a handle to a store to use for its own reading of object head nodes.
func newGraft(st store.Store) *graftMap {
	return &graftMap{
		objGraft: make(map[string]*graftInfo),
		st:       st,
	}
}

// getObjectGraft returns the graftInfo for an object ID.  If the graftMap is
// nil, a nil graftInfo is returned because grafting is not being tracked.
func getObjectGraftInfo(ctx *context.T, sntx store.SnapshotOrTransaction, graft *graftMap, oid string) *graftInfo {
	if graft == nil {
		return nil
	}
	if info := graft.objGraft[oid]; info != nil {
		return info
	}

	info := &graftInfo{
		newNodes:   make(map[string]bool),
		newHeads:   make(map[string]bool),
		graftNodes: make(map[string]uint64),
	}

	// If the object has a head node, include it in the set of new heads.
	// Note: use the store handle of the graftMap if available to avoid
	// contaminating the read-set of the caller's transaction.  Otherwise
	// use the caller's transaction.
	var st store.StoreReader
	st = graft.st
	if st == nil {
		st = sntx
	}
	if head, err := getHead(ctx, st, oid); err == nil {
		info.newHeads[head] = true
		info.oldHeadSnap = head
	}

	graft.objGraft[oid] = info
	return info
}

// forEachAncestor loops over the DAG ancestor nodes of an object in a breadth-
// first traversal starting from given version nodes.  It calls the given
// callback function once for each ancestor node.
func forEachAncestor(ctx *context.T, st store.StoreReader, oid string, startVersions []string, callback func(version string, node *DagNode) error) error {
	visited := make(map[string]bool)
	queue := list.New()
	for _, version := range startVersions {
		queue.PushBack(version)
		visited[version] = true
	}

	for queue.Len() > 0 {
		version := queue.Remove(queue.Front()).(string)
		node, err := getNode(ctx, st, oid, version)
		if err != nil {
			// Ignore it, the parent was previously pruned.
			continue
		}
		for _, parent := range node.Parents {
			if !visited[parent] {
				queue.PushBack(parent)
				visited[parent] = true
			}
		}
		if err = callback(version, node); err != nil {
			return err
		}
	}
	return nil
}

// newBatchPruning allocates an in-memory structure to track batches affected
// by a DAG pruning operation across objects.
func newBatchPruning() batchSet {
	return make(batchSet)
}

// prune trims the DAG of an object at a given version (node) by deleting all
// its ancestor nodes, making it the new root node.  For each deleted node it
// calls the given callback function to delete its log record.  If NoVersion
// is given instead, then all object nodes are deleted, including the head node.
//
// Note: this function is typically used when sync determines that all devices
// that know about this object have gotten past this version, as part of its
// GC operations.  It can also be used when an object history is obliterated,
// for example when destroying a syncgroup, which is also versioned and tracked
// in the DAG.
//
// The batch set passed is used to track batches affected by the deletion of DAG
// objects across multiple calls to prune().  It is later given to pruneDone()
// to do GC on these batches.
func prune(ctx *context.T, tx store.Transaction, oid, version string, batches batchSet, delLogRec func(ctx *context.T, tx store.Transaction, logrec string) error) error {
	if batches == nil {
		return verror.New(verror.ErrInternal, ctx, "missing batch set")
	}

	var parents []string
	if version == NoVersion {
		// Delete all object versions including its head version.
		head, err := getHead(ctx, tx, oid)
		if err != nil {
			return err
		}
		if err := delHead(ctx, tx, oid); err != nil {
			return err
		}
		parents = []string{head}
	} else {
		// Get the node at the pruning point and set its parents to nil.
		// It will become the oldest DAG node (root) for the object.
		node, err := getNode(ctx, tx, oid, version)
		if err != nil {
			return err
		}
		if node.Parents == nil {
			// Nothing to do, this node is already the root.
			return nil
		}
		parents = node.Parents
		node.Parents = nil
		if err = setNode(ctx, tx, oid, version, node); err != nil {
			return err
		}
	}

	// Delete all ancestor nodes and their log records. Delete as many as
	// possible and track the error counts.  Update the batch set to track
	// their pruning.
	nodeErrs, logErrs := 0, 0
	forEachAncestor(ctx, tx, oid, parents, func(v string, nd *DagNode) error {
		if btid := nd.BatchId; btid != NoBatchId {
			if batches[btid] == nil {
				batches[btid] = newBatchInfo()
			}
			batches[btid].Objects[oid] = v
		}

		if err := delLogRec(ctx, tx, nd.Logrec); err != nil {
			logErrs++
		}
		if err := delNode(ctx, tx, oid, v); err != nil {
			nodeErrs++
		}
		return nil
	})
	if nodeErrs != 0 || logErrs != 0 {
		return verror.New(verror.ErrInternal, ctx,
			"prune failed to delete nodes and logs:", nodeErrs, logErrs)
	}
	return nil
}

// pruneDone is called when object pruning is finished within a single pass of
// the sync garbage collector.  It updates the batch sets affected by objects
// deleted by prune().
func pruneDone(ctx *context.T, tx store.Transaction, batches batchSet) error {
	// Update batch sets by removing the pruned objects from them.
	for btid, pruneInfo := range batches {
		info, err := getBatch(ctx, tx, btid)
		if err != nil {
			return err
		}

		for oid := range pruneInfo.Objects {
			delete(info.Objects, oid)
		}

		if len(info.Objects) > 0 {
			err = setBatch(ctx, tx, btid, info)
		} else {
			err = delBatch(ctx, tx, btid)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// getLogRecKey returns the key of the log record for a given object version.
func getLogRecKey(ctx *context.T, st store.StoreReader, oid, version string) (string, error) {
	node, err := getNode(ctx, st, oid, version)
	if err != nil {
		return "", err
	}
	return node.Logrec, nil
}

// Low-level utility functions to access DB entries without tracking their
// relationships.  Use the functions above to manipulate the DAG.

// nodeKey returns the key used to access a DAG node (oid, version).
func nodeKey(oid, version string) string {
	return common.JoinKeyParts(dagNodePrefix, oid, version)
}

// setNode stores the DAG node entry.
func setNode(ctx *context.T, tx store.Transaction, oid, version string, node *DagNode) error {
	if version == NoVersion {
		vlog.Fatalf("sync: setNode: invalid version: %s for oid: %s", version, oid)
	}

	return store.Put(ctx, tx, nodeKey(oid, version), node)
}

// getNode retrieves the DAG node entry for the given (oid, version).
func getNode(ctx *context.T, st store.StoreReader, oid, version string) (*DagNode, error) {
	if version == NoVersion {
		vlog.Fatalf("sync: getNode: invalid version: %s", version)
	}

	var node DagNode
	key := nodeKey(oid, version)
	if err := store.Get(ctx, st, key, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

// delNode deletes the DAG node entry.
func delNode(ctx *context.T, tx store.Transaction, oid, version string) error {
	if version == NoVersion {
		vlog.Fatalf("sync: delNode: invalid version: %s", version)
	}

	return store.Delete(ctx, tx, nodeKey(oid, version))
}

// hasNode returns true if the node (oid, version) exists in the DAG.
func hasNode(ctx *context.T, st store.StoreReader, oid, version string) (bool, error) {
	if version == NoVersion {
		vlog.Fatalf("sync: hasNode: invalid version: %s", version)
	}

	return store.Exists(ctx, st, nodeKey(oid, version))
}

// headKey returns the key used to access the DAG object head.
func headKey(oid string) string {
	return common.JoinKeyParts(dagHeadPrefix, oid)
}

// setHead stores version as the DAG object head.
func setHead(ctx *context.T, tx store.Transaction, oid, version string) error {
	if version == NoVersion {
		vlog.Fatalf("sync: setHead: invalid version: %s", version)
	}

	return store.Put(ctx, tx, headKey(oid), version)
}

// getHead retrieves the DAG object head.
func getHead(ctx *context.T, st store.StoreReader, oid string) (string, error) {
	var version string
	key := headKey(oid)
	if err := store.Get(ctx, st, key, &version); err != nil {
		return NoVersion, err
	}
	return version, nil
}

// delHead deletes the DAG object head.
func delHead(ctx *context.T, tx store.Transaction, oid string) error {
	return store.Delete(ctx, tx, headKey(oid))
}

// batchKey returns the key used to access the DAG batch info.
func batchKey(btid uint64) string {
	return common.JoinKeyParts(dagBatchPrefix, fmt.Sprintf("%d", btid))
}

// setBatch stores the DAG batch entry.
func setBatch(ctx *context.T, tx store.Transaction, btid uint64, info *BatchInfo) error {
	if btid == NoBatchId {
		return verror.New(verror.ErrInternal, ctx, "invalid batch id", btid)
	}

	return store.Put(ctx, tx, batchKey(btid), info)
}

// getBatch retrieves the DAG batch entry.
func getBatch(ctx *context.T, st store.StoreReader, btid uint64) (*BatchInfo, error) {
	if btid == NoBatchId {
		return nil, verror.New(verror.ErrInternal, ctx, "invalid batch id", btid)
	}

	var info BatchInfo
	key := batchKey(btid)
	if err := store.Get(ctx, st, key, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// delBatch deletes the DAG batch entry.
func delBatch(ctx *context.T, tx store.Transaction, btid uint64) error {
	if btid == NoBatchId {
		return verror.New(verror.ErrInternal, ctx, "invalid batch id", btid)
	}

	return store.Delete(ctx, tx, batchKey(btid))
}

// getParentMap is a testing and debug helper function that returns for an
// object a map of its DAG (node-to-parents relations).  If a graft structure
// is given, include its fragments in the map.
func getParentMap(ctx *context.T, st store.StoreReader, oid string, graft *graftMap) map[string][]string {
	parentMap := make(map[string][]string)
	var start []string

	if head, err := getHead(ctx, st, oid); err == nil {
		start = append(start, head)
	}
	if graft != nil && graft.objGraft[oid] != nil {
		for v := range graft.objGraft[oid].newHeads {
			start = append(start, v)
		}
	}

	forEachAncestor(ctx, st, oid, start, func(v string, nd *DagNode) error {
		parentMap[v] = nd.Parents
		return nil
	})
	return parentMap
}
