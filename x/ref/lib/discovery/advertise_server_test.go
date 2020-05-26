// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"testing"

	v23 "v.io/v23"
	"v.io/v23/discovery"
	"v.io/v23/naming"
	idiscovery "v.io/x/ref/lib/discovery"
	fdiscovery "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/discovery/plugins/mock"
	"v.io/x/ref/lib/discovery/testutil"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

func withNewAddresses(ad *discovery.Advertisement, eps []naming.Endpoint, suffix string) discovery.Advertisement {
	newAd := *ad
	newAd.Addresses = make([]string, len(eps))
	for i, ep := range eps {
		newAd.Addresses[i] = naming.JoinAddressName(ep.Name(), suffix)
	}
	return newAd
}

func TestAdvertiseServer(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	df, _ := idiscovery.NewFactory(ctx, mock.New())
	fdiscovery.InjectFactory(df)

	const suffix = "test"

	eps := testutil.ToEndpoints("addr1:123")
	mockServer := testutil.NewMockServer(eps)

	ad := discovery.Advertisement{
		InterfaceName: "v.io/v23/a",
		Attributes:    map[string]string{"a1": "v1"},
	}

	_, err := idiscovery.AdvertiseServer(ctx, nil, mockServer, suffix, &ad, nil)
	if err != nil {
		t.Fatal(err)
	}

	d, _ := v23.NewDiscovery(ctx)

	newAd := withNewAddresses(&ad, eps, suffix)
	if err := testutil.ScanAndMatch(ctx, d, "", newAd); err != nil {
		t.Error(err)
	}

	tests := [][]naming.Endpoint{
		testutil.ToEndpoints("addr2:123", "addr3:456"),
		testutil.ToEndpoints("addr4:123"),
		testutil.ToEndpoints("addr5:123", "addr6:456"),
	}
	for _, eps := range tests {
		mockServer.UpdateNetwork(eps)

		newAd = withNewAddresses(&ad, eps, suffix)
		if err := testutil.ScanAndMatch(ctx, d, "", newAd); err != nil {
			t.Error(err)
		}
	}
}
