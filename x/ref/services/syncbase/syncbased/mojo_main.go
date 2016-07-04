// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build mojo

package main

import (
	"flag"
	"log"

	"mojo/public/go/application"
	"mojo/public/go/bindings"
	"mojo/public/go/system"
	"mojom/syncbase"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/ref/runtime/factories/roaming"
	bridge "v.io/x/ref/services/syncbase/bridge_mojo"
	"v.io/x/ref/services/syncbase/syncbaselib"
)

//#include "mojo/public/c/system/handle.h"
import "C"

type delegate struct {
	ctx      *context.T
	disp     rpc.Dispatcher
	shutdown func()
	srv      rpc.Server
	cleanup  func()
	stubs    []*bindings.Stub
}

func (d *delegate) Initialize(actx application.Context) {
	opts := syncbaselib.Opts{}
	opts.InitFlags(flag.CommandLine)
	roaming.SetArgs(actx)
	d.ctx, d.shutdown = v23.Init() // calls flag.Parse
	if err := bridge.SeekAndSetBlessings(d.ctx, actx); err != nil {
		panic(err)
	}
	d.srv, d.disp, d.cleanup = syncbaselib.Serve(d.ctx, opts)
}

const numGoroutines = 100

func (d *delegate) Create(req syncbase.Syncbase_Request) {
	impl := bridge.NewMojoImpl(d.ctx, d.srv, d.disp)
	stub := syncbase.NewSyncbaseStub(req, impl, bindings.GetAsyncWaiter())
	d.stubs = append(d.stubs, stub)
	// Spawn a bunch of goroutines to handle incoming requests concurrently.
	// Note: It would be better to spawn a goroutine per request as requests
	// arrive, but Mojo's Go interfaces make this difficult at best. Discussions
	// with Mojo team ongoing (as of Nov 25, 2015).
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			for {
				if err := stub.ServeRequest(); err != nil {
					connErr, ok := err.(*bindings.ConnectionError)
					if !ok || !connErr.Closed() {
						log.Println(i, err)
					}
					break
				}
			}
		}(i)
	}
}

func (d *delegate) AcceptConnection(conn *application.Connection) {
	conn.ProvideServices(&syncbase.Syncbase_ServiceFactory{d})
}

func (d *delegate) Quit() {
	for _, stub := range d.stubs {
		stub.Close()
	}
	d.cleanup()
	d.shutdown()
}

//export MojoMain
func MojoMain(handle C.MojoHandle) C.MojoResult {
	application.Run(&delegate{}, system.MojoHandle(handle))
	return C.MOJO_RESULT_OK
}

// NOTE(nlacasse): Mojo runs Go code by calling MojoMain().  The main() method
// below is still needed because the Go tool won't build without it.
func main() {}
