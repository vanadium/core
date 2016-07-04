// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbaselib

import (
	"fmt"
	"os"
	"runtime/pprof"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/rpc"
	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/lib/dispatcher"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/services/syncbase/server"
	"v.io/x/ref/services/syncbase/vsync"
)

// Serve starts the Syncbase server. Returns rpc.Server and rpc.Dispatcher for
// use in the Mojo bindings, along with a cleanup function.
func Serve(ctx *context.T, opts Opts) (rpc.Server, rpc.Dispatcher, func()) {
	// Note: Adding the "runtime/pprof" import does not significantly increase the
	// binary size (only ~4500 bytes), so it seems okay to expose the option to
	// profile.
	if opts.CpuProfile != "" {
		f, err := os.Create(opts.CpuProfile)
		if err != nil {
			ctx.Fatal("Unable to create the cpu profile file: ", err)
		}
		defer f.Close()
		if err = pprof.StartCPUProfile(f); err != nil {
			ctx.Fatal("StartCPUProfile failed: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Create the rpc server before the service so that connections are shared between
	// clients in the service and the rpc server. (i.e. connections are shared if the
	// context returned from WithNewDispatchingServer is used for client calls).
	d := dispatcher.NewDispatcherWrapper()
	ctx, cancel := context.WithCancel(ctx)
	ctx, s, err := v23.WithNewDispatchingServer(ctx, opts.Name, d, options.ChannelTimeout(vsync.NeighborConnectionTimeout))
	if err != nil {
		ctx.Fatal("v23.WithNewDispatchingServer() failed: ", err)
	}

	perms, err := securityflag.PermissionsFromFlag()
	if err != nil {
		ctx.Fatal("securityflag.PermissionsFromFlag() failed: ", err)
	}
	if perms != nil {
		ctx.Infof("Read permissions from command line flag: %v", server.PermsString(ctx, perms))
	}
	var initialDB wire.Id
	if opts.InitialDB != "" {
		if initialDB, err = wire.ParseId(opts.InitialDB); err != nil {
			ctx.Fatalf("ParseId(%s) failed: %v", opts.InitialDB, err)
		}
	}
	service, err := server.NewService(ctx, server.ServiceOptions{
		Perms:           perms,
		RootDir:         opts.RootDir,
		Engine:          opts.Engine,
		SkipPublishInNh: opts.SkipPublishInNh,
		DevMode:         opts.DevMode,
		InitialDB:       initialDB,
	})
	if err != nil {
		ctx.Fatal("server.NewService() failed: ", err)
	}

	// Set the dispatcher in the dispatcher wrapper for the server to start responding
	// to incoming rpcs.
	d.SetDispatcher(server.NewDispatcher(service))

	// Publish syncgroups and such in the various places that they should be
	// published.
	// TODO(sadovsky): Improve comments (and perhaps method name) for AddNames.
	// It's not just publishing more names in the default mount table, and under
	// certain configurations it also publishes to the neighborhood.
	if err := service.AddNames(ctx, s); err != nil {
		ctx.Fatal("AddNames failed: ", err)
	}

	// Print mount name and endpoint.
	if opts.Name != "" {
		ctx.Info("Mounted at: ", opts.Name)
	}
	if eps := s.Status().Endpoints; len(eps) > 0 {
		// Integration tests wait for this to be printed before trying to access the
		// service.
		fmt.Printf("ENDPOINT=%s\n", eps[0].Name())
	}

	cleanup := func() {
		cancel()
		<-s.Closed()
		service.Close()
	}

	return s, d, cleanup
}
