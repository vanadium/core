// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"v.io/v23/context"
	"v.io/v23/security/access"
	"v.io/v23/syncbase"
)

// Rows is a map from keys to values.
type Rows map[string]string

// Collection is a permissions object and Rows.  If Permissions is empty, then
// "..." will be used.
type Collection struct {
	Permissions access.Permissions
	Rows        Rows
}

// Collections is a map from collection names to Collections.
type Collections map[string]Collection

// Database is a permissions object and Collections.  If Permissions if empty,
// then "..." will be used.
type Database struct {
	Permissions access.Permissions
	Collections Collections
}

// Databases is a map from database names to Databases.
type Databases map[string]Database

// SeedService creates databases, collections, and rows in a syncbase service.
func SeedService(ctx *context.T, s syncbase.Service, dbs Databases) error {
	for dbName, db := range dbs {
		sdb := s.Database(ctx, dbName, nil)
		dbPerms := db.Permissions
		if err := sdb.Create(ctx, dbPerms); err != nil {
			return err
		}
		for colName, col := range db.Collections {
			scol := sdb.Collection(ctx, colName)
			colPerms := col.Permissions
			if err := scol.Create(ctx, colPerms); err != nil {
				return err
			}

			for key, val := range col.Rows {
				if err := scol.Put(ctx, key, val); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
