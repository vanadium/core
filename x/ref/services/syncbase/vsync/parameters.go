// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import "time"

var (
	// peerSyncInterval is the duration between two consecutive peer
	// contacts. During every peer contact, the initiator obtains any
	// pending updates from that peer.
	peerSyncInterval = 50 * time.Millisecond

	// channelTimeout is the duration at which health checks will be requested on
	// connections to peers.
	channelTimeout = 2 * time.Second

	// NeighborConnectionTimeout is the time duration we wait for a connection to be
	// established with a peer discovered from the neighborhood.
	//
	// TODO(suharshs): Make the timeouts below more dynamic based on network. (i.e.
	// bt vs tcp). Currently bt connection establishment takes ~3 seconds, so
	// neighborhood connections get more time than cloud connections.
	NeighborConnectionTimeout = 5 * time.Second

	// cloudConnectionTimeout is the duration we wait for a connection to be
	// established with a cloud peer.
	cloudConnectionTimeout = 2 * time.Second

	// syncConnectionTimeout is the duration we wait for syncing to begin with a
	// peer. A value of 0 means we will only use connections that exist in our
	// cache.
	//
	// TODO(hpucha): Make sync connection timeout dynamic based on ping latency.
	// E.g. perhaps we should use a 2s ping timeout, and use k*pingLatency (for
	// some k) as getDeltas timeout
	syncConnectionTimeout = 0

	// memberViewTTL is the shelf-life of the aggregate view of syncgroup members.
	memberViewTTL = 2 * time.Second

	// pingFailuresCap is the number of failures to cap off at when
	// computing how many peer manager rounds to wait before retrying to
	// ping peers via the mount table. This implies that a peer manager will
	// backoff a maximum of (2^pingFailuresCap) rounds or
	// (2^pingFailuresCap)*peerManagementInterval in terms of duration.
	pingFailuresCap uint64 = 6

	// pingFanout is the maximum number of peers that can be pinged in
	// parallel.  TODO(hpucha): support ping fanout of 0.
	pingFanout = 10

	// peerManagementInterval is the duration between two rounds of peer
	// management actions.
	peerManagementInterval = peerSyncInterval / 5

	// maxLocationsInSignpost is the maximum number of Locations that
	// mergeSignposts() will permit in a Signpost.  Locations with more
	// recent WhenSeen values are kept preferentially.  Also, locateBlob
	// considers only the first maxLocationsInSignpost entries in a
	// Signpost's Locations.
	maxLocationsInSignpost = 4

	// blobRecencyTimeSlop:  Otherwise equivalent devices that last
	// synchronized with a server within blobRecencyTimeSlop are considered
	// to have equal sync time for the purposes of blob priority.
	blobRecencyTimeSlop = 2 * time.Minute

	// blobSyncDistanceDecay is used as the decay constant when computing
	// the mean sync hops from a server to the device, when computing blob priority.
	// Higher values mean more inertia.
	blobSyncDistanceDecay = float32(5)

	// initialBlobOwnerShares is the initial number of ownership shares that
	// a device gives itself when it introduces a BlobRef to a syncgroup.
	initialBlobOwnerShares = int32(2)
)
