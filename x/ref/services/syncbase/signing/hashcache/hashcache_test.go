// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hashcache_test tests the hashcache package.
package hashcache_test

import "runtime"
import "testing"
import "time"

import "v.io/x/ref/services/syncbase/signing/hashcache"

// checkHashesWithNoData() checks that hash[start:] have no data in the cache.
// (The start index is passed, rather than expecting the caller to sub-slice,
// so that error messages refer to the index.)
func checkHashesWithNoData(t *testing.T, cache *hashcache.Cache, start int, hash [][]byte) {
	_, _, callerLine, _ := runtime.Caller(1)
	for i := start; i != len(hash); i++ {
		value, found := cache.Lookup(hash[i])
		if value != nil || found {
			t.Errorf("line %d: unset cache entry hash[%d]=%v has value %v, but is expected not to be set", callerLine, i, hash[i], value)
		}
	}
}

func TestCache(t *testing.T) {
	hash := [][]byte{
		[]byte{0x00, 0x01, 0x02, 0x3},
		[]byte{0x04, 0x05, 0x06, 0x7},
		[]byte{0x08, 0x09, 0x0a, 0xb}}
	var value interface{}
	var found bool
	var want string

	cache := hashcache.New(5 * time.Second)

	// The cache should initially have none of the keys.
	checkHashesWithNoData(t, cache, 0, hash)

	// Add the first key, and check that it's there.
	want = "hash0"
	cache.Add(hash[0], want)
	value, found = cache.Lookup(hash[0])
	if s, ok := value.(string); !found || !ok || s != want {
		t.Errorf("cache entry hash[%d]=%v got %v, want %v", 0, hash[0], s, want)
	}
	checkHashesWithNoData(t, cache, 1, hash)

	// Add the second key, and check that both it and the first key are there.
	want = "hash1"
	cache.Add(hash[1], want)
	value, found = cache.Lookup(hash[1])
	if s, ok := value.(string); !ok || s != want {
		t.Errorf("cache entry hash[%d]=%v got %v, want %v", 1, hash[1], s, want)
	}
	want = "hash0"
	value, found = cache.Lookup(hash[0])
	if s, ok := value.(string); !found || !ok || s != want {
		t.Errorf("cache entry hash[%d]=%v got %v, want %v", 0, hash[0], s, want)
	}
	checkHashesWithNoData(t, cache, 2, hash)

	// Wait for all entries to time out.
	time.Sleep(6 * time.Second) // sleep past expiry time

	// Add the first key again, and so many that it will trigger garbage
	// collection.
	for i := 0; i != 10; i++ {
		want = "hash0 again"
		cache.Add(hash[0], want)
		value, found = cache.Lookup(hash[0])
		if s, ok := value.(string); !found || !ok || s != want {
			t.Errorf("cache entry hash[%d]=%v got %v, want %v", 0, hash[0], s, want)
		}
	}
	// The entry for hash1 should have expired, since the expiry time has
	// passed, and many things have been inserted into the cache.
	checkHashesWithNoData(t, cache, 1, hash)

	cache.Delete(hash[0])
	checkHashesWithNoData(t, cache, 0, hash)
}
