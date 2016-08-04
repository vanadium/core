// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lru implements a Least-Recently-Used (LRU) cache of objects keyed by a string.
//
// A given key can have multiple values associated with it in the cache.
package lru

import "container/list"

// Cache implements a Least-Recently-Used cache of objects.
// Cache objects are not safe for concurrent use.
type Cache struct {
	m        map[string]*list.List // map from cache key to elements in l that are associated with that key.
	l        *list.List            // contents of the cache (as lElem objects), LRU at the back.
	capacity int                   // maximum number of (key, value) pairs to cache.
}

// lElem is the type used for elements in the Cache.l list.
type lElem struct {
	key   string
	value interface{}
}

// New creates an LRU-cache that can hold up to capacity objects.
func New(capacity int) *Cache {
	return &Cache{make(map[string]*list.List), list.New(), capacity}
}

// Put adds the provided (key, value) pair to the cache and returns the (key,
// value) pair that was evicted (if any) in order to make space for the
// provided pair.
//
// It is legal to use the same key in multiple Put invocations as the cache can
// hold multiple values associated with the same key.
func (c *Cache) Put(key string, value interface{}) (evictedKey string, evictedValue interface{}, evicted bool) {
	if evict := c.l.Len() >= c.capacity; evict {
		lElem := c.evict()
		evicted = true
		evictedKey = lElem.key
		evictedValue = lElem.value
	}
	ml, ok := c.m[key]
	if !ok {
		ml = list.New()
		c.m[key] = ml
	}
	// Now add to c.l and ml
	ml.PushFront(c.l.PushFront(lElem{key, value}))
	return
}

// Get returns a value in the cache associated with the provided key and
// removes it from the cache's storage.  It returns (nil, false) if no value is
// associated with the key.
//
// If there are multiple values associated with the provided key, Get will
// return the most recently used one.
func (c *Cache) Get(key string) (interface{}, bool) {
	ml, ok := c.m[key]
	if !ok {
		return nil, false
	}
	lElem := c.l.Remove(ml.Remove(ml.Front()).(*list.Element)).(lElem)
	// At this point, lElem.key == key
	// if lElem.key != key { panic("Bug! Cache.m[key] is not in sync with Cacke.l") }
	if ml.Len() == 0 {
		delete(c.m, key)
	}
	return lElem.value, true
}

func (c *Cache) evict() lElem {
	lElem := c.l.Remove(c.l.Back()).(lElem)
	// Remove the entry in c.m[key].
	ml := c.m[lElem.key]
	ml.Remove(ml.Back())
	if ml.Len() == 0 {
		delete(c.m, lElem.key)
	}
	return lElem
}

// Size returns the number of items currently in the cache.
func (c *Cache) Size() int {
	return c.l.Len()
}
