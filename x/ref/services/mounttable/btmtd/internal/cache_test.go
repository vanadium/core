// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"sync"
	"testing"
	"time"

	"google.golang.org/cloud/bigtable"

	"v.io/x/ref/test/timekeeper"
)

func TestRowCache(t *testing.T) {
	clock := timekeeper.NewManualTime()
	clock.AdvanceTime(2 * cacheTimeout)
	SetClock(clock)
	rowCache := &rowCache{}

	testcases := []struct {
		key             string
		returnedVersion string
		expectedVersion string
		expectedCalls   int
		skip            time.Duration
	}{
		// Time 0
		{"one", "100", "100", 1, 0},
		{"two", "200", "200", 1, cacheTimeout/2 + 1},
		// Time 0.5 sec
		{"one", "XXX", "100", 0, 0},
		{"two", "XXX", "200", 0, cacheTimeout/2 + 1},
		// Time 1 sec
		{"one", "101", "101", 1, 0},
		{"two", "201", "201", 1, cacheTimeout/2 + 1},
		// Time 1.5 sec
		{"one", "XXX", "101", 0, 0},
		{"two", "XXX", "201", 0, cacheTimeout/2 + 1},
		// Time 2 sec
		{"one", "102", "102", 1, 0},
		{"two", "202", "202", 1, cacheTimeout/2 + 1},
	}

	for i, tc := range testcases {
		calls := 0
		getRow := func() (bigtable.Row, error) {
			calls++
			row := bigtable.Row{
				metadataFamily: []bigtable.ReadItem{
					{
						Row:    tc.key,
						Column: versionColumn,
						Value:  []byte(tc.returnedVersion),
					},
				},
			}
			return row, nil
		}
		const N = 5
		var wg sync.WaitGroup
		wg.Add(N)
		for x := 0; x < N; x++ {
			go func() {
				defer wg.Done()
				row, err := rowCache.getRefresh(tc.key, getRow)
				if err != nil {
					t.Errorf("Unexpected error for #%d: %v", i, err)
					return
				}
				if got := row.Key(); got != tc.key {
					t.Errorf("Unexpected key for #%d. Got %q, expected %q", i, got, tc.key)
				}
				meta := row[metadataFamily]
				if meta == nil {
					t.Errorf("Missing metadata family for #%d", i)
					return
				}
				if len(meta) != 1 {
					t.Errorf("Unexpected number of cells for #%d: %v", i, meta)
					return
				}
				if got := string(meta[0].Value); got != tc.expectedVersion {
					t.Errorf("Unexpected version for #%d. Got %q, expected %q", i, got, tc.expectedVersion)
				}
			}()
		}
		wg.Wait()
		if got := calls; got != tc.expectedCalls {
			t.Errorf("Unexpected function call count for #%d. Got %d, expected %d", i, got, tc.expectedCalls)
		}
		clock.AdvanceTime(tc.skip)
	}

}
