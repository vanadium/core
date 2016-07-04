// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"testing"
	"time"

	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/vclock"
)

func TestGetTime(t *testing.T) {
	service := createService(t)
	defer destroyService(t, service)

	wantSkew := 3 * time.Second
	if err := store.Put(nil, service.St(), common.VClockPrefix, &vclock.VClockData{
		SystemTimeAtBoot:     time.Time{},
		Skew:                 wantSkew,
		ElapsedTimeSinceBoot: 0,
		LastNtpTs:            time.Now().Add(-10 * time.Minute),
		NumReboots:           0,
		NumHops:              0,
	}); err != nil {
		t.Errorf("Failed to write VClockData: %v", err)
	}

	resp, err := service.Sync().GetTime(nil, nil, interfaces.TimeReq{SendTs: time.Now()}, "test")
	if err != nil {
		t.Fatalf("GetTime RPC failed: %v", err)
	}

	gotSkew := getSkew(resp)
	if abs(gotSkew-wantSkew) > 3*time.Millisecond {
		t.Errorf("GetTime returned skew outside of error bounds; want: %v, got: %v", wantSkew, gotSkew)
	}
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func getSkew(resp interfaces.TimeResp) time.Duration {
	clientRecvTs := time.Now()
	clientSendTs := resp.OrigTs
	serverRecvTs := resp.RecvTs
	serverSendTs := resp.SendTs
	return (serverRecvTs.Sub(clientSendTs) + serverSendTs.Sub(clientRecvTs)) / 2
}
