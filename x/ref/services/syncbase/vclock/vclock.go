// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

// This file defines the VClock struct and methods.

import (
	"sync"
	"time"

	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
)

// VClock holds everything needed to provide UTC time estimates.
// VClock is thread-safe.
type VClock struct {
	// Syncbase top-level store (i.e. not a database-specific store).
	st store.Store
	// Protects sysClock. We use RWMutex because writes can never happen in
	// production: they only happen on calls to InjectFakeSysClock, which can only
	// be called in development mode.
	mu       sync.RWMutex
	sysClock SystemClock
}

// NewVClock creates a new VClock.
func NewVClock(st store.Store) *VClock {
	return &VClock{
		st:       st,
		sysClock: newRealSystemClock(),
	}
}

////////////////////////////////////////////////////////////////////////////////
// Methods for determining the time

// TODO(sadovsky): Cache VClockData in memory, protected by a lock to ensure
// cache consistency. That way, "Now" won't need to return an error and
// "ApplySkew" can stop taking VClockData as input.

// Now returns our estimate of current UTC time based on SystemClock time and
// our estimate of its skew relative to NTP time.
func (c *VClock) Now() (time.Time, error) {
	data := &VClockData{}
	if err := c.GetVClockData(data); err != nil {
		vlog.Errorf("vclock: error fetching VClockData: %v", err)
		return time.Time{}, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ApplySkew(c.sysClock.Now(), data), nil
}

// ApplySkew applies skew correction to the given system clock time.
func (c *VClock) ApplySkew(sysTime time.Time, data *VClockData) time.Time {
	return sysTime.Add(time.Duration(data.Skew))
}

////////////////////////////////////////////////////////////////////////////////
// Methods for initializing, reading, and updating VClockData in the store

const vclockDataKey = common.VClockPrefix

// GetVClockData fills 'data' with VClockData read from the store.
func (c *VClock) GetVClockData(data *VClockData) error {
	return store.Get(nil, c.st, vclockDataKey, data)
}

// InitVClockData initializes VClockData in the store (if needed).
func (c *VClock) InitVClockData() error {
	return store.RunInTransaction(c.st, func(tx store.Transaction) error {
		if err := store.Get(nil, tx, vclockDataKey, &VClockData{}); err == nil {
			return nil
		} else if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return err
		}
		// No existing VClockData; write fresh VClockData.
		now, elapsedTime, err := c.SysClockVals()
		if err != nil {
			return err
		}
		return store.Put(nil, tx, vclockDataKey, &VClockData{
			SystemTimeAtBoot:     now.Add(-elapsedTime),
			ElapsedTimeSinceBoot: elapsedTime,
		})
	})
}

// UpdateVClockData reads VClockData from the store, applies the given function
// to produce new VClockData, and writes the resulting VClockData back to the
// store. The entire read-modify-write operation is performed inside of a
// transaction.
func (c *VClock) UpdateVClockData(fn func(*VClockData) (*VClockData, error)) error {
	return store.RunInTransactionWithOpts(c.st, &store.TransactionOptions{NumAttempts: 1}, func(tx store.Transaction) error {
		data := &VClockData{}
		if err := store.Get(nil, tx, vclockDataKey, data); err != nil {
			return err
		}
		if newVClockData, err := fn(data); err != nil {
			return err
		} else {
			return store.Put(nil, tx, vclockDataKey, newVClockData)
		}
	})
}

////////////////////////////////////////////////////////////////////////////////
// Methods for accessing system clock time

// SysClockVals returns the system clock's Now and ElapsedTime values.
func (c *VClock) SysClockVals() (time.Time, time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if elapsedTime, err := c.sysClock.ElapsedTime(); err != nil {
		return time.Time{}, 0, err
	} else {
		return c.sysClock.Now(), elapsedTime, nil
	}
}

// SysClockValsIfNotChanged returns the system clock's Now and ElapsedTime
// values, but returns an error if it detects that the system clock has changed
// based on the given origNow and origElapsedTime.
// IMPORTANT: origNow and origElapsedTime must have come from a call to
// SysClockVals, not SysClockValsIfNotChanged, to avoid a race condition in
// system clock change detection.
func (c *VClock) SysClockValsIfNotChanged(origNow time.Time, origElapsedTime time.Duration) (time.Time, time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := c.sysClock.Now()
	if elapsedTime, err := c.sysClock.ElapsedTime(); err != nil {
		return time.Time{}, 0, err
	} else if hasSysClockChanged(origNow, now, origElapsedTime, elapsedTime) {
		return time.Time{}, 0, err
	} else {
		return now, elapsedTime, nil
	}
}

////////////////////////////////////////////////////////////////////////////////
// Development mode methods

func (c *VClock) InjectFakeSysClock(now time.Time, elapsedTime time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sysClock = newFakeSystemClock(now, elapsedTime)
}
