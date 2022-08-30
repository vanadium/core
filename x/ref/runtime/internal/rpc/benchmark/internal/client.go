// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/vtrace"
	"v.io/x/ref/runtime/internal/rpc/benchmark"
	tbm "v.io/x/ref/test/benchmark"
)

type generator struct {
	data  []byte
	sizes []int32
	next  int
}

func payloadGenerator(maxSize int, random bool) func() []byte {
	payload := make([]byte, maxSize)
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}

	if random {
		// use to 2000 different buffer sizes.
		const nsizes = 2000
		gen := &generator{
			data:  payload,
			sizes: make([]int32, nsizes),
		}
		for i := 0; i < nsizes; i++ {
			gen.sizes[i] = rand.Int31n(int32(maxSize))
		}
		return func() []byte {
			if gen.next >= len(gen.sizes) {
				gen.next = 0
			}
			p := gen.data[:gen.sizes[gen.next]]
			gen.next += 1
			return p
		}
	}

	return func() []byte {
		return payload
	}
}

// CallEcho calls 'Echo' method 'iterations' times with the given payload size.
func CallEcho(b *testing.B, ctx *context.T, address string, iterations, payloadSize int, random bool, stats *tbm.Stats) {
	stub := benchmark.BenchmarkClient(address)

	genPayload := payloadGenerator(payloadSize, random)

	written := 0
	b.ResetTimer() // Exclude setup time from measurement.

	for i := 0; i < iterations; i++ {
		payload := genPayload()
		written += len(payload)
		ictx, span := vtrace.WithNewTrace(ctx, fmt.Sprintf("iter: % 8d", i), nil)

		b.StartTimer()
		start := time.Now()

		r, err := stub.Echo(ictx, payload)

		elapsed := time.Since(start)
		b.StopTimer()
		span.Finish(err)
		if err != nil {
			ictx.Fatalf("Echo failed: %v", err)
		}
		if !bytes.Equal(r, payload) {
			ictx.Fatalf("Echo returned %v, but expected %v", r, payload)
		}
		if stats != nil {
			stats.Add(elapsed)
		}
	}
	b.SetBytes(int64((written * 2) / iterations)) // 2 for round trip of each payload.
}

// CallEchoStream calls 'EchoStream' method 'iterations' times. Each iteration sends
// 'chunkCnt' chunks on the stream and receives the same number of chunks back. Each
// chunk has the given payload size.
func CallEchoStream(b *testing.B, ctx *context.T, address string, iterations, chunkCnt, payloadSize int, random bool, stats *tbm.Stats) {
	done, _ := StartEchoStream(b, ctx, address, iterations, chunkCnt, payloadSize, random, stats)
	status := <-done
	if b.N > 0 {
		// 2 for round trip of each payload.
		b.SetBytes(int64((status.Written * 2) / status.Iterations))
	}
}

// StreamStatus represents the number of iterations and the total
// bytes written (in one direction) over the stream.
type StreamStatus struct {
	Iterations int
	Written    int
}

// startEchoStream starts to call 'EchoStream' method 'iterations' times. This does
// not block, and returns a channel that will receive the number of iterations and total bytes written (StreamStatus) when
// it's done. It also returns a callback function to stop the streaming. Each iteration
// requests 'chunkCnt' chunks on the stream and receives that number of chunks back.
// Each chunk has the given payload size (or randomized). Zero 'iterations' means unlimited.
func StartEchoStream(b *testing.B, ctx *context.T, address string, iterations, chunkCnt, payloadSize int, random bool, stats *tbm.Stats) (<-chan StreamStatus, func()) { //nolint:gocyclo
	stub := benchmark.BenchmarkClient(address)

	genPayload := payloadGenerator(payloadSize, random)

	stop := make(chan struct{})
	stopped := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}
	done := make(chan StreamStatus, 1)
	written := 0
	b.ResetTimer() // Exclude setup time from measurement.

	go func() {
		defer close(done)

		n := 0
		for ; !stopped() && (iterations == 0 || n < iterations); n++ {
			payload := genPayload()
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

			written += len(payload) * chunkCnt

			if err = stream.Finish(); err != nil {
				ctx.Fatalf("Finish failed: %v", err)
			}

			b.StopTimer()
			elapsed := time.Since(start)
			if stats != nil {
				stats.Add(elapsed)
			}
		}

		done <- StreamStatus{Iterations: n, Written: written}
	}()

	return done, func() {
		close(stop)
		<-done
	}
}
