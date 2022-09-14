// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"v.io/v23/security"
)

type cachedBlessing struct {
	bkey      uint64
	blessings security.Blessings
}

type cachedDischarge struct {
	discharges []security.Discharge
	dkey, bkey uint64
}

type cachedBlessingUID struct {
	blessings security.Blessings
	uid       string
	bkey      uint64
}

// inCache keeps track of incoming blessings, discharges, and keys.
type inCache struct {
	blessings  []cachedBlessing
	discharges []cachedDischarge
}

func (c *inCache) addBlessings(bkey uint64, blessings security.Blessings) {
	c.blessings = append(c.blessings, cachedBlessing{bkey: bkey, blessings: blessings})
}

func (c *inCache) hasBlessings(bkey uint64) (security.Blessings, bool) {
	for _, v := range c.blessings {
		if v.bkey == bkey {
			return v.blessings, true
		}
	}
	return security.Blessings{}, false
}

func (c *inCache) addDischarges(bkey, dkey uint64, discharges []security.Discharge) {
	c.discharges = append(c.discharges, cachedDischarge{dkey: dkey, bkey: bkey, discharges: discharges})
}

func (c *inCache) hasDischarges(dkey uint64) ([]security.Discharge, uint64, bool) {
	for _, v := range c.discharges {
		if v.dkey == dkey {
			return v.discharges, v.bkey, true
		}
	}
	return nil, 0, false
}

// outCache keeps track of outgoing blessings, discharges, and keys.
type outCache struct {
	blessings  []cachedBlessingUID
	discharges []cachedDischarge
}

func (c *outCache) addBlessings(uid string, bkey uint64, blessings security.Blessings) {
	c.blessings = append(c.blessings, cachedBlessingUID{uid: uid, bkey: bkey, blessings: blessings})
}

func (c *outCache) hasBlessings(uid string) (uint64, bool) {
	for _, v := range c.blessings {
		if v.uid == uid {
			return v.bkey, true
		}
	}
	return 0, false
}

func (c *outCache) hasDischarges(bkey uint64) ([]security.Discharge, uint64, bool) {
	for _, v := range c.discharges {
		if v.bkey == bkey {
			return v.discharges, v.dkey, true
		}
	}
	return nil, 0, false
}

func (c *outCache) addDischarges(bkey, dkey uint64, discharges []security.Discharge) {
	c.discharges = append(c.discharges, cachedDischarge{dkey: dkey, bkey: bkey, discharges: discharges})
}
