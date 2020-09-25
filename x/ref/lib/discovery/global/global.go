// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(jhahn): This is an experimental work to see its feasibility and set
// the long-term goal, and can be changed without notice.
package global

import (
	"sync"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/x/ref/lib/timekeeper"
)

const (
	defaultMountTTL     = 120 * time.Second
	defaultScanInterval = 90 * time.Second

	minMountTTLSlack = 2 * time.Second
	maxMountTTLSlack = 10 * time.Second
)

type gdiscovery struct {
	ns namespace.T

	mu            sync.Mutex
	ads           map[discovery.AdId]struct{} // GUARDED_BY(mu)
	adTimestampNs int64                       // GUARDED_BY(mu)

	clock timekeeper.TimeKeeper

	mountTTL          time.Duration
	mountTTLWithSlack time.Duration
	scanInterval      time.Duration
}

// New returns a new global Discovery.T instance that uses the Vanadium namespace
// under 'path' with default mount ttl (120s) and scan interval (90s).
func New(ctx *context.T, path string) (discovery.T, error) {
	return NewWithTTL(ctx, path, 0, 0)
}

// New returns a new global Discovery.T instance that uses the Vanadium
// namespace under 'path'. If mountTTL or scanInterval is zero, the default
// values will be used.
func NewWithTTL(ctx *context.T, path string, mountTTL, scanInterval time.Duration) (discovery.T, error) {
	return newWithClock(ctx, path, mountTTL, scanInterval, timekeeper.RealTime())
}

func newWithClock(ctx *context.T, path string, mountTTL, scanInterval time.Duration, clock timekeeper.TimeKeeper) (discovery.T, error) {
	ns := v23.GetNamespace(ctx)
	if ns == nil {
		return nil, ErrorfNoNamespace(ctx, "namespace not found")
	}

	var roots []string
	if naming.Rooted(path) {
		roots = []string{path}
	} else {
		for _, root := range ns.Roots() {
			roots = append(roots, naming.Join(root, path))
		}
	}
	_, ns, err := v23.WithNewNamespace(ctx, roots...)
	if err != nil {
		return nil, err
	}
	ns.CacheCtl(naming.DisableCache(true))

	if mountTTL == 0 {
		mountTTL = defaultMountTTL
	}
	if scanInterval == 0 {
		scanInterval = defaultScanInterval
	}
	mountTTLSlack := time.Duration(mountTTL.Nanoseconds() / 10)
	if mountTTLSlack < minMountTTLSlack {
		mountTTLSlack = minMountTTLSlack
	} else if mountTTLSlack > maxMountTTLSlack {
		mountTTLSlack = maxMountTTLSlack
	}

	d := &gdiscovery{
		ns:                ns,
		ads:               make(map[discovery.AdId]struct{}),
		clock:             clock,
		mountTTL:          mountTTL,
		mountTTLWithSlack: mountTTL + mountTTLSlack,
		scanInterval:      scanInterval,
	}
	return d, nil
}
