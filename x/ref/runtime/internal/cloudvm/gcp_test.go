// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudvm

import (
	"context"
	"testing"
	"time"

	"v.io/x/ref/lib/stats"
	"v.io/x/ref/runtime/internal/cloudvm/cloudpaths"
	"v.io/x/ref/runtime/internal/cloudvm/cloudvmtest"
)

func startGCPMetadataServer(t *testing.T) (string, func()) {
	host, close := cloudvmtest.StartGCPMetadataServer(t)
	SetGCPMetadataHost(host)
	return host, close
}

func testStats(t *testing.T, idStat, regionStat string) {
	val, err := stats.Value(idStat)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := val.(string), cloudvmtest.WellKnownAccount; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	val, err = stats.Value(regionStat)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := val.(string), cloudvmtest.WellKnownRegion; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGCP(t *testing.T) {
	ctx := context.Background()
	host, stop := startGCPMetadataServer(t)
	defer stop()

	if got, want := OnGCP(ctx, time.Second), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	testStats(t, GCPProjectIDStatName, GCPRegionStatName)

	priv, err := GCPPrivateAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := priv[0].String(), cloudvmtest.WellKnownPrivateIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	pub, err := GCPPublicAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pub[0].String(), cloudvmtest.WellKnownPublicIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	externalURL := host + cloudpaths.GCPExternalIPPath + "/noip"
	noip, err := gcpGetAddr(ctx, externalURL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(noip), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
