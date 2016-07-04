// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package syncbased_vine implements syncbased, the Syncbase daemon, with a
// VINE server to enable test-specific network configuration.
package syncbased_vine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"v.io/v23"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/runtime/protocols/vine"
	"v.io/x/ref/services/syncbase/syncbaselib"
)

func Main(vineServerName string, vineTag string, opts syncbaselib.Opts) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	// Start a goroutine that repeatedly captures pprof data every minute.
	pprofDir := filepath.Join(opts.RootDir, "pprof")
	panicOnErr(os.MkdirAll(pprofDir, 0755))
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(time.Minute):
				capturePprof(pprofDir)
			}
		}
	}()

	// Start a VINE Server and modify our ctx to use the "vine" protocol.
	// The final argument is the discoveryTTL, which uses a sensible default if
	// called with 0.
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}},
	})
	ctx, shutdown, err := vine.Init(ctx, vineServerName, security.AllowEveryone(), vineTag, 0)
	if err != nil {
		panic(err)
	}
	defer shutdown()

	// Syncbase now uses a modified rpc.ListenSpec which has been set to use
	// the VINE protocol.
	s, _, cleanup := syncbaselib.Serve(ctx, opts)
	if eps := s.Status().Endpoints; len(eps) == 0 {
		panic("s.Status().Endpoints is empty")
	}
	gosh.SendVars(map[string]string{
		"ENDPOINT": s.Status().Endpoints[0].String(),
	})
	defer cleanup()
	ctx.Info("Received signal ", <-signals.ShutdownOnSignals(ctx))
}

func capturePprof(dir string) {
	now := time.Now().Format(time.RFC3339)

	heapFile, err := os.Create(filepath.Join(dir, fmt.Sprintf("heap-%s", now)))
	panicOnErr(err)
	panicOnErr(pprof.Lookup("heap").WriteTo(heapFile, 1))
	panicOnErr(heapFile.Close())

	goroutineFile, err := os.Create(filepath.Join(dir, fmt.Sprintf("goroutine-%s", now)))
	panicOnErr(err)
	panicOnErr(pprof.Lookup("goroutine").WriteTo(goroutineFile, 1))
	panicOnErr(goroutineFile.Close())
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}
