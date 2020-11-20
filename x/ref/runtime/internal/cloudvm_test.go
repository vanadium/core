// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"testing"

	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/runtime/internal/cloudvm"
	"v.io/x/ref/runtime/internal/cloudvm/cloudvmtest"
)

func chooseAddrs(t *testing.T, msg string, cfg *flags.VirtualizedFlags) []net.Addr {
	cvm, err := newCloudVM(context.Background(), logger.NewLogger("aws-test"), cfg)
	if err != nil {
		t.Fatalf("%v: %v", msg, err)
	}
	addrs, err := cvm.ChooseAddresses("wsh", nil)
	if err != nil {
		t.Fatalf("%v: %v", msg, err)
	}
	return addrs
}

func hasAddr(addrs []net.Addr, host string) bool {
	for _, a := range addrs {
		if strings.Contains(a.String(), host) {
			return true
		}
	}
	return false
}

func TestCloudVMProviders(t *testing.T) {
	awsHost, awsClose := cloudvmtest.StartAWSMetadataServer(t, true)
	defer awsClose()
	cloudvm.SetAWSMetadataHost(awsHost)

	gcpHost, gcpClose := cloudvmtest.StartGCPMetadataServer(t)
	defer gcpClose()
	cloudvm.SetGCPMetadataHost(gcpHost)

	cfg, err := flags.NewVirtualizedFlags()
	if err != nil {
		t.Fatal(err)
	}

	for _, provider := range []flags.VirtualizationProvider{flags.AWS, flags.GCP} {
		cfg.AdvertisePrivateAddresses = false
		cfg.VirtualizationProvider = flags.VirtualizationProviderFlag{Provider: provider}
		addrs := chooseAddrs(t, fmt.Sprintf("public %v", provider), cfg)
		if got, want := len(addrs), 1; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
		if got, want := addrs, cloudvmtest.WellKnownPublicIP; !hasAddr(got, want) {
			t.Errorf("got %v does not contain %v", got, want)
		}

		cfg.AdvertisePrivateAddresses = true
		addrs = chooseAddrs(t, fmt.Sprintf("public and private %v", provider), cfg)
		if got, want := addrs, cloudvmtest.WellKnownPublicIP; !hasAddr(got, want) {
			t.Errorf("got %v does not contain %v", got, want)
		}
		if got, want := addrs, cloudvmtest.WellKnownPrivateIP; !hasAddr(got, want) {
			t.Errorf("got %v does not contain %v", got, want)
		}
		if got, want := len(addrs), 2; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCloudVMNative(t *testing.T) {
	cfg, err := flags.NewVirtualizedFlags()
	if err != nil {
		t.Fatal(err)
	}

	// PublicAddress trumps a DNS name.
	cfg.AdvertisePrivateAddresses = false
	cfg.VirtualizationProvider.Set(string(flags.Native))
	cfg.PublicProtocol.Set("wsh")
	cfg.PublicAddress.Set("8.8.8.4:20")
	cfg.PublicDNSName.Set("myloadbalancer.com")
	addrs := chooseAddrs(t, "public", cfg)
	if got, want := len(addrs), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := addrs[0].Network(), "wsh"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := addrs[0].String(), "8.8.8.4:20"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// If no port is specified in the public address then none should
	// be returned.
	cfg.PublicAddress.Set("8.8.8.4")
	addrs = chooseAddrs(t, "public", cfg)
	if got, want := len(addrs), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := addrs[0].Network(), "wsh"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := addrs[0].String(), "8.8.8.4"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// With no public address, the DNS name is used.
	cfg.PublicAddress = flags.IPHostPortFlag{}
	addrs = chooseAddrs(t, "public", cfg)
	if got, want := len(addrs), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := addrs[0].String(), "myloadbalancer.com"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Set an explicit port.
	cfg.PublicDNSName.Set("myloadbalancer.com:80")
	addrs = chooseAddrs(t, "public", cfg)
	if got, want := len(addrs), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := addrs[0].String(), "myloadbalancer.com:80"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Google will return an ipv4 and ipv6 address and hence will
	// populate the IPs field of cfg.PublicAddrss.
	cfg.PublicAddress.Set("www.google.com")
	if len(cfg.PublicAddress.IP) < 2 {
		log.Fatalf("expected google.com to resolve to multiple IPs.")
	}
	addrs = chooseAddrs(t, "public", cfg)
	if got, want := len(addrs), 2; got < want {
		t.Fatalf("got %v less %v addresses", got, want)
	}
}
