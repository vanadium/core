// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmark_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/lib/ibe"
	lsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/bcrypter"
	_ "v.io/x/ref/runtime/factories/roaming"
	bm "v.io/x/ref/runtime/internal/rpc/benchmark"
	"v.io/x/ref/runtime/internal/rpc/benchmark/internal"
	"v.io/x/ref/services/xproxy/xproxy"
	"v.io/x/ref/test"
	"v.io/x/ref/test/benchmark"
	"v.io/x/ref/test/testutil"
)

var (
	serverAddr, expServerAddr, tpcServerAddr, proxiedServerAddr string
	ctx, expCtx, tpcCtx                                         *context.T
	serverCrypter, clientCrypter                                *bcrypter.Crypter
)

// Benchmarks for non-streaming RPC.
func runEcho(b *testing.B, payloadSize int) {
	internal.CallEcho(b, ctx, serverAddr, b.N, payloadSize, benchmark.AddStats(b, 16))
}

func Benchmark____1B(b *testing.B) { runEcho(b, 1) }
func Benchmark___10B(b *testing.B) { runEcho(b, 10) }
func Benchmark__100B(b *testing.B) { runEcho(b, 100) }
func Benchmark___1KB(b *testing.B) { runEcho(b, 1000) }
func Benchmark__10KB(b *testing.B) { runEcho(b, 10000) }
func Benchmark_100KB(b *testing.B) { runEcho(b, 100000) }

// Benchmark for non-streaming RPC with a Client ExpiryCaveat.
func runEchoClientExpiry(b *testing.B, payloadSize int) {
	internal.CallEcho(b, expCtx, serverAddr, b.N, payloadSize, benchmark.AddStats(b, 16))
}

func Benchmark____1B_ClientExpiryCav(b *testing.B) { runEchoClientExpiry(b, 1) }
func Benchmark___10B_ClientExpiryCav(b *testing.B) { runEchoClientExpiry(b, 10) }
func Benchmark__100B_ClientExpiryCav(b *testing.B) { runEchoClientExpiry(b, 100) }
func Benchmark___1KB_ClientExpiryCav(b *testing.B) { runEchoClientExpiry(b, 1000) }
func Benchmark__10KB_ClientExpiryCav(b *testing.B) { runEchoClientExpiry(b, 10000) }
func Benchmark_100KB_ClientExpiryCav(b *testing.B) { runEchoClientExpiry(b, 100000) }

func runEchoServerExpiry(b *testing.B, payloadSize int) {
	internal.CallEcho(b, ctx, expServerAddr, b.N, payloadSize, benchmark.AddStats(b, 16))
}

func Benchmark____1B_ServerExpiryCav(b *testing.B) { runEchoServerExpiry(b, 1) }
func Benchmark___10B_ServerExpiryCav(b *testing.B) { runEchoServerExpiry(b, 10) }
func Benchmark__100B_ServerExpiryCav(b *testing.B) { runEchoServerExpiry(b, 100) }
func Benchmark___1KB_ServerExpiryCav(b *testing.B) { runEchoServerExpiry(b, 1000) }
func Benchmark__10KB_ServerExpiryCav(b *testing.B) { runEchoServerExpiry(b, 10000) }
func Benchmark_100KB_ServerExpiryCav(b *testing.B) { runEchoServerExpiry(b, 100000) }

// Benchmark for non-streaming RPC with a Client ThirdPartyCaveat.
func runEchoClientTPC(b *testing.B, payloadSize int) {
	internal.CallEcho(b, tpcCtx, serverAddr, b.N, payloadSize, benchmark.AddStats(b, 16))
}

func Benchmark____1B_ClientTPCav(b *testing.B) { runEchoClientTPC(b, 1) }
func Benchmark___10B_ClientTPCav(b *testing.B) { runEchoClientTPC(b, 10) }
func Benchmark__100B_ClientTPCav(b *testing.B) { runEchoClientTPC(b, 100) }
func Benchmark___1KB_ClientTPCav(b *testing.B) { runEchoClientTPC(b, 1000) }
func Benchmark__10KB_ClientTPCav(b *testing.B) { runEchoClientTPC(b, 10000) }
func Benchmark_100KB_ClientTPCav(b *testing.B) { runEchoClientTPC(b, 100000) }

// Benchmark for non-streaming RPC with a Server ThirdPartyCaveat.
func runEchoServerTPC(b *testing.B, payloadSize int) {
	internal.CallEcho(b, ctx, tpcServerAddr, b.N, payloadSize, benchmark.AddStats(b, 16))
}

func Benchmark____1B_ServerTPCav(b *testing.B) { runEchoServerTPC(b, 1) }
func Benchmark___10B_ServerTPCav(b *testing.B) { runEchoServerTPC(b, 10) }
func Benchmark__100B_ServerTPCav(b *testing.B) { runEchoServerTPC(b, 100) }
func Benchmark___1KB_ServerTPCav(b *testing.B) { runEchoServerTPC(b, 1000) }
func Benchmark__10KB_ServerTPCav(b *testing.B) { runEchoServerTPC(b, 10000) }
func Benchmark_100KB_ServerTPCav(b *testing.B) { runEchoServerTPC(b, 100000) }

// Benchmark for non-streaming RPC with a proxied Server.
func runEchoProxiedServer(b *testing.B, payloadSize int) {
	internal.CallEcho(b, ctx, proxiedServerAddr, b.N, payloadSize, benchmark.AddStats(b, 16))
}

func Benchmark____1B_ProxiedServer(b *testing.B) { runEchoProxiedServer(b, 1) }
func Benchmark___10B_ProxiedServer(b *testing.B) { runEchoProxiedServer(b, 10) }
func Benchmark__100B_ProxiedServer(b *testing.B) { runEchoProxiedServer(b, 100) }
func Benchmark___1KB_ProxiedServer(b *testing.B) { runEchoProxiedServer(b, 1000) }
func Benchmark__10KB_ProxiedServer(b *testing.B) { runEchoProxiedServer(b, 10000) }
func Benchmark_100KB_ProxiedServer(b *testing.B) { runEchoProxiedServer(b, 100000) }

// Benchmarks for streaming RPC.
func runEchoStream(b *testing.B, chunkCnt, payloadSize int) {
	internal.CallEchoStream(b, ctx, serverAddr, b.N, chunkCnt, payloadSize, benchmark.AddStats(b, 16))
}

func Benchmark____1_chunk_____1B(b *testing.B) { runEchoStream(b, 1, 1) }
func Benchmark____1_chunk____10B(b *testing.B) { runEchoStream(b, 1, 10) }
func Benchmark____1_chunk___100B(b *testing.B) { runEchoStream(b, 1, 100) }
func Benchmark____1_chunk____1KB(b *testing.B) { runEchoStream(b, 1, 1000) }
func Benchmark____1_chunk___10KB(b *testing.B) { runEchoStream(b, 1, 10000) }
func Benchmark____1_chunk__100KB(b *testing.B) { runEchoStream(b, 1, 100000) }
func Benchmark___10_chunk_____1B(b *testing.B) { runEchoStream(b, 10, 1) }
func Benchmark___10_chunk____10B(b *testing.B) { runEchoStream(b, 10, 10) }
func Benchmark___10_chunk___100B(b *testing.B) { runEchoStream(b, 10, 100) }
func Benchmark___10_chunk____1KB(b *testing.B) { runEchoStream(b, 10, 1000) }
func Benchmark___10_chunk___10KB(b *testing.B) { runEchoStream(b, 10, 10000) }
func Benchmark___10_chunk__100KB(b *testing.B) { runEchoStream(b, 10, 100000) }
func Benchmark__100_chunk_____1B(b *testing.B) { runEchoStream(b, 100, 1) }
func Benchmark__100_chunk____10B(b *testing.B) { runEchoStream(b, 100, 10) }
func Benchmark__100_chunk___100B(b *testing.B) { runEchoStream(b, 100, 100) }
func Benchmark__100_chunk____1KB(b *testing.B) { runEchoStream(b, 100, 1000) }
func Benchmark__100_chunk___10KB(b *testing.B) { runEchoStream(b, 100, 10000) }
func Benchmark__100_chunk__100KB(b *testing.B) { runEchoStream(b, 100, 100000) }
func Benchmark___1K_chunk_____1B(b *testing.B) { runEchoStream(b, 1000, 1) }
func Benchmark___1K_chunk____10B(b *testing.B) { runEchoStream(b, 1000, 10) }
func Benchmark___1K_chunk___100B(b *testing.B) { runEchoStream(b, 1000, 100) }
func Benchmark___1K_chunk____1KB(b *testing.B) { runEchoStream(b, 1000, 1000) }
func Benchmark___1K_chunk___10KB(b *testing.B) { runEchoStream(b, 1000, 10000) }
func Benchmark___1K_chunk__100KB(b *testing.B) { runEchoStream(b, 1000, 100000) }

// Benchmarks for per-chunk throughput in streaming RPC.
func runPerChunk(b *testing.B, payloadSize int) {
	internal.CallEchoStream(b, ctx, serverAddr, 1, b.N, payloadSize, benchmark.NewStats(1))
}

func Benchmark__per_chunk____1B(b *testing.B) { runPerChunk(b, 1) }
func Benchmark__per_chunk___10B(b *testing.B) { runPerChunk(b, 10) }
func Benchmark__per_chunk__100B(b *testing.B) { runPerChunk(b, 100) }
func Benchmark__per_chunk___1KB(b *testing.B) { runPerChunk(b, 1000) }
func Benchmark__per_chunk__10KB(b *testing.B) { runPerChunk(b, 10000) }
func Benchmark__per_chunk_100KB(b *testing.B) { runPerChunk(b, 100000) }

// Benchmarks for non-streaming RPC while running streaming RPC in background.
func runMux(b *testing.B, payloadSize, chunkCntB, payloadSizeB int) {
	_, stop := internal.StartEchoStream(&testing.B{}, ctx, serverAddr, 0, chunkCntB, payloadSizeB, benchmark.NewStats(1))
	internal.CallEcho(b, ctx, serverAddr, b.N, payloadSize, benchmark.AddStats(b, 16))
	stop()
}

func Benchmark___10B_mux__100_chunks___10B(b *testing.B) { runMux(b, 10, 100, 10) }
func Benchmark___10B_mux__100_chunks__100B(b *testing.B) { runMux(b, 10, 100, 100) }
func Benchmark___10B_mux__100_chunks___1KB(b *testing.B) { runMux(b, 10, 100, 1000) }
func Benchmark___10B_mux__100_chunks__10KB(b *testing.B) { runMux(b, 10, 100, 10000) }
func Benchmark___10B_mux___1K_chunks___10B(b *testing.B) { runMux(b, 10, 1000, 10) }
func Benchmark___10B_mux___1K_chunks__100B(b *testing.B) { runMux(b, 10, 1000, 100) }
func Benchmark___10B_mux___1K_chunks___1KB(b *testing.B) { runMux(b, 10, 1000, 1000) }
func Benchmark___10B_mux___1K_chunks__10KB(b *testing.B) { runMux(b, 10, 1000, 10000) }
func Benchmark___1KB_mux__100_chunks___10B(b *testing.B) { runMux(b, 1000, 100, 10) }
func Benchmark___1KB_mux__100_chunks__100B(b *testing.B) { runMux(b, 1000, 100, 100) }
func Benchmark___1KB_mux__100_chunks___1KB(b *testing.B) { runMux(b, 1000, 100, 1000) }
func Benchmark___1KB_mux__100_chunks__10KB(b *testing.B) { runMux(b, 1000, 100, 10000) }
func Benchmark___1KB_mux___1K_chunks___10B(b *testing.B) { runMux(b, 1000, 1000, 10) }
func Benchmark___1KB_mux___1K_chunks__100B(b *testing.B) { runMux(b, 1000, 1000, 100) }
func Benchmark___1KB_mux___1K_chunks___1KB(b *testing.B) { runMux(b, 1000, 1000, 1000) }
func Benchmark___1KB_mux___1K_chunks__10KB(b *testing.B) { runMux(b, 1000, 1000, 10000) }

// Benchmark for measuring RPC connection time including authentication.
//
// rpc.Client doesn't export an interface for closing connection. So we
// recreate a client on every iteration (avoiding re-use of underlying
// network connections between iterations since the stream manager is
// recreated with the client).
func benchmarkConnectionEstablishment(b *testing.B, ctx *context.T, server string) {
	var buf [1]byte
	for i := 0; i < b.N; i++ {
		// The client and server are running with the same principal
		// here (since they are both keyed off the same 'ctx').
		cctx, cancel := context.WithCancel(ctx)
		cctx, _, err := v23.WithNewClient(cctx)
		if err != nil {
			b.Fatalf("Iteration %d/%d: %v", i, b.N, err)
		}
		// Execute an RPC to ensure that the connection is established
		// by both ends.
		if _, err := bm.BenchmarkClient(server).Echo(cctx, buf[:]); err != nil {
			b.Fatal(err)
		}
		cancel()
	}
}

func benchmarkPrivateConnectionEstablishment(b *testing.B, serverAuth ...security.BlessingPattern) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, server, err := v23.WithNewServer(
		bcrypter.WithCrypter(ctx, serverCrypter),
		"",
		internal.NewService(),
		security.DefaultAuthorizer(),
		options.ServerPeers(serverAuth))
	if err != nil {
		b.Fatal(err)
	}
	cctx := bcrypter.WithCrypter(ctx, clientCrypter)
	benchmarkConnectionEstablishment(b, cctx, server.Status().Endpoints[0].Name())
}

func BenchmarkConnectionEstablishment(b *testing.B) {
	benchmarkConnectionEstablishment(b, ctx, serverAddr)
}

func BenchmarkProxiedConnectionEstablishment(b *testing.B) {
	benchmarkConnectionEstablishment(b, ctx, proxiedServerAddr)
}

func BenchmarkPrivateConnectionEstablishment1(b *testing.B) {
	benchmarkPrivateConnectionEstablishment(b, "root:client")
}

func BenchmarkPrivateConnectionEstablishment3(b *testing.B) {
	benchmarkPrivateConnectionEstablishment(b, "root:client", "root:client2", "root:client3")
}

// A single empty test to avoid:
// testing: warning: no tests to run
// from showing up when running benchmarks in this package via "go test"
func TestNoOp(t *testing.T) {}

func TestMain(m *testing.M) {
	// We do not use defer here since this program will exit at the end of
	// this function through os.Exit().
	var shutdown v23.Shutdown
	ctx, shutdown = test.V23Init()

	setupServerClient(ctx)
	setupExpiryCaveatServerClient(ctx)
	setupThirdPartyCaveatServerClient(ctx)
	setupProxiedServerClient(ctx)
	setupPrivateMutualAuthentication(ctx)

	r := benchmark.RunTestMain(m)
	shutdown()
	os.Exit(r)
}

func setupServerClient(ctx *context.T) {
	// Server with no ThirdPartyCaveat.
	_, server, err := v23.WithNewServer(ctx, "", internal.NewService(), security.DefaultAuthorizer())
	if err != nil {
		ctx.Fatalf("NewServer failed: %v", err)
	}
	serverAddr = server.Status().Endpoints[0].Name()
	// Create a Conn to exclude the Conn setup time from the benchmark.
	internal.CallEcho(&testing.B{}, ctx, serverAddr, 1, 0, benchmark.NewStats(1))
}

func setupExpiryCaveatServerClient(ctx *context.T) {
	p := v23.GetPrincipal(ctx)
	b, _ := p.BlessingStore().Default()
	expcav, err := security.NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		ctx.Fatal(err)
	}
	expcavb, err := p.Bless(p.PublicKey(), b, "expcav", expcav)
	if err != nil {
		ctx.Fatal(err)
	}
	store := lsecurity.FixedBlessingsStore(expcavb, nil)
	expcavp, err := lsecurity.ForkPrincipal(p, store, p.Roots())
	if err != nil {
		ctx.Fatal(err)
	}
	expCtx, err = v23.WithPrincipal(ctx, expcavp)
	if err != nil {
		ctx.Fatal(err)
	}

	// Create a server with the ExpiryCaveat principal.
	_, expServer, err := v23.WithNewServer(expCtx, "", internal.NewService(), security.DefaultAuthorizer())
	if err != nil {
		ctx.Fatalf("NewServer failed: %v", err)
	}
	expServerAddr = expServer.Status().Endpoints[0].Name()

	// Create Conns to exclude the Conn setup time from the benchmark.
	internal.CallEcho(&testing.B{}, expCtx, serverAddr, 1, 0, benchmark.NewStats(1))
	internal.CallEcho(&testing.B{}, ctx, expServerAddr, 1, 0, benchmark.NewStats(1))
}

func setupThirdPartyCaveatServerClient(ctx *context.T) {
	// Create a DischargeServer and a principal with a corresponding ThirdPartyCaveat.
	tpcav := internal.NewDischargeServer(ctx)
	p := v23.GetPrincipal(ctx)
	b, _ := p.BlessingStore().Default()
	tpcavb, err := p.Bless(p.PublicKey(), b, "tpcav", tpcav)
	if err != nil {
		ctx.Fatal(err)
	}
	store := lsecurity.FixedBlessingsStore(tpcavb, nil)
	tpcavp, err := lsecurity.ForkPrincipal(p, store, p.Roots())
	if err != nil {
		ctx.Fatal(err)
	}
	tpcCtx, err = v23.WithPrincipal(ctx, tpcavp)
	if err != nil {
		ctx.Fatal(err)
	}

	// Create a server with the ThirdPartyCaveat principal.
	_, tpcServer, err := v23.WithNewServer(tpcCtx, "", internal.NewService(), security.DefaultAuthorizer())
	if err != nil {
		ctx.Fatalf("NewServer failed: %v", err)
	}
	tpcServerAddr = tpcServer.Status().Endpoints[0].Name()

	// Create Conns to exclude the Conn setup time from the benchmark.
	internal.CallEcho(&testing.B{}, tpcCtx, serverAddr, 1, 0, benchmark.NewStats(1))
	internal.CallEcho(&testing.B{}, ctx, tpcServerAddr, 1, 0, benchmark.NewStats(1))
}

func setupProxiedServerClient(ctx *context.T) {
	// Create a proxy.
	pxy, err := xproxy.New(ctx, "", security.DefaultAuthorizer())
	if err != nil {
		ctx.Fatal(err)
	}
	pname := pxy.ListeningEndpoints()[0].Name()
	// Create a server that listens on the proxy.
	pCtx := v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: pname})
	_, proxiedServer, err := v23.WithNewServer(pCtx, "", internal.NewService(), security.DefaultAuthorizer())
	if err != nil {
		ctx.Fatal(err)
	}
	status := testutil.WaitForProxyEndpoints(proxiedServer, pname)
	fmt.Fprintf(os.Stderr, "WaitForProxyEndpoints: %v : %v -> %v\n", pname, proxiedServer.Status().Endpoints, status.Endpoints)
	proxiedServerAddr = status.Endpoints[0].Name()
	// Create Conns to exclude the Conn setup time from the benchmark.
	internal.CallEcho(&testing.B{}, ctx, proxiedServerAddr, 1, 0, benchmark.NewStats(1))
}

func setupPrivateMutualAuthentication(ctx *context.T) {
	master, err := ibe.SetupBB2()
	if err != nil {
		ctx.Fatalf("ibe.SetupBB2: %v", err)
	}
	root := bcrypter.NewRoot("root", master)
	clientKey, err := root.Extract(ctx, "root:client")
	if err != nil {
		ctx.Fatal(err)
	}

	// The client needs a key to be able to decrypt the server blessings.
	clientCrypter = bcrypter.NewCrypter()
	if err := clientCrypter.AddKey(ctx, clientKey); err != nil {
		ctx.Fatal(err)
	}

	// The server needs the params object so that it can encrypt its blessings.
	serverCrypter = bcrypter.NewCrypter()
	if err := serverCrypter.AddParams(ctx, root.Params()); err != nil {
		ctx.Fatal(err)
	}
}
