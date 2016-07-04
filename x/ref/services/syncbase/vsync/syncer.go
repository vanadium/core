// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

// syncer wakes up every peerSyncInterval to do work: (1) Refresh memberView if
// needed and pick a peer from all the known remote peers to sync with. (2) Act
// as an initiator and sync syncgroup metadata for all common syncgroups with
// the chosen peer (getting updates from the remote peer, detecting and
// resolving conflicts). (3) Act as an initiator and sync data corresponding to
// all common syncgroups across all Databases with the chosen peer. (4) Fetch
// any queued blobs. (5) Transfer ownership of blobs if needed. (6) Act as a
// syncgroup publisher to publish pending syncgroups. (6) Garbage collect older
// generations.
//
// TODO(hpucha): Currently only does initiation. Add rest.
func (s *syncService) syncer(ctx *context.T) {
	defer s.pending.Done()

	ticker := time.NewTicker(peerSyncInterval)
	defer ticker.Stop()

	for !s.isClosed() {
		select {
		case <-ticker.C:
			if s.isClosed() {
				break
			}
			s.syncerWork(ctx)

		case <-s.closed:
			break
		}
	}
	vlog.VI(1).Info("sync: syncer: channel closed, stop work and exit")
}

func (s *syncService) syncerWork(ctx *context.T) {
	// TODO(hpucha): Cut a gen for the responder even if there is no
	// one to initiate to?

	// Do work.
	attemptTs := time.Now()

	// TODO(hpucha): Change the peer passed into initiator and clock to be
	// just one name since we now have a parallel ping stage.
	peer, err := s.pm.pickPeer(ctx)
	if err != nil {
		return
	}

	err = s.syncVClock(ctx, peer)
	// Abort syncing if there is a connection error with peer.
	if verror.ErrorID(err) != interfaces.ErrConnFail.ID {
		err = s.getDeltasFromPeer(ctx, peer)
	}

	s.pm.updatePeerFromSyncer(ctx, peer, attemptTs, verror.ErrorID(err) == interfaces.ErrConnFail.ID)
}
