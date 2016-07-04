// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dbutil

import (
	"flag"
	"fmt"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
)

var (
	flagCreateIfAbsent bool
	flagService        string
	flagDbId           string
)

func init() {
	flag.BoolVar(&flagCreateIfAbsent, "create-if-absent", false, "Create the target database if it doesn't exist.")
	flag.StringVar(&flagService, "service", "", "Syncbase service to connect to.")
	flag.StringVar(&flagDbId, "db", "", "Id of database to connect to.")
}

func GetService() syncbase.Service {
	return syncbase.NewService(flagService)
}

// OpenDB is a user-friendly wrapper for openDB.
func OpenDB(ctx *context.T) (syncbase.Database, error) {
	// Open a connection to syncbase.
	sbService := syncbase.NewService(flagService)
	dbId, err := wire.ParseId(flagDbId)
	if err != nil {
		return nil, err
	}
	return openDB(ctx, sbService, dbId, flagCreateIfAbsent)
}

// openDB opens the db at sbService specified by id.
func openDB(ctx *context.T, sbService syncbase.Service, id wire.Id, createIfAbsent bool) (syncbase.Database, error) {
	db := sbService.DatabaseForId(id, nil)
	if exists, err := db.Exists(ctx); err != nil {
		return nil, fmt.Errorf("failed checking for db %q: %v", db.FullName(), err)
	} else if !exists {
		if createIfAbsent {
			if err := db.Create(ctx, nil); err != nil {
				return nil, fmt.Errorf("%s\nDeveloper's note: double-check your permissions!", err)
			}
		} else {
			return nil, fmt.Errorf("db %q does not exist", db.FullName())
		}
	}

	return db, nil
}
