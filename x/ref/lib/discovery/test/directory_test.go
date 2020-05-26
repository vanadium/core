// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"bytes"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/rpc"
	"v.io/v23/security"

	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/discovery/plugins/mock"
	"v.io/x/ref/lib/discovery/testutil"
	"v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/test"
)

func TestDirectoryBasic(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var dirServer idiscovery.DirectoryServerStub

	mockServer := testutil.NewMockServer(testutil.ToEndpoints("addr:123"))
	ctx = fake.SetServerFactory(ctx, func(_ *context.T, _ string, impl interface{}, _ security.Authorizer, _ ...rpc.ServerOpt) (*context.T, rpc.Server) {
		dirServer = impl.(idiscovery.DirectoryServerStub)
		return ctx, mockServer
	})

	df, err := idiscovery.NewFactory(ctx, mock.New())
	if err != nil {
		t.Fatal(err)
	}
	defer df.Shutdown()

	ad := discovery.Advertisement{
		InterfaceName: "v.io/v23/a",
		Addresses:     []string{"/h1:123/x"},
		Attachments:   discovery.Attachments{"a": []byte{1, 2, 3}},
	}

	d1, err := df.New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	stop, err := testutil.Advertise(ctx, d1, nil, &ad)
	if err != nil {
		t.Fatal(err)
	}

	// The directory server should serve the advertisement.
	fetched, err := dirServer.Lookup(ctx, nil, ad.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fetched.Ad, ad) {
		t.Errorf("got %v, but wanted %v", fetched.Ad, ad)
	}

	attachment, err := dirServer.GetAttachment(ctx, nil, ad.Id, "a")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(attachment, ad.Attachments["a"]) {
		t.Errorf("got %v, but wanted %v", attachment, ad.Attachments["a"])
	}

	attachment, err = dirServer.GetAttachment(ctx, nil, ad.Id, "b")
	if err != nil {
		t.Fatal(err)
	}
	if attachment != nil {
		t.Errorf("got %v, but wanted %v", attachment, nil)
	}

	// Stop advertising; Shouldn't be served by the directory server.
	stop()

	if _, err = dirServer.Lookup(ctx, nil, ad.Id); err == nil {
		t.Error("expected an error, but got none")
	}
	if _, err = dirServer.GetAttachment(ctx, nil, ad.Id, "a"); err == nil {
		t.Error("expected an error, but got none")
	}
}

func TestDirectoryRoaming(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var dirServer idiscovery.DirectoryServerStub

	eps := testutil.ToEndpoints("addr:123")
	mockServer := testutil.NewMockServer(eps)
	ctx = fake.SetServerFactory(ctx, func(_ *context.T, _ string, impl interface{}, _ security.Authorizer, _ ...rpc.ServerOpt) (*context.T, rpc.Server) {
		dirServer = impl.(idiscovery.DirectoryServerStub)
		return ctx, mockServer
	})

	mockPlugin := mock.NewWithAdStatus(idiscovery.AdPartiallyReady)
	df, err := idiscovery.NewFactory(ctx, mockPlugin)
	if err != nil {
		t.Fatal(err)
	}
	defer df.Shutdown()

	ad := discovery.Advertisement{
		InterfaceName: "v.io/v23/a",
		Addresses:     []string{"/h1:123/x"},
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

	// Open a new scan channel and consume expected advertisements first.
	scanCh, scanStop, err := testutil.Scan(ctx, d2, ``)
	if err != nil {
		t.Fatal(err)
	}
	defer scanStop()

	update := <-scanCh
	if !testutil.MatchFound(ctx, []discovery.Update{update}, ad) {
		t.Errorf("unexpected scan: %v", update)
	}

	fetched, err := dirServer.Lookup(ctx, nil, ad.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testutil.ToEndpoints(fetched.DirAddrs...), eps) {
		t.Errorf("got %v, but wanted %v", testutil.ToEndpoints(fetched.DirAddrs...), eps)
	}

	updateCh := make(chan *idiscovery.AdInfo)
	callback := func(ad *idiscovery.AdInfo) {
		select {
		case updateCh <- ad:
		case <-ctx.Done():
		}
	}
	if err = mockPlugin.Scan(ctx, "", callback, func() {}); err != nil {
		t.Fatal(err)
	}
	<-updateCh

	eps = testutil.ToEndpoints("addr:456")
	mockServer.UpdateNetwork(eps)

	// Wait until the address change is applied
	for {
		adinfo := <-updateCh
		if reflect.DeepEqual(testutil.ToEndpoints(adinfo.DirAddrs...), eps) {
			break
		}
	}

	// Make sure that a new advertisement is published.
	update = <-scanCh
	if !testutil.MatchLost(ctx, []discovery.Update{update}, ad) {
		t.Errorf("unexpected scan: %v", update)
	}
	update = <-scanCh
	if !testutil.MatchFound(ctx, []discovery.Update{update}, ad) {
		t.Errorf("unexpected scan: %v", update)
	}

	fetched, err = dirServer.Lookup(ctx, nil, ad.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testutil.ToEndpoints(fetched.DirAddrs...), eps) {
		t.Errorf("got %v, but wanted %v", testutil.ToEndpoints(fetched.DirAddrs...), eps)
	}
}
