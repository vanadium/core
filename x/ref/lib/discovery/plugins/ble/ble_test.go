// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ble

import (
	"testing"
	"time"

	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/discovery/plugins/testutil"
	"v.io/x/ref/test"
)

func TestBasic(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.TestContext()
	defer shutdown()

	adinfos := []idiscovery.AdInfo{
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@foo.com:1234@@/x"},
				Attributes:    discovery.Attributes{"a": "a1234"},
			},
			Hash:        idiscovery.AdHash{1, 2, 3},
			TimestampNs: 1001,
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{4, 5, 6},
				InterfaceName: "v.io/y",
				Addresses:     []string{"/@6@wsh@bar.com:1234@@/y"},
			},
			Hash:        idiscovery.AdHash{4, 5, 6},
			TimestampNs: 1002,
		},
	}

	neighborhood := newNeighborhood()
	defer neighborhood.shutdown()

	p1, err := newWithTTL(ctx, "h1", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p1.Close()

	var stops []func()
	for i := range adinfos {
		stop, err := testutil.Advertise(ctx, p1, &adinfos[i])
		if err != nil {
			t.Fatal(err)
		}
		stops = append(stops, stop)
	}

	p2, err := newWithTTL(ctx, "h2", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p2.Close()

	// Make sure all advertisements are discovered.
	if err := testutil.ScanAndMatch(ctx, p2, "v.io/x", adinfos[0]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, p2, "v.io/y", adinfos[1]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, p2, "", adinfos...); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, p2, "v.io/z"); err != nil {
		t.Error(err)
	}

	// Make sure it is not discovered when advertising is stopped.
	stops[0]()
	if err := testutil.ScanAndMatch(ctx, p2, "v.io/x"); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, p2, "", adinfos[1]); err != nil {
		t.Error(err)
	}

	// Open a new scan channel and consume an expected advertisement first.
	scanCh, scanStop, err := testutil.Scan(ctx, p2, "v.io/y")
	if err != nil {
		t.Error(err)
	}
	defer scanStop()

	if got := *<-scanCh; !testutil.MatchFound([]idiscovery.AdInfo{got}, adinfos[1]) {
		t.Errorf("Unexpected scan: %v, but want %v", got, adinfos[1])
	}

	// Make sure scan returns the lost advertisement when advertising is stopped.
	stops[1]()

	if got := *<-scanCh; !testutil.MatchLost([]idiscovery.AdInfo{got}, adinfos[1]) {
		t.Errorf("Unexpected scan: %v, but want %v as lost", got, adinfos[1])
	}

	// Shouldn't discover anything with a new scan.
	if err := testutil.ScanAndMatch(ctx, p2, ""); err != nil {
		t.Error(err)
	}

	// Make sure it is discovered again when advertising is restarted.
	stops[1], err = testutil.Advertise(ctx, p1, &adinfos[1])
	if err != nil {
		t.Fatal(err)
	}
	defer stops[1]()

	if got := *<-scanCh; !testutil.MatchFound([]idiscovery.AdInfo{got}, adinfos[1]) {
		t.Errorf("Unexpected scan: %v, but want %v", got, adinfos[1])
	}

	// Should discover it with a new scan.
	if err := testutil.ScanAndMatch(ctx, p2, "", adinfos[1]); err != nil {
		t.Error(err)
	}
}

func TestMultipleScans(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	adinfos := []idiscovery.AdInfo{
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@foo.com:1234@@/x"},
				Attributes:    discovery.Attributes{"a": "a1234"},
			},
			Hash:        idiscovery.AdHash{1, 2, 3},
			TimestampNs: 1001,
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{4, 5, 6},
				InterfaceName: "v.io/y",
				Addresses:     []string{"/@6@wsh@bar.com:1234@@/y"},
			},
			Hash:        idiscovery.AdHash{4, 5, 6},
			TimestampNs: 1002,
		},
	}

	neighborhood := newNeighborhood()
	defer neighborhood.shutdown()

	p1, err := newWithTTL(ctx, "h1", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p1.Close()

	for i := range adinfos {
		stop, err := testutil.Advertise(ctx, p1, &adinfos[i])
		if err != nil {
			t.Fatal(err)
		}
		defer stop()
	}

	p2, err := newWithTTL(ctx, "h2", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p2.Close()

	scanCh1, scanStop1, err := testutil.Scan(ctx, p2, "v.io/x")
	if err != nil {
		t.Error(err)
	}
	defer scanStop1()

	adinfo := *<-scanCh1
	if !testutil.MatchFound([]idiscovery.AdInfo{adinfo}, adinfos[0]) {
		t.Errorf("Unexpected scan: %v, but want %v", adinfo, adinfos[0])
	}

	p3, err := newWithTTL(ctx, "h3", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p3.Close()

	scanCh2, scanStop2, err := testutil.Scan(ctx, p3, "v.io/y")
	if err != nil {
		t.Error(err)
	}
	defer scanStop2()

	adinfo = *<-scanCh2
	if !testutil.MatchFound([]idiscovery.AdInfo{adinfo}, adinfos[1]) {
		t.Errorf("Unexpected scan: %v, but want %v", adinfo, adinfos[1])
	}

	// They should not affect other scans.
	select {
	case got := <-scanCh1:
		t.Errorf("Unexpected scan: %v", *got)
	case got := <-scanCh2:
		t.Errorf("Unexpected scan: %v", *got)
	case <-time.After(5 * time.Millisecond):
	}
}

func TestAttachments(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	adinfos := []idiscovery.AdInfo{
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@foo.com:1234@@/x"},
			},
			DirAddrs: []string{"/@6@wsh@foo.com:1234@@/d"},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{4, 5, 6},
				InterfaceName: "v.io/y",
				Addresses:     []string{"/@6@wsh@foo.com:1234@@/x"},
				Attachments:   discovery.Attachments{"a": []byte{11, 12, 13}},
			},
			DirAddrs: []string{"/@6@wsh@foo.com:1234@@/d"},
		},
	}

	neighborhood := newNeighborhood()
	defer neighborhood.shutdown()

	p1, err := newWithTTL(ctx, "h1", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p1.Close()

	for i := range adinfos {
		stop, err := testutil.Advertise(ctx, p1, &adinfos[i])
		if err != nil {
			t.Fatal(err)
		}
		defer stop()
	}

	p2, err := newWithTTL(ctx, "h2", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p2.Close()

	withAdStatus := func(adinfo idiscovery.AdInfo, status idiscovery.AdStatus) idiscovery.AdInfo {
		switch status {
		case idiscovery.AdReady:
			adinfo.DirAddrs = nil
			adinfo.Status = idiscovery.AdReady
		case idiscovery.AdPartiallyReady:
			adinfo.Ad.Attachments = nil
			adinfo.Status = idiscovery.AdPartiallyReady
		}
		return adinfo
	}
	wanted := []idiscovery.AdInfo{
		withAdStatus(adinfos[0], idiscovery.AdReady),
		withAdStatus(adinfos[1], idiscovery.AdPartiallyReady),
	}
	if err := testutil.ScanAndMatch(ctx, p2, "", wanted...); err != nil {
		t.Error(err)
	}
}

func TestMultipleInstances(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	adinfos := []idiscovery.AdInfo{
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@foo.com:1234@@/x"},
			},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{4, 5, 6},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@foo.com:5678@@/x"},
			},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{7, 8, 9},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@bar.com:1234@@/x"},
			},
		},
	}

	neighborhood := newNeighborhood()
	defer neighborhood.shutdown()

	p1, err := newWithTTL(ctx, "h1", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p1.Close()

	stop0, err := testutil.Advertise(ctx, p1, &adinfos[0])
	if err != nil {
		t.Fatal(err)
	}

	p2, err := newWithTTL(ctx, "h2", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p2.Close()

	// Open a new scan channel and consume an expected advertisement first.
	scanCh, scanStop, err := testutil.Scan(ctx, p2, "v.io/x")
	if err != nil {
		t.Error(err)
	}
	defer scanStop()

	if got := *<-scanCh; !testutil.MatchFound([]idiscovery.AdInfo{got}, adinfos[0]) {
		t.Errorf("Unexpected scan: %v, but want %v", got, adinfos[0])
	}

	// Try to advertise another instance of the same interface.
	stop1, err := testutil.Advertise(ctx, p1, &adinfos[1])
	if err != nil {
		t.Fatal(err)
	}
	defer stop1()

	// We might lose the existing advertisements before rescanning under very
	// heavy load. Wait until the expected advertisement is scanned.
	if err := testutil.WaitUntilMatchFound(scanCh, adinfos[1]); err != nil {
		t.Error(err)
	}

	// Make sure scan returns the lost advertisement when advertising is stopped.
	stop0()

	if err := testutil.WaitUntilMatchLost(scanCh, adinfos[0]); err != nil {
		t.Error(err)
	}

	// Other device should be able to advertise an instance of the same interface too.
	p3, err := newWithTTL(ctx, "h3", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p3.Close()

	stop2, err := testutil.Advertise(ctx, p3, &adinfos[2])
	if err != nil {
		t.Fatal(err)
	}
	defer stop2()

	if err := testutil.WaitUntilMatchFound(scanCh, adinfos[2]); err != nil {
		t.Error(err)
	}

	// Make sure all active advertisements are discovered by a new scan.
	if err := testutil.ScanAndMatch(ctx, p2, "", adinfos[1], adinfos[2]); err != nil {
		t.Error(err)
	}
}

func TestMaximumMultipleInstances(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	neighborhood := newNeighborhood()
	defer neighborhood.shutdown()

	p1, err := newWithTTL(ctx, "h1", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p1.Close()

	var adinfos []idiscovery.AdInfo
	for i := 0; i < maxNumPackedServices; i++ {
		adinfo := idiscovery.AdInfo{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{byte(i + 1)},
				InterfaceName: "v.io/y",
				Addresses:     []string{"/@6@wsh@foo.com:1234@@/y"},
			},
		}
		stop, err := testutil.Advertise(ctx, p1, &adinfo)
		if err != nil {
			t.Error(err)
			continue
		}
		defer stop()

		adinfos = append(adinfos, adinfo)
	}

	// Make sure no more instance is allowed.
	oneMore := adinfos[0]
	oneMore.Ad.Id[0] = byte(maxNumPackedServices + 1)
	if _, err := testutil.Advertise(ctx, p1, &oneMore); err == nil {
		t.Error("Expected an error; but got none")
	}

	// Make sure all advertisements are discovered.
	p2, err := newWithTTL(ctx, "h2", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p2.Close()

	if err := testutil.ScanAndMatch(ctx, p2, "", adinfos...); err != nil {
		t.Error(err)
	}
}

func TestTimestamp(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	adinfo := idiscovery.AdInfo{
		Ad: discovery.Advertisement{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "v.io/x",
			Addresses:     []string{"/@6@wsh@foo.com:1234@@/x"},
		},
		Hash:        idiscovery.AdHash{1, 2, 3},
		TimestampNs: 1001,
	}

	neighborhood := newNeighborhood()
	defer neighborhood.shutdown()

	p1, err := newWithTTL(ctx, "h1", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p1.Close()

	stop, err := testutil.Advertise(ctx, p1, &adinfo)
	if err != nil {
		t.Fatal(err)
	}

	p2, err := newWithTTL(ctx, "h2", defaultTTL)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p2.Close()

	// Open a new scan channel and consume expected advertisements first.
	scanCh, scanStop, err := testutil.Scan(ctx, p2, "")
	if err != nil {
		t.Error(err)
	}
	defer scanStop()

	if got := <-scanCh; !testutil.MatchFound([]idiscovery.AdInfo{*got}, adinfo) {
		t.Errorf("Unexpected scan: %v, but want %v", *got, adinfo)
	}

	// Stop and re-advertise it. We should not receive any new advertisement.
	stop()
	stop, err = testutil.Advertise(ctx, p1, &adinfo)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-scanCh:
		t.Errorf("Unexpected scan: %v", *got)
	case <-time.After(5 * time.Millisecond):
	}

	// Stop and re-advertise it with an older timestamp. We should ignore it.
	stop()
	adinfo.Ad.Addresses = []string{"/@6@wsh@foo.com:1234@@/y"}
	adinfo.Hash = idiscovery.AdHash{4, 5, 6}
	adinfo.TimestampNs = 1000
	stop, err = testutil.Advertise(ctx, p1, &adinfo)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-scanCh:
		t.Errorf("Unexpected scan: %v", *got)
	case <-time.After(5 * time.Millisecond):
	}

	// Stop and re-advertise it with a newer timestamp. We should discover it.
	stop()
	adinfo.Ad.Addresses = []string{"/@6@wsh@foo.com:1234@@/z"}
	adinfo.Hash = idiscovery.AdHash{7, 8, 9}
	adinfo.TimestampNs = 1002
	stop, err = testutil.Advertise(ctx, p1, &adinfo)
	defer stop()
	if err != nil {
		t.Fatal(err)
	}

	if got := <-scanCh; !testutil.MatchFound([]idiscovery.AdInfo{*got}, adinfo) {
		t.Errorf("Unexpected scan: %v, but want %v", *got, adinfo)
	}
}
