// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package leveldb_fsck performs consistency checking
// of a leveldb-based syncbase store.
package leveldb_fsck

import "fmt"
import "path/filepath"

import "v.io/v23/syncbase/util"
import "v.io/v23/vom"
import "v.io/x/ref/services/syncbase/server"
import "v.io/x/ref/services/syncbase/common"
import "v.io/x/ref/services/syncbase/store"
import "v.io/x/ref/services/syncbase/store/leveldb"

var startKey []byte // A start key for a scan of everything.
var limitKey []byte // A limit key for a scan of everything.

// init() initializes startKey and limitKey.
func init() {
	startKey = make([]byte, 0)
	limitKey = make([]byte, 32)
	for i := 0; i != len(limitKey); i++ {
		limitKey[i] = 0xff
	}
}

// limit() returns a "limit" argument of StoreReader.Scan(), assuming that
// "start" is the corresponding "start" argument, and the client wishes to
// iterate over all keys with "start" as a prefix.
func limit(start []byte) (r []byte) {
	r = []byte(util.PrefixRangeLimit(string(start)))
	if len(r) == 0 { // Handle case where we wish to iterate to the end of the database.
		r = limitKey
	}
	return r
}

// An appDBData is in-memory information for a given app's data in a syncbase database.
type appDBData struct {
	path  string      // The path of the directiory containing an app's leveldb.
	appDB store.Store // The handle on the app's leveldb.
}

// A dbContext is the in-memory information for a syncbase database being processed
// by leveldb_fsck.
type dbContext struct {
	rootPath  string      // The root path of the database.
	errors    []error     // A list of errors encountered while trying to process the database.
	rootDB    store.Store // A handle on the syncbase's root database.
	blobDB    store.Store // A handle on the syncbase's blobmap database.
	appDBList []appDBData // Data on the various app databases within syncbase.
}

// appendError() appends err to dbCtx.errors if err!=nil.
func appendError(dbCtx *dbContext, err error) {
	if err != nil {
		dbCtx.errors = append(dbCtx.errors, err)
	}
}

// CheckDBRoot() attempts to check the root store of the syncbase database at the specified path.
// It returns a list of errors in *errors.
func CheckDBRoot(path string, errors *[]error) {
	keyBuf := make([]byte, 1024)
	valBuf := make([]byte, 1024)
	var err error

	dbCtx := new(dbContext)

	dbCtx.rootPath = path

	dbCtx.rootDB, err = leveldb.Open(filepath.Join(path, "leveldb"), leveldb.OpenOptions{})
	appendError(dbCtx, err)
	dbCtx.blobDB, err = leveldb.Open(filepath.Join(path, "blobs/chunk"), leveldb.OpenOptions{})
	appendError(dbCtx, err)

	var appDBList []string

	if dbCtx.rootDB != nil {
		fmt.Printf("******************* rootDB\n")
		cantDecode := 0
		var stream store.Stream = dbCtx.rootDB.Scan(startKey, limit(startKey))
		for stream.Advance() {
			keyStr := string(stream.Key(keyBuf))
			valueBytes := stream.Value(valBuf)
			var value interface{}
			err = vom.Decode(valueBytes, &value)
			if err != nil {
				if cantDecode == 0 {
					appendError(dbCtx, fmt.Errorf("rootDB: can't vom.Decode value:   key=%q   value=%v", keyStr, stream.Value(valBuf)))
				}
				cantDecode++
			} else {
				if dbInfo, ok := value.(*server.DbInfo); ok {
					appDBList = append(appDBList, dbInfo.RootDir)
				} else {
					fmt.Printf("key=%q\nvalue=%#v\n\n", common.SplitNKeyParts(keyStr, -1), value)
				}
			}
		}
		if cantDecode > 1 {
			appendError(dbCtx, fmt.Errorf("rootDB: can't vom.Decode %d values in total", cantDecode))
		}

		appendError(dbCtx, stream.Err())
		fmt.Printf("******************* END rootDB\n\n\n")
	}

	// Process the application stores.
	for i := 0; i != len(appDBList); i++ {
		CheckDBApp(dbCtx, appDBList[i])
	}

	// Process the blobmap.
	if dbCtx.blobDB != nil {
		CheckDBBlobs(dbCtx)
	}

	CloseDB(dbCtx)

	*errors = dbCtx.errors
}

// CloseDB() closes the stores associated with the database *dbCtx.
func CloseDB(dbCtx *dbContext) {
	if dbCtx.blobDB != nil {
		appendError(dbCtx, dbCtx.blobDB.Close())
	}
	if dbCtx.rootDB != nil {
		appendError(dbCtx, dbCtx.rootDB.Close())
	}
	for i := range dbCtx.appDBList {
		if dbCtx.appDBList[i].appDB != nil {
			appendError(dbCtx, dbCtx.appDBList[i].appDB.Close())
		}
	}
}
