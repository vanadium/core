// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Initiator requests deltas from a chosen peer for all the syncgroups in common
// across all databases. It then modifies the sync metadata (DAG and local log
// records) based on the deltas, detects and resolves conflicts if any, and
// suitably updates the local Databases.

import (
	"sort"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// getDeltasFromPeer performs an initiation round once per database to the specified
// peer. An initiation round consists of an identification and filtering step
// and two sync rounds:
// * Sync syncgroup metadata.
// * Sync data.
//
// In the identification step, the initiator connects to the peer to identify it
// and obtain its list of blessings. It then uses these blessings to filter the
// list of syncgroups that are in common with the peer (learned based on the
// joiner list) to only include those syncgroups whose acl is satisfied by the
// peer's blessings. These syncgroups are included in the next two sync
// rounds. In the next two sync rounds, the initiator ensures that the peer
// identification (blessings) used in the first step has not changed.
//
// Note that alternately we could have used the blessings a peer uses to join
// the syncgroup to identify and validate it during sync rounds. However, this
// tight coupling prevents a peer who receives another valid blessing that
// satisfies the syncgroup acl after joining but loses the one used during join,
// from syncing.
//
// Each sync round involves:
// * Contacting the peer to receive all the deltas based on the local genvectors.
// * Processing those deltas to discover objects which have been updated.
// * Processing updated objects to detect and resolve any conflicts if needed.
// * Communicating relevant object updates to the Database in case of data.
// * Updating local genvectors to catch up to the received remote genvectors.
//
// The processing of the deltas is done one Database at a time, encompassing all
// the syncgroups common to the initiator and the responder. If a local error is
// encountered during the processing of a Database, that Database is skipped and
// the initiator continues on to the next one. If the connection to the peer
// encounters an error, this initiation round is aborted. Note that until the
// local genvectors are updated based on the received deltas (the last step in an
// initiation round), the work done by the initiator is idempotent.
//
// TODO(hpucha): Check the idempotence, esp in addNode in DAG.
func (s *syncService) getDeltasFromPeer(ctx *context.T, peer connInfo) error {
	vlog.VI(2).Infof("sync: getDeltasFromPeer: begin: contacting peer %v", peer)
	defer vlog.VI(2).Infof("sync: getDeltasFromPeer: end: contacting peer %v", peer)

	var errFinal error // the last non-nil error encountered is returned to the caller.

	info := s.copyMemberInfo(ctx, peer.relName)
	if info == nil {
		vlog.VI(4).Infof("sync: getDeltasFromPeer: copyMemberInfo failed %v", peer)
		return verror.New(verror.ErrInternal, ctx, peer.relName, "no member info found")
	}

	// Sync each Database that may have syncgroups common with this peer,
	// one at a time.
	for dbId, dbInfo := range info.db2sg {
		if len(peer.mtTbls) < 1 && len(peer.addrs) < 1 {
			vlog.Errorf("sync: getDeltasFromPeer: no mount tables or endpoints found to connect to peer %v", peer)
			return verror.New(verror.ErrInternal, ctx, peer.relName, peer.addrs, "all mount tables failed")
		}

		var err error
		peer, err = s.getDBDeltas(ctx, dbId, dbInfo, peer)

		if verror.ErrorID(err) == interfaces.ErrConnFail.ID {
			return err
		}

		if err != nil {
			errFinal = err
		}
	}
	return errFinal
}

// getDBDeltas performs an initiation round for the specified database.
func (s *syncService) getDBDeltas(ctx *context.T, dbId wire.Id, info sgMember, peer connInfo) (connInfo, error) {
	vlog.VI(2).Infof("sync: getDBDeltas: begin: contacting peer %v for db %v", peer, dbId)
	defer vlog.VI(2).Infof("sync: getDBDeltas: end: contacting peer %v for db %v", peer, dbId)

	// Note that the "identify" step is done once per database for privacy
	// reasons. When the app blesses Syncbase during syncgroup create/join,
	// these blessings would be associated with that database, and should
	// not be revealed when that database is not being synced.
	//
	// TODO(hpucha): Revisit which blessings are sent.
	blessingNames := s.identifyPeer(ctx, peer)

	// The initiation config is shared across the sync rounds in an
	// initiation. The mount tables in the config are pruned to eliminate
	// the ones that failed to reach the peer during each RPC call. This
	// helps the next RPC call to use the currently successful or the
	// not-yet tried mount tables to reach a peer, instead of retrying the
	// already failed ones.
	//
	// TODO(hpucha): Clean up sharing of the initiationConfig.
	c, err := newInitiationConfig(ctx, s, dbId, info, peer)
	if err != nil {
		return peer, err
	}

	// Filter the syncgroups using the peer blessings, and populate the
	// syncgroup ids, the syncgroup prefixes and the remote peer blessings
	// to be used in the next two rounds of sync.
	if err := s.filterSyncgroups(ctx, c, blessingNames); err != nil {
		return c.peer, err
	}

	// Sync syncgroup metadata.
	if err = s.getDeltas(ctx, c, true); err != nil {
		// If syncgroup sync fails, abort data sync as well.
		return c.peer, err
	}

	// Sync data.
	err = s.getDeltas(ctx, c, false)
	return c.peer, err
}

func (s *syncService) identifyPeer(ctx *context.T, peer connInfo) []string {
	conn := peer.pinned.Conn()
	blessingNames, _ := security.RemoteBlessingNames(ctx, security.NewCall(&security.CallParams{
		Timestamp:        time.Now(),
		LocalPrincipal:   v23.GetPrincipal(ctx),
		RemoteBlessings:  conn.RemoteBlessings(),
		RemoteDischarges: conn.RemoteDischarges(),
		LocalEndpoint:    conn.LocalEndpoint(),
		RemoteEndpoint:   conn.RemoteEndpoint(),
	}))
	return blessingNames
}

// addPrefixesToMap adds to map m the prefixes of syncgroup *sg and their IDs.
func addPrefixesToMap(m map[string]sgSet, gid interfaces.GroupId, sg *interfaces.Syncgroup) {
	for _, c := range sg.Spec.Collections {
		pfxStr := toCollectionPrefixStr(c)
		sgs, ok := m[pfxStr]
		if !ok {
			sgs = make(sgSet)
			m[pfxStr] = sgs
		}
		sgs[gid] = struct{}{}
	}
}

// filterSyncgroups uses the remote peer's blessings to obtain the set of
// syncgroups that are allowed to be synced with it based on its current
// syncgroup acls.
func (s *syncService) filterSyncgroups(ctx *context.T, c *initiationConfig, blessingNames []string) error {
	vlog.VI(2).Infof("sync: filterSyncGroups: begin")
	defer vlog.VI(2).Infof("sync: filterSyncGroups: end")

	// Perform authorization.
	if len(blessingNames) == 0 {
		return verror.New(verror.ErrNoAccess, ctx)
	}

	vlog.VI(4).Infof("sync: filterSyncGroups: got peer names %v", blessingNames)

	remSgIds := make(sgSet)

	// Prepare the list of syncgroup prefixes known by this syncbase.
	c.allSgPfxs = make(map[string]sgSet)
	// Fetch the syncgroup data entries in the current database by scanning their
	// prefix range.  Use a database snapshot for the scan.
	snapshot := c.st.NewSnapshot()
	defer snapshot.Abort()
	forEachSyncgroup(snapshot, func(gid interfaces.GroupId, sg *interfaces.Syncgroup) bool {
		addPrefixesToMap(c.allSgPfxs, gid, sg) // Add syncgroups prefixes to allSgPfxs.
		return false                           // from forEachSyncgroup closure
	})

	// Prepare the syncgroup prefixes shared with the caller since we are going through each
	// syncgroup. This is an optimization done to avoid looping and reading
	// syncgroup data another time.
	c.sharedSgPfxs = make(map[string]sgSet)
	for gid := range c.sgIds {
		var sg *interfaces.Syncgroup
		sg, err := getSyncgroupByGid(ctx, c.st, gid)
		if err != nil {
			continue
		}

		if _, ok := sg.Joiners[c.peer.relName]; !ok {
			// Peer is no longer part of the syncgroup.
			continue
		}

		// TODO(hpucha): Is using the Read tag to authorize the remote
		// peer ok? The thinking here is that the peer uses the read tag
		// on the GetDeltas RPC to authorize the initiator and this
		// makes it symmetric.
		if !isAuthorizedForTag(sg.Spec.Perms, access.Read, blessingNames) {
			vlog.VI(4).Infof("sync: filterSyncGroups: skipping sg %v", gid)
			continue
		}

		remSgIds[gid] = struct{}{}
		addPrefixesToMap(c.sharedSgPfxs, gid, sg) // Add syncgroups prefixes to sharedSgPfxs.
	}

	c.sgIds = remSgIds

	if len(c.sgIds) == 0 {
		return verror.New(verror.ErrInternal, ctx, "no syncgroups found after filtering", c.peer.relName, c.dbId)
	}

	if len(c.sharedSgPfxs) == 0 {
		return verror.New(verror.ErrInternal, ctx, "no syncgroup prefixes found", c.peer.relName, c.dbId)
	}

	sort.Strings(blessingNames)
	c.auth = &namesAuthorizer{blessingNames}
	return nil
}

// getDeltas gets the deltas from the chosen peer. If sg flag is set to true, it
// will sync syncgroup metadata. If sg flag is false, it will sync data.
func (s *syncService) getDeltas(ctxIn *context.T, c *initiationConfig, sg bool) error {
	vlog.VI(2).Infof("sync: getDeltas: begin: contacting peer sg %v %v", sg, c.peer)
	defer vlog.VI(2).Infof("sync: getDeltas: end: contacting peer sg %v %v", sg, c.peer)

	ctx, cancel := context.WithCancel(ctxIn)
	// cancel() is idempotent.
	defer cancel()

	// Initialize initiation state for syncing this Database.
	iSt := newInitiationState(ctx, c, sg)

	if sg {
		// Create local genvecs so that they contain knowledge about
		// common syncgroups and then send the syncgroup metadata sync
		// request.
		if err := iSt.prepareSGDeltaReq(ctx); err != nil {
			return err
		}
	} else {
		// Create local genvecs so that they contain knowledge only about common
		// prefixes and then send the data sync request.
		if err := iSt.prepareDataDeltaReq(ctx); err != nil {
			return err
		}
	}

	op := func(ctx *context.T, peer string) (interface{}, error) {
		c := interfaces.SyncClient(peer)
		var err error
		// The authorizer passed to the RPC ensures that the remote
		// peer's blessing names are the same as obtained in the
		// identification step done previously.
		//
		// We set options.ConnectionTimeout to 0 here to indicate that we only want to
		// use connections that exist in the client cache. This is because we are trying
		// to connect to a peer that is active as determined by the peer manager.
		iSt.stream, err = c.GetDeltas(ctx, iSt.req, iSt.config.sync.name,
			options.ServerAuthorizer{iSt.config.auth}, options.ChannelTimeout(channelTimeout),
			options.ConnectionTimeout(syncConnectionTimeout))
		return nil, err
	}

	// Make contact with the peer to start getting the deltas.
	var err error
	var runAtPeerCancel context.CancelFunc
	c.peer, _, runAtPeerCancel, err = runAtPeer(ctx, c.peer, op)
	defer runAtPeerCancel()
	if err != nil {
		return err
	}

	// Obtain deltas from the peer over the network.
	if err := iSt.recvAndProcessDeltas(ctx); err != nil {
		// Note, it's important to call cancel before calling Finish so that we
		// don't block waiting for the rest of the stream.
		cancel()
		// Call Finish to clean up local state even on failure.
		iSt.stream.Finish()
		return err
	}

	deltaFinalResp, err := iSt.stream.Finish()
	if err != nil {
		return err
	}

	if !iSt.sg {
		// TODO(m3b): It is unclear what to do if this call returns an error.  We would not wish the GetDeltas call to fail.
		updateAllSyncgroupPriorities(ctx, s.bst, deltaFinalResp.SgPriorities)
	}

	vlog.VI(4).Infof("sync: getDeltas: got reply: %v", iSt.remote)

	// Process deltas locally.
	return iSt.processUpdatedObjects(ctx)
}

////////////////////////////////////////////////////////////////////////////////
// Internal helpers for initiation config.

type sgSet map[interfaces.GroupId]struct{}

// initiationConfig is the configuration information for a Database in an
// initiation round.
type initiationConfig struct {
	// Connection info of the peer to sync with. Contains mount tables that
	// this peer may have registered with. The first entry in this array is
	// the mount table where the peer was successfully reached the last
	// time. Similarly, the first entry in the neighborhood addrs is the one
	// where the peer was successfully reached the last time.
	peer connInfo

	sgIds        sgSet            // Syncgroups being requested in the initiation round.
	sharedSgPfxs map[string]sgSet // Syncgroup prefixes and their ids being requested in the initiation round.
	allSgPfxs    map[string]sgSet // Syncgroup prefixes and their ids, for all syncgroups known to this device.
	sync         *syncService
	dbId         wire.Id
	db           interfaces.Database // handle to the Database.
	st           *watchable.Store    // Store handle to the Database.

	// Authorizer created during the filtering of syncgroups phase. This is
	// to be used during getDeltas to authorize the peer being synced with.
	auth *namesAuthorizer
}

// newInitiatonConfig creates new initiation config. This will be shared between
// the two sync rounds in the initiation round of a Database.
func newInitiationConfig(ctx *context.T, s *syncService, dbId wire.Id, info sgMember, peer connInfo) (*initiationConfig, error) {
	c := &initiationConfig{
		peer: peer,
		// Note: allSgPfxs and sharedSgPfxs will be inited during syncgroup
		// filtering.
		sgIds: make(sgSet),
		sync:  s,
		dbId:  dbId,
	}
	for id := range info {
		c.sgIds[id] = struct{}{}
	}
	if len(c.sgIds) == 0 {
		return nil, verror.New(verror.ErrInternal, ctx, "no syncgroups found", peer.relName, dbId)
	}

	// TODO(hpucha): nil rpc.ServerCall ok?
	var err error
	c.db, err = s.sv.Database(ctx, nil, dbId)
	if err != nil {
		return nil, err
	}
	c.st = c.db.St()
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
// Internal helpers for receiving and for preliminary processing of all the log
// records over the network.

// initiationState is accumulated for a Database in each sync round in an
// initiation round.
type initiationState struct {
	// Config information.
	config *initiationConfig

	// Accumulated sync state.
	local      interfaces.Knowledge         // local generation vectors.
	remote     interfaces.Knowledge         // generation vectors from the remote peer.
	updLocal   interfaces.Knowledge         // updated local generation vectors at the end of sync round.
	updObjects map[string]*objConflictState // tracks updated objects during a log replay.
	dagGraft   *graftMap                    // DAG state that tracks conflicts and common ancestors.

	req    interfaces.DeltaReq                // GetDeltas RPC request.
	stream interfaces.SyncGetDeltasClientCall // stream handle for the GetDeltas RPC.

	// Flag to indicate if this is syncgroup metadata sync.
	sg bool

	// Transaction handle for the sync round. Used during the update
	// of objects in the Database.
	tx *watchable.Transaction
}

// objConflictState contains the conflict state for an object that is updated
// during an initiator round.
type objConflictState struct {
	// In practice, isConflict and isAddedByCr cannot both be true.
	isAddedByCr bool
	isConflict  bool
	newHead     string
	oldHead     string
	ancestor    string
	res         *conflictResolution
	// TODO(jlodhia): Add perms object and version for the row keys for pickNew
}

// newInitiationState creates new initiation state.
func newInitiationState(ctx *context.T, c *initiationConfig, sg bool) *initiationState {
	iSt := &initiationState{}
	iSt.config = c
	iSt.updObjects = make(map[string]*objConflictState)
	iSt.dagGraft = newGraft(c.st)
	iSt.sg = sg
	return iSt
}

// prepareDataDeltaReq creates the generation vectors with local knowledge for
// the initiator to send to the responder, and creates the request to start the
// data sync.
//
// TODO(hpucha): Refactor this code with computeDelta code in sync_state.go.
func (iSt *initiationState) prepareDataDeltaReq(ctx *context.T) error {
	// isDbSyncable reads the in-memory syncState for this db to verify if
	// it is allowed to sync or not. This state is mutated by watcher based
	// on incoming pause/resume requests.
	if !iSt.config.sync.isDbSyncable(ctx, iSt.config.dbId) {
		// The database is offline. Skip the db till it becomes syncable again.
		vlog.VI(1).Infof("sync: prepareDataDeltaReq: database not allowed to sync, skipping sync on db %v", iSt.config.dbId)
		return interfaces.NewErrDbOffline(ctx, iSt.config.dbId)
	}

	local, lgen, err := iSt.config.sync.copyDbGenInfo(ctx, iSt.config.dbId, nil)
	if err != nil {
		return err
	}

	localPfxs := extractAndSortPrefixes(local)

	sgPfxs := make([]string, len(iSt.config.sharedSgPfxs))
	i := 0
	for p := range iSt.config.sharedSgPfxs {
		sgPfxs[i] = p
		i++
	}
	sort.Strings(sgPfxs)

	iSt.local = make(interfaces.Knowledge)

	if len(sgPfxs) == 0 {
		return verror.New(verror.ErrInternal, ctx, "no syncgroups for syncing")
	}

	pfx := sgPfxs[0]
	for _, p := range sgPfxs {
		if strings.HasPrefix(p, pfx) && p != pfx {
			continue
		}

		// Process this prefix as this is the start of a new set of
		// nested prefixes.
		pfx = p
		var lpStart string
		for _, lp := range localPfxs {
			if !strings.HasPrefix(lp, pfx) && !strings.HasPrefix(pfx, lp) {
				// No relationship with pfx.
				continue
			}
			if strings.HasPrefix(pfx, lp) {
				lpStart = lp
			} else {
				iSt.local[lp] = local[lp]
			}
		}
		// Deal with the starting point.
		if lpStart == "" {
			// No matching prefixes for pfx were found.
			iSt.local[pfx] = make(interfaces.GenVector)
			iSt.local[pfx][iSt.config.sync.id] = lgen
		} else {
			iSt.local[pfx] = local[lpStart]
		}
	}

	// Send request.
	req := interfaces.DataDeltaReq{
		DbId:  iSt.config.dbId,
		SgIds: iSt.config.sgIds,
		Gvs:   iSt.local,
	}

	iSt.req = interfaces.DeltaReqData{req}

	vlog.VI(4).Infof("sync: prepareDataDeltaReq: request: %v", req)

	return nil
}

// prepareSGDeltaReq creates the syncgroup generation vectors with local
// knowledge for the initiator to send to the responder, and prepares the
// request to start the syncgroup sync.
func (iSt *initiationState) prepareSGDeltaReq(ctx *context.T) error {

	if !iSt.config.sync.isDbSyncable(ctx, iSt.config.dbId) {
		// The database is offline. Skip the db till it becomes syncable again.
		vlog.VI(1).Infof("sync: prepareSGDeltaReq: database not allowed to sync, skipping sync on db %v", iSt.config.dbId)
		return interfaces.NewErrDbOffline(ctx, iSt.config.dbId)
	}

	var err error
	iSt.local, _, err = iSt.config.sync.copyDbGenInfo(ctx, iSt.config.dbId, iSt.config.sgIds)
	if err != nil {
		return err
	}

	// Send request.
	req := interfaces.SgDeltaReq{
		DbId: iSt.config.dbId,
		Gvs:  iSt.local,
	}

	iSt.req = interfaces.DeltaReqSgs{req}

	vlog.VI(4).Infof("sync: prepareSGDeltaReq: request: %v", req)

	return nil
}

// recvAndProcessDeltas first receives the log records and generation vectors
// from the GetDeltas RPC and puts them in the Database. It also replays the
// entire log stream as the log records arrive. These records span multiple
// generations from different devices. It does not perform any conflict
// resolution during replay.  This avoids resolving conflicts that have already
// been resolved by other devices.
func (iSt *initiationState) recvAndProcessDeltas(ctx *context.T) error {
	// TODO(hpucha): This works for now, but figure out a long term solution
	// as this may be implementation dependent. It currently works because
	// the RecvStream call is stateless, and grabbing a handle to it
	// repeatedly doesn't affect what data is seen next.
	rcvr := iSt.stream.RecvStream()

	// TODO(hpucha): See if we can avoid committing the entire delta stream
	// as one batch. Currently the dependency is between the log records and
	// the batch info.
	tx := iSt.config.st.NewWatchableTransaction()
	committed := false

	defer func() {
		if !committed {
			tx.Abort()
		}
	}()

	// Track received batches (BatchId --> BatchCount mapping).
	batchMap := make(map[uint64]uint64)

	for rcvr.Advance() {
		resp := rcvr.Value()
		switch v := resp.(type) {
		case interfaces.DeltaRespGvs:
			iSt.remote = v.Value

		case interfaces.DeltaRespRec:
			// Insert log record in Database.
			// TODO(hpucha): Should we reserve more positions in a batch?
			// TODO(hpucha): Handle if syncgroup is left/destroyed while sync is in progress.
			var pos uint64
			if iSt.sg {
				pos = iSt.config.sync.reservePosInDbLog(ctx, iSt.config.dbId, v.Value.Metadata.ObjId, 1)
			} else {
				pos = iSt.config.sync.reservePosInDbLog(ctx, iSt.config.dbId, "", 1)
			}

			rec := &LocalLogRec{Metadata: v.Value.Metadata, Pos: pos}
			batchId := rec.Metadata.BatchId
			if batchId != NoBatchId {
				if cnt, ok := batchMap[batchId]; !ok {
					if iSt.config.sync.startBatch(ctx, tx, batchId) != batchId {
						return verror.New(verror.ErrInternal, ctx, "failed to create batch info")
					}
					batchMap[batchId] = rec.Metadata.BatchCount
				} else if cnt != rec.Metadata.BatchCount {
					return verror.New(verror.ErrInternal, ctx, "inconsistent counts for tid", batchId, cnt, rec.Metadata.BatchCount)
				}
			}

			vlog.VI(4).Infof("sync: recvAndProcessDeltas: processing rec %v", rec)
			if err := iSt.insertRecInLogAndDag(ctx, rec, batchId, tx); err != nil {
				return err
			}

			if iSt.sg {
				// Add the syncgroup value to the Database.
				if err := iSt.insertSgRecInDb(ctx, rec, v.Value.Value, tx); err != nil {
					return err
				}
			} else {
				if err := iSt.insertRecInDb(ctx, rec, v.Value.Value, tx); err != nil {
					return err
				}
				// Check for BlobRefs, and process them.
				if err := iSt.config.sync.processBlobRefs(ctx, iSt.config.dbId, tx, iSt.config.peer.relName, false, iSt.config.allSgPfxs, iSt.config.sharedSgPfxs, &rec.Metadata, v.Value.Value); err != nil {
					return err
				}
			}

			// Mark object dirty.
			iSt.updObjects[rec.Metadata.ObjId] = &objConflictState{}
		}
	}

	if err := rcvr.Err(); err != nil {
		return err
	}

	// End the started batches if any.
	for bid, cnt := range batchMap {
		if err := iSt.config.sync.endBatch(ctx, tx, bid, cnt); err != nil {
			return err
		}
	}

	// Commit this transaction. We do not retry this transaction since it
	// should not conflict with any other keys. So if it fails, it is a
	// non-retriable error.
	err := tx.Commit()
	if verror.ErrorID(err) == store.ErrConcurrentTransaction.ID {
		// Note: This might be triggered with memstore until it handles
		// transactions in a more fine-grained fashion.
		vlog.Fatalf("sync: recvAndProcessDeltas: encountered concurrent transaction")
	}
	if err == nil {
		committed = true
	}
	return err
}

// insertRecInLogAndDag adds a new log record to log and dag data structures.
func (iSt *initiationState) insertRecInLogAndDag(ctx *context.T, rec *LocalLogRec, batchId uint64, tx store.Transaction) error {
	var pfx string
	m := rec.Metadata

	if iSt.sg {
		pfx = m.ObjId
	} else {
		pfx = logDataPrefix
	}

	if err := putLogRec(ctx, tx, pfx, rec); err != nil {
		return err
	}
	logKey := logRecKey(pfx, m.Id, m.Gen)

	var err error
	switch m.RecType {
	case interfaces.NodeRec:
		err = iSt.config.sync.addNode(ctx, tx, m.ObjId, m.CurVers, logKey, m.Delete, m.Parents, m.BatchId, iSt.dagGraft)
	case interfaces.LinkRec:
		err = iSt.config.sync.addParent(ctx, tx, m.ObjId, m.CurVers, m.Parents[0], m.BatchId, iSt.dagGraft)
	default:
		err = verror.New(verror.ErrInternal, ctx, "unknown log record type")
	}

	return err
}

// insertSgRecInDb inserts the versioned value of a syncgroup in the Database.
func (iSt *initiationState) insertSgRecInDb(ctx *context.T, rec *LocalLogRec, rawValue *vom.RawBytes, tx store.Transaction) error {
	m := rec.Metadata
	var sg interfaces.Syncgroup
	if err := rawValue.ToValue(&sg); err != nil {
		return err
	}
	return setSGDataEntryByOID(ctx, tx, m.ObjId, m.CurVers, &sg)
}

// insertRecInDb inserts the versioned value in the Database.
func (iSt *initiationState) insertRecInDb(ctx *context.T, rec *LocalLogRec, rawValue *vom.RawBytes, tx *watchable.Transaction) error {
	m := rec.Metadata
	// TODO(hpucha): Hack right now. Need to change Database's handling of
	// deleted objects. Currently, the initiator needs to treat deletions
	// specially since deletions do not get a version number or a special
	// value in the Database.
	if !rec.Metadata.Delete && rec.Metadata.RecType == interfaces.NodeRec {
		var valbuf []byte
		var err error
		if valbuf, err = vom.Encode(rawValue); err != nil {
			return err
		}
		return watchable.PutAtVersion(ctx, tx, []byte(m.ObjId), valbuf, []byte(m.CurVers))
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Internal helpers for finishing the local processing of all the log records
// received over the network.

// processUpdatedObjects processes all the updates received by the initiator,
// one object at a time. Conflict detection and resolution is carried out after
// the entire delta of log records is replayed, instead of incrementally after
// each record/batch is replayed, to avoid repeating conflict resolution already
// performed by other peers.
//
// For each updated object, we first check if the object has any conflicts,
// resulting in three possibilities:
//
// * There is no conflict, and no updates are needed to the Database
// (isConflict=false, newHead == oldHead). All changes received convey
// information that still keeps the local head as the most recent version. This
// occurs when conflicts are resolved by picking the existing local version.
//
// * There is no conflict, but a remote version is discovered that builds on the
// local head (isConflict=false, newHead != oldHead). In this case, we generate
// a Database update to simply update the Database to the latest value.
//
// * There is a conflict and we call into the app or use a well-known policy to
// resolve the conflict, resulting in three possibilities: (a) conflict was
// resolved by picking the local version. In this case, Database need not be
// updated, but a link is added to record the choice. (b) conflict was resolved
// by picking the remote version. In this case, Database is updated with the
// remote version and a link is added as well. (c) conflict was resolved by
// generating a new Database update. In this case, Database is updated with the
// new version.
//
// We collect all the updates to the Database in a transaction. In addition, as
// part of the same transaction, we update the log and dag state suitably (move
// the head ptr of the object in the dag to the latest version, and create a new
// log record reflecting conflict resolution if any). Finally, we update the
// sync state first on storage. This transaction's commit can fail since
// preconditions on the objects may have been violated. In this case, we wait to
// get the latest versions of objects from the Database, and recheck if the object
// has any conflicts and repeat the above steps, until the transaction commits
// successfully. Upon commit, we also update the in-memory sync state of the
// Database.
func (iSt *initiationState) processUpdatedObjects(ctx *context.T) error {
	// Note that the tx handle in initiation state is cached for the scope of
	// this function only as different stages in the pipeline add to the
	// transaction.
	committed := false
	defer func() {
		if !committed {
			iSt.tx.Abort()
		}
	}()

	for {
		vlog.VI(4).Infof("sync: processUpdatedObjects: begin: %d objects updated", len(iSt.updObjects))

		iSt.tx = iSt.config.st.NewWatchableTransaction()
		watchable.SetTransactionFromSync(iSt.tx) // for echo-suppression

		if count, err := iSt.detectConflicts(ctx); err != nil {
			return err
		} else {
			vlog.VI(4).Infof("sync: processUpdatedObjects: %d conflicts detected", count)
		}

		if err := iSt.resolveConflicts(ctx); err != nil {
			return err
		}

		err := iSt.updateDbAndSyncSt(ctx)
		if err == nil {
			err = iSt.tx.Commit()
		}
		if err == nil {
			committed = true
			// Update in-memory genvectors since commit is successful.
			if err := iSt.config.sync.putDbGenInfoRemote(ctx, iSt.config.dbId, iSt.sg, iSt.updLocal); err != nil {
				vlog.Fatalf("sync: processUpdatedObjects: putting geninfo in memory failed for db %v, err %v", iSt.config.dbId, err)
			}

			// There is no need to wait for the new advertisement to finish so
			// we just run it asynchronously in its own goroutine.
			go iSt.advertiseSyncgroups(ctx)

			vlog.VI(4).Info("sync: processUpdatedObjects: end: changes committed")
			return nil
		}
		if verror.ErrorID(err) != store.ErrConcurrentTransaction.ID {
			return err
		}

		// Either updateDbAndSyncSt() or tx.Commit() detected a
		// concurrent transaction. Retry processing the remote updates.
		//
		// TODO(hpucha): Sleeping and retrying is a temporary
		// solution. Next iteration will have coordination with watch
		// thread to intelligently retry. Hence this value is not a
		// config param.
		vlog.VI(4).Info("sync: processUpdatedObjects: retry due to local mutations")
		iSt.tx.Abort()
		time.Sleep(1 * time.Second)
		iSt.resetConflictResolutionState(ctx)
	}
}

// detectConflicts iterates through all the updated objects to detect conflicts.
func (iSt *initiationState) detectConflicts(ctx *context.T) (int, error) {
	count := 0
	for objid, confSt := range iSt.updObjects {
		// Check if object has a conflict.
		var err error
		confSt.isConflict, confSt.newHead, confSt.oldHead, confSt.ancestor, err = hasConflict(ctx, iSt.tx, objid, iSt.dagGraft)
		if err != nil {
			return 0, err
		}

		if !confSt.isConflict {
			if confSt.newHead == confSt.oldHead {
				confSt.res = &conflictResolution{ty: pickLocal}
			} else {
				confSt.res = &conflictResolution{ty: pickRemote}
			}
		} else {
			count++
		}
	}
	return count, nil
}

// resetConflictResolutionState resets the accumulated state from conflict
// resolution. This is done prior to retrying conflict resolution when the
// initiator fails to update the store with the received changes due to
// concurent mutations from the client.
func (iSt *initiationState) resetConflictResolutionState(ctx *context.T) {
	for objid, confSt := range iSt.updObjects {
		// Check if the object was added during resolution. If so,
		// remove this object from the dirty objects list before
		// retrying; else reset the conflict resolution state.
		if confSt.isAddedByCr {
			delete(iSt.updObjects, objid)
		} else {
			iSt.updObjects[objid] = &objConflictState{}
		}
	}
}

// updateDbAndSync updates the Database, and if that is successful, updates log,
// dag and genvectors data structures as needed.
func (iSt *initiationState) updateDbAndSyncSt(ctx *context.T) error {
	// Track batches being processed (BatchId --> BatchCount mapping).
	batchMap := make(map[uint64]uint64)

	for objid, confSt := range iSt.updObjects {
		// Always update sync state irrespective of local/remote/new
		// versions being picked.
		if err := iSt.updateLogAndDag(ctx, objid, batchMap); err != nil {
			return err
		}

		// No need to update the store for syncgroup changes.
		if iSt.sg {
			continue
		}

		// If the local version is picked, no further updates to the
		// Database are needed, but we want to ensure that the local
		// version has not changed since by entering it into the readset
		// of the transaction. If the remote version is picked or if a
		// new version is created, we put it in the Database.

		// TODO(hpucha): Hack right now. Need to change Database's
		// handling of deleted objects.
		oldVersDeleted := true
		if confSt.oldHead != NoVersion {
			oldDagNode, err := getNode(ctx, iSt.tx, objid, confSt.oldHead)
			if err != nil {
				return err
			}
			oldVersDeleted = oldDagNode.Deleted
		}

		if !oldVersDeleted {
			// Read current version to enter it in the readset of the transaction.
			version, err := watchable.GetVersion(ctx, iSt.tx, []byte(objid))
			if err != nil {
				return err
			}
			if string(version) != confSt.oldHead {
				vlog.VI(4).Infof("sync: updateDbAndSyncSt: concurrent updates %s %s", version, confSt.oldHead)
				return store.NewErrConcurrentTransaction(ctx)
			}
		} else {
			// Ensure key doesn't exist.
			if _, err := watchable.GetVersion(ctx, iSt.tx, []byte(objid)); verror.ErrorID(err) != store.ErrUnknownKey.ID {
				return store.NewErrConcurrentTransaction(ctx)
			}
		}

		if confSt.res.ty == pickLocal {
			// Nothing more to do.
			continue
		}

		var newVersion string
		var newVersDeleted bool
		switch confSt.res.ty {
		case pickRemote:
			newVersion = confSt.newHead
			newDagNode, err := getNode(ctx, iSt.tx, objid, newVersion)
			if err != nil {
				return err
			}
			newVersDeleted = newDagNode.Deleted
		case createNew:
			newVersion = confSt.res.rec.Metadata.CurVers
			newVersDeleted = confSt.res.rec.Metadata.Delete
		}

		// Skip delete followed by a delete.
		if oldVersDeleted && newVersDeleted {
			continue
		}

		if !newVersDeleted {
			if confSt.res.ty == createNew {
				vlog.VI(4).Infof("sync: updateDbAndSyncSt: PutAtVersion %s %s", objid, newVersion)
				if err := watchable.PutAtVersion(ctx, iSt.tx, []byte(objid), confSt.res.val, []byte(newVersion)); err != nil {
					return err
				}
			}
			vlog.VI(4).Infof("sync: updateDbAndSyncSt: PutVersion %s %s", objid, newVersion)
			if err := watchable.PutVersion(ctx, iSt.tx, []byte(objid), []byte(newVersion)); err != nil {
				return err
			}
		} else {
			vlog.VI(4).Infof("sync: updateDbAndSyncSt: Deleting obj %s", objid)
			if err := iSt.tx.Delete([]byte(objid)); err != nil {
				return err
			}
		}
	}

	// End the started batches if any.
	for bid, cnt := range batchMap {
		if err := iSt.config.sync.endBatch(ctx, iSt.tx, bid, cnt); err != nil {
			return err
		}
	}

	return iSt.updateSyncSt(ctx)
}

// updateLogAndDag updates the log and dag data structures.
//
// Each object in the updated objects list (iSt.updObjects) is present because
// of one of the following cases:
//
// Case 1: The object is under conflict after the replay of the deltas, and was
// resolved by conflict resolution (isConflict = true, isAddedByCr = false).
//
// Case 2: The object is not under conflict after the replay of the deltas
// (isConflict = false, isAddedByCr = false). Note that even for these objects
// that are not under conflict, conflict resolution may need to modify its value
// due to its involvement in a batch.
//
// Case 3: The object was not received in the deltas but was added during
// conflict resolution due to its involvement in a batch (isConflict = false,
// isAddedByCr = true).
//
// For each object in iSt.updObjects, the following states are possible after
// conflict detection and resolution:
//
// pickLocal:
// ** Case 1: We need to create a link log record to remember the conflict
// resolution. This happens when conflict resolution picks the local head to be
// suitable for resolution.
// ** Case 2: Do nothing. This happens when this object was not involved in the
// resolution and only link log records are received in the deltas resulting in
// no updates to the local head.
// TODO(hpucha): confirm how this case is handled during app resolution.
// ** Case 3: Do nothing. This happens when conflict resolution picks the local
// head to be suitable for resolution.
//
// pickRemote: Update the dag head in all cases.
// ** Case 1: We need to create a link log record to remember the conflict
// resolution. This happens when conflict resolution picks the remote head to be
// suitable for resolution.
// ** Case 2: This happens either when this object was not involved in the
// resolution, or was involved and the remote value was suitable.
// ** Case 3: This case is not possible since the remote value is unavailable.
//
// createNew: Create a node log record and update the dag head in all cases.
// ** Case 1: This happens when conflict resolution resulted in a new value.
// ** Case 2: This happens either because resolution overwrote the newly
// received remote value with a new value, or the local value was chosen but
// needed to be rewritten with a new version to prevent a cycle in the dag.
// ** Case 3: This happens when conflict resolution resulted in a new value.
func (iSt *initiationState) updateLogAndDag(ctx *context.T, objid string, batchMap map[uint64]uint64) error {
	confSt, ok := iSt.updObjects[objid]
	if !ok {
		return verror.New(verror.ErrInternal, ctx, "object state not found", objid)
	}
	var newVersion string

	// Create log records and dag nodes as needed.
	var rec *LocalLogRec

	switch {
	case confSt.res.ty == pickLocal:
		if confSt.isConflict {
			// Local version was picked as the conflict resolution.
			rec = iSt.createLocalLinkLogRec(ctx, objid, confSt.oldHead, confSt.newHead, confSt.res.batchId, confSt.res.batchCount)
		}
		newVersion = confSt.oldHead
	case confSt.res.ty == pickRemote:
		if confSt.isConflict {
			// Remote version was picked as the conflict resolution.
			rec = iSt.createLocalLinkLogRec(ctx, objid, confSt.newHead, confSt.oldHead, confSt.res.batchId, confSt.res.batchCount)
		}
		if confSt.isAddedByCr {
			vlog.Fatalf("sync: updateLogAndDag: pickRemote with obj %v added by conflict resolution, st %v, res %v", objid, confSt, confSt.res)
		}
		newVersion = confSt.newHead
	case confSt.res.ty == createNew:
		// New version was created to resolve the conflict. Node log
		// records were created during resolution.
		rec = confSt.res.rec
		newVersion = confSt.res.rec.Metadata.CurVers
	default:
		return verror.New(verror.ErrInternal, ctx, "invalid conflict resolution type", confSt.res.ty)
	}

	if rec != nil {
		var pfx string
		if iSt.sg {
			pfx = objid
		} else {
			pfx = logDataPrefix
		}
		if err := putLogRec(ctx, iSt.tx, pfx, rec); err != nil {
			return err
		}

		batchId := rec.Metadata.BatchId
		if batchId != NoBatchId {
			if cnt, ok := batchMap[batchId]; !ok {
				if iSt.config.sync.startBatch(ctx, iSt.tx, batchId) != batchId {
					return verror.New(verror.ErrInternal, ctx, "failed to create batch info")
				}
				batchMap[batchId] = rec.Metadata.BatchCount
			} else if cnt != rec.Metadata.BatchCount {
				return verror.New(verror.ErrInternal, ctx, "inconsistent counts for tid", batchId, cnt, rec.Metadata.BatchCount)
			}
		}

		// Add a new DAG node.
		var err error
		m := rec.Metadata
		switch m.RecType {
		case interfaces.NodeRec:
			err = iSt.config.sync.addNode(ctx, iSt.tx, objid, m.CurVers, logRecKey(pfx, m.Id, m.Gen), m.Delete, m.Parents, NoBatchId, nil)
		case interfaces.LinkRec:
			err = iSt.config.sync.addParent(ctx, iSt.tx, objid, m.CurVers, m.Parents[0], m.BatchId, nil)
		default:
			return verror.New(verror.ErrInternal, ctx, "unknown log record type")
		}
		if err != nil {
			return err
		}
	}

	// Move the head. This should be idempotent. We may move head to the
	// local head in some cases.
	return moveHead(ctx, iSt.tx, objid, newVersion)
}

func (iSt *initiationState) createLocalLinkLogRec(ctx *context.T, objid, vers, par string, batchId, batchCount uint64) *LocalLogRec {
	vlog.VI(4).Infof("sync: createLocalLinkLogRec: obj %s vers %s par %s", objid, vers, par)

	var gen, pos uint64
	if iSt.sg {
		gen, pos = iSt.config.sync.reserveGenAndPosInDbLog(ctx, iSt.config.dbId, objid, 1)
	} else {
		gen, pos = iSt.config.sync.reserveGenAndPosInDbLog(ctx, iSt.config.dbId, "", 1)
	}

	rec := &LocalLogRec{
		Metadata: interfaces.LogRecMetadata{
			Id:      iSt.config.sync.id,
			Gen:     gen,
			RecType: interfaces.LinkRec,

			ObjId:      objid,
			CurVers:    vers,
			Parents:    []string{par},
			UpdTime:    time.Now().UTC(),
			BatchId:    batchId,
			BatchCount: batchCount,
		},
		Pos: pos,
	}
	return rec
}

// updateSyncSt updates local sync state at the end of an initiator cycle.
func (iSt *initiationState) updateSyncSt(ctx *context.T) error {
	// Get the current local sync state.
	dsInMem, err := iSt.config.sync.copyDbSyncStateInMem(ctx, iSt.config.dbId)
	if err != nil {
		return err
	}
	// Create the state to be persisted.
	ds := &DbSyncState{
		GenVecs:   dsInMem.genvecs,
		SgGenVecs: dsInMem.sggenvecs,
	}

	genvecs := ds.GenVecs
	if iSt.sg {
		genvecs = ds.SgGenVecs
	}
	// remote can be a subset of local.
	for rpfx, respgv := range iSt.remote {
		for lpfx, lpgv := range genvecs {
			if strings.HasPrefix(lpfx, rpfx) {
				mergeGenVectors(lpgv, respgv)
			}
		}
		if _, ok := genvecs[rpfx]; !ok {
			genvecs[rpfx] = respgv
		}

		if iSt.sg {
			// Flip sync pending if needed in case of syncgroup
			// syncing. See explanation for SyncPending flag in
			// types.vdl.
			gid, err := sgID(rpfx)
			if err != nil {
				return err
			}
			state, err := getSGIdEntry(ctx, iSt.tx, gid)
			if err != nil {
				return err
			}
			if state.SyncPending {
				curgv := genvecs[rpfx]
				res := curgv.Compare(state.PendingGenVec)
				vlog.VI(4).Infof("sync: updateSyncSt: checking join pending %v, curgv %v, res %v", state.PendingGenVec, curgv, res)
				if res >= 0 {
					state.SyncPending = false
					if err := setSGIdEntry(ctx, iSt.tx, gid, state); err != nil {
						return err
					}
				}
			}
		}
	}

	iSt.updLocal = genvecs
	// Clean the genvector of any local state. Note that local state is held
	// in gen/ckPtGen in sync state struct.
	for _, gv := range iSt.updLocal {
		delete(gv, iSt.config.sync.id)
	}

	// TODO(hpucha): Add knowledge compaction.

	// Read sync state from db within transaction to achieve atomic
	// read-modify-write semantics for isolation.
	dsOnDisk, err := getDbSyncState(ctx, iSt.tx)
	if err != nil {
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return err
		}
		dsOnDisk = &DbSyncState{}
	}
	ds.IsPaused = dsOnDisk.IsPaused
	return putDbSyncState(ctx, iSt.tx, ds)
}

// mergeGenVectors merges responder genvector into local genvector.
func mergeGenVectors(lpgv, respgv interfaces.GenVector) {
	for devid, rgen := range respgv {
		gen, ok := lpgv[devid]
		if !ok || gen < rgen {
			lpgv[devid] = rgen
		}
	}
}

func (iSt *initiationState) advertiseSyncgroups(ctx *context.T) error {
	if !iSt.sg {
		return nil
	}

	// For all the syncgroup changes we learned, see if the latest acl makes
	// this node an admin or removes it from its admin role, and if so,
	// advertise the syncgroup or cancel the existing advertisement over the
	// neighborhood as applicable.
	for objid := range iSt.updObjects {
		gid, err := sgID(objid)
		if err != nil {
			return err
		}
		var sg *interfaces.Syncgroup
		sg, err = getSyncgroupByGid(ctx, iSt.config.st, gid)
		if err != nil {
			return err
		}
		if err := iSt.config.sync.advertiseSyncgroupInNeighborhood(sg); err != nil {
			return err
		}
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Internal helpers to make different RPCs needed for syncing.

// remoteOp encapsulates an RPC operation run against "peer". The returned
// interface can contain a response message (e.g. TimeResp, for GetTime) or can
// be nil (e.g. for GetDeltas stream connections).
type remoteOp func(ctx *context.T, peer string) (interface{}, error)

// runAllCancelFuncs runs the context.CancelFunc routines in cancelFuncs.
func runAllCancelFuncs(cancelFuncs []context.CancelFunc) {
	for _, f := range cancelFuncs {
		f()
	}
}

// runAtPeer attempts to connect to the remote peer using the neighborhood
// address when specified or the mount tables obtained from all the common
// syncgroups, and runs the specified op.
func runAtPeer(ctx *context.T, peer connInfo, op remoteOp) (connInfo, interface{}, context.CancelFunc, error) {
	if len(peer.mtTbls) < 1 && len(peer.addrs) < 1 {
		return peer, nil, func() {}, verror.New(verror.ErrInternal, ctx, "no mount tables or endpoints found", peer)
	}

	updPeer := peer
	var cancelFuncs []context.CancelFunc
	if peer.addrs != nil {
		for i, addr := range peer.addrs {
			absName := naming.Join(addr, common.SyncbaseSuffix)
			resp, cancel, err := runRemoteOp(ctx, absName, op)
			cancelFuncs = append(cancelFuncs, cancel)
			if verror.ErrorID(err) != interfaces.ErrConnFail.ID {
				updPeer.addrs = updPeer.addrs[i:]
				return updPeer, resp, func() { runAllCancelFuncs(cancelFuncs) }, err
			}
		}
		updPeer.addrs = nil
	} else {
		for i, mt := range peer.mtTbls {
			absName := naming.Join(mt, peer.relName, common.SyncbaseSuffix)
			resp, cancel, err := runRemoteOp(ctx, absName, op)
			if verror.ErrorID(err) != interfaces.ErrConnFail.ID {
				cancelFuncs = append(cancelFuncs, cancel)
				updPeer.mtTbls = updPeer.mtTbls[i:]
				return updPeer, resp, func() { runAllCancelFuncs(cancelFuncs) }, err
			}
			cancelFuncs = append(cancelFuncs, cancel)
		}
		updPeer.mtTbls = nil
	}

	return updPeer, nil, func() { runAllCancelFuncs(cancelFuncs) },
		verror.New(interfaces.ErrConnFail, ctx, "couldn't connect to peer", updPeer.relName, updPeer.addrs)
}

// runRemoteOp runs the remoteOp on the server specified by absName.
// It is the caller's responsibility to call the returned context.CancelFunc
// if this call returns a nil error value.
func runRemoteOp(ctxIn *context.T, absName string, op remoteOp) (interface{}, context.CancelFunc, error) {
	vlog.VI(4).Infof("sync: runRemoteOp: begin for %v", absName)

	ctx, cancel := context.WithCancel(ctxIn)

	resp, err := op(ctx, absName)

	if err == nil {
		vlog.VI(4).Infof("sync: runRemoteOp: end op established for %s", absName)
		// Responsibility for calling cancel() is passed to caller.
		return resp, cancel, nil
	}

	vlog.VI(4).Infof("sync: runRemoteOp: end for %s, err %v", absName, err)

	// TODO(hpucha): Fix error handling so that we do not assume that any error
	// that is not ErrNoAccess or ErrGetTimeFailed is a connection error. Need to
	// chat with m3b, mattr, et al to figure out how applications should
	// distinguish RPC errors from application errors in general.
	if verror.ErrorID(err) != verror.ErrNoAccess.ID && verror.ErrorID(err) != interfaces.ErrGetTimeFailed.ID {
		err = verror.New(interfaces.ErrConnFail, ctx, "couldn't connect to peer", absName)
	}

	cancel()
	return nil, func() {}, err
}
