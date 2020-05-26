// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23/discovery"
	"v.io/v23/security"

	"v.io/x/lib/ibe"
	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/discovery/plugins/mock"
	"v.io/x/ref/lib/discovery/testutil"
	"v.io/x/ref/lib/security/bcrypter"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

func TestBasic(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, err := idiscovery.NewFactory(ctx, mock.New())
	if err != nil {
		t.Fatal(err)
	}
	defer df.Shutdown()

	ads := []discovery.Advertisement{
		{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "v.io/a",
			Addresses:     []string{"/h1:123/x", "/h2:123/y"},
			Attributes:    discovery.Attributes{"a1": "v1"},
		},
		{
			InterfaceName: "v.io/b",
			Addresses:     []string{"/h1:123/x", "/h2:123/z"},
			Attributes:    discovery.Attributes{"b1": "v1"},
		},
	}

	d1, err := df.New(ctx)
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
	d2, err := df.New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := testutil.ScanAndMatch(ctx, d2, `v.InterfaceName="v.io/a"`, ads[0]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, d2, `v.InterfaceName="v.io/b"`, ads[1]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, d2, ``, ads...); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, d2, `v.InterfaceName="v.io/c"`); err != nil {
		t.Error(err)
	}

	// Open a new scan channel and consume expected advertisements first.
	scanCh, scanStop, err := testutil.Scan(ctx, d2, `v.InterfaceName="v.io/a"`)
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
	if err := testutil.ScanAndMatch(ctx, d2, `v.InterfaceName="v.io/b"`, ads[1]); err != nil {
		t.Error(err)
	}

	// Stop advertising the remaining one; Shouldn't discover any service.
	stops[1]()
	if err := testutil.ScanAndMatch(ctx, d2, ``); err != nil {
		t.Error(err)
	}
}

// TODO(jhahn): Add a low level test that ensures the advertisement is unusable
// by the listener, if encrypted rather than replying on a higher level API.
func TestVisibility(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, _ := idiscovery.NewFactory(ctx, mock.New())
	defer df.Shutdown()

	master, err := ibe.SetupBB2()
	if err != nil {
		ctx.Fatalf("ibe.SetupBB2 failed: %v", err)
	}
	root := bcrypter.NewRoot("v.io", master)
	crypter := bcrypter.NewCrypter()
	if err := crypter.AddParams(ctx, root.Params()); err != nil {
		ctx.Fatalf("bcrypter.AddParams failed: %v", err)
	}

	ad := discovery.Advertisement{
		InterfaceName: "v.io/v23/a",
		Addresses:     []string{"/h1:123/x", "/h2:123/y"},
		Attributes:    map[string]string{"a1": "v1", "a2": "v2"},
	}
	visibility := []security.BlessingPattern{
		security.BlessingPattern("v.io:bob"),
		security.BlessingPattern("v.io:alice").MakeNonExtendable(),
	}

	d1, _ := df.New(ctx)

	sctx := bcrypter.WithCrypter(ctx, crypter)
	stop, err := testutil.Advertise(sctx, d1, visibility, &ad)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	d2, _ := df.New(ctx)

	// Bob and his friend should discover the advertisement.
	bobctx, _ := testutil.WithPrivateKey(ctx, root, "v.io:bob")
	if err := testutil.ScanAndMatch(bobctx, d2, ``, ad); err != nil {
		t.Error(err)
	}
	bobfriendctx, _ := testutil.WithPrivateKey(ctx, root, "v.io:bob:friend")
	if err := testutil.ScanAndMatch(bobfriendctx, d2, ``, ad); err != nil {
		t.Error(err)
	}

	// Alice should discover the advertisement, but her friend shouldn't.
	alicectx, _ := testutil.WithPrivateKey(ctx, root, "v.io:alice")
	if err := testutil.ScanAndMatch(alicectx, d2, ``, ad); err != nil {
		t.Error(err)
	}
	alicefriendctx, _ := testutil.WithPrivateKey(ctx, root, "v.io:alice:friend")
	if err := testutil.ScanAndMatch(alicefriendctx, d2, ``); err != nil {
		t.Error(err)
	}

	// Other people shouldn't discover the advertisement.
	carolctx, _ := testutil.WithPrivateKey(ctx, root, "v.io:carol")
	if err := testutil.ScanAndMatch(carolctx, d2, ``); err != nil {
		t.Error(err)
	}
}

func TestDuplicates(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, _ := idiscovery.NewFactory(ctx, mock.New())
	defer df.Shutdown()

	ad := discovery.Advertisement{
		InterfaceName: "v.io/v23/a",
		Addresses:     []string{"/h1:123/x"},
	}

	d, _ := df.New(ctx)
	stop, err := testutil.Advertise(ctx, d, nil, &ad)
	if stop != nil {
		defer stop()
	}
	if err != nil {
		t.Fatal(err)
	}
	stop, err = testutil.Advertise(ctx, d, nil, &ad)
	if stop != nil {
		defer stop()
	}
	if err == nil {
		t.Error("expect an error; but got none")
	}
}

func TestMerge(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	p1, p2 := mock.New(), mock.New()
	df, _ := idiscovery.NewFactory(ctx, p1, p2)
	defer df.Shutdown()

	adinfo := idiscovery.AdInfo{
		Ad: discovery.Advertisement{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "v.io/v23/a",
			Addresses:     []string{"/h1:123/x"},
		},
		Hash: idiscovery.AdHash{1, 2, 3},
	}

	d, _ := df.New(ctx)
	scanCh, scanStop, err := testutil.Scan(ctx, d, ``)
	if err != nil {
		t.Fatal(err)
	}
	defer scanStop()

	// A plugin returns an advertisement and we should see it.
	p1.RegisterAd(&adinfo)
	update := <-scanCh
	if !testutil.MatchFound(ctx, []discovery.Update{update}, adinfo.Ad) {
		t.Errorf("unexpected scan: %v", update)
	}

	// The other plugin returns the same advertisement, but we should not see it.
	p2.RegisterAd(&adinfo)
	select {
	case update = <-scanCh:
		t.Errorf("unexpected scan: %v", update)
	case <-time.After(5 * time.Millisecond):
	}

	// Two plugins update the service, but we should see the update only once.
	newAdinfo := adinfo
	newAdinfo.Ad.Addresses = []string{"/h1:456/x"}
	newAdinfo.Hash = idiscovery.AdHash{4, 5, 6}

	go func() { p1.RegisterAd(&newAdinfo) }()
	go func() { p2.RegisterAd(&newAdinfo) }()

	// Should see 'Lost' first.
	update = <-scanCh
	if !testutil.MatchLost(ctx, []discovery.Update{update}, adinfo.Ad) {
		t.Errorf("unexpected scan: %v", update)
	}
	update = <-scanCh
	if !testutil.MatchFound(ctx, []discovery.Update{update}, newAdinfo.Ad) {
		t.Errorf("unexpected scan: %v", update)
	}
	select {
	case update = <-scanCh:
		t.Errorf("unexpected scan: %v", update)
	case <-time.After(5 * time.Millisecond):
	}

	// Lost in both, should see a lost event once.
	p1.UnregisterAd(&newAdinfo)
	p2.UnregisterAd(&newAdinfo)
	update = <-scanCh
	if !testutil.MatchLost(ctx, []discovery.Update{update}, newAdinfo.Ad) {
		t.Errorf("unexpected scan: %v", update)
	}
	select {
	case update = <-scanCh:
		t.Errorf("unexpected scan: %v", update)
	case <-time.After(5 * time.Millisecond):
	}
}

func TestLostInOneButNotAllPlugins(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	p1, p2 := mock.New(), mock.New()
	df, _ := idiscovery.NewFactory(ctx, p1, p2)
	defer df.Shutdown()

	ad := idiscovery.AdInfo{
		Ad: discovery.Advertisement{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "v.io/v23/a",
			Addresses:     []string{"/h1:123/x"},
		},
		Hash:        idiscovery.AdHash{1, 2, 3},
		TimestampNs: time.Now().UnixNano(),
	}

	olderAd := ad
	olderAd.TimestampNs -= 1000000000
	olderAd.Hash = idiscovery.AdHash{4, 5, 6}

	newerAd := ad
	newerAd.TimestampNs += 1000000000
	newerAd.Hash = idiscovery.AdHash{7, 8, 9}

	d, _ := df.New(ctx)
	scanCh, scanStop, err := testutil.Scan(ctx, d, "")
	if err != nil {
		t.Fatal(err)
	}
	defer scanStop()

	noevent := func() error {
		select {
		case update := <-scanCh:
			return fmt.Errorf("unexpected scan: %v", update)
		case <-time.After(5 * time.Millisecond):
			return nil
		}
	}

	p1.RegisterAd(&ad)
	if update := <-scanCh; !testutil.MatchFound(ctx, []discovery.Update{update}, ad.Ad) {
		t.Errorf("unexpected scan: %v", update)
	}
	// p2 sees the same ad, but no event should be delivered.
	p2.RegisterAd(&ad)
	if err := noevent(); err != nil {
		t.Error(err)
	}

	// And if p1 loses ad, but p2 doesn't, then nothing should be delivered.
	p1.UnregisterAd(&ad)
	if err := noevent(); err != nil {
		t.Error(err)
	}

	// An older ad should be ignored
	p1.RegisterAd(&olderAd)
	if err := noevent(); err != nil {
		t.Error(err)
	}

	// But a newer one should be seen as a LOST + FOUND
	p2.RegisterAd(&newerAd)
	if update := <-scanCh; !testutil.MatchLost(ctx, []discovery.Update{update}, ad.Ad) {
		t.Errorf("unexpected: %v", update)
	}
	if update := <-scanCh; !testutil.MatchFound(ctx, []discovery.Update{update}, newerAd.Ad) {
		t.Errorf("unexpected: %v", update)
	}

	// Newer ad lost by p2 and never seen by p1, should be lost
	p2.UnregisterAd(&newerAd)
	if update := <-scanCh; !testutil.MatchLost(ctx, []discovery.Update{update}, ad.Ad) {
		t.Errorf("unexpected: %v", update)
	}
}

func TestTimestamp(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	p1, p2 := mock.New(), mock.New()
	df, _ := idiscovery.NewFactory(ctx, p1, p2)
	defer df.Shutdown()

	adinfo := idiscovery.AdInfo{
		Ad: discovery.Advertisement{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "v.io/v23/a",
			Addresses:     []string{"/h1:123/x"},
		},
		Hash:        idiscovery.AdHash{1, 2, 3},
		TimestampNs: 1001,
	}

	d, _ := df.New(ctx)
	scanCh, scanStop, err := testutil.Scan(ctx, d, ``)
	if err != nil {
		t.Fatal(err)
	}
	defer scanStop()

	// A plugin returns an advertisement and we should see it.
	p1.RegisterAd(&adinfo)
	update := <-scanCh
	if !testutil.MatchFound(ctx, []discovery.Update{update}, adinfo.Ad) {
		t.Errorf("unexpected scan: %v", update)
	}

	// The other plugin returns an old advertisement, but we should not see it.
	oldAdinfo := adinfo
	oldAdinfo.Ad.Addresses = []string{"/h0:123/x"}
	oldAdinfo.Hash = idiscovery.AdHash{0, 1, 2}
	oldAdinfo.TimestampNs = 1000
	p2.RegisterAd(&oldAdinfo)
	select {
	case update = <-scanCh:
		t.Errorf("unexpected scan: %v", update)
	case <-time.After(5 * time.Millisecond):
	}
}

func TestLargeAdvertisement(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, err := idiscovery.NewFactory(ctx, mock.NewWithAdStatus(idiscovery.AdNotReady))
	if err != nil {
		t.Fatal(err)
	}
	defer df.Shutdown()

	ads := []discovery.Advertisement{
		{
			InterfaceName: "v.io/a",
			Addresses:     []string{"/h1:123/x"},
			Attributes:    discovery.Attributes{"a": "v"},
		},
		{
			InterfaceName: "v.io/b",
			Addresses:     []string{"/h1:123/y"},
			Attachments: discovery.Attachments{
				"a1": bytes.Repeat([]byte{1}, 2048),
				"a2": bytes.Repeat([]byte{2}, 4096),
				"a3": bytes.Repeat([]byte{3}, 1024),
				"a4": bytes.Repeat([]byte{4}, 2048),
			},
		},
	}

	d1, err := df.New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i := range ads {
		stop, err := testutil.Advertise(ctx, d1, nil, &ads[i])
		if err != nil {
			t.Fatal(err)
		}
		defer stop()
	}

	d2, err := df.New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := testutil.ScanAndMatch(ctx, d2, `v.InterfaceName="v.io/a"`, ads[0]); err != nil {
		t.Error(err)
	}
	if err := testutil.ScanAndMatch(ctx, d2, `v.Attributes["a"]="v"`, ads[0]); err != nil {
		t.Error(err)
	}

	scanCh, scanStop, err := testutil.Scan(ctx, d2, `v.InterfaceName="v.io/b"`)
	if err != nil {
		t.Fatal(err)
	}
	defer scanStop()
	update := <-scanCh

	// Make sure that the directory server does not return all of the attachments if they are too large.
	if reflect.DeepEqual(update.Advertisement().Attachments, ads[1].Attachments) {
		t.Errorf("did not expect all of attachments, but got all: %v", update.Advertisement())
	}
	// But we should be able to fetch them lazily.
	if !testutil.UpdateEqual(ctx, update, ads[1]) {
		t.Errorf("Match failed; got %v, but wanted %v", update, ads[1])
	}
}

func TestLargeAttachments(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, err := idiscovery.NewFactory(ctx, mock.NewWithAdStatus(idiscovery.AdPartiallyReady))
	if err != nil {
		t.Fatal(err)
	}
	defer df.Shutdown()

	ad := discovery.Advertisement{
		InterfaceName: "v.io/a",
		Addresses:     []string{"/h1:123/x", "/h2:123/y"},
		Attachments: discovery.Attachments{
			"a1": []byte{1, 2, 3},
			"a2": []byte{4, 5, 6},
		},
	}

	d1, err := df.New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	stop, err := testutil.Advertise(ctx, d1, nil, &ad)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	d2, err := df.New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := testutil.ScanAndMatch(ctx, d2, ``, ad); err != nil {
		t.Error(err)
	}
}

func TestShutdown(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, _ := idiscovery.NewFactory(ctx, mock.New())

	ad := discovery.Advertisement{
		InterfaceName: "v.io/v23/a",
		Addresses:     []string{"/h1:123/x"},
	}

	d1, _ := df.New(ctx)
	stop, err := testutil.Advertise(ctx, d1, nil, &ad)
	if stop != nil {
		defer stop()
	}
	if err != nil {
		t.Error(err)
	}
	d2, _ := df.New(ctx)
	if err := testutil.ScanAndMatch(ctx, d2, ``, ad); err != nil {
		t.Error(err)
	}

	// Verify Close can be called multiple times.
	df.Shutdown()
	df.Shutdown()

	// Make sure advertise and scan do not work after closed.
	ad.Id = discovery.AdId{} // To avoid dup error.
	stop, err = testutil.Advertise(ctx, d1, nil, &ad)
	if stop != nil {
		defer stop()
	}
	if err == nil {
		t.Error("expect an error; but got none")
	}
	if err := testutil.ScanAndMatch(ctx, d2, ``, ad); err == nil {
		t.Error("expect an error; but got none")
	}
}
