// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags_test

import (
	"flag"
	"reflect"
	"testing"

	"v.io/x/ref/lib/flags"
)

func TestVirtualizedFlags(t *testing.T) {
	fl, err := flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Virtualized)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fl.HasGroup(flags.Virtualized), true; got != want {
		t.Errorf("got %t, want %t", got, want)
	}

	expected := flags.VirtualizedFlags{
		AdvertisePrivateAddresses: true,
		PublicProtocol:            flags.TCPProtocolFlag{Protocol: "wsh"},
	}

	if got, want := fl.VirtualizedFlags(), expected; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if err := fl.Parse([]string{
		"--v23.virtualized.docker=true",
		"--v23.virtualized.provider=aws",
		"--v23.virtualized.tcp.public-protocol=tcp",
		"--v23.virtualized.tcp.public-address=8.8.2.2:17",
		"--v23.virtualized.dns.public-name=my-load-balancer:20",
	}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected = flags.VirtualizedFlags{
		Dockerized:                true,
		AdvertisePrivateAddresses: true,
		VirtualizationProvider:    flags.VirtualizationProviderFlag{Provider: flags.AWS},
	}
	expected.PublicDNSName.Set("my-load-balancer:20")
	expected.PublicProtocol.Set("tcp")
	expected.PublicAddress.Set("8.8.2.2:17")
	if got, want := fl.VirtualizedFlags(), expected; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
