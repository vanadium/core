// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib

import (
	"fmt"
	"net"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/x/ref/lib/signals"
)

func Main(opts Opts) error {
	ctx, shutdown := v23.Init()
	defer shutdown()
	return MainWithCtx(ctx, opts)
}

func MainWithCtx(ctx *context.T, opts Opts) error {
	name, stop, err := StartServers(ctx, v23.GetListenSpec(ctx), opts.MountName, opts.NhName, opts.AclFile, opts.PersistDir, "mounttable")
	if err != nil {
		return fmt.Errorf("mounttablelib.StartServers failed: %v", err)
	}
	defer stop()
	// Consumed by integration tests and the like.
	fmt.Printf("NAME=%s\n", name)
	ctx.Info("Received signal ", <-signals.ShutdownOnSignals(ctx))
	return nil
}

func StartServers(ctx *context.T, listenSpec rpc.ListenSpec, mountName, nhName, permsFile, persistDir, debugPrefix string) (string, func(), error) {
	var stopFuncs []func()
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		cancel()
		for i := len(stopFuncs) - 1; i >= 0; i-- {
			stopFuncs[i]()
		}
	}

	mt, err := NewMountTableDispatcher(ctx, permsFile, persistDir, debugPrefix)
	if err != nil {
		ctx.Errorf("NewMountTable failed: %v", err)
		return "", nil, err
	}
	ctx = v23.WithListenSpec(ctx, listenSpec)
	ctx, mtServer, err := v23.WithNewDispatchingServer(ctx, mountName, mt, options.ServesMountTable(true))
	if err != nil {

		ctx.Errorf("v23.WithNewServer failed: %v", err)
		return "", nil, err
	}
	stopFuncs = append(stopFuncs, func() {
		<-mtServer.Closed()
	})
	var mtName string
	var mtEndpoints []naming.Endpoint
	for {
		status := mtServer.Status()
		mtEndpoints = status.Endpoints
		mtName = mtEndpoints[0].Name()
		if mtEndpoints[0].Addr().Network() != "bidi" {
			break
		}
		<-status.Dirty
	}
	ctx.Infof("Mount table service at: %q endpoint: %s", mountName, mtName)

	if len(nhName) > 0 {
		// The ListenSpec code ensures that we have a valid address here.
		host, port, _ := net.SplitHostPort(listenSpec.Addrs[0].Address)
		if port != "" {
			neighborhoodListenSpec := listenSpec.Copy()
			neighborhoodListenSpec.Addrs[0].Address = net.JoinHostPort(host, "0")
			ctx = v23.WithListenSpec(ctx, neighborhoodListenSpec)
		}

		names := []string{}
		for _, ep := range mtEndpoints {
			names = append(names, ep.Name())
		}
		var nh rpc.Dispatcher
		if host == "127.0.0.1" || host == "localhost" {
			nh, err = NewLoopbackNeighborhoodDispatcher(nhName, names...)
		} else {
			nh, err = NewNeighborhoodDispatcher(nhName, names...)
		}
		if err != nil {
			ctx.Errorf("NewLoopback or Neighborhood dispatcher (host %s) failed: %v", host, err)
			return "", nil, err
		}

		ctx, nhServer, err := v23.WithNewDispatchingServer(ctx, naming.Join(mtName, "nh"), nh, options.ServesMountTable(true))
		if err != nil {
			ctx.Errorf("v23.WithNewServer failed: %v", err)
			stop()
			return "", nil, err
		}
		stopFuncs = append(stopFuncs, func() {
			<-nhServer.Closed()
		})
	}
	return mtName, stop, nil
}
