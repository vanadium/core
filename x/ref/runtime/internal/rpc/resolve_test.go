// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc_test

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/runtime/factories/fake"
	irpc "v.io/x/ref/runtime/internal/rpc"
	grt "v.io/x/ref/runtime/internal/rt"
	"v.io/x/ref/test"
)

var commonFlags *flags.Flags

func init() {
	var err error
	commonFlags, err = flags.CreateAndRegister(flag.CommandLine, flags.Runtime)
	if err != nil {
		panic(err)
	}
	if err := commonFlags.Parse(os.Args[1:], nil); err != nil {
		panic(err)
	}
}

func setupRuntime() {
	listenSpec := rpc.ListenSpec{Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}}}

	rootctx, rootcancel := context.RootContext()
	ctx, cancel := context.WithCancel(rootctx)
	runtime, ctx, sd, err := grt.Init(ctx,
		nil,
		nil,
		nil,
		&listenSpec,
		nil,
		commonFlags.RuntimeFlags(),
		nil,
		&access.PermissionsSpec{},
		0)
	if err != nil {
		panic(err)
	}
	shutdown := func() {
		cancel()
		sd()
		rootcancel()
	}
	fake.InjectRuntime(runtime, ctx, shutdown)
}

type fakeService struct{}

func (f *fakeService) Foo(ctx *context.T, call rpc.ServerCall) error { return nil }

func TestResolveToEndpoint(t *testing.T) {
	setupRuntime()
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ns := v23.GetNamespace(ctx)

	proxyEp, _ := naming.ParseEndpoint("proxy.v.io:123#")
	proxyEpStr := proxyEp.String()
	proxyAddr := naming.JoinAddressName(proxyEpStr, "")
	if err := ns.Mount(ctx, "proxy", proxyAddr, time.Hour); err != nil {
		t.Fatalf("ns.Mount failed: %s", err)
	}

	_, server, err := v23.WithNewServer(ctx, "", &fakeService{}, nil)
	if err != nil {
		t.Fatalf("runtime.NewServer failed: %s", err)
	}

	notfound := fmt.Errorf("not found")
	testcases := []struct {
		address string
		result  string
		err     error
	}{
		{"/proxy.v.io:123#", proxyEpStr, nil},
		{"proxy.v.io:123", "", notfound},
		{"proxy", proxyEpStr, nil},
		{naming.JoinAddressName(ns.Roots()[0], "proxy"), proxyEpStr, nil},
		{proxyAddr, proxyEpStr, nil},
		{proxyEpStr, "", notfound},
		{"unknown", "", notfound},
	}
	for _, tc := range testcases {
		result, err := irpc.InternalServerResolveToEndpoint(ctx, server, tc.address)
		if (err == nil) != (tc.err == nil) {
			t.Errorf("Unexpected err for %q. Got %v, expected %v", tc.address, err, tc.err)
		}
		if result != tc.result {
			t.Errorf("Unexpected result for %q. Got %q, expected %q", tc.address, result, tc.result)
		}
	}
	if t.Failed() {
		t.Logf("proxyEpStr: %v", proxyEpStr)
		t.Logf("proxyAddr: %v", proxyAddr)
	}
}
