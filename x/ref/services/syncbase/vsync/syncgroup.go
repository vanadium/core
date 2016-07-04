// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Syncgroup management and storage in Syncbase.  Handles the lifecycle
// of syncgroups (create, join, leave, etc.) and their persistence as
// sync metadata in the application databases.  Provides helper functions
// to the higher levels of sync (Initiator, Watcher) to get membership
// information and map key/value changes to their matching syncgroups.

// TODO(hpucha): Add high level commentary about the logic behind create/join
// etc.

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	blob "v.io/x/ref/services/syncbase/localblobstore"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
	sbwatchable "v.io/x/ref/services/syncbase/watchable"
)

////////////////////////////////////////////////////////////
// Syncgroup management internal to Syncbase.

// memberView holds an aggregated view of all syncgroup members across
// databases. The view is not coherent, it gets refreshed according to a
// configured TTL and not (coherently) when syncgroup membership is updated in
// the various databases. It is needed by the sync Initiator, which must select
// a peer to contact from a global view of all syncgroup members gathered from
// all databases. This is why a slightly stale view is acceptable.
// The members are identified by their Vanadium names (map keys).
type memberView struct {
	expiration time.Time
	members    map[string]*memberInfo
}

// memberInfo holds the member metadata for each syncgroup this member belongs
// to within each database. It's a mapping of database ids (globally unique
// because they include app blessings) to sets of syncgroup member information.
// It also maintains all the mount table candidates that could be used to reach
// this peer, learned from the syncgroup metadata.
type memberInfo struct {
	db2sg    map[wire.Id]sgMember
	mtTables map[string]struct{}
}

// sgMember maps syncgroups to their member metadata.
type sgMember map[interfaces.GroupId]interfaces.SyncgroupMemberState

// newSyncgroupVersion generates a random syncgroup version ("etag").
func (s *syncService) newSyncgroupVersion() string {
	return fmt.Sprintf("%x", s.rand64())
}

// verifySyncgroup verifies if a Syncgroup struct is well-formed.
// TODO(rdaoud): define verrors for all ErrBadArg cases.
func verifySyncgroup(ctx *context.T, sg *interfaces.Syncgroup) error {
	if sg == nil {
		return verror.New(verror.ErrBadArg, ctx, "group information not specified")
	}
	if err := pubutil.ValidateId(sg.Id); err != nil {
		return verror.New(verror.ErrBadArg, ctx, fmt.Sprintf("invalid sg id: %v", err))
	}
	if err := pubutil.ValidateId(sg.DbId); err != nil {
		return verror.New(verror.ErrBadArg, ctx, fmt.Sprintf("invalid db id: %v", err))
	}
	if sg.Creator == "" {
		return verror.New(verror.ErrBadArg, ctx, "creator id not specified")
	}
	if sg.SpecVersion == "" {
		return verror.New(verror.ErrBadArg, ctx, "group version not specified")
	}
	if len(sg.Joiners) == 0 {
		return verror.New(verror.ErrBadArg, ctx, "group has no joiners")
	}
	return verifySyncgroupSpec(ctx, &sg.Spec)
}

// verifySyncgroupSpec verifies if a SyncgroupSpec is well-formed.
func verifySyncgroupSpec(ctx *context.T, spec *wire.SyncgroupSpec) error {
	if spec == nil {
		return verror.New(verror.ErrBadArg, ctx, "group spec not specified")
	}
	if len(spec.Collections) == 0 {
		return verror.New(verror.ErrBadArg, ctx, "group has no collections specified")
	}

	// Duplicate collections are not allowed.
	colls := make(map[wire.Id]bool, len(spec.Collections))
	for _, c := range spec.Collections {
		if err := pubutil.ValidateId(c); err != nil {
			return verror.New(verror.ErrBadArg, ctx, fmt.Sprintf("group has an invalid collection id %v", c))
		}
		colls[c] = true
	}
	if len(colls) != len(spec.Collections) {
		return verror.New(verror.ErrBadArg, ctx, "group has duplicate collections specified")
	}

	if err := common.ValidatePerms(ctx, spec.Perms, wire.AllSyncgroupTags); err != nil {
		return err
	}

	return nil
}

// sameCollections returns true if the two sets of collections are the same.
func sameCollections(coll1, coll2 []wire.Id) bool {
	collMap := make(map[wire.Id]uint8)
	for _, c := range coll1 {
		collMap[c] |= 0x01
	}
	for _, c := range coll2 {
		collMap[c] |= 0x02
	}
	for _, mask := range collMap {
		if mask != 0x03 {
			return false
		}
	}
	return true
}

// addSyncgroup adds a new syncgroup given its version and information.  This
// also includes creating a DAG node entry and updating the DAG head.  If the
// caller is the creator of the syncgroup, a local log record is also created
// using the given server ID and gen and pos counters to index the log record.
// Otherwise, it's a joiner case and the syncgroup is put in a pending state
// (waiting for its full metadata to be synchronized) and the log record is
// skipped, delaying its creation till the Initiator does p2p sync.
func (s *syncService) addSyncgroup(ctx *context.T, tx *watchable.Transaction, version string, creator bool, remotePublisher string, genvec interfaces.GenVector, servId, gen, pos uint64, sg *interfaces.Syncgroup) error {
	// Verify the syncgroup information before storing it since it may have
	// been received from a remote peer.
	if err := verifySyncgroup(ctx, sg); err != nil {
		return err
	}

	gid := SgIdToGid(sg.DbId, sg.Id)

	// Add the group ID entry.
	if ok, err := hasSGIdEntry(tx, gid); err != nil {
		return err
	} else if ok {
		return verror.New(verror.ErrExist, ctx, "group id already exists")
	}

	// By default, the priority of this device with respect to blobs is
	// low, so the "Distance" metric is non-zero.
	psg := blob.PerSyncgroup{Priority: interfaces.SgPriority{DevType: wire.BlobDevTypeNormal, Distance: 1}}
	if myInfo, ok := sg.Joiners[s.name]; ok {
		psg.Priority.DevType = int32(myInfo.MemberInfo.BlobDevType)
		psg.Priority.Distance = float32(myInfo.MemberInfo.BlobDevType)
	}
	if err := s.bst.SetPerSyncgroup(ctx, gid, &psg); err != nil {
		return err
	}

	state := SgLocalState{
		RemotePublisher: remotePublisher,
		SyncPending:     !creator,
		PendingGenVec:   genvec,
	}
	if remotePublisher == "" {
		state.NumLocalJoiners = 1
	}

	if err := setSGIdEntry(ctx, tx, gid, &state); err != nil {
		return err
	}

	// Add the syncgroup versioned data entry.
	if ok, err := hasSGDataEntry(tx, gid, version); err != nil {
		return err
	} else if ok {
		return verror.New(verror.ErrExist, ctx, "group id version already exists")
	}

	return s.updateSyncgroupVersioning(ctx, tx, gid, version, creator, servId, gen, pos, sg)
}

// updateSyncgroupVersioning updates the per-version information of a syncgroup.
// It writes a new versioned copy of the syncgroup data entry, a new DAG node,
// and updates the DAG head.  Optionally, it also writes a new local log record
// using the given server ID and gen and pos counters to index it.  The caller
// can provide the version number to use otherwise, if NoVersion is given, a new
// version is generated by the function.
// TODO(rdaoud): hook syncgroup mutations (and deletions) to the watch log so
// apps can monitor SG changes as well.
func (s *syncService) updateSyncgroupVersioning(ctx *context.T, tx *watchable.Transaction, gid interfaces.GroupId, version string, withLog bool, servId, gen, pos uint64, sg *interfaces.Syncgroup) error {
	if version == NoVersion {
		version = s.newSyncgroupVersion()
	}

	oid := sgOID(gid)

	// Add the syncgroup versioned data entry.
	if err := setSGDataEntryByOID(ctx, tx, oid, version, sg); err != nil {
		return err
	}

	var parents []string
	if head, err := getHead(ctx, tx, oid); err == nil {
		parents = []string{head}
	} else if verror.ErrorID(err) != verror.ErrNoExist.ID {
		return err
	}

	// Add a sync log record for the syncgroup if needed.
	logKey := ""
	if withLog {
		if err := addSyncgroupLogRec(ctx, tx, oid, version, parents, servId, gen, pos); err != nil {
			return err
		}
		logKey = logRecKey(oid, servId, gen)
	}

	// Add the syncgroup to the DAG.
	if err := s.addNode(ctx, tx, oid, version, logKey, false, parents, NoBatchId, nil); err != nil {
		return err
	}
	return setHead(ctx, tx, oid, version)
}

// addSyncgroupLogRec adds a new local log record for a syncgroup.
func addSyncgroupLogRec(ctx *context.T, tx *watchable.Transaction, oid, version string, parents []string, servId, gen, pos uint64) error {
	// TODO(razvanm): It might be better to have watchable.Store be responsible
	// for attaching any timestamps to log records; client code should not need
	// access to the watchable.Store's clock.
	now, err := tx.St.Clock.Now()
	if err != nil {
		return err
	}
	rec := &LocalLogRec{
		Metadata: interfaces.LogRecMetadata{
			ObjId:   oid,
			CurVers: version,
			Parents: parents,
			Delete:  false,
			UpdTime: now,
			Id:      servId,
			Gen:     gen,
			RecType: interfaces.NodeRec,
			BatchId: NoBatchId,
		},
		Pos: pos,
	}

	return putLogRec(ctx, tx, oid, rec)
}

// getSyncgroupVersion retrieves the current version of the syncgroup.
func getSyncgroupVersion(ctx *context.T, st store.StoreReader, gid interfaces.GroupId) (string, error) {
	return getHead(ctx, st, sgOID(gid))
}

// getSyncgroupById retrieves the syncgroup given its ID.
func getSyncgroupByGid(ctx *context.T, st store.StoreReader, gid interfaces.GroupId) (*interfaces.Syncgroup, error) {
	version, err := getSyncgroupVersion(ctx, st, gid)
	if err != nil {
		return nil, err
	}
	return getSGDataEntry(ctx, st, gid, version)
}

// delSyncgroupById deletes the syncgroup given its ID.
// bst may be nil.
func delSyncgroupByGid(ctx *context.T, bst blob.BlobStore, tx *watchable.Transaction, gid interfaces.GroupId) error {
	sg, err := getSyncgroupByGid(ctx, tx, gid)
	if err != nil {
		return err
	}
	return delSyncgroupBySgId(ctx, bst, tx, sg.DbId, sg.Id)
}

// delSyncgroupByName deletes the syncgroup given its name.
// bst may be nil.
func delSyncgroupBySgId(ctx *context.T, bst blob.BlobStore, tx *watchable.Transaction, dbId, sgId wire.Id) error {
	// Get the syncgroup ID and current version.
	gid := SgIdToGid(dbId, sgId)
	version, err := getSyncgroupVersion(ctx, tx, gid)
	if err != nil {
		return err
	}

	// Delete the ID entry.
	if err := delSGIdEntry(ctx, tx, gid); err != nil {
		return err
	}

	// Delete the blob-related per-syncgroup information; ignore errors
	// from this call.
	if bst != nil {
		bst.DeletePerSyncgroup(ctx, gid)
	}

	// Delete all versioned syncgroup data entries (same versions as DAG
	// nodes).  This is done separately from pruning the DAG nodes because
	// some nodes may have no log record pointing back to the syncgroup data
	// entries (loose coupling to support the pending syncgroup state).
	oid := sgOID(gid)
	err = forEachAncestor(ctx, tx, oid, []string{version}, func(v string, nd *DagNode) error {
		return delSGDataEntry(ctx, tx, gid, v)
	})
	if err != nil {
		return err
	}

	// Delete all DAG nodes and log records.
	bset := newBatchPruning()
	err = prune(ctx, tx, oid, NoVersion, bset, func(ctx *context.T, tx store.Transaction, lr string) error {
		if lr != "" {
			return store.Delete(ctx, tx, lr)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return pruneDone(ctx, tx, bset)
}

// refreshMembersIfExpired updates the aggregate view of syncgroup members
// across databases if the view has expired.
// TODO(rdaoud): track dirty dbs since the last refresh and incrementally update
// the membership view for them instead of always scanning all of them.
func (s *syncService) refreshMembersIfExpired(ctx *context.T) {
	view := s.allMembers
	if view == nil {
		// The empty expiration time in Go is before "now" and treated as expired
		// below.
		view = &memberView{expiration: time.Time{}, members: nil}
		s.allMembers = view
	}

	if time.Now().Before(view.expiration) {
		return
	}

	// Create a new aggregate view of syncgroup members across all databases.
	newMembers := make(map[string]*memberInfo)

	s.forEachDatabaseStore(ctx, func(dbId wire.Id, st *watchable.Store) bool {
		// For each database, fetch its syncgroup data entries by scanning their
		// prefix range.  Use a database snapshot for the scan.
		sn := st.NewSnapshot()
		defer sn.Abort()

		forEachSyncgroup(sn, func(gid interfaces.GroupId, sg *interfaces.Syncgroup) bool {
			// Add all members of this syncgroup to the membership view.
			// A member's info is different across syncgroups, so gather all of them.
			refreshSyncgroupMembers(gid, sg, dbId, newMembers)
			return false
		})
		return false
	})

	view.members = newMembers
	view.expiration = time.Now().Add(memberViewTTL)
}

func refreshSyncgroupMembers(gid interfaces.GroupId, sg *interfaces.Syncgroup, dbId wire.Id, newMembers map[string]*memberInfo) {
	for member, info := range sg.Joiners {
		if _, ok := newMembers[member]; !ok {
			newMembers[member] = &memberInfo{
				db2sg:    make(map[wire.Id]sgMember),
				mtTables: make(map[string]struct{}),
			}
		}
		if _, ok := newMembers[member].db2sg[dbId]; !ok {
			newMembers[member].db2sg[dbId] = make(sgMember)
		}
		newMembers[member].db2sg[dbId][gid] = info

		// Collect mount tables.
		for _, mt := range sg.Spec.MountTables {
			newMembers[member].mtTables[mt] = struct{}{}
		}
	}
}

// forEachSyncgroup iterates over all Syncgroups in the Database and invokes
// the callback function on each one.  The callback returns a "done" flag to
// make forEachSyncgroup() stop the iteration earlier; otherwise the function
// loops across all Syncgroups in the Database.
func forEachSyncgroup(st store.StoreReader, callback func(interfaces.GroupId, *interfaces.Syncgroup) bool) {
	stream := st.Scan(common.ScanPrefixArgs(sgIdPrefix, ""))
	defer stream.Cancel()

	for stream.Advance() {
		key := string(stream.Key(nil))
		gid, err := sgID(key)
		if err != nil {
			vlog.Errorf("sync: forEachSyncgroup: invalid syncgroup ID %s: %v", key, err)
			continue
		}

		sg, err := getSyncgroupByGid(nil, st, gid)
		if err != nil {
			vlog.Errorf("sync: forEachSyncgroup: cannot get syncgroup %d: %v", gid, err)
			continue
		}

		if callback(gid, sg) {
			break // done, early exit
		}
	}

	if err := stream.Err(); err != nil {
		vlog.Errorf("sync: forEachSyncgroup: scan stream error: %v", err)
	}
}

// getMembers returns all syncgroup members and the count of syncgroups each one
// joined.
func (s *syncService) getMembers(ctx *context.T) map[string]uint32 {
	s.allMembersLock.Lock()
	defer s.allMembersLock.Unlock()

	s.refreshMembersIfExpired(ctx)

	members := make(map[string]uint32)
	for member, info := range s.allMembers.members {
		count := 0
		for _, sgmi := range info.db2sg {
			count += len(sgmi)
		}
		members[member] = uint32(count)
	}

	return members
}

// copyMemberInfo returns a copy of the info for the requested peer.
func (s *syncService) copyMemberInfo(ctx *context.T, member string) *memberInfo {
	s.allMembersLock.RLock()
	defer s.allMembersLock.RUnlock()

	info, ok := s.allMembers.members[member]
	if !ok {
		return nil
	}

	// Make a copy.
	infoCopy := &memberInfo{
		db2sg:    make(map[wire.Id]sgMember),
		mtTables: make(map[string]struct{}),
	}
	for dbId, sgInfo := range info.db2sg {
		infoCopy.db2sg[dbId] = make(sgMember)
		for gid, mi := range sgInfo {
			infoCopy.db2sg[dbId][gid] = mi
		}
	}
	for mt := range info.mtTables {
		infoCopy.mtTables[mt] = struct{}{}
	}

	return infoCopy
}

// sgPriorityLowerThan returns whether *a is a lower priority than *b.
func sgPriorityLowerThan(a *interfaces.SgPriority, b *interfaces.SgPriority) bool {
	if a.DevType == wire.BlobDevTypeServer {
		// Nothing has higher priority than a server.
		return false
	} else if a.DevType != b.DevType {
		// Different device types have priority defined by the type.
		return b.DevType < a.DevType
	} else if b.ServerTime.After(a.ServerTime.Add(blobRecencyTimeSlop)) {
		// Devices with substantially fresher data from a server have higher priority.
		return true
	} else {
		// If the devices have equally fresh data, prefer the shorter sync distance.
		return b.ServerTime.After(a.ServerTime.Add(-blobRecencyTimeSlop)) && (b.Distance < a.Distance)
	}
}

// updateSyncgroupPriority updates the local syncgroup priority for blob
// ownership in *local, using *remote, the corresponding priority from a remote
// peer with which this device has recently communicated.  Returns whether
// *local was modified.
func updateSyncgroupPriority(ctx *context.T, local *interfaces.SgPriority, remote *interfaces.SgPriority) (modified bool) {
	if sgPriorityLowerThan(local, remote) { // Never returns true if "local" is a server.
		// The local device is not a "server" for the syncgroup, and
		// has less-recent communication than the remote device.

		// Derive priority from this remote peer.
		local.Distance = (local.Distance*(blobSyncDistanceDecay-1) + (remote.Distance + 1)) / blobSyncDistanceDecay
		if remote.DevType == wire.BlobDevTypeServer {
			local.ServerTime = time.Now()
		} else {
			local.ServerTime = remote.ServerTime
		}
		modified = true
	}
	return modified
}

// updateAllSyncgroupPriorities updates local syncgroup blob-ownership
// priorities, based on priority data from a peer.
func updateAllSyncgroupPriorities(ctx *context.T, bst blob.BlobStore, remoteSgPriorities interfaces.SgPriorities) (anyErr error) {
	for sgId, remoteSgPriority := range remoteSgPriorities {
		var perSyncgroup blob.PerSyncgroup
		err := bst.GetPerSyncgroup(ctx, sgId, &perSyncgroup)
		if err == nil && updateSyncgroupPriority(ctx, &perSyncgroup.Priority, &remoteSgPriority) {
			err = bst.SetPerSyncgroup(ctx, sgId, &perSyncgroup)
		}
		// Keep going even if there was an error, but return the first
		// error encountered.  Higher levels might wish to log the
		// error, but might not wish to consider the syncing operation
		// as failing merely because the priorities could not be
		// updated.
		if err != nil && anyErr == nil {
			anyErr = err
		}
	}
	return anyErr
}

// addSyncgroupPriorities inserts into map sgPriMap the syncgroups in sgIds,
// together with their local blob-ownership priorities.
func addSyncgroupPriorities(ctx *context.T, bst blob.BlobStore, sgIds sgSet, sgPriMap interfaces.SgPriorities) error {
	var firstErr error
	for sgId := range sgIds {
		var perSyncgroup blob.PerSyncgroup
		err := bst.GetPerSyncgroup(ctx, sgId, &perSyncgroup)
		if err == nil {
			sgPriMap[sgId] = perSyncgroup.Priority
		} else if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Low-level utility functions to access DB entries without tracking their
// relationships.  Use the functions above to manipulate syncgroups.
// Note:  As with other syncable objects, the DAG "heads" table contains a
// reference to the current syncgroup version, and the DAG "nodes" table tracks
// its history of mutations.

// SgIdToGid converts a databaseId and syncgroup Id to an internal ID by using a SHA256 hash.
func SgIdToGid(dbId, id wire.Id) interfaces.GroupId {
	hasher := sha256.New()
	hasher.Write([]byte(naming.Join(pubutil.EncodeId(dbId), pubutil.EncodeId(id))))
	gid := base64.RawStdEncoding.EncodeToString(hasher.Sum(nil))
	return interfaces.GroupId(gid)
}

// sgIdKey returns the key used to access the syncgroup ID entry.
func sgIdKey(gid interfaces.GroupId) string {
	return common.JoinKeyParts(sgIdPrefix, string(gid))
}

// sgOID converts a group id into an oid string.
func sgOID(gid interfaces.GroupId) string {
	return common.JoinKeyParts(sgDataPrefix, string(gid))
}

// sgID is the inverse of sgOID and sgIdKey.  It extracts the syncgroup ID from
// either the syncgroup data key or the syncgroup ID entry key.
// TODO(hpucha): Add unittests for sgOID/sgID and other such helpers. In CLs
// v.io/c/16919 and v.io/c/17043, bugs in sgID were only caught by integration
// tests.
func sgID(oid string) (interfaces.GroupId, error) {
	parts := common.SplitNKeyParts(oid, 3)
	if len(parts) != 3 {
		return interfaces.NoGroupId, fmt.Errorf("invalid sgoid %s", oid)
	}
	return interfaces.GroupId(parts[2]), nil
}

// sgDataKey returns the key used to access a version of the syncgroup data.
func sgDataKey(gid interfaces.GroupId, version string) string {
	return sgDataKeyByOID(sgOID(gid), version)
}

// sgDataKeyByOID returns the key used to access a version of the syncgroup
// data.
func sgDataKeyByOID(oid, version string) string {
	return common.JoinKeyParts(oid, version)
}

// hasSGIdEntry returns true if the syncgroup ID entry exists.
func hasSGIdEntry(sntx store.SnapshotOrTransaction, gid interfaces.GroupId) (bool, error) {
	return store.Exists(nil, sntx, sgIdKey(gid))
}

// hasSGDataEntry returns true if the syncgroup versioned data entry exists.
func hasSGDataEntry(sntx store.SnapshotOrTransaction, gid interfaces.GroupId, version string) (bool, error) {
	return store.Exists(nil, sntx, sgDataKey(gid, version))
}

// setSGIdEntry stores the syncgroup ID entry.
func setSGIdEntry(ctx *context.T, tx store.Transaction, gid interfaces.GroupId, state *SgLocalState) error {
	return store.Put(ctx, tx, sgIdKey(gid), state)
}

// setSGDataEntryByOID stores the syncgroup versioned data entry.
func setSGDataEntryByOID(ctx *context.T, tx store.Transaction, sgoid, version string, sg *interfaces.Syncgroup) error {
	return store.Put(ctx, tx, sgDataKeyByOID(sgoid, version), sg)
}

// getSGIdEntry retrieves the syncgroup local state for a given group ID.
func getSGIdEntry(ctx *context.T, st store.StoreReader, gid interfaces.GroupId) (*SgLocalState, error) {
	var state SgLocalState
	if err := store.Get(ctx, st, sgIdKey(gid), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// getSGDataEntry retrieves the syncgroup data for a given group ID and version.
func getSGDataEntry(ctx *context.T, st store.StoreReader, gid interfaces.GroupId, version string) (*interfaces.Syncgroup, error) {
	return getSGDataEntryByOID(ctx, st, sgOID(gid), version)
}

// getSGDataEntryByOID retrieves the syncgroup data for a given group OID and
// version.
func getSGDataEntryByOID(ctx *context.T, st store.StoreReader, sgoid string, version string) (*interfaces.Syncgroup, error) {
	var sg interfaces.Syncgroup
	if err := store.Get(ctx, st, sgDataKeyByOID(sgoid, version), &sg); err != nil {
		return nil, err
	}
	return &sg, nil
}

// delSGIdEntry deletes the syncgroup ID entry.
func delSGIdEntry(ctx *context.T, tx store.Transaction, gid interfaces.GroupId) error {
	return store.Delete(ctx, tx, sgIdKey(gid))
}

// delSGDataEntry deletes the syncgroup versioned data entry.
func delSGDataEntry(ctx *context.T, tx store.Transaction, gid interfaces.GroupId, version string) error {
	return store.Delete(ctx, tx, sgDataKey(gid, version))
}

////////////////////////////////////////////////////////////
// Syncgroup methods between Client and Syncbase.

// TODO(hpucha): Pass blessings along.
func (sd *syncDatabase) CreateSyncgroup(ctx *context.T, call rpc.ServerCall, sgId wire.Id, spec wire.SyncgroupSpec, myInfo wire.SyncgroupMemberInfo) error {
	vlog.VI(2).Infof("sync: CreateSyncgroup: begin: %s, spec %+v", sgId, spec)
	defer vlog.VI(2).Infof("sync: CreateSyncgroup: end: %s", sgId)

	if err := pubutil.ValidateId(sgId); err != nil {
		return verror.New(wire.ErrInvalidName, ctx, pubutil.EncodeId(sgId), err)
	}

	ss := sd.sync.(*syncService)
	dbId := sd.db.Id()

	now, err := ss.vclock.Now()
	if err != nil {
		return err
	}
	sm := interfaces.SyncgroupMemberState{WhenUpdated: now.Unix(), HasLeft: false, MemberInfo: myInfo}
	// Instantiate sg. Add self as joiner.
	// TODO(ivanpi): Spec is sanity checked later in addSyncgroup. Do it here
	// instead to fail early?
	gid, version := SgIdToGid(dbId, sgId), ss.newSyncgroupVersion()
	sg := &interfaces.Syncgroup{
		Id:          sgId,
		SpecVersion: version,
		Spec:        spec,
		Creator:     ss.name,
		DbId:        dbId,
		Status:      interfaces.SyncgroupStatusPublishPending,
		Joiners:     map[string]interfaces.SyncgroupMemberState{ss.name: sm},
	}

	err = watchable.RunInTransaction(sd.db.St(), func(tx *watchable.Transaction) error {
		// Check permissions on Database.
		if err := sd.db.CheckPermsInternal(ctx, call, tx); err != nil {
			return err
		}

		// Check implicit perms derived from blessing pattern in id.
		// TODO(ivanpi): The signing blessing should be (re?)checked against the
		// implicit perms when signing the syncgroup metadata.
		if _, err := common.CheckImplicitPerms(ctx, call, sgId, wire.AllSyncgroupTags); err != nil {
			return err
		}

		// Check that all the collections on which the syncgroup is
		// being created exist.
		for _, c := range spec.Collections {
			collKey := common.JoinKeyParts(common.CollectionPermsPrefix, pubutil.EncodeId(c), "")
			if err := store.Get(ctx, tx, collKey, &interfaces.CollectionPerms{}); err != nil {
				// TODO(hpucha): Is this error ok to return to
				// the end user in terms of error visibility.
				return verror.New(verror.ErrNoExist, ctx, "collection missing", c)
			}
		}

		// TODO(hpucha): Check ACLs on all SG collections.
		// This may need another method on util.Database interface.

		// Reserve a log generation and position counts for the new syncgroup.
		gen, pos := ss.reserveGenAndPosInDbLog(ctx, dbId, sgOID(gid), 1)

		if err := ss.addSyncgroup(ctx, tx, version, true, "", nil, ss.id, gen, pos, sg); err != nil {
			return err
		}

		// Take a snapshot of the data to bootstrap the syncgroup.
		return sd.bootstrapSyncgroup(ctx, tx, gid, spec.Collections)
	})

	if err != nil {
		return err
	}

	ss.initSyncStateInMem(ctx, dbId, sgOID(gid))

	if err := ss.checkptSgLocalGen(ctx, dbId, gid); err != nil {
		return err
	}

	// Advertise the Syncbase at the chosen mount table and in the
	// neighborhood.
	if err := ss.advertiseSyncbase(ctx, call, sg); err != nil {
		// The failure in this step is rare. However, if there is a
		// failure, create must be failed as well.
		//
		// TODO(hpucha): Implement failure handling here and in
		// advertiseSyncbase. Currently, with the transaction above,
		// failure here means rolling back the create. However, roll
		// back is not straight forward since by the time we are ready
		// to roll back, the persistent sg state could be used for
		// another join or a leave request from the app. To handle this
		// contention, we might have to serialize all syncgroup related
		// operations pertaining to a database with a single lock, and
		// further serialize all syncbase publishing with another
		// lock across all databases.
		return err
	}

	// Local SG create succeeded. Publish the SG at the chosen server, or if
	// that fails, enqueue it for later publish retries.
	if spec.PublishSyncbaseName != "" {
		go func() {
			if err := sd.publishSyncgroup(ctx, call, sgId, spec.PublishSyncbaseName); err != nil {
				ss.enqueuePublishSyncgroup(sgId, dbId, true)
			}
		}()
	}

	return nil
}

func (sd *syncDatabase) JoinSyncgroup(ctx *context.T, call rpc.ServerCall, remoteSyncbaseName string, expectedSyncbaseBlessings []string, sgId wire.Id, myInfo wire.SyncgroupMemberInfo) (wire.SyncgroupSpec, error) {
	vlog.VI(2).Infof("sync: JoinSyncgroup: begin: %v at %s, call is %v", sgId, remoteSyncbaseName, call)
	defer vlog.VI(2).Infof("sync: JoinSyncgroup: end: %v at %s", sgId, remoteSyncbaseName)

	var sgErr error
	var sg *interfaces.Syncgroup
	nullSpec := wire.SyncgroupSpec{}
	gid := SgIdToGid(sd.db.Id(), sgId)
	err := watchable.RunInTransaction(sd.db.St(), func(tx *watchable.Transaction) error {
		// Check permissions on Database.
		if err := sd.db.CheckPermsInternal(ctx, call, tx); err != nil {
			return err
		}

		// Check if syncgroup already exists and get its info.
		sg, sgErr = getSyncgroupByGid(ctx, tx, gid)
		if sgErr != nil {
			return sgErr
		}

		// Check SG ACL. Caller must have Read access on the syncgroup
		// acl to join a syncgroup.
		if err := authorize(ctx, call.Security(), sg); err != nil {
			return err
		}

		// Syncgroup already exists, increment the number of local
		// joiners in its local state information.  This presents
		// different scenarios:
		// 1- An additional local joiner: the current number of local
		//    joiners is > 0 and the syncgroup was already bootstrapped
		//    to the Watcher, so there is nothing else to do.
		// 2- A new local joiner after all previous local joiners had
		//    left: the number of local joiners is 0, the Watcher must
		//    be re-notified via a syncgroup bootstrap because the last
		//    previous joiner to leave had un-notified the Watcher.  In
		//    this scenario the syncgroup was not destroyed after the
		//    last joiner left because the syncgroup was also published
		//    here by a remote peer and thus cannot be destroyed only
		//    based on the local joiners.
		// 3- A first local joiner for a syncgroup that was published
		//    here from a remote Syncbase: the number of local joiners
		//    is also 0 (and the remote publish flag is set), and the
		//    Watcher must be notified via a syncgroup bootstrap.
		// Conclusion: bootstrap if the number of local joiners is 0.
		sgState, err := getSGIdEntry(ctx, tx, gid)
		if err != nil {
			return err
		}

		if sgState.NumLocalJoiners == 0 {
			if err := sd.bootstrapSyncgroup(ctx, tx, gid, sg.Spec.Collections); err != nil {
				return err
			}
		}
		sgState.NumLocalJoiners++
		return setSGIdEntry(ctx, tx, gid, sgState)
	})

	// The presented blessing is allowed to make this Syncbase instance join
	// the specified syncgroup, but this Syncbase instance has in fact
	// already joined the syncgroup. Join is idempotent, so we simply return
	// the spec to indicate success.
	if err == nil {
		return sg.Spec, nil
	}

	// Join is not allowed (possibilities include Database permissions check
	// failed, SG ACL check failed or error during fetching SG information).
	if verror.ErrorID(sgErr) != verror.ErrNoExist.ID {
		return nullSpec, err
	}

	// Brand new join.

	// Get this Syncbase's sync module handle.
	ss := sd.sync.(*syncService)

	// Contact a syncgroup Admin to join the syncgroup.
	sg2, version, genvec, err := sd.joinSyncgroupAtAdmin(ctx, call, sd.db.Id(), sgId, remoteSyncbaseName, expectedSyncbaseBlessings, ss.name, myInfo)
	if err != nil {
		return nullSpec, err
	}

	// Verify that the database id is valid for this syncgroup.
	if sg2.DbId != sd.db.Id() {
		return nullSpec, verror.New(verror.ErrBadArg, ctx, "bad db with syncgroup")
	}

	err = watchable.RunInTransaction(sd.db.St(), func(tx *watchable.Transaction) error {
		if err := ss.addSyncgroup(ctx, tx, version, false, "", genvec, 0, 0, 0, &sg2); err != nil {
			return err
		}

		// Take a snapshot of the data to bootstrap the syncgroup.
		return sd.bootstrapSyncgroup(ctx, tx, gid, sg2.Spec.Collections)
	})

	if err != nil {
		return nullSpec, err
	}

	ss.initSyncStateInMem(ctx, sg2.DbId, sgOID(gid))

	// Advertise the Syncbase at the chosen mount table and in the
	// neighborhood.
	if err := ss.advertiseSyncbase(ctx, call, &sg2); err != nil {
		// TODO(hpucha): Implement failure handling. See note in
		// CreateSyncgroup for more details.
		return nullSpec, err
	}

	return sg2.Spec, nil
}

type syncgroups []wire.Id

func (s syncgroups) Len() int {
	return len(s)
}
func (s syncgroups) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s syncgroups) Less(i, j int) bool {
	if s[i].Name < s[j].Name {
		return true
	}
	if s[i].Name > s[j].Name {
		return false
	}
	return s[i].Blessing < s[j].Blessing
}

func (sd *syncDatabase) ListSyncgroups(ctx *context.T, call rpc.ServerCall) ([]wire.Id, error) {
	vlog.VI(2).Infof("sync: ListSyncgroups: begin")
	defer vlog.VI(2).Infof("sync: ListSyncgroups: end")

	sn := sd.db.St().NewSnapshot()
	defer sn.Abort()

	// Check permissions on Database.
	if err := sd.db.CheckPermsInternal(ctx, call, sn); err != nil {
		return nil, err
	}

	// Scan all the syncgroups found in the Database.
	var sgIds []wire.Id
	forEachSyncgroup(sn, func(gid interfaces.GroupId, sg *interfaces.Syncgroup) bool {
		sgIds = append(sgIds, sg.Id)
		return false
	})

	sort.Sort(syncgroups(sgIds))

	vlog.VI(2).Infof("sync: ListSyncgroups: %v", sgIds)
	return sgIds, nil
}

func (sd *syncDatabase) GetSyncgroupSpec(ctx *context.T, call rpc.ServerCall, sgId wire.Id) (wire.SyncgroupSpec, string, error) {
	vlog.VI(2).Infof("sync: GetSyncgroupSpec: begin %v", sgId)
	defer vlog.VI(2).Infof("sync: GetSyncgroupSpec: end: %v", sgId)

	sn := sd.db.St().NewSnapshot()
	defer sn.Abort()

	var spec wire.SyncgroupSpec

	// Check permissions on Database.
	if err := sd.db.CheckPermsInternal(ctx, call, sn); err != nil {
		return spec, "", err
	}

	// Get the syncgroup information.
	sg, err := getSyncgroupByGid(ctx, sn, SgIdToGid(sd.db.Id(), sgId))
	if err != nil {
		return spec, "", err
	}
	// TODO(hpucha): Check syncgroup ACL.

	vlog.VI(2).Infof("sync: GetSyncgroupSpec: %v spec %v", sgId, sg.Spec)
	return sg.Spec, sg.SpecVersion, nil
}

func (sd *syncDatabase) GetSyncgroupMembers(ctx *context.T, call rpc.ServerCall, sgId wire.Id) (map[string]wire.SyncgroupMemberInfo, error) {
	vlog.VI(2).Infof("sync: GetSyncgroupMembers: begin %v", sgId)
	defer vlog.VI(2).Infof("sync: GetSyncgroupMembers: end: %v", sgId)

	sn := sd.db.St().NewSnapshot()
	defer sn.Abort()

	// Check permissions on Database.
	if err := sd.db.CheckPermsInternal(ctx, call, sn); err != nil {
		return nil, err
	}

	// Get the syncgroup information.
	sg, err := getSyncgroupByGid(ctx, sn, SgIdToGid(sd.db.Id(), sgId))
	if err != nil {
		return nil, err
	}

	// TODO(hpucha): Check syncgroup ACL.

	vlog.VI(2).Infof("sync: GetSyncgroupMembers: %v members %v, len %v", sgId, sg.Joiners, len(sg.Joiners))
	joiners := make(map[string]wire.SyncgroupMemberInfo)
	for key, value := range sg.Joiners {
		joiners[key] = value.MemberInfo
	}
	return joiners, nil
}

func (sd *syncDatabase) SetSyncgroupSpec(ctx *context.T, call rpc.ServerCall, sgId wire.Id, spec wire.SyncgroupSpec, version string) error {
	vlog.VI(2).Infof("sync: SetSyncgroupSpec: begin %v %v %s", sgId, spec, version)
	defer vlog.VI(2).Infof("sync: SetSyncgroupSpec: end: %v", sgId)

	if err := verifySyncgroupSpec(ctx, &spec); err != nil {
		return err
	}

	ss := sd.sync.(*syncService)
	dbId := sd.db.Id()
	gid := SgIdToGid(dbId, sgId)
	var sg *interfaces.Syncgroup

	err := watchable.RunInTransaction(sd.db.St(), func(tx *watchable.Transaction) error {
		// Check permissions on Database.
		if err := sd.db.CheckPermsInternal(ctx, call, tx); err != nil {
			return err
		}

		var err error
		sg, err = getSyncgroupByGid(ctx, tx, gid)
		if err != nil {
			return err
		}

		if version != NoVersion && sg.SpecVersion != version {
			return verror.NewErrBadVersion(ctx)
		}

		// Client must not modify the set of collections for this syncgroup.
		if !sameCollections(spec.Collections, sg.Spec.Collections) {
			return verror.New(verror.ErrBadArg, ctx, "cannot modify collections")
		}

		sgState, err := getSGIdEntry(ctx, tx, gid)
		if err != nil {
			return err
		}
		if sgState.SyncPending {
			return verror.NewErrBadState(ctx)
		}

		// Check if this peer is allowed to change the spec.
		blessingNames, _ := security.RemoteBlessingNames(ctx, call.Security())
		vlog.VI(4).Infof("sync: SetSyncgroupSpec: authorizing blessings %v against permissions %v", blessingNames, sg.Spec.Perms)
		if !isAuthorizedForTag(sg.Spec.Perms, access.Admin, blessingNames) {
			return verror.New(verror.ErrNoAccess, ctx)
		}

		// Reserve a log generation and position counts for the new syncgroup.
		gen, pos := ss.reserveGenAndPosInDbLog(ctx, dbId, sgOID(gid), 1)

		newVersion := ss.newSyncgroupVersion()
		sg.Spec = spec
		sg.SpecVersion = newVersion
		return ss.updateSyncgroupVersioning(ctx, tx, gid, newVersion, true, ss.id, gen, pos, sg)
	})

	if err != nil {
		return err
	}
	if err = ss.advertiseSyncgroupInNeighborhood(sg); err != nil {
		return err
	}
	return ss.checkptSgLocalGen(ctx, dbId, gid)
}

//////////////////////////////
// Helper functions

// checkptSgLocalGen cuts a local generation for the specified syncgroup to
// capture its updates.
func (s *syncService) checkptSgLocalGen(ctx *context.T, dbId wire.Id, sgid interfaces.GroupId) error {
	// Cut a new generation to capture the syncgroup updates. Unlike the
	// interaction between the watcher and the pause/resume bits, there is
	// no guarantee of ordering between the syncgroup updates that are
	// synced and when sync is paused/resumed. This is because in the
	// current design syncgroup updates are not serialized through the watch
	// queue.
	sgs := sgSet{sgid: struct{}{}}
	return s.checkptLocalGen(ctx, dbId, sgs)
}

// publishSyncgroup publishes the syncgroup at the remote peer and update its
// status.  If the publish operation is either successful or rejected by the
// peer, the status is updated to "running" or "rejected" respectively and the
// function returns "nil" to indicate to the caller there is no need to make
// further attempts.  Otherwise an error (typically RPC error, but could also
// be a store error) is returned to the caller.
// TODO(rdaoud): make all SG admins try to publish after they join.
func (sd *syncDatabase) publishSyncgroup(ctx *context.T, call rpc.ServerCall, sgId wire.Id, publishSyncbaseName string) error {
	st := sd.db.St()
	ss := sd.sync.(*syncService)
	dbId := sd.db.Id()

	// If this admin is offline, it shouldn't attempt to publish the syncgroup
	// since it would be unable to send out the new syncgroup updates. However, it
	// is still possible that the admin goes offline right after processing the
	// request.
	if !ss.isDbSyncable(ctx, dbId) {
		return interfaces.NewErrDbOffline(ctx, dbId)
	}

	gid := SgIdToGid(dbId, sgId)
	version, err := getSyncgroupVersion(ctx, st, gid)
	if err != nil {
		return err
	}
	sg, err := getSGDataEntry(ctx, st, gid, version)
	if err != nil {
		return err
	}

	if sg.Status != interfaces.SyncgroupStatusPublishPending {
		return nil
	}

	// Note: the remote peer is given the syncgroup version and genvec at
	// the point before the post-publish update, at which time the status
	// and joiner list of the syncgroup get updated.  This is functionally
	// correct, just not symmetrical with what happens at joiner, which
	// receives the syncgroup state post-join.
	status := interfaces.SyncgroupStatusPublishRejected

	sgs := sgSet{gid: struct{}{}}
	gv, _, err := ss.copyDbGenInfo(ctx, dbId, sgs)
	if err != nil {
		return err
	}
	// TODO(hpucha): Do we want to pick the head version corresponding to
	// the local gen of the sg? It appears that it shouldn't matter.

	publishAddress := naming.Join(publishSyncbaseName, common.SyncbaseSuffix)
	c := interfaces.SyncClient(publishAddress)
	peer, err := c.PublishSyncgroup(ctx, ss.name, *sg, version, gv[sgOID(gid)])

	if err == nil {
		status = interfaces.SyncgroupStatusRunning
	} else {
		errId := verror.ErrorID(err)
		if errId == interfaces.ErrDupSyncgroupPublish.ID {
			// Duplicate publish: another admin already published
			// the syncgroup, nothing else needs to happen because
			// that other admin would have updated the syncgroup
			// status and p2p SG sync will propagate the change.
			// TODO(rdaoud): what if that other admin crashes and
			// never updates the syncgroup status (dies permanently
			// or is ejected before the status update)?  Eventually
			// some admin must decide to update the SG status anyway
			// even if that causes extra SG mutations and conflicts.
			vlog.VI(3).Infof("sync: publishSyncgroup: %v: duplicate publish", sgId)
			return nil
		}

		if errId != verror.ErrExist.ID {
			// The publish operation failed with an error other
			// than ErrExist then it must be retried later on.
			// TODO(hpucha): Is there an RPC error that we can check here?
			vlog.VI(3).Infof("sync: publishSyncgroup: %v: failed, retry later: %v", sgId, err)
			return err
		}
	}

	// The publish operation is done because either it succeeded or it
	// failed with the ErrExist error.  Update the syncgroup status and, if
	// the publish was successful, add the remote peer to the syncgroup.
	vlog.VI(3).Infof("sync: publishSyncgroup: %v: peer %s: done: status %s: %v",
		sgId, peer, status.String(), err)

	err = watchable.RunInTransaction(st, func(tx *watchable.Transaction) error {
		// Ensure SG still exists.
		sg, err := getSyncgroupByGid(ctx, tx, gid)
		if err != nil {
			return err
		}

		// Reserve a log generation and position counts for the new
		// syncgroup version.
		gen, pos := ss.reserveGenAndPosInDbLog(ctx, dbId, sgOID(gid), 1)

		sg.Status = status
		if status == interfaces.SyncgroupStatusRunning {
			// TODO(hpucha): Default priority?
			// TODO(fredq): shouldn't the PublishSyncgroup() call return the MemberInfo?
			memberInfo := wire.SyncgroupMemberInfo{}
			now, err := ss.vclock.Now()
			if err != nil {
				return err
			}
			sg.Joiners[peer] = interfaces.SyncgroupMemberState{WhenUpdated: now.Unix(), HasLeft: false, MemberInfo: memberInfo}
		}

		return ss.updateSyncgroupVersioning(ctx, tx, gid, NoVersion, true, ss.id, gen, pos, sg)
	})
	if err != nil {
		vlog.Errorf("sync: publishSyncgroup: cannot update syncgroup %v status to %s: %v",
			sgId, status.String(), err)
		return err
	}

	return ss.checkptSgLocalGen(ctx, dbId, gid)
}

// bootstrapSyncgroup inserts into the transaction log a syncgroup operation and
// a set of Snapshot operations to notify the sync watcher about the syncgroup
// collections to start accepting and the initial state of existing store keys that
// match these collections (both data and permission keys).
// TODO(rdaoud): this operation scans the managed keys of the database and can
// be time consuming.  Consider doing it asynchronously and letting the server
// reply to the client earlier.  However it must happen within the scope of this
// transaction (and its snapshot view).
func (sd *syncDatabase) bootstrapSyncgroup(ctx *context.T, tx *watchable.Transaction, sgId interfaces.GroupId, collections []wire.Id) error {
	if len(collections) == 0 {
		return verror.New(verror.ErrInternal, ctx, "no collections specified")
	}

	// Get the store options to retrieve the list of managed key prefixes.
	opts, err := watchable.GetOptions(sd.db.St())
	if err != nil {
		return err
	}
	if len(opts.ManagedPrefixes) == 0 {
		return verror.New(verror.ErrInternal, ctx, "store has no managed prefixes")
	}

	// While a syncgroup is defined over a list of collections, the sync
	// protocol treats them as key prefixes (with an "" row component).
	pfxStrs := make([]string, len(collections))
	for i, c := range collections {
		pfxStrs[i] = toCollectionPrefixStr(c)
	}

	// Notify the watcher of the syncgroup prefixes to start accepting.
	if err := sbwatchable.AddSyncgroupOp(ctx, tx, sgId, pfxStrs, false); err != nil {
		return err
	}

	// Loop over the store managed key prefixes (e.g. data and permissions).
	// For each one, scan the ranges of the given syncgroup prefixes.  For
	// each matching key, insert a snapshot operation in the log.  Scanning
	// is done over the version entries to retrieve the matching keys and
	// their version numbers (the key values).  Remove the version prefix
	// from the key used in the snapshot operation.
	for _, mp := range opts.ManagedPrefixes {
		for _, p := range pfxStrs {
			start, limit := common.ScanPrefixArgs(common.JoinKeyParts(common.VersionPrefix, mp), p)
			stream := tx.Scan(start, limit)
			for stream.Advance() {
				k, v := stream.Key(nil), stream.Value(nil)
				// Remove version prefix.
				key := []byte(common.StripFirstKeyPartOrDie(string(k)))
				if err := sbwatchable.AddSyncSnapshotOp(ctx, tx, key, v); err != nil {
					return err
				}

			}
			if err := stream.Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

// advertiseSyncbase advertises this Syncbase at the chosen mount tables and
// over the neighborhood.
func (s *syncService) advertiseSyncbase(ctx *context.T, call rpc.ServerCall, sg *interfaces.Syncgroup) error {
	s.nameLock.Lock()
	defer s.nameLock.Unlock()

	for _, mt := range sg.Spec.MountTables {
		name := naming.Join(mt, s.name)
		// AddName is idempotent. Note that AddName will retry the
		// publishing if not successful. So if a node is offline, it
		// will publish the name when possible.
		if err := call.Server().AddName(name); err != nil {
			return err
		}
	}

	// TODO(ashankar): Should the advertising libraries bail when they can't
	// advertise or just wait for changes to the network and retry.
	if err := s.advertiseSyncbaseInNeighborhood(); err != nil {
		// We ignore errors on neighborhood advertising since sync can
		// continue when members are online despite these errors.
		vlog.Errorf("sync: advertiseSyncbaseInNeighborhood: failed with err %v", err)
	}
	// TODO(hpucha): In case of a joiner, this can be optimized such that we
	// don't advertise until the syncgroup is out of the pending state.
	if err := s.advertiseSyncgroupInNeighborhood(sg); err != nil {
		// We ignore errors on neighborhood advertising since sync when
		// members are online can continue despite these errors.
		vlog.Errorf("sync: advertiseSyncgroupInNeighborhood: failed with err %v", err)
	}
	return nil
}

func (sd *syncDatabase) joinSyncgroupAtAdmin(ctxIn *context.T, call rpc.ServerCall, dbId, sgId wire.Id, remoteSyncbaseName string, expectedSyncbaseBlessings []string, localSyncbaseName string, myInfo wire.SyncgroupMemberInfo) (interfaces.Syncgroup, string, interfaces.GenVector, error) {
	vlog.VI(2).Infof("sync: joinSyncgroupAtAdmin: begin, dbId %v, sgId %v, remoteSyncbaseName %v", dbId, sgId, remoteSyncbaseName)

	if remoteSyncbaseName != "" {
		ctx, cancel := context.WithTimeout(ctxIn, cloudConnectionTimeout)
		c := interfaces.SyncClient(naming.Join(remoteSyncbaseName, common.SyncbaseSuffix))
		sg, vers, gv, err := c.JoinSyncgroupAtAdmin(ctx, dbId, sgId, localSyncbaseName, myInfo)
		cancel()
		if err == nil {
			vlog.VI(2).Infof("sync: joinSyncgroupAtAdmin: end succeeded at %v, returned sg %v vers %v gv %v", sgId, sg, vers, gv)
			return sg, vers, gv, err
		}
	}

	vlog.VI(2).Infof("sync: joinSyncgroupAtAdmin: try neighborhood %v", sgId)

	// TODO(hpucha): Restrict the set of errors when retry happens to
	// network related errors or other retriable errors.

	// Get this Syncbase's sync module handle.
	ss := sd.sync.(*syncService)

	// Try to join using an Admin on neighborhood in case this node does not
	// have connectivity.
	neighbors := ss.filterSyncgroupAdmins(dbId, sgId)
	c := interfaces.SyncClient("")
	for _, svc := range neighbors {
		me := &naming.MountEntry{IsLeaf: true, Name: common.SyncbaseSuffix}
		// TODO(fredq): check that the service at addr has the expectedSyncbaseBlessings.
		for _, addr := range svc.Addresses {
			me.Servers = append(me.Servers, naming.MountedServer{Server: addr})
		}
		ctx, cancel := context.WithTimeout(ctxIn, NeighborConnectionTimeout)
		// We construct a preresolved opt with all the addresses for the peer. This allows us
		// to try the endpoints for this peer with all of their resolved addresses, rather
		// than a single resolved address at a time. This is important because it is possible for
		// addresses in the list to be unreachable, causing large delays when trying join calls
		// in serial.
		sg, vers, gv, err := c.JoinSyncgroupAtAdmin(ctx, dbId, sgId, localSyncbaseName, myInfo, options.Preresolved{me})
		cancel()
		if err == nil {
			vlog.VI(2).Infof("sync: joinSyncgroupAtAdmin: end succeeded at addresses %v, returned sg %v vers %v gv %v", me.Servers, sg, vers, gv)
			return sg, vers, gv, err
		}
	}

	vlog.VI(2).Infof("sync: joinSyncgroupAtAdmin: failed %v", sgId)
	return interfaces.Syncgroup{}, "", interfaces.GenVector{}, verror.New(wire.ErrSyncgroupJoinFailed, ctxIn)
}

func authorize(ctx *context.T, call security.Call, sg *interfaces.Syncgroup) error {
	auth := access.TypicalTagTypePermissionsAuthorizer(sg.Spec.Perms)
	if err := auth.Authorize(ctx, call); err != nil {
		return verror.New(verror.ErrNoAccess, ctx, err)
	}
	return nil
}

// isAuthorizedForTag returns whether at least one of the blessingNames is
// authorized via the specified tag in perms.
func isAuthorizedForTag(perms access.Permissions, tag access.Tag, blessingNames []string) bool {
	acl, exists := perms[string(tag)]
	return exists && acl.Includes(blessingNames...)
}

// Check the acl against all known blessings.
//
// TODO(hpucha): Should this be restricted to default or should we use
// ForPeer?
func syncgroupAdmin(ctx *context.T, perms access.Permissions) bool {
	var blessingNames []string
	p := v23.GetPrincipal(ctx)
	for _, blessings := range p.BlessingStore().PeerBlessings() {
		blessingNames = append(blessingNames, security.BlessingNames(p, blessings)...)
	}

	return isAuthorizedForTag(perms, access.Admin, blessingNames)
}

////////////////////////////////////////////////////////////
// Methods for syncgroup create/join between Syncbases.

func (s *syncService) PublishSyncgroup(ctx *context.T, call rpc.ServerCall, publisher string, sg interfaces.Syncgroup, version string, genvec interfaces.GenVector) (string, error) {
	vlog.VI(2).Infof("sync: PublishSyncgroup: begin: %s from peer %s", sg.Id, publisher)
	defer vlog.VI(2).Infof("sync: PublishSyncgroup: end: %s from peer %s", sg.Id, publisher)

	st, err := s.getDbStore(ctx, call, sg.DbId)
	if err != nil {
		return s.name, err
	}

	gid := SgIdToGid(sg.DbId, sg.Id)

	err = watchable.RunInTransaction(st, func(tx *watchable.Transaction) error {
		// Check if the syncgroup exists locally.
		state, err := getSGIdEntry(ctx, tx, gid)
		if err != nil && verror.ErrorID(err) != verror.ErrNoExist.ID {
			return err
		}

		if err == nil {
			// SG exists locally, either locally created/joined or
			// previously published.  Make it idempotent for the
			// same publisher, otherwise it's a duplicate.
			if state.RemotePublisher == "" {
				// Locally created/joined syncgroup: update its
				// state to include the publisher.
				state.RemotePublisher = publisher
				return setSGIdEntry(ctx, tx, gid, state)
			}
			if publisher == state.RemotePublisher {
				// Same previous publisher: nothing to change,
				// the old genvec and version info is valid.
				return nil
			}
			return interfaces.NewErrDupSyncgroupPublish(ctx, sg.Id)
		}

		// Publish the syncgroup.

		// TODO(hpucha): Use some ACL check to allow/deny publishing.
		// TODO(hpucha): Ensure node is on Admin ACL.

		return s.addSyncgroup(ctx, tx, version, false, publisher, genvec, 0, 0, 0, &sg)
	})

	if err != nil {
		return s.name, err
	}

	s.initSyncStateInMem(ctx, sg.DbId, sgOID(gid))

	// Advertise the Syncbase at the chosen mount table and in the
	// neighborhood.
	//
	// TODO(hpucha): Implement failure handling. See note in
	// CreateSyncgroup for more details.
	err = s.advertiseSyncbase(ctx, call, &sg)

	return s.name, err
}

func (s *syncService) JoinSyncgroupAtAdmin(ctx *context.T, call rpc.ServerCall, dbId, sgId wire.Id, joinerName string, joinerInfo wire.SyncgroupMemberInfo) (interfaces.Syncgroup, string, interfaces.GenVector, error) {
	vlog.VI(2).Infof("sync: JoinSyncgroupAtAdmin: begin: %+v from peer %s", sgId, joinerName)
	defer vlog.VI(2).Infof("sync: JoinSyncgroupAtAdmin: end: %+v from peer %s", sgId, joinerName)

	nullSG, nullGV := interfaces.Syncgroup{}, interfaces.GenVector{}

	// If this admin is offline, it shouldn't accept the join request since it
	// would be unable to send out the new syncgroup updates. However, it is still
	// possible that the admin goes offline right after processing the request.
	if !s.isDbSyncable(ctx, dbId) {
		return nullSG, "", nullGV, interfaces.NewErrDbOffline(ctx, dbId)
	}

	// Find the database store for this syncgroup.
	dbSt, err := s.getDbStore(ctx, call, dbId)
	if err != nil {
		return nullSG, "", nullGV, verror.New(verror.ErrNoExist, ctx, "Database not found", dbId)
	}

	gid := SgIdToGid(dbId, sgId)
	if _, err = getSyncgroupVersion(ctx, dbSt, gid); err != nil {
		vlog.VI(4).Infof("sync: JoinSyncgroupAtAdmin: end: %v from peer %s, err in sg search %v", sgId, joinerName, err)
		return nullSG, "", nullGV, verror.New(verror.ErrNoExist, ctx, "Syncgroup not found", sgId)
	}

	version := s.newSyncgroupVersion()
	sgoid := sgOID(gid)
	var sg *interfaces.Syncgroup
	var gen, pos uint64

	err = watchable.RunInTransaction(dbSt, func(tx *watchable.Transaction) error {
		var err error
		sg, err = getSyncgroupByGid(ctx, tx, gid)
		if err != nil {
			return err
		}

		// Check SG ACL to see if this node is still a valid admin.
		if !syncgroupAdmin(s.ctx, sg.Spec.Perms) {
			return interfaces.NewErrNotAdmin(ctx)
		}

		// Check SG ACL. Caller must have Read access on the syncgroup
		// ACL to join a syncgroup.
		if err := authorize(ctx, call.Security(), sg); err != nil {
			return err
		}

		// Check that the SG is not in pending state.
		state, err := getSGIdEntry(ctx, tx, gid)
		if err != nil {
			return err
		}
		if state.SyncPending {
			return verror.NewErrBadState(ctx)
		}

		// Reserve a log generation and position counts for the new syncgroup.
		gen, pos = s.reserveGenAndPosInDbLog(ctx, dbId, sgoid, 1)

		// Add to joiner list.
		now, err := s.vclock.Now()
		if err != nil {
			return err
		}
		sg.Joiners[joinerName] = interfaces.SyncgroupMemberState{WhenUpdated: now.Unix(), HasLeft: false, MemberInfo: joinerInfo}
		return s.updateSyncgroupVersioning(ctx, tx, gid, version, true, s.id, gen, pos, sg)
	})

	if err != nil {
		vlog.VI(4).Infof("sync: JoinSyncgroupAtAdmin: end: %v from peer %s, err in tx %v", sgId, joinerName, err)
		return nullSG, "", nullGV, err
	}

	if err := s.checkptSgLocalGen(ctx, dbId, gid); err != nil {
		return nullSG, "", nullGV, err
	}

	sgs := sgSet{gid: struct{}{}}
	gv, _, err := s.copyDbGenInfo(ctx, dbId, sgs)
	if err != nil {
		vlog.VI(4).Infof("sync: JoinSyncgroupAtAdmin: end: %v from peer %s, err in copy %v", sgId, joinerName, err)
		return nullSG, "", nullGV, err
	}
	// The retrieved genvector does not contain the mutation that adds the
	// joiner to the list since initiator is the one checkpointing the
	// generations. Add that generation to this genvector.
	gv[sgoid][s.id] = gen

	vlog.VI(2).Infof("sync: JoinSyncgroupAtAdmin: returning: sg %v, vers %v, genvec %v", sg, version, gv[sgoid])
	return *sg, version, gv[sgoid], nil
}
