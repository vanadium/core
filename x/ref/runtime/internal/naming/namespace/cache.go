// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"math/rand"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
)

// maxCacheEntries is the max number of cache entries to keep.  It exists only so that we
// can avoid edge cases blowing us up the cache.
const maxCacheEntries = 4000

// cacheHisteresisSize is how much we back off to if the cache gets filled up.
const cacheHisteresisSize = (3 * maxCacheEntries) / 4

// cache is a generic interface to the resolution cache.
type cache interface {
	remember(ctx *context.T, prefix string, entry *naming.MountEntry)
	forget(ctx *context.T, names []string)
	lookup(ctx *context.T, name string) (naming.MountEntry, error)
	isNotMT(s string) bool
	setNotMT(s string)
}

// ttlCache is an instance of cache that obeys ttl from the mount points.
type ttlCache struct {
	sync.Mutex
	entries map[string]naming.MountEntry
	notMT   map[string]time.Time
}

// newTTLCache creates an empty ttlCache.
func newTTLCache() cache {
	return &ttlCache{entries: make(map[string]naming.MountEntry), notMT: make(map[string]time.Time)}
}

func isStale(now time.Time, e naming.MountEntry) bool {
	for _, s := range e.Servers {
		if s.Deadline.Before(now) {
			return true
		}
	}
	return false
}

// randomDrop randomly removes one cache entry.  Assumes we've already locked the cache.
func (c *ttlCache) randomDrop() {
	n := rand.Intn(len(c.entries))
	for k := range c.entries {
		if n == 0 {
			delete(c.entries, k)
			break
		}
		n--
	}
}

// cleaner reduces the number of entries.  Assumes we've already locked the cache.
func (c *ttlCache) cleaner() {
	// First dump any stale entries.
	now := time.Now()
	for k, v := range c.entries {
		if len(c.entries) < cacheHisteresisSize {
			return
		}
		if isStale(now, v) {
			delete(c.entries, k)
		}
	}

	// If we haven't gotten low enough, dump randomly.
	for len(c.entries) >= cacheHisteresisSize {
		c.randomDrop()
	}
}

// remember the servers associated with name with suffix removed.
func (c *ttlCache) remember(ctx *context.T, prefix string, entry *naming.MountEntry) {
	// Remove suffix.  We only care about the name that gets us
	// to the mounttable from the last mounttable.
	prefix = naming.Clean(prefix)
	entry.Name = naming.Clean(entry.Name)
	prefix = naming.TrimSuffix(prefix, entry.Name)
	// Copy the entry.
	var ce naming.MountEntry
	ce.Servers = append(ce.Servers, entry.Servers...)
	ce.ServesMountTable = entry.ServesMountTable
	c.Lock()
	// Enforce an upper limit on the cache size.
	if len(c.entries) >= maxCacheEntries {
		if _, ok := c.entries[prefix]; !ok {
			c.cleaner()
		}
	}
	c.entries[prefix] = ce
	c.Unlock()
}

// forget cache entries whose index begins with an element of names.  If names is nil
// forget all cached entries.
func (c *ttlCache) forget(ctx *context.T, names []string) {
	c.Lock()
	defer c.Unlock()
	for key := range c.entries {
		for _, n := range names {
			n = naming.Clean(n)
			if strings.HasPrefix(key, n) {
				delete(c.entries, key)
				break
			}
		}
	}
}

// lookup searches the cache for a maximal prefix of name and returns the associated servers,
// prefix, and suffix.  If any of the associated servers is expired, don't return anything
// since that would reduce availability.
func (c *ttlCache) lookup(ctx *context.T, name string) (naming.MountEntry, error) {
	name = naming.Clean(name)
	c.Lock()
	defer c.Unlock()
	now := time.Now()
	for prefix, suffix := name, ""; len(prefix) > 0; prefix, suffix = backup(prefix, suffix) {
		e, ok := c.entries[prefix]
		if !ok {
			continue
		}
		if isStale(now, e) {
			return e, naming.ErrNoSuchName.Errorf(ctx, "name %v doesn't exist", name)
		}
		ctx.VI(2).Infof("namespace cache %s -> %v %s", name, e.Servers, e.Name)
		e.Name = suffix
		if l := len(e.Servers); l > 1 {
			// Rotate returned servers to provide some degree of load balancing.
			// The rotated set is stored and returned on the next invocation
			// of lookup.
			tmp := e
			tmp.Servers = append(tmp.Servers[1:], e.Servers[0])
			c.entries[prefix] = tmp
		}
		return e, nil
	}
	return naming.MountEntry{}, naming.ErrNoSuchName.Errorf(ctx, "name %v doesn't exist", name)
}

// setNotMT caches the fact that a server as not a mounttable.
func (c *ttlCache) setNotMT(s string) {
	c.Lock()
	defer c.Unlock()
	// Don't set if this is an endpoint since the endpoint contains the
	// mounttable attribute and we should not override it.
	//
	// While looking for "@@" is not definitive for an endpoint containing
	// the mounttable attribute, it is only incorrect for older version
	// endpoints which should be rare.  This is just an optimization and
	// not performing it is not an error.
	if strings.Contains(s, "@@") {
		return
	}
	// Set it for a minute.  This should be long enough to cut down on the
	// extra resolutions without preserving mistakes.
	c.notMT[s] = time.Now().Add(time.Minute)
}

// isNotMT looks in the cache to see if the server has been found to not be
// a mounttable.
func (c *ttlCache) isNotMT(s string) bool {
	c.Lock()
	defer c.Unlock()
	if strings.Contains(s, "@@") {
		return false
	}
	if expires, ok := c.notMT[s]; ok {
		if !time.Now().After(expires) {
			return true
		}
		delete(c.notMT, s)
	}
	return false
}

// backup moves the last element of the prefix to the suffix.
func backup(prefix, suffix string) (string, string) {
	for i := len(prefix) - 1; i > 0; i-- {
		if prefix[i] != '/' {
			continue
		}
		suffix = naming.Join(prefix[i+1:], suffix)
		prefix = prefix[:i]
		return prefix, suffix
	}
	return "", naming.Join(prefix, suffix)
}

// nullCache is an instance of cache that does nothing.
type nullCache int

func newNullCache() cache                                                          { return nullCache(1) }
func (nullCache) remember(ctx *context.T, prefix string, entry *naming.MountEntry) {}
func (nullCache) forget(ctx *context.T, names []string)                            {}
func (nullCache) lookup(ctx *context.T, name string) (e naming.MountEntry, err error) {
	return e, naming.ErrNoSuchName.Errorf(ctx, "name %s doesn't exist", name)
}
func (nullCache) isNotMT(s string) bool { return false }
func (nullCache) setNotMT(s string)     {}

func newCache(disabled bool) cache {
	if disabled {
		return newNullCache()
	}
	return newTTLCache()
}
