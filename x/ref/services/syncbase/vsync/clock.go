// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/vclock"
)

// GetTime implements the responder side of the GetTime RPC.
// Important: This method wraps all errors in ErrGetTimeFailed so that
// runRemoteOp can reliably distinguish RPC errors from application errors. See
// TODO in runRemoteOp.
// TODO(sadovsky): This method does zero authorization, which means any client
// can send a syncgroup these requests. This seems undesirable.
func (s *syncService) GetTime(ctx *context.T, call rpc.ServerCall, req interfaces.TimeReq, initiator string) (interfaces.TimeResp, error) {
	vlog.VI(2).Infof("sync: GetTime: begin: from initiator %s", initiator)
	defer vlog.VI(2).Infof("sync: GetTime: end: from initiator %s", initiator)

	// For detecting sysclock changes that would break the NTP send/recv time
	// calculations. See detailed comment in vclock/ntp.go.
	recvNow, recvElapsedTime, err := s.vclock.SysClockVals()
	if err != nil {
		vlog.Errorf("sync: GetTime: SysClockVals failed: %v", err)
		return interfaces.TimeResp{}, verror.New(interfaces.ErrGetTimeFailed, ctx, err)
	}

	// Fetch local vclock data.
	data := &vclock.VClockData{}
	if err := s.vclock.GetVClockData(data); err != nil {
		vlog.Errorf("sync: GetTime: error fetching VClockData: %v", err)
		return interfaces.TimeResp{}, verror.New(interfaces.ErrGetTimeFailed, ctx, err)
	}

	// Check for sysclock changes.
	sendNow, _, err := s.vclock.SysClockValsIfNotChanged(recvNow, recvElapsedTime)
	if err != nil {
		vlog.Errorf("sync: GetTime: SysClockValsIfNotChanged failed: %v", err)
		return interfaces.TimeResp{}, verror.New(interfaces.ErrGetTimeFailed, ctx, err)
	}

	return interfaces.TimeResp{
		OrigTs:     req.SendTs,
		RecvTs:     s.vclock.ApplySkew(recvNow, data),
		SendTs:     s.vclock.ApplySkew(sendNow, data),
		LastNtpTs:  data.LastNtpTs,
		NumReboots: data.NumReboots,
		NumHops:    data.NumHops,
	}, nil
}

// syncVClock syncs the local Syncbase vclock with peer's Syncbase vclock.
// Works by treating the peer as an NTP server, obtaining a single sample, and
// then updating the local VClockData as appropriate.
func (s *syncService) syncVClock(ctxIn *context.T, peer connInfo) error {
	vlog.VI(2).Infof("sync: syncVClock: begin: contacting peer %v", peer)
	defer vlog.VI(2).Infof("sync: syncVClock: end: contacting peer %v", peer)

	// For detecting sysclock changes that would break the NTP send/recv time
	// calculations. See detailed comment in vclock/ntp.go.
	origNow, origElapsedTime, err := s.vclock.SysClockVals()
	if err != nil {
		vlog.Errorf("sync: syncVClock: SysClockVals failed: %v", err)
		return err
	}

	ctx, cancel := context.WithCancel(ctxIn)
	defer cancel()

	// TODO(sadovsky): Propagate the returned connInfo back to syncer so that we
	// leverage what we learned here about available mount tables and endpoints
	// when we continue on to the next initiation phase.
	var timeRespInt interface{}
	var runAtPeerCancel context.CancelFunc
	if _, timeRespInt, runAtPeerCancel, err = runAtPeer(ctx, peer, func(ctx *context.T, peer string) (interface{}, error) {
		return interfaces.SyncClient(peer).GetTime(ctx, interfaces.TimeReq{SendTs: origNow}, s.name, options.ConnectionTimeout(syncConnectionTimeout))
	}); err != nil {
		vlog.Errorf("sync: syncVClock: GetTime failed: %v", err)
		runAtPeerCancel()
		return err
	}
	defer runAtPeerCancel()

	// Check for sysclock changes.
	recvNow, _, err := s.vclock.SysClockValsIfNotChanged(origNow, origElapsedTime)
	if err != nil {
		vlog.Errorf("sync: syncVClock: SysClockValsIfNotChanged failed: %v", err)
		return err
	}

	timeResp, ok := timeRespInt.(interfaces.TimeResp)
	if !ok {
		err := fmt.Errorf("sync: syncVClock: unexpected GetTime response type: %#v", timeRespInt)
		vlog.Fatal(err)
		return err
	}

	return s.vclock.UpdateVClockData(func(data *vclock.VClockData) (*vclock.VClockData, error) {
		psd := &vclock.PeerSyncData{
			MySendTs:   s.vclock.ApplySkew(origNow, data),
			RecvTs:     timeResp.RecvTs,
			SendTs:     timeResp.SendTs,
			MyRecvTs:   s.vclock.ApplySkew(recvNow, data),
			LastNtpTs:  timeResp.LastNtpTs,
			NumReboots: timeResp.NumReboots,
			NumHops:    timeResp.NumHops,
		}
		if newData, err := vclock.MaybeUpdateFromPeerData(s.vclock, data, psd); err != nil {
			if _, ok := err.(*vclock.NoUpdateErr); ok {
				// Note, we still return an error in this case so that UpdateVClockData
				// does not even attempt to write VClockData back to the store.
				vlog.VI(2).Infof("sync: syncVClock: decided not to update VClockData: %v", err)
			} else {
				vlog.Errorf("sync: syncVClock: error updating VClockData: %v", err)
			}
			return nil, err
		} else {
			// Check again for sysclock changes after reading VClockData and calling
			// MaybeUpdateFromPeerData. VClockData is influenced by the system clock
			// via VClockD, and MaybeUpdateFromPeerData consults the system clock.
			if _, _, err := s.vclock.SysClockValsIfNotChanged(origNow, origElapsedTime); err != nil {
				vlog.Errorf("sync: syncVClock: SysClockValsIfNotChanged failed: %v", err)
				return nil, err
			} else {
				return newData, nil
			}
		}
	})
}
