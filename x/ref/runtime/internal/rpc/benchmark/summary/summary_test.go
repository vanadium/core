// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/runtime/internal/rpc/benchmark/internal"
	"v.io/x/ref/test"
)

var (
	serverAddr, expServerAddr string
	ctx                       *context.T
)

func runConnections(b *testing.B, payloadSize int) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx, _, _ := v23.WithNewClient(ctx)
		internal.CallEcho(&testing.B{}, ctx, serverAddr, 1, payloadSize, false, nil)
	}
}

func runEcho(b *testing.B, payloadSize int, random bool) {
	b.ReportAllocs()
	internal.CallEcho(b, ctx, serverAddr, b.N, payloadSize, random, nil)
}

func runEchoStream(b *testing.B, chunkCnt, payloadSize int, random bool) {
	b.ReportAllocs()
	internal.CallEchoStream(b, ctx, serverAddr, b.N, chunkCnt, payloadSize, random, nil)
}

func Benchmark_ConnectionSetup___0KB(b *testing.B) { runConnections(b, 0) }

func Benchmark_ConnectionSetup__10KB(b *testing.B) { runConnections(b, 10000) }

func Benchmark__Echo____________10KB(b *testing.B) { runEcho(b, 10000, false) }
func Benchmark__Echo_____________1MB(b *testing.B) { runEcho(b, 1000000, false) }

func Benchmark__Echo________Rnd_10KB(b *testing.B) { runEcho(b, 10000, true) }
func Benchmark__Echo_Stream_____10KB(b *testing.B) { runEchoStream(b, 10, 10000, false) }
func Benchmark__Echo_Stream____500KB(b *testing.B) { runEchoStream(b, 10, 500000, false) }
func Benchmark__Echo_Stream______1MB(b *testing.B) { runEchoStream(b, 10, 1000000, false) }
func Benchmark__Echo_Stream__Rnd_1MB(b *testing.B) { runEchoStream(b, 10, 1000000, true) }

func setupServerClient(ctx *context.T) {
	_, server, err := v23.WithNewServer(ctx, "", internal.NewService(), security.DefaultAuthorizer())
	if err != nil {
		ctx.Fatalf("NewServer failed: %v", err)
	}
	serverAddr = server.Status().Endpoints[0].Name()
	internal.CallEcho(&testing.B{}, ctx, serverAddr, 1, 0, false, nil)
}

func TestMain(m *testing.M) {
	var shutdown v23.Shutdown
	ctx, shutdown = test.V23Init()

	setupServerClient(ctx)

	r := m.Run()
	shutdown()

	os.Exit(r)
}
