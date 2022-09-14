// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"v.io/v23/security"
)

// inCache keeps track of incoming blessings, discharges, and keys.
type inCache struct {
	blessings  blessingsCache // indexed by bkey-1
	discharges dischargeCache // indexed by dkey-1
	dkeys      keyCache       // dkey of the latest discharges, indexed by bkey-1.
}

type keyCache []uint64

func (c keyCache) extend(key uint64) keyCache {
	if needed := int(key); needed > len(c) {
		tmp := make([]uint64, needed)
		copy(tmp, c)
		return tmp
	}
	return c
}

func (c keyCache) has(key uint64) (uint64, bool) {
	if int(key) < len(c) {
		b := c[key]
		return b, b != 0
	}
	return 0, false
}

type blessingsCache []security.Blessings

func (c blessingsCache) extend(key uint64) blessingsCache {
	if needed := int(key); needed > len(c) {
		tmp := make([]security.Blessings, needed)
		copy(tmp, c)
		return tmp
	}
	return c
}

func (c blessingsCache) has(key uint64) (security.Blessings, bool) {
	if int(key) < len(c) {
		b := c[key]
		return b, !b.IsZero()
	}
	return security.Blessings{}, false
}

type dischargeCache [][]security.Discharge

func (c dischargeCache) extend(key uint64) dischargeCache {
	if needed := int(key); needed > len(c) {
		tmp := make([][]security.Discharge, needed)
		copy(tmp, c)
		return tmp
	}
	return c
}

func (c dischargeCache) has(key uint64) ([]security.Discharge, bool) {
	if int(key) < len(c) {
		b := c[key]
		return b, b != nil
	}
	return nil, false
}

func (c *inCache) addBlessings(bkey uint64, blessings security.Blessings) {
	c.blessings = c.blessings.extend(bkey)
	c.blessings[bkey-1] = blessings
}

func (c *inCache) hasBlessings(bkey uint64) (security.Blessings, bool) {
	return c.blessings.has(bkey)
}

func (c *inCache) addDischarges(bkey, dkey uint64, discharges []security.Discharge) {
	c.discharges = c.discharges.extend(dkey)
	c.discharges[dkey-1] = discharges
	c.dkeys = c.dkeys.extend(bkey)
	c.dkeys[bkey-1] = dkey
}

func (c *inCache) hasDischarges(dkey uint64) ([]security.Discharge, bool) {
	return c.discharges.has(dkey)
}

// outCache keeps track of outgoing blessings, discharges, and keys.
type outCache struct {
	bkeys uidCache // blessings uid -> bkey
	dkeys keyCache // blessings bkey -> dkey of latest discharges

	blessings  blessingsCache // indexed by bkey
	discharges dischargeCache // indexed by dkey

	/*bkeys map[string]uint64 // blessings uid -> bkey
	dkeys      map[uint64]uint64      // blessings bkey -> dkey of latest discharges
	blessings  []security.Blessings   // keyed by bkey
	discharges [][]security.Discharge // keyed by dkey*/
}

type uidCache []string

func (c uidCache) extend(key string) uidCache {
	if needed := int(key); needed > len(c) {
		tmp := make([]string, needed)
		copy(tmp, c)
		return tmp
	}
	return c
}

func (c uidCache) has(key uint64) (uint64, bool) {
	if int(key) < len(c) {
		b := c[key]
		return b, b != 0
	}
	return 0, false
}

func (c *outCache) addBlessings(buid string, bkey uint64) {
	if needed := int(bkey) + 1; needed > len(c.bkeys) {
		tmp := make([]string, needed)
		copy(tmp, c)
		return tmp
	}

}

func (c *outCache) hasBlessings(buid string) (uint64, bool) {
	for i, v := range c.bkeys {
		if v == buid {
			return uint64(i), true
		}
	}
	return 0, false
}

/*
func newOutCache() *outCache {
	return &outCache{
		bkeys:      make(map[string]uint64),
		dkeys:      make(map[uint64]uint64),
		blessings:  make(map[uint64]security.Blessings),
		discharges: make(map[uint64][]security.Discharge),
	}
}*/
