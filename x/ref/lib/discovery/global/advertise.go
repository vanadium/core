// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package global

import (
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	idiscovery "v.io/x/ref/lib/discovery"
)

func (d *gdiscovery) Advertise(ctx *context.T, ad *discovery.Advertisement, visibility []security.BlessingPattern) (<-chan struct{}, error) {
	if !ad.Id.IsValid() {
		var err error
		if ad.Id, err = discovery.NewAdId(); err != nil {
			return nil, err
		}
	}
	if err := validateAd(ad); err != nil {
		return nil, idiscovery.ErrorfBadAdvertisement(ctx, "invalid advertisement: %v", err)
	}
	if len(visibility) == 0 {
		visibility = []security.BlessingPattern{security.AllPrincipals}
	}

	principal := v23.GetPrincipal(ctx)
	self := security.DefaultBlessingPatterns(principal)
	perms := access.Permissions{
		string(access.Admin): access.AccessList{In: self},
		string(access.Read):  access.AccessList{In: visibility},
	}

	if !d.addAd(ad) {
		return nil, idiscovery.ErrorfAlreadyBeingAdvertised(ctx, "already being advertised: %v", ad.Id)
	}

	// TODO(jhahn): There is no atomic way to check and reserve the name under mounttable.
	// For example, the name can be overwritten by other applications of the same owner.
	// But this would be OK for now.
	name, err := encodeAdToSuffix(ad, d.newAdTimestampNs())
	if err != nil {
		return nil, err
	}
	if err := d.ns.SetPermissions(ctx, name, perms, "", naming.IsLeaf(true)); err != nil {
		d.removeAd(ad)
		return nil, err
	}

	// TODO(jhahn): We're using one goroutine per advertisement, but we can do
	// better like have one goroutine that takes care of all advertisements.
	// But this is OK for now as an experiment.
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer d.removeAd(ad)
		// We need a context that is detached from the deadlines and cancellation
		// of ctx since we have to unmount after ctx is canceled.
		rctx, rcancel := context.WithRootCancel(ctx)
		defer d.unmount(rctx, rcancel, name)

		for {
			d.mount(ctx, name, ad.Addresses)

			select {
			case <-d.clock.After(d.mountTTL):
			case <-ctx.Done():
				return
			}
		}
	}()
	return done, nil
}

func (d *gdiscovery) newAdTimestampNs() int64 {
	now := time.Now()
	timestampNs := now.UnixNano()
	d.mu.Lock()
	if d.adTimestampNs >= timestampNs {
		timestampNs = d.adTimestampNs + 1
	}
	d.adTimestampNs = timestampNs
	d.mu.Unlock()
	return timestampNs
}

func (d *gdiscovery) addAd(ad *discovery.Advertisement) bool {
	d.mu.Lock()
	if _, exist := d.ads[ad.Id]; exist {
		d.mu.Unlock()
		return false
	}
	d.ads[ad.Id] = struct{}{}
	d.mu.Unlock()
	return true
}

func (d *gdiscovery) removeAd(ad *discovery.Advertisement) {
	d.mu.Lock()
	delete(d.ads, ad.Id)
	d.mu.Unlock()
}

func (d *gdiscovery) mount(ctx *context.T, name string, addrs []string) {
	for _, addr := range addrs {
		if err := d.ns.Mount(ctx, name, addr, d.mountTTLWithSlack); err != nil {
			ctx.Errorf("mount(%q, %q) failed: %v", name, addr, err)
		}
	}
}

func (d *gdiscovery) unmount(ctx *context.T, cancel context.CancelFunc, name string) {
	if err := d.ns.Delete(ctx, name, true); err != nil {
		ctx.Infof("unmount(%q) failed: %v", name, err)
	}
	cancel()
}
