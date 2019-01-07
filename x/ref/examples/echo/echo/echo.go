// Copyright 2018 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command echo is a client for the Echo service.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/ref/examples/echo"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/static"
)

var (
	nameFlag     string
	cancelFlag   bool
	intervalFlag time.Duration
	deadlineFlag time.Duration
	sizeFlag     int
)

func init() {
	flag.StringVar(&nameFlag, "name", os.ExpandEnv("users/${USER}/echod"), "Name of the server to connect to")
	flag.BoolVar(&cancelFlag, "cancel", true, "Cancel every RPC context once it has returned successfully")
	flag.DurationVar(&intervalFlag, "interval", time.Second, "Interval between client calls")
	flag.DurationVar(&deadlineFlag, "deadline", time.Second*60, "Deadline for the rpc")
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()
	client := echo.EchoServiceClient(nameFlag)
	ticker := time.NewTicker(intervalFlag)
	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(ctx, deadlineFlag)
			now := time.Now().String()
			if len(now) < sizeFlag {
				now += strings.Repeat(" ", sizeFlag-len(now))
			}
			result, err := client.Echo(ctx, now)
			if err != nil {
				ctx.Errorf("%v.%v failed: %v", nameFlag, "ping", err)
			}
			if len(result) < 100 {
				fmt.Println(result)
			} else {
				fmt.Printf("%s[...] %d bytes\n", result[:100], len(result))
			}
			if cancelFlag {
				cancel()
			}
		}
	}()
	<-signals.ShutdownOnSignals(ctx)
}
