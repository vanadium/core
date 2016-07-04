// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

func newDatabaseBatch(parentFullName string, id wire.Id, batchHandle wire.BatchHandle) *databaseBatch {
	fullName := naming.Join(parentFullName, util.EncodeId(id))
	return &databaseBatch{
		c:              wire.DatabaseClient(fullName),
		parentFullName: parentFullName,
		fullName:       fullName,
		id:             id,
		bh:             batchHandle,
	}
}

type databaseBatch struct {
	c              wire.DatabaseClientMethods
	parentFullName string
	fullName       string
	id             wire.Id
	bh             wire.BatchHandle
}

var _ DatabaseHandle = (*databaseBatch)(nil)

// Id implements DatabaseHandle.Id.
func (d *databaseBatch) Id() wire.Id {
	return d.id
}

// FullName implements DatabaseHandle.FullName.
func (d *databaseBatch) FullName() string {
	return d.fullName
}

// Collection implements DatabaseHandle.Collection.
func (d *databaseBatch) Collection(ctx *context.T, name string) Collection {
	_, user, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(ctx))...)
	if err != nil {
		ctx.Error(verror.New(wire.ErrInferUserBlessingFailed, ctx, "Collection", name, err))
		// A handle with a no-match Id blessing is returned, so all RPCs will fail.
		// TODO(ivanpi): Return the more specific error from RPCs instead of logging
		// it here.
	}
	return newCollection(d.fullName, wire.Id{Blessing: string(user), Name: name}, d.bh)
}

// CollectionForId implements DatabaseHandle.CollectionForId.
func (d *databaseBatch) CollectionForId(id wire.Id) Collection {
	return newCollection(d.fullName, id, d.bh)
}

// ListCollections implements DatabaseHandle.ListCollections.
func (d *databaseBatch) ListCollections(ctx *context.T) ([]wire.Id, error) {
	// See comment in v.io/v23/services/syncbase/service.vdl for why we
	// can't implement ListCollections using Glob (via util.ListChildren).
	ids, err := d.c.ListCollections(ctx, d.bh)
	if err != nil {
		return nil, err
	}
	util.SortIds(ids)
	return ids, nil
}

// Exec implements DatabaseHandle.Exec.
// TODO(ivanpi): Parameterized Exec currently allows struct comparisons, which
// we wish to prevent. However, cases like JavaScript JSValue benefit from this.
func (d *databaseBatch) Exec(ctx *context.T, query string, params ...interface{}) ([]string, ResultStream, error) {
	paramsVom := make([]*vom.RawBytes, len(params))
	for i, p := range params {
		var err error
		if paramsVom[i], err = vom.RawBytesFromValue(p); err != nil {
			return nil, nil, err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	call, err := d.c.Exec(ctx, d.bh, query, paramsVom)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	resultStream := newResultStream(cancel, call)
	// The first row contains column headers. Pull them off the stream
	// and return them separately.
	var headers []string
	if !resultStream.Advance() {
		if err = resultStream.Err(); err != nil {
			// Since there was an error, can't get headers.
			// Just return the error.
			return nil, nil, err
		}
	}
	for i, n := 0, resultStream.ResultCount(); i != n; i++ {
		var header string
		if err := resultStream.Result(i, &header); err == nil {
			headers = append(headers, header)
		} else {
			return nil, nil, verror.New(wire.ErrBadExecStreamHeader, ctx, query)
		}
	}
	return headers, resultStream, nil
}

// GetResumeMarker implements DatabaseHandle.GetResumeMarker.
func (d *databaseBatch) GetResumeMarker(ctx *context.T) (watch.ResumeMarker, error) {
	return d.c.GetResumeMarker(ctx, d.bh)
}
