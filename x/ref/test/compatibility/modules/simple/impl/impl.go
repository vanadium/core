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

func RunClient(ctx *context.T, name string, numCalls int) error {
	client := simple.SimpleClient(name)
	ticker := time.NewTicker(time.Second)
	done := make(chan struct{})
	go func() {
		i := 1
		for range ticker.C {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			now := time.Now().String()
			result, err := client.Ping(ctx, now)
			if err != nil {
				ctx.Errorf("%v.%v failed: %v", name, "ping", err)
			}
			fmt.Println(result)
			cancel()
			if i == numCalls {
				close(done)
				return
			}
			i++
		}
	}()
	select {
	case <-done:
	case <-signals.ShutdownOnSignals(ctx):
	}
	return nil
}

type impl struct{}

func (s *impl) Ping(ctx *context.T, call rpc.ServerCall, msg string) (response string, err error) {
	response = fmt.Sprintf("%s: %v\n", time.Now(), msg)
	ctx.Infof("%v: %v", call.RemoteEndpoint(), msg)
	return
}

func RunServer(ctx *context.T, name string) error {
	ctx, server, err := v23.WithNewServer(ctx, name, simple.SimpleServer(&impl{}), securityflag.NewAuthorizerOrDie(ctx))
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
