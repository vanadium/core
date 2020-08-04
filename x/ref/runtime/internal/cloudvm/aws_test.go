// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudvm

import (
	"context"
	"testing"
	"time"

	"v.io/x/ref/runtime/internal/cloudvm/cloudpaths"
	"v.io/x/ref/runtime/internal/cloudvm/cloudvmtest"
)

func startAWSMetadataServer(t *testing.T) (string, func()) {
	host, close := cloudvmtest.StartAWSMetadataServer(t)
	SetAWSMetadataHost(host)
	return host, close
}

func TestAWS(t *testing.T) {
	ctx := context.Background()
	host, stop := startAWSMetadataServer(t)
	defer stop()

	if got, want := OnAWS(ctx, time.Second), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	testStats(t, AWSAccountIDStatName, AWSRegionStatName)

	priv, err := AWSPrivateAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := priv[0].String(), cloudvmtest.WellKnownPrivateIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	pub, err := AWSPublicAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pub[0].String(), cloudvmtest.WellKnownPrivateIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	externalURL := host + cloudpaths.AWSPublicIPPath + "/noip"
	noip, err := awsGetAddr(ctx, externalURL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(noip), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
