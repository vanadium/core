// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"google.golang.org/cloud/bigtable"
)

// The purpose of this cache is to minimize the impact of "hot" rows, i.e.
// rows that get accessed significantly more often than others.
//
// For example, rows associated to the root node and its immediate children are
// expected to be hotter than rows associated with nodes further down the
// tree.
//
// The cache also makes it so that concurrent read requests for the same row
// are bundled together and only one read request is sent.
//
// The cacheTimeout controls how often rows can be refetched, which directly
// affects how quickly other servers see mutations from this server. A short
// timeout means that mutations propagate faster.
//
// With the current value, the bigtable server will see at most 1 request per
// node per server per second, with the assumption that the server see at
// most 'cacheSize' nodes per second.

const (
	cacheSize    = 1000
	cacheTimeout = time.Second
)

type rowCache struct {
	// mu guards cache insertions and lookups.
	mu    sync.Mutex
	cache *lru.Cache
}

type cacheEntry struct {
	// mu guards 'row' and 'updated', and prevents concurrent rpc requests
	// for the same row. It is locked when the refresh rpc is running.
	mu      sync.Mutex
	row     bigtable.Row
	updated time.Time
}

func (c *rowCache) getRefresh(key string, getRow func() (bigtable.Row, error)) (bigtable.Row, error) {
	c.mu.Lock()
	if c.cache == nil {
		c.cache = lru.New(cacheSize)
	}
	entry, ok := c.cache.Get(key)
	if !ok {
		entry = &cacheEntry{}
		c.cache.Add(key, entry)
	}
	c.mu.Unlock()

	e := entry.(*cacheEntry)
	e.mu.Lock()
	defer e.mu.Unlock()

	if clock.Now().Sub(e.updated) < cacheTimeout {
		return e.row, nil
	}
	var err error
	if e.row, err = getRow(); err != nil || e.row.Key() == "" {
		return e.row, err
	}
	e.updated = clock.Now()
	return e.row, nil
}

func (c *rowCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		return
	}
	c.cache.Remove(key)
}
