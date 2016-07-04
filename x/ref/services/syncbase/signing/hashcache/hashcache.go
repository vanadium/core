// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hashcache implements a simple cache intended to be indexed by hash
// values.  The keys are of type []byte.  Values are arbitrary interface{}
// values.  Entries may expire if not used for a duration specified by the
// client.
package hashcache

import "sync"
import "time"

// An internalValue is the client's data plus the data's expiry time.
type internalValue struct {
	data   interface{}
	expiry time.Time
}

// A Cache allows the user to store arbitrary values, keyed by the contents of
// byte vectors.  Entries may be added, deleted, and looked up.  They may
// expire if not used.
type Cache struct {
	expiry            time.Duration
	mu                sync.Mutex // protects fields below.
	entries           map[string]*internalValue
	insertionsSinceGC int // number of insertions since last GC
}

// New() returns a pointer to a new, empty Cache.
// Entries may expire if not used for "expiry".
func New(expiry time.Duration) *Cache {
	return &Cache{expiry: expiry, entries: make(map[string]*internalValue)}
}

// Lookup() returns the data associated with key[] in *c, and whether there is
// such a value.  The client may not modify the returned data; it is shared
// with *c.
func (c *Cache) Lookup(key []byte) (data interface{}, isPresent bool) {
	var value *internalValue
	c.mu.Lock()
	value, isPresent = c.entries[string(key)]
	if isPresent {
		value.expiry = time.Now().Add(c.expiry)
		data = value.data
	}
	c.mu.Unlock()
	return data, isPresent
}

// Add() associates data with key[] in *c.  Any data previously associated with
// key[] are forgotten.  The implementation may discard the association at some
// future time (set by NewCache()) to limit the size of the cache.  data may
// not be modified after this call; it is shared with *c.
func (c *Cache) Add(key []byte, data interface{}) {
	c.mu.Lock()
	now := time.Now()
	c.entries[string(key)] = &internalValue{data: data, expiry: now.Add(c.expiry)}
	c.insertionsSinceGC++
	// Scan to expire entries if 20% were added since last scan.
	if c.insertionsSinceGC*5 > len(c.entries) {
		for ik, iv := range c.entries {
			if iv.expiry.Before(now) {
				delete(c.entries, ik)
			}
		}
		c.insertionsSinceGC = 0
	}
	c.mu.Unlock()
}

// Delete() removes any association of data with key[] in *c.
func (c *Cache) Delete(key []byte) {
	c.mu.Lock()
	delete(c.entries, string(key))
	c.mu.Unlock()
}
