// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mounttabled wraps the mounttable so that it can be called by
// gosh.CmdFunc.
package mounttabled

import (
	"v.io/v23"
	"v.io/v23/rpc"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/services/mounttable/mounttablelib"
)

// This is essentially the same as the Main func in
// services/mounttable/mounttablelib/servers.go, except this one uses
// gosh.SendVars to report the mounttable name, rather than printing the
// variable directly to stdout.
func Main(opts mounttablelib.Opts) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}},
	})

	name, stop, err := mounttablelib.StartServers(ctx, v23.GetListenSpec(ctx), opts.MountName, opts.NhName, opts.AclFile, opts.PersistDir, "mounttable")
	if err != nil {
		panic(err)
	}
	gosh.SendVars(map[string]string{
		"NAME": name,
	})
	defer stop()
	ctx.Info("Received signal ", <-signals.ShutdownOnSignals(ctx))
}
