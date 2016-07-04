// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb_fsck

import "fmt"
import "path/filepath"

import "v.io/v23/vom"
import "v.io/x/ref/services/syncbase/common"
import "v.io/x/ref/services/syncbase/store"
import "v.io/x/ref/services/syncbase/store/leveldb"

// CheckDBApp() attempts to check a store in directory "path", associated with
// an app, within database *dbCtx.
func CheckDBApp(dbCtx *dbContext, path string) {
	keyBuf := make([]byte, 1024)
	valBuf := make([]byte, 1024)
	var err error

	idx := len(dbCtx.appDBList)
	dbCtx.appDBList = append(dbCtx.appDBList, appDBData{})

	dbCtx.appDBList[idx].path = path

	dbCtx.appDBList[idx].appDB, err = leveldb.Open(filepath.Join(path, "leveldb"), leveldb.OpenOptions{})
	var appDB store.Store = dbCtx.appDBList[idx].appDB
	appendError(dbCtx, err)

	if appDB != nil {
		fmt.Printf("******************* appDB %q\n", path)
		cantDecode := 0
		var stream store.Stream = appDB.Scan(startKey, limit(startKey))
		for stream.Advance() {
			keyStr := string(stream.Key(keyBuf))
			valueBytes := stream.Value(valBuf)
			var value interface{}
			err = vom.Decode(valueBytes, &value)
			if err != nil {
				if cantDecode == 0 {
					appendError(dbCtx, fmt.Errorf("appDB: can't vom.Decode value:   key=%q   value=%v", keyStr, stream.Value(valBuf)))
				}
				cantDecode++
			} else {
				fmt.Printf("key=%q\n\tvalue=%#v\n\n", common.SplitNKeyParts(keyStr, -1), value)
			}
		}
		if cantDecode > 1 {
			appendError(dbCtx, fmt.Errorf("appDB: can't vom.Decode %d values in total", cantDecode))
		}
		appendError(dbCtx, stream.Err())
		fmt.Printf("******************* END appDB %q\n\n\n", path)
	}
}
