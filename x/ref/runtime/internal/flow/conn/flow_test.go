// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"crypto/rand"
	"io"
	"reflect"
	"runtime"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/rpc/version"
	"v.io/x/ref/test"
)

func totalSize(parts [][]byte) (s int) {
	for _, p := range parts {
		s += len(p)
	}
	return
}

func TestReadAtMost(t *testing.T) {
	input := [][]byte{
		make([]byte, 64),
		make([]byte, 32),
		make([]byte, 33),
		make([]byte, 96),
	}
	for i := range input {
		io.ReadFull(rand.Reader, input[i])
	}

	var send []byte
	var nextSlice, nextOffset, size int
	assert := func(wSend []byte, wNextSlice, wNextOffset, wSize int) {
		_, _, line, _ := runtime.Caller(1)
		if got, want := nextSlice, wNextSlice; got != want {
			t.Errorf("line: %v, next slice: got %v, want %v", line, got, want)
		}
		if got, want := nextOffset, wNextOffset; got != want {
			t.Errorf("line: %v, nextOffset: got %v, want %v", line, got, want)
		}
		if got, want := size, wSize; got != want {
			t.Errorf("line: %v, size: got %v, want %v", line, got, want)
		}
		if got, want := size, len(wSend); got != want {
			t.Errorf("line: %v, size: got %v, want %v", line, got, want)
		}
		if got, want := send, wSend; !reflect.DeepEqual(got, want) {
			t.Errorf("line: %v, send: got % 02x, want % 02x", line, got, want)
		}
	}

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, 1)
	assert(input[0][0:1], 0, 1, 1)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 100)
	assert(input[0][1:], 1, 0, len(input[0])-1)

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, len(input[0]))
	assert(input[0], 1, 0, len(input[0]))

	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, len(input[0]))
	assert(nil, 0, 0, 0)

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, len(input[0])+100)
	assert(input[0], 1, 0, len(input[0]))
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 1)
	assert(nil, 0, 0, 0)

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, 33)
	assert(input[0][:33], 0, 33, 33)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 1)
	assert(input[0][33:34], 0, 34, 1)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 100)
	assert(input[0][34:], 1, 0, 30)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 100)
	assert(nil, 0, 0, 0)

	partial := len(input[1]) / 3
	atMost := len(input[0]) + partial
	send, nextSlice, nextOffset, size = readAtMost(input, 0, 0, atMost)
	assert(append(input[0], input[1][:partial]...), 1, partial, atMost)

	prevOffset := partial
	partial = len(input[1]) - len(input[1])/3
	atMost = partial + len(input[2])
	send, nextSlice, nextOffset, size = readAtMost(input, nextSlice, nextOffset, atMost)
	assert(append(input[1][prevOffset:], input[2]...), 3, 0, atMost)

	send, nextSlice, nextOffset, size = readAtMost(input, nextSlice, nextOffset, 1000)
	assert(input[3], 4, 0, len(input[3]))

	send, nextSlice, nextOffset, size = readAtMost(input, nextSlice, nextOffset, 1000)
	assert(nil, 0, 0, 0)

	send, nextSlice, nextOffset, size = readAtMost(input, 0, 0, 1000)
	tmpOut := input[0]
	tmpOut = append(tmpOut, input[1]...)
	tmpOut = append(tmpOut, input[2]...)
	tmpOut = append(tmpOut, input[3]...)
	assert(tmpOut, 4, 0, totalSize(input))
}

func runFlowBenchmark(b *testing.B, ctx *context.T, dialed, accepted flow.Flow, rxbuf []byte, payload []byte) {
	errCh := make(chan error, 1)

	go func() {
		for i := 0; i < b.N; i++ {
			n, err := dialed.WriteMsg(payload)
			if err != nil || n != len(payload) {
				errCh <- err
				return
			}
		}
		errCh <- nil
		dialed.Close()
	}()

	var err error
	i := 0
	for {
		if rxbuf != nil {
			_, err = accepted.ReadMsg2(rxbuf)
		} else {
			_, err = accepted.ReadMsg()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			b.Fatal(err)
		}
		i++
	}

	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
}

func benchmarkFlow(b *testing.B, size int, bufferingFlow, userxbuf bool, rpcversion version.RPCVersion) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	payload := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, payload); err != nil {
		b.Fatal(err)
	}

	var rxbuf []byte
	if userxbuf {
		rxbuf = make([]byte, size+2048)
	}

	aflows := make(chan flow.Flow, 1)
	dc, _, derr, aerr := setupConns(b, "tcp", "", ctx, ctx, nil, aflows, nil, nil)
	if derr != nil || aerr != nil {
		b.Fatal(derr, aerr)
	}

	df, af := oneFlow(b, ctx, dc, aflows, time.Second)
	if bufferingFlow {
		df = NewBufferingFlow(ctx, df)
		af = NewBufferingFlow(ctx, af)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.SetBytes(int64(size) * 2)
	runFlowBenchmark(b, ctx, df, af, rxbuf, payload)
}

func BenchmarkFlow__RPC11__NewBuf__1KB(b *testing.B) {
	benchmarkFlow(b, 1000, false, false, version.RPCVersion11)
}
func BenchmarkFlow__RPC11__NewBuf__1MB(b *testing.B) {
	benchmarkFlow(b, 1000000, false, false, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__NewBuf__10MB(b *testing.B) {
	benchmarkFlow(b, 10000000, false, false, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__NewBuf__MTU(b *testing.B) {
	benchmarkFlow(b, DefaultMTU, false, false, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__UseBuf__1KB(b *testing.B) {
	benchmarkFlow(b, 1000, false, true, version.RPCVersion11)
}
func BenchmarkFlow__RPC11__UseBuf__1MB(b *testing.B) {
	benchmarkFlow(b, 1000000, false, true, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__UseBuf___10MB(b *testing.B) {
	benchmarkFlow(b, 10000000, false, true, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__UseBuf__MTU(b *testing.B) {
	benchmarkFlow(b, DefaultMTU, false, true, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__NewBuf__BufferingFlow__1KB(b *testing.B) {
	benchmarkFlow(b, 1000, true, false, version.RPCVersion11)
}
func BenchmarkFlow__RPC11__NewBuf__BufferingFlow__1MB(b *testing.B) {
	benchmarkFlow(b, 1000000, true, false, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__NewBuf__BufferingFlow__10MB(b *testing.B) {
	benchmarkFlow(b, 10000000, true, false, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__NewBuf__BufferingFlow__MTU(b *testing.B) {
	benchmarkFlow(b, DefaultMTU, true, false, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__UseBuf__BufferingFlow__1KB(b *testing.B) {
	benchmarkFlow(b, 1000, true, true, version.RPCVersion11)
}
func BenchmarkFlow__RPC11__UseBuf__BufferingFlow__1MB(b *testing.B) {
	benchmarkFlow(b, 1000000, true, true, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__UseBuf__BufferingFlow__10MB(b *testing.B) {
	benchmarkFlow(b, 10000000, true, true, version.RPCVersion11)
}

func BenchmarkFlow__RPC11__UseBuf__BufferingFlow__MTU(b *testing.B) {
	benchmarkFlow(b, DefaultMTU, true, true, version.RPCVersion11)
}
