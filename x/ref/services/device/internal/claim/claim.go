// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package claim

import (
	"crypto/subtle"
	"os"
	"sync"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/device"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/internal/pathperms"
)

// NewClaimableDispatcher returns an rpc.Dispatcher that allows the device to
// be Claimed if it hasn't been already and a channel that will be closed once
// the device has been claimed.
//
// It returns (nil, nil) if the device is no longer claimable.
func NewClaimableDispatcher(ctx *context.T, permsDir, pairingToken string, auth security.Authorizer) (rpc.Dispatcher, <-chan struct{}) {
	permsStore := pathperms.NewPathStore(ctx)
	if _, _, err := permsStore.Get(permsDir); !os.IsNotExist(err) {
		// The device is claimable only if Claim hasn't been called before. The
		// existence of the Permissions file is an indication of a successful prior
		// call to Claim.
		return nil, nil
	}
	notify := make(chan struct{})
	return &claimable{token: pairingToken, permsStore: permsStore, permsDir: permsDir, notify: notify, auth: auth}, notify
}

// claimable implements the device.Claimable RPC interface and the
// rpc.Dispatcher and security.Authorizer to serve it.
//
// It allows the Claim RPC to be successfully invoked exactly once.
type claimable struct {
	token      string
	permsStore *pathperms.PathStore
	permsDir   string
	notify     chan struct{} // GUARDED_BY(mu)
	auth       security.Authorizer

	// Lock used to ensure that a successful claim can happen at most once.
	// This is done by allowing only a single goroutine to execute the
	// meaty parts of Claim at a time.
	mu sync.Mutex
}

func (c *claimable) Claim(ctx *context.T, call rpc.ServerCall, pairingToken string) error {
	// Verify that the claimer pairing tokens match that of the device manager.
	if subtle.ConstantTimeCompare([]byte(pairingToken), []byte(c.token)) != 1 {
		return verror.New(errors.ErrInvalidPairingToken, ctx)
	}
	var (
		granted   = call.GrantedBlessings() // blessings granted by the claimant
		principal = v23.GetPrincipal(ctx)
		store     = principal.BlessingStore()
	)
	if granted.IsZero() {
		return verror.New(errors.ErrInvalidBlessing, ctx)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.notify == nil {
		// Device has already been claimed (by a concurrent
		// RPC perhaps), it cannot be reclaimed
		return verror.New(errors.ErrDeviceAlreadyClaimed, ctx)
	}
	// TODO(ashankar): If the claim fails, would make sense
	// to remove from roots as well.
	if err := security.AddToRoots(principal, granted); err != nil {
		return verror.New(errors.ErrInvalidBlessing, ctx)
	}
	if _, err := store.Set(granted, security.AllPrincipals); err != nil {
		return verror.New(errors.ErrInvalidBlessing, ctx, err)
	}
	if err := store.SetDefault(granted); err != nil {
		return verror.New(errors.ErrInvalidBlessing, ctx, err)
	}

	// Create Permissions with all the granted blessings (which are now the default blessings)
	// (irrespective of caveats).
	patterns := security.DefaultBlessingPatterns(principal)
	if len(patterns) == 0 {
		return verror.New(errors.ErrInvalidBlessing, ctx)
	}

	// Create Permissions that allow principals with the caller's blessings to
	// administer and use the device.
	perms := make(access.Permissions)
	for _, bp := range patterns {
		// TODO(caprita,ataly,ashankar): Do we really need the
		// NonExtendable restriction below?
		patterns := bp.MakeNonExtendable().PrefixPatterns()
		for _, p := range patterns {
			for _, tag := range access.AllTypicalTags() {
				perms.Add(p, string(tag))
			}
		}
	}
	if err := c.permsStore.Set(c.permsDir, perms, ""); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx)
	}
	ctx.Infof("Device claimed and Permissions set to: %v", perms)
	close(c.notify)
	c.notify = nil
	return nil
}

// TODO(ashankar): Remove this and use Serve instead of ServeDispatcher to setup
// the Claiming service. Shouldn't need the "device" suffix.
func (c *claimable) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if suffix != "" && suffix != "device" {
		return nil, nil, verror.New(errors.ErrUnclaimedDevice, nil)
	}
	return device.ClaimableServer(c), c.auth, nil
}
