// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
)

// database implements the syncbase.Database interface.
type database struct {
	common
	listCollectionsErr error
	specErr            error
}

func (*database) FullName() string                                  { return "the-database" }
func (*database) Collection(*context.T, string) syncbase.Collection { return nil }
func (*database) CollectionForId(id wire.Id) syncbase.Collection    { return nil }
func (db *database) ListCollections(ctx *context.T) ([]wire.Id, error) {
	return nil, db.listCollectionsErr
}
func (*database) Exec(
	*context.T, string, ...interface{},
) ([]string, syncbase.ResultStream, error) {
	return nil, nil, nil
}
func (*database) GetResumeMarker(*context.T) (watch.ResumeMarker, error) {
	return nil, nil
}
func (*database) Create(*context.T, access.Permissions) error { return nil }
func (*database) Exists(*context.T) (bool, error) {
	return true, nil
}
func (*database) BeginBatch(
	*context.T, wire.BatchOptions,
) (syncbase.BatchDatabase, error) {
	return nil, nil
}
func (
	*database) Watch(*context.T, watch.ResumeMarker, []wire.CollectionRowPattern,
) syncbase.WatchStream {
	return nil
}
func (db *database) SyncgroupForId(wire.Id) syncbase.Syncgroup {
	return &syncgroup{specErr: db.specErr}
}
func (*database) Syncgroup(*context.T, string) syncbase.Syncgroup {
	return nil
}
func (*database) ListSyncgroups(ctx *context.T) ([]wire.Id, error) {
	return []wire.Id{wire.Id{}}, nil
}
func (*database) CreateBlob(*context.T) (syncbase.Blob, error) { return nil, nil }
func (*database) Blob(wire.BlobRef) syncbase.Blob              { return nil }
func (*database) EnforceSchema(*context.T) error               { return nil }
func (*database) PauseSync(*context.T) error                   { return nil }
func (*database) ResumeSync(*context.T) error                  { return nil }
func (*database) Close()                                       {}
