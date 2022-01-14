// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command simple is a client for the Simple service.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/static"
	"v.io/x/ref/test/compatibility/modules/simple"
)

var (
	nameFlag       string
	iterationsFlag int
)

func init() {
	flag.StringVar(&nameFlag, "name", os.ExpandEnv("users/${USER}/simpled"), "name of the server to connect to")
	flag.IntVar(&iterationsFlag, "num-calls", 2, "number of RPC calls to make to the server")
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()
	client := simple.SimpleClient(nameFlag)
	ticker := time.NewTicker(time.Millisecond * 500)
	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			now := time.Now().String()
			result, err := client.Ping(ctx, now)
			if err != nil {
				ctx.Errorf("%v.%v failed: %v", nameFlag, "ping", err)
			}
			if len(result) < 100 {
				fmt.Println(result)
			} else {
				fmt.Printf("%s[...] %d bytes\n", result[:100], len(result))
			}
			cancel()
		}
	}()
	<-signals.ShutdownOnSignals(ctx)
}
