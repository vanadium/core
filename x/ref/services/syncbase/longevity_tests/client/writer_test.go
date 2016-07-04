// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/syncbase"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/syncbase/longevity_tests/client"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/testutil"
)

func TestWriter(t *testing.T) {
	ctx, sbName, cleanup := testutil.SetupOrDie(nil)
	defer cleanup()

	var counter int32
	keyValueFunc := func(_ time.Time) (string, interface{}) {
		if counter >= 5 {
			return "", nil
		}
		counter++
		return fmt.Sprintf("%d", counter), counter
	}

	// mu is locked until Writer writes 5 rows to each collection.
	mu := sync.Mutex{}
	mu.Lock()
	wroteKeys := []string{}
	wroteVals := []interface{}{}
	onWrite := func(_ syncbase.Collection, key string, value interface{}, err error) {
		if err != nil {
			t.Fatalf("error writing key: %v: %v", key, err)
		}
		// Only append the key if it's one we haven't seen before.  This
		// function runs once for each collection, so we end up seeing each key
		// twice.
		if len(wroteKeys) == 0 || key != wroteKeys[len(wroteKeys)-1] {
			wroteKeys = append(wroteKeys, key)
			wroteVals = append(wroteVals, value)
		} else if len(wroteKeys) == 5 {
			// The 5th row has been written twice (once to each collection).
			mu.Unlock()
		}
	}
	writer := &client.Writer{
		WriteInterval: 50 * time.Millisecond,
		KeyValueFunc:  keyValueFunc,
		OnWrite:       onWrite,
	}

	blessing, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	blessingString := blessing.String()

	dbColsMap := model.DatabaseSet{
		&model.Database{
			Name:     "test_db",
			Blessing: blessingString,
			Collections: []model.Collection{
				model.Collection{
					Name:     "test_col_1",
					Blessing: blessingString,
				},
				model.Collection{
					Name:     "test_col_2",
					Blessing: blessingString,
				},
			},
		},
	}

	// Run the writer.
	writer.Start(ctx, sbName, dbColsMap)

	// Wait for 5 writes to be written.
	mu.Lock()

	// Stop the writer.
	if err := writer.Stop(); err != nil {
		t.Fatalf("writer.Stop() returned error: %v")
	}

	// Make sure database exists.
	service := syncbase.NewService(sbName)
	db := service.DatabaseForId(dbColsMap[0].Id(), nil)
	if exists, err := db.Exists(ctx); !exists || err != nil {
		t.Fatalf("expected db.Exists() to return true, nil but got: %v, %v", exists, err)
	}

	// Check collections.
	for _, colModel := range dbColsMap[0].Collections {
		// Make sure collection exists.
		col := db.CollectionForId(colModel.Id())
		if exists, err := col.Exists(ctx); !exists || err != nil {
			t.Fatalf("expected collection.Exists() to return true, nil but got: %v, %v", exists, err)
		}
		// Check that collection contains the wroteKeys and wroteVals.
		testutil.CheckScan(t, ctx, col, syncbase.Prefix(""), wroteKeys, wroteVals)
	}
}
