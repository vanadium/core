// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// This file contains blob handling code.
//
// A summary of inter-device blob management:
//
// Syncbase implements a system of "blob ownership" to reduce the
// probability that a blob will be lost due to the loss of a mobile
// device.  This works as follows.
//
// When a device that has a blob Put()s the blobref in an existing syncgroup,
// or makes a row containing a blobref syncable for the first time by creating
// a syncgroup, it assigns itself an initial number of "ownership shares" to
// the blob within that syncgroup.  Usually the initial number of shares per
// blob per syncgroup is 2.
//
// If a device does not have the blob, or associates the blobref with a
// syncgroup via the creation of an overlapping syncgroup, or via syncing,
// no ownership shares are assigned.  Instead, the blobref is allowed to
// propagate around the syncgroup until devices request
// the blob (note that servers always request blobs; see below).
//
// Any device with a non-zero number of ownership shares for a blob
// in a syncgroup has an obligation not to discard its copy of the
// blob, and not to reduce the number of ownership shares it has
// without first passing shares to another device within the
// syncgroup.  This obligation lapses if the blob ceases to be
// accessible via the syncgroup.
//
// When a device acquires a copy of a blob, it may elect to take on
// the burden of one or more ownership shares in each syncgroup in
// which the blob is accessible.  To decide whether to transfer a
// share for a syncgroup, devices compare their "ownership
// priorities" within the syncgroup.  The priorities are defined in
// terms of metrics suggestive of the "sync distance" of the device
// from the "servers" for the relevant syncgroup.
//
// A device that is marked as a "server" within the syncgroup is
// expected:
// - to acquire and keep copies of every blob accessible within the
//   syncgroup,
// - to make these copies available to other devices on demand, and
// - to use techniques such as replication and backup to make their
//   loss unlikely.
// Thus, servers have the highest ownership priority.  Other
// devices have lower ownership priorities according to their
// presumed ability to transfer the blob to the servers.  Each server
// periodically attempts to fetch any blob that it does not yet have that
// is referenced by the syncgroup.
//
// If a syncgroup has no servers, it is more likely that the loss of a device
// will lead to the loss of a blob, and that other devices will fail to find
// the blob should they request it.  This is because servers are more likely to
// be available and to store blobs reliably, and because the location hints
// work best when well-known servers exist.
//
// Each device computes its ownership priority by keeping track of various
// numbers in the SgPriority struct in
// v.io/x/ref/services/syncbase/server/interfaces/sync_types.vdl
// - DevType indicates the type of device.
// - Distance is the mean distance (in "hops") from the servers, maintained via
//   decaying average.
// - ServerTime is the time on a server at which data created there has reached
//   this device.  Times within timeSlop (syncgroup.go) are considered equivalent.
//
// Device types are manually assigned to devices, perhaps by default by the
// device manufacturer.  The set is
//    BlobDevTypeServer (accummulates blobs when possible, like a cloud server),
//    BlobDevTypeNormal,
//    BlobDevTypeLeaf (sheds blobs when possible, like a camera),
// defined in v.io/v23/services/syncbase/nosql/types.vdl
//
// Priorities can be compared via sgPriorityLowerThan() in syncgroup.go.
//
// Each time a device "local" receives data from a peer "remote", the remote
// device sends its Priority value, and the local device then adjusts its
// priority.  See updateSyncgroupPriority() in syncgroup.go
//
// The rules for transferring an ownership share when a device receives a copy
// of a blob are:
// - A server always sets its ownership shares to non-zero for any blob
//   accessible within its syncgroups.  That is, a server never
//   unilaterally deletes a reachable blob.
// - A non-server may transfer all its ownership shares to a server, even if
//   that server already has shares.  (It can choose to transfer fewer; if it
//   does so, the excess shares will likely be transferred in subsequent
//   communications.)
// - A non-server with at least one share may transfer one share to a
//   non-server with no shares and a higher priority.
//
// When a device syncs with a device to which it would transfer some ownership
// shares, it informs that device that it should take a copy of the blob and
// accept the share.  The recipient then tells the first device that it may
// decrement its share count.
//
// Example:  Suppose four devices are in a syncgroup:
// - a server S (BlobDevTypeServer)
// - a laptop L (BlobDevTypeNormal)
// - a tablet T (BlobDevTypeNormal)
// - a camera C that communicates with L and T, but not directly with S.
//   (BlobDevTypeLeaf)
// C's images are turned into blobs accessible via the syncgroup.
//
// When online, L and T can communicate directly with the server, so
// L.ServerTime and T.ServerTime will be recent, while L.Distance and
// T.Distance will both be close to 1.  C.ServerTime will be somewhat less
// recent, and C.Distance will be close to 2.  The priorities will satisfy:
// S > T > C and S > L > C.  Sometimes T > L and sometimes the reverse,
// depending on when they sync with S.
//
// If L, T, and C go offline (imagine taking a vacation cruise with them),
// their ServerTime values will remain unchanged from when they were last
// online.  One of L and T will have a higher priority; its Distance metric
// will remain constant, while the other two devices' distances will increase
// somewhat.  The relative priority ordering of the devices will then stay the
// same until contact is re-stablished with S.
//
// Suppose C takes a picture P giving it two shares, and syncs first with L.
// L will accept one share, and refuse to accept more while its share count is
// non-zero.  L's share may be taken by T or by S (if L is on-line).  L could
// then take C's second share.  Alternatively, C might transfer its second
// share to T, if T has not received a share from L. At this point, C is free
// to delete its copy of P, knowing that either S has the blob, or at least two
// other devices do.
//
//                              -----------
//
// An additional mechanism of location hints, called Signposts, exists to allow
// devices to find blobs whose blobrefs they have received.
//
// A Signpost consists of two lists:
// - a list of syncgroups whose data mention the blobref
// - a list of devices that have had the blob, or at least been instrumental
//   in forwarding its blobref.
// When a device receives a blobref, it constructs a Signpost that contains the
// syncgroups and peer that the blobref arrived through, plus the peer that
// inserted the blobref into the structured data (known through the log record)
//
// When a device requests a blob that the callee does not have, the callee may
// return its Signpost for the blob.  The caller then merges this into its own
// Signpost.  In this way, improved hints may propagate through the system.
// The list of devices in a Signpost is restricted in length so that Signposts
// do not become too large (see mergeSignposts()).  Ultimately, devices are
// expected to request blobs from the server machines within the relevant
// syncgroups.
//
// In transferring Signposts, devices filter the lists they send to avoid
// revealing private information about syncgroups (see filterSignpost()).  In
// particular, they avoid sending the name of a syncgroup to a peer that does
// not have permission to join the group unless the group is "public" (a
// property chosen at group creation).  Also, device location hints are given
// only to peers who have permission to join a syncgroup to which the device
// belongs, or if the device is a server in one of the syncgroups.  The
// expectation is that servers have no privacy constraints; many syncgroups and
// users may be associated with any given server.

import (
	"io"
	"sort"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	blob "v.io/x/ref/services/syncbase/localblobstore"
	_ "v.io/x/ref/services/syncbase/localblobstore/blobmap"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

const (
	chunkSize = 8 * 1024
)

////////////////////////////////////////////////////////////
// RPCs for managing blobs between Syncbase and its clients.

func (sd *syncDatabase) CreateBlob(ctx *context.T, call rpc.ServerCall) (wire.BlobRef, error) {
	vlog.VI(2).Infof("sync: CreateBlob: begin")
	defer vlog.VI(2).Infof("sync: CreateBlob: end")

	// Get this Syncbase's blob store handle.
	ss := sd.sync.(*syncService)
	bst := ss.bst

	writer, err := bst.NewBlobWriter(ctx, "")
	if err != nil {
		return wire.NullBlobRef, err
	}
	defer writer.CloseWithoutFinalize()

	name := writer.Name()
	vlog.VI(4).Infof("sync: CreateBlob: blob ref %s", name)
	return wire.BlobRef(name), nil
}

func (sd *syncDatabase) PutBlob(ctx *context.T, call wire.BlobManagerPutBlobServerCall, br wire.BlobRef) error {
	vlog.VI(2).Infof("sync: PutBlob: begin br %v", br)
	defer vlog.VI(2).Infof("sync: PutBlob: end br %v", br)

	// Get this Syncbase's blob store handle.
	ss := sd.sync.(*syncService)
	bst := ss.bst

	writer, err := bst.ResumeBlobWriter(ctx, string(br))
	if err != nil {
		return err
	}
	defer writer.CloseWithoutFinalize()

	stream := call.RecvStream()
	for stream.Advance() {
		item := blob.BlockOrFile{Block: stream.Value()}
		if err = writer.AppendBytes(item); err != nil {
			return err
		}
	}
	return stream.Err()
}

func (sd *syncDatabase) CommitBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	vlog.VI(2).Infof("sync: CommitBlob: begin br %v", br)
	defer vlog.VI(2).Infof("sync: CommitBlob: end br %v", br)

	// Get this Syncbase's blob store handle.
	ss := sd.sync.(*syncService)
	bst := ss.bst

	writer, err := bst.ResumeBlobWriter(ctx, string(br))
	if err != nil {
		return err
	}
	return writer.Close()
}

func (sd *syncDatabase) GetBlobSize(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) (int64, error) {
	vlog.VI(2).Infof("sync: GetBlobSize: begin br %v", br)
	defer vlog.VI(2).Infof("sync: GetBlobSize: end br %v", br)

	// Get this Syncbase's blob store handle.
	ss := sd.sync.(*syncService)
	bst := ss.bst

	reader, err := bst.NewBlobReader(ctx, string(br))
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	return reader.Size(), nil
}

func (sd *syncDatabase) DeleteBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	return verror.NewErrNotImplemented(ctx)
}

func (sd *syncDatabase) GetBlob(ctx *context.T, call wire.BlobManagerGetBlobServerCall, br wire.BlobRef, offset int64) error {
	vlog.VI(2).Infof("sync: GetBlob: begin br %v", br)
	defer vlog.VI(2).Infof("sync: GetBlob: end br %v", br)

	// First get the blob locally if available.
	ss := sd.sync.(*syncService)
	err := getLocalBlob(ctx, call.SendStream(), ss.bst, br, offset)
	if err == nil || verror.ErrorID(err) == wire.ErrBlobNotCommitted.ID {
		return err
	}

	return sd.fetchBlobRemote(ctx, br, nil, call, offset)
}

func (sd *syncDatabase) FetchBlob(ctx *context.T, call wire.BlobManagerFetchBlobServerCall, br wire.BlobRef, priority uint64) error {
	vlog.VI(2).Infof("sync: FetchBlob: begin br %v", br)
	defer vlog.VI(2).Infof("sync: FetchBlob: end br %v", br)

	clientStream := call.SendStream()

	// Check if BlobRef already exists locally.
	ss := sd.sync.(*syncService)
	bst := ss.bst

	bReader, err := bst.NewBlobReader(ctx, string(br))
	if err == nil {
		finalized := bReader.IsFinalized()
		bReader.Close()

		if !finalized {
			return wire.NewErrBlobNotCommitted(ctx)
		}
		clientStream.Send(wire.BlobFetchStatus{State: wire.BlobFetchStateDone})
		return nil
	}

	// Wait for this blob's turn.
	// TODO(hpucha): Implement a blob queue.
	clientStream.Send(wire.BlobFetchStatus{State: wire.BlobFetchStatePending})

	return sd.fetchBlobRemote(ctx, br, call, nil, 0)
}

func (sd *syncDatabase) PinBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	return verror.NewErrNotImplemented(ctx)
}

func (sd *syncDatabase) UnpinBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef) error {
	return verror.NewErrNotImplemented(ctx)
}

func (sd *syncDatabase) KeepBlob(ctx *context.T, call rpc.ServerCall, br wire.BlobRef, rank uint64) error {
	return verror.NewErrNotImplemented(ctx)
}

////////////////////////////////////////////////////////////
// RPC for blob fetch between Syncbases.

func (s *syncService) FetchBlob(ctx *context.T, call interfaces.SyncFetchBlobServerCall, br wire.BlobRef,
	remoteSgPriorities interfaces.SgPriorities) (sharesToTransfer interfaces.BlobSharesBySyncgroup, err error) {

	vlog.VI(2).Infof("sync: FetchBlob: sb-sb begin br %v", br)
	defer vlog.VI(2).Infof("sync: FetchBlob: sb-sb end br %v", br)

	err = getLocalBlob(ctx, call.SendStream(), s.bst, br, 0)
	if err == nil {
		// Compute how many shares in each syncgroup this syncbase should
		// request that the caller take from it.
		var blobMetadata blob.BlobMetadata
		// Start by computing the total shares this syncbase has in all
		// syncgroups.  We save time later if it has none.
		var totalShares int32
		if s.bst.GetBlobMetadata(ctx, br, &blobMetadata) == nil {
			for _, shares := range blobMetadata.OwnerShares {
				totalShares += shares
			}
		}
		if totalShares != 0 {
			// For each syncgroup, compute whether to transfer shares.
			// At present, we offer only one per syncgroup, unless
			// the caller is a server, and could take all of them.
			// No need to filter localSgPriorities explicitly; they
			// will be filtered against the remoteSgPriorities in
			// the loop below.
			localSgPriorities := make(interfaces.SgPriorities)
			if addBlobSyncgroupPriorities(ctx, s.bst, br, localSgPriorities) == nil {
				// We will request that the caller take different numbers of shares
				// depending on whether it is a "server" in the relevant syncgroup.
				for sgId, remoteSgPriority := range remoteSgPriorities {
					localShares := blobMetadata.OwnerShares[sgId]
					localSgPriority, gotLocalSgPriority := localSgPriorities[sgId]
					if gotLocalSgPriority && localShares > 0 && sgPriorityLowerThan(&localSgPriority, &remoteSgPriority) {
						if remoteSgPriority.DevType == wire.BlobDevTypeServer {
							// Caller is a server in this syncgroup----give it all the shares.
							sharesToTransfer[sgId] = localShares
						} else { // Caller is not a server, give it one share.
							sharesToTransfer[sgId] = 1
						}
					}
				}
			}
		}
	}
	return sharesToTransfer, err
}

func (s *syncService) HaveBlob(ctx *context.T, call rpc.ServerCall,
	br wire.BlobRef) (size int64, signpost interfaces.Signpost, err error) {

	vlog.VI(2).Infof("sync: HaveBlob: begin br %v", br)
	defer vlog.VI(2).Infof("sync: HaveBlob: end br %v", br)

	// In this routine we do not set err!=nil if the blob is unavailable.
	// Instead set size==-1, and set signpost.
	size = -1
	if bReader, err2 := s.bst.NewBlobReader(ctx, string(br)); err2 == nil {
		if bReader.IsFinalized() { // found blob, and it's complete
			size = bReader.Size()
		}
		bReader.Close()
	}
	if size == -1 { // can't find blob; try to return signpost
		err = s.bst.GetSignpost(ctx, br, &signpost)
		if err == nil {
			var blessingNames []string
			blessingNames, _ = security.RemoteBlessingNames(ctx, call.Security())
			filterSignpost(ctx, blessingNames, s, &signpost)
		}
	}
	return size, signpost, err
}

func (s *syncService) FetchBlobRecipe(ctx *context.T, call interfaces.SyncFetchBlobRecipeServerCall,
	br wire.BlobRef, callerName string, remoteSgPriorities interfaces.SgPriorities) (interfaces.BlobSharesBySyncgroup, error) {

	return nil, verror.NewErrNotImplemented(ctx)
}

func (s *syncService) FetchChunks(ctx *context.T, call interfaces.SyncFetchChunksServerCall) error {
	return verror.NewErrNotImplemented(ctx)
}

// RequestTakeBlob tells the server that client wishes the server to take some
// ownership shares for the blob br.
func (s *syncService) RequestTakeBlob(ctx *context.T, call rpc.ServerCall,
	br wire.BlobRef, callerName string, shares interfaces.BlobSharesBySyncgroup) error {

	return verror.NewErrNotImplemented(ctx)
}

// AcceptedBlobOwnership tells the server that the caller has accepted
// ownership shares of the blob, detailed in acceptedSharesBySyncgroup.
// TODO(m3b): need to pass mttables?
func (s *syncService) AcceptedBlobOwnership(ctx *context.T, call rpc.ServerCall, br wire.BlobRef, callerName string,
	acceptedSharesBySyncgroup interfaces.BlobSharesBySyncgroup) (serverName string, keepingBlob bool, err error) {

	// TODO(m3b):  Perhaps verify that the caller matches the ACL on the
	// syncgroups on which it's accepting ownership shares.
	// TODO(m3b):  Add synchronization so that two calls to
	// AcceptedBlobOwnership() or calls assigning ownership in
	// processBlobRefs for the same blob won't overlap.  This may cause
	// shares either to be lost or gained accidentally.

	var blobMetadata blob.BlobMetadata
	err = s.bst.GetBlobMetadata(ctx, br, &blobMetadata)
	var totalShares int32
	var mutatedBlobMetdata bool
	if err == nil {
		// Get the syncgroups associated with this blob into the sgSet, sgs.
		var sgs sgSet = make(sgSet)
		for groupId := range blobMetadata.OwnerShares {
			sgs[groupId] = struct{}{}
		}

		// Get the list of syncgroups in sgs for which callerName is a server in
		// sgs.
		var serverSgsForCaller sgSet = s.syncgroupsWithServer(ctx, wire.Id{}, callerName, sgs)

		// For each syncgroup for which the client will accept some
		// shares, decrement our ownership count.  Keep track of how
		// many shares this syncbase has kept.
		for groupId, gotShares := range blobMetadata.OwnerShares {
			acceptedShares := acceptedSharesBySyncgroup[groupId]
			if acceptedShares > 0 && gotShares > 0 {
				if _, callerIsServer := serverSgsForCaller[groupId]; !callerIsServer {
					acceptedShares = 1 // callerName not a server; give it only one share
				} // else callerName is a server in this group; it can take all the shares.
				if acceptedShares >= gotShares {
					gotShares = 0
					delete(blobMetadata.OwnerShares, groupId) // Last share taken.
				} else { // Otherwise, the caller may not take our last share.
					gotShares -= acceptedShares
					blobMetadata.OwnerShares[groupId] = gotShares
				}
				mutatedBlobMetdata = true
			}
			totalShares += gotShares
		}
		if mutatedBlobMetdata {
			err = s.bst.SetBlobMetadata(ctx, br, &blobMetadata)
		}
		if mutatedBlobMetdata && err == nil && totalShares == 0 {
			// This device successfully reduced its total shares to zero,
			// and may therefore discard the blob.  The device that just
			// accepted the shares will keep it, so add that device to the
			// Signpost.
			newLocData := peerLocationData(len(serverSgsForCaller) != 0, false)
			sp := interfaces.Signpost{Locations: interfaces.PeerToLocationDataMap{callerName: newLocData}}
			s.addToBlobSignpost(ctx, br, &sp)
		}
	}
	// TODO(m3b): return mttables, as well as just name of syncbase?
	return s.name, err == nil && totalShares > 0, err
}

////////////////////////////////////////////////////////////
// Helpers.

type byteStream interface {
	Send(item []byte) error
}

// getLocalBlob looks for a blob in the local store and, if found, reads the
// blob and sends it to the client.  If the blob is found, it starts reading it
// from the given offset and sends its bytes into the client stream.
func getLocalBlob(ctx *context.T, stream byteStream, bst blob.BlobStore, br wire.BlobRef, offset int64) error {
	vlog.VI(4).Infof("sync: getLocalBlob: begin br %v, offset %v", br, offset)
	defer vlog.VI(4).Infof("sync: getLocalBlob: end br %v, offset %v", br, offset)

	reader, err := bst.NewBlobReader(ctx, string(br))
	if err != nil {
		return err
	}
	defer reader.Close()

	if !reader.IsFinalized() {
		return wire.NewErrBlobNotCommitted(ctx)
	}

	buf := make([]byte, chunkSize)
	for {
		nbytes, err := reader.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			return err
		}
		if nbytes <= 0 {
			break
		}
		offset += int64(nbytes)
		stream.Send(buf[:nbytes])
		if err == io.EOF {
			break
		}
	}

	return nil
}

func (sd *syncDatabase) fetchBlobRemote(ctx *context.T, br wire.BlobRef, statusCall wire.BlobManagerFetchBlobServerCall, dataCall wire.BlobManagerGetBlobServerCall, offset int64) error {
	vlog.VI(4).Infof("sync: fetchBlobRemote: begin br %v, offset %v", br, offset)
	defer vlog.VI(4).Infof("sync: fetchBlobRemote: end br %v, offset %v", br, offset)

	// TODO(m3b): If this is called concurrently on the same blobref, we'll do redundant work.
	// We might also transfer too many ownership shares.

	var sendStatus, sendData bool
	var statusStream interface {
		Send(item wire.BlobFetchStatus) error
	}
	var dataStream interface {
		Send(item []byte) error
	}

	if statusCall != nil {
		sendStatus = true
		statusStream = statusCall.SendStream()
	}
	if dataCall != nil {
		sendData = true
		dataStream = dataCall.SendStream()
	}

	if sendStatus {
		// Start blob source discovery.
		statusStream.Send(wire.BlobFetchStatus{State: wire.BlobFetchStateLocating})
	}

	// Locate blob.
	peer, size, err := sd.locateBlob(ctx, br)
	if err != nil {
		return err
	}

	// Start blob fetching.
	status := wire.BlobFetchStatus{State: wire.BlobFetchStateFetching, Total: size}
	if sendStatus {
		statusStream.Send(status)
	}

	ss := sd.sync.(*syncService)
	bst := ss.bst

	bWriter, err := bst.NewBlobWriter(ctx, string(br))
	if err != nil {
		return err
	}

	// Get the syncgroup priorities for the blob that the peer is permitted
	// to know about.
	sgPriorities := make(interfaces.SgPriorities)
	var signpost interfaces.Signpost
	var blessingNames []string
	if ss.bst.GetSignpost(ctx, br, &signpost) == nil {
		blessingNames, err = getPeerBlessingsForFetchBlob(ctx, peer)
		if err == nil {
			filterSignpost(ctx, blessingNames, ss, &signpost)
			addSyncgroupPriorities(ctx, ss.bst, signpost.SgIds, sgPriorities)
		}
	}

	c := interfaces.SyncClient(peer)
	ctxPeer, cancel := context.WithRootCancel(ctx)
	// Run FetchBlob(), but checking that the peer has at least the
	// blessing names used above to generate the syncgroup priorities.
	stream, err := c.FetchBlob(ctxPeer, br, sgPriorities,
		options.ServerAuthorizer{namesAuthorizer{expNames: blessingNames}})
	if err == nil {
		peerStream := stream.RecvStream()
		for peerStream.Advance() {
			item := blob.BlockOrFile{Block: peerStream.Value()}
			if err = bWriter.AppendBytes(item); err != nil {
				break
			}
			curSize := int64(len(item.Block))
			status.Received += curSize
			if sendStatus {
				statusStream.Send(status)
			}
			if sendData {
				if curSize <= offset {
					offset -= curSize
				} else if offset != 0 {
					dataStream.Send(item.Block[offset:])
					offset = 0
				} else {
					dataStream.Send(item.Block)
				}
			}
		}

		if err != nil {
			cancel()
			stream.Finish()
		} else {
			err = peerStream.Err()
			remoteSharesBySgId, terr := stream.Finish()
			if err == nil {
				err = terr
			}
			if err == nil {
				// We successfully fetched the blob.  Maybe
				// take ownership in one or more syncgroups.
				ss := sd.sync.(*syncService)
				takingOwnership := make(interfaces.BlobSharesBySyncgroup)
				for sgId, shares := range remoteSharesBySgId {
					myPriority, havePriority := sgPriorities[sgId]
					if shares > 0 && havePriority {
						if myPriority.DevType != wire.BlobDevTypeServer && shares > 1 {
							// Non server devices never accept more than one share.
							shares = 1
						}
						takingOwnership[sgId] = shares
					}
				}

				// If we are accepting ownership shares, tell the peer.
				if len(takingOwnership) != 0 {
					// Don't worry if the following call fails; its
					// safe for this syncbase to treat
					// itself as an owner even if the peer
					// has not relinquished ownership.
					// TODO(m3b): With the current code, a peer accepts blob ownership only if
					// it retrieves the blob.  This may mean that a laptop accepts some of
					// the shares for an image from a camera, but if the laptop keeps a copy
					// despite passing its shares to a server, it may never come back to
					// accept the last share, forcing the camera to keep the blob forever.
					// Among the possible fixes:
					// a) accept all ownership shares (undesirable, to protect against
					//    loss of device accepting shares), or
					// b) somehow (in signposts?) communicate to peers when the blob has
					//    reached a "server" so that they may unilaterally drop their shares, or
					// c) (most likely) sometimes accept shares when we have none even for
					//    blobs we already have, triggered perhaps via the RequestTakeBlob() call.
					var peerName string
					var peerKeepingBlob bool
					peerName, peerKeepingBlob, _ = c.AcceptedBlobOwnership(ctx, br, ss.name, takingOwnership)

					var blobMetadata blob.BlobMetadata
					ss.bst.GetBlobMetadata(ctx, br, &blobMetadata)

					for sgId, shares := range takingOwnership {
						blobMetadata.OwnerShares[sgId] += shares
					}
					ss.bst.SetBlobMetadata(ctx, br, &blobMetadata)

					// Remove peer from local signpost if it's not keeping blob.
					if !peerKeepingBlob {
						var sp interfaces.Signpost
						if ss.bst.GetSignpost(ctx, br, &sp) == nil {
							delete(sp.Locations, peerName)
							ss.bst.SetSignpost(ctx, br, &sp)
						}
					}
				}
			}
			cancel()
		}
	}

	bWriter.Close()
	if err != nil {
		// Clean up the blob with failed download, so that it can be
		// downloaded again. Ignore any error from deletion.
		bst.DeleteBlob(ctx, string(br))
	} else {
		status := wire.BlobFetchStatus{State: wire.BlobFetchStateDone}
		if sendStatus {
			statusStream.Send(status)
		}
	}
	return err
}

// getPeerBlessingsForFetchBlob returns the list of blessing names for
// the given peer by invoking a null FetchBlob call on that peer.
func getPeerBlessingsForFetchBlob(ctx *context.T, peer string) (blessingNames []string, err error) {
	var call rpc.ClientCall
	call, err = v23.GetClient(ctx).StartCall(ctx, peer, "FetchBlob",
		[]interface{}{wire.BlobRef(""), interfaces.SgPriorities{}})
	if err == nil {
		blessingNames, _ = call.RemoteBlessings()
		call.Finish()
	}
	return blessingNames, err
}

// filterSignpost removes from Signpost signpost any information that cannot be
// given to an endpoint with blessings blessingNames[], or wouldn't be useful.
func filterSignpost(ctx *context.T, blessingNames []string, s *syncService, signpost *interfaces.Signpost) {
	keepPeer := make(map[string]bool) // Location hints to keep.

	s.forEachDatabaseStore(ctx, func(dbId wire.Id, st *watchable.Store) bool {
		// For each database, fetch its syncgroup data entries by scanning their
		// prefix range.  Use a database snapshot for the scan.
		snapshot := st.NewSnapshot()
		defer snapshot.Abort()

		forEachSyncgroup(snapshot, func(gid interfaces.GroupId, sg *interfaces.Syncgroup) bool {
			_, sgIsPresent := signpost.SgIds[gid]
			if sgIsPresent {
				// Reveal a hinted syncgroup only if not private, or
				// the caller has permission to join it.
				isVisible := !sg.Spec.IsPrivate || isAuthorizedForTag(sg.Spec.Perms, access.Read, blessingNames)
				if !isVisible { // Otherwise omit the syncgroup.
					delete(signpost.SgIds, gid)
				}

				// Reveal a hinted location only if either:
				// - the location is a public server (marked in the Signpost), or
				// - the location is in a hinted syncgroup, and either is a server,
				//   or that syncgroup is being revealed to the caller.
				for peer := range signpost.Locations {
					if signpost.Locations[peer].IsServer {
						keepPeer[peer] = true
					} else {
						sgMember, joinerInSg := sg.Joiners[peer]
						if joinerInSg && (isVisible || sgMember.MemberInfo.BlobDevType == byte(wire.BlobDevTypeServer)) {
							keepPeer[peer] = true
						}
					}
				}
			}
			return false // from forEachSyncgroup closure
		})
		return false // from forEachDatabaseStore closure
	})

	for peer := range signpost.Locations {
		if !keepPeer[peer] {
			delete(signpost.Locations, peer)
		}
	}
}

// addBlobSyncgroupPriorities inserts into map sgPriMap the syncgroup
// priorities for the syncgroups in blob br's Signpost.
// This routine does not filter the information---this is done by the calling routine.
func addBlobSyncgroupPriorities(ctx *context.T, bst blob.BlobStore, br wire.BlobRef, sgPriMap interfaces.SgPriorities) error {
	var signpost interfaces.Signpost
	err := bst.GetSignpost(ctx, br, &signpost)
	if err == nil {
		return addSyncgroupPriorities(ctx, bst, signpost.SgIds, sgPriMap)
	}
	return err
}

// A peerAndLocData is a pair (peer, interfaces.LocationData),
// which represent the entries in an interfaces.PeerToLocationDataMap.
type peerAndLocData struct {
	peer    string
	locData interfaces.LocationData
}

// A peerAndLocDataVector is a slice of peerAndLocData.
// It is used to sort the list, to allow the list to be pruned.
type peerAndLocDataVector []peerAndLocData

// The following functions implement sort.Interface for peerAndLocDataVector.
// It's used to keep the top few values for each Signpost.

func (v peerAndLocDataVector) Len() int          { return len(v) }
func (v peerAndLocDataVector) Swap(i int, j int) { v[i], v[j] = v[j], v[i] }
func (v peerAndLocDataVector) Less(i int, j int) bool {
	if v[i].locData.IsServer && !v[j].locData.IsServer { // Prefer to keep servers.
		return true
	}
	if v[i].locData.IsProxy && !v[j].locData.IsProxy { // Prefer to keep proxies.
		return true
	}
	// Prefer to keep entries with later timestamps.
	return v[j].locData.WhenSeen.Before(v[i].locData.WhenSeen)
}

// mergeSignposts merges data from a source Signpost into a target Spignpost.
func mergeSignposts(targetSp *interfaces.Signpost, sourceSp *interfaces.Signpost) {
	// Target's maps exist because GetSignpost() ensures they do.
	if targetSp.Locations == nil || targetSp.SgIds == nil {
		panic("mergeSignposts called with targetSp with nil map")
	}

	// Merge the source data into the target.
	for sgId := range sourceSp.SgIds {
		targetSp.SgIds[sgId] = struct{}{}
	}

	for peer, sourceLocData := range sourceSp.Locations {
		if targetLocData, merging := targetSp.Locations[peer]; !merging {
			targetSp.Locations[peer] = sourceLocData
		} else if targetLocData.WhenSeen.Before(sourceLocData.WhenSeen) {
			targetSp.Locations[peer] = sourceLocData
		}
	}

	// If there are too many locations in the target Signpost, trim it.
	if len(targetSp.Locations) > maxLocationsInSignpost {
		locList := make(peerAndLocDataVector, 0, len(targetSp.Locations))
		for peer, locData := range targetSp.Locations {
			locList = append(locList, peerAndLocData{peer: peer, locData: locData})
		}
		sort.Sort(locList) // Sort by WhenSeen timestamp.
		for i := maxLocationsInSignpost; i != len(locList); i++ {
			delete(targetSp.Locations, locList[i].peer)
		}
	}
}

// TODO(hpucha): Add syncgroup driven blob discovery.
func (sd *syncDatabase) locateBlob(ctx *context.T, br wire.BlobRef) (string, int64, error) {
	vlog.VI(4).Infof("sync: locateBlob: begin br %v", br)
	defer vlog.VI(4).Infof("sync: locateBlob: end br %v", br)

	ss := sd.sync.(*syncService)
	var sp interfaces.Signpost
	err := ss.bst.GetSignpost(ctx, br, &sp)
	if err != nil {
		return "", 0, err
	}
	var updatedSp bool // whether "sp" has been updated since being fetched

	// Search for blob amongst the Locations in the Signpost.

	// Move list of peers into a slice so that we can extend iteration as
	// more hints are found.  Never look at more than maxLocationsInSignpost hints.
	var locationList peerAndLocDataVector
	locationMap := make(map[string]bool) // Set of peers in locationList.
	for p, locData := range sp.Locations {
		locationList = append(locationList, peerAndLocData{peer: p, locData: locData})
		locationMap[p] = true
	}
	for i := 0; i != len(locationList) && i != maxLocationsInSignpost; i++ {
		var p string = locationList[i].peer
		vlog.VI(4).Infof("sync: locateBlob: attempting %s", p)
		// Get the mount tables for this peer.
		mtTables, err := sd.getMountTables(ctx, p)
		if err != nil {
			continue
		}

		for mt := range mtTables {
			absName := naming.Join(mt, p, common.SyncbaseSuffix)
			c := interfaces.SyncClient(absName)
			size, remoteSp, err := c.HaveBlob(ctx, br)
			if size >= 0 {
				if updatedSp {
					ss.bst.SetSignpost(ctx, br, &sp)
				}
				vlog.VI(4).Infof("sync: locateBlob: found blob on %s", absName)
				return absName, size, nil
			} else if err == nil { // no size, but remoteSp is valid.
				// Add new locations to locationList so
				// subsequent iterations use them.
				for newPeer, locData := range remoteSp.Locations {
					if !locationMap[newPeer] {
						locationList = append(locationList, peerAndLocData{peer: newPeer, locData: locData})
						locationMap[newPeer] = true
					}
				}
				sort.Sort(locationList[i+1:]) // sort yet to be visited locations, so loop picks freshest one next
				mergeSignposts(&sp, &remoteSp)
				updatedSp = true
			}
		}
	}
	if updatedSp {
		ss.bst.SetSignpost(ctx, br, &sp)
	}

	return "", 0, verror.New(verror.ErrInternal, ctx, "blob not found")
}

func (sd *syncDatabase) getMountTables(ctx *context.T, peer string) (map[string]struct{}, error) {
	ss := sd.sync.(*syncService)
	mInfo := ss.copyMemberInfo(ctx, peer)
	return mInfo.mtTables, nil
}

func (s *syncService) addToBlobSignpost(ctx *context.T, br wire.BlobRef, sp *interfaces.Signpost) (err error) {
	var curSp interfaces.Signpost
	if err = s.bst.GetSignpost(ctx, br, &curSp); err == nil {
		mergeSignposts(&curSp, sp)
		err = s.bst.SetSignpost(ctx, br, &curSp)
	} else {
		err = s.bst.SetSignpost(ctx, br, sp)
	}

	return err
}

// syncgroupsWithServer returns an sgSet  containing those syncgroups in sgs for
// which "peer" is known to be a server.  If dbId is the zero value, the routine
// searches across all available dbId values.
func (s *syncService) syncgroupsWithServer(ctx *context.T, dbId wire.Id, peer string, sgs sgSet) sgSet {
	// Fill in serverSgsForPeer with the list of syncgroups in which "peer" is a server.
	serverSgsForPeer := make(sgSet)
	s.allMembersLock.Lock()
	member := s.allMembers.members[peer]
	if member != nil {
		var sgMemberInfoMaps map[wire.Id]sgMember = member.db2sg // All dbId entries.
		if dbId != (wire.Id{}) {
			// If dbId was specified, pick that one entry.
			sgMemberInfoMaps = map[wire.Id]sgMember{dbId: member.db2sg[dbId]}
		}
		for _, sgMemberInfoMap := range sgMemberInfoMaps {
			for gid := range sgs {
				if int32(sgMemberInfoMap[gid].MemberInfo.BlobDevType) == wire.BlobDevTypeServer {
					serverSgsForPeer[gid] = struct{}{}
				}
			}
		}
	}
	s.allMembersLock.Unlock()
	return serverSgsForPeer
}

// peerLocationData returns current LocationData for peer, based on
// the other arguments.
func peerLocationData(isServer bool, isProxy bool) interfaces.LocationData {
	return interfaces.LocationData{
		WhenSeen: time.Now(),
		IsServer: isServer,
		IsProxy:  isProxy,
	}
}

// processBlobRefs decodes the VOM-encoded value in the buffer and extracts from
// it all blob refs.  For each of these blob refs, it updates the blob metadata
// to associate to it the sync peer, the source, and the matching syncgroups.
// isCreator indicates whether the current device is the likely initial creator of the blob.
// allSgPrefixes contains all the syncgroups in the db that the current device is aware of.
// sharedSgPrefixes contains only those shared with a peer device that provided this data;
// it is nil if the data was created locally.
func (s *syncService) processBlobRefs(ctx *context.T, dbId wire.Id, st store.StoreReader, peer string, isCreator bool,
	allSgPrefixes map[string]sgSet, sharedSgPrefixes map[string]sgSet, m *interfaces.LogRecMetadata, rawValue *vom.RawBytes) error {

	objid := m.ObjId
	srcPeer := syncbaseIdToName(m.Id)

	vlog.VI(4).Infof("sync: processBlobRefs: begin: objid %s, peer %s, src %s", objid, peer, srcPeer)
	defer vlog.VI(4).Infof("sync: processBlobRefs: end: objid %s, peer %s, src %s", objid, peer, srcPeer)

	if rawValue == nil {
		return nil
	}

	var val *vdl.Value
	if err := rawValue.ToValue(&val); err != nil {
		// If we cannot decode the value, ignore blob processing and
		// continue. This is fine since all stored values need not be
		// vom encoded.
		return nil
	}

	brs := extractBlobRefs(val)
	if len(brs) == 0 {
		return nil // no BlobRefs => nothing to do
	}

	// The key (objid) starts with one of the store's reserved prefixes for
	// managed namespaces.  Remove that prefix to be able to compare it with
	// the syncgroup prefixes which are defined by the application.
	appKey := common.StripFirstKeyPartOrDie(objid)

	// Determine the set of syncgroups that cover this application key, both locally
	// and shared with the peer.
	allSgIds := make(sgSet)
	for p, sgs := range allSgPrefixes {
		if strings.HasPrefix(appKey, p) {
			for sg := range sgs {
				allSgIds[sg] = struct{}{}
			}
		}
	}
	sharedSgIds := make(sgSet)
	for p, sgs := range sharedSgPrefixes {
		if strings.HasPrefix(appKey, p) {
			for sg := range sgs {
				sharedSgIds[sg] = struct{}{}
			}
		}
	}

	// Associate the blob metadata with each blob ref.  Create a separate
	// copy of the syncgroup set for each blob ref.
	for br := range brs {
		vlog.VI(4).Infof("sync: processBlobRefs: found blobref %v, sgs %v", br, allSgIds)
		sp := interfaces.Signpost{Locations: make(interfaces.PeerToLocationDataMap), SgIds: make(sgSet)}
		var peerSyncgroups sgSet = s.syncgroupsWithServer(ctx, dbId, peer, allSgIds)
		var srcPeerSyncgroups sgSet
		sp.Locations[peer] = peerLocationData(len(peerSyncgroups) != 0, false /*not proxy*/)
		if peer != srcPeer {
			srcPeerSyncgroups = s.syncgroupsWithServer(ctx, dbId, srcPeer, allSgIds)
			sp.Locations[srcPeer] = peerLocationData(len(srcPeerSyncgroups) != 0, false /*not proxy*/)
		}
		for gid := range allSgIds {
			sp.SgIds[gid] = struct{}{}
		}
		plausibleProxy := false
		if !isCreator && len(sharedSgIds) < len(allSgIds) {
			// BlobRef was received via syncing and this device
			// puts it in more syncgroups than the peer, so it's a
			// plausible proxy.
			plausibleProxy = true
		} else if isCreator {
			var curSp interfaces.Signpost
			if err := s.bst.GetSignpost(ctx, br, &curSp); err == nil && len(curSp.SgIds) > 0 {
				// BlobRef is known to be associated with some syncgroups already.
				for gid := range sp.SgIds {
					if _, inExistingSyncgroup := curSp.SgIds[gid]; !inExistingSyncgroup {
						// BlobRef is being added to at least one syncgroup
						// different from those previously known.  So this
						// device is a plausible proxy.
						plausibleProxy = true
						break
					}
				}
			}
		}

		if plausibleProxy {
			var selfSyncgroups sgSet
			if s.name == peer {
				selfSyncgroups = peerSyncgroups
			} else if s.name == srcPeer {
				selfSyncgroups = srcPeerSyncgroups
			} else {
				selfSyncgroups = s.syncgroupsWithServer(ctx, dbId, s.name, allSgIds)
			}
			sp.Locations[s.name] = peerLocationData(len(selfSyncgroups) != 0, true /*proxy*/)
		}
		if err := s.addToBlobSignpost(ctx, br, &sp); err != nil {
			return err
		}
	}

	if isCreator { // This device put this BlobRef in the syncgroups; assign ownership shares.
		for br := range brs {
			reader, _ := s.bst.NewBlobReader(ctx, string(br))
			if reader != nil { // blob is present on device
				reader.Close()
				var bmd blob.BlobMetadata
				var changed bool
				// TODO(m3b): put a lock around the GetBlobMetadata() and the subsequent SetBlobMetadata
				// to avoid losing or gaining ownership shares via concurrent updates.
				if s.bst.GetBlobMetadata(ctx, br, &bmd) != nil { // no BlobMetadata yet available
					bmd.Referenced = time.Now()
					bmd.Accessed = bmd.Referenced
					changed = true
				}
				// Set the initial ownership shares for each syncgroup for which it's not yet set.
				for gid := range allSgIds {
					if _, isPresent := bmd.OwnerShares[gid]; !isPresent {
						bmd.OwnerShares[gid] = initialBlobOwnerShares
						changed = true
					}
				}
				if changed {
					if err := s.bst.SetBlobMetadata(ctx, br, &bmd); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// extractBlobRefs traverses a VDL value and extracts blob refs from it.
func extractBlobRefs(val *vdl.Value) map[wire.BlobRef]struct{} {
	brs := make(map[wire.BlobRef]struct{})
	extractBlobRefsInternal(val, brs)
	return brs
}

// extractBlobRefsInternal traverses a VDL value recursively and extracts blob
// refs from it.  The blob refs are accumulated in the given map of blob refs.
// The function returns true if the data may contain blob refs, which means it
// either contains blob refs or contains dynamic data types (VDL union or any)
// which may in some instances contain blob refs.  Otherwise the function
// returns false to indicate that the data definitely cannot contain blob refs.
func extractBlobRefsInternal(val *vdl.Value, brs map[wire.BlobRef]struct{}) bool {
	mayContain := false

	if val != nil {
		switch val.Kind() {
		case vdl.String:
			// Could be a BlobRef.
			var br wire.BlobRef
			if val.Type() == vdl.TypeOf(br) {
				mayContain = true
				if b := wire.BlobRef(val.RawString()); b != wire.NullBlobRef {
					brs[b] = struct{}{}
				}
			}

		case vdl.Struct:
			for i := 0; i < val.Type().NumField(); i++ {
				if extractBlobRefsInternal(val.StructField(i), brs) {
					mayContain = true
				}
			}

		case vdl.Array, vdl.List:
			for i := 0; i < val.Len(); i++ {
				if extractBlobRefsInternal(val.Index(i), brs) {
					mayContain = true
				} else {
					// Look no further, no blob refs in the rest.
					break
				}
			}

		case vdl.Map:
			lookInKey, lookInVal := true, true
			for _, v := range val.Keys() {
				if lookInKey {
					if extractBlobRefsInternal(v, brs) {
						mayContain = true
					} else {
						// No need to look in the keys anymore.
						lookInKey = false
					}
				}
				if lookInVal {
					if extractBlobRefsInternal(val.MapIndex(v), brs) {
						mayContain = true
					} else {
						// No need to look in values anymore.
						lookInVal = false
					}
				}
				if !lookInKey && !lookInVal {
					// Look no further, no blob refs in the rest.
					break
				}
			}

		case vdl.Set:
			for _, v := range val.Keys() {
				if extractBlobRefsInternal(v, brs) {
					mayContain = true
				} else {
					// Look no further, no blob refs in the rest.
					break
				}
			}

		case vdl.Union:
			_, val = val.UnionField()
			extractBlobRefsInternal(val, brs)
			mayContain = true

		case vdl.Any, vdl.Optional:
			extractBlobRefsInternal(val.Elem(), brs)
			mayContain = true
		}
	}

	return mayContain
}
