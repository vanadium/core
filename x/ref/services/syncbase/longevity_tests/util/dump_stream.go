// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"reflect"

	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/vdl"
)

// Row represents a syncbase Row, Collection, or Database.
type Row struct {
	DatabaseId   wire.Id
	CollectionId wire.Id

	// If the Row represents a Collection or Database, then Key and Value will
	// be "" and nil respectively.
	Key   string
	Value *vdl.Value

	// If the Row represents a real Row, then Permissions will be nil,
	// otherwise it will be the Permissions of the Collection or Database.
	Permissions access.Permissions
}

// Equals returns true if the rows have the same properties.
func (r *Row) Equals(rr *Row) bool {
	return reflect.DeepEqual(r.DatabaseId, rr.DatabaseId) &&
		reflect.DeepEqual(r.CollectionId, rr.CollectionId) &&
		r.Key == rr.Key &&
		vdl.EqualValue(r.Value, rr.Value) &&
		reflect.DeepEqual(r.Permissions, rr.Permissions)
}

// DumpStream iterates through all collections in all database, and all rows in
// each collection, emiting Rows for each row in the collection.  The stream is
// ordered first by database, then by collection, then by row.  In addition, a
// Row is emitted for each database before all of its collections, and a Row is
// emitted for each collections before its rows.
type DumpStream struct {
	// ServiceName is the name of the syncbase service from which Rows will be
	// dumped.
	ServiceName string

	ctx                 *context.T
	databaseCollections []databaseCollection
	err                 error
	nextRow             *Row
	scanStream          syncbase.ScanStream
}

var _ (syncbase.Stream) = (*DumpStream)(nil)

// databaseCollection wraps a database and collection.
type databaseCollection struct {
	database   syncbase.Database
	collection syncbase.Collection
}

func NewDumpStream(ctx *context.T, service syncbase.Service) (*DumpStream, error) {
	// Get all database ids.
	dbIds, err := service.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}

	// Get collections for each database.
	dbCols := []databaseCollection{}
	for _, dbId := range dbIds {
		db := service.DatabaseForId(dbId, nil)
		// Add a databaseCollection for just this database.
		dbCols = append(dbCols, databaseCollection{database: db})
		colIds, err := db.ListCollections(ctx)
		if err != nil {
			return nil, err
		}
		for _, colId := range colIds {
			dbCols = append(dbCols, databaseCollection{
				database:   db,
				collection: db.CollectionForId(colId),
			})
		}
	}

	return &DumpStream{
		ServiceName:         service.FullName(),
		ctx:                 ctx,
		databaseCollections: dbCols,
	}, nil
}

// Row returns the next row in the DumpStream.
func (s *DumpStream) Row() *Row {
	return s.nextRow
}

func (s *DumpStream) Advance() bool {
	// Loop while we either have a stream or collections left.
	for s.scanStream != nil || len(s.databaseCollections) > 0 {
		if s.scanStream == nil {
			dbCol := s.databaseCollections[0]
			if dbCol.collection == nil {
				// A database with no collection.  Emit a Row for just the
				// database, and remove it from the databaseCollections slice.
				perms, _, err := dbCol.database.GetPermissions(s.ctx)
				if err != nil {
					s.setErr(err)
					return false
				}
				s.nextRow = &Row{
					DatabaseId:  dbCol.database.Id(),
					Permissions: perms,
				}
				s.databaseCollections = s.databaseCollections[1:]
				return true
			}

			// Get new scan stream for the collection.
			s.scanStream = dbCol.collection.Scan(s.ctx, syncbase.Prefix(""))

			// Emit a Row for just the collection.
			perms, err := dbCol.collection.GetPermissions(s.ctx)
			if err != nil {
				s.setErr(err)
				return false
			}
			s.nextRow = &Row{
				CollectionId: dbCol.collection.Id(),
				DatabaseId:   dbCol.database.Id(),
				Permissions:  perms,
			}
			return true
		}

		// We have a stream.  Pull rows from it.
		if !s.scanStream.Advance() {
			// Current stream has ended.  Loop to get a new stream from the
			// next collection.
			s.scanStream = nil
			s.databaseCollections = s.databaseCollections[1:]
			continue
		}

		// Get next row key and value from current scan stream.
		key := s.scanStream.Key()
		var val *vdl.Value
		if err := s.scanStream.Value(&val); err != nil {
			s.setErr(err)
			return false
		}
		dbCol := s.databaseCollections[0]
		s.nextRow = &Row{
			Key:          key,
			Value:        val,
			CollectionId: dbCol.collection.Id(),
			DatabaseId:   dbCol.database.Id(),
		}
		return true
	}

	// No more stream or collections.  We are done.
	s.nextRow = nil
	return false
}

func (s *DumpStream) setErr(err error) {
	s.err = err
	s.databaseCollections = nil
	s.nextRow = nil
	s.scanStream = nil
}

func (s *DumpStream) Err() error {
	return s.err
}

func (s *DumpStream) Cancel() {
	if s.scanStream != nil {
		s.scanStream.Cancel()
	}
	s.databaseCollections = []databaseCollection{}
	s.scanStream = nil
}
