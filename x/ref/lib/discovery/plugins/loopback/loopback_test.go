// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loopback_test

import (
	"testing"

	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/discovery/plugins/loopback"
	"v.io/x/ref/lib/discovery/plugins/testutil"
	"v.io/x/ref/test"
)

func TestBasic(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	adinfos := []idiscovery.AdInfo{
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@v.com@@/x1"},
				Attributes:    discovery.Attributes{"a": "123"},
				Attachments:   discovery.Attachments{"a": []byte{1, 2, 3}},
			},
			Hash: idiscovery.AdHash{1, 2, 3},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{4, 5, 6},
				InterfaceName: "v.io/y",
				Addresses:     []string{"/@6@wsh@v.com@@/y"},
			},
			Hash: idiscovery.AdHash{4, 5, 6},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{7, 8, 9},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@v.com@@/x2"},
			},
			Hash: idiscovery.AdHash{7, 8, 9},
		},
	}

	p, err := loopback.New(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}

	var stops []func()
	for i := range adinfos {
		stop, err := testutil.Advertise(ctx, p, &adinfos[i])
		if err != nil {
			t.Fatal(err)
		}
		stops = append(stops, stop)
	}

	// Make sure all advertisements are discovered.
	if err := testutil.ScanAndMatch(ctx, p, "v.io/x", adinfos[0], adinfos[2]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, p, "v.io/y", adinfos[1]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, p, "", adinfos...); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, p, "v.io/z"); err != nil {
		t.Error(err)
	}

	// Make sure it is not discovered when advertising is stopped.
	stops[0]()
	if err := testutil.ScanAndMatch(ctx, p, "v.io/x", adinfos[2]); err != nil {
		t.Error(err)
	}

	// Open a new scan channel and consume expected advertisements first.
	scanCh, scanStop, err := testutil.Scan(ctx, p, "v.io/y")
	if err != nil {
		t.Error(err)
	}
	defer scanStop()

	adinfo := *<-scanCh
	if !testutil.MatchFound([]idiscovery.AdInfo{adinfo}, adinfos[1]) {
		t.Errorf("Unexpected scan: %v, but want %v", adinfo, adinfos[1])
	}

	// Make sure scan returns the lost advertisement when advertising is stopped.
	stops[1]()

	adinfo = *<-scanCh
	if !testutil.MatchLost([]idiscovery.AdInfo{adinfo}, adinfos[1]) {
		t.Errorf("Unexpected scan: %v, but want %v as lost", adinfo, adinfos[1])
	}

	// Stop advertising the remaining one; Shouldn't discover anything.
	stops[2]()
	if err := testutil.ScanAndMatch(ctx, p, ""); err != nil {
		t.Error(err)
	}
}
