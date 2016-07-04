// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// New log records are created when objects in the local store are created,
// updated or deleted. Local log records are also replayed to keep the
// per-object dags consistent with the local store state. Sync module assigns
// each log record created within a Database a unique sequence number, called
// the generation number. Locally on each device, the position of each log
// record is also recorded relative to other local and remote log records.
//
// When a device receives a request to send log records, it first computes the
// missing generations between itself and the incoming request on a per-prefix
// basis. It then sends all the log records belonging to the missing generations
// in the order they occur locally (using the local log position). A device that
// receives log records over the network replays all the records received from
// another device in a single batch. Each replayed log record adds a new version
// to the dag of the object contained in the log record. At the end of replaying
// all the log records, conflict detection and resolution is carried out for all
// the objects learned during this iteration. Conflict detection and resolution
// is carried out after a batch of log records are replayed, instead of
// incrementally after each record is replayed, to avoid repeating conflict
// resolution already performed by other devices.
//
// Sync module tracks the current generation number and the current local log
// position for each Database. In addition, it also tracks the current
// generation vectors for a Database. Log records are indexed such that they can
// be selectively retrieved from the store for any missing generation from any
// device.
//
// Sync also tracks the current generation number and the current local log
// position for each mutation of a syncgroup, created on a Database. Similar to
// the data log records, these log records are used to sync syncgroup metadata.
//
// The generations for the data mutations and mutations for each syncgroup are
// in separate spaces. Data mutations in a Database start at gen 1, and grow.
// Mutations for each syncgroup start at gen 1, and grow. Thus, for the local
// data log records, the keys are of the form y:l:d:<devid>:<gen>, and the keys
// for local syncgroup log record are of the form y:l:<sgoid>:<devid>:<gen>.

// TODO(hpucha): Should this space be separate from the data or not? If it is
// not, it can provide consistency between data and syncgroup metadata. For
// example, lets say we mutate the data in a syncgroup and soon after change the
// syncgroup ACL to prevent syncing with a device. This device may not get the
// last batch of updates since the next time it will try to sync, it will be
// rejected. However implementing consistency is not straightforward. Even if we
// had syncgroup updates in the same space as the data, we need to switch to the
// right syncgroup ACL at the responder based on the requested generations.

import (
	"fmt"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// localGenInfoInMem represents the state corresponding to local generations.
type localGenInfoInMem struct {
	gen        uint64
	pos        uint64
	checkptGen uint64
}

func (in *localGenInfoInMem) deepCopy() *localGenInfoInMem {
	out := &localGenInfoInMem{
		gen:        in.gen,
		pos:        in.pos,
		checkptGen: in.checkptGen,
	}
	return out
}

// dbSyncStateInMem represents the in-memory sync state of a Database and all
// its syncgroups.
type dbSyncStateInMem struct {
	data *localGenInfoInMem // info for data.

	// Info for syncgroups. The key here is the syncgroup oid of the form
	// y:s:<groupId>. More details in syncgroup.go.
	sgs map[string]*localGenInfoInMem

	// Note: Generation vectors contain state from remote devices only.
	genvecs   interfaces.Knowledge
	sggenvecs interfaces.Knowledge

	// Tracks if sync is paused for this database.
	isPaused bool
}

func (in *dbSyncStateInMem) deepCopy() *dbSyncStateInMem {
	out := &dbSyncStateInMem{}
	out.data = in.data.deepCopy()

	out.sgs = make(map[string]*localGenInfoInMem)
	for oid, info := range in.sgs {
		out.sgs[oid] = info.deepCopy()
	}

	out.genvecs = in.genvecs.DeepCopy()
	out.sggenvecs = in.sggenvecs.DeepCopy()
	out.isPaused = in.isPaused

	return out
}

// sgPublishInfo holds information on a syncgroup waiting to be published to a
// remote peer.  It is an in-memory entry in a queue of pending syncgroups.
type sgPublishInfo struct {
	sgId    wire.Id
	dbId    wire.Id
	queued  time.Time
	lastTry time.Time
}

// initSync initializes the sync module during startup. It scans all the
// databases to initialize the following:
// a) in-memory sync state of a Database and all its syncgroups consisting of
// the current generation number, log position and generation vectors.
// b) watcher map of prefixes currently being synced.
// c) republish names in mount tables for all syncgroups.
// d) in-memory queue of syncgroups to be published.
func (s *syncService) initSync(ctx *context.T) error {
	vlog.VI(2).Infof("sync: initSync: begin")
	defer vlog.VI(2).Infof("sync: initSync: end")
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	var errFinal error
	s.syncState = make(map[wire.Id]*dbSyncStateInMem)
	newMembers := make(map[string]*memberInfo)

	s.forEachDatabaseStore(ctx, func(dbId wire.Id, st *watchable.Store) bool {
		// Fetch the sync state for data and syncgroups.
		ds, err := getDbSyncState(ctx, st)
		if err != nil && verror.ErrorID(err) != verror.ErrNoExist.ID {
			errFinal = err
			return false
		}

		dsInMem := &dbSyncStateInMem{
			data: &localGenInfoInMem{},
			sgs:  make(map[string]*localGenInfoInMem),
		}

		if err == nil {
			// Initialize in memory state from the persistent state.
			dsInMem.genvecs = ds.GenVecs
			dsInMem.sggenvecs = ds.SgGenVecs
			dsInMem.isPaused = ds.IsPaused
		}

		vlog.VI(2).Infof("sync: initSync: initing db %v, dsInMem %v", dbId, dsInMem)

		sgCount := 0

		// Scan the syncgroups and init relevant metadata.
		forEachSyncgroup(st, func(gid interfaces.GroupId, sg *interfaces.Syncgroup) bool {
			sgCount++

			// Only use syncgroups that have been marked as
			// "watchable" by the sync watcher thread. This is to
			// handle the case of a syncgroup being created but
			// Syncbase restarting before the watcher processed the
			// SyncgroupOp entry in the watch queue. It should not
			// be syncing that syncgroup's data after restart, but
			// wait until the watcher processes the entry as would
			// have happened without a restart.
			state, err := getSGIdEntry(ctx, st, gid)
			if err != nil {
				errFinal = err
				return false
			}
			if state.Watched {
				for _, c := range sg.Spec.Collections {
					addWatchPrefixSyncgroup(dbId, toCollectionPrefixStr(c), gid)
				}
			}

			if sg.Status == interfaces.SyncgroupStatusPublishPending {
				s.enqueuePublishSyncgroup(sg.Id, dbId, false)
			}

			// Refresh membership view.
			refreshSyncgroupMembers(gid, sg, dbId, newMembers)

			sgoid := sgOID(gid)
			info := &localGenInfoInMem{}
			dsInMem.sgs[sgoid] = info

			// Adjust the gen and pos for the sgoid.
			info.gen, info.pos, err = s.computeCurGenAndPos(ctx, st, sgoid, dsInMem.sggenvecs[sgoid])
			if err != nil {
				errFinal = err
				return false
			}
			info.checkptGen = info.gen - 1

			vlog.VI(4).Infof("sync: initSync: initing db %v sg %v info %v", dbId, sgoid, info)

			return false
		})

		if sgCount == 0 {
			vlog.VI(2).Infof("sync: initSync: initing db %v done (no sgs found)", dbId)
			return false
		}

		// Compute the max known data generation for each known device.
		maxgenvec := interfaces.GenVector{}
		for _, gv := range dsInMem.genvecs {
			for dev, gen := range gv {
				if gen > maxgenvec[dev] {
					maxgenvec[dev] = gen
				}
			}
		}

		// Adjust the gen and pos for the data.
		dsInMem.data.gen, dsInMem.data.pos, err = s.computeCurGenAndPos(ctx, st, logDataPrefix, maxgenvec)
		if err != nil {
			errFinal = err
			return false
		}
		dsInMem.data.checkptGen = dsInMem.data.gen - 1

		s.syncState[dbId] = dsInMem

		vlog.VI(2).Infof("sync: initSync: initing db %v done dsInMem %v (data %v)", dbId, dsInMem, dsInMem.data)

		return false
	})

	s.allMembersLock.Lock()
	s.allMembers = &memberView{expiration: time.Now().Add(memberViewTTL), members: newMembers}
	s.allMembersLock.Unlock()

	return errFinal
}

// computeCurGenAndPos computes the current local generation count and local log
// position for data or a specified syncgroup.
func (s *syncService) computeCurGenAndPos(ctx *context.T, st store.Store, pfx string, genvec interfaces.GenVector) (uint64, uint64, error) {
	found := false

	// Scan the local log records to determine latest gen and its pos.
	stream := st.Scan(common.ScanPrefixArgs(logRecsPerDeviceScanPrefix(pfx, s.id), ""))
	defer stream.Cancel()

	// Get the last value.
	var val []byte
	for stream.Advance() {
		val = stream.Value(val)
		found = true
	}

	if err := stream.Err(); err != nil {
		return 0, 0, err
	}

	var maxpos, maxgen uint64
	if found {
		var lrec LocalLogRec
		if err := vom.Decode(val, &lrec); err != nil {
			return 0, 0, err
		}
		maxpos = lrec.Pos
		maxgen = lrec.Metadata.Gen
	}

	for id, gen := range genvec {
		if gen == 0 {
			continue
		}
		// Since log records may be filtered, we search for the last
		// available log record going backwards from the generation up
		// to which a device is caught up.
		lrec, err := getPrevLogRec(ctx, st, pfx, id, gen)
		if err != nil {
			return 0, 0, err
		}
		if lrec != nil && lrec.Pos > maxpos {
			found = true
			maxpos = lrec.Pos
		}
	}

	if found {
		maxpos++
	}

	return maxgen + 1, maxpos, nil
}

// TODO(hpucha): This can be optimized using a backwards scan or a better
// search.
func getPrevLogRec(ctx *context.T, st store.Store, pfx string, dev, gen uint64) (*LocalLogRec, error) {
	for i := gen; i > 0; i-- {
		rec, err := getLogRec(ctx, st, pfx, dev, i)
		if err == nil {
			return rec, nil
		}
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return nil, err
		}
	}
	return nil, nil
}

// enqueuePublishSyncgroup appends the given syncgroup to the publish queue.
func (s *syncService) enqueuePublishSyncgroup(sgId, dbId wire.Id, attempted bool) {
	s.sgPublishQueueLock.Lock()
	defer s.sgPublishQueueLock.Unlock()

	entry := &sgPublishInfo{
		sgId:   sgId,
		dbId:   dbId,
		queued: time.Now(),
	}
	if attempted {
		entry.lastTry = entry.queued
	}
	s.sgPublishQueue.PushBack(entry)
}

// Note: For all the utilities below, if the sgid parameter is non-nil, the
// operation is performed in the syncgroup space. If nil, it is performed in the
// data space for the Database.

// reserveGenAndPosInDbLog reserves a chunk of generation numbers and log
// positions in a Database's log. Used when local updates result in log
// entries.
func (s *syncService) reserveGenAndPosInDbLog(ctx *context.T, dbId wire.Id, sgoid string, count uint64) (uint64, uint64) {
	return s.reserveGenAndPosInternal(dbId, sgoid, count, count)
}

// reservePosInDbLog reserves a chunk of log positions in a Database's log. Used
// when remote log records are received.
func (s *syncService) reservePosInDbLog(ctx *context.T, dbId wire.Id, sgoid string, count uint64) uint64 {
	_, pos := s.reserveGenAndPosInternal(dbId, sgoid, 0, count)
	return pos
}

func (s *syncService) reserveGenAndPosInternal(dbId wire.Id, sgoid string, genCount, posCount uint64) (uint64, uint64) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	ds := s.getOrCreateSyncStateInternal(dbId)
	var info *localGenInfoInMem
	if sgoid != "" {
		var ok bool
		info, ok = ds.sgs[sgoid]
		if !ok {
			info = &localGenInfoInMem{gen: 1}
			ds.sgs[sgoid] = info
		}
	} else {
		info = ds.data
	}
	gen := info.gen
	pos := info.pos

	info.gen += genCount
	info.pos += posCount

	return gen, pos
}

// checkptLocalGen freezes the local generation number for the responder's use.
func (s *syncService) checkptLocalGen(ctx *context.T, dbId wire.Id, sgs sgSet) error {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	ds, ok := s.syncState[dbId]
	if !ok {
		return verror.New(verror.ErrInternal, ctx, "db state not found", dbId)
	}

	// The frozen generation is the last generation number used, i.e. one
	// below the next available one to use.
	if len(sgs) > 0 {
		// Checkpoint requested syncgroups.
		for id := range sgs {
			info, ok := ds.sgs[sgOID(id)]
			if !ok {
				return verror.New(verror.ErrInternal, ctx, "sg state not found", dbId, id)
			}
			info.checkptGen = info.gen - 1
		}
	} else {
		ds.data.checkptGen = ds.data.gen - 1
	}
	return nil
}

// initSyncStateInMem initializes the in memory sync state of the
// database/syncgroup if needed.
func (s *syncService) initSyncStateInMem(ctx *context.T, dbId wire.Id, sgoid string) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	if s.syncState[dbId] == nil {
		s.syncState[dbId] = &dbSyncStateInMem{
			data: &localGenInfoInMem{gen: 1},
			sgs:  make(map[string]*localGenInfoInMem),
		}
	}
	if sgoid != "" {
		ds := s.syncState[dbId]
		if _, ok := ds.sgs[sgoid]; !ok {
			ds.sgs[sgoid] = &localGenInfoInMem{gen: 1}
		}
	}
	return
}

// copyDbSyncStateInMem returns a copy of the current in memory sync state of the Database.
func (s *syncService) copyDbSyncStateInMem(ctx *context.T, dbId wire.Id) (*dbSyncStateInMem, error) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	ds, ok := s.syncState[dbId]
	if !ok {
		return nil, verror.New(verror.ErrInternal, ctx, "db state not found", dbId)
	}
	return ds.deepCopy(), nil
}

// copyDbGenInfo returns a copy of the current generation information of the Database.
func (s *syncService) copyDbGenInfo(ctx *context.T, dbId wire.Id, sgs sgSet) (interfaces.Knowledge, uint64, error) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	ds, ok := s.syncState[dbId]
	if !ok {
		return nil, 0, verror.New(verror.ErrInternal, ctx, "db state not found", dbId)
	}

	var genvecs interfaces.Knowledge
	var gen uint64
	if len(sgs) > 0 {
		genvecs = make(interfaces.Knowledge)
		for id := range sgs {
			sgoid := sgOID(id)
			gv := ds.sggenvecs[sgoid]
			genvecs[sgoid] = gv.DeepCopy()
			var ckpt uint64 = 0
			if info, ok := ds.sgs[sgoid]; ok {
				ckpt = info.checkptGen
			}
			genvecs[sgoid][s.id] = ckpt
		}
	} else {
		genvecs = ds.genvecs.DeepCopy()

		// Add local generation information to the genvec.
		for _, gv := range genvecs {
			gv[s.id] = ds.data.checkptGen
		}
		gen = ds.data.checkptGen
	}
	return genvecs, gen, nil
}

// putDbGenInfoRemote puts the current remote generation information of the Database.
func (s *syncService) putDbGenInfoRemote(ctx *context.T, dbId wire.Id, sg bool, genvecs interfaces.Knowledge) error {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()

	ds, ok := s.syncState[dbId]
	if !ok {
		return verror.New(verror.ErrInternal, ctx, "db state not found", dbId)
	}

	if sg {
		ds.sggenvecs = genvecs.DeepCopy()
	} else {
		ds.genvecs = genvecs.DeepCopy()
	}

	return nil
}

// isDbSyncable checks if the given database is currently syncable.
func (s *syncService) isDbSyncable(ctx *context.T, dbId wire.Id) bool {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()
	ds := s.getOrCreateSyncStateInternal(dbId)
	return !ds.isPaused
}

// updateInMemoryPauseSt updates the in-memory state with the given isPaused state.
func (s *syncService) updateInMemoryPauseSt(ctx *context.T, dbId wire.Id, isPaused bool) {
	s.syncStateLock.Lock()
	defer s.syncStateLock.Unlock()
	ds := s.getOrCreateSyncStateInternal(dbId)
	ds.isPaused = isPaused
}

// updateDbPauseSt updates the db with the given isPaused state.
func (s *syncService) updateDbPauseSt(ctx *context.T, tx store.Transaction, dbId wire.Id, isPaused bool) error {
	vlog.VI(3).Infof("sync: updateDbPauseSt: updating sync paused for db %v with value %t", dbId, isPaused)
	ss, err := getDbSyncState(ctx, tx)
	if err != nil {
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return err
		}
		ss = &DbSyncState{}
	}
	ss.IsPaused = isPaused
	return putDbSyncState(ctx, tx, ss)
}

func (s *syncService) getOrCreateSyncStateInternal(dbId wire.Id) *dbSyncStateInMem {
	ds, ok := s.syncState[dbId]
	if !ok {
		ds = &dbSyncStateInMem{
			data: &localGenInfoInMem{gen: 1},
			sgs:  make(map[string]*localGenInfoInMem),
		}
		s.syncState[dbId] = ds
	}
	return s.syncState[dbId]
}

////////////////////////////////////////////////////////////
// Low-level utility functions to access sync state.

// putDbSyncState persists the sync state object for a given Database.
func putDbSyncState(ctx *context.T, tx store.Transaction, ds *DbSyncState) error {
	return store.Put(ctx, tx, dbssKey, ds)
}

// getDbSyncState retrieves the sync state object for a given Database.
func getDbSyncState(ctx *context.T, st store.StoreReader) (*DbSyncState, error) {
	var ds DbSyncState
	if err := store.Get(ctx, st, dbssKey, &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

////////////////////////////////////////////////////////////
// Low-level utility functions to access log records.

// logRecsPerDeviceScanPrefix returns the prefix used to scan log records for a particular device.
func logRecsPerDeviceScanPrefix(pfx string, id uint64) string {
	return common.JoinKeyParts(logPrefix, pfx, fmt.Sprintf("%d", id))
}

// logRecKey returns the key used to access a specific log record.
func logRecKey(pfx string, id, gen uint64) string {
	return common.JoinKeyParts(logPrefix, pfx, fmt.Sprintf("%d", id), fmt.Sprintf("%016x", gen))
}

// hasLogRec returns true if the log record for (devid, gen) exists.
func hasLogRec(st store.StoreReader, pfx string, id, gen uint64) (bool, error) {
	return store.Exists(nil, st, logRecKey(pfx, id, gen))
}

// putLogRec stores the log record.
func putLogRec(ctx *context.T, tx store.Transaction, pfx string, rec *LocalLogRec) error {
	return store.Put(ctx, tx, logRecKey(pfx, rec.Metadata.Id, rec.Metadata.Gen), rec)
}

// getLogRec retrieves the log record for a given (devid, gen).
func getLogRec(ctx *context.T, st store.StoreReader, pfx string, id, gen uint64) (*LocalLogRec, error) {
	return getLogRecByKey(ctx, st, logRecKey(pfx, id, gen))
}

// getLogRecByKey retrieves the log record for a given log record key.
func getLogRecByKey(ctx *context.T, st store.StoreReader, key string) (*LocalLogRec, error) {
	var rec LocalLogRec
	if err := store.Get(ctx, st, key, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// delLogRec deletes the log record for a given (devid, gen).
func delLogRec(ctx *context.T, tx store.Transaction, pfx string, id, gen uint64) error {
	return store.Delete(ctx, tx, logRecKey(pfx, id, gen))
}
