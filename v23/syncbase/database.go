// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"sync"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

const (
	// Wait time before we try to reconnect a broken conflict resolution stream.
	waitBeforeReconnectInMillis = 2 * time.Second
	reconnectionCount           = "rcc"
)

func newDatabase(parentFullName string, id wire.Id, schema *Schema) *database {
	return &database{
		databaseBatch: *newDatabaseBatch(parentFullName, id, ""),
		schema:        schema,
		crState: conflictResolutionState{
			reconnectWaitTime: waitBeforeReconnectInMillis,
		},
	}
}

type database struct {
	databaseBatch
	schema  *Schema
	crState conflictResolutionState
}

// conflictResolutionState maintains data about the connection of a conflict
// resolution stream with syncbase. It provides a way to disconnect an existing
// open stream.
type conflictResolutionState struct {
	mu                sync.Mutex // guards access to all fields in this struct
	crContext         *context.T
	cancelFn          context.CancelFunc
	isClosed          bool
	reconnectWaitTime time.Duration
}

func (crs *conflictResolutionState) disconnect() {
	crs.mu.Lock()
	defer crs.mu.Unlock()
	crs.isClosed = true
	crs.cancelFn()
}

func (crs *conflictResolutionState) isDisconnected() bool {
	crs.mu.Lock()
	defer crs.mu.Unlock()
	return crs.isClosed
}

var _ Database = (*database)(nil)

// Create implements Database.Create.
func (d *database) Create(ctx *context.T, perms access.Permissions) error {
	var schemaMetadata *wire.SchemaMetadata = nil
	if d.schema != nil {
		schemaMetadata = &d.schema.Metadata
	}
	if perms == nil {
		// Default to giving full permissions to the creator.
		_, user, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(ctx))...)
		if err != nil {
			return verror.New(wire.ErrInferDefaultPermsFailed, ctx, "Database", util.EncodeId(d.id), err)
		}
		perms = access.Permissions{}.Add(user, access.TagStrings(wire.AllDatabaseTags...)...)
	}
	return d.c.Create(ctx, schemaMetadata, perms)
}

// Destroy implements Database.Destroy.
func (d *database) Destroy(ctx *context.T) error {
	return d.c.Destroy(ctx)
}

// Exists implements Database.Exists.
func (d *database) Exists(ctx *context.T) (bool, error) {
	return d.c.Exists(ctx)
}

// BeginBatch implements Database.BeginBatch.
func (d *database) BeginBatch(ctx *context.T, opts wire.BatchOptions) (BatchDatabase, error) {
	batchHandle, err := d.c.BeginBatch(ctx, opts)
	if err != nil {
		return nil, err
	}
	return newBatch(d.parentFullName, d.id, batchHandle), nil
}

// SetPermissions implements Database.SetPermissions.
func (d *database) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return d.c.SetPermissions(ctx, perms, version)
}

// GetPermissions implements Database.GetPermissions.
func (d *database) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	return d.c.GetPermissions(ctx)
}

// Watch implements Database.Watch.
func (d *database) Watch(ctx *context.T, resumeMarker watch.ResumeMarker, patterns []wire.CollectionRowPattern) WatchStream {
	ctx, cancel := context.WithCancel(ctx)
	call, err := d.c.WatchPatterns(ctx, resumeMarker, patterns)
	if err != nil {
		cancel()
		return &invalidWatchStream{invalidStream{err: err}}
	}
	return newWatchStream(cancel, call)
}

// Syncgroup implements Database.Syncgroup.
func (d *database) Syncgroup(ctx *context.T, name string) Syncgroup {
	_, user, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(ctx))...)
	if err != nil {
		ctx.Error(verror.New(wire.ErrInferUserBlessingFailed, ctx, "Syncgroup", name, err))
		// A handle with a no-match Id blessing is returned, so all RPCs will fail.
		// TODO(ivanpi): Return the more specific error from RPCs instead of logging
		// it here.
	}
	return newSyncgroup(d.fullName, wire.Id{Blessing: string(user), Name: name})
}

// SyncgroupForId implements Database.SyncgroupForId.
func (d *database) SyncgroupForId(id wire.Id) Syncgroup {
	return newSyncgroup(d.fullName, id)
}

// ListSyncgroups implements Database.ListSyncgroups.
func (d *database) ListSyncgroups(ctx *context.T) ([]wire.Id, error) {
	return d.c.ListSyncgroups(ctx)
}

// Blob implements Database.Blob.
func (d *database) Blob(br wire.BlobRef) Blob {
	return newBlob(d.fullName, br)
}

// CreateBlob implements Database.CreateBlob.
func (d *database) CreateBlob(ctx *context.T) (Blob, error) {
	return createBlob(ctx, d.fullName)
}

// PauseSync implements Database.PauseSync.
func (d *database) PauseSync(ctx *context.T) error {
	return d.c.PauseSync(ctx)
}

// ResumeSync implements Database.ResumeSync.
func (d *database) ResumeSync(ctx *context.T) error {
	return d.c.ResumeSync(ctx)
}

// EnforceSchema implements Database.EnforceSchema.
func (d *database) EnforceSchema(ctx *context.T) error {
	var schema *Schema = d.schema
	if schema == nil {
		return verror.New(verror.ErrBadState, ctx, "EnforceSchema cannot be used since a nil *Schema was provided at Database handle creation time.")
	}

	if schema.Metadata.Version < 0 {
		return verror.New(verror.ErrBadState, ctx, "Schema version cannot be less than zero.")
	}

	if needsResolver(d.schema.Metadata) && d.schema.Resolver == nil {
		return verror.New(verror.ErrBadState, ctx, "ResolverTypeAppResolves cannot be used in CrRule without providing a ConflictResolver in Schema.")
	}

	if _, err := d.updateSchemaMetadata(ctx); err != nil {
		return err
	}

	if d.schema.Resolver == nil {
		return nil
	}

	childCtx, cancelFn := context.WithCancel(ctx)
	d.crState.crContext = childCtx
	d.crState.cancelFn = cancelFn

	go d.establishConflictResolution(childCtx)
	return nil
}

// Close implements Database.Close.
func (d *database) Close() {
	d.crState.disconnect()
	// TODO(ivanpi): Abort all batches.
}

// updateSchemaMetadata reads the current SchemaMetadata from db and checks
// if the SchemaMetadata provided by the app is newer. If so, it updates the
// db with the new SchemaMetadata.
func (d *database) updateSchemaMetadata(ctx *context.T) (bool, error) {
	var schema *Schema = d.schema
	schemaMgr := d.getSchemaManager()
	currMeta, err := schemaMgr.getSchemaMetadata(ctx)
	if err != nil {
		// If the client app did not set a schema as part of database creation,
		// getSchemaMetadata() will return ErrNoExist. In this case, we set the
		// schema here.
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			err := schemaMgr.setSchemaMetadata(ctx, schema.Metadata)
			// The database may not yet exist. If so, the above call will return
			// ErrNoExist, and here we return (false, nil). If the error is anything
			// other than ErrNoExist, here we return (false, err).
			if (err != nil) && (verror.ErrorID(err) != verror.ErrNoExist.ID) {
				return false, err
			}
			return false, nil
		}
		return false, err
	}

	if currMeta.Version >= schema.Metadata.Version {
		return false, nil
	}

	// Update the schema metadata in db to the latest version.
	if metadataErr := schemaMgr.setSchemaMetadata(ctx, schema.Metadata); metadataErr != nil {
		vlog.Error(metadataErr)
		return false, metadataErr
	}
	return true, nil
}

func (d *database) establishConflictResolution(ctx *context.T) {
	count := 0
	for {
		count++
		vlog.Infof("Starting a new conflict resolution connection. Reconnection count: %d", count)
		childCtx := context.WithValue(ctx, reconnectionCount, count)
		// listenForConflicts is a blocking method that returns only when the
		// conflict stream is broken.
		if err := d.listenForConflicts(childCtx); err != nil && !d.crState.isDisconnected() {
			vlog.Errorf("Conflict resolution connection ended with error: %v", err)
		}

		// Check if database is closed and if we need to shutdown conflict
		// resolution.
		if d.crState.isDisconnected() {
			vlog.Infof("Shutting down conflict resolution connection.")
			break
		}

		// The connection might have broken because the syncbase server went down.
		// Sleep for a few seconds to allow syncbase to come back up.
		time.Sleep(d.crState.reconnectWaitTime)
	}
}

func (d *database) listenForConflicts(ctx *context.T) error {
	resolver, err := d.c.StartConflictResolver(ctx)
	if err != nil {
		return err
	}
	conflictStream := resolver.RecvStream()
	resolutionStream := resolver.SendStream()
	var c *Conflict = &Conflict{}
	for conflictStream.Advance() {
		row := conflictStream.Value()
		addRowToConflict(c, &row)
		if !row.Continued {
			resolution := d.schema.Resolver.OnConflict(ctx, c)
			if err := sendResolution(resolutionStream, resolution); err != nil {
				return err
			}
			c = &Conflict{} // create a new conflict object for the next batch
		}
	}
	if err := conflictStream.Err(); err != nil {
		return err
	}
	return resolver.Finish()
}

// TODO(jlodhia): Should we check that the Resolution provided by the
// application resolves all conflicts in the write set?
func sendResolution(stream interface {
	Send(item wire.ResolutionInfo) error
}, resolution Resolution) error {
	size := len(resolution.ResultSet)
	count := 0
	for _, v := range resolution.ResultSet {
		count++
		ri := toResolutionInfo(v, count != size)
		if err := stream.Send(ri); err != nil {
			vlog.Errorf("Error while sending resolution: %v", err)
			return err
		}
	}
	return nil
}

func addRowToConflict(c *Conflict, ci *wire.ConflictInfo) {
	switch v := ci.Data.(type) {
	case wire.ConflictDataBatch:
		if c.Batches == nil {
			c.Batches = map[uint64]wire.BatchInfo{}
		}
		c.Batches[v.Value.Id] = v.Value
	case wire.ConflictDataRow:
		rowInfo := v.Value
		switch op := rowInfo.Op.(type) {
		case wire.OperationWrite:
			if c.WriteSet == nil {
				c.WriteSet = &ConflictRowSet{map[string]ConflictRow{}, map[uint64][]ConflictRow{}}
			}
			cr := toConflictRow(op.Value, rowInfo.BatchIds)
			c.WriteSet.ByKey[cr.Key] = cr
			for _, bid := range rowInfo.BatchIds {
				c.WriteSet.ByBatch[bid] = append(c.WriteSet.ByBatch[bid], cr)
			}
		case wire.OperationRead:
			if c.ReadSet == nil {
				c.ReadSet = &ConflictRowSet{map[string]ConflictRow{}, map[uint64][]ConflictRow{}}
			}
			cr := toConflictRow(op.Value, rowInfo.BatchIds)
			c.ReadSet.ByKey[cr.Key] = cr
			for _, bid := range rowInfo.BatchIds {
				c.ReadSet.ByBatch[bid] = append(c.ReadSet.ByBatch[bid], cr)
			}
		case wire.OperationScan:
			if c.ScanSet == nil {
				c.ScanSet = &ConflictScanSet{map[uint64][]wire.ScanOp{}}
			}
			for _, bid := range rowInfo.BatchIds {
				c.ScanSet.ByBatch[bid] = append(c.ScanSet.ByBatch[bid], op.Value)
			}
		}
	}
}

func toConflictRow(op wire.RowOp, batchIds []uint64) ConflictRow {
	local := Value{
		State:     op.LocalValue.State,
		Val:       op.LocalValue.Bytes,
		WriteTs:   op.LocalValue.WriteTs,
		Selection: wire.ValueSelectionLocal,
	}
	remote := Value{
		State:     op.RemoteValue.State,
		Val:       op.RemoteValue.Bytes,
		WriteTs:   op.RemoteValue.WriteTs,
		Selection: wire.ValueSelectionRemote,
	}
	ancestor := Value{
		State:     op.AncestorValue.State,
		Val:       op.AncestorValue.Bytes,
		WriteTs:   op.AncestorValue.WriteTs,
		Selection: wire.ValueSelectionOther,
	}
	return ConflictRow{
		Key:           op.Key,
		LocalValue:    local,
		RemoteValue:   remote,
		AncestorValue: ancestor,
		BatchIds:      batchIds,
	}
}

func toResolutionInfo(r ResolvedRow, lastRow bool) wire.ResolutionInfo {
	sel := wire.ValueSelectionOther
	resVal := (*wire.Value)(nil)
	if r.Result != nil {
		sel = r.Result.Selection
		resVal = &wire.Value{
			Bytes:   r.Result.Val,
			WriteTs: r.Result.WriteTs, // ignored by syncbase
		}
	}
	return wire.ResolutionInfo{
		Key:       r.Key,
		Selection: sel,
		Result:    resVal,
		Continued: lastRow,
	}
}

func needsResolver(metadata wire.SchemaMetadata) bool {
	for _, rule := range metadata.Policy.Rules {
		if rule.Resolver == wire.ResolverTypeAppResolves {
			return true
		}
	}
	return false
}

func (d *database) getSchemaManager() schemaManagerImpl {
	return newSchemaManager(d.c)
}
