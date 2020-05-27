// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/vtrace"
	"v.io/x/ref/runtime/internal/rpc/benchmark"
	tbm "v.io/x/ref/test/benchmark"
)

// CallEcho calls 'Echo' method 'iterations' times with the given payload size.
func CallEcho(b *testing.B, ctx *context.T, address string, iterations, payloadSize int, stats *tbm.Stats) {
	stub := benchmark.BenchmarkClient(address)
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}

	b.SetBytes(int64(payloadSize) * 2) // 2 for round trip of each payload.
	b.ResetTimer()                     // Exclude setup time from measurement.

	for i := 0; i < iterations; i++ {
		ictx, span := vtrace.WithNewTrace(ctx)
		b.StartTimer()
		start := time.Now()

		r, err := stub.Echo(ictx, payload)

		elapsed := time.Since(start)
		b.StopTimer()
		span.Finish()
		if err != nil {
			ictx.Fatalf("Echo failed: %v", err)
		}
		if !bytes.Equal(r, payload) {
			ictx.Fatalf("Echo returned %v, but expected %v", r, payload)
		}

		stats.Add(elapsed)
	}
}

// CallEchoStream calls 'EchoStream' method 'iterations' times. Each iteration sends
// 'chunkCnt' chunks on the stream and receives the same number of chunks back. Each
// chunk has the given payload size.
func CallEchoStream(b *testing.B, ctx *context.T, address string, iterations, chunkCnt, payloadSize int, stats *tbm.Stats) {
	done, _ := StartEchoStream(b, ctx, address, iterations, chunkCnt, payloadSize, stats)
	<-done
}

// StartEchoStream starts to call 'EchoStream' method 'iterations' times. This does
// not block, and returns a channel that will receive the number of iterations when
// it's done. It also returns a callback function to stop the streaming. Each iteration
// requests 'chunkCnt' chunks on the stream and receives that number of chunks back.
// Each chunk has the given payload size. Zero 'iterations' means unlimited.
func StartEchoStream(b *testing.B, ctx *context.T, address string, iterations, chunkCnt, payloadSize int, stats *tbm.Stats) (<-chan int, func()) { //nolint:gocyclo
	stub := benchmark.BenchmarkClient(address)
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}

	stop := make(chan struct{})
	stopped := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}
	done := make(chan int, 1)

	if b.N > 0 {
		// 2 for round trip of each payload.
		b.SetBytes(int64((iterations*chunkCnt/b.N)*payloadSize) * 2)
	}
	b.ResetTimer() // Exclude setup time from measurement.

	go func() {
		defer close(done)

		n := 0
		for ; !stopped() && (iterations == 0 || n < iterations); n++ {
			b.StartTimer()
			start := time.Now()

			stream, err := stub.EchoStream(ctx)
			if err != nil {
				ctx.Fatalf("EchoStream failed: %v", err)
			}

			rDone := make(chan error, 1)
			go func() {
				defer close(rDone)

				rStream := stream.RecvStream()
				i := 0
				for ; rStream.Advance(); i++ {
					r := rStream.Value()
					if !bytes.Equal(r, payload) {
						rDone <- fmt.Errorf("EchoStream returned %v, but expected %v", r, payload)
						return
					}
				}
				if i != chunkCnt {
					rDone <- fmt.Errorf("EchoStream returned %d chunks, but expected %d", i, chunkCnt)
					return
				}
				rDone <- rStream.Err()
			}()

			sStream := stream.SendStream()
			for i := 0; i < chunkCnt; i++ {
				if err = sStream.Send(payload); err != nil {
					ctx.Fatalf("EchoStream Send failed: %v", err)
				}
			}
			if err = sStream.Close(); err != nil {
				ctx.Fatalf("EchoStream Close failed: %v", err)
			}

			if err = <-rDone; err != nil {
				ctx.Fatalf("%v", err)
			}

			if err = stream.Finish(); err != nil {
				ctx.Fatalf("Finish failed: %v", err)
			}

			elapsed := time.Since(start)
			b.StopTimer()

			stats.Add(elapsed)
		}

		done <- n
	}()

	return done, func() {
		close(stop)
		<-done
	}
}
