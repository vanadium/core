// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sbtree

import (
	"fmt"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/vdl"
)

// CollectionTree has all the data for the collection page of the Syncbase debug
// viewer.
type CollectionTree struct {
	Service    syncbase.Service
	Database   syncbase.Database
	Collection syncbase.Collection
	RowCount   int
	TotKeySize uint64
	KeysPage   keysPage
}

type keyVal struct {
	Index int
	Key   string
	Value interface{}
}

// keysPage is a contiguous subset of the keys, used for pagination.
type keysPage struct {
	HasPrev bool
	KeyVals []keyVal
	NextKey string
}

// AssembleCollectionTree returns information describing the given Syncbase
// collection, including a "page" of keys starting at the given first key.
func AssembleCollectionTree(
	ctx *context.T, server, dbBlessing, dbName, collBlessing, collName, firstKey string, keysPerPage int,
) *CollectionTree {
	var (
		dbID    = wire.Id{Blessing: dbBlessing, Name: dbName}
		collID  = wire.Id{Blessing: collBlessing, Name: collName}
		service = syncbase.NewService(server)

		// TODO(eobrain) Confirm nil for schema is appropriate
		db   = service.DatabaseForId(dbID, nil)
		coll = db.CollectionForId(collID)
	)

	rowCount, totKeySize, page := scanCollection(ctx, coll, firstKey, keysPerPage)
	return &CollectionTree{service, db, coll, rowCount, totKeySize, page}
}

// scanCollection gets some statistics, plus a page of key-value pairs starting
// with firstKey.
func scanCollection(
	ctx *context.T, coll syncbase.Collection, firstKey string, keysPerPage int,
) (rowCount int, totKeySize uint64, page keysPage) {
	page.KeyVals = make([]keyVal, 0, keysPerPage)

	stream := coll.Scan(ctx, syncbase.Prefix(""))

	// We scan through all the keys lexicographically, and when we come to a
	// key >= firstKey we start gathering a "page" of keys. As we scan, a
	// state machine keeps track of whether we are before the page, in the
	// page, or after the page.
	const (
		before    = 0
		gathering = iota
		done      = iota
	)
	state := before
	for stream.Advance() {
		key := stream.Key()
		totKeySize += uint64(len(key))

		switch state {
		case before:
			if key >= firstKey {
				// First key found: transition to gathering the page
				state = gathering
			} else {
				// There is at least one key before the page
				page.HasPrev = true
			}
		case gathering:
			if len(page.KeyVals) >= keysPerPage {
				// Page full: transition to done.  There is at
				// least one key after the page.
				state = done
				page.NextKey = key
			}
		case done:
			// Done gathering.  Terminal state.
		}
		if state == gathering {
			// Grab the value, put it and the key into a KeyVal, and
			// add it to the page.
			kv := keyVal{Index: rowCount, Key: key}
			var value *vdl.Value
			if err := stream.Value(&value); err != nil {
				kv.Value = fmt.Sprintf("ERROR getting value: %v", err)
			} else {
				kv.Value = value
			}
			page.KeyVals = append(page.KeyVals, kv)
		}
		rowCount++
	}
	return
}
