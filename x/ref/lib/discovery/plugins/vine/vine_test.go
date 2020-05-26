// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vine

import (
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/discovery/plugins/testutil"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/timekeeper"
)

func TestBasic(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	adinfos := []idiscovery.AdInfo{
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@v1.com@@/x"},
				Attributes:    discovery.Attributes{"a": "123"},
				Attachments:   discovery.Attachments{"a": []byte{1, 2, 3}},
			},
			Hash: idiscovery.AdHash{1, 2, 3},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{4, 5, 6},
				InterfaceName: "v.io/y",
				Addresses:     []string{"/@6@wsh@v2.com@@/y"},
			},
			Hash: idiscovery.AdHash{4, 5, 6},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{7, 8, 9},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@v3.com@@/y"},
			},
			Hash: idiscovery.AdHash{7, 8, 9},
		},
	}

	configs := []struct {
		name  string
		peers []string
	}{
		{"v1", []string{"v2", "v3"}},
		{"v2", []string{"v1"}},
		{"v3", []string{"v1"}},
	}

	plugins := make([]idiscovery.Plugin, len(configs))
	for i, config := range configs {
		var err error
		peers := config.peers
		plugins[i], err = New(ctx, config.name, func(*context.T) []string { return peers })
		if err != nil {
			t.Fatal(err)
		}
		defer plugins[i].Close()
	}

	var stops []func()
	for i := range adinfos {
		stop, err := testutil.Advertise(ctx, plugins[i], &adinfos[i])
		if err != nil {
			t.Fatal(err)
		}
		stops = append(stops, stop)
	}

	// Make sure all advertisements are discovered by connected peers.
	if err := testutil.ScanAndMatch(ctx, plugins[0], "v.io/x", adinfos[2]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[0], "v.io/y", adinfos[1]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[0], "", adinfos[1], adinfos[2]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[0], "v.io/z"); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[1], "", adinfos[0]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[1], "v.io/y"); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[2], "", adinfos[0]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[1], "v.io/y"); err != nil {
		t.Error(err)
	}

	// Stop advertisings and make sure they are not discovered by others.
	stops[0]()
	if err := testutil.ScanAndMatch(ctx, plugins[1], "v.io/x"); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, plugins[2], ""); err != nil {
		t.Error(err)
	}

	stops[1]()
	if err := testutil.ScanAndMatch(ctx, plugins[0], "v.io/y"); err != nil {
		t.Error(err)
	}

	// Open a new scan channel and consume expected advertisements first.
	scanCh, scanStop, err := testutil.Scan(ctx, plugins[0], "")
	if err != nil {
		t.Error(err)
	}
	defer scanStop()

	adinfo := *<-scanCh
	if !testutil.MatchFound([]idiscovery.AdInfo{adinfo}, adinfos[2]) {
		t.Errorf("Unexpected scan: %v, but want %v", adinfo, adinfos[2])
	}

	// Make sure scan returns the lost advertisement when advertising is stopped.
	stops[2]()

	adinfo = *<-scanCh
	if !testutil.MatchLost([]idiscovery.AdInfo{adinfo}, adinfos[2]) {
		t.Errorf("Unexpected scan: %v, but want %v as lost", adinfo, adinfos[2])
	}
}

func TestExpired(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	adinfos := []idiscovery.AdInfo{
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "v.io/x",
				Addresses:     []string{"/@6@wsh@v1.com@@/x"},
			},
			Hash: idiscovery.AdHash{1, 2, 3},
		},
		{
			Ad: discovery.Advertisement{
				Id:            discovery.AdId{4, 5, 6},
				InterfaceName: "v.io/y",
				Addresses:     []string{"/@6@wsh@v2.com@@/y"},
			},
			Hash: idiscovery.AdHash{4, 5, 6},
		},
	}

	configs := []struct {
		name  string
		peers []string
		ttl   time.Duration
		clock timekeeper.ManualTime
	}{
		{"v1", []string{"v3"}, 10 * time.Minute, timekeeper.NewManualTime()},
		{"v2", []string{"v3"}, 20 * time.Minute, timekeeper.NewManualTime()},
		{"v3", []string{}, 0, timekeeper.NewManualTime()},
	}

	plugins := make([]idiscovery.Plugin, len(configs))
	for i, config := range configs {
		var err error
		peers := config.peers
		plugins[i], err = newWithClock(ctx, config.name, func(*context.T) []string { return peers }, config.ttl, config.clock)
		if err != nil {
			t.Fatal(err)
		}
		defer plugins[i].Close()
	}

	for i := range adinfos {
		stop, err := testutil.Advertise(ctx, plugins[i], &adinfos[i])
		if err != nil {
			t.Fatal(err)
		}
		defer stop()
	}

	// All advertisements should be discovered for now.
	if err := testutil.ScanAndMatch(ctx, plugins[2], "", adinfos...); err != nil {
		t.Error(err)
	}

	// Advance the clock past the first advertisement TTL and make sure it has been expired.
	<-configs[2].clock.Requests()
	configs[2].clock.AdvanceTime(15 * time.Minute)

	if err := testutil.ScanAndMatch(ctx, plugins[2], "", adinfos[1]); err != nil {
		t.Error(err)
	}

	// Advance the clock past the second advertisement TTL and make sure it has been expired too.
	<-configs[2].clock.Requests()
	configs[2].clock.AdvanceTime(10 * time.Minute)

	if err := testutil.ScanAndMatch(ctx, plugins[2], ""); err != nil {
		t.Error(err)
	}
}

func TestRefresh(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	adinfo := idiscovery.AdInfo{
		Ad: discovery.Advertisement{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "v.io/x",
			Addresses:     []string{"/@6@wsh@v1.com@@/x"},
			Attributes:    discovery.Attributes{"a": "123"},
			Attachments:   discovery.Attachments{"a": []byte{1, 2, 3}},
		},
		Hash: idiscovery.AdHash{1, 2, 3},
	}

	configs := []struct {
		name  string
		peers []string
	}{
		{"v1", []string{"v2"}},
		{"v2", []string{}},
	}

	const TTL = 10 * time.Minute

	clock := timekeeper.NewManualTime()

	plugins := make([]idiscovery.Plugin, len(configs))
	for i, config := range configs {
		var err error
		peers := config.peers
		plugins[i], err = newWithClock(ctx, config.name, func(*context.T) []string { return peers }, TTL, clock)
		if err != nil {
			t.Fatal(err)
		}
		defer plugins[i].Close()
	}

	stop, err := testutil.Advertise(ctx, plugins[0], &adinfo)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// Make sure all advertisements are discovered.
	if err := testutil.ScanAndMatch(ctx, plugins[1], "", adinfo); err != nil {
		t.Error(err)
	}

	// Make sure that the advertisement are refreshed on every ttl time.
	for i := 0; i < 5; i++ {
		for {
			// Wait until the advertising plugin is waiting for refreshing explicitly
			// since the scanning plugin may call After() multiple times depending on
			// the goroutine scheduling during the test. Note that the scanning plugin
			// waits for TTL with a slack time.
			if d := <-clock.Requests(); d == TTL {
				break
			}
		}
		clock.AdvanceTime(TTL * 2)

		if err := testutil.ScanAndMatch(ctx, plugins[1], "", adinfo); err != nil {
			t.Error(err)
		}
	}
}
