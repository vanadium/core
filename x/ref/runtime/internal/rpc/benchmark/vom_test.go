// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmark_test

import (
	"bytes"
	"testing"
	"time"

	"v.io/v23/rpc"
	"v.io/v23/uniqueid"
	vtime "v.io/v23/vdlroot/time"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
)

var req = rpc.Request{
	Method:        "Echo",
	NumPosArgs:    1,
	Deadline:      vtime.Deadline{Time: time.Now()},
	EndStreamArgs: false,
	TraceRequest: vtrace.Request{
		SpanId:  uniqueid.Id{0x2a, 0xa2, 0xfd, 0xce, 0x42, 0xa7, 0xbd, 0x34, 0x88, 0x8, 0x11, 0x2f, 0x71, 0xce, 0x80, 0xa2},
		TraceId: uniqueid.Id{0x2a, 0xa2, 0xfd, 0xce, 0x42, 0xa7, 0xbd, 0x34, 0x88, 0x8, 0x11, 0x2f, 0x71, 0xce, 0x80, 0xa0},
	},
	Language: "en-US.UTF-8",
}

var resp = rpc.Response{
	EndStreamResults: true,
	NumPosResults:    1,
	TraceResponse: vtrace.Response{
		Trace: vtrace.TraceRecord{
			Id: uniqueid.Id{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		},
	},
}

func BenchmarkVomEncodeRequest(b *testing.B) {
	benchmarkEncode(b, req)
}

func BenchmarkVomDecodeRequest(b *testing.B) {
	benchmarkDecode(b, req, func() interface{} { return &rpc.Request{} })
}

func BenchmarkVomEncodeResponse(b *testing.B) {
	benchmarkEncode(b, resp)
}

func BenchmarkVomDecodeResponse(b *testing.B) {
	benchmarkDecode(b, resp, func() interface{} { return &rpc.Response{} })
}

func benchmarkEncode(b *testing.B, value interface{}) {
	typeBuf := &bytes.Buffer{}
	valBuf := &bytes.Buffer{}

	tenc := vom.NewTypeEncoder(typeBuf)
	enc := vom.NewEncoderWithTypeEncoder(valBuf, tenc)

	if err := enc.Encode(value); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		typeBuf.Reset()
		valBuf.Reset()
		if err := enc.Encode(value); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkDecode(b *testing.B, value interface{}, zero func() interface{}) {
	typeBuf := &bytes.Buffer{}
	valBuf := &bytes.Buffer{}

	tenc := vom.NewTypeEncoder(typeBuf)
	enc := vom.NewEncoderWithTypeEncoder(valBuf, tenc)
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(value); err != nil {
			b.Fatal(err)
		}
	}

	tdec := vom.NewTypeDecoder(typeBuf)
	tdec.Start()
	defer tdec.Stop()
	dec := vom.NewDecoderWithTypeDecoder(valBuf, tdec)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := dec.Decode(zero()); err != nil {
			b.Fatal(i, err)
		}
	}
}
