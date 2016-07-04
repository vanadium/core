// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Syncbase Watcher is a goroutine that listens to local Database updates from
// applications and modifies sync metadata (e.g. DAG and local log records).
// The coupling between Syncbase storage and sync is loose, via asynchronous
// listening by the Watcher, to unblock the application operations as soon as
// possible, and offload the sync metadata update to the Watcher.  When the
// application mutates objects in a Database, additional entries are written
// to a log queue, persisted in the same Database.  This queue is read by the
// sync Watcher to learn of the changes.

import (
	"fmt"
	"strings"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
	sbwatchable "v.io/x/ref/services/syncbase/watchable"
)

var (
	// watchPrefixes is an in-memory cache of syncgroup prefixes for each
	// database.  It is filled at startup from persisted syncgroup data
	// and updated at runtime when syncgroups are joined or left.  It is
	// not guarded by a mutex because only the watcher goroutine uses it
	// beyond the startup phase (before any sync goroutines are started).
	// The map keys are the database ids (globally unique).
	watchPrefixes = make(map[wire.Id]sgPrefixes)
)

// sgPrefixes tracks syncgroup prefixes being synced in a database and their
// syncgroups.
type sgPrefixes map[string]sgSet

// StartStoreWatcher starts a Sync goroutine to watch the database store for
// updates and process them into the Sync subsystem.  This function is called
// when a database is created or reopened during service restart, thus spawning
// a Sync watcher for each database.
func (sd *syncDatabase) StartStoreWatcher(ctx *context.T) {
	s := sd.sync.(*syncService)
	dbId, st := sd.db.Id(), sd.db.St()

	s.pending.Add(1)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		s.watchStore(ctx, dbId, st)
		cancel()
		s.pending.Done()
	}()
}

// watchStore processes updates obtained by watching the store.  This is the
// sync watcher goroutine that learns about store updates asynchronously by
// reading log records that track object mutation histories in each database.
// For each batch mutation, the watcher updates the sync DAG and log records.
// When an application makes a single non-transactional put, it is represented
// as a batch of one log record. Thus the watcher only deals with batches.
func (s *syncService) watchStore(ctx *context.T, dbId wire.Id, st *watchable.Store) {
	vlog.VI(1).Infof("sync: watchStore: DB %v: start watching updates", dbId)

	updatesChan, cancel := watchable.WatchUpdates(st)
	defer cancel()

	moreWork := true
	for moreWork && !s.isClosed() {
		if s.processDatabase(ctx, dbId, st) {
			vlog.VI(2).Infof("sync: watchStore: DB %v: had updates", dbId)
		} else {
			vlog.VI(2).Infof("sync: watchStore: DB %v: idle, wait for updates", dbId)
			select {
			case _, moreWork = <-updatesChan:

			case <-s.closed:
				moreWork = false
			}
		}
	}

	vlog.VI(1).Infof("sync: watchStore: DB %v: channel closed, stop watching and exit", dbId)
}

// processDatabase fetches from the given database at most one new batch update
// (transaction) and processes it.  A batch is stored as a contiguous set of log
// records ending with one record having the "continued" flag set to false.  The
// call returns true if a new batch update was processed.
func (s *syncService) processDatabase(ctx *context.T, dbId wire.Id, st store.Store) bool {
	vlog.VI(2).Infof("sync: processDatabase: begin: %v", dbId)
	defer vlog.VI(2).Infof("sync: processDatabase: end: %v", dbId)

	resMark, err := getResMark(ctx, st)
	if err != nil {
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			vlog.Errorf("sync: processDatabase: %v: cannot get resMark: %v", dbId, err)
			return false
		}
		resMark = watchable.MakeResumeMarker(0)
	}

	// Initialize Database sync state if needed.
	s.initSyncStateInMem(ctx, dbId, "")

	// Get a batch of watch log entries, if any, after this resume marker.
	logs, nextResmark, err := watchable.ReadBatchFromLog(st, resMark)
	if err != nil {
		// An error here (scan stream cancelled) is possible when the watcher is in
		// the middle of processing a database when it is destroyed. Hence, we just
		// ignore this database and proceed.
		vlog.Errorf("sync: processDatabase: %v: cannot get watch log batch: %v", dbId, verror.DebugString(err))
		return false
	} else if logs == nil {
		return false
	}

	if err = s.processWatchLogBatch(ctx, dbId, st, logs, nextResmark); err != nil {
		// TODO(rdaoud): quarantine this database.
		return false
	}

	// The requirement for pause is that any write after the pause must not
	// be synced until sync is resumed. The same for resume is that every
	// write before resume must be seen and added to sync data structures
	// before sync resumes.
	//
	// Hence, at the end of processing a batch, the watcher checks if sync is
	// paused/resumed. If it is paused, it does not cut any more generations for
	// future batches. If sync is resumed, it cuts a new generation. Cutting a
	// generation freezes the most recent batch of local changes. This frozen
	// state is used by the responder when responding to GetDeltas RPC.
	if s.isDbSyncable(ctx, dbId) {
		// The database is online. Cut a gen.
		if err := s.checkptLocalGen(ctx, dbId, nil); err != nil {
			vlog.Errorf("sync: processDatabase: %v: cannot cut a generation: %v", dbId, verror.DebugString(err))
			return false
		}
	} else {
		vlog.VI(1).Infof("sync: processDatabase: %v database not allowed to sync, skipping cutting a gen", dbId)
	}
	return true
}

// processWatchLogBatch parses the given batch of watch log records, updates the
// watchable syncgroup prefixes, uses the prefixes to filter the batch to the
// subset of syncable records, and transactionally applies these updates to the
// sync metadata (DAG & log records) and updates the watch resume marker.
func (s *syncService) processWatchLogBatch(ctx *context.T, dbId wire.Id, st store.Store, logs []*watchable.LogEntry, resMark watch.ResumeMarker) error {
	if len(logs) == 0 {
		return nil
	}

	if processDbStateChangeLogRecord(ctx, s, st, dbId, logs[0], resMark) {
		// A batch containing DbStateChange will not have any more records.
		// This batch is done processing.
		return nil
	}

	// If the first log entry is a syncgroup prefix operation, then this is
	// a syncgroup snapshot and not an application batch.  In this case,
	// handle the syncgroup prefix changes by updating the watch prefixes
	// and exclude the first entry from the batch.  Also inform the batch
	// processing below to not assign it a batch ID in the DAG.
	sgop, err := processSyncgroupLogRecord(dbId, logs)
	if err != nil {
		vlog.Errorf("sync: processWatchLogBatch: %v: bad log entry: %v", dbId, err)
		return err
	}

	appBatch := true
	if sgop != nil {
		appBatch = false
	}

	// Preprocess the log entries before calling RunInTransaction():
	// - Filter out log entries made by sync (echo suppression).
	// - Convert the log entries to initial sync log records that contain
	//   the metadata from the log entries.  These records will be later
	//   augmented with DAG information inside RunInTransaction().
	// - Determine which entries are syncable (included in some syncgroup)
	//   and ignore the private ones (not currently shared in a syncgroup).
	// - Extract blob refs from syncable values and update blob metadata.
	//
	// These steps are idempotent.  If Syncbase crashes before completing
	// the transaction below (which updates the resume marker), these steps
	// are re-executed.
	totalCount := uint64(len(logs))
	batch := make([]*LocalLogRec, 0, totalCount)

	for _, e := range logs {
		if !e.FromSync {
			if rec, err := convertLogRecord(ctx, s.id, e); err != nil {
				vlog.Errorf("sync: processWatchLogBatch: %v: bad entry: %v: %v", dbId, *e, err)
				return err
			} else if rec != nil {
				if syncable(dbId, rec.Metadata.ObjId) {
					batch = append(batch, rec)
				}
			}
		}
	}

	vlog.VI(3).Infof("sync: processWatchLogBatch: %v: sg snap %t, syncable %d, total %d", dbId, !appBatch, len(batch), totalCount)

	if err := s.processWatchBlobRefs(ctx, dbId, st, batch); err != nil {
		// There may be an error here if the database is recently
		// destroyed.  Ignore the error and continue to another database.
		vlog.Errorf("sync: processWatchLogBatch: %v: watcher cannot process blob refs: %v", dbId, err)
		return nil
	}

	// Transactional processing of the batch: Fixup syncable log records to
	// augment them with DAG information.
	err = store.RunInTransaction(st, func(tx store.Transaction) error {
		txBatch, err := fixupLogRecordBatch(ctx, tx, batch, appBatch)
		if err != nil {
			return err
		}

		if err := s.processBatch(ctx, dbId, txBatch, appBatch, totalCount, tx); err != nil {
			return err
		}

		if !appBatch {
			if err := setSyncgroupWatchable(ctx, tx, sgop); err != nil {
				return err
			}
		}

		return setResMark(ctx, tx, resMark)
	})

	if err != nil {
		// There may be an error here if the database is recently
		// destroyed. Ignore the error and continue to another database.
		// TODO(rdaoud): quarantine this database for other errors.
		vlog.Errorf("sync: processWatchLogBatch: %v: watcher cannot process batch: %v", dbId, err)
	}
	return nil
}

// processBatch applies a single batch of changes (object mutations) received
// from watching a particular Database.
func (s *syncService) processBatch(ctx *context.T, dbId wire.Id, batch []*LocalLogRec, appBatch bool, totalCount uint64, tx store.Transaction) error {
	count := uint64(len(batch))
	if count == 0 {
		return nil
	}

	// If an application batch has more than one mutation, start a batch for it.
	batchId := NoBatchId
	if appBatch && totalCount > 1 {
		batchId = s.startBatch(ctx, tx, batchId)
		if batchId == NoBatchId {
			return verror.New(verror.ErrInternal, ctx, "failed to generate batch ID")
		}
	}

	gen, pos := s.reserveGenAndPosInDbLog(ctx, dbId, "", count)

	vlog.VI(3).Infof("sync: processBatch: %v: len %d, total %d, btid %x, gen %d, pos %d", dbId, count, totalCount, batchId, gen, pos)

	for _, rec := range batch {
		// Update the log record. Portions of the record Metadata must
		// already be filled.
		rec.Metadata.Gen = gen

		rec.Metadata.BatchId = batchId
		rec.Metadata.BatchCount = totalCount

		rec.Pos = pos

		gen++
		pos++

		if err := s.processLocalLogRec(ctx, tx, rec); err != nil {
			return verror.New(verror.ErrInternal, ctx, err)
		}
	}

	// End the batch if any.
	if batchId != NoBatchId {
		if err := s.endBatch(ctx, tx, batchId, totalCount); err != nil {
			return err
		}
	}

	return nil
}

// processLocalLogRec processes a local log record by adding to the Database and
// suitably updating the DAG metadata.
func (s *syncService) processLocalLogRec(ctx *context.T, tx store.Transaction, rec *LocalLogRec) error {
	// Insert the new log record into the log.
	if err := putLogRec(ctx, tx, logDataPrefix, rec); err != nil {
		return err
	}

	m := &rec.Metadata
	logKey := logRecKey(logDataPrefix, m.Id, m.Gen)

	// Insert the new log record into dag.
	if err := s.addNode(ctx, tx, m.ObjId, m.CurVers, logKey, m.Delete, m.Parents, m.BatchId, nil); err != nil {
		return err
	}

	// Move the head.
	return moveHead(ctx, tx, m.ObjId, m.CurVers)
}

// processWatchBlobRefs extracts blob refs from the data values of the updates
// received in the watch batch and updates the blob-to-syncgroup metadata.
func (s *syncService) processWatchBlobRefs(ctx *context.T, dbId wire.Id, st store.Store, batch []*LocalLogRec) error {
	if len(batch) == 0 {
		return nil
	}

	sgPfxs := watchPrefixes[dbId]
	if len(sgPfxs) == 0 {
		return verror.New(verror.ErrInternal, ctx, "processWatchBlobRefs: no sg prefixes in db", dbId)
	}

	for _, rec := range batch {
		m := &rec.Metadata
		if m.Delete {
			continue
		}

		buf, err := watchable.GetAtVersion(ctx, st, []byte(m.ObjId), nil, []byte(m.CurVers))
		if err != nil {
			return err
		}
		var rawValue *vom.RawBytes
		if err = vom.Decode(buf, &rawValue); err != nil {
			return err
		}

		if err = s.processBlobRefs(ctx, dbId, st, s.name, true, sgPfxs, nil, m, rawValue); err != nil {
			return err
		}
	}
	return nil
}

// addWatchPrefixSyncgroup adds a syncgroup prefix-to-ID mapping for an app
// database in the watch prefix cache.
func addWatchPrefixSyncgroup(dbId wire.Id, prefix string, gid interfaces.GroupId) {
	if pfxs := watchPrefixes[dbId]; pfxs != nil {
		if sgs := pfxs[prefix]; sgs != nil {
			sgs[gid] = struct{}{}
		} else {
			pfxs[prefix] = sgSet{gid: struct{}{}}
		}
	} else {
		watchPrefixes[dbId] = sgPrefixes{prefix: sgSet{gid: struct{}{}}}
	}
}

// rmWatchPrefixSyncgroup removes a syncgroup prefix-to-ID mapping for an app
// database in the watch prefix cache.
func rmWatchPrefixSyncgroup(dbId wire.Id, prefix string, gid interfaces.GroupId) {
	if pfxs := watchPrefixes[dbId]; pfxs != nil {
		if sgs := pfxs[prefix]; sgs != nil {
			delete(sgs, gid)
			if len(sgs) == 0 {
				delete(pfxs, prefix)
				if len(pfxs) == 0 {
					delete(watchPrefixes, dbId)
				}
			}
		}
	}
}

// setSyncgroupWatchable sets the local watchable state of the syncgroup.
func setSyncgroupWatchable(ctx *context.T, tx store.Transaction, sgop *sbwatchable.SyncgroupOp) error {
	state, err := getSGIdEntry(ctx, tx, sgop.SgId)
	if err != nil {
		return err
	}
	state.Watched = !sgop.Remove

	return setSGIdEntry(ctx, tx, sgop.SgId, state)
}

// convertLogRecord converts a store log entry to an initial sync log record
// that contains metadata from the store log entry and the Syncbase ID, but no
// information from the DAG which is filled later within the scope of a store
// transaction.  For a delete, it generates a new object version because the
// store does not version a deletion.
// TODO(rdaoud): change Syncbase to store and version a deleted object to
// simplify the store-to-sync interaction.  A deleted key would still have a
// version and its value entry would encode the "deleted" flag, either in the
// key or probably in a value wrapper that would contain other metadata.
func convertLogRecord(ctx *context.T, syncId uint64, logEnt *watchable.LogEntry) (*LocalLogRec, error) {
	var op interface{}
	if err := logEnt.Op.ToValue(&op); err != nil {
		return nil, err
	}

	var key, version string
	deleted := false
	timestamp := logEnt.CommitTimestamp

	switch op := op.(type) {
	case *watchable.PutOp:
		key, version = string(op.Key), string(op.Version)

	case *sbwatchable.SyncSnapshotOp:
		key, version = string(op.Key), string(op.Version)

	case *watchable.DeleteOp:
		key, version, deleted = string(op.Key), string(watchable.NewVersion()), true

	case *watchable.GetOp:
		// TODO(rdaoud): save read-set in sync.
		return nil, nil

	case *watchable.ScanOp:
		// TODO(rdaoud): save scan-set in sync.
		return nil, nil

	case *sbwatchable.SyncgroupOp:
		// We can ignore the syncgroup op.
		return nil, nil

	default:
		return nil, verror.New(verror.ErrInternal, ctx, "cannot convert unknown watch log entry")
	}

	rec := &LocalLogRec{
		Metadata: interfaces.LogRecMetadata{
			ObjId:   key,
			CurVers: version,
			Delete:  deleted,
			UpdTime: unixNanoToTime(timestamp),
			Id:      syncId,
			RecType: interfaces.NodeRec,
		},
	}
	return rec, nil
}

// fixupLogRecordBatch updates the sync log records in a batch with information
// retrieved from the DAG within the scope of a store transaction and returns
// an updated batch.  This allows the transaction to track these data fetches in
// its read-set, which is required for the proper handling of optimistic locking
// between store transactions.  If the input batch is not an application batch
// (e.g. one generated by a syncgroup snapshot), the returned batch may include
// fewer log records because duplicates were filtered out.
func fixupLogRecordBatch(ctx *context.T, tx store.Transaction, batch []*LocalLogRec, appBatch bool) ([]*LocalLogRec, error) {
	newBatch := make([]*LocalLogRec, 0, len(batch))

	for _, rec := range batch {
		m := &rec.Metadata

		// Check if this object version already exists in the DAG.  This
		// is only allowed for non-application batches and can happen in
		// the cases of syncgroup initialization snapshots for nested or
		// peer syncgroups creating duplicate log records that are
		// skipped here.  Otherwise this is an error.
		if ok, err := hasNode(ctx, tx, m.ObjId, m.CurVers); err != nil {
			return nil, err
		} else if ok {
			if appBatch {
				return nil, verror.New(verror.ErrInternal, ctx, "duplicate log record", m.ObjId, m.CurVers)
			}
		} else {
			// Set the current DAG head as the parent of this update.
			m.Parents = nil
			if head, err := getHead(ctx, tx, m.ObjId); err == nil {
				m.Parents = []string{head}
			} else if m.Delete || (verror.ErrorID(err) != verror.ErrNoExist.ID) {
				return nil, verror.New(verror.ErrInternal, ctx, "cannot getHead to fixup log record", m.ObjId)
			}

			newBatch = append(newBatch, rec)
		}
	}

	return newBatch, nil
}

// processDbStateChangeLogRecord checks if the log entry is a
// DbStateChangeRequest and if so, it executes the state change request
// appropriately.
// TODO(razvanm): change the return type to error.
func processDbStateChangeLogRecord(ctx *context.T, s *syncService, st store.Store, dbId wire.Id, logEnt *watchable.LogEntry, resMark watch.ResumeMarker) bool {
	var op interface{}
	if err := logEnt.Op.ToValue(&op); err != nil {
		vlog.Fatalf("sync: processDbStateChangeLogRecord: %v: bad VOM: %v", dbId, err)
	}
	switch op := op.(type) {
	case *sbwatchable.DbStateChangeRequestOp:
		dbStateChangeRequest := op
		vlog.VI(1).Infof("sync: processDbStateChangeLogRecord: found a dbState change log record with state %#v", dbStateChangeRequest)
		isPaused := false
		if err := store.RunInTransaction(st, func(tx store.Transaction) error {
			switch dbStateChangeRequest.RequestType {
			case sbwatchable.StateChangePauseSync:
				vlog.VI(1).Infof("sync: processDbStateChangeLogRecord: PauseSync request found. Pausing sync.")
				isPaused = true
			case sbwatchable.StateChangeResumeSync:
				vlog.VI(1).Infof("sync: processDbStateChangeLogRecord: ResumeSync request found. Resuming sync.")
				isPaused = false
			default:
				return fmt.Errorf("Unexpected DbStateChangeRequest found: %#v", dbStateChangeRequest)
			}
			// Update isPaused state in db.
			if err := s.updateDbPauseSt(ctx, tx, dbId, isPaused); err != nil {
				return err
			}
			return setResMark(ctx, tx, resMark)
		}); err != nil {
			// TODO(rdaoud): don't crash, quarantine this database.
			vlog.Fatalf("sync: processDbStateChangeLogRecord: %v: watcher failed to reset dbState bits: %v", dbId, err)
		}
		// Update isPaused state in cache.
		s.updateInMemoryPauseSt(ctx, dbId, isPaused)
		return true

	default:
		return false
	}
}

// processSyncgroupLogRecord checks if the log entries contain a syncgroup
// update and, if they do, updates the watch prefixes for the database and
// returns a syncgroup operation. Otherwise it returns a nil operation with no
// other changes.
func processSyncgroupLogRecord(dbId wire.Id, logs []*watchable.LogEntry) (*sbwatchable.SyncgroupOp, error) {
	for _, logEnt := range logs {
		var op interface{}

		if err := logEnt.Op.ToValue(&op); err != nil {
			return nil, err
		}

		switch op := op.(type) {
		case *sbwatchable.SyncgroupOp:
			gid, remove := op.SgId, op.Remove
			for _, prefix := range op.Prefixes {
				if remove {
					rmWatchPrefixSyncgroup(dbId, prefix, gid)
				} else {
					addWatchPrefixSyncgroup(dbId, prefix, gid)
				}
			}
			vlog.VI(3).Infof("sync: processSyncgroupLogRecord: %v: gid %s, remove %t, prefixes: %q", dbId, gid, remove, op.Prefixes)
			return op, nil
		}
	}
	return nil, nil
}

// syncable returns true if the given key falls within the scope of a syncgroup
// prefix for the given database, and thus should be synced.
func syncable(dbId wire.Id, key string) bool {
	// The key starts with one of the store's reserved prefixes for managed
	// namespaces (e.g. "r" for row, "c" for collection perms). Remove that
	// prefix before comparing it with the syncgroup prefixes.
	key = common.StripFirstKeyPartOrDie(key)

	for prefix := range watchPrefixes[dbId] {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// resMarkKey returns the key used to access the watcher resume marker.
func resMarkKey() string {
	return common.JoinKeyParts(common.SyncPrefix, "w", "rm")
}

// setResMark stores the watcher resume marker for a database.
func setResMark(ctx *context.T, tx store.Transaction, resMark watch.ResumeMarker) error {
	return store.Put(ctx, tx, resMarkKey(), resMark)
}

// getResMark retrieves the watcher resume marker for a database.
func getResMark(ctx *context.T, st store.StoreReader) (watch.ResumeMarker, error) {
	var resMark watch.ResumeMarker
	key := resMarkKey()
	if err := store.Get(ctx, st, key, &resMark); err != nil {
		return nil, err
	}
	return resMark, nil
}
