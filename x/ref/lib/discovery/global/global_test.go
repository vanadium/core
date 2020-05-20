// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package global

import (
	"fmt"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/discovery"
	"v.io/v23/options"
	"v.io/v23/security"

	"v.io/x/ref/lib/discovery/testutil"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/test"
	"v.io/x/ref/test/timekeeper"
)

const testPath = "a/b/c"

func TestBasic(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	ads := []discovery.Advertisement{
		{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "foo/bar/baz",
			Addresses:     []string{"/h1:123/x", "/h2:123/y"},
			Attributes:    discovery.Attributes{"k": "v"},
		},
		{
			InterfaceName: "foo/bar/baz",
			Addresses:     []string{"/h1:123/x", "/h2:123/z"},
		},
	}

	d1, err := New(ctx, testPath)
	if err != nil {
		t.Fatal(err)
	}

	var stops []func()
	for i := range ads {
		stop, err := testutil.Advertise(ctx, d1, nil, &ads[i])
		if err != nil {
			t.Fatal(err)
		}
		stops = append(stops, stop)
	}

	// Make sure none of advertisements are discoverable by the same discovery instance.
	if err := testutil.ScanAndMatch(ctx, d1, ``); err != nil {
		t.Error(err)
	}

	// Create a new discovery instance. All advertisements should be discovered with that.
	d2, err := NewWithTTL(ctx, testPath, 0, 1*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	if err := testutil.ScanAndMatch(ctx, d2, `k="01020300000000000000000000000000"`, ads[0]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, d2, fmt.Sprintf(`k="%s"`, ads[1].Id), ads[1]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, d2, ``, ads...); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, d2, `k="not_exist"`); err != nil {
		t.Error(err)
	}

	// Open a new scan channel and consume expected advertisements first.
	scanCh, scanStop, err := testutil.Scan(ctx, d2, fmt.Sprintf(`k="%s"`, ads[0].Id))
	if err != nil {
		t.Fatal(err)
	}
	defer scanStop()

	update := <-scanCh
	if !testutil.MatchFound(ctx, []discovery.Update{update}, ads[0]) {
		t.Errorf("unexpected scan: %v", update)
	}

	// Make sure scan returns the lost advertisement when advertising is stopped.
	stops[0]()

	update = <-scanCh
	if !testutil.MatchLost(ctx, []discovery.Update{update}, ads[0]) {
		t.Errorf("unexpected scan: %v", update)
	}

	// Also it shouldn't affect the other.
	if err := testutil.ScanAndMatch(ctx, d2, fmt.Sprintf(`k="%s"`, ads[1].Id), ads[1]); err != nil {
		t.Error(err)
	}

	// Stop advertising the remaining one; Shouldn't discover any service.
	stops[1]()
	if err := testutil.ScanAndMatch(ctx, d2, ``); err != nil {
		t.Error(err)
	}
}

func TestVisibility(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	ad := discovery.Advertisement{
		InterfaceName: "foo/bar/baz",
		Addresses:     []string{"/h1:123/x"},
	}
	visibility := []security.BlessingPattern{
		security.BlessingPattern("test-blessing:bob"),
		security.BlessingPattern("test-blessing:alice").MakeNonExtendable(),
	}

	d1, _ := New(ctx, testPath)

	mectx, _ := testutil.WithPrincipal(ctx, "me")
	stop, _ := testutil.Advertise(mectx, d1, visibility, &ad)
	defer stop()

	d2, _ := NewWithTTL(ctx, testPath, 0, 1*time.Millisecond)

	// Bob and his friend should discover the advertisement.
	bobctx, _ := testutil.WithPrincipal(ctx, "bob")
	if err := testutil.ScanAndMatch(bobctx, d2, ``, ad); err != nil {
		t.Error(err)
	}
	bobfriendctx, _ := testutil.WithPrincipal(ctx, "bob:friend")
	if err := testutil.ScanAndMatch(bobfriendctx, d2, ``, ad); err != nil {
		t.Error(err)
	}

	// Alice should discover the advertisement, but her friend shouldn't.
	alicectx, _ := testutil.WithPrincipal(ctx, "alice")
	if err := testutil.ScanAndMatch(alicectx, d2, ``, ad); err != nil {
		t.Error(err)
	}
	alicefriendctx, _ := testutil.WithPrincipal(ctx, "alice:friend")
	if err := testutil.ScanAndMatch(alicefriendctx, d2, ``); err != nil {
		t.Error(err)
	}

	// Other people shouldn't discover the advertisement.
	carolctx, _ := testutil.WithPrincipal(ctx, "carol")
	if err := testutil.ScanAndMatch(carolctx, d2, ``); err != nil {
		t.Error(err)
	}
}

func TestDuplicates(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	ad := discovery.Advertisement{
		InterfaceName: "foo/bar/baz",
		Addresses:     []string{"/h1:123/x"},
	}

	d, _ := New(ctx, testPath)

	stop, _ := testutil.Advertise(ctx, d, nil, &ad)
	defer stop()

	if _, err := testutil.Advertise(ctx, d, nil, &ad); err == nil {
		t.Error("expect an error; but got none")
	}
}

func TestRefresh(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	clock := timekeeper.NewManualTime()
	mt, err := mounttablelib.NewMountTableDispatcherWithClock(ctx, "", "", "", clock)
	if err != nil {
		t.Fatal(err)
	}
	_, mtserver, err := v23.WithNewDispatchingServer(ctx, "", mt, options.ServesMountTable(true))
	if err != nil {
		t.Fatal(err)
	}
	ns := v23.GetNamespace(ctx)
	if err := ns.SetRoots(mtserver.Status().Endpoints[0].Name()); err != nil {
		t.Fatal(err)
	}

	ad := discovery.Advertisement{
		InterfaceName: "foo/bar/baz",
		Addresses:     []string{"/h1:123/x"},
	}

	const mountTTL = 10 * time.Second
	d1, _ := newWithClock(ctx, testPath, mountTTL, 0, clock)

	stop, _ := testutil.Advertise(ctx, d1, nil, &ad)
	defer stop()

	d2, _ := NewWithTTL(ctx, testPath, 0, 1*time.Millisecond)
	if err := testutil.ScanAndMatch(ctx, d2, ``, ad); err != nil {
		t.Error(err)
	}

	// Make sure that the advertisement are refreshed on every ttl time.
	for i := 0; i < 5; i++ {
		<-clock.Requests()
		clock.AdvanceTime(mountTTL * 2)

		if err := testutil.ScanAndMatch(ctx, d2, ``, ad); err != nil {
			t.Error(err)
		}
	}
}
