// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"fmt"
	"testing"
	"time"

	"v.io/x/lib/cmdline"

	"v.io/v23/context"

	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/runtime/internal/rpc/benchmark/internal"
	"v.io/x/ref/test/benchmark"
)

var (
	server                                                         string
	iterations, chunkCnt, payloadSize, chunkCntMux, payloadSizeMux int
)

func main() {
	cmdRoot.Flags.StringVar(&server, "server", "", "Address of the server to connect to.")
	cmdRoot.Flags.IntVar(&iterations, "iterations", 100, "Number of iterations to run.")
	cmdRoot.Flags.IntVar(&chunkCnt, "chunk_count", 0, "Number of chunks to send per streaming RPC (if zero, use non-streaming RPC).")
	cmdRoot.Flags.IntVar(&payloadSize, "payload_size", 0, "Size of payload in bytes.")
	cmdRoot.Flags.IntVar(&chunkCntMux, "mux_chunk_count", 0, "Number of chunks to send in background.")
	cmdRoot.Flags.IntVar(&payloadSizeMux, "mux_payload_size", 0, "Size of payload to send in background.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

var cmdRoot = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runBenchmark),
	Name:   "benchmark",
	Short:  "Run the benchmark client",
	Long:   "Command benchmark runs the benchmark client.",
}

func runBenchmark(ctx *context.T, env *cmdline.Env, args []string) error {
	if chunkCntMux > 0 && payloadSizeMux > 0 {
		dummyB := testing.B{}
		_, stop := internal.StartEchoStream(&dummyB, ctx, server, 0, chunkCntMux, payloadSizeMux, nil)
		defer stop()
		ctx.Infof("Started background streaming (chunk_size=%d, payload_size=%d)", chunkCntMux, payloadSizeMux)
	}

	dummyB := testing.B{}
	stats := benchmark.NewStats(16)

	now := time.Now()
	if chunkCnt == 0 {
		internal.CallEcho(&dummyB, ctx, server, iterations, payloadSize, stats)
	} else {
		internal.CallEchoStream(&dummyB, ctx, server, iterations, chunkCnt, payloadSize, stats)
	}
	elapsed := time.Since(now)

	fmt.Printf("iterations: %d  chunk_count: %d  payload_size: %d\n", iterations, chunkCnt, payloadSize)
	fmt.Printf("elapsed time: %v\n", elapsed)
	stats.Print(env.Stdout)
	return nil
}
