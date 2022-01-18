// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"os"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/test/compatibility/modules/simple"
)

func RunClient(ctx *context.T, name, msg string, numCalls int) error {
	client := simple.SimpleClient(name)
	ticker := time.NewTicker(time.Second)
	done := make(chan error)
	go func() {
		defer close(done)
		i := 1
		for range ticker.C {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			before := time.Now()
			result, err := client.Ping(ctx, msg)
			if err != nil {
				done <- fmt.Errorf("%v.%v failed: %v", name, "ping", err)
				return
			}
			fmt.Printf("RESPONSE=%v\n", result)
			fmt.Printf("TOOK=%v\n", time.Since(before))
			cancel()
			if i == numCalls {
				return
			}
			i++
		}
	}()
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-signals.ShutdownOnSignals(ctx):
	}
	return nil
}

type impl struct{ name string }

func (s *impl) Ping(ctx *context.T, call rpc.ServerCall, msg string) (response string, err error) {
	response = s.name + ":" + msg
	return
}

func RunServer(ctx *context.T, name string) error {
	ctx, server, err := v23.WithNewServer(ctx, name, simple.SimpleServer(&impl{name}), securityflag.NewAuthorizerOrDie(ctx))
	if err != nil {
		return err
	}
	fmt.Printf("PID=%d\n", os.Getpid())
	for _, ep := range server.Status().Endpoints {
		fmt.Printf("NAME=%s\n", ep.Name())
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
