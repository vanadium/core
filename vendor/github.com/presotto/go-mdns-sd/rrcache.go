// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mdns

// A cache of DNS RRs (resource records).

import (
	"log"
	"reflect"
	"time"

	"github.com/presotto/go-mdns-sd/go_dns"
)

type rrCacheEntry struct {
	expires time.Time
	rr      dns.RR
}

type rrCache struct {
	// The first key is the domain name and the second is the RR type
	cache map[string]map[uint16][]*rrCacheEntry

	logLevel int
}

// Create a new rr cache.  Make sure at least the top level map exists.
func newRRCache(logLevel int) *rrCache {
	rrcache := new(rrCache)
	rrcache.cache = make(map[string]map[uint16][]*rrCacheEntry, 0)
	rrcache.logLevel = logLevel
	return rrcache
}

// Add a resource record (RR) to the cache.
//
// In MDNS there are two types of RR sets, private ones that are only answered by a single machine and shared ones that
// are made up of responses from any machine. The most significant bit in the rrclass (can you say hack?) has been
// purloined as a cache flush bit.  If this bit is set this RR replaces all cached ones of the same type.
//
// Returns true if this entry was not already in the cache.
func (c *rrCache) Add(rr dns.RR) bool {
	// Create an entry for the domain name if none exists.
	dnmap, ok := c.cache[rr.Header().Name]
	if !ok {
		dnmap = make(map[uint16][]*rrCacheEntry, 0)
		c.cache[rr.Header().Name] = dnmap
	}

	// Remove all rr's matching this one's type if a cache flush is requested.
	if rr.Header().Class&0x8000 == 0x8000 {
		if c.logLevel >= 2 {
			log.Printf("cache flush for %v\n", rr)
		}
		dnmap[rr.Header().Rrtype] = make([]*rrCacheEntry, 0)
	}

	switch {
	case rr.Header().Ttl > 4500:
		// Don't believe TTLs greater than 75 minutes. Entries should refresh much faster than this.
		rr.Header().Ttl = 4500
	case rr.Header().Ttl == 0:
		// This is a goodbye packet. RFC 6762 specifies that queriers receiving a multicast DNS
		// response with a TTL of zero should record a TTL of 1 and then delete the record one
		// second later. This gives the other cooperating responders one second to rescue the
		// records when the goodbye packet was sent incorrectly.
		rr.Header().Ttl = 1
	}

	// Add absolute expiration time to the entry.
	entry := &rrCacheEntry{time.Now().Add(time.Duration(rr.Header().Ttl) * time.Second), rr}

	// If the slice doesn't exist yet, create it.
	rrslice, ok := dnmap[rr.Header().Rrtype]
	if !ok {
		dnmap[rr.Header().Rrtype] = make([]*rrCacheEntry, 0)
		rrslice = dnmap[rr.Header().Rrtype]
	}

	// If an existing cache rr has the same data fields, replace it.  Otherwise, just append.  We just
	// worry about a subset of rr types used by mdns.
	firstnil := -1
	for i := range rrslice {
		if rrslice[i] == nil {
			// When we take down entries with expired TTLs, we just nil out the pointer so we
			// remember the first nil'd pointer for subsequent reuse.
			if firstnil < 0 {
				firstnil = i
			}
			continue
		}
		same := false
		switch x := rr.(type) {
		case *dns.RR_A:
			y := rrslice[i].rr.(*dns.RR_A)
			if same = x.A == y.A; same {
				break
			}
		case *dns.RR_AAAA:
			y := rrslice[i].rr.(*dns.RR_AAAA)
			same = true
			for j := range x.AAAA {
				if x.AAAA[j] != y.AAAA[j] {
					same = false
					break
				}
			}
			if same {
				break
			}
		case *dns.RR_TXT:
			y := rrslice[i].rr.(*dns.RR_TXT)
			if same = reflect.DeepEqual(x.Txt, y.Txt); same {
				break
			}
		case *dns.RR_PTR:
			y := rrslice[i].rr.(*dns.RR_PTR)
			if same = x.Ptr == y.Ptr; same {
				break
			}
		case *dns.RR_SRV:
			y := rrslice[i].rr.(*dns.RR_SRV)
			if same = x.Priority == y.Priority && x.Weight == y.Weight && x.Port == y.Port && x.Target == y.Target; same {
				break
			}
		}
		if same {
			if c.logLevel >= 2 {
				log.Printf("replacing cached entry for %v with %v\n", rrslice[i].rr, rr)
			}
			rrslice[i] = entry
			return false
		}
	}
	// If we get to here, we have a new record.
	if firstnil >= 0 {
		// Fill in a hole.
		rrslice[firstnil] = entry
		if c.logLevel >= 2 {
			log.Printf("adding cached entry for %v (in a hole)\n", rr)
		}
	} else {
		// Append to the end of the list.
		dnmap[rr.Header().Rrtype] = append(rrslice, entry)
		if c.logLevel >= 2 {
			log.Printf("adding cached entry for %v (append)\n", rr)
		}
	}
	return true
}

// Send all RRs in entries to rc.  Ignore expired entries.
func sendRRs(entries []*rrCacheEntry, rc chan dns.RR) {
	now := time.Now()
	for _, e := range entries {
		if e == nil {
			continue
		}
		ttl := e.expires.Sub(now).Seconds()
		if ttl <= 0 {
			continue
		}
		e.rr.Header().Ttl = uint32(ttl)
		rc <- e.rr // Don't read this as err!
	}
}

// Lookup and Write to rc any cached RRs for name of the given rrtype.
//
// Note: it is up to the immediate caller to close rc.  This allows him to chain together
//	multiple calls to Lookup, directly feeding all the answers to his caller.
func (c *rrCache) Lookup(name string, rrtype uint16, rc chan dns.RR) {
	if dnmap, ok := c.cache[name]; ok {
		// TypeAll matches all RR types.
		if rrtype == dns.TypeALL {
			for _, entries := range dnmap {
				sendRRs(entries, rc)
			}
			return
		}

		// Otherwise, look for the specific type.
		if entries, ok := dnmap[rrtype]; ok {
			sendRRs(entries, rc)
		}
	}
}

// CleanExpired cleans out expired entries.  We run this occasionally to kill off entries that haven't been seen in a while.
func (c *rrCache) CleanExpired() []dns.RR {
	var expired []dns.RR
	now := time.Now()
	for _, dnmap := range c.cache {
		for _, entries := range dnmap {
			for i, e := range entries {
				if e == nil {
					continue
				}
				if now.After(e.expires) {
					// Nil out any expired entries, faster than rebuilding the slice.
					expired = append(expired, e.rr)
					entries[i] = nil
					continue
				}
			}
		}
	}
	return expired
}
