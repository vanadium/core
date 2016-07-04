// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client_test

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	"v.io/v23"
	"v.io/v23/syncbase"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/syncbase/longevity_tests/client"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/testutil"
)

func TestWatcher(t *testing.T) {
	ctx, sbName, cleanup := testutil.SetupOrDie(nil)
	defer cleanup()

	// gotRows records the rows in the watchChanges that the watcher receives.
	// Since OnChange is called in a goroutine, this must be locked by a mutex.
	gotRows, mu := []string{}, sync.Mutex{}
	// wg is used to stop the test when watcher.OnChange has fired for each of
	// the rows we put.
	wg := sync.WaitGroup{}
	watcher := &client.Watcher{
		OnChange: func(watchChange syncbase.WatchChange) {
			mu.Lock()
			defer mu.Unlock()
			if watchChange.EntityType == syncbase.EntityRow {
				gotRows = append(gotRows, watchChange.Row)
				wg.Done()
			}
		},
	}

	blessing, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	blessingString := blessing.String()

	dbModel := &model.Database{
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
	}

	// Create the database and each collection.
	// The watcher will attempt to do this as well, but doing it ourselves
	// avoids a race condition in the tests.
	dbColsMap, _, err := client.CreateDbsAndCollections(ctx, sbName, model.DatabaseSet{dbModel})
	if err != nil {
		t.Fatal(err)
	}

	watcher.Start(ctx, sbName, model.DatabaseSet{dbModel})

	// Put 3 rows to each collection.
	wantRows := []string{}
	for _, colSlice := range dbColsMap {
		for i, col := range colSlice {
			for j := 0; j < 3; j++ {
				wg.Add(1)
				key := fmt.Sprintf("key-%d-%d", i, j)
				val := fmt.Sprintf("val-%d-%d", i, j)
				if err := col.Put(ctx, key, val); err != nil {
					t.Fatal(err)
				}
				wantRows = append(wantRows, key)
			}
		}
	}

	// Wait for watcher to receive all the rows we put.
	wg.Wait()

	// Stop the watcher.
	if err := watcher.Stop(); err != nil {
		t.Fatalf("watcher.Stop() returned error: %v", err)
	}

	// Check that we got watchChanges for each row that we put to.
	sort.Strings(wantRows)
	sort.Strings(gotRows)
	if !reflect.DeepEqual(wantRows, gotRows) {
		t.Errorf("wanted gotRows to be %v but got %v", wantRows, gotRows)
	}
}
