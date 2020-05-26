// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testutil defines helpers for discovery tests.
package testutil

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/bcrypter"
	"v.io/x/ref/test/testutil"
)

// WithPrincipal creates a new principal with an extension of the default
// blessing of the principal in the returned context.
func WithPrincipal(ctx *context.T, extension string) (*context.T, error) {
	idp := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	ctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal())
	if err != nil {
		return nil, err
	}
	if err := idp.Bless(v23.GetPrincipal(ctx), extension); err != nil {
		return nil, err
	}
	return ctx, nil
}

// WithPrivateKey creates a new principal with the provided blessing and
// stores the private key of the blessing in the returned context.
func WithPrivateKey(ctx *context.T, root *bcrypter.Root, blessing string) (*context.T, error) {
	ctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal(blessing))
	if err != nil {
		return nil, err
	}
	key, err := root.Extract(ctx, blessing)
	if err != nil {
		return nil, err
	}
	crypter := bcrypter.NewCrypter()
	ctx = bcrypter.WithCrypter(ctx, crypter)
	if err := crypter.AddKey(ctx, key); err != nil {
		return nil, err
	}
	return ctx, nil
}

func Advertise(ctx *context.T, d discovery.T, visibility []security.BlessingPattern, ads ...*discovery.Advertisement) (func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	dones := make([]<-chan struct{}, 0, len(ads))
	for _, ad := range ads {
		done, err := d.Advertise(ctx, ad, visibility)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("Advertise failed: %v", err)
		}
		dones = append(dones, done)
	}
	stop := func() {
		cancel()
		for _, done := range dones {
			<-done
		}
	}
	return stop, nil
}

func Scan(ctx *context.T, d discovery.T, query string) (<-chan discovery.Update, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	scanCh, err := d.Scan(ctx, query)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("Scan failed: %v", err)
	}
	stop := func() {
		cancel()
		for range scanCh {
		}
	}
	return scanCh, stop, err
}

func ScanAndMatch(ctx *context.T, d discovery.T, query string, wants ...discovery.Advertisement) error {
	const timeout = 3 * time.Minute

	var updates []discovery.Update
	for now := time.Now(); time.Since(now) < timeout; {
		var err error
		updates, err = doScan(ctx, d, query, len(wants))
		if err != nil {
			return err
		}
		if MatchFound(ctx, updates, wants...) {
			return nil
		}

		runtime.Gosched()
	}
	return fmt.Errorf("Match failed; got %v, but wanted %v", updates, wants)
}

func doScan(ctx *context.T, d discovery.T, query string, expectedUpdates int) ([]discovery.Update, error) {
	scanCh, stop, err := Scan(ctx, d, query)
	if err != nil {
		return nil, err
	}
	defer stop()

	updates := make([]discovery.Update, 0, expectedUpdates)
	for {
		var timer <-chan time.Time
		if len(updates) >= expectedUpdates {
			timer = time.After(5 * time.Millisecond)
		}

		select {
		case update := <-scanCh:
			updates = append(updates, update)
		case <-timer:
			return updates, nil
		}
	}
}

func MatchFound(ctx *context.T, updates []discovery.Update, wants ...discovery.Advertisement) bool {
	return match(ctx, updates, false, wants...)
}

func MatchLost(ctx *context.T, updates []discovery.Update, wants ...discovery.Advertisement) bool {
	return match(ctx, updates, true, wants...)
}

func match(ctx *context.T, updates []discovery.Update, lost bool, wants ...discovery.Advertisement) bool {
	updateMap := make(map[discovery.AdId]discovery.Update)
	for _, update := range updates {
		updateMap[update.Id()] = update
	}

	for _, want := range wants {
		update := updateMap[want.Id]
		if update == nil {
			return false
		}
		if update.IsLost() != lost {
			return false
		}
		if !UpdateEqual(ctx, update, want) {
			return false
		}
		delete(updateMap, update.Id())
	}
	return len(updateMap) == 0
}

func UpdateEqual(ctx *context.T, update discovery.Update, ad discovery.Advertisement) bool {
	if update.Id() != ad.Id {
		return false
	}
	if update.InterfaceName() != ad.InterfaceName {
		return false
	}
	// Not that addresses may be in different orders.
	a1 := append([]string(nil), update.Addresses()...)
	a2 := append([]string(nil), ad.Addresses...)
	sort.Strings(a1)
	sort.Strings(a2)
	if !reflect.DeepEqual(a1, a2) {
		return false
	}
	for k, v := range ad.Attributes {
		if update.Attribute(k) != v {
			return false
		}
	}
	for k, v := range ad.Attachments {
		r, ok := <-update.Attachment(ctx, k)
		if !ok {
			return false
		}
		if r.Error != nil {
			return false
		}
		if !bytes.Equal(r.Data, v) {
			return false
		}
	}
	return true
}
