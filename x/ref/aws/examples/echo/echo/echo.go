// Copyright 2018 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command echo is a client for the Echo service.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/vtrace"
	"v.io/x/ref/aws/lib/vxray"
	"v.io/x/ref/examples/echo"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/static"
)

var (
	nameFlag     string
	serverFlag   string
	cancelFlag   bool
	intervalFlag time.Duration
	deadlineFlag time.Duration
	sizeFlag     int
	iterations   int
	httpAddr     string
)

func init() {
	flag.StringVar(&nameFlag, "name", os.ExpandEnv("users/${USER}/echod"), "Name of the server to connect to")
	flag.StringVar(&serverFlag, "forward-to", "", "Set of comma separated server's for echo server to forward the request to, thus creating a chain")
	flag.BoolVar(&cancelFlag, "cancel", true, "Cancel every RPC context once it has returned successfully")
	flag.DurationVar(&intervalFlag, "interval", time.Second, "Interval between client calls")
	flag.DurationVar(&deadlineFlag, "deadline", time.Second*60, "Deadline for the rpc")
	flag.IntVar(&iterations, "iterations", -1, "number of iterations before exiting")
	flag.StringVar(&httpAddr, "http", "", "Specify an http address to run a simple http server to initiate x-ray enabled RPCs to the echo server")
}

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()

	ctx, _ = vxray.InitXRay(ctx,
		v23.GetRuntimeFlags().VtraceFlags,
		xray.Config{ServiceVersion: ""},
		vxray.EC2Plugin(),
		vxray.EKSCluster(),
		vxray.ContainerIDAndHost(),
		vxray.MergeLogging(true))

	servers := strings.Split(serverFlag, ",")
	if len(servers) > 0 {
		ctx.Infof("waiting for: %v servers: %v", len(servers), servers)
		waitForServers(ctx, servers)
		ctx.Infof("servers ready: %v", servers)
	}

	client := echo.EchoServiceClient(nameFlag)

	var wg sync.WaitGroup
	wg.Add(1)
	if len(httpAddr) > 0 {
		wg.Add(1)
		go func() {
			runHTTPServer(ctx, httpAddr, client)
			wg.Done()
		}()
	}

	ticker := time.NewTicker(intervalFlag)

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	samplingRequest := &vtrace.SamplingRequest{
		Name: nameFlag,
	}
	if len(servers) > 0 {
		samplingRequest.Method = "Ping"
	} else {
		samplingRequest.Method = "Echo"
	}
	go func() {
		nticks := 0
		for range ticker.C {
			ctx, cancel := context.WithTimeout(ctx, deadlineFlag)
			ctx, span := vtrace.WithNewTrace(ctx, "echo.client", samplingRequest)
			var err error
			if len(servers) > 0 {
				err = callPing(ctx, client, os.Stdout, servers)
			} else {
				err = callEcho(ctx, client, os.Stdout)
			}
			span.Finish(err)
			if cancelFlag {
				cancel()
			}
			nticks++
			if iterations > 0 && nticks >= iterations {
				wg.Done()
				return
			}
		}
	}()
	select {
	case <-done:
		time.Sleep(time.Second * 2)
	case <-signals.ShutdownOnSignals(ctx):
	}
}

// Requests of the form:
// <addr>/call and <addr>/call?forward-to=<server>
// will issue RPCs to the echo server.
// <addr>/quit will cause the client to exit gracefully.
func runHTTPServer(ctx *context.T, addr string, client echo.EchoServiceClientStub) {
	xrayHandler := xray.Handler(
		xray.NewFixedSegmentNamer("http.echo.client"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Merge in any xray related data from the http request.
			ctx = vxray.MergeHTTPRequestContext(ctx, r)

			pars := r.URL.Query()["forward-to"]
			var err error
			if len(pars) > 0 {
				err = callPing(ctx, client, w, strings.Split(pars[0], ","))
			} else {
				err = callEcho(ctx, client, w)
			}
			if err != nil {
				ctx.Errorf("error: %v", err)
			}
			fmt.Fprintf(w, "error: %v", err)
		}))
	http.Handle("/call", xrayHandler)
	httpDone := make(chan struct{})
	http.Handle("/quit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(httpDone)
	}))
	go func() {
		http.ListenAndServe(addr, nil)
	}()
	<-httpDone
}

func callEcho(ctx *context.T, client echo.EchoServiceClientStub, out io.Writer) error {
	now := time.Now().Format(time.StampMicro)
	if len(now) < sizeFlag {
		now += strings.Repeat(" ", sizeFlag-len(now))
	}
	result, err := client.Echo(ctx, now)
	if err != nil {
		ctx.Errorf("%v.%v failed: %v", nameFlag, "ping", err)
	}
	if len(result) < 100 {
		fmt.Fprintln(out, result)
	} else {
		fmt.Fprintf(out, "%s[...] %d bytes\n", result[:100], len(result))
	}
	return err
}

func callPing(ctx *context.T, client echo.EchoServiceClientStub, out io.Writer, servers []string) error {
	now := time.Now().Format(time.StampMicro)
	if len(now) < sizeFlag {
		now += strings.Repeat(" ", sizeFlag-len(now))
	}
	result, err := client.Ping(ctx, now, servers)
	if err != nil {
		ctx.Errorf("%v.%v failed: %v", servers, "ping", err)
	}
	if len(result) < 100 {
		fmt.Fprintln(out, result)
	} else {
		fmt.Fprintf(out, "%s[...] %d bytes\n", result[:100], len(result))
	}
	return err
}

func waitForServers(ctx *context.T, servers []string) {
	var wg sync.WaitGroup
	wg.Add(len(servers))
	ns := v23.GetNamespace(ctx)
	for _, server := range servers {
		go func(server string) {
			for {
				_, err := ns.Resolve(ctx, server)
				ctx.Infof("%v: %v: %v", server, err, errors.Is(err, naming.ErrNoSuchName))
				if errors.Is(err, naming.ErrNoSuchName) {
					time.Sleep(time.Second)
					continue
				}
				if err == nil {
					break
				}
				ctx.Infof("%v: %v\n", server, err)
			}
			wg.Done()
		}(server)
	}
	wg.Wait()
}
