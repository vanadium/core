// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"math/rand"
	"path/filepath"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/query/engine"
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/syncql"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/services/syncbase/store"
	storeutil "v.io/x/ref/services/syncbase/store/util"
	"v.io/x/ref/services/syncbase/store/watchable"
	"v.io/x/ref/services/syncbase/vsync"
	sbwatchable "v.io/x/ref/services/syncbase/watchable"
)

// database is a per-database singleton (i.e. not per-request) that handles
// Database RPCs.
// Note: If a database does not exist at the time of a database RPC, the
// dispatcher creates a short-lived database object to service that particular
// request.
type database struct {
	id wire.Id
	s  *service
	// The fields below are initialized iff this database exists.
	exists bool
	// TODO(sadovsky): Make st point to a store.Store wrapper that handles paging,
	// and do not actually open the store in NewDatabase.
	st *watchable.Store // stores all data for a single database

	// Active snapshots and transactions corresponding to client batches.
	// TODO(sadovsky): Add timeouts and GC.
	mu  sync.Mutex // protects the fields below
	sns map[uint64]store.Snapshot
	txs map[uint64]*transactionState

	// Active ConflictResolver connection from the app to this database.
	// NOTE: For now, we assume there's only one open conflict resolution stream
	// per database (typically, from the app that owns the database).
	crStream wire.ConflictManagerStartConflictResolverServerCall
	// Mutex lock to protect concurrent read/write of crStream pointer
	crMu sync.Mutex
}

var (
	_ wire.DatabaseServerMethods = (*database)(nil)
	_ interfaces.Database        = (*database)(nil)
)

// DatabaseOptions configures a database.
type DatabaseOptions struct {
	// Database-level permissions.
	Perms access.Permissions
	// Root dir for data storage. This path is relative from the service's RootDir.
	RootDir string
	// Storage engine to use.
	Engine string
}

type permissionState struct {
	dataChanged  bool
	permsChanged bool
	initialPerms access.Permissions
	finalPerms   access.Permissions
}

type transactionState struct {
	tx           *watchable.Transaction
	permsChanges map[wire.Id]*permissionState
}

// Keeps track that this collection had a mutation and the permissions at the time. If no
// permissions are yet known for the collection then remember the current permissions as the
// initial permissions of the collection.
// When the transaction is committed we will know to validate this collection's permissions.
func (ts *transactionState) MarkDataChanged(collectionId wire.Id, perms access.Permissions) {
	state := ts.permsState(collectionId)
	state.dataChanged = true
	if state.initialPerms == nil {
		state.initialPerms = perms
	}
	state.finalPerms = perms
}

// Keeps track that the permissions were changed on this collection and the before and after
// permissions. If no permissions are yet known for the collection then remember the current
// permissions as the initial permissions of the collection.
func (ts *transactionState) MarkPermsChanged(collectionId wire.Id, permsBefore access.Permissions, permsAfter access.Permissions) {
	state := ts.permsState(collectionId)
	state.permsChanged = true
	if state.initialPerms == nil {
		state.initialPerms = permsBefore
	}
	state.finalPerms = permsAfter
}

// Resets all tracked changes to the collection. Used on collection destroy. Since destroy
// cannot happen on a synced collection, the destroy and any updates before it will not be
// seen remotely, so validation must start from the implicit permissions in case the
// collection is created again. This also allows destroy to not require both write and
// admin permissions.
func (ts *transactionState) ResetCollectionChanges(collectionId wire.Id) {
	delete(ts.permsChanges, collectionId)
}

// validatePermissionChanges performs an auth check on each collection that has a data change or
// permission change and returns false if any of the auth checks fail.
// TODO(ivanpi): This check should be done against signing blessings at signing time, in
// both batch and non-batch cases.
func (ts *transactionState) validatePermissionChanges(ctx *context.T, securityCall security.Call) bool {
	for _, collectionState := range ts.permsChanges {
		// This collection was modified, make sure that the write acl is either present at
		// the end or that it had the write acl to begin with. This way we can be sure that
		// a mutation didn't take place when it appeared that there was no write acl before
		// and after the transaction.
		if collectionState.dataChanged {
			before := hasPermission(ctx, securityCall, collectionState.initialPerms, access.Write)
			after := hasPermission(ctx, securityCall, collectionState.finalPerms, access.Write)
			if !after && !before {
				return false
			}
		}

		// The permissions were changed on the collection, make sure that the admin acl is
		// present at the beginning.
		if collectionState.permsChanged {
			if !hasPermission(ctx, securityCall, collectionState.initialPerms, access.Admin) {
				return false
			}
		}
	}
	return true
}

func (ts *transactionState) permsState(collectionId wire.Id) *permissionState {
	if ts.permsChanges == nil {
		ts.permsChanges = make(map[wire.Id]*permissionState)
	}
	state, ok := ts.permsChanges[collectionId]
	if !ok {
		state = &permissionState{}
		ts.permsChanges[collectionId] = state
	}
	return state
}

// hasPermission returns true if the caller is authorized for the specific tag based on the
// passed in perms.
func hasPermission(ctx *context.T, securityCall security.Call, perms access.Permissions, tag access.Tag) bool {
	permForTag, ok := perms[string(tag)]
	// Authorize returns either an error or nil, so nil means the caller is authorized.
	return ok && permForTag.Authorize(ctx, securityCall) == nil
}

// openDatabase opens a database and returns a *database for it. Designed for
// use from within newDatabase and newService.
func openDatabase(ctx *context.T, s *service, id wire.Id, opts DatabaseOptions, openOpts storeutil.OpenOptions) (*database, error) {
	// DatabaseOption's RootDir is relative to the service's RootDir (but for backwards compatibility
	// s.absRootDir will return any absolute paths as-is).
	p := s.absRootDir(filepath.Join(opts.RootDir, opts.Engine))
	st, err := storeutil.OpenStore(opts.Engine, p, openOpts)
	if err != nil {
		return nil, err
	}
	wst, err := watchable.Wrap(st, s.vclock, &watchable.Options{
		// TODO(ivanpi): Since ManagedPrefixes control what gets synced, they
		// should be moved to a more visible place (e.g. constants). Also consider
		// decoupling managed and synced prefixes.
		ManagedPrefixes: []string{common.CollectionPermsPrefix, common.RowPrefix},
	})
	if err != nil {
		return nil, err
	}
	return &database{
		id:     id,
		s:      s,
		exists: true,
		st:     wst,
		sns:    make(map[uint64]store.Snapshot),
		txs:    make(map[uint64]*transactionState),
	}, nil
}

// newDatabase creates a new database instance and returns it.
// Designed for use from within service.createDatabase.
func newDatabase(ctx *context.T, s *service, id wire.Id, metadata *wire.SchemaMetadata, opts DatabaseOptions) (*database, error) {
	if opts.Perms == nil {
		return nil, verror.New(verror.ErrInternal, ctx, "perms must be specified")
	}
	d, err := openDatabase(ctx, s, id, opts, storeutil.OpenOptions{CreateIfMissing: true, ErrorIfExists: true})
	if err != nil {
		return nil, err
	}
	data := &DatabaseData{
		Perms:          opts.Perms,
		SchemaMetadata: metadata,
	}
	if err := store.Put(ctx, d.st, d.stKey(), data); err != nil {
		return nil, err
	}

	// Start a Sync watcher on this newly created database store.
	vsync.NewSyncDatabase(d).StartStoreWatcher(ctx)

	return d, nil
}

////////////////////////////////////////
// RPC methods

func (d *database) Create(ctx *context.T, call rpc.ServerCall, metadata *wire.SchemaMetadata, perms access.Permissions) error {
	if d.exists {
		return verror.New(verror.ErrExist, ctx, d.id)
	}
	// This database does not yet exist; d is just an ephemeral handle that holds
	// {id wire.Id, s *service}. d.s.createDatabase will create a new database
	// handle and store it in d.s.dbs[d.id].
	return d.s.createDatabase(ctx, call, d.id, perms, metadata)
}

func (d *database) Destroy(ctx *context.T, call rpc.ServerCall) error {
	return d.s.destroyDatabase(ctx, call, d.id)
}

func (d *database) Exists(ctx *context.T, call rpc.ServerCall) (bool, error) {
	if !d.exists {
		return false, nil
	}
	return util.ErrorToExists(util.GetWithAuth(ctx, call, d.st, d.stKey(), &DatabaseData{}))
}

var rng *rand.Rand = rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

func (d *database) BeginBatch(ctx *context.T, call rpc.ServerCall, opts wire.BatchOptions) (wire.BatchHandle, error) {
	if !d.exists {
		return "", verror.New(verror.ErrNoExist, ctx, d.id)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var id uint64
	var batchType common.BatchType
	for {
		id = uint64(rng.Int63())
		if opts.ReadOnly {
			if _, ok := d.sns[id]; !ok {
				d.sns[id] = d.st.NewSnapshot()
				batchType = common.BatchTypeSn
				break
			}
		} else {
			if _, ok := d.txs[id]; !ok {
				d.txs[id] = &transactionState{tx: d.st.NewWatchableTransaction()}
				batchType = common.BatchTypeTx
				break
			}
		}
	}
	return common.JoinBatchHandle(batchType, id), nil
}

func (d *database) Commit(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	if bh == "" {
		return wire.NewErrNotBoundToBatch(ctx)
	}
	_, ts, batchId, err := d.batchLookupInternal(ctx, bh)
	if err != nil {
		return err
	}
	if ts == nil {
		return wire.NewErrReadOnlyBatch(ctx)
	}
	if !ts.validatePermissionChanges(ctx, call.Security()) {
		return wire.NewErrInvalidPermissionsChange(ctx)
	}
	if err = ts.tx.Commit(); err == nil {
		d.mu.Lock()
		delete(d.txs, batchId)
		d.mu.Unlock()
	}
	// TODO(ivanpi): Best effort abort if commit fails? Watchable Commit can fail
	// before the underlying snapshot is aborted.
	if verror.ErrorID(err) == store.ErrConcurrentTransaction.ID {
		return verror.New(wire.ErrConcurrentBatch, ctx, err)
	}
	return err
}

func (d *database) Abort(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	if bh == "" {
		return wire.NewErrNotBoundToBatch(ctx)
	}
	sn, ts, batchId, err := d.batchLookupInternal(ctx, bh)
	if err != nil {
		return err
	}
	if ts != nil {
		d.mu.Lock()
		delete(d.txs, batchId)
		d.mu.Unlock()
		// TODO(ivanpi): If tx.Abort fails, retry later?
		return ts.tx.Abort()
	} else {
		d.mu.Lock()
		delete(d.sns, batchId)
		d.mu.Unlock()
		// TODO(ivanpi): If sn.Abort fails, retry later?
		return sn.Abort()
	}
}

func (d *database) Exec(ctx *context.T, call wire.DatabaseExecServerCall, bh wire.BatchHandle, q string, params []*vom.RawBytes) error {
	// RunInTransaction() cannot be used here because we may or may not be
	// creating a transaction. qe.Exec must be called and the statement must be
	// parsed before we know if a snapshot or a transaction should be created. To
	// duplicate the semantics of RunInTransaction, if we are not inside a batch,
	// we attempt the Exec up to 100 times and retry on ErrConcurrentTransaction.
	// TODO(ivanpi): Refactor query parsing into a separate step, simplify request
	// handling. Consider separate Query and Exec methods.
	maxAttempts := 100
	attempt := 0
	for {
		err := d.execInternal(ctx, call, bh, q, params)
		if bh != "" || attempt >= maxAttempts || verror.ErrorID(err) != store.ErrConcurrentTransaction.ID {
			return err
		}
		attempt++
	}
}

func (d *database) execInternal(ctx *context.T, call wire.DatabaseExecServerCall, bh wire.BatchHandle, q string, params []*vom.RawBytes) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	impl := func() error {
		db := &queryDb{
			ctx:  ctx,
			call: call,
			d:    d,
			bh:   bh,
			sntx: nil, // Filled in later with existing or created sn/tx.
			ts:   nil, // Only filled in if a new batch was created.
		}
		st, err := engine.Create(db).PrepareStatement(q)
		if err != nil {
			return execCommitOrAbort(db, err)
		}
		headers, rs, err := st.Exec(params...)
		if err != nil {
			return execCommitOrAbort(db, err)
		}
		if rs.Err() != nil {
			return execCommitOrAbort(db, err)
		}
		sender := call.SendStream()
		// Push the headers first -- the client will retrieve them and return
		// them separately from the results.
		var resultHeaders []*vom.RawBytes
		for _, header := range headers {
			resultHeaders = append(resultHeaders, vom.RawBytesOf(header))
		}
		sender.Send(resultHeaders)
		for rs.Advance() {
			result := rs.Result()
			if err := sender.Send(result); err != nil {
				rs.Cancel()
				return execCommitOrAbort(db, err)
			}
		}
		return execCommitOrAbort(db, rs.Err())
	}
	return impl()
}

func execCommitOrAbort(qdb *queryDb, err error) error {
	if qdb.bh != "" {
		return err // part of an enclosing sn/tx
	}
	if err != nil {
		if qdb.sntx != nil {
			qdb.sntx.Abort()
		}
		return err
	} else { // err is nil
		if qdb.ts != nil {
			return qdb.ts.tx.Commit()
		} else if qdb.sntx != nil {
			return qdb.sntx.Abort()
		}
		return nil
	}
}

func (d *database) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	return d.s.setDatabasePerms(ctx, call, d.id, perms, version)
}

func (d *database) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	if !d.exists {
		return nil, "", verror.New(verror.ErrNoExist, ctx, d.id)
	}
	data := &DatabaseData{}
	if err := util.GetWithAuth(ctx, call, d.st, d.stKey(), data); err != nil {
		return nil, "", err
	}
	return data.Perms, util.FormatVersion(data.Version), nil
}

func (d *database) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, matcher *glob.Element) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	impl := func(sntx store.SnapshotOrTransaction) error {
		// Check perms.
		if err := util.GetWithAuth(ctx, call, sntx, d.stKey(), &DatabaseData{}); err != nil {
			return err
		}
		return util.GlobChildren(ctx, call, matcher, sntx, common.CollectionPermsPrefix)
	}
	return store.RunWithSnapshot(d.st, impl)
}

// See comment in v.io/v23/services/syncbase/service.vdl for why we can't
// implement ListCollections using Glob.
func (d *database) ListCollections(ctx *context.T, call rpc.ServerCall, bh wire.BatchHandle) ([]wire.Id, error) {
	if !d.exists {
		return nil, verror.New(verror.ErrNoExist, ctx, d.id)
	}
	var res []wire.Id
	impl := func(sntx store.SnapshotOrTransaction) error {
		// Check perms.
		if err := util.GetWithAuth(ctx, call, sntx, d.stKey(), &DatabaseData{}); err != nil {
			return err
		}
		it := sntx.Scan(common.ScanPrefixArgs(common.CollectionPermsPrefix, ""))
		keyBytes := []byte{}
		for it.Advance() {
			keyBytes = it.Key(keyBytes)
			id, err := common.ParseCollectionPermsKey(string(keyBytes))
			if err != nil {
				it.Cancel()
				return verror.New(verror.ErrInternal, ctx, err)
			}
			res = append(res, id)
		}
		if err := it.Err(); err != nil {
			return err
		}
		return nil
	}
	if err := d.runWithExistingBatchOrNewSnapshot(ctx, bh, impl); err != nil {
		return nil, err
	}
	return res, nil
}

func (d *database) PauseSync(ctx *context.T, call rpc.ServerCall) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	return d.runInTransaction(func(ts *transactionState) error {
		return sbwatchable.AddDbStateChangeRequestOp(ctx, ts.tx, sbwatchable.StateChangePauseSync)
	})
}

func (d *database) ResumeSync(ctx *context.T, call rpc.ServerCall) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}
	return d.runInTransaction(func(ts *transactionState) error {
		return sbwatchable.AddDbStateChangeRequestOp(ctx, ts.tx, sbwatchable.StateChangeResumeSync)
	})
}

////////////////////////////////////////
// interfaces.Database methods

func (d *database) St() *watchable.Store {
	if !d.exists {
		vlog.Fatalf("database %v does not exist", d.id)
	}
	return d.st
}

func (d *database) Service() interfaces.Service {
	return d.s
}

func (d *database) CheckPermsInternal(ctx *context.T, call rpc.ServerCall, st store.StoreReader) error {
	if !d.exists {
		vlog.Fatalf("database %v does not exist", d.id)
	}
	return util.GetWithAuth(ctx, call, st, d.stKey(), &DatabaseData{})
}

func (d *database) Id() wire.Id {
	return d.id
}

func (d *database) CrConnectionStream() wire.ConflictManagerStartConflictResolverServerStream {
	d.crMu.Lock()
	defer d.crMu.Unlock()
	return d.crStream
}

func (d *database) ResetCrConnectionStream() {
	d.crMu.Lock()
	defer d.crMu.Unlock()
	// TODO(jlodhia): figure out a way for the connection to gracefully shutdown
	// so that the client can get an appropriate error msg.
	d.crStream = nil
}

////////////////////////////////////////
// query interface implementations

// queryDb implements ds.Database.
type queryDb struct {
	ctx  *context.T
	call rpc.ServerCall
	d    *database
	bh   wire.BatchHandle
	sntx store.SnapshotOrTransaction
	ts   *transactionState // Will only be set if in a transaction (else nil)
}

func (qdb *queryDb) GetContext() *context.T {
	return qdb.ctx
}

func (qdb *queryDb) GetTable(name string, writeAccessReq bool) (ds.Table, error) {
	// At this point, when the query package calls GetTable with the
	// writeAccessReq arg, we know whether or not we need a [writable] transaction
	// or a snapshot. If batchId is already set, there's nothing to do; but if
	// not, the writeAccessReq arg dictates whether a snapshot or a transaction is
	// should be created.
	// TODO(ivanpi): Allow passing in non-default user blessings.
	userBlessings, _ := security.RemoteBlessingNames(qdb.ctx, qdb.call.Security())
	_, user, err := pubutil.AppAndUserPatternFromBlessings(userBlessings...)
	if err != nil {
		return nil, err
	}
	qt := &queryTable{
		qdb: qdb,
		cReq: &collectionReq{
			id: wire.Id{Blessing: string(user), Name: name},
			d:  qdb.d,
		},
	}
	if qt.qdb.bh != "" {
		var err error
		if writeAccessReq {
			// We are in a batch (could be snapshot or transaction)
			// and Write access is required.  Attempt to get a
			// transaction from the request.
			qt.qdb.ts, err = qt.qdb.d.batchTransaction(qt.qdb.GetContext(), qt.qdb.bh)
			if err != nil {
				if verror.ErrorID(err) == wire.ErrReadOnlyBatch.ID {
					// We are in a snapshot batch, write access cannot be provided.
					// Return NotWritable.
					return nil, syncql.NewErrNotWritable(qt.qdb.GetContext(), pubutil.EncodeId(qt.cReq.id))
				}
				return nil, err
			}
			qt.qdb.sntx = qt.qdb.ts.tx
		} else {
			qt.qdb.sntx, err = qt.qdb.d.batchReader(qt.qdb.GetContext(), qt.qdb.bh)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Now that we know if write access is required, create a snapshot
		// or transaction.
		if !writeAccessReq {
			qt.qdb.sntx = qt.qdb.d.st.NewSnapshot()
		} else { // writeAccessReq
			qt.qdb.ts = &transactionState{tx: qt.qdb.d.st.NewWatchableTransaction()}
			qt.qdb.sntx = qt.qdb.ts.tx
		}
	}
	// Now that we have a collection, we need to check permissions.
	collectionPerms, err := qt.cReq.checkAccess(qdb.ctx, qdb.call, qdb.sntx)
	if err != nil {
		return nil, err
	}
	if writeAccessReq {
		qt.qdb.ts.MarkDataChanged(qt.cReq.id, collectionPerms)
	}
	return qt, nil
}

// queryTable implements ds.Table.
type queryTable struct {
	qdb  *queryDb
	cReq *collectionReq
}

func (t *queryTable) GetIndexFields() []ds.Index {
	// TODO(jkline): If and when secondary indexes are supported, they
	// would be supplied here.
	return []ds.Index{}
}

func (t *queryTable) Delete(k string) (bool, error) {
	// Create a rowReq and call delete.  Permissions will be checked.
	rowReq := &rowReq{
		key: k,
		c:   t.cReq,
	}
	if err := rowReq.delete(t.qdb.GetContext(), t.qdb.call, t.qdb.ts); err != nil {
		return false, err
	}
	return true, nil
}

func (t *queryTable) Scan(indexRanges ...ds.IndexRanges) (ds.KeyValueStream, error) {
	streams := []store.Stream{}
	// Syncbase does not currently support secondary indexes. As such, indexRanges
	// is guaranteed to be one in size as it will only specify the key ranges;
	// hence, indexRanges[0] below.
	for _, keyRange := range *indexRanges[0].StringRanges {
		// TODO(jkline): For now, acquire all of the streams at once to minimize the
		// race condition. Need a way to Scan multiple ranges at the same state of
		// uncommitted changes.
		streams = append(streams, t.qdb.sntx.Scan(common.ScanRangeArgs(common.JoinKeyParts(common.RowPrefix, t.cReq.stKeyPart()), keyRange.Start, keyRange.Limit)))
	}
	return &kvs{
		t:        t,
		curr:     0,
		validRow: false,
		it:       streams,
		err:      nil,
	}, nil
}

// kvs implements ds.KeyValueStream.
type kvs struct {
	t         *queryTable
	curr      int
	validRow  bool
	currKey   string
	currValue *vom.RawBytes
	it        []store.Stream // array of store.Streams
	err       error
}

func (s *kvs) Advance() bool {
	if s.err != nil {
		return false
	}
	for s.curr < len(s.it) {
		if s.it[s.curr].Advance() {
			// key
			keyBytes := s.it[s.curr].Key(nil)
			parts := common.SplitNKeyParts(string(keyBytes), 3)
			// TODO(rogulenko): Check access for the key.
			s.currKey = parts[2]
			// value
			valueBytes := s.it[s.curr].Value(nil)
			var currValue *vom.RawBytes
			if err := vom.Decode(valueBytes, &currValue); err != nil {
				s.validRow = false
				s.err = err
				s.Cancel() // to cancel iterators after s.curr
				return false
			}
			s.currValue = currValue
			s.validRow = true
			return true
		}
		// Advance returned false.  It could be an err, or it could
		// be we've reached the end.
		if err := s.it[s.curr].Err(); err != nil {
			s.validRow = false
			s.err = err
			s.Cancel() // to cancel iterators after s.curr
			return false
		}
		// We've reached the end of the iterator for this keyRange.
		// Jump to the next one.
		s.it[s.curr] = nil
		s.curr++
		s.validRow = false
	}
	// There are no more prefixes to scan.
	return false
}

func (s *kvs) KeyValue() (string, *vom.RawBytes) {
	if !s.validRow {
		return "", nil
	}
	return s.currKey, s.currValue
}

func (s *kvs) Err() error {
	return s.err
}

func (s *kvs) Cancel() {
	if s.it != nil {
		for i := s.curr; i < len(s.it); i++ {
			s.it[i].Cancel()
		}
		s.it = nil
	}
	// set curr to end of keyRanges so Advance will return false
	s.curr = len(s.it)
}

////////////////////////////////////////
// Internal helpers

func (d *database) stKey() string {
	return common.DatabasePrefix
}

func (d *database) runWithExistingBatchOrNewSnapshot(ctx *context.T, bh wire.BatchHandle, fn func(sntx store.SnapshotOrTransaction) error) error {
	if bh != "" {
		if sntx, err := d.batchReader(ctx, bh); err != nil {
			// Batch does not exist.
			return err
		} else {
			return fn(sntx)
		}
	} else {
		return store.RunWithSnapshot(d.st, fn)
	}
}

func (d *database) runInExistingBatchOrNewTransaction(ctx *context.T, bh wire.BatchHandle, fn func(ts *transactionState) error) error {
	if bh != "" {
		if batch, err := d.batchTransaction(ctx, bh); err != nil {
			// Batch does not exist or is readonly (snapshot).
			return err
		} else {
			return fn(batch)
		}
	} else {
		return d.runInTransaction(fn)
	}
}

func (d *database) batchReader(ctx *context.T, bh wire.BatchHandle) (store.SnapshotOrTransaction, error) {
	sn, ts, _, err := d.batchLookupInternal(ctx, bh)
	if err != nil {
		return nil, err
	}
	if sn != nil {
		return sn, nil
	}
	return ts.tx, nil
}

func (d *database) batchTransaction(ctx *context.T, bh wire.BatchHandle) (*transactionState, error) {
	sn, ts, _, err := d.batchLookupInternal(ctx, bh)
	if err != nil {
		return nil, err
	}
	if sn != nil {
		return nil, wire.NewErrReadOnlyBatch(ctx)
	}
	return ts, nil
}

// batchLookupInternal parses the batch handle and retrieves the corresponding
// snapshot or transaction. It returns an error if the handle is malformed or
// the batch does not exist. Otherwise, exactly one of sn and ts will be != nil.
func (d *database) batchLookupInternal(ctx *context.T, bh wire.BatchHandle) (sn store.Snapshot, ts *transactionState, batchId uint64, _ error) {
	if bh == "" {
		return nil, nil, 0, verror.New(verror.ErrInternal, ctx, "batch lookup for empty handle")
	}
	bType, bId, err := common.SplitBatchHandle(bh)
	if err != nil {
		return nil, nil, 0, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var found bool
	switch bType {
	case common.BatchTypeSn:
		sn, found = d.sns[bId]
	case common.BatchTypeTx:
		ts, found = d.txs[bId]
	}
	if !found {
		return nil, nil, bId, wire.NewErrUnknownBatch(ctx)
	}
	return sn, ts, bId, nil
}

func (d *database) setPermsInternal(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	if !d.exists {
		vlog.Fatalf("database %v does not exist", d.id)
	}
	if err := common.ValidatePerms(ctx, perms, wire.AllDatabaseTags); err != nil {
		return err
	}
	return store.RunInTransaction(d.st, func(tx store.Transaction) error {
		data := &DatabaseData{}
		return util.UpdateWithAuth(ctx, call, tx, d.stKey(), data, func() error {
			if err := util.CheckVersion(ctx, version, data.Version); err != nil {
				return err
			}
			data.Perms = perms
			data.Version++
			return nil
		})
	})
}

// runInTransaction runs the given fn in a transaction, managing retries and
// commit/abort.
func (d *database) runInTransaction(fn func(ts *transactionState) error) error {
	// TODO(rogulenko): Change the default number of attempts to 3. Currently,
	// some storage engine tests fail when the number of attempts is that low.
	return d.runInTransactionWithOpts(&store.TransactionOptions{NumAttempts: 100}, fn)
}

// runInTransactionWithOpts runs the given fn in a transaction, managing retries
// and commit/abort.
func (d *database) runInTransactionWithOpts(opts *store.TransactionOptions, fn func(ts *transactionState) error) error {
	var err error
	for i := 0; i < opts.NumAttempts; i++ {
		// TODO(sadovsky): Should NewTransaction return an error? If not, how will
		// we deal with RPC errors when talking to remote storage engines? (Note,
		// client-side BeginBatch returns an error.)
		ts := &transactionState{tx: d.st.NewWatchableTransaction()}
		if err = fn(ts); err != nil {
			ts.tx.Abort()
			return err
		}
		// TODO(sadovsky): Commit() can fail for a number of reasons, e.g. RPC
		// failure or ErrConcurrentTransaction. Depending on the cause of failure,
		// it may be desirable to retry the Commit() and/or to call Abort().
		if err = ts.tx.Commit(); verror.ErrorID(err) != store.ErrConcurrentTransaction.ID {
			return err
		}
	}
	return err
}
