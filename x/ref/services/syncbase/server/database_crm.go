// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

////////////////////////////////////////
// ConflictManager RPC methods

func (d *database) StartConflictResolver(ctx *context.T, call wire.ConflictManagerStartConflictResolverServerCall) error {
	if !d.exists {
		return verror.New(verror.ErrNoExist, ctx, d.id)
	}

	// Store the conflict resolver connection in the per-database singleton
	// so that sync can access it.
	vlog.VI(2).Infof("cr: StartConflictResolver: resolution stream established")

	d.crMu.Lock()
	d.crStream = call
	d.crMu.Unlock()

	// In order to keep the CrStream alive, we must block here until the context
	// for this RPC is cancelled or closed.
	<-ctx.Done()

	// The channel is closed. Remove the crStream instance from cache.
	// NOTE: Any code that accesses crStream must make a copy of the pointer
	// before using it to make sure that the value does not suddenly become nil
	// during their processing.
	d.crMu.Lock()
	d.crStream = nil
	d.crMu.Unlock()
	return nil
}
